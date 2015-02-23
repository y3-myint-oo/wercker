package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

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
