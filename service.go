package main

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"strings"
)

// ServiceBox wraps a box as a service
type ServiceBox struct {
	*Box
}

// ToServiceBox turns a box into a ServiceBox
func (b *RawBox) ToServiceBox(options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return NewServiceBox(string(*b), options, boxOptions)
}

// NewServiceBox from a name and other references
func NewServiceBox(name string, options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := NewBox(name, options, boxOptions)
	return &ServiceBox{Box: box}, err
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

	return container, nil
}
