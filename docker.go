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
	"github.com/flynn/go-shlex"
	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

func requireDockerEndpoint(options *DockerOptions) error {
	client, err := NewDockerClient(options)
	if err != nil {
		if err == docker.ErrInvalidEndpoint {
			return fmt.Errorf(`You don't seem to have a working Docker 
			environment or the given Docker endpoint is invalid:
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
			return fmt.Errorf(`You don't seem to have a working Docker environment
			or wercker can't connect to the Docker endpoint:
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

	return c.AttachTerminal(container.ID)
}

// AttachInteractive starts an interactive session and runs cmd
func (c *DockerClient) AttachInteractive(containerID string, cmd []string) error {

	exec, err := c.CreateExec(docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
		Container:    containerID,
	})

	if err != nil {
		return err
	}

	err = c.StartExec(exec.ID, docker.StartExecOptions{
		InputStream:  os.Stdin,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Tty:          true,
		RawTerminal:  true,
	})

	return err
}

// AttachTerminal connects us to container and gives us a terminal
func (c *DockerClient) AttachTerminal(containerID string) error {
	c.logger.Println("Attaching to ", containerID)
	opts := docker.AttachToContainerOptions{
		Container:    containerID,
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

	oldState, err := term.SetRawTerminal(os.Stdin.Fd())
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

	_, err = c.WaitContainer(containerID)
	return err
}

// ExecOne uses docker exec to run a command in the container
func (c *DockerClient) ExecOne(containerID string, cmd []string, output io.Writer) error {
	exec, err := c.CreateExec(docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmd,
		Container:    containerID,
	})
	if err != nil {
		return err
	}

	err = c.StartExec(exec.ID, docker.StartExecOptions{
		OutputStream: output,
	})
	if err != nil {
		return err
	}

	return nil
}

// CheckAccessOptions is just args for CheckAccess
type CheckAccessOptions struct {
	Auth       docker.AuthConfiguration
	Access     string
	Repository string
	Tag        string
	Registry   string
}

// normalizeRepo only really applies to the repository name used in the registry
// the full name is still used within the other calls to docker stuff
func normalizeRepo(name string) string {
	// NOTE(termie): the local name of the repository is something like
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
	possiblyAPIVersionStr := parts[len(parts)-1]

	// we only support v1, so...
	if possiblyAPIVersionStr == "v2" {
		logger.Warnln("Registry API v2 not supported, using v1")
		newParts := append(parts[:len(parts)-1], "v1")
		address = strings.Join(newParts, "/")
	} else if possiblyAPIVersionStr != "v1" {
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
		if opts.Auth.Username != "" {
			if _, err := client.Hub.GetReadTokenWithAuth(name, auth); err != nil {
				if err.Error() == "Server returned status 401" || err.Error() == "Server returned status 403" {
					return false, nil
				}
				return false, err
			}
		} else {
			if _, err := client.Hub.GetReadToken(name); err != nil {
				if err.Error() == "Server returned status 401" || err.Error() == "Server returned status 403" {
					return false, nil
				}
				return false, err
			}
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
	repository string
	author     string
	message    string
	tag        string
	registry   string
	ports      string
	volumes    string
	cmd        []string
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

// The IStep Interface

// InitEnv parses our data into our config
func (s *DockerPushStep) InitEnv(env *Environment) {
	if username, ok := s.data["username"]; ok {
		s.username = env.Interpolate(username)
	}

	if password, ok := s.data["password"]; ok {
		s.password = env.Interpolate(password)
	}

	if email, ok := s.data["email"]; ok {
		s.email = env.Interpolate(email)
	}

	if authServer, ok := s.data["auth-server"]; ok {
		s.authServer = env.Interpolate(authServer)
	}

	if repository, ok := s.data["repository"]; ok {
		s.repository = env.Interpolate(repository)
	}

	if tag, ok := s.data["tag"]; ok {
		s.tag = env.Interpolate(tag)
	}

	if author, ok := s.data["author"]; ok {
		s.author = env.Interpolate(author)
	}

	if message, ok := s.data["message"]; ok {
		s.message = env.Interpolate(message)
	}

	if ports, ok := s.data["ports"]; ok {
		s.ports = env.Interpolate(ports)
	}

	if volumes, ok := s.data["volumes"]; ok {
		s.volumes = env.Interpolate(volumes)
	}

	if registry, ok := s.data["registry"]; ok {
		// s.registry = env.Interpolate(registry)
		s.registry = normalizeRegistry(env.Interpolate(registry))
	} else {
		// s.registry = "https://registry.hub.docker.com"
		s.registry = normalizeRegistry("https://registry.hub.docker.com")
	}

	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.cmd = parts
		}
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

	s.logger.WithFields(LogFields{
		"Registry":   s.registry,
		"Repository": s.repository,
		"Tag":        s.tag,
		"Message":    s.message,
	}).Debug("Push to registry")

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
		Repository: s.repository,
		Tag:        s.tag,
		Registry:   s.registry,
	}

	check, err := client.CheckAccess(checkOpts)
	if err != nil {
		s.logger.Errorln("Error during check access", err)
		return -1, err
	}
	if !check {
		s.logger.Errorln("Not allowed to interact with this repository:", s.repository)
		return -1, fmt.Errorf("Not allowed to interact with this repository: %s", s.repository)
	}

	s.logger.Debugln("Init env:", s.data)

	config := docker.Config{
		Cmd: s.cmd,
	}
	if s.ports != "" {
		parts := strings.Split(s.ports, ",")
		portmap := make(map[docker.Port]struct{})
		for _, port := range parts {
			port = strings.TrimSpace(port)
			if !strings.Contains(port, "/") {
				port = port + "/tcp"
			}
			portmap[docker.Port(port)] = struct{}{}
		}
		config.ExposedPorts = portmap
	}

	if s.volumes != "" {
		parts := strings.Split(s.volumes, ",")
		volumemap := make(map[string]struct{})
		for _, volume := range parts {
			volume = strings.TrimSpace(volume)
			volumemap[volume] = struct{}{}
		}
		config.Volumes = volumemap
	}

	commitOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: s.repository,
		Tag:        s.tag,
		Author:     s.author,
		Message:    s.message,
		Run:        &config,
	}
	s.logger.Debugln("Commit container:", containerID)
	i, err := client.CommitContainer(commitOpts)
	if err != nil {
		return -1, err
	}
	s.logger.WithField("Image", i).Debug("Commit completed")

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()

	// emitStatusses in a different go routine
	go emitStatus(r, s.options)
	defer w.Close()

	pushOpts := docker.PushImageOptions{
		Name:          s.repository,
		Tag:           s.tag,
		Registry:      s.registry,
		OutputStream:  w,
		RawJSONStream: true,
	}

	s.logger.Println("Push container:", s.repository, s.registry)
	err = client.PushImage(pushOpts, auth)

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
