package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
	"github.com/termie/go-shutil"
	"io/ioutil"
	"log"
	"os"
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
		cli.StringFlag{Name: "buildId", Value: "", Usage: "build id"},

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
				BuildProject(c)
			},
			Flags: []cli.Flag{},
		},
	}
	app.Run(os.Args)
}

func BuildProject(c *cli.Context) {
	log.Println("############# Building project #############")
	log.Println(c.Args().First())
	log.Println("############################################")

	// Parse CLI and local env
	options, err := CreateGlobalOptions(c, os.Environ())
	if err != nil {
		log.Panicln(err)
	}
	// log.Println(options)

	// Setup a docker client
	client, _ := docker.NewClient(options.DockerEndpoint)

	// NOTE(termie): For now we are expecting it to be downloaded
	// before we start so we are just expecting it to exist in our
	// projects directory.
	project := c.Args().First()
	projectDir := fmt.Sprintf("%s/%s", options.ProjectDir, project)

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
	box, err := rawConfig.RawBox.ToBox(build, options)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Box:", box.Name)
	log.Println("Steps:", len(build.Steps))

	// Make sure we have the box available
	image, err := box.Fetch()
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Docker Image:", image.ID)

	serviceLinks := []string{}
	for _, service := range rawConfig.RawServices {
		log.Println("Fetching service:", service)

		// TODO(mh): fetch the image

		if _, err := client.InspectImage(service); err != nil {
			log.Panicln(err)
		}

		containerName := fmt.Sprintf("wercker-service-%s-%s",
			service, options.BuildId)

		container, err := client.CreateContainer(
			docker.CreateContainerOptions{
				Name: containerName,
				Config: &docker.Config{
					Image: service,
				},
			})

		if err != nil {
			log.Panicln(err)
		}

		client.StartContainer(container.ID, &docker.HostConfig{})

		serviceLinks = append(serviceLinks, fmt.Sprintf("%s:%s", containerName, service))
		// TODO(mh): We want to make sure container is running fully before
		// allowing build steps to run. We may need custom steps which block
		// until service services are running.
	}
	fmt.Println("creating links: ", serviceLinks)

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
		log.Println("Fetching Step:", step.Name, step.Id)
		if _, err := step.Fetch(); err != nil {
			log.Panicln(err)
		}
	}

	// Make our list of binds for the Docker attach
	// NOTE(termie): we don't appear to need the "volumes" stuff, leaving
	//               it commented out in case it actually does something
	binds := []string{}
	// volumes := make(map[string]struct{})
	entries, err := ioutil.ReadDir(build.HostPath())
	for _, entry := range entries {
		if entry.IsDir() {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", build.HostPath(entry.Name()), build.MntPath(entry.Name())))
			// volumes[build.MntPath(entry.Name())] = struct{}{}
		}
	}

	// Make and start the container
	containerName := "wercker-build-" + options.BuildId
	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:        box.Name,
				Tty:          false,
				OpenStdin:    true,
				Cmd:          []string{"/bin/bash"},
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				// Volumes: volumes,
			},
		})
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Docker Container:", container.ID)
	client.StartContainer(container.ID, &docker.HostConfig{
		Binds: binds,
		Links: serviceLinks,
	})

	// Start our session
	sess := CreateSession(options.DockerEndpoint, container.ID)
	sess, err = sess.Attach()
	if err != nil {
		log.Fatalln(err)
	}

	// Some helpful logging
	log.Println("Base Build Environment:")
	for k, v := range build.Env.Map {
		log.Println(" ", k, v)
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
		log.Println(step.Name, step.Id)
		log.Println("===========================================")

		step.InitEnv()
		log.Println("Step Environment")
		for k, v := range step.Env.Map {
			log.Println(" ", k, v)
		}
		exit, err = step.Execute(sess)
		if exit != 0 {
			log.Fatalln("Build failed with exit code:", exit)
		}
		if err != nil {
			log.Panicln(err)
		}
		log.Println("============ Step successful! =============")
	}

	log.Println("########### Build successful! #############")
	// TODO(mh): Stop containers.
	// https://github.com/docker/docker/blob/master/api/client/commands.go#L2255
}
