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

	"golang.org/x/net/context"

	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"github.com/wercker/werckerclient"
	"github.com/wercker/werckerclient/credentials"
)

type TriggerRunStep struct {
	*core.BaseStep
	// ApplicationID string
	SourceRunID   string
	TargetID      string
	Message       string
	EnvVars       map[string]string
	data          map[string]string
	logger        *util.LogEntry
	env           *util.Environment
	options       *core.PipelineOptions
	dockerOptions *DockerOptions
}

func NewTriggerRunStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *DockerOptions) (*TriggerRunStep, error) {
	name := "trigger-run"
	displayName := "trigger run"
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

	return &TriggerRunStep{
		BaseStep:      baseStep,
		options:       options,
		dockerOptions: dockerOptions,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "TriggerRunStep"),
	}, nil
}

func (s *TriggerRunStep) InitEnv(env *util.Environment) {
	if target, ok := s.data["target-id"]; ok {
		s.TargetID = target
	}

	if message, ok := s.data["message"]; ok {
		s.Message = message
	}

	s.SourceRunID = s.options.RunID
}

// Execute collects and uploads the current pipeline artifact, and triggers
// a new run
func (s *TriggerRunStep) Execute(ctx context.Context, sess *core.Session) (int, error) {

	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return -1, err
	}

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	e.Emit(core.Logs, &core.LogsArgs{
		Logs: "Storing artifacts\n",
	})

	artifact, err := CollectPipelineArtifact(containerID, s.options, s.dockerOptions)
	// Ignore ErrEmptyTarball errors
	if err != util.ErrEmptyTarball {
		if err != nil {
			e.Emit(core.Logs, &core.LogsArgs{
				Logs: fmt.Sprintf("Storing artifacts failed: %s\n", err.Error()),
			})
			return -1, err
		}

		if s.options.ShouldStoreS3 {
			artificer := NewArtificer(s.options, s.dockerOptions)
			err = artificer.Upload(artifact)
			if err != nil {
				e.Emit(core.Logs, &core.LogsArgs{
					Logs: fmt.Sprintf("Storing artifacts failed: %s\n", err.Error()),
				})
				return -1, err
			}
		}
	}

	e.Emit(core.Logs, &core.LogsArgs{
		Logs: "Storing artifacts complete\n",
	})

	e.Emit(core.Logs, &core.LogsArgs{
		Logs: "Triggering run\n",
	})

	config := &werckerclient.Config{
		Endpoint: s.options.BaseURL,
	}
	if s.options.AuthToken != "" {
		config.Credentials = credentials.Token(s.options.AuthToken)
	}

	client := werckerclient.NewClient(config)

	params := &werckerclient.CreateChainRunOptions{
		SourceRunID: s.SourceRunID,
		TargetID:    s.TargetID,
	}

	newRun, err := client.CreateChainRun(params)
	if err != nil {
		return -1, err
	}

	e.Emit(core.Logs, &core.LogsArgs{
		Logs: fmt.Sprintf("Started run: %s", newRun.ID),
	})

	return 0, nil
}

// Fetch NOP
func (s *TriggerRunStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// CollectFile NOP
func (s *TriggerRunStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *TriggerRunStep) CollectArtifact(string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *TriggerRunStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *TriggerRunStep) ShouldSyncEnv() bool {
	return true
}
