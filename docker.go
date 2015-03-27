package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/CenturyLinkLabs/docker-reg-client/registry"
	"github.com/chuckpreslar/emission"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

func requireDockerEndpoint(options *DockerOptions) error {
	client, err := NewDockerClient(options)
	if err != nil {
		if err == docker.ErrInvalidEndpoint {
			return fmt.Errorf(`The given docker endpoint seems invalid:
  %s
To specify a different endpoint use the DOCKER_HOST environment variable,
or the --docker-host command-line flag.
`, options.DockerHost)
		}
		return err
	}
	_, err = client.Version()
	if err != nil {
		if err == docker.ErrConnectionRefused {
			return fmt.Errorf(`Can't connect to the Docker endpoint:
  %s
To specify a different endpoint use the DOCKER_HOST environment variable,
or the --docker-host command-line flag.
`, options.DockerHost)
		}
		return err
	}
	return nil
}

// DockerClient is our wrapper for docker.Client
type DockerClient struct {
	*docker.Client
	logger *LogEntry
}

// NewDockerClient based on options and env
func NewDockerClient(options *DockerOptions) (*DockerClient, error) {
	dockerHost := options.DockerHost
	tlsVerify := options.DockerTLSVerify

	logger := rootLogger.WithField("Logger", "Docker")

	var (
		client *docker.Client
		err    error
	)

	if tlsVerify == "1" {
		// We're using TLS, let's locate our certs and such
		// boot2docker puts its certs at...
		dockerCertPath := options.DockerCertPath

		// TODO(termie): maybe fast-fail if these don't exist?
		cert := path.Join(dockerCertPath, fmt.Sprintf("cert.pem"))
		ca := path.Join(dockerCertPath, fmt.Sprintf("ca.pem"))
		key := path.Join(dockerCertPath, fmt.Sprintf("key.pem"))
		logger.Println("key path", key)
		client, err = docker.NewVersionnedTLSClient(dockerHost, cert, key, ca, "")
		if err != nil {
			return nil, err
		}
	} else {
		client, err = docker.NewClient(dockerHost)
		if err != nil {
			return nil, err
		}
	}
	return &DockerClient{Client: client, logger: logger}, nil
}

// RunAndAttach gives us a raw connection to a newly run container
func (c *DockerClient) RunAndAttach(name string) error {
	container, err := c.CreateContainer(
		docker.CreateContainerOptions{
			Name: uuid.NewRandom().String(),
			Config: &docker.Config{
				Image:        name,
				Tty:          true,
				OpenStdin:    true,
				Cmd:          []string{"/bin/bash"},
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				// NetworkDisabled: b.networkDisabled,
				// Volumes: volumes,
			},
		})
	if err != nil {
		return err
	}
	c.StartContainer(container.ID, &docker.HostConfig{})

	opts := docker.AttachToContainerOptions{
		Container:    container.ID,
		Logs:         true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		InputStream:  os.Stdin,
		ErrorStream:  os.Stderr,
		OutputStream: os.Stdout,
		RawTerminal:  true,
	}

	var oldState *term.State

	oldState, err = term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	go func() {
		err := c.AttachToContainer(opts)
		if err != nil {
			c.logger.Panicln("attach panic", err)
		}
	}()

	_, err = c.WaitContainer(container.ID)
	return err
}

// CheckAccessOptions is just args for CheckAccess
type CheckAccessOptions struct {
	Auth       docker.AuthConfiguration
	Access     string
	Repository string
	Tag        string
	Registry   string
}

// normalizeRepo only really applies to the repo name used in the registry
// the full name is still used within the other calls to docker stuff
func normalizeRepo(name string) string {
	// NOTE(termie): the local name of the repo is something like
	//               quay.io/termie/gox-mirror but we ahve to check for
	//               termie/gox-mirror, so... it's manglin' time
	parts := strings.Split(name, "/")
	if len(parts) == 1 {
		return name
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func normalizeRegistry(address string) string {
	logger := rootLogger.WithField("Logger", "Docker")
	if address == "" {
		logger.Debugln("No registry address provided, using https://registry.hub.docker.com")
		return "https://registry.hub.docker.com/v1/"
	}
	parsed, err := url.Parse(address)
	if err != nil {
		logger.Errorln("Registry address is invalid, this will probably fail:", address)
		return address
	}
	if parsed.Scheme != "https" {
		logger.Warnln("Registry address is expected to begin with 'https://', forcing it to use https")
		parsed.Scheme = "https"
		address = parsed.String()
	}
	if strings.HasSuffix(address, "/") {
		address = address[:len(address)-1]
	}

	parts := strings.Split(address, "/")
	possiblyApiVersionStr := parts[len(parts)-1]

	// we only support v1, so...
	if possiblyApiVersionStr == "v2" {
		logger.Warnln("Registry API v2 not supported, using v1")
		newParts := append(parts[:len(parts)-1], "v1")
		address = strings.Join(newParts, "/")
	} else if possiblyApiVersionStr != "v1" {
		newParts := append(parts, "v1")
		address = strings.Join(newParts, "/")
	}
	return address + "/"
}

// CheckAccess checks whether a user can read or write an image
// TODO(termie): this really uses the docker registry code rather than the
//               client so, maybe this is the wrong place
func (c *DockerClient) CheckAccess(opts CheckAccessOptions) (bool, error) {
	// Do the steps described here: https://gist.github.com/termie/bc0334b086697a162f67
	name := normalizeRepo(opts.Repository)

	auth := registry.BasicAuth{
		Username: opts.Auth.Username,
		Password: opts.Auth.Password,
	}
	client := registry.NewClient()

	reg := normalizeRegistry(opts.Registry)
	client.BaseURL, _ = url.Parse(reg)

	if opts.Access == "write" {
		if _, err := client.Hub.GetWriteToken(name, auth); err != nil {
			if err.Error() == "Server returned status 401" || err.Error() == "Server returned status 403" {
				return false, nil
			}
			return false, err
		}
	} else if opts.Access == "read" {
		if _, err := client.Hub.GetReadTokenWithAuth(name, auth); err != nil {
			if err.Error() == "Server returned status 401" || err.Error() == "Server returned status 403" {
				return false, nil
			}
			return false, err
		}
	} else {
		return false, fmt.Errorf("Invalid access type requested: %s", opts.Access)
	}
	return true, nil
}

// DockerPushStep needs to implemenet IStep
type DockerPushStep struct {
	*BaseStep
	data       map[string]string
	username   string
	password   string
	email      string
	authServer string
	repo       string
	author     string
	message    string
	tag        string
	registry   string
	logger     *LogEntry
	e          *emission.Emitter
}

// NewDockerPushStep is a special step for doing docker pushes
func NewDockerPushStep(stepConfig *StepConfig, options *PipelineOptions) (*DockerPushStep, error) {
	name := "docker-push"
	displayName := "docker push"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := &BaseStep{
		displayName: displayName,
		env:         &Environment{},
		id:          name,
		name:        name,
		options:     options,
		owner:       "wercker",
		safeID:      stepSafeID,
		version:     Version(),
	}

	return &DockerPushStep{
		BaseStep: baseStep,
		data:     stepConfig.Data,
		logger:   rootLogger.WithField("Logger", "DockerPushStep"),
		e:        GetEmitter(),
	}, nil
}

// interpolate is a naive interpolator that attempts to replace variables
// identified by $VAR with the value of the VAR pipeline environment variable
func (s *DockerPushStep) interpolate(value string, pipeline Pipeline) string {
	env := pipeline.Env()
	if strings.HasPrefix(value, "$") {
		if interp, ok := env.Map[value[1:]]; ok {
			return interp
		} else {
			return ""
		}
	}
	return value
}

// The IStep Interface

// InitEnv parses our data into our config
func (s *DockerPushStep) InitEnv(pipeline Pipeline) {
	if username, ok := s.data["username"]; ok {
		s.username = s.interpolate(username, pipeline)
	}

	if password, ok := s.data["password"]; ok {
		s.password = s.interpolate(password, pipeline)
	}

	if email, ok := s.data["email"]; ok {
		s.email = s.interpolate(email, pipeline)
	}

	if authServer, ok := s.data["auth-server"]; ok {
		s.authServer = s.interpolate(authServer, pipeline)
	}

	if repo, ok := s.data["repo"]; ok {
		s.repo = s.interpolate(repo, pipeline)
	}

	if tag, ok := s.data["tag"]; ok {
		s.tag = s.interpolate(tag, pipeline)
	}

	if author, ok := s.data["author"]; ok {
		s.author = s.interpolate(author, pipeline)
	}

	if message, ok := s.data["message"]; ok {
		s.message = s.interpolate(message, pipeline)
	}

	if registry, ok := s.data["registry"]; ok {
		// s.registry = s.interpolate(registry, pipeline)
		s.registry = normalizeRegistry(s.interpolate(registry, pipeline))
	} else {
		// s.registry = "https://registry.hub.docker.com"
		s.registry = normalizeRegistry("https://registry.hub.docker.com")
	}
}

// Fetch NOP
func (s *DockerPushStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute commits the current container and pushes it to the configured
// registry
func (s *DockerPushStep) Execute(ctx context.Context, sess *Session) (int, error) {
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.options.DockerOptions)
	if err != nil {
		return 1, err
	}

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.transport.(*DockerTransport)
	containerID := dt.containerID

	auth := docker.AuthConfiguration{
		Username:      s.username,
		Password:      s.password,
		Email:         s.email,
		ServerAddress: s.authServer,
	}

	checkOpts := CheckAccessOptions{
		Auth:       auth,
		Access:     "write",
		Repository: s.repo,
		Tag:        s.tag,
		Registry:   s.registry,
	}

	check, err := client.CheckAccess(checkOpts)
	if err != nil {
		s.logger.Errorln("Error during check access", err)
		return -1, err
	}
	if !check {
		s.logger.Errorln("Not allowed to interact with this repo:", s.repo)
		return -1, fmt.Errorf("Not allowed to interact with this repo: %s", s.repo)
	}

	s.logger.Debugln("Init env:", s.data)
	commitOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: s.repo,
		Tag:        s.tag,
		Author:     s.author,
		Message:    s.message,
	}
	s.logger.Debugln("Commit container:", containerID)
	client.CommitContainer(commitOpts)

	done := make(chan struct{})
	recv := make(chan string)
	stdout := NewReceiver(recv)

	go func() {
		for {
			select {
			case <-done:
				return
			case line := <-recv:
				s.e.Emit(Logs, &LogsArgs{
					Options: s.options,
					Hidden:  false,
					Logs:    line,
					Stream:  "stdout",
				})
			}
		}
	}()

	pushOpts := docker.PushImageOptions{
		Name:         s.repo,
		Tag:          s.tag,
		Registry:     s.registry,
		OutputStream: stdout,
	}

	s.logger.Println("Push container:", s.repo, s.registry)
	err = client.PushImage(pushOpts, auth)
	done <- struct{}{}

	if err != nil {
		s.logger.Errorln("Failed to push:", err)
		return 1, err
	}

	return 0, nil
}

// CollectFile NOP
func (s *DockerPushStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerPushStep) CollectArtifact(string) (*Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerPushStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}
