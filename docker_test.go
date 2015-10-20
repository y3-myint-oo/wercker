package main

import (
	"os"
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func minimalDockerOptions() *DockerOptions {
	opts := &DockerOptions{GlobalOptions: &GlobalOptions{}}
	guessAndUpdateDockerOptions(opts, NewEnvironment(os.Environ()...))
	return opts
}

func dockerOrSkip(t *testing.T) *DockerClient {
	if os.Getenv("SKIP_DOCKER_TEST") == "true" {
		t.Skip("$SKIP_DOCKER_TEST=true, skipping test")
		return nil
	}

	client, err := NewDockerClient(minimalDockerOptions())
	err = client.Ping()
	if err != nil {
		t.Skip("Docker not available, skipping test")
		return nil
	}
	return client
}

type containerRemover struct {
	*docker.Container
	client *DockerClient
}

func tempBusybox(client *DockerClient) (*containerRemover, error) {
	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: "temp-busybox",
			Config: &docker.Config{
				Image:           "busybox",
				Tty:             false,
				OpenStdin:       true,
				Cmd:             []string{"/bin/sh"},
				AttachStdin:     true,
				AttachStdout:    true,
				AttachStderr:    true,
				NetworkDisabled: true,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return &containerRemover{Container: container, client: client}, nil
}

func (cc *containerRemover) Remove() {
	cc.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            cc.Container.ID,
		RemoveVolumes: true,
	})
}

func TestDockerNormalizeRegistry(t *testing.T) {
	quay := "https://quay.io/v1/"
	dock := "https://registry.hub.docker.com/v1/"
	assert.Equal(t, quay, normalizeRegistry("https://quay.io"))
	assert.Equal(t, quay, normalizeRegistry("https://quay.io/v1"))
	assert.Equal(t, quay, normalizeRegistry("http://quay.io/v1"))
	assert.Equal(t, quay, normalizeRegistry("https://quay.io/v1/"))
	assert.Equal(t, quay, normalizeRegistry("quay.io"))

	assert.Equal(t, dock, normalizeRegistry(""))
	assert.Equal(t, dock, normalizeRegistry("https://registry.hub.docker.com"))
	assert.Equal(t, dock, normalizeRegistry("http://registry.hub.docker.com"))
	assert.Equal(t, dock, normalizeRegistry("registry.hub.docker.com"))
}

func TestDockerNormalizeRepo(t *testing.T) {
	assert.Equal(t, "gox-mirror", normalizeRepo("example.com/gox-mirror"))
	assert.Equal(t, "termie/gox-mirror", normalizeRepo("quay.io/termie/gox-mirror"))
	assert.Equal(t, "termie/gox-mirror", normalizeRepo("termie/gox-mirror"))
	assert.Equal(t, "mongo", normalizeRepo("mongo"))
}

func TestDockerPing(t *testing.T) {
	client := dockerOrSkip(t)
	err := client.Ping()
	assert.Nil(t, err)
}
