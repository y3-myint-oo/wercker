package main

import "fmt"

// Build is our basic wrapper for Build operations
type Build struct {
	*BasePipeline
	options *PipelineOptions
}

// ToPipeline grabs the specified section from the config and configures all the
// instances necessary for the build
func (c *Config) ToPipeline(options *PipelineOptions, pipelineConfig *RawPipelineConfig) (*Build, error) {
	if pipelineConfig == nil {
		return nil, fmt.Errorf("No 'build' pipeline definition in wercker.yml")
	}

	// Either the pipeline's box or the global
	boxConfig := pipelineConfig.Box
	if boxConfig == nil {
		boxConfig = c.Box
	}
	if boxConfig == nil {
		return nil, fmt.Errorf("No box definition in either pipeline or global config")
	}

	// Either the pipeline's services or the global
	servicesConfig := pipelineConfig.Services
	if servicesConfig == nil {
		servicesConfig = c.Services
	}

	stepsConfig := pipelineConfig.Steps
	if stepsConfig == nil {
		return nil, fmt.Errorf("No steps defined in the pipeline")
	}

	afterStepsConfig := pipelineConfig.AfterSteps

	// NewBasePipeline will init all the rest
	basePipeline, err := NewBasePipeline(options, pipelineConfig, boxConfig, servicesConfig, stepsConfig, afterStepsConfig)
	if err != nil {
		return nil, err
	}

	return &Build{basePipeline, options}, nil
}

// InitEnv sets up the internal state of the environment for the build
func (b *Build) InitEnv(hostEnv *Environment) {
	env := b.Env()

	a := [][]string{
		[]string{"BUILD", "true"},
		[]string{"CI", "true"},
		[]string{"WERCKER_BUILD_ID", b.options.BuildID},
		[]string{"WERCKER_BUILD_URL", fmt.Sprintf("%s/#build/%s", b.options.BaseURL, b.options.BuildID)},
		[]string{"WERCKER_GIT_DOMAIN", b.options.GitDomain},
		[]string{"WERCKER_GIT_OWNER", b.options.GitOwner},
		[]string{"WERCKER_GIT_REPOSITORY", b.options.GitRepository},
		[]string{"WERCKER_GIT_BRANCH", b.options.GitBranch},
		[]string{"WERCKER_GIT_COMMIT", b.options.GitCommit},
	}

	env.Update(b.CommonEnv())
	env.Update(a)
	env.Update(hostEnv.getMirror())
	env.Update(hostEnv.getPassthru().Ordered())
	env.Hidden.Update(hostEnv.getHiddenPassthru().Ordered())
}

// DockerRepo calculates our repo name
func (b *Build) DockerRepo() string {
	if b.options.Repository != "" {
		return b.options.Repository
	}
	return fmt.Sprintf("build-%s", b.options.BuildID)
}

// DockerTag calculates our tag
func (b *Build) DockerTag() string {
	if b.options.Tag != "" {
		return b.options.Tag
	}
	return "latest"
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
