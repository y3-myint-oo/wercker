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

package cmd

import (
	"flag"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/docker"
	"github.com/wercker/sentcli/util"
	"golang.org/x/net/context"
)

type DockerBuilder struct {
	options       *core.PipelineOptions
	docekrOptions *dockerlocal.DockerOptions
}

func NewDockerBuilder(options *core.PipelineOptions, dockerOptions *dockerlocal.DockerOptions) *DockerBuilder {
	return &DockerBuilder{
		options:       options,
		dockerOptions: dockerOptions,
	}
}

func (b *DockerBuilder) configURL(config *core.BoxConfig) (*url.URL, error) {
	return url.Parse(config.URL)
}

func (b *DockerBuilder) getOptions(env *util.Environment, config *core.BoxConfig) (*core.PipelineOptions, error) {
	c, err := b.configURL(config)
	if err != nil {
		return nil, err
	}
	servicePath := filepath.Join(c.Host, c.Path)
	if !filepath.IsAbs(servicePath) {
		servicePath, err = filepath.Abs(
			filepath.Join(b.options.ProjectPath, servicePath))
		if err != nil {
			return nil, err
		}
	}

	flagSet := func(name string, flags []cli.Flag) *flag.FlagSet {
		set := flag.NewFlagSet(name, flag.ContinueOnError)

		for _, f := range flags {
			f.Apply(set)
		}
		return set
	}

	set := flagSet("runservice", flagsFor(PipelineFlags, WerckerInternalFlags))
	args := []string{
		servicePath,
	}
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	ctx := cli.NewContext(nil, set, set)
	newOptions, err := NewBuildOptions(ctx, env)
	if err != nil {
		return nil, err
	}

	newOptions.GlobalOptions = s.options.GlobalOptions
	newOptions.ShouldCommit = true
	newOptions.PublishPorts = s.options.PublishPorts
	// TODO(termie): PACKAGING these moved
	// newOptions.DockerLocal = true
	// newOptions.DockerOptions = s.dockerOptions
	newOptions.Pipeline = c.Fragment
	return newOptions, nil
}

// Build the
func (s *DockerBuilder) Build(ctx context.Context, env *util.Environment, config *core.BoxConfig) (*docker.Image, error) {
	newOptions, err := b.getOptions(env)

	if err != nil {
		return nil, err
	}

	shared, err := cmdBuild(ctx, newOptions)
	if err != nil {
		return nil, err
	}
	bc := config
	bc.ID = fmt.Sprintf("%s:%s", shared.pipeline.DockerRepo(),
		shared.pipeline.DockerTag())

	box, err := NewBox(bc, b.options, b.dockerOptions)
	if err != nil {
		return nil, err
	}
	// mh: don't like this...
	s.DockerBox = box
	// will this work for normal services, too?
	s.ShortName = config.ID

	client, err := NewDockerClient(b.dockerOptions)
	image, err = client.InspectImage(b.Name)
	if err != nil {
		return nil, err
	}
	return image, nil
}
