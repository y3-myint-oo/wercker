package sentcli

import (
	"fmt"

	"github.com/wercker/sentcli/util"
)

// Deploy is our basic wrapper for Deploy operations
type Deploy struct {
	*BasePipeline
	options *PipelineOptions
}

// ToDeploy grabs the build section from the config and configures all the
// instances necessary for the build
func (c *Config) ToDeploy(options *PipelineOptions, pipelineConfig *RawPipelineConfig) (*Deploy, error) {
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
	if options.DeployTarget != "" {
		sectionSteps, ok := pipelineConfig.StepsMap[options.DeployTarget]
		if ok {
			stepsConfig = sectionSteps
		}
	}
	if stepsConfig == nil {
		return nil, fmt.Errorf("No steps defined in the pipeline")
	}

	afterStepsConfig := pipelineConfig.AfterSteps

	// NewBasePipeline will init all the rest
	basePipeline, err := NewBasePipeline(options, pipelineConfig, boxConfig, servicesConfig, stepsConfig, afterStepsConfig)
	if err != nil {
		return nil, err
	}

	return &Deploy{basePipeline, options}, nil
}

// InitEnv sets up the internal state of the environment for the build
func (d *Deploy) InitEnv(hostEnv *util.Environment) {
	env := d.Env()

	a := [][]string{
		[]string{"DEPLOY", "true"},
		[]string{"WERCKER_DEPLOY_ID", d.options.DeployID},
		[]string{"WERCKER_DEPLOY_URL", fmt.Sprintf("%s/#deploy/%s", d.options.BaseURL, d.options.DeployID)},
		[]string{"WERCKER_GIT_DOMAIN", d.options.GitDomain},
		[]string{"WERCKER_GIT_OWNER", d.options.GitOwner},
		[]string{"WERCKER_GIT_REPOSITORY", d.options.GitRepository},
		[]string{"WERCKER_GIT_BRANCH", d.options.GitBranch},
		[]string{"WERCKER_GIT_COMMIT", d.options.GitCommit},
	}

	if d.options.DeployTarget != "" {
		a = append(a, []string{"WERCKER_DEPLOYTARGET_NAME", d.options.DeployTarget})
	}

	env.Update(d.CommonEnv())
	env.Update(a)
	env.Update(hostEnv.GetMirror())
	env.Update(hostEnv.GetPassthru().Ordered())
	env.Hidden.Update(hostEnv.GetHiddenPassthru().Ordered())
}

// DockerRepo returns the name where we might store this in docker
func (d *Deploy) DockerRepo() string {
	if d.options.Repository != "" {
		return d.options.Repository
	}
	return fmt.Sprintf("%s/%s", d.options.ApplicationOwnerName, d.options.ApplicationName)
}

// DockerTag returns the tag where we might store this in docker
func (d *Deploy) DockerTag() string {
	tag := d.options.Tag
	if tag == "" {
		tag = fmt.Sprintf("deploy-%s", d.options.DeployID)
	}
	return tag
}

// DockerMessage returns the message to store this with in docker
func (d *Deploy) DockerMessage() string {
	message := d.options.Message
	if message == "" {
		message = fmt.Sprintf("Build %s", d.options.DeployID)
	}
	return message
}

// CollectArtifact copies the artifacts associated with the Deploy.
// Unlike a Build, this will only collect the output directory if we made
// a new one.
func (d *Deploy) CollectArtifact(containerID string) (*Artifact, error) {
	artificer := NewArtificer(d.options)

	artifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     d.options.GuestPath("output"),
		HostPath:      d.options.HostPath("build.tar"),
		ApplicationID: d.options.ApplicationID,
		DeployID:      d.options.DeployID,
		Bucket:        d.options.S3Bucket,
		ContentType:   "application/x-tar",
	}

	// Get the output dir, if it is empty grab the source dir.
	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		return nil, err
	}

	return fullArtifact, nil
}
