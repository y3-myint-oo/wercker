//   Copyright © 2016, 2018, Oracle and/or its affiliates.  All rights reserved.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package dockerlocal

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"

	"golang.org/x/net/context"
)

// TODO(termie): remove references to docker

// Box is our wrapper for Box operations
type DockerBox struct {
	Name                 string
	ShortName            string
	networkDisabled      bool
	fsouzaClient         *DockerClient
	officialDockerClient *OfficialDockerClient
	services             []core.ServiceBox
	options              *core.PipelineOptions
	dockerOptions        *Options
	containerID          string
	containerName        string
	config               *core.BoxConfig
	cmd                  string
	repository           string
	tag                  string
	images               []*docker.Image
	logger               *util.LogEntry
	entrypoint           string
	image                *types.ImageInspect
	volumes              []string
}

// NewDockerBox from a name and other references
func NewDockerBox(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerBox, error) {
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
	// checkpoint support
	if options.Checkpoint != "" {
		tag = fmt.Sprintf("w-%s", options.Checkpoint)
	}
	name = fmt.Sprintf("%s:%s", repository, tag)

	repoParts := strings.Split(repository, "/")
	shortName := repository
	if len(repoParts) > 1 {
		shortName = repoParts[len(repoParts)-1]
	}

	networkDisabled := false

	cmd := boxConfig.Cmd
	if cmd == "" {
		cmd = DefaultDockerCommand
	}

	entrypoint := boxConfig.Entrypoint

	logger := util.RootLogger().WithFields(util.LogFields{
		"Logger":    "Box",
		"Name":      name,
		"ShortName": shortName,
	})

	fsouzaClient, err := NewDockerClient(dockerOptions)
	if err != nil {
		return nil, err
	}

	officialDockerClient, err := NewOfficialDockerClient(dockerOptions)
	if err != nil {
		return nil, err
	}

	return &DockerBox{
		Name:                 name,
		ShortName:            shortName,
		fsouzaClient:         fsouzaClient,
		officialDockerClient: officialDockerClient,
		config:               boxConfig,
		options:              options,
		dockerOptions:        dockerOptions,
		repository:           repository,
		tag:                  tag,
		networkDisabled:      networkDisabled,
		logger:               logger,
		cmd:                  cmd,
		entrypoint:           entrypoint,
		volumes:              []string{},
	}, nil
}

func (b *DockerBox) links() []string {
	serviceLinks := []string{}

	for _, service := range b.services {
		serviceLinks = append(serviceLinks, service.Link())
	}
	b.logger.Debugln("Creating links:", serviceLinks)
	return serviceLinks
}

// Link gives us the parameter to Docker to link to this box
func (b *DockerBox) Link() string {
	name := b.config.Name
	if name == "" {
		name = b.ShortName
	}
	return fmt.Sprintf("%s:%s", b.containerName, name)
}

// GetName gets the box name
func (b *DockerBox) GetName() string {
	return b.Name
}

func (b *DockerBox) Repository() string {
	return b.repository
}

func (b *DockerBox) GetTag() string {
	return b.tag
}

// GetID gets the container ID or empty string if we don't have a container
func (b *DockerBox) GetID() string {
	if b.containerID != "" {
		return b.containerID
	}
	return ""
}

func (b *DockerBox) binds(env *util.Environment) ([]string, error) {
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

	if b.options.EnableVolumes {
		vols := util.SplitSpaceOrComma(b.config.Volumes)
		var interpolatedVols []string
		for _, vol := range vols {
			if strings.Contains(vol, ":") {
				pair := strings.SplitN(vol, ":", 2)
				interpolatedVols = append(interpolatedVols, env.Interpolate(pair[0]))
				interpolatedVols = append(interpolatedVols, env.Interpolate(pair[1]))
			} else {
				interpolatedVols = append(interpolatedVols, env.Interpolate(vol))
				interpolatedVols = append(interpolatedVols, env.Interpolate(vol))
			}
		}
		b.volumes = interpolatedVols
		for i := 0; i < len(b.volumes); i += 2 {
			binds = append(binds, fmt.Sprintf("%s:%s:rw", b.volumes[i], b.volumes[i+1]))
		}
	}

	return binds, nil
}

// RunServices runs the services associated with this box
func (b *DockerBox) RunServices(ctx context.Context, env *util.Environment) error {
	links := []string{}

	// TODO(termie): terrible hack, sorry world
	ctxWithServiceCount := context.WithValue(ctx, "ServiceCount", len(b.services))

	for _, service := range b.services {
		b.logger.Debugln("Startinq service:", service.GetName())
		_, err := service.Run(ctxWithServiceCount, env, links)
		if err != nil {
			return err
		}
		links = append(links, service.Link())
	}
	return nil
}

func dockerEnv(boxEnv map[string]string, env *util.Environment) []string {
	s := []string{}
	for k, v := range boxEnv {
		s = append(s, fmt.Sprintf("%s=%s", strings.ToUpper(k), env.Interpolate(v)))
	}
	return s
}

func portBindings(published []string) (nat.PortMap, error) {
	outer := make(nat.PortMap)
	for _, portdef := range published {
		var ip string
		var hostPort string
		var containerPort string

		// split the supplied string into ip (if supplied), host port and container port (if different)
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

		// split the container port into port number and prototol to construct a nat.Port
		var containerPortNumber string
		var containerPortProto string
		containerPortParts := strings.Split(containerPort, "/")
		switch {
		case len(containerPortParts) == 1:
			containerPortNumber = containerPortParts[0]
			containerPortProto = "tcp"
		case len(containerPortParts) == 2:
			containerPortNumber = containerPortParts[0]
			containerPortProto = containerPortParts[1]
		default:
			return nil, fmt.Errorf("Invalid container port specification: %s", containerPort)
		}
		natContainerPort, err := nat.NewPort(containerPortProto, containerPortNumber)
		if err != nil {
			return nil, err
		}

		// handle the host port to create a nat.PortBinding
		var hostPortNumber string
		if hostPort == "" {
			hostPortNumber = containerPortNumber
		} else {
			// Just in case we have a /tcp in there
			hostParts := strings.Split(hostPort, "/")
			hostPortNumber = hostParts[0]
		}
		portBinding := nat.PortBinding{
			HostPort: hostPortNumber,
		}
		if ip != "" {
			portBinding.HostIP = ip
		}
		outer[natContainerPort] = []nat.PortBinding{portBinding}
	}
	return outer, nil
}

func exposedPorts(published []string) (nat.PortSet, error) {
	portBinds, err := portBindings(published)
	if err != nil {
		return nil, err
	}
	exposed := make(nat.PortSet)
	for port := range portBinds {
		exposed[port] = struct{}{}
	}
	return exposed, nil
}

// ExposedPortMap contains port forwarding information
type ExposedPortMap struct {
	ContainerPort string
	HostURI       string
}

// exposedPortMaps returns a list of exposed ports and the host
func exposedPortMaps(dockerHost string, published []string) ([]ExposedPortMap, error) {
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
	portBindings, err := portBindings(published)
	if err != nil {
		return nil, err
	}

	for k, v := range portBindings {
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
func (b *DockerBox) RecoverInteractive(ctx context.Context, cwd string, pipeline core.Pipeline, step core.Step) error {
	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient
	containerID, err := b.Restart()
	if err != nil {
		b.logger.Panicln("box restart failed")
		return err
	}

	env := []string{}
	env = append(env, pipeline.Env().Export()...)
	env = append(env, pipeline.Env().Hidden.Export()...)
	env = append(env, step.Env().Export()...)
	env = append(env, fmt.Sprintf("cd %s", cwd))
	cmd, err := shlex.Split(b.cmd)
	if err != nil {
		return err
	}
	return fsouzaClient.AttachInteractive(containerID, cmd, env)
}

func (b *DockerBox) getContainerName() string {
	return "wercker-pipeline-" + b.options.RunID
}

// Run creates the container and runs it.
func (b *DockerBox) Run(ctx context.Context, env *util.Environment) (string, error) {
	err := b.RunServices(ctx, env)
	if err != nil {
		return "", err
	}
	b.logger.Debugln("Starting base box:", b.Name)

	// TODO(termie): maybe move the container manipulation outside of here?
	officialDockerClient := b.officialDockerClient

	// Import the environment
	myEnv := dockerEnv(b.config.Env, env)

	var entrypoint []string
	if b.entrypoint != "" {
		entrypoint, err = shlex.Split(b.entrypoint)
		if err != nil {
			return "", err
		}
	}

	cmd, err := shlex.Split(b.cmd)
	if err != nil {
		return "", err
	}

	var ports nat.PortSet
	if len(b.options.PublishPorts) > 0 {
		ports, err = exposedPorts(b.options.PublishPorts)
	} else if b.options.ExposePorts {
		ports, err = exposedPorts(b.config.Ports)
	}
	if err != nil {
		return "", err
	}

	binds, err := b.binds(env)

	portsToBind := []string{""}

	if len(b.options.PublishPorts) >= 1 {
		b.logger.Warnln("--publish is deprecated, please use --expose-ports and define the ports for the boxes. See: https://github.com/wercker/wercker/pull/161")
		portsToBind = b.options.PublishPorts
	} else if b.options.ExposePorts {
		portsToBind = b.config.Ports
	}

	portBindings, err := portBindings(portsToBind)
	if err != nil {
		return "", err
	}

	hostConfig := &container.HostConfig{
		Binds:        binds,
		Links:        b.links(),
		PortBindings: portBindings,
		DNS:          b.dockerOptions.DNS,
	}

	config := &container.Config{
		Image:           env.Interpolate(b.Name),
		Tty:             false,
		OpenStdin:       true,
		Cmd:             cmd,
		Env:             myEnv,
		AttachStdin:     true,
		AttachStdout:    true,
		AttachStderr:    true,
		ExposedPorts:    ports,
		NetworkDisabled: b.networkDisabled,
		Entrypoint:      entrypoint,
	}

	networkingConfig := &network.NetworkingConfig{}

	if b.dockerOptions.Memory != 0 {
		mem := b.dockerOptions.Memory
		if len(b.services) > 0 {
			mem = int64(float64(mem) * 0.75)
		}
		swap := b.dockerOptions.MemorySwap
		if swap == 0 {
			swap = 2 * mem
		}
		hostConfig.Resources = container.Resources{
			Memory:     mem,
			MemorySwap: swap,
		}
	}

	// Make and start the container
	containerCreateCreatedBody, err := officialDockerClient.ContainerCreate(ctx, config, hostConfig, networkingConfig, b.getContainerName())
	if err != nil {
		return "", err
	}

	b.logger.Debugln("Docker Container:", containerCreateCreatedBody.ID)

	err = officialDockerClient.ContainerStart(ctx, containerCreateCreatedBody.ID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	b.containerID = containerCreateCreatedBody.ID
	b.containerName = b.getContainerName()
	return containerCreateCreatedBody.ID, nil
}

// Clean up the containers
func (b *DockerBox) Clean() error {
	containers := []string{}
	if b.containerID != "" {
		containers = append(containers, b.containerID)
	}

	for _, service := range b.services {
		if containerID := service.GetID(); containerID != "" {
			containers = append(containers, containerID)
		}
	}

	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient

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
		err := fsouzaClient.RemoveContainer(opts)
		if err != nil {
			return err
		}
	}

	if !b.options.ShouldCommit {
		for i := len(b.images) - 1; i >= 0; i-- {
			b.logger.WithField("Image", b.images[i].ID).Debugln("Removing image:", b.images[i].ID)
			fsouzaClient.RemoveImage(b.images[i].ID)
		}
	}

	return nil
}

// Restart stops and starts the box
// returns the container ID
func (b *DockerBox) Restart() (string, error) {
	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient
	err := fsouzaClient.RestartContainer(b.containerID, 1)
	if err != nil {
		return "", err
	}
	return b.containerID, nil
}

// AddService needed by this Box
func (b *DockerBox) AddService(service core.ServiceBox) {
	b.services = append(b.services, service)
}

// Stop the box and all its services
func (b *DockerBox) Stop() {
	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient
	for _, service := range b.services {
		b.logger.Debugln("Stopping service", service.GetID())
		err := fsouzaClient.StopContainer(service.GetID(), 1)

		if err != nil {
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				b.logger.Warnln("Service container has already stopped.")
			} else {
				b.logger.WithField("Error", err).Warnln("Wasn't able to stop service container", service.GetID())
			}
		}
	}
	if b.containerID != "" {
		b.logger.Debugln("Stopping container", b.containerID)
		err := fsouzaClient.StopContainer(b.containerID, 1)

		if err != nil {
			if _, ok := err.(*docker.ContainerNotRunning); ok {
				b.logger.Warnln("Box container has already stopped.")
			} else {
				b.logger.WithField("Error", err).Warnln("Wasn't able to stop box container", b.containerID)
			}
		}
	}
}

// Fetch an image (or update the local)
func (b *DockerBox) Fetch(ctx context.Context, env *util.Environment) (*types.ImageInspect, error) {
	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient
	officialClient := b.officialDockerClient

	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return nil, err
	}
	repo := env.Interpolate(b.repository)

	b.config.Auth.Interpolate(env)

	// If user use Azure or AWS container registry we don't infer.
	if b.config.Auth.AzureClientSecret == "" && b.config.Auth.AwsSecretKey == "" {
		repository, registry, err := InferRegistryAndRepository(repo, b.config.Auth.Registry, b.options)
		if err != nil {
			return nil, err
		}
		repo = repository
		b.config.Auth.Registry = registry
	}

	if b.config.Auth.Registry == b.options.WerckerContainerRegistry.String() {
		b.config.Auth.Username = DefaultDockerRegistryUsername
		b.config.Auth.Password = b.options.AuthToken
	}

	authenticator, err := dockerauth.GetRegistryAuthenticator(b.config.Auth)
	if err != nil {
		return nil, err
	}

	b.repository = authenticator.Repository(repo)
	b.Name = fmt.Sprintf("%s:%s", b.repository, b.tag)
	// Shortcut to speed up local dev
	if b.dockerOptions.Local {
		image, _, err := officialClient.ImageInspectWithRaw(ctx, env.Interpolate(b.Name))
		if err != nil {
			return nil, err
		}
		b.image = &image
		return &image, nil
	}

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()
	defer w.Close()

	// emitStatusses in a different go routine
	go EmitStatus(e, r, b.options)

	options := docker.PullImageOptions{
		OutputStream:  w,
		RawJSONStream: true,
		Repository:    b.repository,
		Tag:           env.Interpolate(b.tag),
	}
	authConfig := docker.AuthConfiguration{
		Username: authenticator.Username(),
		Password: authenticator.Password(),
	}
	err = fsouzaClient.PullImage(options, authConfig)
	if err != nil {
		return nil, err
	}
	image, _, err := officialClient.ImageInspectWithRaw(ctx, env.Interpolate(b.Name))
	if err != nil {
		return nil, err
	}
	b.image = &image

	return nil, err
}

// Commit the current running Docker container to an Docker image.
func (b *DockerBox) Commit(name, tag, message string, cleanup bool) (*docker.Image, error) {
	b.logger.WithFields(util.LogFields{
		"Name": name,
		"Tag":  tag,
	}).Debugln("Commit container:", name, tag)

	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient

	commitOptions := docker.CommitContainerOptions{
		Container:  b.containerID,
		Repository: name,
		Tag:        tag,
		Message:    "Build completed",
		Author:     "wercker",
	}
	image, err := fsouzaClient.CommitContainer(commitOptions)
	if err != nil {
		return nil, err
	}

	if cleanup {
		b.images = append(b.images, image)
	}

	return image, nil
}

// ExportImageOptions are the options available for ExportImage.
type ExportImageOptions struct {
	Name         string
	OutputStream io.Writer
}

// ExportImage will export the image to a temporary file and return the path to
// the file.
func (b *DockerBox) ExportImage(options *ExportImageOptions) error {
	b.logger.WithField("ExportName", options.Name).Info("Storing image")

	exportImageOptions := docker.ExportImageOptions{
		Name:         options.Name,
		OutputStream: options.OutputStream,
	}

	// TODO(termie): maybe move the container manipulation outside of here?
	fsouzaClient := b.fsouzaClient

	return fsouzaClient.ExportImage(exportImageOptions)
}
