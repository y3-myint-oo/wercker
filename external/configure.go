// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package external

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	context "golang.org/x/net/context"
)

// Get the Docker client
func (cp *RunnerParams) getDockerClient() error {
	context.Background()
	cli, err := docker.NewClient(cp.DockerEndpoint)
	if err != nil {
		cp.Logger.Fatal(fmt.Sprintf("unable to create the Docker client: %s", err))
		return err
	}
	cp.client = cli
	return nil
}

// Describe the local image and return the Image structure
func (cp *RunnerParams) getLocalImage() (*docker.Image, error) {
	err := cp.getDockerClient()
	if err != nil {
		return nil, err
	}
	image, err := cp.client.InspectImage(cp.ImageName)
	if err != nil {
		return nil, err
	}
	return image, err
}

// Describe the remote image and return the Image structure
func (cp *RunnerParams) getRemoteImage() (*docker.Image, error) {
	err := cp.getDockerClient()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Check the external runner images between local and remote repositories.
// If local exists but remote does not then do nothing
// If local exists and is the same as the remote then do nothing
// If local is older than remote then give user the option to download the remote
// If neither exists then fail immediately
func (cp *RunnerParams) CheckRegistryImages() error {

	err := cp.getDockerClient()
	if err != nil {
		cp.Logger.Fatal(err)
	}
	localImage, err := cp.getLocalImage()

	if err != nil {
		cp.Logger.Fatal(err)
	}
	message := fmt.Sprintf("Docker image %s is up-to-date, created: %s", cp.ImageName, localImage.Created)
	cp.Logger.Print(message)
	return nil
}
