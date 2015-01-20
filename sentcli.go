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
	app.Flags = allFlags()

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
	return executePipeline(c, GetDeployPipeline)
}

func buildProject(c *cli.Context) error {
	return executePipeline(c, GetBuildPipeline)
}

func executePipeline(c *cli.Context, getter GetPipeline) error {
	// Parse CLI and local env
	options, err := NewGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

	soft := &SoftExit{options}

	// Build our common pipeline
	p := NewRunner(options, getter)
	e := p.Emitter()

	buildFinisher := p.StartBuild(options)

	// This will be emitted at the end of the execution, we're going to be
	// pessimistic and report that we failed, unless overridden at the end of the
	// execution.
	defer buildFinisher.Finish(false)

	log.Println("############ Executing Pipeline ############")
	dumpOptions(options)
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

	pr := &PipelineResult{
		Success:           true,
		FailedStepName:    "",
		FailedStepMessage: "",
	}

	offset := 2
	for i, step := range pipeline.Steps() {
		log.Println()
		log.Println("============== Running Step ===============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		err = p.RunStep(ctx, step, offset+i)

		if err != nil {
			pr.Success = false
			pr.FailedStepName = step.DisplayName
			pr.FailedStepMessage = ""
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
			pr.Success = false
			pr.FailedStepName = storeStep.Name
			pr.FailedStepMessage = "Unable to push to registry"
			log.WithField("Error", err).Error("Unable to push to registry")
		}
	}

	// At this point the build has effectively passed but we can still mess it
	// up by being unable to deliver the artifacts

	if pr.Success && options.ShouldArtifacts {
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
			pr.Success = false
			pr.FailedStepName = storeStep.Name
			pr.FailedStepMessage = "Unable to store pipeline output"
			log.WithField("Error", err).Error("Unable to store pipeline output")
		}
	}

	if pr.Success {
		log.Println("########### Pipeline passed! ##############")
	} else {
		log.Warnln("########### Pipeline failed! ##############")
	}

	// We're sending our build finished but we're not done yet,
	// now is time to run after-steps
	buildFinisher.Finish(pr.Success)

	log.Println("########## Starting After Steps ###########")
	// The container may have died, either way we'll have a fresh env
	container, err := box.Restart()
	if err != nil {
		log.Panicln(err)
	}

	newSess, err := p.GetSession(container.ID)
	if err != nil {
		log.Panicln(err)
	}

	newCtx := &RunnerContext{
		box:      ctx.box,
		pipeline: ctx.pipeline,
		sess:     newSess,
		config:   ctx.config,
	}

	// Set up the base environment
	err = pipeline.ExportEnvironment(newSess)
	if err != nil {
		return err
	}

	// Add the After-Step parts
	err = pr.ExportEnvironment(newSess)
	if err != nil {
		return err
	}

	for i, step := range pipeline.AfterSteps() {
		log.Println()
		log.Println("=========== Running After Step ============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		err = p.RunStep(newCtx, step, offset+i)

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
