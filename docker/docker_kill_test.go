/* Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved. */
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
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

type DockerKillSuite struct {
	*util.TestSuite
}

func TestDockerKillSuite(t *testing.T) {
	suiteTester := &DockerKillSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

// TestContainerName tests if passed container name is set properly.
// - internal/docker-kill
func (s *DockerKillSuite) TestContainerName() {
	var data map[string]string
	data = make(map[string]string)
	data["container-name"] = "testContainer"
	config := &core.StepConfig{
		ID:   "internal/docker-kill",
		Data: data,
	}
	options := &core.PipelineOptions{
		GitOptions: &core.GitOptions{
			GitBranch: "master",
			GitCommit: "s4k2r0d6a9b",
		},
		ApplicationID:        "1000001",
		ApplicationName:      "myproject",
		ApplicationOwnerName: "wercker",

		WerckerContainerRegistry: &url.URL{Scheme: "https", Host: "wcr.io", Path: "/v2/"},
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "su69persec420uret0k3n",
		},
	}
	step, _ := NewDockerKillStep(config, options, nil)
	env := util.NewEnvironment("X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included")
	step.InitEnv(env)
	s.Equal("testContainer", step.containerName)
}
