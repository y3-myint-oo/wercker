package main

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
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
		cli.BoolFlag{
			Name:  "keen-metrics",
			Usage: "report metrics to keen.io",
		},
		cli.StringFlag{
			Name:  "keen-project-write-key",
			Value: "",
			Usage: "keen write key"},
		cli.StringFlag{
			Name:  "keen-project-id",
			Value: "",
			Usage: "keen project id"},

		// Reporter settings
		cli.BoolFlag{
			Name:  "report",
			Usage: "Report logs back to wercker (requires build-id, wercker-host, wercker-token)",
		},
		cli.StringFlag{
			Name:  "wercker-host",
			Usage: "Wercker host to use for wercker reporter",
		},
		cli.StringFlag{
			Name:  "wercker-token",
			Usage: "Wercker token to use for wercker reporter",
		},

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

func buildProject(c *cli.Context) {
	// Parse CLI and local env
	options, err := CreateGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}

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

	if options.ShouldKeenMetrics {
		mh, err := NewMetricsHandler(options)
		if err != nil {
			log.WithField("Error", err).Panic("Unable to MetricsHandler")
		}
		mh.ListenTo(e)
	}

	if options.ShouldReport {
		r, err := NewReportHandler(options.WerckerHost, options.WerckerToken)
		if err != nil {
			log.WithField("Error", err).Panic("Unable to ReportHandler")
		}
		r.ListenTo(e)
	}

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

	// Signal handling
	// Later on we'll register stuff to happen when one is received
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	projectDir := fmt.Sprintf("%s/%s", options.ProjectDir, options.ApplicationID)

	// If the target is a tarball fetch and build that
	if options.ProjectURL != "" {
		resp, err := fetchTarball(options.ProjectURL)
		if err != nil {
			log.Panicln(err)
		}
		err = untargzip(projectDir, resp.Body)
		if err != nil {
			log.Panicln(err)
		}
	} else {
		// We were pointed at a path, copy it to projectDir

		// Make sure we don't accidentally recurse or copy extra files
		ignoreFunc := func(src string, files []os.FileInfo) []string {
			ignores := []string{}
			for _, file := range files {
				abspath, err := filepath.Abs(filepath.Join(src, file.Name()))
				if err != nil {
					panic(err)
				}
				if abspath == options.BuildDir || abspath == options.ProjectDir || abspath == options.StepDir {
					ignores = append(ignores, file.Name())
				}
				// TODO(termie): maybe ignore .gitignore files?
			}
			return ignores
		}
		copyOpts := &shutil.CopyTreeOptions{Ignore: ignoreFunc, CopyFunction: shutil.Copy}
		os.Rename(projectDir, fmt.Sprintf("%s-%s", projectDir, uuid.NewRandom().String()))
		err = shutil.CopyTree(options.ProjectPath, projectDir, copyOpts)
		if err != nil {
			panic(err)
		}
	}

	setupEnvironmentStep := &Step{Name: "setup environment"}
	e.Emit(BuildStepStarted, &BuildStepStartedArgs{
		Options: options,
		Step:    setupEnvironmentStep,
		Order:   2,
	})

	// Return a []byte of the yaml we find or create.
	werckerYaml, err := ReadWerckerYaml([]string{projectDir}, false)
	if err != nil {
		log.Panicln(err)
	}

	// Parse that bad boy.
	rawConfig, err := ConfigFromYaml(werckerYaml)
	if err != nil {
		log.Panicln(err)
	}

	// Add some options to the global config
	if rawConfig.SourceDir != "" {
		options.SourceDir = rawConfig.SourceDir
	}

	// Promote the RawBuild to a real Build. We believe in you, Build!
	build, err := rawConfig.RawBuild.ToBuild(options)
	if err != nil {
		log.Panicln(err)
	}

	// Promote RawBox to a real Box. We believe in you, Box!
	box, err := rawConfig.RawBox.ToBox(build, options, nil)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Application:", options.ApplicationName)
	log.Println("Box:", box.Name)
	log.Println("Steps:", len(build.Steps))

	// Make sure we have the box available
	if image, err := box.Fetch(); err != nil {
		log.Panicln(err)
	} else {
		log.Println("Docker Image:", image.ID)
	}

	for _, rawService := range rawConfig.RawServices {
		log.Println("Fetching service:", rawService)

		serviceBox, err := rawService.ToServiceBox(build, options, nil)
		if err != nil {
			log.Panicln(err)
		}

		if _, err := serviceBox.Box.Fetch(); err != nil {
			log.Panicln(err)
		}

		_, err = serviceBox.Run()
		if err != nil {
			log.Panicln(err)
		}

		box.AddService(serviceBox)
		// TODO(mh): We want to make sure container is running fully before
		// allowing build steps to run. We may need custom steps which block
		// until service services are running.
	}

	// Start setting up the build dir
	err = os.MkdirAll(build.HostPath(), 0755)
	if err != nil {
		log.Panicln(err)
	}

	err = shutil.CopyTree(projectDir, build.HostPath("source"), nil)
	if err != nil {
		log.Panicln(err)
	}

	// Make sure we have the steps
	for _, step := range build.Steps {
		log.Println("Fetching Step:", step.Name, step.ID)
		if _, err := step.Fetch(); err != nil {
			log.Panicln(err)
		}
	}

	container, err := box.Run()
	if err != nil {
		log.Panicln(err)
	}
	defer box.Stop()
	// Register our signal handler to clean the box up
	// TODO(termie): we should probably make a little general purpose signal
	// handler and register callbacks with it so that multiple parts of the app
	// can do cleanup
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
	sess := CreateSession(options.DockerHost, container.ID)
	sess, err = sess.Attach()
	if err != nil {
		log.Fatalln(err)
	}

	// Some helpful logging
	log.Println("Base Build Environment:")
	for _, pair := range build.Env.Ordered() {
		log.Println(" ", pair[0], pair[1])
	}

	err = build.SetupGuest(sess)
	exit, _, err := sess.SendChecked(build.Env.Export()...)
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
		Build:      build,
		Options:    options,
		Step:       setupEnvironmentStep,
		Order:      2,
		Successful: true,
	})

	// TODO(bvdberg): Add steps to event
	e.Emit(BuildStepsAdded, &BuildStepsAddedArgs{
		Build:   build,
		Steps:   build.Steps,
		Options: options,
	})

	stepFailed := false
	offset := 2
	for i, step := range build.Steps {
		log.Println()
		log.Println("============= Executing Step ==============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		e.Emit(BuildStepStarted, &BuildStepStartedArgs{
			Build:   build,
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
				Build:      build,
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
				// log.Panicln(err)
			}
			artifacts, err := step.CollectArtifacts(sess)
			if err != nil {
				return err
				// log.Panicln(err)
			}

			artificer := CreateArtificer(options)
			for _, artifact := range artifacts {
				err := artificer.Upload(artifact)
				if err != nil {
					return err
					// log.Panicln(err)
				}
			}
			stepArgs.Successful = true
			return nil
		}()

		if err != nil {
			stepFailed = true
			break
		}

		log.Println("============ Step successful! =============")

		if options.ShouldCommit {
			box.Commit(repoName, tag, message)
		}
	}

	if options.ShouldCommit {
		box.Commit(repoName, tag, message)
	}

	if options.ShouldPush {
		pushOptions := &PushOptions{
			Registry: options.Registry,
			Name:     repoName,
			Tag:      tag,
			Message:  message,
		}

		_, err = box.Push(pushOptions)
		if err != nil {
			log.WithField("Error", err).Error("Unable to push to registry")
		}
	}

	// Only make it passed if we reach this code (ie no panics) and no step
	// failed.
	if !stepFailed {
		buildFinishedArgs.Result = "passed"
	}

	log.Println("########### Build successful! #############")
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
