package main

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"code.google.com/p/go-uuid/uuid"
	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
)

var (
	globalFlags   = flagsFor(GlobalFlags)
	pipelineFlags = flagsFor(PipelineFlags, WerckerInternalFlags)
	emptyFlags    = []cli.Flag{}
)

func clearTheEnvironment() {
	env := os.Environ()
	for _, x := range env {
		parts := strings.Split(x, "=")
		_ = os.Unsetenv(parts[0])
	}
}

func emptyEnv() *Environment {
	return NewEnvironment([]string{})
}

func run(t *testing.T, gFlags []cli.Flag, cFlags []cli.Flag, action func(c *cli.Context), args []string) {
	rootLogger.SetLevel("debug")
	clearTheEnvironment()
	app := cli.NewApp()
	app.Flags = gFlags
	app.Commands = []cli.Command{
		{
			Name:      "test",
			ShortName: "t",
			Usage:     "test command",
			Action:    action,
			Flags:     cFlags,
		},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		t.Fatalf("Command not found: %s", command)
	}
	app.Action = func(c *cli.Context) {
		t.Fatal("No command specified")
	}
	app.Run(args)
}

func defaultArgs(more ...string) []string {
	args := []string{
		"wercker",
		"--debug",
		"--wercker-endpoint", "http://example.com/wercker-endpoint",
		"--base-url", "http://example.com/base-url",
		"--auth-token", "test-token",
		"--auth-token-store", "/tmp/.wercker/test-token",
		"test",
	}
	return append(args, more...)
}

func TestOptionsGlobalOptions(t *testing.T) {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := NewGlobalOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, true, opts.Debug)
		assert.Equal(t, "http://example.com/wercker-endpoint", opts.WerckerEndpoint)
		assert.Equal(t, "http://example.com/base-url", opts.BaseURL)
		assert.Equal(t, "test-token", opts.AuthToken)
		assert.Equal(t, "/tmp/.wercker/test-token", opts.AuthTokenStore)
	}
	run(t, globalFlags, emptyFlags, test, args)
}

func TestOptionsGuessAuthToken(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "test-auth-token")
	assert.Nil(t, err)

	token := uuid.NewRandom().String()
	_, err = tmpFile.Write([]byte(token))
	assert.Nil(t, err)

	tokenStore := tmpFile.Name()
	defer os.Remove(tokenStore)
	defer tmpFile.Close()

	args := []string{
		"wercker",
		"--auth-token-store", tokenStore,
		"test",
	}

	test := func(c *cli.Context) {
		opts, err := NewGlobalOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, token, opts.AuthToken)
	}

	run(t, globalFlags, emptyFlags, test, args)
}

func TestOptionsEmptyPipelineOptionsEmptyDir(t *testing.T) {
	setup(t)
	tmpDir, err := ioutil.TempDir("", "empty-directory")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	basename := filepath.Base(tmpDir)
	currentUser, err := user.Current()
	assert.Nil(t, err)
	username := currentUser.Username

	// Target the path
	args := defaultArgs(tmpDir)
	test := func(c *cli.Context) {
		opts, err := NewPipelineOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, basename, opts.ApplicationID)
		assert.Equal(t, basename, opts.ApplicationName)
		assert.Equal(t, username, opts.ApplicationOwnerName)
		assert.Equal(t, username, opts.ApplicationStartedByName)
		assert.Equal(t, tmpDir, opts.ProjectPath)
		assert.Equal(t, basename, opts.ProjectID)
		// Pretty much all the git stuff should be empty
		assert.Equal(t, "", opts.GitBranch)
		assert.Equal(t, "", opts.GitCommit)
		assert.Equal(t, "", opts.GitDomain)
		assert.Equal(t, username, opts.GitOwner)
		assert.Equal(t, "", opts.GitRepository)
		dumpOptions(opts)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsEmptyBuildOptions(t *testing.T) {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := NewBuildOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.NotEqual(t, "", opts.BuildID)
		assert.Equal(t, opts.BuildID, opts.PipelineID)
		assert.Equal(t, "", opts.DeployID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsBuildOptions(t *testing.T) {
	args := defaultArgs("--build-id", "fake-build-id")
	test := func(c *cli.Context) {
		opts, err := NewBuildOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, "fake-build-id", opts.PipelineID)
		assert.Equal(t, "fake-build-id", opts.BuildID)
		assert.Equal(t, "", opts.DeployID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsEmptyDeployOptions(t *testing.T) {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := NewDeployOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.NotEqual(t, "", opts.DeployID)
		assert.Equal(t, opts.DeployID, opts.PipelineID)
		assert.Equal(t, "", opts.BuildID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsDeployOptions(t *testing.T) {
	args := defaultArgs("--deploy-id", "fake-deploy-id")
	test := func(c *cli.Context) {
		opts, err := NewDeployOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, "fake-deploy-id", opts.PipelineID)
		assert.Equal(t, "fake-deploy-id", opts.DeployID)
		assert.Equal(t, "", opts.BuildID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsKeenOptions(t *testing.T) {
	args := defaultArgs(
		"--keen-metrics",
		"--keen-project-id", "test-id",
		"--keen-project-write-key", "test-key",
	)
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := NewGlobalOptions(c, e)
		opts, err := NewKeenOptions(c, e, gOpts)
		assert.Nil(t, err)
		assert.Equal(t, true, opts.ShouldKeenMetrics)
		assert.Equal(t, "test-key", opts.KeenProjectWriteKey)
		assert.Equal(t, "test-id", opts.KeenProjectID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsKeenMissingOptions(t *testing.T) {
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := NewGlobalOptions(c, e)
		_, err = NewKeenOptions(c, e, gOpts)
		assert.NotNil(t, err)
	}

	missingID := defaultArgs(
		"--keen-metrics",
		"--keen-project-write-key", "test-key",
	)

	missingKey := defaultArgs(
		"--keen-metrics",
		"--keen-project-id", "test-id",
	)

	run(t, globalFlags, keenFlags, test, missingID)
	run(t, globalFlags, keenFlags, test, missingKey)
}

func TestOptionsReporterOptions(t *testing.T) {
	args := defaultArgs(
		"--report",
		"--wercker-host", "http://example.com/wercker-host",
		"--wercker-token", "test-token",
	)
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := NewGlobalOptions(c, e)
		opts, err := NewReporterOptions(c, e, gOpts)
		assert.Nil(t, err)
		assert.Equal(t, true, opts.ShouldReport)
		assert.Equal(t, "http://example.com/wercker-host", opts.ReporterHost)
		assert.Equal(t, "test-token", opts.ReporterKey)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}

func TestOptionsReporterMissingOptions(t *testing.T) {
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := NewGlobalOptions(c, e)
		_, err = NewReporterOptions(c, e, gOpts)
		assert.NotNil(t, err)
	}

	missingHost := defaultArgs(
		"--report",
		"--wercker-token", "test-token",
	)

	missingKey := defaultArgs(
		"--report",
		"--wercker-host", "http://example.com/wercker-host",
	)

	run(t, globalFlags, reporterFlags, test, missingHost)
	run(t, globalFlags, reporterFlags, test, missingKey)
}

func TestOptionsTagEscaping(t *testing.T) {
	args := defaultArgs("--tag", "feature/foo")
	test := func(c *cli.Context) {
		opts, err := NewPipelineOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, "feature_foo", opts.Tag)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}
