package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// ServiceBox wraps a box as a service
type ServiceBox struct {
	*Box
	logger *LogEntry
}

// ToServiceBox turns a box into a ServiceBox
func (b *BoxConfig) ToServiceBox(options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return NewServiceBox(b, options, boxOptions)
}

// NewServiceBox from a name and other references
func NewServiceBox(boxConfig *BoxConfig, options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := NewBox(boxConfig, options, boxOptions)
	logger := rootLogger.WithField("Logger", "Service")
	return &ServiceBox{Box: box, logger: logger}, err
}

// Run executes the service
func (b *ServiceBox) Run(env *Environment) (*docker.Container, error) {
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(b.Name, "/", "-", -1), b.options.PipelineID)
	containerName = strings.Replace(containerName, ":", "_", -1)

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}

	// Import the environment and command
	myEnv := dockerEnv(b.config.Env, env)
	cmd := []string{}
	if b.config.Cmd != "" {
		cmd = append(cmd, b.config.Cmd)
	}

	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:           b.Name,
				Cmd:             cmd,
				Env:             myEnv,
				NetworkDisabled: b.networkDisabled,
				DNS:             b.options.DockerDNS,
			},
		})

	if err != nil {
		return nil, err
	}

	client.StartContainer(container.ID, &docker.HostConfig{
		DNS: b.options.DockerDNS,
	})
	b.container = container

	go func() {
		status, err := client.WaitContainer(container.ID)
		if err != nil {
			b.logger.Errorln("Error waiting", err)
		}
		b.logger.Debugln("Service container finished with status code:", status, container.ID)

		if status != 0 {
			var errstream bytes.Buffer
			var outstream bytes.Buffer
			// recv := make(chan string)
			// outputStream := NewReceiver(recv)
			opts := docker.LogsOptions{
				Container:    container.ID,
				Stdout:       true,
				Stderr:       true,
				ErrorStream:  &errstream,
				OutputStream: &outstream,
				RawTerminal:  false,
			}
			err = client.Logs(opts)
			if err != nil {
				b.logger.Panicln(err)
			}
			e := GetGlobalEmitter()
			e.Emit(Logs, &LogsArgs{
				Options: b.options,
				Stream:  fmt.Sprintf("%s-stdout", b.Name),
				Logs:    outstream.String(),
			})
			e.Emit(Logs, &LogsArgs{
				Options: b.options,
				Stream:  fmt.Sprintf("%s-stderr", b.Name),
				Logs:    errstream.String(),
			})
		}
	}()

	return container, nil
}
