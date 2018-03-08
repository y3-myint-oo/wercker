//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package dockerlocal

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerRunStep struct {
	*core.BaseStep
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	email         string
	env           []string
	stopSignal    string
	builtInPush   bool
	labels        map[string]string
	user          string
	authServer    string
	repository    string
	author        string
	message       string
	tags          []string
	ports         map[docker.Port]struct{}
	volumes       map[string]struct{}
	cmd           []string
	entrypoint    []string
	forceTags     bool
	logger        *util.LogEntry
	workingDir    string
	authenticator auth.Authenticator
}

// NewDockerRunStep is a special step for doing docker pushes
func NewDockerRunStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerRunStep, error) {
	name := "docker-run"
	displayName := "docker run"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := core.NewBaseStep(core.BaseStepOptions{
		DisplayName: displayName,
		Env:         &util.Environment{},
		ID:          name,
		Name:        name,
		Owner:       "wercker",
		SafeID:      stepSafeID,
		Version:     util.Version(),
	})

	return &DockerRunStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerRunStep"),
		options:       options,
		dockerOptions: dockerOptions,
	}, nil
}

func (s *DockerRunStep) configure(env *util.Environment) {
	if email, ok := s.data["email"]; ok {
		s.email = env.Interpolate(email)
	}

	if authServer, ok := s.data["auth-server"]; ok {
		s.authServer = env.Interpolate(authServer)
	}

	if repository, ok := s.data["repository"]; ok {
		s.repository = env.Interpolate(repository)
	}

	if tags, ok := s.data["tag"]; ok {
		splitTags := util.SplitSpaceOrComma(tags)
		interpolatedTags := make([]string, len(splitTags))
		for i, tag := range splitTags {
			interpolatedTags[i] = env.Interpolate(tag)
		}
		s.tags = interpolatedTags
	}

	if author, ok := s.data["author"]; ok {
		s.author = env.Interpolate(author)
	}

	if message, ok := s.data["message"]; ok {
		s.message = env.Interpolate(message)
	}

	if ports, ok := s.data["ports"]; ok {
		iPorts := env.Interpolate(ports)
		parts := util.SplitSpaceOrComma(iPorts)
		portmap := make(map[docker.Port]struct{})
		for _, port := range parts {
			port = strings.TrimSpace(port)
			if !strings.Contains(port, "/") {
				port = port + "/tcp"
			}
			portmap[docker.Port(port)] = struct{}{}
		}
		s.ports = portmap
	}

	if volumes, ok := s.data["volumes"]; ok {
		iVolumes := env.Interpolate(volumes)
		parts := util.SplitSpaceOrComma(iVolumes)
		volumemap := make(map[string]struct{})
		for _, volume := range parts {
			volume = strings.TrimSpace(volume)
			volumemap[volume] = struct{}{}
		}
		s.volumes = volumemap
	}

	if workingDir, ok := s.data["working-dir"]; ok {
		s.workingDir = env.Interpolate(workingDir)
	}

	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.cmd = parts
		}
	}

	if entrypoint, ok := s.data["entrypoint"]; ok {
		parts, err := shlex.Split(entrypoint)
		if err == nil {
			s.entrypoint = parts
		}
	}

	if envi, ok := s.data["env"]; ok {
		parsedEnv, err := shlex.Split(envi)

		if err == nil {
			interpolatedEnv := make([]string, len(parsedEnv))
			for i, envVar := range parsedEnv {
				interpolatedEnv[i] = env.Interpolate(envVar)
			}
			s.env = interpolatedEnv
		}
	}

	if stopsignal, ok := s.data["stopsignal"]; ok {
		s.stopSignal = env.Interpolate(stopsignal)
	}

	if labels, ok := s.data["labels"]; ok {
		parsedLabels, err := shlex.Split(labels)
		if err == nil {
			labelMap := make(map[string]string)
			for _, labelPair := range parsedLabels {
				pair := strings.Split(labelPair, "=")
				labelMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
			}
			s.labels = labelMap
		}
	}

	if user, ok := s.data["user"]; ok {
		s.user = env.Interpolate(user)
	}

	if forceTags, ok := s.data["force-tags"]; ok {
		ft, err := strconv.ParseBool(forceTags)
		if err == nil {
			s.forceTags = ft
		}
	} else {
		s.forceTags = true
	}
}

func (s *DockerRunStep) buildAutherOpts(env *util.Environment) dockerauth.CheckAccessOptions {
	opts := dockerauth.CheckAccessOptions{}
	if username, ok := s.data["username"]; ok {
		opts.Username = env.Interpolate(username)
	}
	if password, ok := s.data["password"]; ok {
		opts.Password = env.Interpolate(password)
	}
	if registry, ok := s.data["registry"]; ok {
		opts.Registry = dockerauth.NormalizeRegistry(env.Interpolate(registry))
	}
	if awsAccessKey, ok := s.data["aws-access-key"]; ok {
		opts.AwsAccessKey = env.Interpolate(awsAccessKey)
	}

	if awsSecretKey, ok := s.data["aws-secret-key"]; ok {
		opts.AwsSecretKey = env.Interpolate(awsSecretKey)
	}

	if awsRegion, ok := s.data["aws-region"]; ok {
		opts.AwsRegion = env.Interpolate(awsRegion)
	}

	if awsAuth, ok := s.data["aws-strict-auth"]; ok {
		auth, err := strconv.ParseBool(awsAuth)
		if err == nil {
			opts.AwsStrictAuth = auth
		}
	}

	if awsRegistryID, ok := s.data["aws-registry-id"]; ok {
		opts.AwsRegistryID = env.Interpolate(awsRegistryID)
	}

	if azureClient, ok := s.data["azure-client-id"]; ok {
		opts.AzureClientID = env.Interpolate(azureClient)
	}

	if azureClientSecret, ok := s.data["azure-client-secret"]; ok {
		opts.AzureClientSecret = env.Interpolate(azureClientSecret)
	}

	if azureSubscriptionID, ok := s.data["azure-subscription-id"]; ok {
		opts.AzureSubscriptionID = env.Interpolate(azureSubscriptionID)
	}

	if azureTenantID, ok := s.data["azure-tenant-id"]; ok {
		opts.AzureTenantID = env.Interpolate(azureTenantID)
	}

	if azureResourceGroupName, ok := s.data["azure-resource-group"]; ok {
		opts.AzureResourceGroupName = env.Interpolate(azureResourceGroupName)
	}

	if azureRegistryName, ok := s.data["azure-registry-name"]; ok {
		opts.AzureRegistryName = env.Interpolate(azureRegistryName)
	}

	if azureLoginServer, ok := s.data["azure-login-server"]; ok {
		opts.AzureLoginServer = env.Interpolate(azureLoginServer)
	}

	// If user use Azure or AWS container registry we don't infer.
	if opts.AzureClientSecret == "" && opts.AwsSecretKey == "" {
		s.repository, opts = InferRegistry(s.repository, opts, s.options)
	}

	// Set user and password automatically if using wercker registry
	if opts.Registry == s.options.WerckerContainerRegistry.String() {
		opts.Username = DefaultDockerRegistryUsername
		opts.Password = s.options.AuthToken
		s.builtInPush = true
	}

	return opts
}

// InferRegistry infers the registry from the repository. If no registry is found
// we fallback to Docker Hub registry.

// InitEnv parses our data into our config
func (s *DockerRunStep) InitEnv(env *util.Environment) {
	s.configure(env)
	opts := s.buildAutherOpts(env)
	auther, _ := dockerauth.GetRegistryAuthenticator(opts)
	s.authenticator = auther
}

// Fetch NOP
func (s *DockerRunStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute commits the current container and pushes it to the configured
// registry
func (s *DockerRunStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return 1, err
	}
	conf := &docker.Config{
		Image: s.data["image"],
	}
	hostconfig := &docker.HostConfig{}

	client.CreateContainer(
		docker.CreateContainerOptions{
			Name:       s.BaseStep.DisplayName(),
			Config:     conf,
			HostConfig: hostconfig,
		})

	s.logger.WithFields(util.LogFields{
		"Repository": s.repository,
		"Tags":       s.tags,
		"Message":    s.message,
	}).Debug("Push to registry")

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	s.tags = s.buildTags()

	if !s.dockerOptions.Local {
		check, err := s.authenticator.CheckAccess(s.repository, auth.Push)
		if err != nil {
			s.logger.Errorln("Error interacting with this repository:", s.repository, err)
			return -1, fmt.Errorf("Error interacting with this repository: %s %v", s.repository, err)
		}
		if !check {
			return -1, fmt.Errorf("Not allowed to interact with this repository: %s", s.repository)
		}
	}
	s.repository = s.authenticator.Repository(s.repository)
	s.logger.Debugln("Init env:", s.data)

	config := docker.Config{
		Cmd:          s.cmd,
		Entrypoint:   s.entrypoint,
		WorkingDir:   s.workingDir,
		User:         s.user,
		Env:          s.env,
		StopSignal:   s.stopSignal,
		Labels:       s.labels,
		ExposedPorts: s.ports,
		Volumes:      s.volumes,
	}

	commitOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: s.repository,
		Author:     s.author,
		Message:    s.message,
		Run:        &config,
		Tag:        s.tags[0],
	}

	s.logger.Debugln("Commit container:", containerID)
	i, err := client.CommitContainer(commitOpts)
	if err != nil {
		return -1, err
	}

	if s.dockerOptions.CleanupImage {
		defer cleanupImage(s.logger, client, s.repository, s.tags[0])
	}

	s.logger.WithField("Image", i).Debug("Commit completed")
	return s.tagAndPush(i.ID, e, client)
}

// CollectFile NOP
func (s *DockerRunStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerRunStep) CollectArtifact(string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerRunStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerRunStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}

func (s *DockerRunStep) buildTags() []string {
	if len(s.tags) == 0 && !s.builtInPush {
		s.tags = []string{"latest"}
	} else if len(s.tags) == 0 && s.builtInPush {
		gitTag := fmt.Sprintf("%s-%s", s.options.GitBranch, s.options.GitCommit)
		s.tags = []string{"latest", gitTag}
	}
	return s.tags
}

func (s *DockerRunStep) tagAndPush(imageID string, e *core.NormalizedEmitter, client *DockerClient) (int, error) {
	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()
	// emitStatusses in a different go routine
	go EmitStatus(e, r, s.options)
	defer w.Close()
	for _, tag := range s.tags {
		tagOpts := docker.TagImageOptions{
			Repo:  s.repository,
			Tag:   tag,
			Force: s.forceTags,
		}
		err := client.TagImage(imageID, tagOpts)
		s.logger.Println("Pushing image for tag ", tag)
		if err != nil {
			s.logger.Errorln("Failed to push:", err)
			return 1, err
		}
		pushOpts := docker.PushImageOptions{
			Name:          s.repository,
			OutputStream:  w,
			RawJSONStream: true,
			Tag:           tag,
		}
		if !s.dockerOptions.Local {
			auth := docker.AuthConfiguration{
				Username: s.authenticator.Username(),
				Password: s.authenticator.Password(),
				Email:    s.email,
			}
			err := client.PushImage(pushOpts, auth)
			if err != nil {
				s.logger.Errorln("Failed to push:", err)
				return 1, err
			}
			s.logger.Println("Pushed container:", s.repository, s.tags)

			if s.dockerOptions.CleanupImage {
				defer cleanupImage(s.logger, client, s.repository, tag)
			}
		}
	}
	return 0, nil
}
