package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

const initEnvErrorMessage = "InitEnv failed"

type RunnerSuite struct {
	*util.TestSuite
}

func TestRunnerSuite(t *testing.T) {
	suiteTester := &RunnerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//MockStep mocks Step
type MockStep struct {
	*core.BaseStep
}

// Mock methods not implemented by BaseStep
func (s *MockStep) CollectArtifact(string) (*core.Artifact, error) {
	return nil, nil
}

func (s *MockStep) CollectFile(string, string, string, io.Writer) error {
	return nil
}

func (s *MockStep) Execute(context.Context, *core.Session) (int, error) {
	return 0, nil
}

func (s *MockStep) Fetch() (string, error) {
	return "", nil
}

func (s *MockStep) ReportPath(...string) string {
	return ""
}

func (s *MockStep) ShouldSyncEnv() bool {
	return false
}

func (s *MockStep) InitEnv(*util.Environment) error {
	return errors.New(initEnvErrorMessage)
}

//MockPipeline mocks Pipeline
type MockPipeline struct {
	*core.BasePipeline
}

//Mock methods not implemented by BasePipeLine
func (s *MockPipeline) CollectArtifact(string) (*core.Artifact, error) {
	return nil, nil
}

func (s *MockPipeline) CollectCache(string) error {
	return nil
}

func (s *MockPipeline) DockerMessage() string {
	return ""
}

func (s *MockPipeline) DockerRepo() string {
	return ""
}

func (s *MockPipeline) DockerTag() string {
	return ""
}

func (s *MockPipeline) InitEnv(*util.Environment) {

}

func (s *MockPipeline) LocalSymlink() {

}

func (s *MockPipeline) Env() *util.Environment {
	return nil
}

//TestRunnerStepFailedOnInitEnvError tests the scenario when a step in the pipleine
// will fail when an error occurs in initEnv() in step
func (s *RunnerSuite) TestRunnerStepFailedOnInitEnvError() {
	mockPipeline := &MockPipeline{}
	shared := &RunnerShared{pipeline: mockPipeline}
	step := &MockStep{}
	runner := &Runner{}
	runner.emitter = core.NewNormalizedEmitter()

	sr, err := runner.RunStep(shared, step, 1)
	s.Error(err)
	fmt.Println(err.Error())
	s.Contains(err.Error(), "Step initEnv failed with error message")
	s.Equal(sr.Message, initEnvErrorMessage)
	s.NotEqual(sr.Message, 0)
}
