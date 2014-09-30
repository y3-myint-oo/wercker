package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
)

// APIClient is a very dumb client for the wercker API
type APIClient struct {
	endpoint string
}

// CreateAPIClient returns our dumb client
func CreateAPIClient(endpoint string) *APIClient {
	return &APIClient{endpoint: endpoint}
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
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		log.Debugln(ioutil.ReadAll(resp.Body))
		return nil, fmt.Errorf("Got non-200 response: %d", resp.StatusCode)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
