package main

import (
	"fmt"
)

// Build is our basic wrapper for Build operations
type Build struct {
	*BasePipeline
	options *PipelineOptions
}

// ToBuild converts a RawPipeline into a Build
func (p *RawPipeline) ToBuild(options *PipelineOptions) (*Build, error) {
	var steps []*Step
	var afterSteps []*Step

	// Start with the secret step, wercker-init that runs before everything
	initStep, err := NewWerckerInitStep(options)
	if err != nil {
		return nil, err
	}
	steps = append(steps, initStep)

	realSteps, err := ExtraRawStepsToSteps(p.RawSteps, options)
	if err != nil {
		return nil, err
	}
	steps = append(steps, realSteps...)

	// For after steps we again need werker-init
	realAfterSteps, err := ExtraRawStepsToSteps(p.RawAfterSteps, options)
	if err != nil {
		return nil, err
	}
	if len(realAfterSteps) > 0 {
		afterSteps = append(afterSteps, initStep)
		afterSteps = append(afterSteps, realAfterSteps...)
	}

	build := &Build{NewBasePipeline(options, steps, afterSteps), options}
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
		[]string{"WERCKER_GIT_DOMAIN", b.options.GitDomain},
		[]string{"WERCKER_GIT_OWNER", b.options.GitOwner},
		[]string{"WERCKER_GIT_REPOSITORY", b.options.GitRepository},
		[]string{"WERCKER_GIT_BRANCH", b.options.GitBranch},
		[]string{"WERCKER_GIT_COMMIT", b.options.GitCommit},
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

// DockerTag calculates our tag
func (b *Build) DockerTag() string {
	tag := b.options.Tag
	if tag == "" {
		tag = fmt.Sprintf("build-%s", b.options.BuildID)
	}
	return tag
}

// DockerMessage calculates our message
func (b *Build) DockerMessage() string {
	message := b.options.Message
	if message == "" {
		message = fmt.Sprintf("Build %s", b.options.BuildID)
	}
	return message
}

// CollectArtifact copies the artifacts associated with the Build.
func (b *Build) CollectArtifact(containerID string) (*Artifact, error) {
	artificer := NewArtificer(b.options)

	// Ensure we have the host directory

	artifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     b.options.GuestPath("output"),
		HostPath:      b.options.HostPath("build.tar"),
		ApplicationID: b.options.ApplicationID,
		BuildID:       b.options.BuildID,
		Bucket:        b.options.S3Bucket,
	}

	sourceArtifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     b.options.SourcePath(),
		HostPath:      b.options.HostPath("build.tar"),
		ApplicationID: b.options.ApplicationID,
		BuildID:       b.options.BuildID,
		Bucket:        b.options.S3Bucket,
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
