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
func (b *RawBox) ToServiceBox(options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return NewServiceBox(string(*b), options, boxOptions)
}

// NewServiceBox from a name and other references
func NewServiceBox(name string, options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := NewBox(name, options, boxOptions)
	logger := rootLogger.WithField("Logger", "Service")
	return &ServiceBox{Box: box, logger: logger}, err
}

// Run executes the service
func (b *ServiceBox) Run() (*docker.Container, error) {
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(b.Name, "/", "-", -1), b.options.PipelineID)

	container, err := b.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:           b.Name,
				NetworkDisabled: b.networkDisabled,
			},
		})

	if err != nil {
		return nil, err
	}

	b.client.StartContainer(container.ID, &docker.HostConfig{})
	b.container = container

	go func() {
		status, err := b.client.WaitContainer(container.ID)
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
			err = b.client.Logs(opts)
			if err != nil {
				b.logger.Panicln(err)
			}
			e := GetEmitter()
			e.Emit(Logs, &LogsArgs{
				Options: b.options,
				Hidden:  false,
				Stream:  fmt.Sprintf("%s-stdout", b.Name),
				Logs:    outstream.String(),
			})
			e.Emit(Logs, &LogsArgs{
				Options: b.options,
				Hidden:  false,
				Stream:  fmt.Sprintf("%s-stderr", b.Name),
				Logs:    errstream.String(),
			})
		}
	}()

	return container, nil
}
