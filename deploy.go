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
func (p *RawPipeline) ToDeploy(options *PipelineOptions) (*Deploy, error) {
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

	deploy := &Deploy{NewBasePipeline(options, steps, afterSteps), options}
	deploy.InitEnv()
	return deploy, nil
}

// InitEnv sets up the internal state of the environment for the build
func (d *Deploy) InitEnv() {
	env := d.Env()

	a := [][]string{
		[]string{"DEPLOY", "true"},
		[]string{"WERCKER_DEPLOY_ID", d.options.DeployID},
		[]string{"WERCKER_DEPLOY_URL", fmt.Sprintf("%s#deploy/%s", d.options.BaseURL, d.options.DeployID)},
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
