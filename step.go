package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/fsouza/go-dockerclient"
	"github.com/termie/go-shutil"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

// StepDesc represents a wercker-step.yml
type StepDesc struct {
	Name        string
	Version     string
	Description string
	Keywords    []string
	Properties  map[string]StepDescProperty
}

// StepDescProperty is the structure of the values in the "properties"
// section of the config
type StepDescProperty struct {
	Default  string
	Required bool
	Type     string
}

// ReadStepDesc reads a file, expecting it to be parsed into a StepDesc.
func ReadStepDesc(descPath string) (*StepDesc, error) {
	file, err := ioutil.ReadFile(descPath)
	if err != nil {
		return nil, err
	}

	var m StepDesc
	err = yaml.Unmarshal(file, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Defaults returns the default properties for a step as a map.
func (sc *StepDesc) Defaults() map[string]string {
	m := make(map[string]string)
	if sc == nil || sc.Properties == nil {
		return m
	}
	for k, v := range sc.Properties {
		m[k] = v.Default
	}
	return m
}

// IStep interface for steps, to be renamed
type IStep interface {
	// Bunch of getters
	DisplayName() string
	Env() *Environment
	Cwd() string
	ID() string
	Name() string
	Owner() string
	SafeID() string
	Version() string

	// Actual methods
	Fetch() (string, error)

	InitEnv(*Environment)
	Execute(context.Context, *Session) (int, error)
	CollectFile(string, string, string, io.Writer) error
	CollectArtifact(string) (*Artifact, error)
	// TODO(termie): don't think this needs to be universal
	ReportPath(...string) string
}

// BaseStep type for extending
type BaseStep struct {
	displayName string
	env         *Environment
	id          string
	name        string
	options     *PipelineOptions
	owner       string
	safeID      string
	version     string
	cwd         string
}

// DisplayName getter
func (s *BaseStep) DisplayName() string {
	return s.displayName
}

// Env getter
func (s *BaseStep) Env() *Environment {
	return s.env
}

// Cwd getter
func (s *BaseStep) Cwd() string {
	return s.cwd
}

// ID getter
func (s *BaseStep) ID() string {
	return s.id
}

// Name getter
func (s *BaseStep) Name() string {
	return s.name
}

// Owner getter
func (s *BaseStep) Owner() string {
	return s.owner
}

// SafeID getter
func (s *BaseStep) SafeID() string {
	return s.safeID
}

// Version getter
func (s *BaseStep) Version() string {
	return s.version
}

// Step is the holder of the Step methods.
type Step struct {
	*BaseStep
	url      string
	data     map[string]string
	stepDesc *StepDesc
	logger   *LogEntry
}

// ToSteps builds a list of steps from RawStepsConfig
func (s RawStepsConfig) ToSteps(options *PipelineOptions) ([]IStep, error) {
	steps := []IStep{}
	for _, stepConfig := range s {
		step, err := stepConfig.ToStep(options)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// ToStep converts a StepConfig into a Step.
func (s *StepConfig) ToStep(options *PipelineOptions) (IStep, error) {

	// NOTE(termie) Special case steps are special
	if s.ID == "internal/docker-push" {
		return NewDockerPushStep(s, options)
	}
	if s.ID == "internal/docker-scratch-push" {
		return NewDockerScratchPushStep(s, options)
	}
	if options.EnableDevSteps {
		if s.ID == "internal/watch" {
			return NewWatchStep(s, options)
		}
		if s.ID == "internal/shell" {
			return NewShellStep(s, options)
		}
	return NewStep(s, options)
}

// NewStep sets up the basic parts of a Step.
// Step names can come in a couple forms (x means currently supported):
//   x setup-go-environment (fetches from api)
//   x wercker/hipchat-notify (fetches from api)
//   x wercker/hipchat-notify "http://someurl/thingee.tar" (downloads tarball)
//   x setup-go-environment "file:///some_path" (uses local path)
func NewStep(stepConfig *StepConfig, options *PipelineOptions) (*Step, error) {
	var identifier string
	var name string
	var owner string
	var version string

	url := ""

	stepID := stepConfig.ID
	data := stepConfig.Data

	// Check for urls
	_, err := fmt.Sscanf(stepID, "%s %q", &identifier, &url)
	if err != nil {
		// There was probably no url part
		identifier = stepID
	}

	// Check for owner/name
	parts := strings.SplitN(identifier, "/", 2)
	if len(parts) > 1 {
		owner = parts[0]
		name = parts[1]
	} else {
		// No owner, "wercker" is the default
		owner = "wercker"
		name = identifier
	}

	versionParts := strings.SplitN(name, "@", 2)
	if len(versionParts) == 2 {
		name = versionParts[0]
		version = versionParts[1]
	} else {
		version = "*"
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	// Script steps need unique IDs
	if name == "script" {
		stepID = uuid.NewRandom().String()
		version = Version()
	}

	// If there is a name in data, make it our displayName and delete it
	displayName := stepConfig.Name
	if displayName == "" {
		displayName = name
	}

	logger := rootLogger.WithFields(LogFields{
		"Logger": "Step",
		"SafeID": stepSafeID,
	})

	return &Step{
		BaseStep: &BaseStep{
			displayName: displayName,
			env:         NewEnvironment(),
			id:          identifier,
			name:        name,
			options:     options,
			owner:       owner,
			safeID:      stepSafeID,
			version:     version,
			cwd:         stepConfig.Cwd,
		},
		data:   data,
		url:    url,
		logger: logger,
	}, nil
}

// IsScript should probably not be exported.
func (s *Step) IsScript() bool {
	return s.name == "script"
}

func normalizeCode(code string) string {
	if !strings.HasPrefix(code, "#!") {
		code = strings.Join([]string{
			"set -e",
			code,
		}, "\n")
	}
	return code
}

// FetchScript turns the raw code in a step into a shell file.
func (s *Step) FetchScript() (string, error) {
	hostStepPath := s.options.HostPath(s.safeID)
	scriptPath := s.options.HostPath(s.safeID, "run.sh")
	content := normalizeCode(s.data["code"])

	err := os.MkdirAll(hostStepPath, 0755)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(scriptPath, []byte(content), 0755)
	if err != nil {
		return "", err
	}

	return hostStepPath, nil
}

// Fetch grabs the Step content (or calls FetchScript for script steps).
func (s *Step) Fetch() (string, error) {
	// NOTE(termie): polymorphism based on kind, we could probably do something
	//               with interfaces here, but this is okay for now
	if s.IsScript() {
		return s.FetchScript()
	}

	stepPath := filepath.Join(s.options.StepDir, s.CachedName())
	stepExists, err := exists(stepPath)
	if err != nil {
		return "", err
	}

	if !stepExists {
		// If we don't have a url already
		if s.url == "" {
			// Grab the info about the step from the api
			client := NewAPIClient(s.options.GlobalOptions)
			stepInfo, err := client.GetStepVersion(s.Owner(), s.Name(), s.Version())
			if err != nil {
				return "", err
			}

			s.url = stepInfo.TarballURL
		}

		// If we have a file uri let's just copytree it.
		if strings.HasPrefix(s.url, "file:///") {
			localPath := s.url[len("file://"):]
			err = shutil.CopyTree(localPath, stepPath, nil)
		} else {
			// Grab the tarball and untargzip it
			resp, err := fetchTarball(s.url)
			if err != nil {
				return "", err
			}

			// Assuming we have a gzip'd tarball at this point
			err = untargzip(stepPath, resp.Body)
			if err != nil {
				return "", err
			}
		}
	}

	hostStepPath := s.HostPath()

	err = shutil.CopyTree(stepPath, hostStepPath, nil)
	if err != nil {
		return "", nil
	}

	// Now that we have the code, load any step config we might find
	desc, err := ReadStepDesc(s.HostPath("wercker-step.yml"))
	if err != nil && !os.IsNotExist(err) {
		// TODO(termie): Log an error instead of printing
		s.logger.Println("ERROR: Reading wercker-step.yml:", err)
	}
	if err == nil {
		s.stepDesc = desc
	}
	return hostStepPath, nil
}

// SetupGuest ensures that the guest is ready to run a Step.
func (s *Step) SetupGuest(sessionCtx context.Context, sess *Session) error {
	// TODO(termie): can this even fail? i.e. exit code != 0
	sess.HideLogs()
	defer sess.ShowLogs()
	_, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`mkdir -p "%s"`, s.ReportPath("artifacts")))
	_, _, err = sess.SendChecked(sessionCtx, "set +e")
	_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cp -r "%s" "%s"`, s.MntPath(), s.GuestPath()))
	_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cd $WERCKER_SOURCE_DIR`))
	if s.Cwd() != "" {
		_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cd "%s"`, s.Cwd()))
	}
	return err
}

// Execute actually sends the commands for the step.
func (s *Step) Execute(sessionCtx context.Context, sess *Session) (int, error) {
	err := s.SetupGuest(sessionCtx, sess)
	if err != nil {
		return 1, err
	}
	_, _, err = sess.SendChecked(sessionCtx, s.env.Export()...)
	if err != nil {
		return 1, err
	}

	if yes, _ := exists(s.HostPath("init.sh")); yes {
		exit, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`source "%s"`, s.GuestPath("init.sh")))
		if exit != 0 {
			return exit, errors.New("Ack!")
		}
		if err != nil {
			return 1, err
		}
	}

	if yes, _ := exists(s.HostPath("run.sh")); yes {
		exit, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`source "%s" < /dev/null`, s.GuestPath("run.sh")))
		return exit, err
	}

	return 0, nil
}

// CollectFile gets an individual file from the container
func (s *Step) CollectFile(containerID, path, name string, dst io.Writer) error {
	client, err := NewDockerClient(s.options.DockerOptions)
	if err != nil {
		return err
	}

	pipeReader, pipeWriter := io.Pipe()
	opts := docker.CopyFromContainerOptions{
		OutputStream: pipeWriter,
		Container:    containerID,
		Resource:     filepath.Join(path, name),
	}

	errs := make(chan error)
	go func() {
		defer close(errs)
		errs <- untarOne(name, dst, pipeReader)
	}()

	if err = client.CopyFromContainer(opts); err != nil {
		s.logger.Debug("Probably expected error:", err)
		return ErrEmptyTarball
	}

	return <-errs
}

// CollectArtifact copies the artifacts associated with the Step.
func (s *Step) CollectArtifact(containerID string) (*Artifact, error) {
	artificer := NewArtificer(s.options)

	// Ensure we have the host directory

	artifact := &Artifact{
		ContainerID:   containerID,
		GuestPath:     s.ReportPath("artifacts"),
		HostPath:      s.options.HostPath("artifacts", s.safeID, "artifacts.tar"),
		ApplicationID: s.options.ApplicationID,
		BuildID:       s.options.BuildID,
		DeployID:      s.options.DeployID,
		BuildStepID:   s.safeID,
		Bucket:        s.options.S3Bucket,
	}

	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		if err == ErrEmptyTarball {
			return nil, nil
		}
		return nil, err
	}

	return fullArtifact, nil
}

// InitEnv sets up the internal environment for the Step.
func (s *Step) InitEnv(env *Environment) {
	a := [][]string{
		[]string{"WERCKER_STEP_ROOT", s.GuestPath()},
		[]string{"WERCKER_STEP_ID", s.safeID},
		[]string{"WERCKER_STEP_OWNER", s.owner},
		[]string{"WERCKER_STEP_NAME", s.name},
		[]string{"WERCKER_REPORT_NUMBERS_FILE", s.ReportPath("numbers.ini")},
		[]string{"WERCKER_REPORT_MESSAGE_FILE", s.ReportPath("message.txt")},
		[]string{"WERCKER_REPORT_ARTIFACTS_DIR", s.ReportPath("artifacts")},
	}
	s.Env().Update(a)

	defaults := s.stepDesc.Defaults()

	for k, defaultValue := range defaults {
		value, ok := s.data[k]
		key := fmt.Sprintf("WERCKER_%s_%s", s.name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		if !ok {
			s.Env().Add(key, defaultValue)
		} else {
			s.Env().Add(key, value)
		}
	}

	for k, value := range s.data {
		if k == "code" || k == "name" {
			continue
		}
		key := fmt.Sprintf("WERCKER_%s_%s", s.name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		s.Env().Add(key, value)
	}
}

// CachedName returns a name suitable for caching
func (s *Step) CachedName() string {
	name := fmt.Sprintf("%s-%s", s.owner, s.name)
	if s.version != "*" {
		name = fmt.Sprintf("%s@%s", name, s.version)
	}
	return name
}

// HostPath returns a path relative to the Step on the host.
func (s *Step) HostPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.HostPath(newArgs...)
}

// GuestPath returns a path relative to the Step on the guest.
func (s *Step) GuestPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.GuestPath(newArgs...)
}

// MntPath returns a path relative to the read-only mount of the Step on
// the guest.
func (s *Step) MntPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.MntPath(newArgs...)
}

// ReportPath returns a path to the reports for the step on the guest.
func (s *Step) ReportPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.ReportPath(newArgs...)
}

// NewWerckerInitStep returns our fake initial step
func NewWerckerInitStep(options *PipelineOptions) (*Step, error) {
	werckerInit := `wercker-init "https://api.github.com/repos/wercker/wercker-init/tarball"`
	stepConfig := &StepConfig{ID: werckerInit, Data: make(map[string]string)}
	initStep, err := NewStep(stepConfig, options)
	if err != nil {
		return nil, err
	}
	return initStep, nil
}
