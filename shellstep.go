package main

import (
	"fmt"
	"io"

	"code.google.com/p/go-uuid/uuid"
	"github.com/chuckpreslar/emission"
	"github.com/flynn/go-shlex"
	"golang.org/x/net/context"
)

// ShellStep needs to implemenet IStep
type ShellStep struct {
	*BaseStep
	Code   string
	Cmd    []string
	data   map[string]string
	logger *LogEntry
	e      *emission.Emitter
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
		logger:   rootLogger.WithField("Logger", "ShellStep"),
		e:        GetEmitter(),
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
	err = client.AttachInteractive(containerID, s.Cmd, []string{s.Code})
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
