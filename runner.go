package main

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/termie/go-shutil"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
)

// GetPipeline is a function that will fetch the appropriate pipeline
// object from the rawConfig.
type GetPipeline func(*RawConfig, *GlobalOptions) (Pipeline, error)

// GetBuildPipeline grabs the "build" section of the yaml.
func GetBuildPipeline(rawConfig *RawConfig, options *GlobalOptions) (Pipeline, error) {
	build, err := rawConfig.RawBuild.ToBuild(options)
	if err != nil {
		return nil, err
	}
	return build, nil
}

// GetDeployPipeline gets the "deploy" section of the yaml.
func GetDeployPipeline(rawConfig *RawConfig, options *GlobalOptions) (Pipeline, error) {
	build, err := rawConfig.RawDeploy.ToDeploy(options)
	if err != nil {
		return nil, err
	}
	return build, nil
}

// Runner is the base type for running the pipelines.
type Runner struct {
	options        *GlobalOptions
	emitter        *emission.Emitter
	logger         *LogHandler
	literalLogger  *LiteralLogHandler
	metrics        *MetricsEventHandler
	reporter       *ReportHandler
	pipelineGetter GetPipeline
}

// NewRunner from global options
func NewRunner(options *GlobalOptions, pipelineGetter GetPipeline) *Runner {

	e := GetEmitter()

	h, err := NewLogHandler()
	if err != nil {
		log.WithField("Error", err).Panic("Unable to LogHandler")
	}
	h.ListenTo(e)

	l, err := NewLiteralLogHandler(options)
	if err != nil {
		log.WithField("Error", err).Panic("Unable to LiteralLogHandler")
	}
	l.ListenTo(e)

	var mh *MetricsEventHandler
	if options.ShouldKeenMetrics {
		mh, err = NewMetricsHandler(options)
		if err != nil {
			log.WithField("Error", err).Panic("Unable to MetricsHandler")
		}
		mh.ListenTo(e)
	}

	var r *ReportHandler
	if options.ShouldReport {
		r, err := NewReportHandler(options.WerckerHost, options.WerckerToken)
		if err != nil {
			log.WithField("Error", err).Panic("Unable to ReportHandler")
		}
		r.ListenTo(e)
	}

	return &Runner{
		options:        options,
		emitter:        e,
		logger:         h,
		literalLogger:  l,
		metrics:        mh,
		reporter:       r,
		pipelineGetter: pipelineGetter,
	}
}

// Emitter shares the Runner's emitter.
func (p *Runner) Emitter() *emission.Emitter {
	return p.emitter
}

// ProjectDir returns the directory where we expect to find the code for this project
func (p *Runner) ProjectDir() string {
	return fmt.Sprintf("%s/%s", p.options.ProjectDir, p.options.ApplicationID)
}

// EnsureCode makes sure the code is in the ProjectDir.
// NOTE(termie): When launched by kiddie-pool the ProjectPath will be
// set to the location where grappler checked out the code and the copy
// will be a little superfluous, but in the case where this is being
// run in Single Player Mode this copy is necessary to avoid screwing
// with the local dir.
func (p *Runner) EnsureCode() (string, error) {
	projectDir := p.ProjectDir()

	// If the target is a tarball feetch and build that
	if p.options.ProjectURL != "" {
		resp, err := fetchTarball(p.options.ProjectURL)
		if err != nil {
			return projectDir, err
		}
		err = untargzip(projectDir, resp.Body)
		if err != nil {
			return projectDir, err
		}
	} else {
		// We were pointed at a path with ProjectPath, copy it to projectDir

		// Make sure we don't accidentally recurse or copy extra files
		ignoreFunc := func(src string, files []os.FileInfo) []string {
			ignores := []string{}
			for _, file := range files {
				abspath, err := filepath.Abs(filepath.Join(src, file.Name()))
				if err != nil {
					// Something went sufficiently wrong
					panic(err)
				}
				if abspath == p.options.BuildDir || abspath == p.options.ProjectDir || abspath == p.options.StepDir {
					ignores = append(ignores, file.Name())
				}
				// TODO(termie): maybe ignore .gitignore files?
			}
			return ignores
		}
		copyOpts := &shutil.CopyTreeOptions{Ignore: ignoreFunc, CopyFunction: shutil.Copy}
		os.Rename(projectDir, fmt.Sprintf("%s-%s", projectDir, uuid.NewRandom().String()))
		err := shutil.CopyTree(p.options.ProjectPath, projectDir, copyOpts)
		if err != nil {
			return projectDir, err
		}
	}
	return projectDir, nil
}

// GetConfig parses and returns the wercker.yml file.
func (p *Runner) GetConfig() (*RawConfig, string, error) {
	// Return a []byte of the yaml we find or create.
	var werckerYaml []byte
	var err error
	if p.options.WerckerYml != "" {
		werckerYaml, err = ioutil.ReadFile(p.options.WerckerYml)
		if err != nil {
			return nil, "", err
		}
	} else {
		werckerYaml, err = ReadWerckerYaml([]string{p.ProjectDir()}, false)
		if err != nil {
			return nil, "", err
		}
	}

	// Parse that bad boy.
	rawConfig, err := ConfigFromYaml(werckerYaml)
	if err != nil {
		return nil, "", err
	}

	// Add some options to the global config
	if rawConfig.SourceDir != "" {
		p.options.SourceDir = rawConfig.SourceDir
	}

	return rawConfig, string(werckerYaml), nil
}

// GetBox fetches and returns the base box for the pipeline.
func (p *Runner) GetBox(rawConfig *RawConfig) (*Box, error) {
	// Promote RawBox to a real Box. We believe in you, Box!
	box, err := rawConfig.RawBox.ToBox(p.options, nil)
	if err != nil {
		return nil, err
	}

	log.Println("Box:", box.Name)

	// Make sure we have the box available
	image, err := box.Fetch()
	if err != nil {
		return nil, err
	}
	log.Println("Docker Image:", image.ID)
	return box, nil
}

// AddServices fetches and links the services to the base box.
func (p *Runner) AddServices(rawConfig *RawConfig, box *Box) error {
	for _, rawService := range rawConfig.RawServices {
		log.Println("Fetching service:", rawService)

		serviceBox, err := rawService.ToServiceBox(p.options, nil)
		if err != nil {
			return err
		}

		if _, err := serviceBox.Box.Fetch(); err != nil {
			return err
		}

		box.AddService(serviceBox)
		// TODO(mh): We want to make sure container is running fully before
		// allowing build steps to run. We may need custom steps which block
		// until service services are running.
	}
	return nil
}

// CopySource copies the source into the HostPath
func (p *Runner) CopySource() error {
	// Start setting up the pipeline dir
	err := os.MkdirAll(p.options.HostPath(), 0755)
	if err != nil {
		return err
	}

	err = shutil.CopyTree(p.ProjectDir(), p.options.HostPath("source"), nil)
	if err != nil {
		return err
	}
	return nil
}

// GetSession attaches to the container and returns a session.
func (p *Runner) GetSession(containerID string) (*Session, error) {
	sess, err := NewSession(p.options, containerID)
	if err != nil {
		return nil, err
	}
	err = sess.Attach()
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// GetPipeline returns a pipeline based on the "build" config section
func (p *Runner) GetPipeline(rawConfig *RawConfig) (Pipeline, error) {
	return p.pipelineGetter(rawConfig, p.options)
}

// RunnerContext holds on to the information we got from setting up our
// environment.
type RunnerContext struct {
	box      *Box
	pipeline Pipeline
	sess     *Session
	config   *RawConfig
}

// StartStep emits BuildStepStarted and returns a Finisher for the end event.
func (p *Runner) StartStep(ctx *RunnerContext, step *Step, order int) *Finisher {
	p.emitter.Emit(BuildStepStarted, &BuildStepStartedArgs{
		Build:   ctx.pipeline,
		Options: p.options,
		Step:    step,
		Order:   order,
	})
	return NewFinisher(func(result interface{}) {
		r := result.(*StepResult)
		artifactURL := ""
		if r.Artifact != nil {
			artifactURL = r.Artifact.URL()
		}
		p.emitter.Emit(BuildStepFinished, &BuildStepFinishedArgs{
			Build:               ctx.pipeline,
			Options:             p.options,
			Step:                step,
			Order:               order,
			Successful:          r.Success,
			Message:             r.Message,
			ArtifactURL:         artifactURL,
			PackageURL:          r.PackageURL,
			WerckerYamlContents: r.WerckerYamlContents,
		})
	})
}

// StartBuild emits a BuildStarted and returns for a Finisher for the end.
func (p *Runner) StartBuild(options *GlobalOptions) *Finisher {
	p.emitter.Emit(BuildStarted, &BuildStartedArgs{Options: options})
	return NewFinisher(func(result interface{}) {
		r := result.(bool)
		msg := "failed"
		if r {
			msg = "passed"
		}
		p.emitter.Emit(BuildFinished, &BuildFinishedArgs{
			Options: options,
			Result:  msg,
		})
	})
}

// SetupEnvironment does a lot of boilerplate legwork and returns a pipeline,
// box, and session. This is a bit of a long method, but it is pretty much
// the entire "Setup Environment" step.
func (p *Runner) SetupEnvironment() (*RunnerContext, error) {
	ctx := &RunnerContext{}

	sr := &StepResult{
		Success:  false,
		Artifact: nil,
		Message:  "",
		ExitCode: 1,
	}

	setupEnvironmentStep := &Step{Name: "setup environment"}
	finisher := p.StartStep(ctx, setupEnvironmentStep, 2)
	defer finisher.Finish(sr)

	log.Println("Application:", p.options.ApplicationName)

	// Grab our config
	rawConfig, stringConfig, err := p.GetConfig()
	if err != nil {
		return ctx, err
	}
	ctx.config = rawConfig
	sr.WerckerYamlContents = stringConfig

	box, err := p.GetBox(rawConfig)
	if err != nil {
		return ctx, err
	}
	ctx.box = box

	err = p.AddServices(rawConfig, box)
	if err != nil {
		return ctx, err
	}

	// Start setting up the pipeline dir
	log.Debugln("Copying source to build directory")
	err = p.CopySource()
	if err != nil {
		return ctx, err
	}

	pipeline, err := p.GetPipeline(rawConfig)
	ctx.pipeline = pipeline

	log.Println("Steps:", len(pipeline.Steps()))

	// Make sure we have the steps
	err = pipeline.FetchSteps()
	if err != nil {
		return ctx, err
	}

	// Start booting up our services
	// TODO(termie): maybe move this into box.Run?
	err = box.RunServices()
	if err != nil {
		return ctx, err
	}

	// Boot up our main container
	container, err := box.Run()
	if err != nil {
		return ctx, err
	}

	// Register our signal handler to clean the box up
	// TODO(termie): we should probably make a little general purpose signal
	// handler and register callbacks with it so that multiple parts of the app
	// can do cleanup
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	go func() {
		tries := 0
		for _ = range sigint {
			if tries == 0 {
				tries = 1
				box.Stop()
				os.Exit(1)
			} else {
				panic("Exiting forcefully")
			}
		}
	}()

	log.Debugln("Attaching session to base box")
	// Start our session
	sess, err := p.GetSession(container.ID)
	if err != nil {
		return ctx, err
	}
	ctx.sess = sess

	// Some helpful logging
	pipeline.LogEnvironment()

	log.Debugln("Setting up guest (base box)")
	err = pipeline.SetupGuest(sess)
	if err != nil {
		return ctx, err
	}

	err = pipeline.ExportEnvironment(sess)
	if err != nil {
		return ctx, err
	}

	sr.Success = true
	sr.ExitCode = 0
	return ctx, nil
}

// StepResult holds the info we need to report on steps
type StepResult struct {
	Success             bool
	Artifact            *Artifact
	PackageURL          string
	Message             string
	ExitCode            int
	WerckerYamlContents string
}

// RunStep runs a step and tosses error if it fails
func (p *Runner) RunStep(ctx *RunnerContext, step *Step, order int) (*StepResult, error) {
	finisher := p.StartStep(ctx, step, order)
	sr := &StepResult{
		Success:  false,
		Artifact: nil,
		Message:  "",
		ExitCode: 1,
	}
	defer finisher.Finish(sr)

	step.InitEnv()
	log.Println("Step Environment")
	for _, pair := range step.Env.Ordered() {
		log.Println(" ", pair[0], pair[1])
	}

	exit, err := step.Execute(ctx.sess)
	if exit != 0 {
		sr.ExitCode = exit
	} else if err != nil {
		return sr, err
	} else {
		sr.Success = true
		sr.ExitCode = 0
	}

	// Grab the message
	var message bytes.Buffer
	err = step.CollectFile(ctx.sess, step.ReportPath(), "message.txt", &message)
	if err != nil {
		if err != ErrEmptyTarball {
			return sr, err
		}
	}
	sr.Message = message.String()

	// Grab artifacts if we want them
	if p.options.ShouldArtifacts {
		artifact, err := step.CollectArtifact(ctx.sess)
		if err != nil {
			return sr, err
		}

		if artifact != nil {
			artificer := NewArtificer(p.options)
			err = artificer.Upload(artifact)
			if err != nil {
				return sr, err
			}
		}
		sr.Artifact = artifact
	}

	if !sr.Success {
		return sr, fmt.Errorf("Step failed with exit code: %d", sr.ExitCode)
	}
	return sr, nil
}
