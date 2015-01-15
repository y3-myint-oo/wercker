package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
	"github.com/fsouza/go-dockerclient"
)

// Box is our wrapper for Box operations
type Box struct {
	Name            string
	networkDisabled bool
	services        []*ServiceBox
	options         *GlobalOptions
	client          *docker.Client
	container       *docker.Container
	repository      string
	tag             string
}

// BoxOptions are box options, duh
type BoxOptions struct {
	NetworkDisabled bool
}

// ToBox will convert a RawBox into a Box
func (b *RawBox) ToBox(options *GlobalOptions, boxOptions *BoxOptions) (*Box, error) {
	return NewBox(string(*b), options, boxOptions)
}

// NewBox from a name and other references
func NewBox(name string, options *GlobalOptions, boxOptions *BoxOptions) (*Box, error) {
	// TODO(termie): right now I am just tacking the version into the name
	//               by replacing @ with _
	name = strings.Replace(name, "@", "_", 1)
	parts := strings.Split(name, ":")
	repository := parts[0]
	tag := "latest"

	if len(parts) > 1 {
		tag = parts[1]
	}

	client, err := docker.NewClient(options.DockerHost)
	if err != nil {
		return nil, err
	}

	networkDisabled := false
	if boxOptions != nil {
		networkDisabled = boxOptions.NetworkDisabled
	}

	return &Box{
		client:          client,
		Name:            name,
		options:         options,
		repository:      repository,
		tag:             tag,
		networkDisabled: networkDisabled,
	}, nil
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
	entries, err := ioutil.ReadDir(b.options.HostPath())
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", b.options.HostPath(entry.Name()), b.options.MntPath(entry.Name())))
			// volumes[b.options.MntPath(entry.Name())] = struct{}{}
		}
	}
	return binds, nil
}

// RunServices runs the services associated with this box
func (b *Box) RunServices() error {
	for _, serviceBox := range b.services {
		_, err := serviceBox.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// Run creates the container and runs it.
func (b *Box) Run() (*docker.Container, error) {
	// Make and start the container
	containerName := "wercker-pipeline-" + b.options.PipelineID
	container, err := b.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:           b.Name,
				Tty:             false,
				OpenStdin:       true,
				Cmd:             []string{"/bin/bash"},
				AttachStdin:     true,
				AttachStdout:    true,
				AttachStderr:    true,
				NetworkDisabled: b.networkDisabled,
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

// AddService needed by this Box
func (b *Box) AddService(service *ServiceBox) {
	b.services = append(b.services, service)
}

// Stop the box and all its services
func (b *Box) Stop() {
	for _, service := range b.services {
		log.Println("Stopping service", service.Box.container.ID)
		err := b.client.StopContainer(service.Box.container.ID, 1)
		if err != nil {
			log.Errorln("Wasn't able to stop service container", service.Box.container.ID)
		}
	}
	log.Println("Stopping container", b.container.ID)
	err := b.client.StopContainer(b.container.ID, 1)
	if err != nil {
		log.Errorln("Wasn't able to stop box container", b.container.ID)
	}
}

// Fetch an image if we don't have it already
func (b *Box) Fetch() (*docker.Image, error) {
	if image, err := b.client.InspectImage(b.Name); err == nil {
		return image, nil
	}

	log.Println("Couldn't find image locally, fetching.")

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()

	// emitStatusses in a different go routine
	go emitStatus(r)

	options := docker.PullImageOptions{
		// changeme if we have a private registry
		// Registry:      "docker.tsuru.io",
		OutputStream:  w,
		RawJSONStream: true,
		Repository:    b.repository,
		Tag:           b.tag,
	}

	err := b.client.PullImage(options, docker.AuthConfiguration{})
	if err == nil {
		image, err := b.client.InspectImage(b.Name)
		if err == nil {
			return image, nil
		}
	}

	// Cleanup the go routine by closing the writer.
	w.Close()

	return nil, err
}

// PushOptions configures what we push to a registry
type PushOptions struct {
	Registry string
	Name     string
	Tag      string
	Message  string
}

// Commit the current running Docker container to an Docker image.
func (b *Box) Commit(name, tag, message string) (*docker.Image, error) {
	log.WithFields(log.Fields{
		"Name": name,
		"Tag":  tag,
	}).Debug("Commit container")

	commitOptions := docker.CommitContainerOptions{
		Container:  b.container.ID,
		Repository: name,
		Tag:        tag,
		Message:    "Build completed",
		Author:     "wercker",
	}
	image, err := b.client.CommitContainer(commitOptions)
	return image, err
}

// Push commits and tag a container. Then push the image to the registry.
// Returns the new image, no cleanup is provided.
func (b *Box) Push(options *PushOptions) (*docker.Image, error) {
	log.WithFields(log.Fields{
		"Registry": options.Registry,
		"Name":     options.Name,
		"Tag":      options.Tag,
		"Message":  options.Message,
	}).Debug("Push to registry")

	imageName := options.Name
	if options.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", options.Registry, options.Name)
	}

	i, err := b.Commit(imageName, options.Tag, options.Message)
	if err != nil {
		return nil, err
	}

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()

	// emitStatusses in a different go routine
	go emitStatus(r)

	pushOptions := docker.PushImageOptions{
		Name:          imageName,
		OutputStream:  w,
		RawJSONStream: true,
		Registry:      options.Registry,
		Tag:           options.Tag,
	}
	auth := docker.AuthConfiguration{}

	err = b.client.PushImage(pushOptions, auth)
	if err != nil {
		return nil, err
	}
	log.WithField("Image", i).Debug("Commit completed")

	// Cleanup the go routine by closing the writer.
	w.Close()

	return i, nil
}

// emitStatus will decode the messages coming from r and decode these into
// JSONMessage
func emitStatus(r io.Reader) {
	e := GetEmitter()

	s := NewJSONMessageProcessor()
	dec := json.NewDecoder(r)
	for {
		var m utils.JSONMessage
		if err := dec.Decode(&m); err == io.EOF {
			// Once the EOF is reached the function will stop
			break
		} else if err != nil {
			log.Panic(err)
		}

		line := s.ProcessJSONMessage(&m)
		e.Emit(Logs, &LogsArgs{
			Logs:   line,
			Stream: "docker",
			Hidden: false,
		})
	}
}
