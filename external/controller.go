// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package external

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

// NewDockerController -
func NewDockerController() *RunnerParams {
	return &RunnerParams{
		RunnerCount:  1,
		ShutdownFlag: false,
	}
}

// RunnerParams - The parameters that drive the control of Docker
// containers twhere the external runner executes. This structure is
// passed from the Wercker CLI when runner is specified.
type RunnerParams struct {
	WerckerToken string // API Bearer token
	InstanceName string // Runner name
	GroupName    string // Runner group name
	ImageName    string // Docker image
	OrgID        string // Organization ID
	AppNames     string // Application names
	OrgList      string // Organizations
	Workflows    string // Workflows
	OutputPath   string // Local storage locatioin
	LoggerPath   string // Where to write logs
	RunnerCount  int    // Number of runner containers
	ShutdownFlag bool   // Shutdown if true
	// following values are set during processing
	client *docker.Client
}

// RunDockerController - This is commander-in-chief of external runners. It is called from
// Wercker CLI to start or stop external runners. The Wercker CLI builds the RunnParams and
// calls this function.
func (cp *RunnerParams) RunDockerController(statusOnly bool) {

	// When no instance name was supplied, use the hostname
	if cp.InstanceName == "" {
		hostName, err := os.Hostname()

		if err != nil {
			log.Print(fmt.Sprintf("unable to access hostname: %s", err))
			return
		}
		cp.InstanceName = hostName
	}

	endpoint := "unix:///var/run/docker.sock"
	cli, err := docker.NewClient(endpoint)
	if err != nil {
		log.Print(fmt.Sprintf("unable to create the Docker client: %s", err))
		return
	}
	cp.client = cli

	// Get the list of running containers and determine if there are already
	// any running for the runner instance name.
	clist, err := cp.client.ListContainers(docker.ListContainersOptions{
		All: true,
	})

	// Pick out containers related to this runner instance set.
	runners := []*docker.Container{}
	lName := fmt.Sprintf("/wercker-external-runner-%s", cp.InstanceName)
	for _, dockerAPIContainer := range clist {
		for _, label := range dockerAPIContainer.Labels {
			if label == lName {
				dockerContainer, err := cp.client.InspectContainer(dockerAPIContainer.ID)
				if err == nil {
					runners = append(runners, dockerContainer)
					break
				}
			}
		}
	}

	// runners contains the containers running for this external runner
	if cp.ShutdownFlag == true {
		// Go handle shutdown of our runners.
		cp.shutdownRunners(runners)
		return
	}

	if statusOnly == true {
		if len(runners) > 0 {
			for _, dockerContainer := range runners {
				cname := stripSlashFromName(dockerContainer.Name)
				stats := dockerContainer.State.Status
				if stats != "running" {
					detail := fmt.Sprintf("Inactive external runner container %s is being removed.", cname)
					log.Print(detail)
					opts := docker.RemoveContainerOptions{
						ID: dockerContainer.ID,
					}
					cp.client.RemoveContainer(opts)
					continue
				}
				detail := fmt.Sprintf("External runner container: %s is active, status=%s", cname, stats)
				log.Print(detail)
			}
			return
		}
		log.Print("There are no external runners active.")
		return
	}

	// OK, we want to start something.
	cp.startTheRunners()
	return
}

// Starting runner(s).  Initiate a container to run the external runner for as many times as
// specified by the user.
func (cp *RunnerParams) startTheRunners() {

	ct := 1
	for i := cp.RunnerCount; i > 0; i-- {
		cmd, err := cp.createTheRunnerCommand()
		if err == nil {
			cp.startTheContainer(cmd, ct)
			ct++
		}
	}
}

// Create the command to run the external runner in a container.
// Temporarily just read in the arguments from the runner.args file
// to create the command and argument array.
func (cp *RunnerParams) createTheRunnerCommand() ([]string, error) {

	cmd := []string{}
	cmd = append(cmd, "/externalRunner.sh")

	/*
		// Temporary code for building the command by reading arguments from a file.
		file, err := os.Open("runner.args")
		if err != nil {
			return cmd, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			cmd = append(cmd, scanner.Text())
		}
	*/
	return cmd, nil
}

// Start the runner container(s). The command and arguments are supplied so
// create the container, then start it.
func (cp *RunnerParams) startTheContainer(cmd []string, ct int) error {

	name := fmt.Sprintf("%s_%d", cp.InstanceName, ct)
	labels := map[string]string{}
	lName := fmt.Sprintf("runner=/wercker-external-runner-%s", cp.InstanceName)
	labels["runner"] = lName

	// This is a super Kludge until go-dockerclient is updated to support mounts.

	args := []string{}
	args = append(args, "run")
	args = append(args, "--detach")
	args = append(args, "--label")
	args = append(args, lName)
	args = append(args, "--name")
	args = append(args, name)
	args = append(args, "--volume")
	args = append(args, "/Users/bihaber/wercher:/var/lib/wercker:rw")
	args = append(args, "--volume")
	args = append(args, "/var/run/docker.sock:/var/run/docker.sock")
	args = append(args, "external-runner:latest")
	args = append(args, "/externalRunner.sh")

	err := runDocker(args)
	if err != nil {
		log.Print(err)
		return err
	}

	message := fmt.Sprintf("External runner %s has started.", name)
	log.Print(message)
	return nil
}

// Execute the docker command
func runDocker(args []string) error {
	dockerCmd := exec.Command("docker", args...)
	// run using a pseudo-terminal so that we get the nice docker output :)
	err := dockerCmd.Start()
	//_, err := pty.Start(dockerCmd)
	if err != nil {
		return err
	}
	// Stream output of the command to stdout
	//io.Copy(os.Stdout, outFile)
	return nil
}

// Shutdown all the external runners that have been started for this instance. Each
// container is killed, then waited for it to exit. Then delete the container.
func (cp *RunnerParams) shutdownRunners(runners []*docker.Container) {

	if len(runners) == 0 {
		log.Print("There are no external runners to terminate")
		return
	}

	// For each runner, kill it and wait for it exited before destorying the container.
	for _, dockerContainer := range runners {

		containerName := stripSlashFromName(dockerContainer.Name)
		stats := dockerContainer.State.Status
		// If container is not in a running state then remove it
		if stats != "running" {
			detail := fmt.Sprintf("Inactive external runner container %s is removed.", containerName)
			log.Print(detail)
			opts := docker.RemoveContainerOptions{
				ID: dockerContainer.ID,
			}
			cp.client.RemoveContainer(opts)
			continue
		}

		err := cp.client.KillContainer(docker.KillContainerOptions{
			ID: dockerContainer.ID,
		})
		if err != nil {
			message := fmt.Sprintf("failed to kill runner container: %s, err=%s", containerName, err)
			log.Print(message)
			continue
		}
		// Container was killed, now wait for it to exit.
		for {
			time.Sleep(1000 * time.Millisecond)
			container, err := cp.client.InspectContainer(dockerContainer.ID)

			if err != nil {
				// Assume that an error is because container terminated
				break
			}
			if container.State.Status == "exited" {
				opts := docker.RemoveContainerOptions{
					ID: container.ID,
				}
				cp.client.RemoveContainer(opts)
				message := fmt.Sprintf("External runner %s has terminated.", containerName)
				log.Print(message)
				break
			}
		}
	}
	var finalMessage = "External-runner shutdown complete."
	log.Print(finalMessage)
}

// Remove the slash from the beginning of the name
func stripSlashFromName(name string) string {
	return strings.TrimPrefix(name, "/")
}
