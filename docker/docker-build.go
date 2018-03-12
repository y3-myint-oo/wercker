//   Copyright © 2018, Oracle and/or its affiliates.  All rights reserved.
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
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	digest "github.com/opencontainers/go-digest"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerBuildStep needs to implemenet Step
type DockerBuildStep struct {
	*core.BaseStep
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	tags          []string
	logger        *util.LogEntry
	dockerfile    string
	extrahosts    []string
	q             bool
	squash        bool
	buildargs     []docker.BuildArg
	labels        map[string]string
}

// NewDockerBuildStep is a special step for doing docker builds
func NewDockerBuildStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerBuildStep, error) {
	name := "docker-build"
	displayName := "docker build"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := core.NewBaseStep(core.BaseStepOptions{
		DisplayName: displayName,
		Env:         &util.Environment{},
		ID:          name,
		Name:        name,
		Owner:       "wercker",
		SafeID:      stepSafeID,
		Version:     util.Version(),
	})

	return &DockerBuildStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerBuildStep"),
		options:       options,
		dockerOptions: dockerOptions,
	}, nil
}

func (s *DockerBuildStep) configure(env *util.Environment) {

	if tags, ok := s.data["tag"]; ok {
		splitTags := util.SplitSpaceOrComma(tags)
		interpolatedTags := make([]string, len(splitTags))
		for i, tag := range splitTags {
			interpolatedTags[i] = env.Interpolate(tag)
		}
		s.tags = interpolatedTags
	}

	if dockerfile, ok := s.data["dockerfile"]; ok {
		s.dockerfile = env.Interpolate(dockerfile)
	}

	if labelsProp, ok := s.data["labels"]; ok {
		parsedLabels, err := shlex.Split(labelsProp)
		if err == nil {
			labelMap := make(map[string]string)
			for _, labelPair := range parsedLabels {
				pair := strings.Split(labelPair, "=")
				labelMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
			}
			s.labels = labelMap
		}
	}

	if buildargsProp, ok := s.data["buildargs"]; ok {
		parsedArgs, err := shlex.Split(buildargsProp)
		if err == nil {
			argMap := make(map[string]string)
			var buildArgs = make([]docker.BuildArg, len(parsedArgs))
			for i, labelPair := range parsedArgs {
				pair := strings.Split(labelPair, "=")
				argMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
				buildArgs[i] = docker.BuildArg{Name: pair[0], Value: pair[1]}
			}
			s.buildargs = buildArgs
		}
	}

	if qProp, ok := s.data["q"]; ok {
		q, err := strconv.ParseBool(qProp)
		if err == nil {
			s.q = q
		} else {
			// bad value, default to false (verbose)
			s.q = true
		}
	} else {
		// not set, default to false (verbose)
		s.q = true
	}

	if extrahostsProp, ok := s.data["extrahosts"]; ok {
		parsedExtrahosts, err := shlex.Split(extrahostsProp)
		if err == nil {
			interpolatedExtrahosts := make([]string, len(parsedExtrahosts))
			for i, thisExtrahost := range parsedExtrahosts {
				interpolatedExtrahosts[i] = env.Interpolate(thisExtrahost)
			}
			s.extrahosts = interpolatedExtrahosts
		}
	}

	if squashProp, ok := s.data["squash"]; ok {
		squash, err := strconv.ParseBool(squashProp)
		if err == nil {
			s.squash = squash
		} else {
			// bad value, default to false (do not squash)
			s.squash = false
		}
	} else {
		// not set, default to false (do not squash)
		s.squash = false
	}

	if labelsProp, ok := s.data["labels"]; ok {
		parsedLabels, err := shlex.Split(labelsProp)
		if err == nil {
			labelMap := make(map[string]string)
			for _, labelPair := range parsedLabels {
				pair := strings.Split(labelPair, "=")
				labelMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
			}
			s.labels = labelMap
		}
	}

}

// InitEnv parses our data into our config
func (s *DockerBuildStep) InitEnv(env *util.Environment) {
	s.configure(env)
}

// Fetch NOP
func (s *DockerBuildStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute builds an image
func (s *DockerBuildStep) Execute(ctx context.Context, sess *core.Session) (int, error) {

	s.logger.Debugln("Starting DockerBuildStep", s.data)

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	// Extract the /pipeline/source directory from the running pipeline container
	// and save it as a tarfile currentSource.tar
	_, err := s.CollectArtifact(containerID)
	if err != nil {
		return -1, err
	}

	// In currentSource.tar, the source directory is in /source
	// Copy all the files that are under /source in currentSource.tar
	// into the / directory of a new tarfile currentSourceInRoot.tar
	// This will be the tar we sent to the docker build command
	currentSourceUnderRootTar := "currentSourceUnderRoot.tar"
	err = s.buildInputTar("currentSource.tar", currentSourceUnderRootTar)
	if err != nil {
		return -1, err
	}

	// TODO(termie): could probably re-use the transport's client
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return 1, err
	}

	// Create an io.Writer to which the BuildImage API call will write build status messages
	// EmitBuildStatus will emit these messages as Log messages
	r, w := io.Pipe()
	go emitBuildStatus(e, r, s.options)
	defer w.Close()

	tarFile, err := os.Open(s.options.HostPath(currentSourceUnderRootTar))
	tarReader := bufio.NewReader(tarFile)

	buildOpts := docker.BuildImageOptions{
		Dockerfile:     s.dockerfile,
		InputStream:    tarReader,
		OutputStream:   w,
		Name:           s.tags[0], // go-dockerclient only allows us to set one tag
		BuildArgs:      s.buildargs,
		SuppressOutput: s.q,
		// cannot set Labels paramater as it is not supported by BuildImageOptions
		// cannot set Extrahosts paramater as it is not supported by BuildImageOptions
		// cannot set Squash parameter as it is not supported by BuildImageOptions
	}

	s.logger.Debugln("Build image")
	err = client.BuildImage(buildOpts)
	if err != nil {
		s.logger.Errorln("Failed to build image:", err)
		return -1, err
	}

	// TODO delete intermediate images created when running the Dockerfile
	// See cleanupImage in docker.go

	s.logger.Debug("Image built")
	return 0, nil
}

// CollectFile NOP
func (s *DockerBuildStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact copies the /pipeline/source directory from the running pipeline container
// and saves it as a directory currentSource and a tarfile currentSource.tar
func (s *DockerBuildStep) CollectArtifact(containerID string) (*core.Artifact, error) {
	artificer := NewArtificer(s.options, s.dockerOptions)

	artifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.GuestPath("source"),
		HostPath:      s.options.HostPath("currentSource"),
		HostTarPath:   s.options.HostPath("currentSource.tar"),
		ApplicationID: s.options.ApplicationID,
		RunID:         s.options.RunID,
		Bucket:        s.options.S3Bucket,
	}

	s.logger.WithFields(util.LogFields{
		"ContainerID":   artifact.ContainerID,
		"GuestPath":     artifact.GuestPath,
		"HostPath":      artifact.HostPath,
		"HostTarPath":   artifact.HostTarPath,
		"ApplicationID": artifact.ApplicationID,
		"RunID":         artifact.RunID,
		"Bucket":        artifact.Bucket,
	}).Debugln("Collecting artifacts from container to ", artifact.HostTarPath)

	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		return nil, err
	}
	return fullArtifact, nil
}

func (s *DockerBuildStep) buildInputTar(sourceTar string, destTar string) error {
	// In currentSource.tar, the source directory is in /source
	// Copy all the files that are under /source in currentSource.tar
	// into the / directory of a new tarfile currentSourceInRoot.tar
	artifactReader, err := os.Open(s.options.HostPath(sourceTar))
	if err != nil {
		return err
	}
	defer artifactReader.Close()

	s.logger.Debugln("Building input tar", s.options.HostPath(destTar))

	layerFile, err := os.OpenFile(s.options.HostPath(destTar), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer layerFile.Close()

	digester := digest.Canonical.Digester()
	mwriter := io.MultiWriter(layerFile, digester.Hash())

	tr := tar.NewReader(artifactReader)
	tw := tar.NewWriter(mwriter)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// finished the tarball
			err = tw.Close()
			if err != nil {
				return err
			}
			break
		}

		if err != nil {
			return err
		}

		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		// copy files from /source into the root of the new tar
		if strings.HasPrefix(hdr.Name, "source/") {
			hdr.Name = hdr.Name[len("source/"):]
		}

		if len(hdr.Name) == 0 {
			continue
		}

		tw.WriteHeader(hdr)
		_, err = io.Copy(tw, tr)
		if err != nil {
			return err
		}

	}
	return nil
}

// ReportPath NOP
func (s *DockerBuildStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerBuildStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}

// EmitBuildStatus is used to Log the normal docker build progress messages returned by the BuildImage API call
// There's a slight bug here in that image downloads are displayed as separate lines rather than a single animated line
func emitBuildStatus(e *core.NormalizedEmitter, r io.Reader, options *core.PipelineOptions) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		e.Emit(core.Logs, &core.LogsArgs{
			Logs:   scanner.Text() + "\n",
			Stream: "docker",
		})

	}
}
