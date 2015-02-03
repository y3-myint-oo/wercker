package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/joho/godotenv"
	"golang.org/x/net/context"
)

var (
	buildCommand = cli.Command{
		Name:      "build",
		ShortName: "b",
		Usage:     "build a project",
		Action: func(c *cli.Context) {
			envfile := c.GlobalString("environment")
			_ = godotenv.Load(envfile)

			opts, err := NewBuildOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdBuild(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
		Flags: flagsFor(PipelineFlags, WerckerInternalFlags),
	}

	deployCommand = cli.Command{
		Name:      "deploy",
		ShortName: "d",
		Usage:     "deploy a project",
		Action: func(c *cli.Context) {
			envfile := c.GlobalString("environment")
			_ = godotenv.Load(envfile)

			opts, err := NewDeployOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdDeploy(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
		Flags: flagsFor(PipelineFlags, WerckerInternalFlags),
	}

	detectCommand = cli.Command{
		Name:      "detect",
		ShortName: "de",
		Usage:     "detect the type of project",
		Flags:     []cli.Flag{},
		Action: func(c *cli.Context) {
			opts, err := NewDetectOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdDetect(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
	}

	inspectCommand = cli.Command{
		Name:      "inspect",
		ShortName: "i",
		Usage:     "inspect a recent container",
		Action: func(c *cli.Context) {
			// envfile := c.GlobalString("environment")
			// _ = godotenv.Load(envfile)

			opts, err := NewInspectOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdInspect(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
		Flags: flagsFor(PipelineFlags, WerckerInternalFlags),
	}

	loginCommand = cli.Command{
		Name:      "login",
		ShortName: "l",
		Usage:     "log into wercker",
		Flags:     []cli.Flag{},
		Action: func(c *cli.Context) {
			opts, err := NewLoginOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdLogin(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
	}

	pullCommand = cli.Command{
		Name:      "pull",
		ShortName: "p",
		Usage:     "pull a recent build",
		Action: func(c *cli.Context) {
			// envfile := c.GlobalString("environment")
			// _ = godotenv.Load(envfile)

			opts, err := NewPullOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdPull(c, opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
		Flags: flagsFor(DockerFlags),
	}

	versionCommand = cli.Command{
		Name:      "version",
		ShortName: "v",
		Usage:     "print versions",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "json",
				Usage: "Output version information as JSON",
			},
		},
		Action: func(c *cli.Context) {
			opts, err := NewVersionOptions(c, NewEnvironment(os.Environ()))
			if err != nil {
				log.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdVersion(opts)
			if err != nil {
				log.Errorln("Command failed\n", err)
				os.Exit(1)
			}
		},
	}
)

func main() {
	log.SetLevel(log.DebugLevel)

	app := cli.NewApp()
	app.Version = Version
	app.Flags = flagsFor(GlobalFlags)
	app.Commands = []cli.Command{
		buildCommand,
		deployCommand,
		detectCommand,
		inspectCommand,
		loginCommand,
		pullCommand,
		versionCommand,
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

func cmdBuild(options *PipelineOptions) error {
	return executePipeline(options, GetBuildPipeline)
}

func cmdDeploy(options *PipelineOptions) error {
	return executePipeline(options, GetDeployPipeline)
}

// detectProject inspects the the current directory that sentcli is running in
// and detects the project's programming language
func cmdDetect(options *DetectOptions) error {
	soft := &SoftExit{options.GlobalOptions}

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
	return nil
}

func cmdInspect(options *InspectOptions) error {
	// soft := &SoftExit{options}

	repoName := fmt.Sprintf("%s/%s", options.ApplicationOwnerName, options.ApplicationName)
	tag := options.Tag

	client, err := NewDockerClient(options.DockerOptions)
	if err != nil {
		return err
	}

	return client.RunAndAttach(fmt.Sprintf("%s:%s", repoName, tag))
}

func cmdLogin(options *LoginOptions) error {
	soft := &SoftExit{options.GlobalOptions}

	log.Println("########### Logging into wercker! #############")
	url := fmt.Sprintf("%s/api/1.0/%s", options.BaseURL, "oauth/basicauthaccesstoken")

	username := readUsername()
	password := readPassword()

	token, err := getAccessToken(username, password, url)
	if err != nil {
		log.WithField("Error", err).Error("Unable to log into wercker")
		return soft.Exit(err)
	}

	log.Println("Saving token to: ", options.AuthTokenStore)
	return saveToken(options.AuthTokenStore, token)
}

func cmdPull(c *cli.Context, options *PullOptions) error {
	soft := &SoftExit{options.GlobalOptions}

	dumpOptions(options)

	client, err := NewDockerClient(options.DockerOptions)
	if err != nil {
		return soft.Exit(err)
	}

	auth := docker.AuthConfiguration{}
	if options.AuthToken != "" {
		auth = docker.AuthConfiguration{
			Username:      options.AuthToken,
			Password:      options.AuthToken,
			ServerAddress: options.Registry,
		}
	}

	repo := c.Args().First()
	tag := c.Args().Get(1)
	log.Println("Repo: ", repo)
	log.Println("Tag: ", tag)
	opts := docker.PullImageOptions{
		Repository:   fmt.Sprintf("%s/%s", options.Registry, repo),
		Registry:     options.Registry,
		OutputStream: os.Stdout,
	}

	if tag != "" {
		opts.Tag = tag
	}

	err = client.PullImage(opts, auth)
	if err != nil {
		log.Panicln(err)
	}
	return nil
}

type versions struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
}

func cmdVersion(options *VersionOptions) error {
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
	return nil
}

// TODO(mies): maybe move to util.go at some point
func getYml(detected string, options *DetectOptions) {

	yml := "wercker.yml"
	if _, err := os.Stat(yml); err == nil {
		log.Println(yml, "already exists. Do you want to overwrite? (yes/no)")
		if !askForConfirmation() {
			log.Println("Exiting...")
			os.Exit(1)
		}
	}
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

func executePipeline(options *PipelineOptions, getter GetPipeline) error {
	soft := &SoftExit{options.GlobalOptions}

	// Build our common pipeline
	p := NewRunner(options, getter)
	e := p.Emitter()

	// All bool properties will be initialized on false
	pipelineArgs := &FullPipelineFinishedArgs{}
	fullPipelineFinished := p.StartFullPipeline(options)
	defer fullPipelineFinished.Finish(pipelineArgs)

	buildFinisher := p.StartBuild(options)

	// This will be emitted at the end of the execution, we're going to be
	// pessimistic and report that we failed, unless overridden at the end of the
	// execution.
	defer buildFinisher.Finish(false)

	log.Println("############ Executing Pipeline ############")
	dumpOptions(options)
	log.Println("############################################")

	runnerCtx := context.Background()

	_, err := p.EnsureCode()
	if err != nil {
		soft.Exit(err)
	}

	shared, err := p.SetupEnvironment(runnerCtx)
	if shared.box != nil {
		if options.ShouldRemove {
			defer shared.box.Clean()
		}
		defer shared.box.Stop()
	}
	if err != nil {
		return soft.Exit(err)
	}

	// Expand our context object
	box := shared.box
	pipeline := shared.pipeline

	repoName := pipeline.DockerRepo()
	tag := pipeline.DockerTag()
	message := pipeline.DockerMessage()

	storeStep := &Step{Name: "store"}

	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:      pipeline,
		Steps:      pipeline.Steps(),
		StoreStep:  storeStep,
		AfterSteps: pipeline.AfterSteps(),
		Options:    options,
	})

	pr := &PipelineResult{
		Success:           true,
		FailedStepName:    "",
		FailedStepMessage: "",
	}

	// stepCounter starts at 3, step 1 is "get code", step 2 is "setup
	// environment".
	stepCounter := &Counter{Current: 3}
	for _, step := range pipeline.Steps() {
		log.Println()
		log.Println("============== Running Step ===============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		sr, err := p.RunStep(shared, step, stepCounter.Increment())
		if err != nil {
			pr.Success = false
			pr.FailedStepName = step.DisplayName
			pr.FailedStepMessage = sr.Message
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

	if options.ShouldPush || (pr.Success && options.ShouldArtifacts) {
		// At this point the build has effectively passed but we can still mess it
		// up by being unable to deliver the artifacts

		err = func() error {
			sr := &StepResult{
				Success:    false,
				Artifact:   nil,
				Message:    "",
				PackageURL: "",
				ExitCode:   1,
			}
			finisher := p.StartStep(shared, storeStep, stepCounter.Increment())
			defer finisher.Finish(sr)

			pr.FailedStepName = storeStep.Name

			if options.ShouldPush {
				pr.FailedStepMessage = "Unable to push to registry"

				pushOptions := &PushOptions{
					Registry: options.Registry,
					Name:     repoName,
					Tag:      tag,
					Message:  message,
				}

				auth := docker.AuthConfiguration{}
				if options.AuthToken != "" {
					auth = docker.AuthConfiguration{
						Username:      options.AuthToken,
						Password:      options.AuthToken,
						ServerAddress: options.Registry,
					}
				}

				_, err = box.Push(pushOptions, auth)
				if err != nil {
					return err
				}
			}

			if pr.Success && options.ShouldArtifacts {
				pr.FailedStepMessage = "Unable to store pipeline output"

				artifact, err := pipeline.CollectArtifact(shared.containerID)
				// Ignore ErrEmptyTarball errors
				if err != ErrEmptyTarball {
					if err != nil {
						return err
					}

					artificer := NewArtificer(options)
					err = artificer.Upload(artifact)
					if err != nil {
						return err
					}

					sr.PackageURL = artifact.URL()
				}
			}

			// Everything went ok, so reset failed related fields
			pr.Success = true
			pr.FailedStepName = ""
			pr.FailedStepMessage = ""

			sr.Success = true
			sr.ExitCode = 0

			return nil
		}()
		if err != nil {
			pr.Success = false
			log.WithField("Error", err).Error("Unable to store pipeline output")
		}
	}

	if pr.Success {
		log.Println("########### Pipeline passed! ##############")
	} else {
		log.Warnln("########### Pipeline failed! ##############")
	}

	// We're sending our build finished but we're not done yet,
	// now is time to run after-steps if we have any
	buildFinisher.Finish(pr.Success)
	pipelineArgs.MainSuccessful = pr.Success

	if len(pipeline.AfterSteps()) == 0 {
		return nil
	}

	pipelineArgs.RanAfterSteps = true

	log.Println("########## Starting After Steps ###########")
	// The container may have died, either way we'll have a fresh env
	container, err := box.Restart()
	if err != nil {
		log.Panicln(err)
	}

	newSessCtx, newSess, err := p.GetSession(runnerCtx, container.ID)
	if err != nil {
		log.Panicln(err)
	}

	newShared := &RunnerShared{
		box:         shared.box,
		pipeline:    shared.pipeline,
		sess:        newSess,
		sessionCtx:  newSessCtx,
		containerID: shared.containerID,
		config:      shared.config,
	}

	// Set up the base environment
	err = pipeline.ExportEnvironment(newSessCtx, newSess)
	if err != nil {
		return err
	}

	// Add the After-Step parts
	err = pr.ExportEnvironment(newSessCtx, newSess)
	if err != nil {
		return err
	}

	for _, step := range pipeline.AfterSteps() {
		log.Println()
		log.Println("=========== Running After Step ============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		_, err := p.RunStep(newShared, step, stepCounter.Increment())
		if err != nil {
			log.Warnln("=========== After Step failed! ============")
			break
		}
		log.Println("=========== After Step passed! ============")
	}

	if pr.Success {
		log.Println("########### Pipeline passed! ##############")
	} else {
		log.Warnln("########### Pipeline failed! ##############")
	}

	if !pr.Success {
		return fmt.Errorf("Step failed: %s", pr.FailedStepName)
	}

	pipelineArgs.AfterStepSuccessful = pr.Success

	return nil
}
