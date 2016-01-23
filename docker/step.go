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
	"strings"

	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
)

func NewStep(config *core.StepConfig, options *core.PipelineOptions, dockerOptions *DockerOptions) (core.Step, error) {
	// NOTE(termie) Special case steps are special
	if config.ID == "internal/docker-push" {
		return NewDockerPushStep(config, options, dockerOptions)
	}
	if config.ID == "internal/docker-scratch-push" {
		return NewDockerScratchPushStep(config, options, dockerOptions)
	}
	if config.ID == "internal/store-container" {
		return NewStoreContainerStep(config, options, dockerOptions)
	}
	if strings.HasPrefix(config.ID, "internal/") {
		if !options.EnableDevSteps {
			util.RootLogger().Warnln("Ignoring dev step:", config.ID)
			return nil, nil
		}
	}
	if options.EnableDevSteps {
		if config.ID == "internal/watch" {
			return NewWatchStep(config, options, dockerOptions)
		}
		if config.ID == "internal/shell" {
			return NewShellStep(config, options, dockerOptions)
		}
	}
	return core.NewStep(config, options)
}
