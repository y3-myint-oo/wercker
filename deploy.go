package main

import (
	"fmt"
)

// Deploy is our basic wrapper for Deploy operations
type Deploy struct {
	*BasePipeline
	options *PipelineOptions
}

// ToDeploy converts a RawPipeline into a Deploy
func (p *PipelineConfig) ToDeploy(options *PipelineOptions) (*Deploy, error) {
	var box *Box
	var err error
	configBox := p.Box
	if configBox != nil {
		box, err = configBox.ToBox(options, &BoxOptions{})
		if err != nil {
			return nil, err
		}
	}

	var steps []IStep
	var afterSteps []IStep

	// Start with the secret step, wercker-init that runs before everything
	initStep, err := NewWerckerInitStep(options)
	if err != nil {
		return nil, err
	}
	steps = append(steps, initStep)

	// If p is nil it means no Deploy section was found
	// TODO(bvdberg): fail the build, fall back to default steps, idk. Just run
	// init for now.
	var configSteps []RawStepConfig
	if p != nil {
		if options.DeployTarget != "" {
			sectionSteps, ok := p.StepsMap[options.DeployTarget]
			if ok {
				configSteps = sectionSteps
			} else {
				configSteps = p.Steps
			}
		}
		realSteps, err := StepConfigsToSteps(configSteps, options)
		if err != nil {
			return nil, err
		}
		steps = append(steps, realSteps...)

		// For after steps we again need werker-init
		realAfterSteps, err := StepConfigsToSteps(p.AfterSteps, options)
		if err != nil {
			return nil, err
		}
		if len(realAfterSteps) > 0 {
			afterSteps = append(afterSteps, initStep)
			afterSteps = append(afterSteps, realAfterSteps...)
		}
	}

	deploy := &Deploy{NewBasePipeline(options, box, steps, afterSteps, p), options}
	deploy.InitEnv()
	return deploy, nil
}

// InitEnv sets up the internal state of the environment for the build
func (d *Deploy) InitEnv() {
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
	env.Update(d.MirrorEnv())
	env.Update(d.PassthruEnv())
}

// DockerRepo returns the name where we might store this in docker
func (d *Deploy) DockerRepo() string {
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
	}

	// Get the output dir, if it is empty grab the source dir.
	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		return nil, err
	}

	return fullArtifact, nil
}
