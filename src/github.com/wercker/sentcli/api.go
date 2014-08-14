package main

import (
  "errors"
  "fmt"
  "io/ioutil"
  "net/http"
  "strings"
)


type ApiClient struct {
  endpoint string
}


func CreateApiClient(endpoint string) *ApiClient {
  return &ApiClient{endpoint:endpoint}
}

func (c *ApiClient) Url(parts ...string) string {
  allParts := append([]string{c.endpoint}, parts...)
  return strings.Join(allParts, "/")
}

func (c *ApiClient) Get(parts ...string) ([]byte, error) {
  url := c.Url(parts...)
  fmt.Println("GET URL", url)
  resp, err := http.Get(url)
  if err != nil {
    return nil, err
  }
  if resp.StatusCode != 200 {
    fmt.Println(ioutil.ReadAll(resp.Body))
    return nil, errors.New(fmt.Sprintf("Got non-200 response: %d", resp.StatusCode))
  }

  buf, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }
  return buf, nil
}
