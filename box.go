package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

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
	container       *docker.Container
	config          *BoxConfig
	cmd             string
	repository      string
	tag             string
	images          []*docker.Image
	logger          *LogEntry
}

// BoxOptions are box options, duh
type BoxOptions struct {
	NetworkDisabled bool
}

// ToBox will convert a BoxConfig into a Box
func (b *BoxConfig) ToBox(options *PipelineOptions, boxOptions *BoxOptions) (*Box, error) {
	return NewBox(b, options, boxOptions)
}

// NewBox from a name and other references
func NewBox(boxConfig *BoxConfig, options *PipelineOptions, boxOptions *BoxOptions) (*Box, error) {
	name := boxConfig.ID

	if strings.Contains(name, "@") {
		return nil, fmt.Errorf("Invalid box name, '@' is not allowed in docker repositories.")
	}

	parts := strings.Split(name, ":")
	repository := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}
	if boxConfig.Tag != "" {
		tag = boxConfig.Tag
	}
	name = fmt.Sprintf("%s:%s", repository, tag)

	repoParts := strings.Split(repository, "/")
	shortName := repository
	if len(repoParts) > 1 {
		shortName = repoParts[len(repoParts)-1]
	}

	networkDisabled := false
	if boxOptions != nil {
		networkDisabled = boxOptions.NetworkDisabled
	}

	cmd := boxConfig.Cmd
	if cmd == "" {
		cmd = "/bin/bash"
	}

	logger := rootLogger.WithFields(LogFields{
		"Logger":    "Box",
		"Name":      name,
		"ShortName": shortName,
	})

	return &Box{
		Name:            name,
		ShortName:       shortName,
		config:          boxConfig,
		options:         options,
		repository:      repository,
		tag:             tag,
		networkDisabled: networkDisabled,
		logger:          logger,
		cmd:             cmd,
	}, nil
}

func (b *Box) links() []string {
	serviceLinks := []string{}

	for _, service := range b.services {
		serviceLinks = append(serviceLinks, fmt.Sprintf("%s:%s", service.container.Name, service.ShortName))
	}
	b.logger.Debugln("Creating links:", serviceLinks)
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
		if entry.IsDir() || entry.Mode()&os.ModeSymlink == os.ModeSymlink {

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
func (b *Box) RunServices(env *Environment) error {
	for _, serviceBox := range b.services {
		b.logger.Debugln("Starting service:", serviceBox.Name)
		_, err := serviceBox.Run(env)
		if err != nil {
			return err
		}
	}
	return nil
}

func dockerEnv(boxEnv map[string]string, env *Environment) []string {
	s := []string{}
	for k, v := range boxEnv {
		s = append(s, fmt.Sprintf("%s=%s", strings.ToUpper(k), env.Interpolate(v)))
	}
	return s
}

func portBindings(published []string) map[docker.Port][]docker.PortBinding {
	outer := make(map[docker.Port][]docker.PortBinding)
	for _, portdef := range published {
		var ip string
		var hostPort string
		var containerPort string

		parts := strings.Split(portdef, ":")

		switch {
		case len(parts) == 3:
			ip = parts[0]
			hostPort = parts[1]
			containerPort = parts[2]
		case len(parts) == 2:
			hostPort = parts[0]
			containerPort = parts[1]
		case len(parts) == 1:
			hostPort = parts[0]
			containerPort = parts[0]
		}
		// Make sure we have a protocol in the container port
		if !strings.Contains(containerPort, "/") {
			containerPort = containerPort + "/tcp"
		}

		if hostPort == "" {
			hostPort = containerPort
		}

		// Just in case we have a /tcp in there
		hostParts := strings.Split(hostPort, "/")
		hostPort = hostParts[0]
		portBinding := docker.PortBinding{
			HostPort: hostPort,
		}
		if ip != "" {
			portBinding.HostIP = ip
		}
		outer[docker.Port(containerPort)] = []docker.PortBinding{portBinding}
	}
	return outer
}

func exposedPorts(published []string) map[docker.Port]struct{} {
	portBinds := portBindings(published)
	exposed := make(map[docker.Port]struct{})
	for port := range portBinds {
		exposed[port] = struct{}{}
	}
	return exposed
}

// ExposedPortMap contains port forwarding information
type ExposedPortMap struct {
	ContainerPort string
	HostURI       string
}

// exposedPortMaps returns a list of exposed ports and the host
func exposedPortMaps(published []string) ([]ExposedPortMap, error) {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost != "" {
		docker, err := url.Parse(dockerHost)
		if err != nil {
			return nil, err
		}
		if docker.Scheme == "unix" {
			dockerHost = "localhost"
		} else {
			dockerHost = strings.Split(docker.Host, ":")[0]
		}
	}
	portMap := []ExposedPortMap{}
	for k, v := range portBindings(published) {
		for _, port := range v {
			p := ExposedPortMap{
				ContainerPort: k.Port(),
				HostURI:       fmt.Sprintf("%s:%s", dockerHost, port.HostPort),
			}
			portMap = append(portMap, p)
		}
	}
	return portMap, nil
}

//RecoverInteractive restarts the box with a terminal attached
func (b *Box) RecoverInteractive(cwd string, pipeline Pipeline, step Step) error {
	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil
	}

	container, err := b.Restart()
	if err != nil {
		b.logger.Panicln("box restart failed")
		return err
	}

	env := []string{}
	env = append(env, pipeline.Env().Export()...)
	env = append(env, pipeline.Env().Hidden.Export()...)
	env = append(env, step.Env().Export()...)
	env = append(env, fmt.Sprintf("cd %s", cwd))
	cmd := []string{b.cmd}
	return client.AttachInteractive(container.ID, cmd, env)
}

// Run creates the container and runs it.
func (b *Box) Run(env *Environment) (*docker.Container, error) {
	err := b.RunServices(env)
	if err != nil {
		return nil, err
	}
	b.logger.Debugln("Starting base box:", b.Name)

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}

	// Import the environment
	myEnv := dockerEnv(b.config.Env, env)

	// Make and start the container
	containerName := "wercker-pipeline-" + b.options.PipelineID
	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:           b.Name,
				Tty:             false,
				OpenStdin:       true,
				Cmd:             []string{b.cmd},
				Env:             myEnv,
				AttachStdin:     true,
				AttachStdout:    true,
				AttachStderr:    true,
				ExposedPorts:    exposedPorts(b.options.PublishPorts),
				NetworkDisabled: b.networkDisabled,
				DNS:             b.options.DockerDNS,
				// Volumes: volumes,
			},
		})
	if err != nil {
		return nil, err
	}

	b.logger.Debugln("Docker Container:", container.ID)

	binds, err := b.binds()
	if err != nil {
		return nil, err
	}

	client.StartContainer(container.ID, &docker.HostConfig{
		Binds:        binds,
		Links:        b.links(),
		PortBindings: portBindings(b.options.PublishPorts),
		DNS:          b.options.DockerDNS,
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

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return err
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
		b.logger.WithField("Container", container).Debugln("Removing container:", container)
		err := client.RemoveContainer(opts)
		if err != nil {
			return err
		}
	}

	if !b.options.ShouldCommit {
		for i := len(b.images) - 1; i >= 0; i-- {
			b.logger.WithField("Image", b.images[i].ID).Debugln("Removing image:", b.images[i].ID)
			client.RemoveImage(b.images[i].ID)
		}
	}

	return nil
}

// Restart stops and starts the box
func (b *Box) Restart() (*docker.Container, error) {
	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}
	err = client.RestartContainer(b.container.ID, 1)
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
	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return
	}
	for _, service := range b.services {
		b.logger.Debugln("Stopping service", service.Box.container.ID)
		err := client.StopContainer(service.Box.container.ID, 1)

		if err != nil {
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				b.logger.Warnln("Service container has already stopped.")
			} else {
				b.logger.WithField("Error", err).Warnln("Wasn't able to stop service container", service.Box.container.ID)
			}
		}
	}
	if b.container != nil {
		b.logger.Debugln("Stopping container", b.container.ID)
		err := client.StopContainer(b.container.ID, 1)

		if err != nil {
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				b.logger.Warnln("Box container has already stopped.")
			} else {
				b.logger.WithField("Error", err).Warnln("Wasn't able to stop box container", b.container.ID)
			}
		}
	}
}

// Fetch an image (or update the local)
func (b *Box) Fetch(env *Environment) (*docker.Image, error) {
	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}

	// Check for access to this image
	auth := docker.AuthConfiguration{
		Username: env.Interpolate(b.config.Username),
		Password: env.Interpolate(b.config.Password),
		// Email:         s.email,
		// ServerAddress: s.authServer,
	}

	checkOpts := CheckAccessOptions{
		Auth:       auth,
		Access:     "read",
		Repository: env.Interpolate(b.repository),
		Tag:        env.Interpolate(b.tag),
		Registry:   env.Interpolate(b.config.Registry),
	}

	check, err := client.CheckAccess(checkOpts)
	if err != nil {
		b.logger.Errorln("Error during check access")
		return nil, err
	}
	if !check {
		b.logger.Errorln("Not allowed to interact with this repository:", b.repository)
		return nil, fmt.Errorf("Not allowed to interact with this repository: %s", b.repository)
	}

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()

	// emitStatusses in a different go routine
	go emitStatus(r, b.options)

	options := docker.PullImageOptions{
		// changeme if we have a private registry
		// Registry:      "docker.tsuru.io",
		OutputStream:  w,
		RawJSONStream: true,
		Repository:    env.Interpolate(b.repository),
		Tag:           env.Interpolate(b.tag),
	}

	err = client.PullImage(options, auth)
	if err == nil {
		image, err := client.InspectImage(b.Name)
		if err == nil {
			return image, nil
		}
	}

	// Cleanup the go routine by closing the writer.
	w.Close()

	return nil, err
}

// Commit the current running Docker container to an Docker image.
func (b *Box) Commit(name, tag, message string) (*docker.Image, error) {
	b.logger.WithFields(LogFields{
		"Name": name,
		"Tag":  tag,
	}).Debugln("Commit container:", name, tag)

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}

	commitOptions := docker.CommitContainerOptions{
		Container:  b.container.ID,
		Repository: name,
		Tag:        tag,
		Message:    "Build completed",
		Author:     "wercker",
	}
	image, err := client.CommitContainer(commitOptions)

	b.images = append(b.images, image)

	return image, err
}

// ExportImageOptions are the options available for ExportImage.
type ExportImageOptions struct {
	Name         string
	OutputStream io.Writer
}

// ExportImage will export the image to a temporary file and return the path to
// the file.
func (b *Box) ExportImage(options *ExportImageOptions) error {
	b.logger.WithField("ExportName", options.Name).Info("Storing image")

	exportImageOptions := docker.ExportImageOptions{
		Name:         options.Name,
		OutputStream: options.OutputStream,
	}

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return err
	}

	return client.ExportImage(exportImageOptions)
}

// emitStatus will decode the messages coming from r and decode these into
// JSONMessage
func emitStatus(r io.Reader, options *PipelineOptions) {
	e := GetGlobalEmitter()

	s := NewJSONMessageProcessor()
	dec := json.NewDecoder(r)
	for {
		var m utils.JSONMessage
		if err := dec.Decode(&m); err == io.EOF {
			// Once the EOF is reached the function will stop
			break
		} else if err != nil {
			rootLogger.Panic(err)
		}

		line := s.ProcessJSONMessage(&m)
		e.Emit(Logs, &LogsArgs{
			Logs:   line,
			Stream: "docker",
		})
	}
}
