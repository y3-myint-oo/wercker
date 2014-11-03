package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/termie/go-shutil"
	"os"
	"os/signal"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "projectDir", Value: "./projects", Usage: "path where projects live"},
		cli.StringFlag{Name: "stepDir", Value: "./steps", Usage: "path where steps live"},
		cli.StringFlag{Name: "buildDir", Value: "./builds", Usage: "path where builds live"},

		cli.StringFlag{Name: "dockerEndpoint", Value: "tcp://127.0.0.1:2375", Usage: "docker api endpoint"},
		cli.StringFlag{Name: "werckerEndpoint", Value: "https://app.wercker.com/api/v2", Usage: "wercker api endpoint"},
		cli.StringFlag{Name: "mntRoot", Value: "/mnt", Usage: "directory on the guest where volumes are mounted"},
		cli.StringFlag{Name: "guestRoot", Value: "/pipeline", Usage: "directory on the guest where work is done"},
		cli.StringFlag{Name: "reportRoot", Value: "/report", Usage: "directory on the guest where reports will be written"},
		cli.StringFlag{Name: "buildID", Value: "", Usage: "build id"},
		cli.StringFlag{Name: "projectID", Value: "", Usage: "project id"},
		cli.StringFlag{Name: "baseURL", Value: "https://app.wercker.com/", Usage: "base url for the web app"},

		// Code fetching
		// TODO(termie): this should probably be a separate command run beforehand.
		cli.StringFlag{Name: "projectURL", Value: "", Usage: "url of the project tarball"},

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
	log.Println(c.Args().First())
	log.Println("############################################")

	// Parse CLI and local env
	options, err := CreateGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}
	// log.Println(options)

	// Signal handling
	// Later on we'll register stuff to happen when one is received
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	// NOTE(termie): For now we are expecting it to be downloaded
	// before we start so we are just expecting it to exist in our
	// projects directory.
	project := c.Args().First()
	projectDir := fmt.Sprintf("%s/%s", options.ProjectDir, project)

	// TODO(termie): We'll probably do this externally eventually, but
	// this is the easiest place to start fetching code.
	if options.ProjectURL != "" {
		resp, err := fetchTarball(options.ProjectURL)
		if err != nil {
			log.Panicln(err)
		}
		err = untargzip(projectDir, resp.Body)
		if err != nil {
			log.Panicln(err)
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

	log.Println("Project:", options.ProjectID)
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
	sess := CreateSession(options.DockerEndpoint, container.ID)
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

	log.Println("########### Build successful! #############")
}
