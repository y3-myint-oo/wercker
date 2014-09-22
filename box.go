package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"strings"
)

// Box is our wrapper for Box operations
type Box struct {
	Name      string
	services  []*ServiceBox
	build     *Build
	options   *GlobalOptions
	client    *docker.Client
	container *docker.Container
}

// ToBox will convert a RawBox into a Box
func (b *RawBox) ToBox(build *Build, options *GlobalOptions) (*Box, error) {
	return CreateBox(string(*b), build, options)
}

// CreateBox from a name and other references
func CreateBox(name string, build *Build, options *GlobalOptions) (*Box, error) {
	// TODO(termie): right now I am just tacking the version into the name
	//               by replacing @ with _
	name = strings.Replace(name, "@", "_", 1)

	client, err := docker.NewClient(options.DockerEndpoint)
	if err != nil {
		return nil, err
	}
	return &Box{client: client, Name: name, build: build, options: options}, nil
}

func (b *Box) links() []string {
	serviceLinks := []string{}

	for _, service := range b.services {
		serviceLinks = append(serviceLinks, fmt.Sprintf("%s:%s", service.container.Name, service.Name))
	}
	log.Println("Creating links: ", serviceLinks)
	return serviceLinks
}

func (b *Box) binds() ([]string, error) {
	binds := []string{}
	// Make our list of binds for the Docker attach
	// NOTE(termie): we don't appear to need the "volumes" stuff, leaving
	//               it commented out in case it actually does something
	// volumes := make(map[string]struct{})
	entries, err := ioutil.ReadDir(b.build.HostPath())
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", b.build.HostPath(entry.Name()), b.build.MntPath(entry.Name())))
			// volumes[build.MntPath(entry.Name())] = struct{}{}
		}
	}
	return binds, nil
}

// Creates the container and runs it.
func (b *Box) Run() (*docker.Container, error) {
	// Make and start the container
	containerName := "wercker-build-" + b.options.BuildID
	container, err := b.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:        b.Name,
				Tty:          false,
				OpenStdin:    true,
				Cmd:          []string{"/bin/bash"},
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				// Volumes: volumes,
			},
		})
	if err != nil {
		return nil, err
	}

	log.Println("Docker Container:", container.ID)

	binds, err := b.binds()
	if err != nil {
		return nil, err
	}

	b.client.StartContainer(container.ID, &docker.HostConfig{
		Binds: binds,
		Links: b.links(),
	})
	b.container = container
	return container, nil
}

// Add a service needed by this Box
func (b *Box) AddService(service *ServiceBox) {
	b.services = append(b.services, service)
}

// Stop the box and all its services
func (b *Box) Stop() {
	for _, service := range b.services {
		log.Println("Stopping service ", service.Box.container.ID)
		err := b.client.StopContainer(service.Box.container.ID, 1)
		if err != nil {
			log.Println("Wasn't able to stop service container", service.Box.container.ID)
		}
	}
	log.Println("Stopping container ", b.container.ID)
	err := b.client.StopContainer(b.container.ID, 1)
	if err != nil {
		log.Println("Wasn't able to stop box container", b.container.ID)
	}
}

// Fetch an image if we don't have it already
func (b *Box) Fetch() (*docker.Image, error) {
	if image, err := b.client.InspectImage(b.Name); err == nil {
		return image, nil
	}

	log.Println("Couldn't find image locally, fetching.")

	options := docker.PullImageOptions{
		Repository: b.Name,
		// changeme if we have a private registry
		//Registry:     "docker.tsuru.io",
		Tag: "latest",
	}

	err := b.client.PullImage(options, docker.AuthConfiguration{})
	if err == nil {
		image, err := b.client.InspectImage(b.Name)
		if err == nil {
			return image, nil
		}
	}

	return nil, err
}
