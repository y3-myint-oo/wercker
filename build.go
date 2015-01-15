package main

import (
	"fmt"
)

// Build is our basic wrapper for Build operations
type Build struct {
	*BasePipeline
	options *GlobalOptions
}

// ToBuild converts a RawBuild into a Build
func (b *RawBuild) ToBuild(options *GlobalOptions) (*Build, error) {
	var steps []*Step

	// Start with the secret step, wercker-init that runs before everything
	rawStepData := RawStepData{}
	werckerInit := `wercker-init "https://api.github.com/repos/wercker/wercker-init/tarball"`
	initStep, err := NewStep(werckerInit, rawStepData, options)
	if err != nil {
		return nil, err
	}
	steps = append(steps, initStep)

	for _, extraRawStep := range b.RawSteps {
		rawStep, err := NormalizeStep(extraRawStep)
		if err != nil {
			return nil, err
		}
		step, err := rawStep.ToStep(options)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	build := &Build{NewBasePipeline(options, steps), options}

	id, ok := build.options.Env.Map["WERCKER_BUILD_ID"]
	if ok {
		build.options.BuildID = id
	}

	build.InitEnv()

	return build, nil
}

// InitEnv sets up the internal state of the environment for the build
func (b *Build) InitEnv() {
	env := b.Env()

	a := [][]string{
		[]string{"BUILD", "true"},
		[]string{"CI", "true"},
		[]string{"WERCKER_BUILD_ID", b.options.BuildID},
		[]string{"WERCKER_BUILD_URL", fmt.Sprintf("%s#build/%s", b.options.BaseURL, b.options.BuildID)},
	}

	env.Update(b.CommonEnv())
	env.Update(a)
	env.Update(b.MirrorEnv())
	env.Update(b.PassthruEnv())
}

// DockerRepo calculates our repo name
func (b *Build) DockerRepo() string {
	return fmt.Sprintf("%s/%s", b.options.ApplicationOwnerName, b.options.ApplicationName)
}

// DockerRepo calculates our tag
func (b *Build) DockerTag() string {
	tag := b.options.Tag
	if tag == "" {
		tag = fmt.Sprintf("build-%s", b.options.BuildID)
	}
	return tag
}

// DockerRepo calculates our message
func (b *Build) DockerMessage() string {
	message := b.options.Message
	if message == "" {
		message = fmt.Sprintf("Build %s", b.options.BuildID)
	}
	return message
}

// CollectArtifact copies the artifacts associated with the Build.
func (b *Build) CollectArtifact(sess *Session) (*Artifact, error) {
	artificer := NewArtificer(b.options)

	// Ensure we have the host directory

	artifact := &Artifact{
		ContainerID:   sess.ContainerID,
		GuestPath:     b.options.GuestPath("output"),
		HostPath:      b.options.HostPath("build.tar"),
		ApplicationID: b.options.ApplicationID,
		BuildID:       b.options.BuildID,
	}

	sourceArtifact := &Artifact{
		ContainerID:   sess.ContainerID,
		GuestPath:     b.options.SourcePath(),
		HostPath:      b.options.HostPath("build.tar"),
		ApplicationID: b.options.ApplicationID,
		BuildID:       b.options.BuildID,
	}

	// Get the output dir, if it is empty grab the source dir.
	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		if err == ErrEmptyTarball {
			fullArtifact, err = artificer.Collect(sourceArtifact)
			if err != nil {
				return nil, err
			}
			return fullArtifact, nil
		}
		return nil, err
	}

	return fullArtifact, nil
}
