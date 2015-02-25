package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/jtacoma/uritemplates"
)

// routes is a map containing all UriTemplates indexed by name.
var routes = make(map[string]*uritemplates.UriTemplate)

func init() {
	// Add templates to the route map
	addURITemplate("GetBuilds", "/api/v3/applications{/username,name}/builds{?commit,branch,status,limit,skip,sort,result}")
	addURITemplate("GetDockerRepository", "/api/v2/builds{/buildId}/docker")
}

// addURITemplate adds rawTemplate to routes using name as the key. Should only
// be used from init().
func addURITemplate(name, rawTemplate string) {
	uriTemplate, err := uritemplates.Parse(rawTemplate)
	if err != nil {
		panic(err)
	}
	routes[name] = uriTemplate
}

// APIClient is a very dumb client for the wercker API
type APIClient struct {
	baseURL string
	client  *http.Client
	options *GlobalOptions
	logger  *LogEntry
}

// NewAPIClient returns our dumb client
func NewAPIClient(options *GlobalOptions) *APIClient {
	logger := rootLogger.WithFields(LogFields{
		"Logger": "API",
	})
	return &APIClient{
		baseURL: options.BaseURL,
		client:  &http.Client{},
		options: options,
		logger:  logger,
	}
}

// URL joins some strings to the endpoint
func (c *APIClient) URL(parts ...string) string {
	realParts := append([]string{c.baseURL}, parts...)
	return strings.Join(realParts, "/")
}

// GetBody does a GET request. If the status code is 200, it will return the
// body.
func (c *APIClient) GetBody(parts ...string) ([]byte, error) {
	res, err := c.Get(parts...)

	if res.StatusCode != 200 {
		body, _ := ioutil.ReadAll(res.Body)
		c.logger.Debugln(string(body))
		return nil, fmt.Errorf("Got non-200 response: %d", res.StatusCode)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return buf, nil
}

// Get will do a GET http request, it adds the wercker endpoint and will add
// some default headers.
func (c *APIClient) Get(parts ...string) (*http.Response, error) {
	url := c.URL(parts...)
	c.logger.Debugln("API Get:", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.logger.WithField("Error", err).Debug("Unable to create request to wercker API")
		return nil, err
	}

	AddRequestHeaders(req)
	c.addAuthToken(req)

	return c.client.Do(req)
}

// GetBuildsOptions are the optional parameters associated with GetBuilds
type GetBuildsOptions struct {
	Sort   string `qs:"sort"`
	Limit  int    `qs:"limit"`
	Skip   int    `qs:"skip"`
	Commit string `qs:"commit"`
	Branch string `qs:"branch"`
	Status string `qs:"status"`
	Result string `qs:"result"`
}

// APIBuild represents a build from wercker api.
type APIBuild struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	Status     string  `json:"status"`
	Result     string  `json:"result"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	FinishedAt string  `json:"finishedAt"`
	Progress   float64 `json:"progress"`
}

// GetBuilds will fetch multiple builds for application username/name.
func (c *APIClient) GetBuilds(username, name string, options *GetBuildsOptions) ([]*APIBuild, error) {
	model := queryString(options)
	model["username"] = username
	model["name"] = name

	template := routes["GetBuilds"]
	url, err := template.Expand(model)
	if err != nil {
		return nil, err
	}

	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, c.parseError(res)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var payload []*APIBuild
	err = json.Unmarshal(buf, &payload)

	return payload, err
}

// DockerRepository represents the meta information of a downloadable docker
// repository. This is a tarball compressed using snappy-stream.
type DockerRepository struct {
	// Content is the compressed tarball. It is the caller's responsibility to
	// close Content.
	Content io.ReadCloser

	// Sha256 checksum of the compressed tarball.
	Sha256 string

	// Size of the compressed tarball.
	Size int64
}

// GetDockerRepository will retrieve a snappy-stream compressed tarball.
func (c *APIClient) GetDockerRepository(buildID string) (*DockerRepository, error) {
	model := make(map[string]interface{})
	model["buildId"] = buildID

	template := routes["GetDockerRepository"]
	url, err := template.Expand(model)
	if err != nil {
		return nil, err
	}

	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, c.parseError(res)
	}

	return &DockerRepository{
		Content: res.Body,
		Sha256:  res.Header.Get("x-amz-meta-Sha256"),
		Size:    res.ContentLength,
	}, nil
}

// addAuthToken adds the authentication token to the querystring if available.
// TODO(bvdberg): we should migrate to authentication header.
func (c *APIClient) addAuthToken(req *http.Request) {
	authToken := c.options.AuthToken

	if authToken != "" {
		q := req.URL.Query()
		q.Set("token", authToken)
		req.URL.RawQuery = q.Encode()
	}
}

// AddRequestHeaders add a few default headers to req. Currently added: User-
// Agent, X-Wercker-Version, X-Wercker-Git.
func AddRequestHeaders(req *http.Request) {
	userAgent := fmt.Sprintf("sentcli %s", FullVersion())

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Wercker-Version", Version())
	if GitCommit != "" {
		req.Header.Set("X-Wercker-Git", GitCommit)
	}
}

// APIError represents a wercker error.
type APIError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// Error returns the message and status code.
func (e *APIError) Error() string {
	return fmt.Sprintf("wercker-api: %s (status code: %d)", e.Message, e.StatusCode)
}

// parseError will check if res.Body contains a wercker generated error and
// return that, otherwise it will return a generic message based on statuscode.
func (c *APIClient) parseError(res *http.Response) error {
	// Check if the Body contains a wercker JSON error.
	if res.ContentLength > 0 {
		contentType := strings.Trim(res.Header.Get("Content-Type"), " ")

		if strings.HasPrefix(contentType, "application/json") {
			buf, err := ioutil.ReadAll(res.Body)
			if err != nil {
				goto generic
			}
			defer res.Body.Close()

			var payload *APIError
			err = json.Unmarshal(buf, &payload)
			if err == nil && payload.Message != "" && payload.StatusCode != 0 {
				return payload
			}
		}
	}

generic:
	var message string
	switch res.StatusCode {
	case 401:
		message = "authentication required"
	case 403:
		message = "not authorized to access this resource"
	case 404:
		message = "resource not found"
	default:
		message = "unknown error"
	}

	return &APIError{
		Message:    message,
		StatusCode: res.StatusCode,
	}
}
