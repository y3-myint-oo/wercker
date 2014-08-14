package main

import (
  "reflect"
  "strings"
  "github.com/fsouza/go-dockerclient"
)


type BoxConfig struct {
  Env map[string]string
  Name string
  Owner string
  Version string
}


type Box struct {
  Name string
  build *Build
  options *GlobalOptions
}


// Convert RawBox into a Box
func (b *RawBox) ToBox(build *Build, options *GlobalOptions) (*Box, error) {
  v := reflect.ValueOf(*b)
  return CreateBox(v.String(), build, options)
}


// CreateBox from a name and other references
func CreateBox(name string, build *Build, options *GlobalOptions) (*Box, error) {
  // TODO(termie): right now I am just tacking the version into the name
  //               by replacing @ with _
  name = strings.Replace(name, "@", "_", 1)
  return &Box{Name: name, build: build, options: options}, nil
}


// Fetch an image if we don't have it already
// TODO(termie): we don't actually fetch new ones yet!
func (b *Box) Fetch() (*docker.Image, error) {
  client, err := docker.NewClient(b.options.DockerEndpoint)
  if err != nil {
    panic(err)
  }

  image, err := client.InspectImage(b.Name)
  if err == nil {
    return image, nil
  }
  return nil, err
}
