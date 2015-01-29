package main

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"code.google.com/p/go-uuid/uuid"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
)

var (
	globalFlags   = flagsFor(GlobalFlags)
	pipelineFlags = flagsFor(PipelineFlags, WerckerInternalFlags)
	emptyFlags    = []cli.Flag{}
)

func emptyEnv() *Environment {
	return NewEnvironment([]string{})
}

func run(t *testing.T, gFlags []cli.Flag, cFlags []cli.Flag, action func(c *cli.Context), args []string) {
	log.SetLevel(log.DebugLevel)
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
		"--registry", "example.com:3000",
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
		assert.Equal(t, "example.com:3000", opts.Registry)
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

func TestOptionsEmptyBuildOptionsEmptyDir(t *testing.T) {
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
		opts, err := NewBuildOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.NotEqual(t, "", opts.BuildID)
		assert.Equal(t, opts.BuildID, opts.PipelineID)
		assert.Equal(t, "", opts.DeployID)
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

func TestOptionsBuildOptions(t *testing.T) {
	args := defaultArgs("--build-id", "fake-build-id")
	test := func(c *cli.Context) {
		opts, err := NewBuildOptions(c, emptyEnv())
		assert.Nil(t, err)
		assert.Equal(t, "fake-build-id", opts.PipelineID)
		assert.Equal(t, "fake-build-id", opts.BuildID)
	}
	run(t, globalFlags, pipelineFlags, test, args)
}
