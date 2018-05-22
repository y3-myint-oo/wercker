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
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// Builder interface to create an image based on a service config
// kinda needed so we can break a bunch of circular dependencies with cmd
type Builder interface {
	Build(context.Context, *util.Environment, *core.BoxConfig) (*DockerBox, *types.ImageInspect, error)
}

type nilBuilder struct{}

func (b *nilBuilder) Build(ctx context.Context, env *util.Environment, config *core.BoxConfig) (*DockerBox, *types.ImageInspect, error) {
	return nil, nil, nil
}

func NewNilBuilder() *nilBuilder {
	return &nilBuilder{}
}

// InternalServiceBox wraps a box as a service
type InternalServiceBox struct {
	*DockerBox
	logger *util.LogEntry
}

// ExternalServiceBox wraps a box as a service
type ExternalServiceBox struct {
	*InternalServiceBox
	externalConfig *core.BoxConfig
	builder        Builder
}

// NewExternalServiceBox gives us an ExternalServiceBox from config
func NewExternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (*ExternalServiceBox, error) {
	logger := util.RootLogger().WithField("Logger", "ExternalService")
	box := &DockerBox{options: options, dockerOptions: dockerOptions, config: boxConfig}
	return &ExternalServiceBox{
		InternalServiceBox: &InternalServiceBox{DockerBox: box, logger: logger},
		externalConfig:     boxConfig,
		builder:            builder,
	}, nil
}

// Fetch the image representation of an ExternalServiceBox
// this means running the ExternalServiceBox and comitting the image
func (s *ExternalServiceBox) Fetch(ctx context.Context, env *util.Environment) (*types.ImageInspect, error) {
	originalShortName := s.externalConfig.ID
	box, image, err := s.builder.Build(ctx, env, s.externalConfig)
	if err != nil {
		return nil, err
	}
	box.image = image
	s.DockerBox = box
	s.ShortName = originalShortName
	return image, err
}

func NewServiceBox(config *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (core.ServiceBox, error) {
	if config.IsExternal() {
		return NewExternalServiceBox(config, options, dockerOptions, builder)
	}
	return NewInternalServiceBox(config, options, dockerOptions)
}

// NewServiceBox from a name and other references
func NewInternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options) (*InternalServiceBox, error) {
	box, err := NewDockerBox(boxConfig, options, dockerOptions)
	logger := util.RootLogger().WithField("Logger", "Service")
	return &InternalServiceBox{DockerBox: box, logger: logger}, err
}

// TODO(mh) need to add to interface?
func (b *InternalServiceBox) getContainerName() string {
	name := b.config.Name
	if name == "" {
		name = b.Name
	}
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(name, "/", "-", -1), b.options.RunID)
	containerName = strings.Replace(containerName, ":", "_", -1)
	return strings.Replace(containerName, ":", "_", -1)
}

// Run executes the service
func (b *InternalServiceBox) Run(ctx context.Context, env *util.Environment) (string, error) {
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return "", err
	}
	f := &util.Formatter{}

	officialDockerClient, err := NewOfficialDockerClient(b.dockerOptions)
	if err != nil {
		return "", err
	}

	// Import the environment and command
	myEnv := dockerEnv(b.config.Env, env)

	origEntrypoint := b.image.Config.Entrypoint
	origCmd := b.image.Config.Cmd
	cmdInfo := []string{}

	var entrypoint []string
	if b.entrypoint != "" {
		entrypoint, err = shlex.Split(b.entrypoint)
		if err != nil {
			return "", err
		}
		cmdInfo = append(cmdInfo, entrypoint...)
	} else {
		cmdInfo = append(cmdInfo, origEntrypoint...)
	}

	var cmd []string
	if b.config.Cmd != "" {
		cmd, err = shlex.Split(b.config.Cmd)
		if err != nil {
			return "", err
		}
		cmdInfo = append(cmdInfo, cmd...)
	} else {
		cmdInfo = append(cmdInfo, origCmd...)
	}

	binds := make([]string, 0)

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

	portsToBind := []string{""}

	if b.options.ExposePorts {
		portsToBind = b.config.Ports
	}

	portBindings, err := portBindings(portsToBind)
	if err != nil {
		return "", err
	}

	networkName, err := b.GetDockerNetworkName()
	if err != nil {
		return "", err
	}

	hostConfig := &container.HostConfig{
		DNS:          b.dockerOptions.DNS,
		PortBindings: portBindings,
		//Links:        links,
		//NetworkMode:  networkName,
		//NetworkMode: container.NetworkMode(),
	}

	if len(binds) > 0 {
		hostConfig.Binds = binds
	}

	var ports nat.PortSet
	if b.options.ExposePorts {
		ports, err = exposedPorts(b.config.Ports)
	}
	if err != nil {
		return "", err
	}

	config := &container.Config{
		Image:           b.Name,
		Cmd:             cmd,
		Env:             myEnv,
		ExposedPorts:    ports,
		NetworkDisabled: b.networkDisabled,
		Entrypoint:      entrypoint,
	}

	// TODO(termie): terrible hack
	// Get service count so we can divvy memory
	serviceCount := ctx.Value("ServiceCount").(int)
	if b.dockerOptions.Memory != 0 {
		mem := b.dockerOptions.Memory
		mem = int64(float64(mem) * 0.25 / float64(serviceCount))

		swap := b.dockerOptions.MemorySwap
		if swap == 0 {
			swap = 2 * mem
		}

		hostConfig.Resources = container.Resources{
			Memory:     mem,
			MemorySwap: swap,
		}
	}

	endpointSettings := &network.EndpointSettings{
		Aliases: []string{b.GetServiceAlias()},
	}

	endpointsConfig := make(map[string]*network.EndpointSettings)
	endpointsConfig[networkName] = endpointSettings

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: endpointsConfig,
	}

	containerCreateCreatedBody, err := officialDockerClient.ContainerCreate(ctx, config, hostConfig, networkingConfig, b.getContainerName())
	if err != nil {
		return "", err
	}

	out := []string{}
	for _, part := range cmdInfo {
		if strings.Contains(part, " ") {
			out = append(out, fmt.Sprintf("%q", part))
		} else {
			out = append(out, part)
		}
	}
	if b.options.Verbose {
		b.logger.Println(f.Info(fmt.Sprintf("Starting service %s", b.ShortName), strings.Join(out, " ")))
	}

	err = officialDockerClient.ContainerStart(ctx, containerCreateCreatedBody.ID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	fsouzaDockerClient, err := NewDockerClient(b.dockerOptions)
	if err != nil {
		return "", err
	}

	go func() {
		status, err := fsouzaDockerClient.WaitContainer(containerCreateCreatedBody.ID)
		if err != nil {
			b.logger.Errorln("Error waiting", err)
		}
		b.logger.Debugln("Service container finished with status code:", status, containerCreateCreatedBody.ID)

		if status != 0 {
			var errstream bytes.Buffer
			var outstream bytes.Buffer
			// recv := make(chan string)
			// outputStream := NewReceiver(recv)
			opts := docker.LogsOptions{
				Container:    containerCreateCreatedBody.ID,
				Stdout:       true,
				Stderr:       true,
				ErrorStream:  &errstream,
				OutputStream: &outstream,
				RawTerminal:  false,
			}
			err = fsouzaDockerClient.Logs(opts)
			if err != nil {
				b.logger.Panicln(err)
			}
			e.Emit(core.Logs, &core.LogsArgs{
				Stream: fmt.Sprintf("%s-stdout", b.Name),
				Logs:   outstream.String(),
			})
			e.Emit(core.Logs, &core.LogsArgs{
				Stream: fmt.Sprintf("%s-stderr", b.Name),
				Logs:   errstream.String(),
			})
		}
	}()

	b.containerID = containerCreateCreatedBody.ID
	b.containerName = b.getContainerName()
	return containerCreateCreatedBody.ID, nil
}

// GetServiceAlias returns service alias for the service.
func (b *InternalServiceBox) GetServiceAlias() string {
	name := b.config.Name
	if name == "" {
		name = b.ShortName
	}
	return name
}
