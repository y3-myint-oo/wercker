package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.google.com/p/go-uuid/uuid"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/joho/godotenv"
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
		cli.StringFlag{Name: "deploy-id", Value: "", Usage: "deploy id", EnvVar: "WERCKER_DEPLOY_ID"},
		cli.StringFlag{Name: "application-id", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_ID"},
		cli.StringFlag{Name: "application-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_NAME"},
		cli.StringFlag{Name: "application-owner-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_OWNER_NAME"},
		cli.StringFlag{Name: "application-started-by-name", Value: "", Usage: "application started by", EnvVar: "WERCKER_APPLICATION_STARTED_BY_NAME"},

		// Should we push finished builds to the registry?
		cli.BoolFlag{Name: "push", Usage: "push the build result to registry"},
		cli.BoolFlag{Name: "commit", Usage: "commit the build result locally"},
		cli.StringFlag{Name: "tag", Value: "", Usage: "tag for this build", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "message", Value: "", Usage: "message for this build"},

		// Should we push artifacts
		cli.BoolFlag{Name: "no-artifacts", Usage: "don't upload artifacts"},

		// Load additional environment variables from a file
		cli.StringFlag{Name: "environment", Value: "ENVIRONMENT", Usage: "specify additional environment variables in a file"},

		// Debug controls whether we soft-exit
		cli.BoolFlag{Name: "debug", Usage: "print stack traces on failures"},

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
				envfile := c.GlobalString("environment")
				_ = godotenv.Load(envfile)
				// ensure we have an ID
				id := guessBuildID(c, NewEnvironment(os.Environ()))
				if id == "" {
					_ = os.Setenv("WERCKER_BUILD_ID", uuid.NewRandom().String())
				}
				err := buildProject(c)
				if err != nil {
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{},
		},
		{
			Name:      "deploy",
			ShortName: "d",
			Usage:     "deploy a project",
			Action: func(c *cli.Context) {
				envfile := c.GlobalString("environment")
				_ = godotenv.Load(envfile)
				// ensure we have an ID
				id := guessDeployID(c, NewEnvironment(os.Environ()))
				if id == "" {
					_ = os.Setenv("WERCKER_DEPLOY_ID", uuid.NewRandom().String())
				}
				err := deployProject(c)
				if err != nil {
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{},
		},
		{
			Name:      "detect",
			ShortName: "de",
			Usage:     "detect the type of project",
			Action: func(c *cli.Context) {
				detectProject(c)
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

// SoftExit is a helper for determining when to show stack traces
type SoftExit struct {
	options *GlobalOptions
}

// Exit with either an error or a panic
func (s *SoftExit) Exit(v ...interface{}) error {
	if s.options.Debug {
		// Clearly this will cause it's own exit if it gets called.
		log.Panicln(v...)
	}
	log.Errorln(v...)
	return fmt.Errorf("Exiting.")
}

func deployProject(c *cli.Context) error {
	// Parse CLI and local env
	options, err := NewGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

	soft := &SoftExit{options}

	// Build our common pipeline
	p := NewRunner(options, GetDeployPipeline)
	e := p.Emitter()

	e.Emit(BuildStarted, &BuildStartedArgs{Options: options})

	// This will be emitted at the end of the execution, we're going to be
	// pessimistic and report that we failed, unless overridden at the end of the
	// execution.
	// TODO(bvdberg): This is good for now, but we should be able to report
	// halfway through (when the build finishes, but after steps have not yet run)
	buildFinishedArgs := &BuildFinishedArgs{Options: options, Result: "failed"}
	defer e.Emit(BuildFinished, buildFinishedArgs)

	log.Println("############ Deploying project #############")
	LogOptions(options)
	log.Println("############################################")

	_, err = p.EnsureCode()
	if err != nil {
		soft.Exit(err)
	}

	ctx, err := p.SetupEnvironment()
	if ctx.box != nil {
		defer ctx.box.Stop()
	}
	if err != nil {
		return soft.Exit(err)
	}

	// Expand our context object
	// box := ctx.box
	pipeline := ctx.pipeline
	// sess := ctx.sess

	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:   pipeline,
		Steps:   pipeline.Steps(),
		Options: options,
	})

	stepFailed := false
	offset := 2
	for i, step := range pipeline.Steps() {
		log.Println()
		log.Println("============== Running Step ===============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		err = p.RunStep(ctx, step, offset+i)

		if err != nil {
			stepFailed = true
			log.Warnln("============== Step failed! ===============")
			break
		}
		log.Println("============== Step passed! ===============")
	}

	// Only make it passed if we reach this code (ie no panics) and no step
	// failed.
	if !stepFailed {
		buildFinishedArgs.Result = "passed"
	}

	if buildFinishedArgs.Result == "passed" {
		log.Println("############# Deploy passed! ##############")
	} else {
		log.Warnln("############# Deploy failed! ##############")
		return fmt.Errorf("Build failed.")
	}
	return nil
}

func buildProject(c *cli.Context) error {
	// Parse CLI and local env
	options, err := NewGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

	soft := &SoftExit{options}

	// Build our common pipeline
	p := NewRunner(options, GetBuildPipeline)
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
	LogOptions(options)
	log.Println("############################################")

	_, err = p.EnsureCode()
	if err != nil {
		soft.Exit(err)
	}

	ctx, err := p.SetupEnvironment()
	if ctx.box != nil {
		defer ctx.box.Stop()
	}
	if err != nil {
		return soft.Exit(err)
	}

	// Expand our context object
	box := ctx.box
	pipeline := ctx.pipeline
	sess := ctx.sess

	repoName := pipeline.DockerRepo()
	tag := pipeline.DockerTag()
	message := pipeline.DockerMessage()

	// TODO(bvdberg):
	storeStep := &Step{Name: "Store"}
	// Package should be the last item, + "setup environemnt" and "get code"
	storeStepOrder := len(pipeline.Steps()) + 1 + 2

	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:     pipeline,
		Steps:     pipeline.Steps(),
		StoreStep: storeStep,
		Options:   options,
	})

	stepFailed := false
	offset := 2
	for i, step := range pipeline.Steps() {
		log.Println()
		log.Println("============== Running Step ===============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		err = p.RunStep(ctx, step, offset+i)

		if err != nil {
			stepFailed = true
			log.Warnln("============== Step failed! ===============")
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
		err = func() error {
			finisher := p.StartStep(ctx, storeStep, storeStepOrder)
			defer finisher.Finish(false)

			pushOptions := &PushOptions{
				Registry: options.Registry,
				Name:     repoName,
				Tag:      tag,
				Message:  message,
			}

			_, err = box.Push(pushOptions)
			if err != nil {
				return err
			}
			finisher.Finish(true)
			return nil
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

	if buildFinishedArgs.Result == "passed" && options.ShouldArtifacts {
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
		log.Warnln("############# Build failed! ###############")
		return fmt.Errorf("Build failed.")
	}
	return nil
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

// detectProject inspects the the current directory that sentcli is running in
// and detects the project's programming language
func detectProject(c *cli.Context) {
	// Parse CLI and local env
	options, err := NewGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

	soft := &SoftExit{options}

	log.Println("########### Detecting your project! #############")

	detected := ""

	d, err := os.Open(".")
	if err != nil {
		log.WithField("Error", err).Error("Unable to open directory")
		soft.Exit(err)
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		log.WithField("Error", err).Error("Unable to read directory")
		soft.Exit(err)
	}
outer:
	for _, f := range files {
		switch {
		case f.Name() == "package.json":
			detected = "nodejs"
			break outer

		case f.Name() == "requirements.txt":
			detected = "python"
			break outer

		case f.Name() == "Gemfile":
			detected = "ruby"
			break outer

		case filepath.Ext(f.Name()) == ".go":
			detected = "golang"
			break outer
		}
	}
	if detected == "" {
		log.Println("No stack detected, generating default wercker.yml")
		detected = "default"
	} else {
		log.Println("Detected:", detected)
		log.Println("Generating wercker.yml")
	}
	getYml(detected, options)
}

// TODO(mies): maybe move to util.go at some point
func getYml(detected string, options *GlobalOptions) {
	url := fmt.Sprintf("%s/yml/%s", options.WerckerEndpoint, detected)
	res, err := http.Get(url)
	if err != nil {
		log.WithField("Error", err).Error("Unable to reach wercker API")
		os.Exit(1)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithField("Error", err).Error("Unable to read response")
	}

	err = ioutil.WriteFile("wercker.yml", body, 0644)
	if err != nil {
		log.WithField("Error", err).Error("Unable to write wercker.yml file")
	}
}
