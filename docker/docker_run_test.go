package dockerlocal

import (
	"testing"

	"github.com/fsouza/go-dockerclient"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

type RunSuite struct {
	*util.TestSuite
}

func TestRunSuite(t *testing.T) {
	suiteTester := &RunSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *RunSuite) TestCreateContainer() {
	config := &core.StepConfig{
		ID:   "internal/docker-run",
		Data: map[string]string{},
	}
	options := &core.PipelineOptions{}

	step, _ := NewDockerRunStep(config, options, nil)
	step.containerName = "test_container"
	step.dockerOptions = &Options{Host: "unix:///var/run/docker.sock"}
	client, err := NewDockerClient(step.dockerOptions)
	if err != nil {
		s.Fail("Failed to create docker client.")
	}

	conf := &docker.Config{
		Image: "elasticsearch:latest",
	}
	hostConfig := &docker.HostConfig{}
	container, err := step.createContainer(client, conf, hostConfig)

	if err != nil {
		s.Fail("Failed to create container.")
	}

	actual_container, err := client.InspectContainer(container.ID)

	if err != nil {
		s.Fail("Failed to retrieve container")
	}

	s.NotNilf(actual_container, "actual container is not nil")
	s.NotEmptyf(actual_container, "actual container should not be empty")
	s.Equal("/"+step.containerName, actual_container.Name)

	cleanupContainer(client, actual_container.ID)
}

func cleanupContainer(client *DockerClient, id string) {

	client.RemoveContainer(
		docker.RemoveContainerOptions{
			ID:    id,
			Force: true,
		})
}
