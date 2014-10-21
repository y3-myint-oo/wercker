package main

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
)

type ServiceBox struct {
	*Box
}

func (b *RawBox) ToServiceBox(build *Build, options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return CreateServiceBox(string(*b), build, options, boxOptions)
}

// CreateBox from a name and other references
func CreateServiceBox(name string, build *Build, options *GlobalOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := CreateBox(name, build, options, boxOptions)
	return &ServiceBox{Box: box}, err
}

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
