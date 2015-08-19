package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flynn/go-shlex"
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
func (b *ServiceBox) Run(env *Environment, links []string) (*docker.Container, error) {
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(b.Name, "/", "-", -1), b.options.PipelineID)
	containerName = strings.Replace(containerName, ":", "_", -1)
	f := &Formatter{b.options.GlobalOptions}

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		cmdInfo = append(cmdInfo, entrypoint...)
	} else {
		cmdInfo = append(cmdInfo, origEntrypoint...)
	}

	var cmd []string
	if b.config.Cmd != "" {
		cmd, err = shlex.Split(b.config.Cmd)
		if err != nil {
			return nil, err
		}
		cmdInfo = append(cmdInfo, cmd...)
	} else {
		cmdInfo = append(cmdInfo, origCmd...)
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
				Entrypoint:      entrypoint,
			},
		})

	if err != nil {
		return nil, err
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

	client.StartContainer(container.ID, &docker.HostConfig{
		DNS:   b.options.DockerDNS,
		Links: links,
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
				Stream: fmt.Sprintf("%s-stdout", b.Name),
				Logs:   outstream.String(),
			})
			e.Emit(Logs, &LogsArgs{
				Stream: fmt.Sprintf("%s-stderr", b.Name),
				Logs:   errstream.String(),
			})
		}
	}()

	return container, nil
}
