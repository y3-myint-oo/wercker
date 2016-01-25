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

	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
)

// DockerPipeline is our docker PipelineConfigurer and Pipeline impl
type DockerPipeline struct {
	*core.BasePipeline
}

func NewDockerPipeline(config *core.Config, options *core.PipelineOptions, dockerOptions *DockerOptions, builder Builder) (*DockerPipeline, error) {
	// decide which configs to use for each thing
	// TODO(termie): this code is not specific to docker and should be made
	//               into something shared
	// TODO(termie): do different things based on "dev", "build", or "deploy"
	//               commands
	pipelineName := "build"
	pipelineConfig, ok := config.PipelinesMap[pipelineName]
	if !ok {
		return nil, fmt.Errorf("No pipeline named %s", pipelineName)
	}

	// Select this pipeline's config or the global config
	boxConfig := pipelineConfig.Box.BoxConfig
	if boxConfig == nil {
		boxConfig = config.Box.BoxConfig
	}

	// Select this pipeline's service or the global config
	servicesConfig := pipelineConfig.Services
	if servicesConfig == nil {
		servicesConfig = config.Services
	}

	stepsConfig := pipelineConfig.Steps
	afterStepsConfig := pipelineConfig.AfterSteps

	box, err := NewBox(boxConfig, options, dockerOptions)
	if err != nil {
		return nil, err
	}

	var services []core.ServiceBox
	for _, serviceConfig := range servicesConfig {
		service, err := NewServiceBox(serviceConfig.BoxConfig, options, dockerOptions, builder)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	initStep, err := core.NewWerckerInitStep(options)
	if err != nil {
		return nil, err
	}

	steps := []core.Step{initStep}
	for _, stepConfig := range stepsConfig {
		step, err := NewStep(stepConfig.StepConfig, options, dockerOptions)
		if err != nil {
			return nil, err
		}
		if step != nil {
			// we can return a nil step if it's internal and EnableDevSteps is
			// false
			steps = append(steps, step)
		}
	}

	var afterSteps []core.Step
	for _, stepConfig := range afterStepsConfig {
		step, err := NewStep(stepConfig.StepConfig, options, dockerOptions)
		if err != nil {
			return nil, err
		}
		if step != nil {
			// we can return a nil step if it's internal and EnableDevSteps is
			// false
			steps = append(steps, step)
		}
	}
	// if we found some valid after steps, prepend init
	if len(afterSteps) > 0 {
		afterSteps = append([]core.Step{initStep}, afterSteps...)
	}

	logger := util.RootLogger().WithField("Logger", "Pipeline")
	base := core.NewBasePipeline(core.BasePipelineOptions{
		Options:    options,
		Env:        util.NewEnvironment(),
		Box:        box,
		Services:   services,
		Steps:      steps,
		AfterSteps: afterSteps,
		Logger:     logger,
	})
	return &DockerPipeline{base}, nil
}
