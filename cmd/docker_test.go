package cmd

import (
	"io/ioutil"
	"net/url"
	"path"
	"testing"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

type DockerSuite struct {
	*util.TestSuite
}

func TestDockerSuite(t *testing.T) {
	suiteTester := &DockerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *DockerSuite) TestEnsureWerckerCredentialsNoToken() {
	opts := &core.WerckerDockerOptions{
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "",
		},
	}

	err := ensureWerckerCredentials(opts)
	s.Equal(errNoWerckerToken, err, "errNoWerckerToken was not returned when token was not present")
}

func (s *DockerSuite) TestEnsureWerckerCredentialsTokenNoConfig() {
	testWerckerRegistry, _ := url.Parse("")

	opts := &core.WerckerDockerOptions{
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "1234",
		},
		WerckerContainerRegistry: testWerckerRegistry,
	}

	tempDir := s.WorkingDir()
	config.SetDir(tempDir)
	filename := path.Join(tempDir, "config.json")
	data := []byte("{}")

	err := ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		s.Fail(err.Error(), "Failed to write docker config file")
	}

	err = ensureWerckerCredentials(opts)
	s.NoError(err, "Error ensuring wercker credentials")
	s.TearDownTest()
}

func (s *DockerSuite) TestEnsureWerckerCredentialsWithDockerConfig() {
	testWerckerRegistry, _ := url.Parse("")

	opts := &core.WerckerDockerOptions{
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "1234",
		},
		WerckerContainerRegistry: testWerckerRegistry,
	}

	tempDir := s.WorkingDir()
	config.SetDir(tempDir)
	filename := path.Join(tempDir, "config.json")
	data := []byte("{}")

	err := ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		s.Fail(err.Error(), "Failed to write docker config file")
	}

	dockerConfig := config.LoadDefaultConfigFile(ioutil.Discard)
	dockerConfig.AuthConfigs[opts.WerckerContainerRegistry.String()] = types.AuthConfig{
		Username: "token",
		Password: opts.AuthToken,
	}
	err = dockerConfig.Save()
	if err != nil {
		s.Fail(err.Error(), "Failed to save docker config file")
	}
	err = ensureWerckerCredentials(opts)
	s.NoError(err, "Error ensuring wercker credentials")
	s.TearDownTest()
}
