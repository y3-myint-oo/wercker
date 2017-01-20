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

package core

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/util"
)

type ConfigSuite struct {
	*util.TestSuite
}

func TestConfigSuite(t *testing.T) {
	suiteTester := &ConfigSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *ConfigSuite) TestConfigBoxStrings() {
	b, err := ioutil.ReadFile("../tests/box_strings.yml")
	s.Nil(err)
	config, err := ConfigFromYaml(b)
	s.Require().Nil(err)
	s.Equal("strings_box", config.Box.ID)
	s.Equal("strings_service", config.Services[0].ID)
	//check to see if both the service and box has an auth
	assert.NotNil(s.T(), config.Box.Auth)
	assert.NotNil(s.T(), config.Services[0].Auth)

	_, ok := config.Box.Auth.(*DockerAuth)
	_, t := config.Services[0].Auth.(*DockerAuth)
	s.Equal(ok, true)
	s.Equal(t, true)
}

func (s *ConfigSuite) TestConfigBoxStructs() {
	b, err := ioutil.ReadFile("../tests/box_structs.yml")
	s.Nil(err)
	config, err := ConfigFromYaml(b)
	s.Require().Nil(err)
	s.Equal("structs_box", config.Box.ID)
	s.Equal("structs_service", config.Services[0].ID)
	assert.NotNil(s.T(), config.Box.Auth)
	assert.NotNil(s.T(), config.Services[0].Auth)

	pipeline := config.PipelinesMap["pipeline"]
	s.Equal(pipeline.Box.ID, "blue")
	s.Equal(pipeline.Steps[0].ID, "string-step")
	s.Equal(pipeline.Steps[1].ID, "script")
	s.Equal(pipeline.Steps[2].ID, "script")

	// test to see if proper authenticatables are set
	// and if those return the proper authenticators
	amzn := config.PipelinesMap["amzn"]
	assert.NotNil(s.T(), amzn.Box.Auth)
	_, ok := amzn.Box.Auth.(*AmazonAuth)
	s.Equal(ok, true)
	env := util.NewEnvironment(os.Environ()...)
	authenticator, _ := amzn.Box.Auth.ToAuthenticator(env)
	_, ok = authenticator.(*auth.AmazonAuth)
	s.Equal(ok, true)

	docker := config.PipelinesMap["docker-v2"]
	assert.NotNil(s.T(), docker.Box.Auth)
	_, ok = docker.Box.Auth.(*DockerAuth)
	s.Equal(ok, true)
	authenticator, _ = docker.Box.Auth.ToAuthenticator(env)
	_, ok = authenticator.(*auth.DockerAuth)
	s.Equal(ok, true)

	dockerV1 := config.PipelinesMap["docker"]
	assert.NotNil(s.T(), dockerV1.Box.Auth)
	_, ok = dockerV1.Box.Auth.(*DockerAuth)
	s.Equal(ok, true)
	authenticator, _ = dockerV1.Box.Auth.ToAuthenticator(env)
	_, ok = authenticator.(auth.DockerAuthV1)
	s.Equal(ok, true)
}

func (s *ConfigSuite) TestIfaceToString() {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"string input", "string input"},
		{int(1234), "1234"},
		{int32(123432), "123432"},
		{int64(123464), "123464"},
		{true, "true"},
		{false, "false"},

		// The following types are not supported, so a empty string is returned
		{nil, ""},
		{float32(123.123), ""},
		{float64(123.123), ""},
	}

	for _, test := range tests {
		actual := ifaceToString(test.input)
		s.Equal(test.expected, actual, "")
	}
}
