package sentcli

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/sentcli/util"
)

func defaultPipelineOptions(s *util.TestSuite, more ...string) *PipelineOptions {
	args := []string{
		"wercker",
		"--debug",
		"test",
		"--working-dir",
		s.WorkingDir(),
	}
	args = append(args, more...)
	os.Clearenv()

	var options *PipelineOptions

	action := func(c *cli.Context) {
		opts, err := NewPipelineOptions(c, emptyEnv())
		s.Nil(err)
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

type StepSuite struct {
	*util.TestSuite
}

func TestStepSuite(t *testing.T) {
	suiteTester := &StepSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *StepSuite) TestFetchApi() {
	options := defaultPipelineOptions(s.TestSuite)

	cfg := &StepConfig{
		ID:   "create-file",
		Data: map[string]string{"filename": "foo.txt", "content": "bar"},
	}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}

func (s *StepSuite) TestFetchTar() {
	options := defaultPipelineOptions(s.TestSuite)

	werckerInit := `wercker-init "https://github.com/wercker/wercker-init/archive/v1.0.0.tar.gz"`
	cfg := &StepConfig{ID: werckerInit, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}

func (s *StepSuite) TestFetchFileNoDev() {
	options := defaultPipelineOptions(s.TestSuite)

	tmpdir, err := ioutil.TempDir("", "sentcli")
	s.Nil(err)
	defer os.RemoveAll(tmpdir)

	fileStep := fmt.Sprintf(`foo "file:///%s"`, tmpdir)
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.NotNil(err)
}

func (s *StepSuite) TestFetchFileDev() {
	options := defaultPipelineOptions(s.TestSuite, "--enable-dev-steps")

	tmpdir, err := ioutil.TempDir("", "sentcli")
	s.Nil(err)
	defer os.RemoveAll(tmpdir)

	fileStep := fmt.Sprintf(`foo "file:///%s"`, tmpdir)
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}
