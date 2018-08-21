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
	"encoding/hex"
	"testing"
	"time"

	"gopkg.in/mgo.v2/bson"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerSuite struct {
	*util.TestSuite
}

func TestDockerSuite(t *testing.T) {
	suiteTester := &DockerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *DockerSuite) TestPing() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())
	_, err := client.Ping(ctx)
	s.Nil(err)
}

func (s *DockerSuite) TestGenerateDockerID() {
	id, err := GenerateDockerID()
	s.Require().NoError(err, "Unable to generate Docker ID")

	// The ID needs to be a valid hex value
	b, err := hex.DecodeString(id)
	s.Require().NoError(err, "Generated Docker ID was not a hex value")

	// The ID needs to be 256 bits
	s.Equal(256, len(b)*8)
}

//TestCreateContainerWithRetries - Verifies that CreateContainerWithRetries is able to create the container
// even after few "no such image" errors due to image not being available
func (s *DockerSuite) TestCreateContainerWithRetries() {
	repoName := "alpine"
	originalTag := "3.1"
	testTag := bson.NewObjectId().Hex()
	testContainerName := bson.NewObjectId().Hex()

	// Check of docker is available
	client, err := NewDockerClient(MinimalDockerOptions())
	err = client.Ping()
	if err != nil {
		s.Skip("Docker not available, skipping test")
		return
	}

	// Check if alpine base image is available, if not pull the image
	_, err = client.InspectImage(repoName + ":" + originalTag)
	if err == docker.ErrNoSuchImage {
		options := docker.PullImageOptions{
			Repository: repoName,
			Tag:        originalTag,
		}
		err := client.PullImage(options, docker.AuthConfiguration{})
		s.NoError(err, "Unable to pull image")
		_, err = client.InspectImage(repoName + ":" + originalTag)
		s.NoError(err, "Unable to verify pulled image")
		// Cleanup image we just pulled
		defer func() {
			client.RemoveImage(repoName + ":" + originalTag)
		}()
	}

	// Check if image is already tagged to our test tag, if yes remove
	// We create a separate test tag so that we do not touch the original image
	_, err = client.InspectImage(repoName + ":" + testTag)
	if err == nil {
		client.RemoveImage(repoName + ":" + testTag)
	}

	// Check if there is a container with name same as test container, if yes remove
	existingContainerID := getContainerIDByContainerName(client, testContainerName)
	if existingContainerID != nil {
		client.RemoveContainer(docker.RemoveContainerOptions{Force: true, ID: *existingContainerID})
	}

	finished := make(chan int, 1)
	// Fire off CreateContainerWithRetries in a separate thread
	conf := &docker.Config{
		Image: repoName + ":" + testTag,
	}
	go func() {
		defer func() {
			finished <- 0
		}()
		container, err := client.CreateContainerWithRetries(docker.CreateContainerOptions{Name: testContainerName, Config: conf})

		s.NoError(err, "Error while creating container")
		s.NotNil(container, "Container is nil")
		s.Equal(testContainerName, container.Name, "Container created with a different name")

		//cleanup
		client.RemoveContainer(docker.RemoveContainerOptions{Force: true, ID: container.ID})
	}()

	// Wait for some time before making the image tag available
	// docker should respond with "no such image" during this time
	time.Sleep(4 * time.Second)

	// Now tag the image to make it available to CreateContainerWithRetries
	err = client.TagImage(repoName+":"+originalTag, docker.TagImageOptions{Repo: repoName, Tag: testTag})
	s.NoError(err, "Unable to tag image for testing")
	<-finished

	// Cleanup image we are testing
	client.RemoveImage(repoName + ":" + testTag)

}

func getContainerIDByContainerName(client *DockerClient, containerName string) *string {
	listContainerFilter := make(map[string][]string)
	names := make([]string, 1)
	names[0] = containerName
	listContainerFilter["name"] = names

	containers, err := client.ListContainers(docker.ListContainersOptions{All: true, Filters: listContainerFilter})
	if err != nil {
		return nil
	}

	if containers != nil && len(containers) != 0 {
		for _, container := range containers {
			if container.Names != nil && len(container.Names) == 0 && container.Names[0] == containerName {
				return &container.ID
			}
		}
	}
	return nil
}
