//   Copyright @2018, Oracle and/or its affiliates. All rights reserved.

package dockerlocal

import (
	"fmt"
	"io"
	"strconv"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerRunStep struct {
	*core.BaseStep
	options         *core.PipelineOptions
	dockerOptions   *Options
	data            map[string]string
	env             []string
	logger          *util.LogEntry
	cmd             []string
	entrypoint      []string
	workingDir      string
	portBindings    map[docker.Port][]docker.PortBinding
	exposedPorts    map[docker.Port]struct{}
	user            string
	containerName   string
	networkDisabled bool
	image           string
	links           []string
}

// NewDockerRunStep is a special step for doing docker runs
func NewDockerRunStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerRunStep, error) {
	name := "docker-run"
	displayName := "docker run"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := core.NewBaseStep(core.BaseStepOptions{
		DisplayName: displayName,
		Env:         &util.Environment{},
		ID:          name,
		Name:        name,
		Owner:       "wercker",
		SafeID:      stepSafeID,
		Version:     util.Version(),
	})

	return &DockerRunStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerRunStep"),
		options:       options,
		dockerOptions: dockerOptions,
	}, nil
}

// InitEnv parses our data into our config
func (s *DockerRunStep) InitEnv(hostEnv *util.Environment) {
	env := s.Env()
	s.configure(env)
}

func (s *DockerRunStep) configure(env *util.Environment) {

	if ports, ok := s.data["ports"]; ok {
		parts, err := shlex.Split(ports)
		if err == nil {
			s.portBindings = portBindings(parts)
			s.exposedPorts = exposedPorts(parts)
		}
	}

	if workingDir, ok := s.data["working-dir"]; ok {
		s.workingDir = env.Interpolate(workingDir)
	}

	if image, ok := s.data["image"]; ok {
		s.image = env.Interpolate(image)
	}

	if containerName, ok := s.data["container-name"]; ok {
		s.containerName = s.options.RunID + env.Interpolate(containerName)
	}

	if workingDir, ok := s.data["links"]; ok {
		s.workingDir = env.Interpolate(workingDir)
	}

	if networkDisabled, ok := s.data["networkdisabled"]; ok {
		n, err := strconv.ParseBool(networkDisabled)
		if err == nil {
			s.networkDisabled = n
		}
	} else {
		s.networkDisabled = true
	}

	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.cmd = parts
		}
	}

	if entrypoint, ok := s.data["entrypoint"]; ok {
		parts, err := shlex.Split(entrypoint)
		if err == nil {
			s.entrypoint = parts
		}
	}

	if envi, ok := s.data["env"]; ok {
		parsedEnv, err := shlex.Split(envi)

		if err == nil {
			interpolatedEnv := make([]string, len(parsedEnv))
			for i, envVar := range parsedEnv {
				interpolatedEnv[i] = env.Interpolate(envVar)
			}
			s.env = interpolatedEnv
		}
	}

	if user, ok := s.data["user"]; ok {
		s.user = env.Interpolate(user)
	}
}

// Fetch NOP
func (s *DockerRunStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute creates the container and starts the container.
func (s *DockerRunStep) Execute(ctx context.Context, sess *core.Session) (int, error) {

	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}

	if err != nil {
		return 1, err
	}

	conf := &docker.Config{
		Image:           s.image,
		Cmd:             s.cmd,
		Env:             s.env,
		ExposedPorts:    s.exposedPorts,
		NetworkDisabled: s.networkDisabled,
		Entrypoint:      s.entrypoint,
		DNS:             s.dockerOptions.DNS,
		WorkingDir:      s.workingDir,
	}

	hostconfig := &docker.HostConfig{
		DNS:          s.dockerOptions.DNS,
		PortBindings: s.portBindings,
		Links:        s.links,
	}

	s.createContainer(client, conf, hostconfig)

	s.startContainer(client, hostconfig)

	return 0, nil
}

func (s *DockerRunStep) createContainer(client *DockerClient, conf *docker.Config, hostconfig *docker.HostConfig) (*docker.Container, error) {
	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name:       s.containerName,
			Config:     conf,
			HostConfig: hostconfig,
		})
	return container, err
}

func (s *DockerRunStep) startContainer(client *DockerClient, hostConfig *docker.HostConfig) error {
	err := client.StartContainer(s.containerName, hostConfig)
	return err
}

// CollectFile NOP
func (s *DockerRunStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerRunStep) CollectArtifact(string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerRunStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerRunStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}
