package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/termie/go-shutil"
	"os"
	"os/signal"
	"path/filepath"
)

// Runner is the base type for running the pipelines
type Runner struct {
	options       *GlobalOptions
	emitter       *emission.Emitter
	logger        *LogHandler
	literalLogger *LiteralLogHandler
	metrics       *MetricsEventHandler
	reporter      *ReportHandler
}

// NewRunner from global options
func NewRunner(options *GlobalOptions) *Runner {
	e := GetEmitter()

	h, err := NewLogHandler()
	if err != nil {
		log.WithField("Error", err).Panic("Unable to LogHandler")
	}
	h.ListenTo(e)

	l, err := NewLiteralLogHandler()
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
		options:       options,
		emitter:       e,
		logger:        h,
		literalLogger: l,
		metrics:       mh,
		reporter:      r,
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
// TODO(termie): This may end up being BuildRunner only,
// if we split that off
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
func (p *Runner) GetConfig() (*RawConfig, error) {
	// Return a []byte of the yaml we find or create.
	werckerYaml, err := ReadWerckerYaml([]string{p.ProjectDir()}, false)
	if err != nil {
		return nil, err
	}

	// Parse that bad boy.
	rawConfig, err := ConfigFromYaml(werckerYaml)
	if err != nil {
		return nil, err
	}

	// Add some options to the global config
	if rawConfig.SourceDir != "" {
		p.options.SourceDir = rawConfig.SourceDir
	}

	return rawConfig, nil
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

// GetSession returns a read-write connection to a container
func (p *Runner) GetSession(containerID string) (*Session, error) {
	sess := NewSession(p.options.DockerHost, containerID)
	sess, err := sess.Attach()
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// BuildRunner is the runner type for a Build pipeline
type BuildRunner struct {
	*Runner
}

// GetPipeline returns a pipeline based on the "build" config section
func (b *BuildRunner) GetPipeline(rawConfig *RawConfig) (*Build, error) {
	// Promote the RawBuild to a real Build. We believe in you, Build!
	build, err := rawConfig.RawBuild.ToBuild(b.options)
	if err != nil {
		return nil, err
	}
	return build, nil
}

type RunnerContext struct {
	box      *Box
	pipeline *Build
	sess     *Session
	config   *RawConfig
}

// SetupEnvironment does a lot of boilerplate legwork and returns a pipeline,
// box, and session. This is a bit of a long method, but it is pretty much
// the entire "Setup Environment" step.
func (b *BuildRunner) SetupEnvironment() (*RunnerContext, error) {
	ctx := &RunnerContext{}

	log.Println("Application:", b.options.ApplicationName)

	// Grab our config
	rawConfig, err := b.GetConfig()
	if err != nil {
		return ctx, err
	}
	ctx.config = rawConfig

	box, err := b.GetBox(rawConfig)
	if err != nil {
		return ctx, err
	}
	ctx.box = box

	err = b.AddServices(rawConfig, box)
	if err != nil {
		return ctx, err
	}

	// Start setting up the pipeline dir
	err = b.CopySource()
	if err != nil {
		return ctx, err
	}

	pipeline, err := b.GetPipeline(rawConfig)
	ctx.pipeline = pipeline

	log.Println("Steps:", len(pipeline.Steps))

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

	// Start our session
	sess, err := b.GetSession(container.ID)
	if err != nil {
		return ctx, err
	}
	ctx.sess = sess

	// Some helpful logging
	pipeline.logEnvironment()

	err = pipeline.SetupGuest(sess)
	if err != nil {
		return ctx, err
	}

	err = pipeline.ExportEnvironment(sess)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}
