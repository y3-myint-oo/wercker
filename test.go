package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/suite"
)

var (
	globalFlags   = flagsFor(GlobalFlags)
	pipelineFlags = flagsFor(PipelineFlags, WerckerInternalFlags)
	emptyFlags    = []cli.Flag{}
)

// TestSuite is our base class for test suites
type TestSuite struct {
	suite.Suite
	workingDir string
}

// SetupTest mostly just configures logging now
func (s *TestSuite) SetupTest() {
	setupTestLogging(s.T())
}

// TearDownTest cleans up our working dir if we made one
func (s *TestSuite) TearDownTest() {
	if s.workingDir != "" {
		s.workingDir = ""
		err := os.RemoveAll(s.WorkingDir())
		if err != nil {
			s.T().Error(err.Error())
		}
	}
}

// WorkingDir makes a new temp dir to run tests in
func (s *TestSuite) WorkingDir() string {
	if s.workingDir == "" {
		s.workingDir, _ = ioutil.TempDir("", "sentcli-")
	}
	return s.workingDir
}

// FailNow just proxies to testing.T.FailNow
func (s *TestSuite) FailNow() {
	s.T().FailNow()
}

// Skip just proxies to testing.T.Skip
func (s *TestSuite) Skip(msg string) {
	s.T().Skip(msg)
}

// DockerOrSkip checks for a docker container and skips the test
// if one is not available
func (s *TestSuite) DockerOrSkip() *DockerClient {
	if os.Getenv("SKIP_DOCKER_TEST") == "true" {
		s.Skip("$SKIP_DOCKER_TEST=true, skipping test")
		return nil
	}

	client, err := NewDockerClient(minimalDockerOptions())
	err = client.Ping()
	if err != nil {
		s.Skip("Docker not available, skipping test")
		return nil
	}
	return client
}

func emptyEnv() *Environment {
	return NewEnvironment()
}

func emptyPipelineOptions() *PipelineOptions {
	return &PipelineOptions{GlobalOptions: &GlobalOptions{}}
}

func minimalDockerOptions() *DockerOptions {
	opts := &DockerOptions{GlobalOptions: &GlobalOptions{}}
	guessAndUpdateDockerOptions(opts, NewEnvironment(os.Environ()...))
	return opts
}

type containerRemover struct {
	*docker.Container
	client *DockerClient
}

func tempBusybox(client *DockerClient) (*containerRemover, error) {
	_, err := client.InspectImage("alpine")
	if err != nil {
		options := docker.PullImageOptions{
			Repository: "alpine",
			Tag:        "3.1",
		}

		err = client.PullImage(options, docker.AuthConfiguration{})
		if err != nil {
			return nil, err
		}
	}

	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: "temp-busybox",
			Config: &docker.Config{
				Image:           "alpine:3.1",
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
	if cc == nil {
		return
	}
	cc.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            cc.Container.ID,
		RemoveVolumes: true,
	})
}

// TestLogWriter writes our logs to the test output
type TestLogWriter struct {
	t *testing.T
}

// NewTestLogWriter constructor
func NewTestLogWriter(t *testing.T) *TestLogWriter {
	return &TestLogWriter{t: t}
}

// Write for io.Writer
func (l *TestLogWriter) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}

// TestLogFormatter removes the last newline character
type TestLogFormatter struct {
	*logrus.TextFormatter
}

// NewTestLogFormatter constructor
func NewTestLogFormatter() *TestLogFormatter {
	return &TestLogFormatter{&logrus.TextFormatter{}}
}

// Format like a text log but strip the last newline
func (f *TestLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b, err := f.TextFormatter.Format(entry)
	if err == nil {
		b = b[:len(b)-1]
	}
	return b, err
}

func setupTestLogging(t *testing.T) {
	writer := NewTestLogWriter(t)
	rootLogger.SetLevel("debug")
	rootLogger.Out = writer
	rootLogger.Formatter = NewTestLogFormatter()
}

// Stepper lets use step and sync goroutines
type Stepper struct {
	stepper chan struct{}
}

// NewStepper constructor
func NewStepper() *Stepper {
	return &Stepper{stepper: make(chan struct{})}
}

// Wait until Step has been called
func (s *Stepper) Wait() {
	s.stepper <- struct{}{}
	<-s.stepper
}

// Step through a waiting goroutine with optional delay
func (s *Stepper) Step(delay ...int) {
	<-s.stepper
	for _, d := range delay {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	s.stepper <- struct{}{}
}
