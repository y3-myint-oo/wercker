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
	ShortName       string
	networkDisabled bool
	services        []*ServiceBox
	options         *PipelineOptions
	client          *DockerClient
	container       *docker.Container
	repository      string
	tag             string
	images          []*docker.Image
}

// BoxOptions are box options, duh
type BoxOptions struct {
	NetworkDisabled bool
}

// ToBox will convert a RawBox into a Box
func (b *RawBox) ToBox(options *PipelineOptions, boxOptions *BoxOptions) (*Box, error) {
	return NewBox(string(*b), options, boxOptions)
}

// NewBox from a name and other references
func NewBox(name string, options *PipelineOptions, boxOptions *BoxOptions) (*Box, error) {
	// TODO(termie): right now I am just tacking the version into the name
	//               by replacing @ with _
	name = strings.Replace(name, "@", "_", 1)
	parts := strings.Split(name, ":")
	repository := parts[0]
	tag := "latest"

	repoParts := strings.Split(repository, "/")
	shortName := repository
	if len(repoParts) > 1 {
		shortName = repoParts[len(repoParts)-1]
	}

	if len(parts) > 1 {
		tag = parts[1]
	}
	client, err := NewDockerClient(options.DockerOptions)
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
		ShortName:       shortName,
		options:         options,
		repository:      repository,
		tag:             tag,
		networkDisabled: networkDisabled,
	}, nil
}

func (b *Box) links() []string {
	serviceLinks := []string{}

	for _, service := range b.services {
		serviceLinks = append(serviceLinks, fmt.Sprintf("%s:%s", service.container.Name, service.ShortName))
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

			// For local dev we can mount read-write and avoid a copy, so we'll mount
			// directly in the pipeline path
			if b.options.DirectMount {
				binds = append(binds, fmt.Sprintf("%s:%s:rw", b.options.HostPath(entry.Name()), b.options.GuestPath(entry.Name())))
			} else {
				binds = append(binds, fmt.Sprintf("%s:%s:ro", b.options.HostPath(entry.Name()), b.options.MntPath(entry.Name())))
			}
			// volumes[b.options.MntPath(entry.Name())] = struct{}{}
		}
	}
	return binds, nil
}

// RunServices runs the services associated with this box
func (b *Box) RunServices() error {
	for _, serviceBox := range b.services {
		log.Debugln("Starting service:", serviceBox.Name)
		_, err := serviceBox.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// Run creates the container and runs it.
func (b *Box) Run() (*docker.Container, error) {
	log.Debugln("Starting base box:", b.Name)
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

// Clean up the containers
func (b *Box) Clean() error {
	containers := []string{}
	if b.container != nil {
		containers = append(containers, b.container.ID)
	}

	for _, service := range b.services {
		if service.container != nil {
			containers = append(containers, service.container.ID)
		}
	}

	for _, container := range containers {
		opts := docker.RemoveContainerOptions{
			ID: container,
			// God, if you exist, thank you for removing these containers,
			// that their biological and cultural diversity is not added
			// to our own but is expunged from us with fiery vengeance.
			RemoveVolumes: true,
			Force:         true,
		}
		err := b.client.RemoveContainer(opts)
		if err != nil {
			return err
		}
	}

	for i := len(b.images) - 1; i >= 0; i-- {
		b.client.RemoveImage(b.images[i].ID)
	}

	return nil
}

// Restart stops and starts the box
func (b *Box) Restart() (*docker.Container, error) {
	err := b.client.RestartContainer(b.container.ID, 1)
	if err != nil {
		return nil, err
	}
	return b.container, nil
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
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				log.Warnln("Service container has already stopped.")
			} else {
				log.WithField("Error", err).Warnln("Wasn't able to stop service container", service.Box.container.ID)
			}
		}
	}
	if b.container != nil {
		log.Println("Stopping container", b.container.ID)
		err := b.client.StopContainer(b.container.ID, 1)

		if err != nil {
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				log.Warnln("Box container has already stopped.")
			} else {
				log.WithField("Error", err).Warnln("Wasn't able to stop box container", b.container.ID)
			}
		}
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
	go emitStatus(r, b.options)

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

	b.images = append(b.images, image)

	return image, err
}

// Push commits and tag a container. Then push the image to the registry.
// Returns the new image, no cleanup is provided.
func (b *Box) Push(options *PushOptions, auth docker.AuthConfiguration) (*docker.Image, error) {
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
	go emitStatus(r, b.options)

	pushOptions := docker.PushImageOptions{
		Name:          imageName,
		OutputStream:  w,
		RawJSONStream: true,
		Registry:      options.Registry,
		Tag:           options.Tag,
	}

	err = b.client.PushImage(pushOptions, auth)
	if err != nil {
		return nil, err
	}
	log.WithField("Image", i).Debug("Commit completed")

	// Cleanup the go routine by closing the writer.
	w.Close()

	return i, nil
}

// ExportImageOptions are the options available for ExportImage.
type ExportImageOptions struct {
	Name         string
	OutputStream io.Writer
}

// ExportImage will export the image to a temporary file and return the path to
// the file.
func (b *Box) ExportImage(options *ExportImageOptions) error {
	logger := log.WithField("Name", options.Name)
	logger.Info("Storing image")

	exportImageOptions := docker.ExportImageOptions{
		Name:         options.Name,
		OutputStream: options.OutputStream,
	}

	return b.client.ExportImage(exportImageOptions)
}

// emitStatus will decode the messages coming from r and decode these into
// JSONMessage
func emitStatus(r io.Reader, options *PipelineOptions) {
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
			Options: options,
			Logs:    line,
			Stream:  "docker",
			Hidden:  false,
		})
	}
}
