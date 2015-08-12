package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/joho/godotenv"
	"github.com/mreiferson/go-snappystream"
	"github.com/wercker/journalhook"
	"golang.org/x/net/context"
)

var (
	cliLogger    = rootLogger.WithField("Logger", "CLI")
	buildCommand = cli.Command{
		Name:      "build",
		ShortName: "b",
		Usage:     "build a project",
		Action: func(c *cli.Context) {
			envfile := c.GlobalString("environment")
			_ = godotenv.Load(envfile)

			opts, err := NewBuildOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdBuild(opts)
			if err != nil {
				os.Exit(1)
			}
		},
		Flags: flagsFor(PipelineFlags, WerckerInternalFlags),
	}

	devCommand = cli.Command{
		Name:  "dev",
		Usage: "develop and run a local project",
		Action: func(c *cli.Context) {
			envfile := c.GlobalString("environment")
			_ = godotenv.Load(envfile)

			opts, err := NewDevOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdDev(opts)
			if err != nil {
				os.Exit(1)
			}
		},
		Flags: flagsFor(DevPipelineFlags, WerckerInternalFlags),
	}

	checkConfigCommand = cli.Command{
		Name: "check-config",
		// ShortName: "b",
		Usage: "check the project's yaml",
		Action: func(c *cli.Context) {
			envfile := c.GlobalString("environment")
			_ = godotenv.Load(envfile)

			opts, err := NewCheckConfigOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdCheckConfig(opts)
			if err != nil {
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

			opts, err := NewDeployOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdDeploy(opts)
			if err != nil {
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
			opts, err := NewDetectOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdDetect(opts)
			if err != nil {
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

			opts, err := NewInspectOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdInspect(opts)
			if err != nil {
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
			opts, err := NewLoginOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdLogin(opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	logoutCommand = cli.Command{
		Name:      "logout",
		ShortName: "l",
		Usage:     "logout from wercker",
		Flags:     []cli.Flag{},
		Action: func(c *cli.Context) {
			opts, err := NewLogoutOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdLogout(opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	pullCommand = cli.Command{
		Name:        "pull",
		ShortName:   "p",
		Usage:       "pull <build id>",
		Description: "download a Docker repository, and load it into Docker",
		Flags:       flagsFor(DockerFlags, pullFlags),
		Action: func(c *cli.Context) {
			opts, err := NewPullOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}

			err = cmdPull(c, opts)
			if err != nil {
				os.Exit(1)
			}
		},
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
			cli.BoolFlag{
				Name:  "beta",
				Usage: "Checks for the latest beta version",
			},
			cli.BoolFlag{
				Name:  "no-update-check",
				Usage: "Do not check for update",
			},
		},
		Action: func(c *cli.Context) {
			opts, err := NewVersionOptions(c, NewEnvironment(os.Environ()...))
			if err != nil {
				cliLogger.Errorln("Invalid options\n", err)
				os.Exit(1)
			}
			err = cmdVersion(opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	documentCommand = func(app *cli.App) cli.Command {
		return cli.Command{
			Name:  "doc",
			Usage: "Generate usage documentation",
			Action: func(c *cli.Context) {
				opts, err := NewGlobalOptions(c, NewEnvironment(os.Environ()...))
				if err != nil {
					cliLogger.Errorln("Invalid options\n", err)
					os.Exit(1)
				}
				if err := GenerateDocumentation(opts, app); err != nil {
					cliLogger.Panic(err)
				}
			},
		}
	}
)

func main() {
	// logger.SetLevel(logger.DebugLevel)
	// rootLogger.SetLevel("debug")
	// rootLogger.Formatter = &logger.JSONFormatter{}

	app := cli.NewApp()
	setupUsageFormatter(app)
	app.Author = "Team wercker"
	app.Name = "wercker"
	app.Usage = "build and deploy from the command line"
	app.Email = "pleasemailus@wercker.com"
	app.Version = FullVersion()
	app.Flags = flagsFor(GlobalFlags)
	app.Commands = []cli.Command{
		buildCommand,
		devCommand,
		checkConfigCommand,
		deployCommand,
		detectCommand,
		// inspectCommand,
		loginCommand,
		logoutCommand,
		pullCommand,
		versionCommand,
		documentCommand(app),
	}
	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool("debug") {
			rootLogger.Formatter = &VerboseFormatter{}
			rootLogger.SetLevel("debug")
		} else {
			rootLogger.Formatter = &TerseFormatter{}
			rootLogger.SetLevel("info")
		}
		if ctx.GlobalBool("journal") {
			rootLogger.Hooks.Add(&journalhook.JournalHook{})
			rootLogger.Out = ioutil.Discard
		}
		// Register the global signal handler
		globalSigint.Register(os.Interrupt)
		return nil
	}
	app.Run(os.Args)
}

// SoftExit is a helper for determining when to show stack traces
type SoftExit struct {
	options *GlobalOptions
}

// NewSoftExit constructor
func NewSoftExit(options *GlobalOptions) *SoftExit {
	return &SoftExit{options}
}

// Exit with either an error or a panic
func (s *SoftExit) Exit(v ...interface{}) error {
	if s.options.Debug {
		// Clearly this will cause it's own exit if it gets called.
		rootLogger.Panicln(v...)
	}
	rootLogger.Errorln(v...)
	return fmt.Errorf("Exiting.")
}

func cmdDev(options *PipelineOptions) error {
	var pipelineGetter GetPipeline
	if options.Pipeline != "" {
		pipelineGetter = GetDevPipelineFactory(options.Pipeline)
	} else {
		pipelineGetter = GetDevPipeline
	}
	return executePipeline(options, pipelineGetter)
}

func cmdBuild(options *PipelineOptions) error {
	var pipelineGetter GetPipeline
	if options.Pipeline != "" {
		pipelineGetter = GetBuildPipelineFactory(options.Pipeline)
	} else {
		pipelineGetter = GetBuildPipeline
	}
	return executePipeline(options, pipelineGetter)
}

func cmdDeploy(options *PipelineOptions) error {
	var pipelineGetter GetPipeline
	if options.Pipeline != "" {
		pipelineGetter = GetDeployPipelineFactory(options.Pipeline)
	} else {
		pipelineGetter = GetDeployPipeline
	}
	return executePipeline(options, pipelineGetter)
}

func cmdCheckConfig(options *PipelineOptions) error {
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")

	// TODO(termie): this is pretty much copy-paste from the
	//               runner.GetConfig step, we should probably refactor
	var werckerYaml []byte
	var err error
	if options.WerckerYml != "" {
		werckerYaml, err = ioutil.ReadFile(options.WerckerYml)
		if err != nil {
			return soft.Exit(err)
		}
	} else {
		werckerYaml, err = ReadWerckerYaml([]string{"."}, false)
		if err != nil {
			return soft.Exit(err)
		}
	}

	// Parse that bad boy.
	rawConfig, err := ConfigFromYaml(werckerYaml)
	fmt.Println(rawConfig)
	if err != nil {
		return soft.Exit(err)
	}

	if rawConfig.Box != nil {
		box, err := rawConfig.Box.ToBox(options, &BoxOptions{})
		if err != nil {
			return soft.Exit(err)
		}
		logger.Println("Found box:", box.Name)
	}

	if rawConfig.Dev != nil {
		build, err := rawConfig.ToPipeline(options, rawConfig.Dev)
		if err != nil {
			return soft.Exit(err)
		}
		logger.Println("Found dev section")
		if build.box != nil {
			logger.Println("  with box:", build.box.Name)
		}
	}

	if rawConfig.Build != nil {
		build, err := rawConfig.ToPipeline(options, rawConfig.Build)
		if err != nil {
			return soft.Exit(err)
		}
		logger.Println("Found build section")
		if build.box != nil {
			logger.Println("  with box:", build.box.Name)
		}
	}

	if rawConfig.Deploy != nil {
		deploy, err := rawConfig.ToDeploy(options, rawConfig.Deploy)
		if err != nil {
			return soft.Exit(err)
		}
		logger.Println("Found deploy section")
		if deploy.box != nil {
			logger.Println("  with box:", deploy.box.Name)
		}
	}

	return nil
}

// detectProject inspects the the current directory that sentcli is running in
// and detects the project's programming language
func cmdDetect(options *DetectOptions) error {
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")

	logger.Println("########### Detecting your project! #############")

	detected := ""

	d, err := os.Open(".")
	if err != nil {
		logger.WithField("Error", err).Error("Unable to open directory")
		soft.Exit(err)
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to read directory")
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
		logger.Println("No stack detected, generating default wercker.yml")
		detected = "default"
	} else {
		logger.Println("Detected:", detected)
		logger.Println("Generating wercker.yml")
	}
	getYml(detected, options)
	return nil
}

func cmdInspect(options *InspectOptions) error {
	repoName := fmt.Sprintf("%s/%s", options.ApplicationOwnerName, options.ApplicationName)
	tag := options.Tag

	client, err := NewDockerClient(options.DockerOptions)
	if err != nil {
		return err
	}

	return client.RunAndAttach(fmt.Sprintf("%s:%s", repoName, tag))
}

func cmdLogin(options *LoginOptions) error {
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")

	logger.Println("########### Logging into wercker! #############")
	url := fmt.Sprintf("%s/api/1.0/%s", options.BaseURL, "oauth/basicauthaccesstoken")

	username := readUsername()
	password := readPassword()

	token, err := getAccessToken(username, password, url)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to log into wercker")
		return soft.Exit(err)
	}

	logger.Println("Saving token to: ", options.AuthTokenStore)
	return saveToken(options.AuthTokenStore, token)
}

func cmdLogout(options *LogoutOptions) error {
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")

	logger.Println("Logging out")

	err := removeToken(options.GlobalOptions)
	if err != nil {
		return soft.Exit(err)
	}
	return nil
}

func cmdPull(c *cli.Context, options *PullOptions) error {
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")

	if options.Debug {
		dumpOptions(options)
	}

	client := NewAPIClient(options.GlobalOptions)

	var buildID string

	if IsBuildID(options.Repository) {
		buildID = options.Repository
	} else {
		username, applicationName, err := ParseApplicationID(options.Repository)
		if err != nil {
			return soft.Exit(err)
		}

		logger.Println("Fetching build information for application", options.Repository)

		opts := &GetBuildsOptions{
			Limit:  1,
			Branch: options.Branch,
			Result: options.Result,
			Status: "finished",
			Stack:  5,
		}

		builds, err := client.GetBuilds(username, applicationName, opts)
		if err != nil {
			return soft.Exit(err)
		}

		if len(builds) != 1 {
			return soft.Exit(errors.New("No finished builds found for this application"))
		}

		buildID = builds[0].ID
	}

	if buildID == "" {
		return soft.Exit(errors.New("Unable to parse argument as application or build-id"))
	}

	logger.Println("Downloading Docker repository for build", buildID)

	if !options.Force {
		outputExists, err := exists(options.Output)
		if err != nil {
			logger.WithField("Error", err).Error("Unable to create output file")
			return soft.Exit(err)
		}

		if outputExists {
			return soft.Exit(errors.New("The file repository.tar already exists. Delete it, or run again with -f"))
		}
	}

	file, err := os.Create(options.Output)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to create output file")
		return soft.Exit(err)
	}

	repository, err := client.GetDockerRepository(buildID)
	if err != nil {
		os.Remove(file.Name())
		return soft.Exit(err)
	}
	defer repository.Content.Close()

	// Diagram of the various readers/writers
	//   repository <-- tee <-- s <-- [io.Copy] --> file
	//               |
	//               +--> hash       *Legend: --> == write, <-- == read

	counter := NewCounterReader(repository.Content)

	stopEmit := emitProgress(counter, repository.Size, NewRawLogger())

	hash := sha256.New()
	tee := io.TeeReader(counter, hash)
	s := snappystream.NewReader(tee, true)

	_, err = io.Copy(file, s)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to copy data from URL to file")
		os.Remove(file.Name())
		return soft.Exit(err)
	}

	stopEmit <- true

	logger.Println("Download complete")

	calculatedHash := hex.EncodeToString(hash.Sum(nil))
	if calculatedHash != repository.Sha256 {
		return soft.Exit(fmt.Errorf("Calculated hash did not match provided hash (calculated: %s ; expected: %s)", calculatedHash, repository.Sha256))
	}

	if options.Load {
		_, err = file.Seek(0, 0)
		if err != nil {
			logger.WithField("Error", err).Error("Unable to reset seeker")
			return soft.Exit(err)
		}

		dockerClient, err := NewDockerClient(options.DockerOptions)
		if err != nil {
			logger.WithField("Error", err).Error("Unable to create Docker client")
			return soft.Exit(err)
		}

		logger.Println("Importing into Docker")

		importImageOptions := docker.LoadImageOptions{InputStream: file}
		err = dockerClient.LoadImage(importImageOptions)
		if err != nil {
			logger.WithField("Error", err).Error("Unable to load image")
			return soft.Exit(err)
		}

		logger.Println("Finished importing into Docker")
	}

	return nil
}

// emitProgress will keep emitting progress until a value is send into the
// returned channel.
func emitProgress(counter *CounterReader, total int64, logger *Logger) chan<- bool {
	stop := make(chan bool)
	go func(stop chan bool, counter *CounterReader, total int64) {
		prev := int64(-1)
		for {
			current := counter.Count()
			percentage := (100 * current) / total

			select {
			case <-stop:
				logger.Infof("\rDownloading: %3d%%\n", percentage)
				return
			default:
				if percentage != prev {
					logger.Infof("\rDownloading: %3d%%", percentage)
					prev = percentage
				}
				time.Sleep(1 * time.Second)
			}
		}
	}(stop, counter, total)
	return stop
}

func cmdVersion(options *VersionOptions) error {
	logger := rootLogger.WithField("Logger", "Main")
	v := GetVersions()

	if options.OutputJSON {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			logger.WithField("Error", err).Panic("Unable to marshal versions")
		}
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
		return nil
	}

	logger.Infoln("Version:", v.Version)
	logger.Infoln("Compiled at:", v.CompiledAt.Local())

	if v.GitCommit != "" {
		logger.Infoln("Git commit:", v.GitCommit)
	}

	if options.CheckForUpdate {
		channel := "stable"
		if options.BetaChannel {
			channel = "beta"
		}
		updater, err := NewUpdater(channel)
		if err != nil {
			return err
		}
		if updater.UpdateAvailable() {
			logger.Infoln("A new version is available:",
				updater.ServerVersion.FullVersion())
			logger.Infoln("Download it from:", updater.DownloadURL())
			if AskForUpdate() {
				if err := updater.Update(); err != nil {
					logger.WithField("Error", err).Warn(
						"Unable to download latest version. Please try again.")
					return err
				}
			}
		} else {
			logger.Infoln("No new version available")
		}
	}
	return nil
}

// TODO(mies): maybe move to util.go at some point
func getYml(detected string, options *DetectOptions) {
	logger := rootLogger.WithField("Logger", "Main")

	yml := "wercker.yml"
	if _, err := os.Stat(yml); err == nil {
		logger.Println(yml, "already exists. Do you want to overwrite? (yes/no)")
		if !askForConfirmation() {
			logger.Println("Exiting...")
			os.Exit(1)
		}
	}
	url := fmt.Sprintf("%s/api/v2/yml/%s", options.BaseURL, detected)
	res, err := http.Get(url)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to reach wercker API")
		os.Exit(1)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to read response")
	}

	err = ioutil.WriteFile("wercker.yml", body, 0644)
	if err != nil {
		logger.WithField("Error", err).Error("Unable to write wercker.yml file")
	}

}

func executePipeline(options *PipelineOptions, getter GetPipeline) error {
	// Boilerplate
	soft := NewSoftExit(options.GlobalOptions)
	logger := rootLogger.WithField("Logger", "Main")
	e := GetGlobalEmitter()
	f := &Formatter{options.GlobalOptions}

	// Set up the runner
	r := NewRunner(options, getter)
	runnerCtx := context.Background()

	// Main timer
	mainTimer := NewTimer()
	timer := NewTimer()

	// These will be emitted at the end of the execution, we're going to be
	// pessimistic and report that we failed, unless overridden at the end of the
	// execution.
	fullPipelineFinisher := r.StartFullPipeline(options)
	pipelineArgs := &FullPipelineFinishedArgs{}
	defer fullPipelineFinisher.Finish(pipelineArgs)

	buildFinisher := r.StartBuild(options)
	buildFinishedArgs := &BuildFinishedArgs{Box: nil, Result: "failed"}
	defer buildFinisher.Finish(buildFinishedArgs)

	// Debug information
	dumpOptions(options)

	// Do some sanity checks before starting
	err := requireDockerEndpoint(options.DockerOptions)
	if err != nil {
		return soft.Exit(err)
	}

	// Start copying code
	logger.Println(f.Info("Executing pipeline"))
	timer.Reset()
	_, err = r.EnsureCode()
	if err != nil {
		e.Emit(Logs, &LogsArgs{
			Stream: "stderr",
			Logs:   err.Error() + "\n",
		})
		return soft.Exit(err)
	}
	if options.Verbose {
		logger.Printf(f.Success("Copied working dir", timer.String()))
	}

	// Setup environment is still a fairly special step, it needs
	// to start our boxes and get everything set up
	logger.Println(f.Info("Running step", "setup environment"))
	timer.Reset()
	shared, err := r.SetupEnvironment(runnerCtx)
	if shared.box != nil {
		if options.ShouldRemove {
			defer shared.box.Clean()
		}
		defer shared.box.Stop()
	}
	if err != nil {
		logger.Errorln(f.Fail("Step failed", "setup environment", timer.String()))
		e.Emit(Logs, &LogsArgs{
			Stream: "stderr",
			Logs:   err.Error() + "\n",
		})
		return soft.Exit(err)
	}
	if options.Verbose {
		logger.Printf(f.Success("Step passed", "setup environment", timer.String()))
	}

	// Expand our context object
	box := shared.box
	buildFinishedArgs.Box = box
	pipeline := shared.pipeline
	repoName := pipeline.DockerRepo()
	tag := pipeline.DockerTag()
	message := pipeline.DockerMessage()

	shouldStore := options.ShouldStoreS3 || options.ShouldStoreLocal

	// TODO(termie): hack for now, probably can be made into a naive class
	var storeStep Step

	if shouldStore {
		storeStep = &ExternalStep{
			BaseStep: &BaseStep{
				name:    "store",
				owner:   "wercker",
				version: Version(),
			},
		}
	}

	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:      pipeline,
		Steps:      pipeline.Steps(),
		StoreStep:  storeStep,
		AfterSteps: pipeline.AfterSteps(),
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
		logger.Printf(f.Info("Running step", step.DisplayName()))
		timer.Reset()
		sr, err := r.RunStep(shared, step, stepCounter.Increment())
		if err != nil {
			pr.Success = false
			pr.FailedStepName = step.DisplayName()
			pr.FailedStepMessage = sr.Message
			logger.Printf(f.Fail("Step failed", step.DisplayName(), timer.String()))
			break
		}

		if options.Verbose {
			logger.Printf(f.Success("Step passed", step.DisplayName(), timer.String()))
		}
	}

	if options.ShouldCommit {
		_, err = box.Commit(repoName, tag, message)
		if err != nil {
			logger.Errorln("Failed to commit:", err.Error())
		}
	}

	// We need to wind the counter to where it should be if we failed a step
	// so that is the number of steps + get code + setup environment + store
	// TODO(termie): remove all the this "order" stuff completely
	stepCounter.Current = len(pipeline.Steps()) + 3

	if shouldStore || (pr.Success && options.ShouldArtifacts) {
		// At this point the build has effectively passed but we can still mess it
		// up by being unable to deliver the artifacts

		// If we are not saving a commit we still need one temporarily to
		// perform the export, it will get cleaned up when the build finishes.
		if !options.ShouldCommit {
			_, err = box.Commit(repoName, tag, message)
			if err != nil {
				logger.Errorln("Failed to commit:", err.Error())
			}
		}

		err = func() error {
			sr := &StepResult{
				Success:    false,
				Artifact:   nil,
				Message:    "",
				PackageURL: "",
				ExitCode:   1,
			}
			finisher := r.StartStep(shared, storeStep, stepCounter.Increment())
			defer finisher.Finish(sr)

			originalFailedStepName := pr.FailedStepName
			originalFailedStepMessage := pr.FailedStepMessage

			pr.FailedStepName = storeStep.Name()

			if shouldStore {
				pr.FailedStepMessage = "Unable to store container"

				e.Emit(Logs, &LogsArgs{
					Logs: "Exporting container\n",
				})

				file, err := ioutil.TempFile("", "export-image-")
				if err != nil {
					logger.WithField("Error", err).Error("Unable to create temporary file")
					return err
				}
				defer os.Remove(file.Name())

				hash := sha256.New()
				w := snappystream.NewWriter(io.MultiWriter(file, hash))

				logger.WithField("RepositoryName", repoName).Println("Exporting image")

				exportImageOptions := &ExportImageOptions{
					Name:         repoName,
					OutputStream: w,
				}
				err = box.ExportImage(exportImageOptions)
				if err != nil {
					logger.WithField("Error", err).Error("Unable to export image")
					return err
				}

				// Copy is done now, so close temporary file and set the calculatedHash
				file.Close()

				calculatedHash := hex.EncodeToString(hash.Sum(nil))

				logger.WithFields(LogFields{
					"SHA256":            calculatedHash,
					"TemporaryLocation": file.Name(),
				}).Println("Export image successful")

				key := GenerateBaseKey(options)
				key = fmt.Sprintf("%s/%s", key, "docker.tar.sz")

				storeFromFileArgs := &StoreFromFileArgs{
					ContentType: "application/x-snappy-framed",
					Path:        file.Name(),
					Key:         key,
					MaxTries:    3,
					Meta: map[string]*string{
						"Sha256": &calculatedHash,
					},
				}

				if options.ShouldStoreS3 {
					logger.Println("Storing docker repository on S3")

					e.Emit(Logs, &LogsArgs{
						Logs: "Storing container\n",
					})

					s3Store := NewS3Store(options.AWSOptions)

					err = s3Store.StoreFromFile(storeFromFileArgs)
					if err != nil {
						logger.WithField("Error", err).Error("Unable to store to S3 store")
						return err
					}

					e.Emit(Logs, &LogsArgs{
						Logs: "Storing container complete\n",
					})
				}

				if options.ShouldStoreLocal {
					logger.Println("Storing docker repository to local storage")

					localStore := NewLocalStore(options.ContainerDir)

					err = localStore.StoreFromFile(storeFromFileArgs)
					if err != nil {
						logger.WithField("Error", err).Error("Unable to store to local store")
						return err
					}
				}
			}

			if pr.Success && options.ShouldArtifacts {
				pr.FailedStepMessage = "Unable to store pipeline output"

				e.Emit(Logs, &LogsArgs{
					Logs: "Storing artifacts\n",
				})

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

				e.Emit(Logs, &LogsArgs{
					Logs: "Storing artifacts complete\n",
				})
			}

			// Everything went ok, so reset failed related fields
			pr.FailedStepName = originalFailedStepName
			pr.FailedStepMessage = originalFailedStepMessage

			sr.Success = true
			sr.ExitCode = 0

			return nil
		}()
		if err != nil {
			pr.Success = false
			logger.WithField("Error", err).Error("Unable to store pipeline output")
		}
	}

	// We're sending our build finished but we're not done yet,
	// now is time to run after-steps if we have any
	if pr.Success {
		logger.Println(f.Success("Steps passed", mainTimer.String()))
		buildFinishedArgs.Result = "passed"
	}
	buildFinisher.Finish(buildFinishedArgs)
	pipelineArgs.MainSuccessful = pr.Success

	if len(pipeline.AfterSteps()) == 0 {
		// We're about to end the build, so pull the cache and explode it
		// into the CacheDir
		if !options.DirectMount {
			timer.Reset()
			err = pipeline.CollectCache(shared.containerID)
			if err != nil {
				logger.WithField("Error", err).Error("Unable to store cache")
			}
		}

		if pr.Success {
			logger.Println(f.Success("Pipeline finished", mainTimer.String()))
		} else {
			logger.Println(f.Fail("Pipeline failed", mainTimer.String()))
		}

		if !pr.Success {
			return fmt.Errorf("Step failed: %s", pr.FailedStepName)
		}
		return nil
	}

	pipelineArgs.RanAfterSteps = true

	logger.Println(f.Info("Starting after-steps"))
	// The container may have died, either way we'll have a fresh env
	container, err := box.Restart()
	if err != nil {
		logger.Panicln(err)
	}

	newSessCtx, newSess, err := r.GetSession(runnerCtx, container.ID)
	if err != nil {
		logger.Panicln(err)
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
		logger.Println(f.Info("Running after-step", step.DisplayName()))
		timer.Reset()
		_, err := r.RunStep(newShared, step, stepCounter.Increment())
		if err != nil {
			logger.Println(f.Fail("After-step failed", step.DisplayName(), timer.String()))
			break
		}
		logger.Println(f.Success("After-step passed", step.DisplayName(), timer.String()))
	}

	// We're about to end the build, so pull the cache and explode it
	// into the CacheDir
	if !options.DirectMount {
		timer.Reset()
		err = pipeline.CollectCache(newShared.containerID)
		if err != nil {
			logger.WithField("Error", err).Error("Unable to store cache")
		}
		if options.Verbose {
			logger.Printf(f.Success("Exported Cache", timer.String()))
		}

	}

	if pr.Success {
		logger.Println(f.Success("Pipeline finished", mainTimer.String()))
	} else {
		logger.Println(f.Fail("Pipeline failed", mainTimer.String()))
	}

	if !pr.Success {
		return fmt.Errorf("Step failed: %s", pr.FailedStepName)
	}

	pipelineArgs.AfterStepSuccessful = pr.Success

	return nil
}
