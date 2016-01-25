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
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
)

// Artificer collects artifacts from containers and uploads them.
type Artificer struct {
	options       *core.PipelineOptions
	dockerOptions *DockerOptions
	logger        *util.LogEntry
	store         core.Store
}

// NewArtificer returns an Artificer
func NewArtificer(options *core.PipelineOptions, dockerOptions *DockerOptions) *Artificer {
	logger := util.RootLogger().WithField("Logger", "Artificer")

	s3store := core.NewS3Store(options.AWSOptions)

	return &Artificer{
		options: options,
		logger:  logger,
		store:   s3store,
	}
}

// Collect an artifact from the container, if it doesn't have any files in
// the tarball return util.ErrEmptyTarball
func (a *Artificer) Collect(artifact *core.Artifact) (*core.Artifact, error) {
	client, _ := NewDockerClient(a.dockerOptions)

	if err := os.MkdirAll(filepath.Dir(artifact.HostPath), 0755); err != nil {
		return nil, err
	}

	outputFile, err := os.Create(artifact.HostPath)
	defer outputFile.Close()
	if err != nil {
		return nil, err
	}

	opts := docker.CopyFromContainerOptions{
		OutputStream: outputFile,
		Container:    artifact.ContainerID,
		Resource:     artifact.GuestPath,
	}
	if err = client.CopyFromContainer(opts); err != nil {
		return nil, err
	}

	if _, err = outputFile.Seek(0, 0); err != nil {
		return nil, err
	}

	hasFiles := false
	tarReader := tar.NewReader(outputFile)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !header.FileInfo().IsDir() {
			hasFiles = true
			break
		}
	}
	if !hasFiles {
		return nil, util.ErrEmptyTarball
	}

	return artifact, nil
}

// Upload an artifact to S3
func (a *Artificer) Upload(artifact *core.Artifact) error {
	return a.store.StoreFromFile(&core.StoreFromFileArgs{
		Path:        artifact.HostPath,
		Key:         artifact.RemotePath(),
		ContentType: artifact.ContentType,
		MaxTries:    3,
		Meta:        artifact.Meta,
	})
}

// DockerFileCollector impl of FileCollector
type DockerFileCollector struct {
	client      *DockerClient
	containerID string
	logger      *util.LogEntry
}

// NewDockerFileCollector constructor
func NewDockerFileCollector(client *DockerClient, containerID string) *DockerFileCollector {
	return &DockerFileCollector{
		client:      client,
		containerID: containerID,
		logger:      util.RootLogger().WithField("Logger", "DockerFileCollector"),
	}
}

// Collect grabs a path and returns an archive containing stream along with
// an error channel to select on
func (fc *DockerFileCollector) Collect(path string) (*util.Archive, chan error) {
	pipeReader, pipeWriter := io.Pipe()
	opts := docker.CopyFromContainerOptions{
		OutputStream: pipeWriter,
		Container:    fc.containerID,
		Resource:     path,
	}

	errs := make(chan error)

	go func() {
		defer close(errs)
		if err := fc.client.CopyFromContainer(opts); err != nil {
			switch err.(type) {
			case *docker.Error:
				derr := err.(*docker.Error)
				if derr.Status == 500 && strings.HasPrefix(derr.Message, "Could not find the file") {
					errs <- util.ErrEmptyTarball
				}
			default:
				errs <- err
			}
		}
		pipeWriter.Close()
	}()

	return util.NewArchive(pipeReader), errs
}
