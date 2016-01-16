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
	"io"

	"github.com/fsouza/go-dockerclient"
)

type Containerer interface {
	RunAndAttach(string) error
	AttachInteractive(string, []string, []string) error
	ResizeTTY(string) error
	AttachTerminal(string) error
	ExecOne(string, []string, io.Writer) error
	CheckAccess(CheckAccessOptions) (bool, error)

	// These ones are hitting fsouza/go-dockerclient stuff
	// TODO(termie): wrap these?
	CreateContainer(docker.CreateContainerOptions) (*docker.Container, error)
	StartContainer(string, *docker.HostConfig) error
	RemoveContainer(docker.RemoveContainerOptions) error
	RemoveImage(string) error
	RestartContainer(string, int) error
	StopContainer(string, int) error
	InspectImage(string) (*docker.Image, error)
	LoadImage(docker.LoadImageOptions) error
	PushImage(docker.PushImageOptions) error
	CommitContainer(docker.CommitContainerOptions) (*docker.Image, error)
	PullImage(docker.PullImageOptions, docker.AuthConfiguration) error
	ExportImage(docker.ExportImageOptions) error
}

// CheckAccessOptions is just args for CheckAccess
type CheckAccessOptions struct {
	Auth       docker.AuthConfiguration
	Access     string
	Repository string
	Tag        string
	Registry   string
}
