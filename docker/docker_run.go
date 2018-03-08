//   Copyright 2016 Wercker Holding BV
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
	"fmt"
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/pborman/uuid"
	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerRunStep struct {
	*core.BaseStep
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	email         string
	env           []string
	stopSignal    string
	builtInPush   bool
	labels        map[string]string
	user          string
	authServer    string
	repository    string
	author        string
	message       string
	tags          []string
	ports         map[docker.Port]struct{}
	volumes       map[string]struct{}
	cmd           []string
	entrypoint    []string
	forceTags     bool
	logger        *util.LogEntry
	workingDir    string
	authenticator auth.Authenticator
}

// NewDockerRunStep is a special step for doing docker pushes
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
	a := [][]string{
		[]string{"WERCKER_GIT_COMMIT", hostEnv.Map["WERCKER_GIT_COMMIT"]},
		[]string{"WERCKER_GIT_REPOSITORY", hostEnv.Map["WERCKER_GIT_REPOSITORY"]},
		[]string{"WERCKER_APPLICATION_OWNER_NAME", hostEnv.Map["WERCKER_APPLICATION_OWNER_NAME"]},
	}
	env.Update(a)
}

// Fetch NOP
func (s *DockerRunStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute commits the current container and pushes it to the configured
// registry
func (s *DockerRunStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}

	//TODO use this
	//e, err := core.EmitterFromContext(ctx)

	if err != nil {
		return 1, err
	}
	conf := &docker.Config{
		Image: s.data["image"],
	}
	hostconfig := &docker.HostConfig{}

	client.CreateContainer(
		docker.CreateContainerOptions{
			Name:       s.BaseStep.DisplayName(),
			Config:     conf,
			HostConfig: hostconfig,
		})

	client.StartContainer(s.BaseStep.DisplayName(), hostconfig)

	return 0, nil
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
