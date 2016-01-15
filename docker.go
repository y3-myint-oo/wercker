package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/CenturyLinkLabs/docker-reg-client/registry"
	dockersignal "github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/flynn/go-shlex"
	"github.com/fsouza/go-dockerclient"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
)

func requireDockerEndpoint(options *DockerOptions) error {
	client, err := NewDockerClient(options)
	if err != nil {
		if err == docker.ErrInvalidEndpoint {
			return fmt.Errorf(`The given Docker endpoint is invalid:
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
			return fmt.Errorf(`You don't seem to have a working Docker environment or wercker can't connect to the Docker endpoint:
	%s
To specify a different endpoint use the DOCKER_HOST environment variable,
or the --docker-host command-line flag.`, options.DockerHost)
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
func (c *DockerClient) AttachInteractive(containerID string, cmd []string, initialStdin []string) error {

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

	// Dump any initial stdin then go into os.Stdin
	readers := []io.Reader{}
	for _, s := range initialStdin {
		if s != "" {
			readers = append(readers, strings.NewReader(s+"\n"))
		}
	}
	readers = append(readers, os.Stdin)
	stdin := io.MultiReader(readers...)

	// This causes our ctrl-c's to be passed to the stuff in the terminal
	var oldState *term.State
	oldState, err = term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	// Handle resizes
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, dockersignal.SIGWINCH)
	go func() {
		for range sigchan {
			c.ResizeTTY(exec.ID)
		}
	}()

	err = c.StartExec(exec.ID, docker.StartExecOptions{
		InputStream:  stdin,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Tty:          true,
		RawTerminal:  true,
	})

	return err
}

// ResizeTTY resizes the tty size of docker connection so output looks normal
func (c *DockerClient) ResizeTTY(execID string) error {
	ws, err := term.GetWinsize(os.Stdout.Fd())
	if err != nil {
		c.logger.Debugln("Error getting term size: %s", err)
		return err
	}
	err = c.ResizeExecTTY(execID, int(ws.Height), int(ws.Width))
	if err != nil {
		c.logger.Debugln("Error resizing term: %s", err)
		return err
	}
	return nil
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

	for strings.Contains(parts[0], ".") {
		parts = parts[1:]
	}

	return strings.Join(parts, "/")
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
	logger := rootLogger.WithField("Logger", "Docker")
	logger.Debug("Checking access for ", opts.Repository)

	// Do the steps described here: https://gist.github.com/termie/bc0334b086697a162f67
	name := normalizeRepo(opts.Repository)
	logger.Debug("Normalized repo ", name)

	auth := registry.BasicAuth{
		Username: opts.Auth.Username,
		Password: opts.Auth.Password,
	}
	client := registry.NewClient()

	reg := normalizeRegistry(opts.Registry)
	logger.Debug("Normalized Registry ", reg)

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

// DockerScratchPushStep creates a new image based on a scratch tarball and
// pushes it
type DockerScratchPushStep struct {
	*DockerPushStep
}

// DockerImageJSON is a minimal JSON description for a docker layer
type DockerImageJSON struct {
	Architecture    string                         `json:"architecture"`
	Created         time.Time                      `json:"created"`
	Config          docker.Config                  `json:"config"`
	Container       string                         `json:"container"`
	ContainerConfig DockerImageJSONContainerConfig `json:"container_config"`
	ID              string                         `json:"id"`
	OS              string                         `json:"os"`
	DockerVersion   string                         `json:"docker_version"`
	Size            int64                          `json:"Size"`
}

// DockerImageJSONContainerConfig substructure
type DockerImageJSONContainerConfig struct {
	Hostname string
	// Cmd      []string
	// Memory int
	// OpenStdin bool
}

// NewDockerScratchPushStep constructorama
func NewDockerScratchPushStep(stepConfig *StepConfig, options *PipelineOptions) (*DockerScratchPushStep, error) {
	name := "docker-scratch-push"
	displayName := "docker scratch'n'push"
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

	dockerPushStep := &DockerPushStep{
		BaseStep: baseStep,
		data:     stepConfig.Data,
		logger:   rootLogger.WithField("Logger", "DockerScratchPushStep"),
	}

	return &DockerScratchPushStep{DockerPushStep: dockerPushStep}, nil
}

// Execute the scratch-n-push
func (s *DockerScratchPushStep) Execute(ctx context.Context, sess *Session) (int, error) {
	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.transport.(*DockerTransport)
	containerID := dt.containerID
	_, err := s.CollectArtifact(containerID)
	if err != nil {
		return -1, err
	}

	// At this point we've written the layer to disk, we're going to add up the
	// sizes of all the files to add to our json format, and sha256 the data
	layerFile, err := os.Open(s.options.HostPath("layer.tar"))
	if err != nil {
		return -1, err
	}
	defer layerFile.Close()

	var layerSize int64

	layerTar := tar.NewReader(layerFile)
	for {
		hdr, err := layerTar.Next()
		if err == io.EOF {
			// finished the tarball
			break
		}
		if err != nil {
			return -1, err
		}

		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		layerSize += hdr.Size
	}

	config := docker.Config{
		Cmd:        s.cmd,
		Entrypoint: s.entrypoint,
		Hostname:   containerID[:16],
		WorkingDir: s.workingDir,
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

	layerID, err := GenerateDockerID()
	if err != nil {
		return -1, err
	}

	// Make the JSON file we need
	imageJSON := DockerImageJSON{
		Architecture: "amd64",
		Container:    containerID,
		ContainerConfig: DockerImageJSONContainerConfig{
			Hostname: containerID[:16],
		},
		DockerVersion: "1.5",
		Created:       time.Now(),
		ID:            layerID,
		OS:            "linux",
		Size:          layerSize,
		Config:        config,
	}

	jsonOut, err := json.MarshalIndent(imageJSON, "", "  ")
	if err != nil {
		return -1, err
	}
	s.logger.Debugln(jsonOut)

	// Write out the files to disk that we are going to care about
	err = os.MkdirAll(s.options.HostPath("scratch", layerID), 0755)
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(s.options.HostPath("scratch"))

	// VERSION file
	versionFile, err := os.OpenFile(s.options.HostPath("scratch", layerID, "VERSION"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer versionFile.Close()
	_, err = versionFile.Write([]byte("1.0"))
	if err != nil {
		return -1, err
	}
	err = versionFile.Sync()
	if err != nil {
		return -1, err
	}

	// json file
	jsonFile, err := os.OpenFile(s.options.HostPath("scratch", layerID, "json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer jsonFile.Close()
	_, err = jsonFile.Write(jsonOut)
	if err != nil {
		return -1, err
	}
	err = jsonFile.Sync()
	if err != nil {
		return -1, err
	}

	// repositories file
	repositoriesFile, err := os.OpenFile(s.options.HostPath("scratch", "repositories"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer repositoriesFile.Close()
	_, err = repositoriesFile.Write([]byte(fmt.Sprintf(`{"%s":{"%s":"%s"}}`, s.repository, s.tag, layerID)))
	if err != nil {
		return -1, err
	}
	err = repositoriesFile.Sync()
	if err != nil {
		return -1, err
	}

	// layer.tar has an extra folder in it so we have to strip it :/
	tempLayerFile, err := os.Open(s.options.HostPath("layer.tar"))
	if err != nil {
		return -1, err
	}
	defer os.Remove(s.options.HostPath("layer.tar"))
	defer tempLayerFile.Close()

	realLayerFile, err := os.OpenFile(s.options.HostPath("scratch", layerID, "layer.tar"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer realLayerFile.Close()

	tr := tar.NewReader(tempLayerFile)
	tw := tar.NewWriter(realLayerFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// finished the tarball
			break
		}
		if err != nil {
			return -1, err
		}
		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}
		if strings.HasPrefix(hdr.Name, "output/") {
			hdr.Name = hdr.Name[len("output/"):]
		} else if strings.HasPrefix(hdr.Name, "source/") {
			hdr.Name = hdr.Name[len("source/"):]
		}
		if len(hdr.Name) == 0 {
			continue
		}
		tw.WriteHeader(hdr)
		_, err = io.Copy(tw, tr)
		if err != nil {
			return -1, err
		}
	}
	tw.Close()

	// Build our output tarball and start writing to it
	imageFile, err := os.Create(s.options.HostPath("scratch.tar"))
	defer imageFile.Close()
	if err != nil {
		return -1, err
	}
	err = tarPath(imageFile, s.options.HostPath("scratch"))
	if err != nil {
		return -1, err
	}
	imageFile.Close()

	// --exhale--

	// Okay, we have the tarball, we now need to do the whole load and push part.
	// For the moment this is copy-pasta from DockerPushStep
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
	}).Debug("Scratch push to registry")

	// Check the auth
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

	// Okay, we can access it, do a docker load to import the image then push it
	loadFile, err := os.Open(s.options.HostPath("scratch.tar"))
	defer loadFile.Close()
	err = client.LoadImage(docker.LoadImageOptions{InputStream: loadFile})
	if err != nil {
		return -1, err
	}

	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()
	e, err := EmitterFromContext(ctx)
	if err != nil {
		return -1, err
	}

	// emitStatusses in a different go routine
	go EmitStatus(e, r, s.options)
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

// CollectArtifact is copied from the build, we use this to get the layer
// tarball that we'll include in the image tarball
func (s *DockerScratchPushStep) CollectArtifact(containerID string) (*Artifact, error) {
	artificer := NewArtificer(s.options)

	// Ensure we have the host directory

	artifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.GuestPath("output"),
		HostPath:      s.options.HostPath("layer.tar"),
		ApplicationID: s.options.ApplicationID,
		BuildID:       s.options.PipelineID,
		Bucket:        s.options.S3Bucket,
	}

	sourceArtifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.SourcePath(),
		HostPath:      s.options.HostPath("layer.tar"),
		ApplicationID: s.options.ApplicationID,
		BuildID:       s.options.PipelineID,
		Bucket:        s.options.S3Bucket,
	}

	// Get the output dir, if it is empty grab the source dir.
	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		if err == ErrEmptyTarball {
			fullArtifact, err = artificer.Collect(sourceArtifact)
			if err != nil {
				return nil, err
			}
			return fullArtifact, nil
		}
		return nil, err
	}

	return fullArtifact, nil
}

// DockerPushStep needs to implemenet IStep
type DockerPushStep struct {
	*BaseStep
	data       map[string]string
	username   string
	password   string
	email      string
	env        []string
	stopSignal string
	labels     map[string]string
	user       string
	authServer string
	repository string
	author     string
	message    string
	tag        string
	registry   string
	ports      string
	volumes    string
	cmd        []string
	entrypoint []string
	logger     *LogEntry
	workingDir string
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

	if workingDir, ok := s.data["working-dir"]; ok {
		s.workingDir = env.Interpolate(workingDir)
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

	if entrypoint, ok := s.data["entrypoint"]; ok {
		parts, err := shlex.Split(entrypoint)
		if err == nil {
			s.entrypoint = parts
		}
	}

	if env, ok := s.data["env"]; ok {
		s.env = env.Interpolate(env)
	}

	if stopsignal, ok := s.data["stopsignal"]; ok {
		s.stopsignal = env.Interpolate(stopsignal)
	}

	if labels, ok := s.data["labels"]; ok {
		s.labels = env.Interpolate(labels)
	}

	if user, ok := s.data["user"]; ok {
		s.user = env.Interpolate(user)
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
	e, err := EmitterFromContext(ctx)
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

	if !s.options.DockerLocal {
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
	}

	s.logger.Debugln("Init env:", s.data)

	config := docker.Config{
		Cmd:        s.cmd,
		Entrypoint: s.entrypoint,
		WorkingDir: s.workingDir,
		User:       s.user,
		Env:        s.env,
		StopSignal: s.stopSignal,
		Labels:     s.labels,
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

	if !s.options.DockerLocal {
		// Create a pipe since we want a io.Reader but Docker expects a io.Writer
		r, w := io.Pipe()

		// emitStatusses in a different go routine
		go EmitStatus(e, r, s.options)
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

// ShouldSyncEnv before running this step = TRUE
func (s *DockerPushStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}
