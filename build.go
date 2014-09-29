package main

import (
	"errors"
	"fmt"
	"path"
)

// Build is our basic wrapper for Build operations
type Build struct {
	Env     *Environment
	Steps   []*Step
	options *GlobalOptions
}

var mirroredEnv = [...]string{"WERCKER_GIT_DOMAIN",
	"WERCKER_GIT_OWNER",
	"WERCKER_GIT_REPOSITORY",
	"WERCKER_GIT_BRANCH",
	"WERCKER_GIT_COMMIT",
	"WERCKER_STARTED_BY",
	"WERCKER_MAIN_PIPELINE_STARTED",
	"WERCKER_APPLICATION_URL",
	"WERCKER_APPLICATION_ID",
	"WERCKER_APPLICATION_NAME",
	"WERCKER_APPLICATION_OWNER_NAME"}

// ToBuild converts a RawBuild into a Build
func (b *RawBuild) ToBuild(options *GlobalOptions) (*Build, error) {
	var steps []*Step
	var build Build

	// Start with the secret step, wercker-init that runs before everything
	rawStepData := RawStepData{}
	werckerInit := `wercker-init "https://api.github.com/repos/wercker/wercker-init/tarball"`
	initStep, err := CreateStep(werckerInit, rawStepData, &build, options)
	if err != nil {
		return nil, err
	}
	steps = append(steps, initStep)

	for _, extraRawStep := range b.RawSteps {
		rawStep, err := NormalizeStep(extraRawStep)
		if err != nil {
			return nil, err
		}
		step, err := rawStep.ToStep(&build, options)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	build.options = options
	build.Steps = steps

	id, ok := build.options.Env.Map["WERCKER_BUILD_ID"]
	if ok {
		build.options.BuildID = id
	}

	build.InitEnv()

	return &build, nil
}

// InitEnv sets up the internal state of the environment for the build
func (b *Build) InitEnv() {
	b.Env = &Environment{}
	// TODO(termie): deal with PASSTHRU args from the user here
	b.Env.Update(b.getMirrorEnv())

	// Add all of our basic env vars
	m := map[string]string{
		"WERCKER":              "true",
		"BUILD":                "true",
		"CI":                   "true",
		"WERCKER_BUILD_ID":     b.options.BuildID,
		"WERCKER_ROOT":         b.GuestPath("source"),
		"WERCKER_SOURCE_DIR":   b.GuestPath("source", b.options.SourceDir),
		"WERCKER_CACHE_DIR":    "/cache",
		"WERCKER_OUTPUT_DIR":   b.GuestPath("output"),
		"WERCKER_PIPELINE_DIR": b.GuestPath(),
		"WERCKER_REPORT_DIR":   b.GuestPath("report"),
		"TERM":                 "xterm-256color",
	}
	b.Env.Update(m)
}

func (b *Build) getMirrorEnv() map[string]string {
	var m = make(map[string]string)
	for _, key := range mirroredEnv {
		value, ok := b.options.Env.Map[key]
		if ok {
			m[key] = value
		}
	}
	return m
}

// SourcePath returns the path to the source dir
func (b *Build) SourcePath() string {
	return b.GuestPath("source", b.options.SourceDir)
}

// HostPath returns a path relative to the build root on the host.
func (b *Build) HostPath(s ...string) string {
	hostPath := path.Join(b.options.BuildDir, b.options.BuildID)
	for _, v := range s {
		hostPath = path.Join(hostPath, v)
	}
	return hostPath
}

// GuestPath returns a path relative to the build root on the guest.
func (b *Build) GuestPath(s ...string) string {
	guestPath := b.options.GuestRoot
	for _, v := range s {
		guestPath = path.Join(guestPath, v)
	}
	return guestPath
}

// MntPath returns a path relative to the read-only mount root on the guest.
func (b *Build) MntPath(s ...string) string {
	mntPath := b.options.MntRoot
	for _, v := range s {
		mntPath = path.Join(mntPath, v)
	}
	return mntPath
}

// ReportPath returns a path relative to the report root on the guest.
func (b *Build) ReportPath(s ...string) string {
	reportPath := b.options.ReportRoot
	for _, v := range s {
		reportPath = path.Join(reportPath, v)
	}
	return reportPath
}

// SetupGuest ensures that the guest is prepared to run the pipeline.
func (b *Build) SetupGuest(sess *Session) error {
	// Make sure our guest path exists
	exit, _, err := sess.SendChecked(fmt.Sprintf(`mkdir "%s"`, b.GuestPath()))
	if err != nil {
		return err
	}
	if exit != 0 {
		return errors.New("Guest command failed.")
	}

	// And the cache path
	exit, _, err = sess.SendChecked(fmt.Sprintf(`mkdir "%s"`, "/cache"))
	if err != nil {
		return err
	}
	if exit != 0 {
		return errors.New("Guest command failed.")
	}

	// Copy the source dir to the guest path
	exit, _, err = sess.SendChecked(fmt.Sprintf(`cp -r "%s" "%s"`, b.MntPath("source"), b.GuestPath("source")))
	if err != nil {
		return err
	}
	if exit != 0 {
		return errors.New("Guest command failed.")
	}

	return nil
}
