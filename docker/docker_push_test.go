package dockerlocal

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"fmt"
)

type PushSuite struct {
	*util.TestSuite
}

func TestPushSuite(t *testing.T) {
	suiteTester := &PushSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//TestEmptyPush tests if you juse did something like this
// - internal/docker-push
// it should fill in a tag of the git branch and commit
// check to see if its pushing up to the right registry or not
func (s *PushSuite) TestEmptyPush() {
	config := &core.StepConfig{
		ID:   "internal/docker-push",
		Data: map[string]string{},
	}
	u, _ := url.Parse("https://container-reg.oracle.com")
	options := &core.PipelineOptions{
		GitOptions: &core.GitOptions{
			GitBranch: "master",
			GitCommit: "s4k2r0d6a9b",
		},
		ApplicationID:            "1000001",
		ApplicationName:          "myproject",
		ApplicationOwnerName:     "wercker",
		WerckerContainerRegistry: u,
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "su69persec420uret0k3n",
		},
	}
	step, _ := NewDockerPushStep(config, options, nil)
	step.InitEnv(nil)
	repositoryName := step.authenticator.Repository(step.repository)
	s.Equal("container-reg.oracle.com/wercker/myproject", repositoryName)
	tags := step.buildTags()
	s.Equal([]string{"latest", "master-s4k2r0d6a9b"}, tags)
}

func (s *DockerSuite) TestRegistryRepository() { //TODO (tvjames) better name for this test
	testWerckerRegistry, _ := url.Parse("https://test-wercker-registry.com/v2")
	dockerhub := "https://registry.hub.docker.com/v1/"
	tests := map[[2]string][2]string{
		//TODO (Tvjames) use a struct here
		{"", "appowner/appname"} : 					{dockerhub, dockerhub + "appowner/appname"},
		{"", ""} : 									{testWerckerRegistry.String(), testWerckerRegistry.String() + "/appowner/appname"},
		{"", "someregistry.com/appowner/appname"} : {"https://someregistry.com/v2/", "appowner/appname"},
		{"someregistry.com", "appowner/appname"} : 	{"someregistry.com", "appowner/appname"},
	}

	for input, expected := range tests {
		config := &core.StepConfig{
			ID: "internal/docker-push",
			Data: map[string]string{},
		}
		if input[0] != "" {
			config.Data["registry"] = input[0]
		}
		options := &core.PipelineOptions{
			ApplicationOwnerName: "appowner",
			ApplicationName: "appname",
			WerckerContainerRegistry: testWerckerRegistry,
		}
		step, _ := NewDockerPushStep(config, options, nil)
		step.repository = input[1]

		configurePushStep(step, util.NewEnvironment())
		opts := buildAutherOpts(step, util.NewEnvironment([]string{}...))
		fmt.Printf("input: registry: %v, repo: %v\n", input[0], input[1])
		s.Equal(expected[0], opts.Registry, fmt.Sprintf("input: registry: %v, repo: %v\n", input[0], input[1]))
		s.Equal(expected[1], step.repository, fmt.Sprintf("input: registry: %v, repo: %v\n", input[0], input[1]))
	}

}
