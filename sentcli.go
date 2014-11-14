package main

import (
	"code.google.com/p/go-uuid/uuid"
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
	app.Flags = []cli.Flag{
		// These flags control where we store local files
		cli.StringFlag{Name: "projectDir", Value: "./_projects", Usage: "path where downloaded projects live"},
		cli.StringFlag{Name: "stepDir", Value: "./_steps", Usage: "path where downloaded steps live"},
		cli.StringFlag{Name: "buildDir", Value: "./_builds", Usage: "path where created builds live"},

		// These flags tell us where to go for operations
		cli.StringFlag{Name: "dockerHost", Value: "tcp://127.0.0.1:2375", Usage: "docker api host", EnvVar: "DOCKER_HOST"},
		cli.StringFlag{Name: "werckerEndpoint", Value: "https://app.wercker.com/api/v2", Usage: "wercker api endpoint"},
		cli.StringFlag{Name: "baseURL", Value: "https://app.wercker.com/", Usage: "base url for the web app"},
		cli.StringFlag{Name: "registry", Value: "127.0.0.1:3000", Usage: "registry endpoint to push images to"},

		// These flags control paths on the guest and probably shouldn't change
		cli.StringFlag{Name: "mntRoot", Value: "/mnt", Usage: "directory on the guest where volumes are mounted"},
		cli.StringFlag{Name: "guestRoot", Value: "/pipeline", Usage: "directory on the guest where work is done"},
		cli.StringFlag{Name: "reportRoot", Value: "/report", Usage: "directory on the guest where reports will be written"},

		// These flags are usually pulled from the env
		cli.StringFlag{Name: "buildID", Value: "", Usage: "build id", EnvVar: "WERCKER_BUILD_ID"},
		cli.StringFlag{Name: "applicationID", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_ID"},
		cli.StringFlag{Name: "applicationName", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_NAME"},
		cli.StringFlag{Name: "applicationOwnerName", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_OWNER_NAME"},

		// Should we push finished builds to the registry?
		cli.BoolFlag{Name: "pushToRegistry", Usage: "auto push the build result to registry"},

		// AWS bits
		cli.StringFlag{Name: "awsSecretAccessKey", Value: "", Usage: "secret access key"},
		cli.StringFlag{Name: "awsAccessKeyID", Value: "", Usage: "access key id"},
		cli.StringFlag{Name: "awsBucket", Value: "wercker-development", Usage: "bucket for artifacts"},
		cli.StringFlag{Name: "awsRegion", Value: "us-east-1", Usage: "region"},

		// These options might be overwritten by the wercker.yml
		cli.StringFlag{Name: "sourceDir", Value: "", Usage: "source path relative to checkout root"},
		cli.IntFlag{Name: "noResponseTimeout", Value: 5, Usage: "timeout if no script output is received in this many minutes"},
		cli.IntFlag{Name: "commandTimeout", Value: 10, Usage: "timeout if command does not complete in this many minutes"},
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
	}
	app.Run(os.Args)
}

func buildProject(c *cli.Context) {
	log.Println("############# Building project #############")

	// Parse CLI and local env
	options, err := CreateGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}
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

	for _, step := range build.Steps {
		log.Println()
		log.Println("============= Executing Step ==============")
		log.Println(step.Name, step.ID)
		log.Println("===========================================")

		step.InitEnv()
		log.Println("Step Environment")
		for _, pair := range step.Env.Ordered() {
			log.Println(" ", pair[0], pair[1])
		}
		exit, err = step.Execute(sess)
		if exit != 0 {
			box.Stop()
			log.Fatalln("Build failed with exit code:", exit)
		}
		if err != nil {
			log.Panicln(err)
		}
		artifacts, err := step.CollectArtifacts(sess)
		if err != nil {
			log.Panicln(err)
		}

		artificer := CreateArtificer(options)
		for _, artifact := range artifacts {
			err := artificer.Upload(artifact)
			if err != nil {
				log.Panicln(err)
			}
		}
		log.Println("============ Step successful! =============")
	}

	if options.PushToRegistry {
		name := fmt.Sprintf("projects/%s", options.ApplicationName)
		tag := fmt.Sprintf("build-%s", options.BuildID)

		pushOptions := &PushOptions{
			Registry: options.Registry,
			Name:     name,
			Tag:      tag,
		}

		_, err = box.Push(pushOptions)
		if err != nil {
			log.WithField("Error", err).Error("Unable to push to registry")
		}
	}

	log.Println("########### Build successful! #############")
}
