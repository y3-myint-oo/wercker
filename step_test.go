package main

import (
	"os"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
)

func defaultPipelineOptions(t *testing.T, more ...string) *PipelineOptions {
	args := []string{
		"wercker",
		"--debug",
		"test",
	}
	args = append(args, more...)
	os.Clearenv()

	var options *PipelineOptions

	action := func(c *cli.Context) {
		opts, err := NewPipelineOptions(c, emptyEnv())
		assert.Nil(t, err)
		options = opts
	}

	app := cli.NewApp()
	app.Flags = globalFlags
	app.Commands = []cli.Command{
		{
			Name:   "test",
			Action: action,
			Flags:  pipelineFlags,
		},
	}
	app.Run(args)
	return options
}

func TestStepFetchApi(t *testing.T) {
	options := defaultPipelineOptions(t)

	cfg := &StepConfig{
		ID:   "create-file",
		Data: map[string]string{"filename": "foo.txt", "content": "bar"},
	}

	step, err := NewStep(cfg, options)
	assert.Nil(t, err)
	_, err = step.Fetch()
	assert.Nil(t, err)
}

func TestStepFetchTar(t *testing.T) {
	options := defaultPipelineOptions(t)

	werckerInit := `wercker-init "https://github.com/wercker/wercker-init/archive/v1.0.0.tar.gz"`
	cfg := &StepConfig{ID: werckerInit, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	assert.Nil(t, err)
	_, err = step.Fetch()
	assert.Nil(t, err)
}

func TestStepFetchFileNoDev(t *testing.T) {
	options := defaultPipelineOptions(t)

	fileStep := `foo "file:///etc/"`
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	assert.Nil(t, err)
	_, err = step.Fetch()
	assert.NotNil(t, err)
}

func TestStepFetchFileDev(t *testing.T) {
	options := defaultPipelineOptions(t, "--enable-dev-steps")

	fileStep := `foo "file:///etc/"`
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	assert.Nil(t, err)
	_, err = step.Fetch()
	assert.Nil(t, err)
}
