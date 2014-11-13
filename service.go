package main

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
)

// ServiceBox wraps a box as a service
type ServiceBox struct {
	*Box
}

// ToServiceBox turns a box into a ServiceBox
func (b *RawBox) ToServiceBox(build *Build, options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return CreateServiceBox(string(*b), build, options, boxOptions)
}

// CreateServiceBox from a name and other references
func CreateServiceBox(name string, build *Build, options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := CreateBox(name, build, options, boxOptions)
	return &ServiceBox{Box: box}, err
}

// Run executes the service
func (b *ServiceBox) Run() (*docker.Container, error) {
	containerName := fmt.Sprintf("wercker-service-%s-%s", b.Name, b.options.BuildID)

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
