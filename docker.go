package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
	"os"
	"path"
)

type DockerClient struct {
	*docker.Client
}

// NewDockerClient based on options and env
func NewDockerClient(options *GlobalOptions) (*DockerClient, error) {
	dockerHost := options.DockerHost
	tlsVerify, ok := options.Env.Map["DOCKER_TLS_VERIFY"]
	var (
		client *docker.Client
		err    error
	)
	if ok && tlsVerify == "1" {
		// We're using TLS, let's locate our certs and such
		// boot2docker puts its certs at...
		dockerCertPath := options.Env.Map["DOCKER_CERT_PATH"]

		// TODO(termie): maybe fast-fail if these don't exist?
		cert := path.Join(dockerCertPath, fmt.Sprintf("cert.pem"))
		ca := path.Join(dockerCertPath, fmt.Sprintf("ca.pem"))
		key := path.Join(dockerCertPath, fmt.Sprintf("key.pem"))
		log.Println("key path", key)
		client, err = docker.NewVersionnedTLSClient(dockerHost, cert, key, ca, "")
		if err != nil {
			return nil, err
		}
	} else {
		client, err = docker.NewClient(dockerHost)
		if err != nil {
			return nil, err
		}
	}
	return &DockerClient{client}, nil
}

func (c *DockerClient) RunAndAttach(name string) error {
	container, err := c.CreateContainer(
		docker.CreateContainerOptions{
			Name: uuid.NewRandom().String(),
			Config: &docker.Config{
				Image:        name,
				Tty:          true,
				OpenStdin:    true,
				Cmd:          []string{"/bin/bash"},
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				// NetworkDisabled: b.networkDisabled,
				// Volumes: volumes,
			},
		})
	if err != nil {
		return err
	}
	c.StartContainer(container.ID, &docker.HostConfig{})

	opts := docker.AttachToContainerOptions{
		Container:    container.ID,
		Logs:         true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		InputStream:  os.Stdin,
		ErrorStream:  os.Stderr,
		OutputStream: os.Stdout,
		RawTerminal:  true,
	}

	var oldState *term.State

	oldState, err = term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	go func() {
		err := c.AttachToContainer(opts)
		if err != nil {
			log.Panicln("attach panic", err)
		}
	}()

	_, err = c.WaitContainer(container.ID)
	return err
}
