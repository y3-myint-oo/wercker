package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"reflect"
	"strings"
)

// BoxConfig is the content of a wercker-box.yml
type BoxConfig struct {
	Env     map[string]string
	Name    string
	Owner   string
	Version string
}

// Box is our wrapper for Box operations
type Box struct {
	Name    string
	build   *Build
	options *GlobalOptions
}

// ToBox will convert a RawBox into a Box
func (b *RawBox) ToBox(build *Build, options *GlobalOptions) *Box {
	v := reflect.ValueOf(*b)
	return CreateBox(v.String(), build, options)
}

// CreateBox from a name and other references
func CreateBox(name string, build *Build, options *GlobalOptions) *Box {
	// TODO(termie): right now I am just tacking the version into the name
	//               by replacing @ with _
	name = strings.Replace(name, "@", "_", 1)
	return &Box{Name: name, build: build, options: options}
}

// Fetch an image if we don't have it already
func (b *Box) Fetch() (*docker.Image, error) {
	client, err := docker.NewClient(b.options.DockerEndpoint)
	if err != nil {
		return nil, err
	}

	if image, err := client.InspectImage(b.Name); err == nil {
		return image, nil
	}

	log.Println("couldn't find image locally, fetching.")

	options := docker.PullImageOptions{
		Repository: b.Name,
		// changeme if we have a private registry
		//Registry:     "docker.tsuru.io",
		Tag: "latest",
	}

	if err = client.PullImage(options, docker.AuthConfiguration{}); err == nil {
		image, err := client.InspectImage(b.Name)
		if err == nil {
			return image, nil
		}
	}

	return nil, err
}
