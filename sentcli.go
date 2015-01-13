package main

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/codegangsta/cli"
	"github.com/termie/go-shutil"
	"os"
	"os/signal"
	"path/filepath"
)

func main() {
	log.SetLevel(log.DebugLevel)

	app := cli.NewApp()
	app.Version = Version
	app.Flags = []cli.Flag{
		// These flags control where we store local files
		cli.StringFlag{Name: "project-dir", Value: "./_projects", Usage: "path where downloaded projects live"},
		cli.StringFlag{Name: "step-dir", Value: "./_steps", Usage: "path where downloaded steps live"},
		cli.StringFlag{Name: "build-dir", Value: "./_builds", Usage: "path where created builds live"},

		// These flags tell us where to go for operations
		cli.StringFlag{Name: "docker-host", Value: "tcp://127.0.0.1:2375", Usage: "docker api host", EnvVar: "DOCKER_HOST"},
		cli.StringFlag{Name: "wercker-endpoint", Value: "https://app.wercker.com/api/v2", Usage: "wercker api endpoint"},
		cli.StringFlag{Name: "base-url", Value: "https://app.wercker.com/", Usage: "base url for the web app"},
		cli.StringFlag{Name: "registry", Value: "127.0.0.1:3000", Usage: "registry endpoint to push images to"},

		// These flags control paths on the guest and probably shouldn't change
		cli.StringFlag{Name: "mnt-root", Value: "/mnt", Usage: "directory on the guest where volumes are mounted"},
		cli.StringFlag{Name: "guest-root", Value: "/pipeline", Usage: "directory on the guest where work is done"},
		cli.StringFlag{Name: "report-root", Value: "/report", Usage: "directory on the guest where reports will be written"},

		// These flags are usually pulled from the env
		cli.StringFlag{Name: "build-id", Value: "", Usage: "build id", EnvVar: "WERCKER_BUILD_ID"},
		cli.StringFlag{Name: "application-id", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_ID"},
		cli.StringFlag{Name: "application-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_NAME"},
		cli.StringFlag{Name: "application-owner-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_OWNER_NAME"},
		cli.StringFlag{Name: "application-started-by-name", Value: "", Usage: "application started by", EnvVar: "WERCKER_APPLICATION_STARTED_BY_NAME"},

		// Should we push finished builds to the registry?
		cli.BoolFlag{Name: "push", Usage: "push the build result to registry"},
		cli.BoolFlag{Name: "commit", Usage: "commit the build result locally"},
		cli.StringFlag{Name: "tag", Value: "", Usage: "tag for this build", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "message", Value: "", Usage: "message for this build"},

		// AWS bits
		cli.StringFlag{Name: "aws-secret-key", Value: "", Usage: "secret access key"},
		cli.StringFlag{Name: "aws-access-key", Value: "", Usage: "access key id"},
		cli.StringFlag{Name: "s3-bucket", Value: "wercker-development", Usage: "bucket for artifacts"},
		cli.StringFlag{Name: "aws-region", Value: "us-east-1", Usage: "region"},

		// keen.io bits
		cli.BoolFlag{Name: "keen-metrics", Usage: "report metrics to keen.io"},
		cli.StringFlag{Name: "keen-project-write-key", Value: "", Usage: "keen write key"},
		cli.StringFlag{Name: "keen-project-id", Value: "", Usage: "keen project id"},

		// Reporter settings
		cli.BoolFlag{Name: "report", Usage: "Report logs back to wercker (requires build-id, wercker-host, wercker-token)"},
		cli.StringFlag{Name: "wercker-host", Usage: "Wercker host to use for wercker reporter"},
		cli.StringFlag{Name: "wercker-token", Usage: "Wercker token to use for wercker reporter"},

		// These options might be overwritten by the wercker.yml
		cli.StringFlag{Name: "source-dir", Value: "", Usage: "source path relative to checkout root"},
		cli.IntFlag{Name: "no-response-timeout", Value: 5, Usage: "timeout if no script output is received in this many minutes"},
		cli.IntFlag{Name: "command-timeout", Value: 10, Usage: "timeout if command does not complete in this many minutes"},
	}

	app.Commands = []cli.Command{
		{
			Name:      "build",
			ShortName: "b",
			Usage:     "build a project",
			Action: func(c *cli.Context) {
				buildProject(c)
			},
			Flags: []cli.Flag{},
		},
		{
			Name:      "version",
			ShortName: "v",
			Usage:     "display version information",
			Action: func(c *cli.Context) {
				options, err := createVersionOptions(c)
				if err != nil {
					log.Panicln(err)
				}

				displayVersion(options)
			},
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "json",
					Usage: "Output version information as JSON",
				},
			},
		},
	}
	app.Run(os.Args)
}

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

func buildProject(c *cli.Context) {
	// Parse CLI and local env
	options, err := NewGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

	// Build our common pipeline
	p := NewRunner(options)
	b := &BuildRunner{p}
	e := p.Emitter()

	e.Emit(BuildStarted, &BuildStartedArgs{Options: options})

	// This will be emitted at the end of the execution, we're going to be
	// pessimistic and report that we failed, unless overridden at the end of the
	// execution.
	// TODO(bvdberg): This is good for now, but we should be able to report
	// halfway through (when the build finishes, but after steps have not yet run)
	buildFinishedArgs := &BuildFinishedArgs{Options: options, Result: "failed"}
	defer e.Emit(BuildFinished, buildFinishedArgs)

	log.Println("############# Building project #############")
	log.Debugln(fmt.Sprintf("%+v", options))
	log.Println(options.ApplicationName)
	log.Println("############################################")

	projectDir, err := p.EnsureCode()
	if err != nil {
		log.Panicln(err)
	}

	setupEnvironmentStep := &Step{Name: "setup environment"}
	e.Emit(BuildStepStarted, &BuildStepStartedArgs{
		Options: options,
		Step:    setupEnvironmentStep,
		Order:   2,
	})

	log.Println("Application:", options.ApplicationName)
	// Grab our config
	rawConfig, err := p.GetConfig()
	if err != nil {
		log.Panicln(err)
	}

	box, err := p.GetBox(rawConfig)
	if err != nil {
		log.Panicln(err)
	}

	err = p.AddServices(rawConfig, box)
	if err != nil {
		log.Panicln(err)
	}

	pipeline, err := b.GetPipeline(rawConfig)

	log.Println("Steps:", len(pipeline.Steps))

	// Start setting up the pipeline dir
	err = os.MkdirAll(options.HostPath(), 0755)
	if err != nil {
		log.Panicln(err)
	}

	err = shutil.CopyTree(projectDir, options.HostPath("source"), nil)
	if err != nil {
		log.Panicln(err)
	}

	// Make sure we have the steps
	for _, step := range pipeline.Steps {
		log.Println("Fetching Step:", step.Name, step.ID)
		if _, err := step.Fetch(); err != nil {
			log.Panicln(err)
		}
	}

	err = box.RunServices()
	if err != nil {
		log.Panicln(err)
	}

	// TODO(termie): can we remove the reliance on pipeline here?
	container, err := box.Run()
	if err != nil {
		log.Panicln(err)
	}
	defer box.Stop()

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
	sess := NewSession(options.DockerHost, container.ID)
	sess, err = sess.Attach()
	if err != nil {
		log.Fatalln(err)
	}

	// Some helpful logging
	log.Println("Base Build Environment:")
	for _, pair := range pipeline.Env.Ordered() {
		log.Println(" ", pair[0], pair[1])
	}

	err = pipeline.SetupGuest(sess)
	exit, _, err := sess.SendChecked(pipeline.Env.Export()...)
	if err != nil {
		log.Panicln(err)
	}
	if exit != 0 {
		log.Fatalln("Build failed with exit code:", exit)
	}

	repoName := fmt.Sprintf("%s/%s", options.ApplicationOwnerName, options.ApplicationName)
	tag := options.Tag
	if tag == "" {
		tag = fmt.Sprintf("build-%s", options.BuildID)
	}
	message := options.Message
	if message == "" {
		message = fmt.Sprintf("Build %s", options.BuildID)
	}

	e.Emit(BuildStepFinished, &BuildStepFinishedArgs{
		Build:      pipeline,
		Options:    options,
		Step:       setupEnvironmentStep,
		Order:      2,
		Successful: true,
	})

	// TODO(bvdberg):
	storeStep := &Step{Name: "Store"}
	// Package should be the last item, + "setup environemnt" and "get code"
	storeStepOrder := len(pipeline.Steps) + 1 + 2

	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:     pipeline,
		Steps:     pipeline.Steps,
		StoreStep: storeStep,
		Options:   options,
	})

	stepFailed := false
	offset := 2
	for i, step := range pipeline.Steps {
		log.Println()
		log.Println("============= Executing Step ==============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		e.Emit(BuildStepStarted, &BuildStepStartedArgs{
			Build:   pipeline,
			Step:    step,
			Options: options,
			Order:   offset + i,
		})

		step.InitEnv()
		log.Println("Step Environment")
		for _, pair := range step.Env.Ordered() {
			log.Println(" ", pair[0], pair[1])
		}

		err = func() error {
			// Get ready to report this
			stepArgs := &BuildStepFinishedArgs{
				Build:      pipeline,
				Options:    options,
				Step:       step,
				Order:      offset + i,
				Successful: false,
			}
			defer e.Emit(BuildStepFinished, stepArgs)

			exit, err = step.Execute(sess)
			if exit != 0 {
				box.Stop()
				return fmt.Errorf("Build failed with exit code: %d", exit)
			}
			if err != nil {
				return err
			}
			artifact, err := step.CollectArtifact(sess)
			if err != nil {
				return err
			}

			if artifact != nil {
				artificer := NewArtificer(options)
				err = artificer.Upload(artifact)
				if err != nil {
					return err
				}
			}
			stepArgs.Successful = true
			return nil
		}()

		if err != nil {
			stepFailed = true
			log.Errorln("============== Step failed! ===============")
			break
		}

		log.Println("============== Step passed! ===============")

		if options.ShouldCommit {
			box.Commit(repoName, tag, message)
		}
	}

	if options.ShouldCommit {
		box.Commit(repoName, tag, message)
	}

	if options.ShouldPush {
		e.Emit(BuildStepStarted, &BuildStepStartedArgs{
			Build:   pipeline,
			Step:    storeStep,
			Options: options,
			Order:   storeStepOrder,
		})

		err = func() error {
			// Get ready to report this
			stepArgs := &BuildStepFinishedArgs{
				Build:      pipeline,
				Options:    options,
				Step:       storeStep,
				Order:      storeStepOrder,
				Successful: false,
			}
			defer e.Emit(BuildStepFinished, stepArgs)

			pushOptions := &PushOptions{
				Registry: options.Registry,
				Name:     repoName,
				Tag:      tag,
				Message:  message,
			}

			_, err = box.Push(pushOptions)
			stepArgs.Successful = true
			return err
		}()

		if err != nil {
			log.WithField("Error", err).Error("Unable to push to registry")
		}
	}

	// Only make it passed if we reach this code (ie no panics) and no step
	// failed.
	if !stepFailed {
		buildFinishedArgs.Result = "passed"
	}

	if buildFinishedArgs.Result == "passed" {
		err = func() error {
			artifact, err := pipeline.CollectArtifact(sess)
			if err != nil {
				return err
			}

			artificer := NewArtificer(options)
			err = artificer.Upload(artifact)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			log.WithField("Error", err).Error("Unable to store pipeline output")
			buildFinishedArgs.Result = "failed"
		}
	}

	if buildFinishedArgs.Result == "passed" {
		log.Println("############# Build passed! ###############")
	} else {
		log.Errorln("############# Build failed! ###############")
	}
}

type versions struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
}

func displayVersion(options *VersionOptions) {
	v := &versions{
		Version:   Version,
		GitCommit: GitVersion,
	}

	if options.OutputJSON {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.WithField("Error", err).Panic("Unable to marshal versions")
		}
		os.Stdout.Write(b)
	} else {
		os.Stdout.WriteString(fmt.Sprintf("Version: %s\n", v.Version))
		os.Stdout.WriteString(fmt.Sprintf("Git commit: %s", v.GitCommit))
	}

	os.Stdout.WriteString("\n")

}
