package main

import (
	"fmt"
	"io"

	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/sentcli/util"
	"golang.org/x/net/context"
)

// ShellStep needs to implemenet IStep
type ShellStep struct {
	*BaseStep
	Code   string
	Cmd    []string
	data   map[string]string
	logger *util.LogEntry
	env    *Environment
}

// NewShellStep is a special step for doing docker pushes
func NewShellStep(stepConfig *StepConfig, options *PipelineOptions) (*ShellStep, error) {
	name := "shell"
	displayName := "shell"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := &BaseStep{
		displayName: displayName,
		env:         &Environment{},
		id:          name,
		name:        name,
		options:     options,
		owner:       "wercker",
		safeID:      stepSafeID,
		version:     Version(),
	}

	return &ShellStep{
		BaseStep: baseStep,
		data:     stepConfig.Data,
		logger:   util.RootLogger().WithField("Logger", "ShellStep"),
	}, nil
}

// InitEnv parses our data into our config
func (s *ShellStep) InitEnv(env *Environment) {
	if code, ok := s.data["code"]; ok {
		s.Code = code
	}
	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.Cmd = parts
		}
	} else {
		s.Cmd = []string{"/bin/bash"}
	}
	s.env = env
}

// Fetch NOP
func (s *ShellStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute a shell and give it to the user
func (s *ShellStep) Execute(ctx context.Context, sess *Session) (int, error) {
	// cheating to get containerID
	// TODO(termie): we should deal with this eventually
	dt := sess.transport.(*DockerTransport)
	containerID := dt.containerID

	client, err := NewDockerClient(s.options.DockerOptions)
	if err != nil {
		return -1, err
	}

	code := s.env.Export()
	code = append(code, "cd $WERCKER_SOURCE_DIR")
	code = append(code, "clear")
	code = append(code, s.Code)

	err = client.AttachInteractive(containerID, s.Cmd, code)
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// CollectFile NOP
func (s *ShellStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *ShellStep) CollectArtifact(string) (*Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *ShellStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *ShellStep) ShouldSyncEnv() bool {
	return true
}
