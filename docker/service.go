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
	"bytes"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
	"golang.org/x/net/context"
)

// InternalServiceBox wraps a box as a service
type InternalServiceBox struct {
	*DockerBox
	logger *util.LogEntry
}

// ExternalServiceBox wraps a box as a service
type ExternalServiceBox struct {
	*InternalServiceBox
	externalConfig *core.BoxConfig
	options        *core.PipelineOptions
}

// NewExternalServiceBox gives us an ExternalServiceBox from config
func NewExternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions) (*ExternalServiceBox, error) {
	logger := util.RootLogger().WithField("Logger", "ExternalService")
	return &ExternalServiceBox{
		InternalServiceBox: &InternalServiceBox{logger: logger},
		externalConfig:     boxConfig,
		options:            options,
	}, nil
}

func (s *ExternalServiceBox) configURL() (*url.URL, error) {
	return url.Parse(s.externalConfig.URL)
}

func (s *ExternalServiceBox) getOptions(env *util.Environment) (*core.PipelineOptions, error) {
	c, err := s.configURL()
	if err != nil {
		return nil, err
	}
	servicePath := filepath.Join(c.Host, c.Path)
	if !filepath.IsAbs(servicePath) {
		servicePath, err = filepath.Abs(
			filepath.Join(s.options.ProjectPath, servicePath))
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
	newOptions.DockerLocal = true
	newOptions.DockerOptions = s.options.DockerOptions
	newOptions.Pipeline = c.Fragment
	return newOptions, nil
}

// Fetch the image representation of an ExternalServiceBox
// this means running the ExternalServiceBox and comitting the image
func (s *ExternalServiceBox) Fetch(ctx context.Context, env *util.Environment) (*docker.Image, error) {
	newOptions, err := s.getOptions(env)

	if err != nil {
		return nil, err
	}

	shared, err := cmdBuild(ctx, newOptions)
	if err != nil {
		return nil, err
	}
	bc := *s.externalConfig
	bc.ID = fmt.Sprintf("%s:%s", shared.pipeline.DockerRepo(),
		shared.pipeline.DockerTag())

	box, err := NewBox(&bc, s.options, &BoxOptions{})
	if err != nil {
		return nil, err
	}
	// mh: don't like this...
	s.Box = box
	// will this work for normal services, too?
	s.ShortName = s.externalConfig.ID

	client, err := NewDockerClient(s.options.DockerOptions)
	s.image, err = client.InspectImage(s.Name)
	if err != nil {
		return nil, err
	}
	return s.image, nil
}

func NewServiceBox(config *core.BoxConfig, options *core.PipelineOptions) (core.ServiceBox, error) {
	if config.IsExternal() {
		return NewExternalServiceBox(config, options)
	}
	return NewServiceBox(config, options)
}

// NewServiceBox from a name and other references
func NewInternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions) (*InternalServiceBox, error) {
	box, err := NewBox(boxConfig, options, boxOptions)
	logger := util.RootLogger().WithField("Logger", "Service")
	return &InternalServiceBox{Box: box, logger: logger}, err
}

// TODO(mh) need to add to interface?
func (b *InternalServiceBox) getContainerName() string {
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(b.Name, "/", "-", -1), b.options.PipelineID)
	containerName = strings.Replace(containerName, ":", "_", -1)
	return strings.Replace(containerName, ":", "_", -1)
}

// Run executes the service
func (b *InternalServiceBox) Run(ctx context.Context, env *util.Environment, links []string) (*docker.Container, error) {
	e, err := EmitterFromContext(ctx)
	if err != nil {
		return nil, err
	}
	f := &util.Formatter{b.options.GlobalOptions.ShowColors}

	client, err := NewDockerClient(b.options.DockerOptions)
	if err != nil {
		return nil, err
	}

	// Import the environment and command
	myEnv := dockerEnv(b.config.Env, env)

	origEntrypoint := b.image.Config.Entrypoint
	origCmd := b.image.Config.Cmd
	cmdInfo := []string{}

	var entrypoint []string
	if b.entrypoint != "" {
		entrypoint, err = shlex.Split(b.entrypoint)
		if err != nil {
			return nil, err
		}
		cmdInfo = append(cmdInfo, entrypoint...)
	} else {
		cmdInfo = append(cmdInfo, origEntrypoint...)
	}

	var cmd []string
	if b.config.Cmd != "" {
		cmd, err = shlex.Split(b.config.Cmd)
		if err != nil {
			return nil, err
		}
		cmdInfo = append(cmdInfo, cmd...)
	} else {
		cmdInfo = append(cmdInfo, origCmd...)
	}

	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: b.getContainerName(),
			Config: &docker.Config{
				Image:           b.Name,
				Cmd:             cmd,
				Env:             myEnv,
				NetworkDisabled: b.networkDisabled,
				DNS:             b.options.DockerDNS,
				Entrypoint:      entrypoint,
			},
		})

	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, part := range cmdInfo {
		if strings.Contains(part, " ") {
			out = append(out, fmt.Sprintf("%q", part))
		} else {
			out = append(out, part)
		}
	}
	if b.options.Verbose {
		b.logger.Println(f.Info(fmt.Sprintf("Starting service %s", b.ShortName), strings.Join(out, " ")))
	}

	client.StartContainer(container.ID, &docker.HostConfig{
		DNS:   b.options.DockerDNS,
		Links: links,
	})
	b.container = container

	go func() {
		status, err := client.WaitContainer(container.ID)
		if err != nil {
			b.logger.Errorln("Error waiting", err)
		}
		b.logger.Debugln("Service container finished with status code:", status, container.ID)

		if status != 0 {
			var errstream bytes.Buffer
			var outstream bytes.Buffer
			// recv := make(chan string)
			// outputStream := NewReceiver(recv)
			opts := docker.LogsOptions{
				Container:    container.ID,
				Stdout:       true,
				Stderr:       true,
				ErrorStream:  &errstream,
				OutputStream: &outstream,
				RawTerminal:  false,
			}
			err = client.Logs(opts)
			if err != nil {
				b.logger.Panicln(err)
			}
			e.Emit(Logs, &LogsArgs{
				Stream: fmt.Sprintf("%s-stdout", b.Name),
				Logs:   outstream.String(),
			})
			e.Emit(Logs, &LogsArgs{
				Stream: fmt.Sprintf("%s-stderr", b.Name),
				Logs:   errstream.String(),
			})
		}
	}()

	return container, nil
}
