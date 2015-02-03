package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// APIClient is a very dumb client for the wercker API
type APIClient struct {
	endpoint string
	client   *http.Client
}

// NewAPIClient returns our dumb client
func NewAPIClient(endpoint string) *APIClient {
	return &APIClient{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

// URL joins some strings to the endpoint
func (c *APIClient) URL(parts ...string) string {
	allParts := append([]string{c.endpoint}, parts...)
	return strings.Join(allParts, "/")
}

// Get is the basic fetch of the JSON
func (c *APIClient) Get(parts ...string) ([]byte, error) {
	url := c.URL(parts...)
	log.Debugln("API Get:", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to post request to wercker API")
		return nil, err
	}

	AddRequestHeaders(req)

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		log.Debugln(ioutil.ReadAll(res.Body))
		return nil, fmt.Errorf("Got non-200 response: %d", res.StatusCode)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return buf, nil
}
