package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/codegangsta/cli"
	"github.com/pborman/uuid"
)

// GlobalOptions applicable to everything
type GlobalOptions struct {
	BaseURL    string
	Debug      bool
	Journal    bool
	Verbose    bool
	ShowColors bool

	// Auth
	AuthToken      string
	AuthTokenStore string
}

// guessAuthToken will attempt to read from the token store location if
// no auth token was provided
func guessAuthToken(c *cli.Context, e *Environment, authTokenStore string) string {
	token := c.GlobalString("auth-token")
	if token != "" {
		return token
	}
	if foundToken, _ := exists(authTokenStore); !foundToken {
		return ""
	}

	tokenBytes, err := ioutil.ReadFile(authTokenStore)
	if err != nil {
		rootLogger.WithField("Logger", "Options").Errorln(err)
		return ""
	}
	return strings.TrimSpace(string(tokenBytes))
}

// NewGlobalOptions constructor
func NewGlobalOptions(c *cli.Context, e *Environment) (*GlobalOptions, error) {
	baseURL := strings.TrimRight(c.GlobalString("base-url"), "/")
	debug := c.GlobalBool("debug")
	journal := c.GlobalBool("journal")
	verbose := c.GlobalBool("verbose")
	showColors := !c.GlobalBool("no-colors")

	authTokenStore := expandHomePath(c.GlobalString("auth-token-store"), e.Get("HOME"))
	authToken := guessAuthToken(c, e, authTokenStore)

	// If debug is true, than force verbose and do not use colors.
	if debug {
		verbose = true
		showColors = false
	}

	return &GlobalOptions{
		BaseURL:    baseURL,
		Debug:      debug,
		Journal:    journal,
		Verbose:    verbose,
		ShowColors: showColors,

		AuthToken:      authToken,
		AuthTokenStore: authTokenStore,
	}, nil
}

// AWSOptions for our artifact storage
type AWSOptions struct {
	*GlobalOptions
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	S3Bucket           string
	S3PartSize         int64
}

// NewAWSOptions constructor
func NewAWSOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*AWSOptions, error) {
	awsAccessKeyID := c.String("aws-access-key")
	awsRegion := c.String("aws-region")
	awsSecretAccessKey := c.String("aws-secret-key")
	s3Bucket := c.String("s3-bucket")

	return &AWSOptions{
		GlobalOptions:      globalOpts,
		AWSAccessKeyID:     awsAccessKeyID,
		AWSRegion:          awsRegion,
		AWSSecretAccessKey: awsSecretAccessKey,
		S3Bucket:           s3Bucket,
		S3PartSize:         100 * 1024 * 1024, // 100 MB
	}, nil
}

// DockerOptions for our docker client
type DockerOptions struct {
	*GlobalOptions
	DockerHost      string
	DockerTLSVerify string
	DockerCertPath  string
	DockerDNS       []string
	DockerLocal     bool
}

func guessAndUpdateDockerOptions(opts *DockerOptions, e *Environment) {
	if opts.DockerHost != "" {
		return
	}

	logger := rootLogger.WithField("Logger", "docker")
	f := &Formatter{opts.GlobalOptions}

	// Check the unix socket, default on linux
	// This will fail instantly so don't bother with the goroutine
	if runtime.GOOS == "linux" {
		unixSocket := "unix:///var/run/docker.sock"
		logger.Println(f.Info("No Docker host specified, checking", unixSocket))
		client, err := NewDockerClient(&DockerOptions{
			DockerHost: unixSocket,
		})
		if err == nil {
			_, err = client.Version()
			if err == nil {
				opts.DockerHost = unixSocket
				return
			}
		}
	}

	// Check the boot2docker port with default cert paths and such
	b2dCertPath := filepath.Join(e.Get("HOME"), ".boot2docker/certs/boot2docker-vm")
	b2dHost := "tcp://192.168.59.103:2376"

	logger.Printf(f.Info("No Docker host specified, checking for boot2docker", b2dHost))
	client, err := NewDockerClient(&DockerOptions{
		DockerHost:      b2dHost,
		DockerCertPath:  b2dCertPath,
		DockerTLSVerify: "1",
	})
	if err == nil {
		// This can take a long time if it isn't up, so toss it in a
		// goroutine so we can time it out
		result := make(chan bool)
		go func() {
			_, err = client.Version()
			if err == nil {
				result <- true
			} else {
				result <- false
			}
		}()
		select {
		case success := <-result:
			if success {
				opts.DockerHost = b2dHost
				opts.DockerCertPath = b2dCertPath
				opts.DockerTLSVerify = "1"
				return
			}
		case <-time.After(1 * time.Second):
		}
	}

	// Pick a default localhost port and hope for the best :/
	opts.DockerHost = "tcp://127.0.0.1:2375"
	logger.Println(f.Info("No Docker host found, falling back to default", opts.DockerHost))
}

// NewDockerOptions constructor
func NewDockerOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*DockerOptions, error) {
	dockerHost := c.String("docker-host")
	dockerTLSVerify := c.String("docker-tls-verify")
	dockerCertPath := c.String("docker-cert-path")
	dockerDNS := c.StringSlice("docker-dns")
	dockerLocal := c.Bool("docker-local")

	speculativeOptions := &DockerOptions{
		GlobalOptions:   globalOpts,
		DockerHost:      dockerHost,
		DockerTLSVerify: dockerTLSVerify,
		DockerCertPath:  dockerCertPath,
		DockerDNS:       dockerDNS,
		DockerLocal:     dockerLocal,
	}

	// We're going to try out a few settings and set DockerHost if
	// one of them works, it they don't we'll get a nice error when
	// requireDockerEndpoint triggers later on
	guessAndUpdateDockerOptions(speculativeOptions, e)
	return speculativeOptions, nil
}

// GitOptions for the users, mostly
type GitOptions struct {
	*GlobalOptions
	GitBranch     string
	GitCommit     string
	GitDomain     string
	GitOwner      string
	GitRepository string
}

func guessGitBranch(c *cli.Context, e *Environment) string {
	branch := c.String("git-branch")
	if branch != "" {
		return branch
	}

	projectPath := guessProjectPath(c, e)
	if projectPath == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	defer os.Chdir(cwd)
	os.Chdir(projectPath)

	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	var out bytes.Buffer
	cmd := exec.Command(git, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return ""
	}
	return strings.Trim(out.String(), "\n")
}

func guessGitCommit(c *cli.Context, e *Environment) string {
	commit := c.String("git-commit")
	if commit != "" {
		return commit
	}

	projectPath := guessProjectPath(c, e)
	if projectPath == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	defer os.Chdir(cwd)
	os.Chdir(projectPath)

	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	var out bytes.Buffer
	cmd := exec.Command(git, "rev-parse", "HEAD")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return ""
	}
	return strings.Trim(out.String(), "\n")
}

func guessGitOwner(c *cli.Context, e *Environment) string {
	owner := c.String("git-owner")
	if owner != "" {
		return owner
	}

	u, err := user.Current()
	if err == nil {
		owner = u.Username
	}
	return owner
}

func guessGitRepository(c *cli.Context, e *Environment) string {
	repository := c.String("git-repository")
	if repository != "" {
		return repository
	}
	// repository, err := guessApplicationName(c, env)
	// if err != nil {
	//   return ""
	// }
	return repository
}

// NewGitOptions constructor
func NewGitOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*GitOptions, error) {
	gitBranch := guessGitBranch(c, e)
	gitCommit := guessGitCommit(c, e)
	gitDomain := c.String("git-domain")
	gitOwner := guessGitOwner(c, e)
	gitRepository := guessGitRepository(c, e)

	return &GitOptions{
		GlobalOptions: globalOpts,
		GitBranch:     gitBranch,
		GitCommit:     gitCommit,
		GitDomain:     gitDomain,
		GitOwner:      gitOwner,
		GitRepository: gitRepository,
	}, nil
}

// KeenOptions for our metrics
type KeenOptions struct {
	*GlobalOptions
	KeenProjectID       string
	KeenProjectWriteKey string
	ShouldKeenMetrics   bool
}

// NewKeenOptions constructor
func NewKeenOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*KeenOptions, error) {
	keenMetrics := c.Bool("keen-metrics")
	keenProjectWriteKey := c.String("keen-project-write-key")
	keenProjectID := c.String("keen-project-id")

	if keenMetrics {
		if keenProjectWriteKey == "" {
			return nil, errors.New("keen-project-write-key is required")
		}

		if keenProjectID == "" {
			return nil, errors.New("keen-project-id is required")
		}
	}

	return &KeenOptions{
		GlobalOptions:       globalOpts,
		KeenProjectID:       keenProjectID,
		KeenProjectWriteKey: keenProjectWriteKey,
		ShouldKeenMetrics:   keenMetrics,
	}, nil
}

// ReporterOptions for our reporting
type ReporterOptions struct {
	*GlobalOptions
	ReporterHost string
	ReporterKey  string
	ShouldReport bool
}

// NewReporterOptions constructor
func NewReporterOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*ReporterOptions, error) {
	shouldReport := c.Bool("report")
	reporterHost := c.String("wercker-host")
	reporterKey := c.String("wercker-token")

	if shouldReport {
		if reporterKey == "" {
			return nil, errors.New("wercker-token is required")
		}

		if reporterHost == "" {
			return nil, errors.New("wercker-host is required")
		}
	}

	return &ReporterOptions{
		GlobalOptions: globalOpts,
		ReporterHost:  reporterHost,
		ReporterKey:   reporterKey,
		ShouldReport:  shouldReport,
	}, nil
}

// PipelineOptions for builds and deploys
type PipelineOptions struct {
	*GlobalOptions
	*AWSOptions
	*DockerOptions
	*GitOptions
	*KeenOptions
	*ReporterOptions

	// TODO(termie): i'd like to remove this, it is only used in a couple
	//               places by BasePipeline
	HostEnv *Environment

	BuildID      string
	DeployID     string
	PipelineID   string
	DeployTarget string
	Pipeline     string

	ApplicationID            string
	ApplicationName          string
	ApplicationOwnerName     string
	ApplicationStartedByName string

	ShouldCommit     bool
	Tag              string
	Message          string
	ShouldStoreLocal bool
	ShouldStoreS3    bool

	BuildDir     string
	CacheDir     string
	ContainerDir string
	ProjectDir   string
	StepDir      string

	GuestRoot  string
	MntRoot    string
	ReportRoot string

	ProjectID   string
	ProjectURL  string
	ProjectPath string

	CommandTimeout    int
	NoResponseTimeout int
	ShouldArtifacts   bool
	ShouldRemove      bool
	SourceDir         string

	AttachOnError  bool
	DirectMount    bool
	EnableDevSteps bool
	PublishPorts   []string
	WerckerYml     string
}

func guessApplicationID(c *cli.Context, e *Environment, name string) string {
	id := c.String("application-id")
	if id == "" {
		id = name
	}
	return id
}

// Some logic to guess the application name
func guessApplicationName(c *cli.Context, e *Environment) (string, error) {
	applicationName := c.String("application-name")
	if applicationName != "" {
		return applicationName, nil
	}

	// Otherwise, check our build target, it can be a url...
	target := c.Args().First()
	projectURL := ""
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		projectURL = target
		base := path.Base(projectURL)
		// Special handling for github tarballs
		if base == "tarball" {
			base = path.Base(path.Dir(projectURL))
		}
		ext := path.Ext(base)
		base = base[:len(ext)]
		return base, nil
	}

	// ... or a file path
	if target == "" {
		target = "."
	}
	stat, err := os.Stat(target)
	if err != nil || !stat.IsDir() {
		return "", fmt.Errorf("target '%s' is not a directory", target)
	}
	abspath, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Base(abspath), nil
}

func guessApplicationOwnerName(c *cli.Context, e *Environment) string {
	name := c.String("application-owner-name")
	if name == "" {
		u, err := user.Current()
		if err == nil {
			name = u.Username
		}
	}
	if name == "" {
		name = "wercker"
	}
	return name
}

func guessMessage(c *cli.Context, e *Environment) string {
	message := c.String("message")
	return message
}

func guessTag(c *cli.Context, e *Environment) string {
	tag := c.String("tag")
	if tag == "" {
		tag = guessGitBranch(c, e)
	}
	tag = strings.Replace(tag, "/", "_", -1)
	return tag
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func guessProjectID(c *cli.Context, e *Environment) string {
	projectID := c.String("project-id")
	if projectID != "" {
		return projectID
	}

	// If this was going to fail it already failed and we exited
	name, _ := guessApplicationName(c, e)
	return name
}

func guessProjectPath(c *cli.Context, e *Environment) string {
	target := c.Args().First()
	if looksLikeURL(target) {
		return ""
	}
	if target == "" {
		target = "."
	}
	abs, _ := filepath.Abs(target)
	return abs
}

func guessProjectURL(c *cli.Context, e *Environment) string {
	target := c.Args().First()
	if !looksLikeURL(target) {
		return ""
	}
	return target
}

// NewPipelineOptions big-ass constructor
func NewPipelineOptions(c *cli.Context, e *Environment) (*PipelineOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	dockerOpts, err := NewDockerOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	awsOpts, err := NewAWSOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	gitOpts, err := NewGitOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	keenOpts, err := NewKeenOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	reporterOpts, err := NewReporterOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	buildID := c.String("build-id")
	deployID := c.String("deploy-id")
	pipelineID := ""
	if deployID != "" {
		pipelineID = deployID
	} else {
		pipelineID = buildID
	}
	deployTarget := c.String("deploy-target")
	pipeline := c.String("pipeline")

	applicationName, err := guessApplicationName(c, e)
	if err != nil {
		return nil, err
	}
	applicationID := guessApplicationID(c, e, applicationName)
	applicationOwnerName := guessApplicationOwnerName(c, e)
	applicationStartedByName := c.String("application-started-by-name")
	if applicationStartedByName == "" {
		applicationStartedByName = applicationOwnerName
	}

	shouldCommit := c.Bool("commit")
	tag := guessTag(c, e)
	message := guessMessage(c, e)
	shouldStoreLocal := c.Bool("store-local")
	shouldStoreS3 := c.Bool("store-s3")

	buildDir, _ := filepath.Abs(c.String("build-dir"))
	cacheDir, _ := filepath.Abs(c.String("cache-dir"))
	containerDir, _ := filepath.Abs(c.String("container-dir"))
	projectDir, _ := filepath.Abs(c.String("project-dir"))
	stepDir, _ := filepath.Abs(c.String("step-dir"))

	guestRoot := c.String("guest-root")
	mntRoot := c.String("mnt-root")
	reportRoot := c.String("report-root")

	projectID := guessProjectID(c, e)
	projectPath := guessProjectPath(c, e)
	projectURL := guessProjectURL(c, e)

	// These timeouts are given in minutes but we store them as milliseconds
	commandTimeout := int(c.Float64("command-timeout") * 1000 * 60)
	noResponseTimeout := int(c.Float64("no-response-timeout") * 1000 * 60)
	shouldArtifacts := c.Bool("artifacts")
	shouldRemove := !c.Bool("no-remove")
	sourceDir := c.String("source-dir")

	attachOnError := c.Bool("attach-on-error")
	directMount := c.Bool("direct-mount")
	enableDevSteps := c.Bool("enable-dev-steps")
	publishPorts := c.StringSlice("publish")
	werckerYml := c.String("wercker-yml")

	return &PipelineOptions{
		GlobalOptions:   globalOpts,
		AWSOptions:      awsOpts,
		DockerOptions:   dockerOpts,
		GitOptions:      gitOpts,
		KeenOptions:     keenOpts,
		ReporterOptions: reporterOpts,

		HostEnv: e,

		BuildID:      buildID,
		DeployID:     deployID,
		PipelineID:   pipelineID,
		DeployTarget: deployTarget,
		Pipeline:     pipeline,

		ApplicationID:            applicationID,
		ApplicationName:          applicationName,
		ApplicationOwnerName:     applicationOwnerName,
		ApplicationStartedByName: applicationStartedByName,

		Message:          message,
		Tag:              tag,
		ShouldCommit:     shouldCommit,
		ShouldStoreLocal: shouldStoreLocal,
		ShouldStoreS3:    shouldStoreS3,

		BuildDir:     buildDir,
		CacheDir:     cacheDir,
		ContainerDir: containerDir,
		ProjectDir:   projectDir,
		StepDir:      stepDir,

		GuestRoot:  guestRoot,
		MntRoot:    mntRoot,
		ReportRoot: reportRoot,

		ProjectID:   projectID,
		ProjectURL:  projectURL,
		ProjectPath: projectPath,

		CommandTimeout:    commandTimeout,
		NoResponseTimeout: noResponseTimeout,
		ShouldArtifacts:   shouldArtifacts,
		ShouldRemove:      shouldRemove,
		SourceDir:         sourceDir,

		AttachOnError:  attachOnError,
		DirectMount:    directMount,
		EnableDevSteps: enableDevSteps,
		PublishPorts:   publishPorts,
		WerckerYml:     werckerYml,
	}, nil
}

// SourcePath returns the path to the source dir
func (o *PipelineOptions) SourcePath() string {
	return o.GuestPath("source", o.SourceDir)
}

// HostPath returns a path relative to the build root on the host.
func (o *PipelineOptions) HostPath(s ...string) string {
	return path.Join(o.BuildDir, o.PipelineID, path.Join(s...))
}

// GuestPath returns a path relative to the build root on the guest.
func (o *PipelineOptions) GuestPath(s ...string) string {
	return path.Join(o.GuestRoot, path.Join(s...))
}

// MntPath returns a path relative to the read-only mount root on the guest.
func (o *PipelineOptions) MntPath(s ...string) string {
	return path.Join(o.MntRoot, path.Join(s...))
}

// ReportPath returns a path relative to the report root on the guest.
func (o *PipelineOptions) ReportPath(s ...string) string {
	return path.Join(o.ReportRoot, path.Join(s...))
}

// dumpOptions prints out a sorted list of options
func dumpOptions(options interface{}, indent ...string) {
	indent = append(indent, "  ")
	s := reflect.ValueOf(options).Elem()
	typeOfT := s.Type()
	names := []string{}
	for i := 0; i < s.NumField(); i++ {
		// f := s.Field(i)
		fieldName := typeOfT.Field(i).Name
		if fieldName != "HostEnv" {
			names = append(names, fieldName)
		}
	}
	sort.Strings(names)
	logger := rootLogger.WithField("Logger", "Options")

	for _, name := range names {
		r := reflect.ValueOf(options)
		f := reflect.Indirect(r).FieldByName(name)
		if strings.HasSuffix(name, "Options") {
			if len(indent) > 1 && name == "GlobalOptions" {
				continue
			}
			logger.Debugln(fmt.Sprintf("%s%s %s", strings.Join(indent, ""), name, f.Type()))
			dumpOptions(f.Interface(), indent...)
		} else {
			logger.Debugln(fmt.Sprintf("%s%s %s = %v", strings.Join(indent, ""), name, f.Type(), f.Interface()))
		}
	}
}

// Options per Command

// NewBuildOptions constructor
func NewBuildOptions(c *cli.Context, e *Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	if pipelineOpts.BuildID == "" {
		pipelineOpts.BuildID = uuid.NewRandom().String()
		pipelineOpts.PipelineID = pipelineOpts.BuildID
	}
	return pipelineOpts, nil
}

// NewDevOptions ctor
func NewDevOptions(c *cli.Context, e *Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewBuildOptions(c, e)
	if err != nil {
		return nil, err
	}
	// dev command implies DirectMount
	pipelineOpts.DirectMount = true
	pipelineOpts.EnableDevSteps = true

	return pipelineOpts, nil
}

// NewCheckConfigOptions constructor
func NewCheckConfigOptions(c *cli.Context, e *Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	return pipelineOpts, nil
}

// NewDeployOptions constructor
func NewDeployOptions(c *cli.Context, e *Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	if pipelineOpts.DeployID == "" {
		pipelineOpts.DeployID = uuid.NewRandom().String()
		pipelineOpts.PipelineID = pipelineOpts.DeployID
	}
	return pipelineOpts, nil
}

// DetectOptions for detect command
type DetectOptions struct {
	*GlobalOptions
}

// NewDetectOptions constructor
func NewDetectOptions(c *cli.Context, e *Environment) (*DetectOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &DetectOptions{globalOpts}, nil
}

// InspectOptions for inspect command
type InspectOptions struct {
	*PipelineOptions
}

// NewInspectOptions constructor
func NewInspectOptions(c *cli.Context, e *Environment) (*InspectOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &InspectOptions{pipelineOpts}, nil
}

// LoginOptions for the login command
type LoginOptions struct {
	*GlobalOptions
}

// NewLoginOptions constructor
func NewLoginOptions(c *cli.Context, e *Environment) (*LoginOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &LoginOptions{globalOpts}, nil
}

// LogoutOptions for the login command
type LogoutOptions struct {
	*GlobalOptions
}

// NewLogoutOptions constructor
func NewLogoutOptions(c *cli.Context, e *Environment) (*LogoutOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &LogoutOptions{globalOpts}, nil
}

// PullOptions for the pull command
type PullOptions struct {
	*GlobalOptions
	*DockerOptions

	Repository string
	Branch     string
	Commit     string
	Status     string
	Result     string
	Output     string
	Load       bool
	Force      bool
}

// NewPullOptions constructor
func NewPullOptions(c *cli.Context, e *Environment) (*PullOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	dockerOpts, err := NewDockerOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	if len(c.Args()) != 1 {
		return nil, errors.New("Pull requires the application ID or the build ID as the only argument")
	}
	repository := c.Args().First()

	output, err := filepath.Abs(c.String("output"))
	if err != nil {
		return nil, err
	}

	return &PullOptions{
		GlobalOptions: globalOpts,
		DockerOptions: dockerOpts,

		Repository: repository,
		Branch:     c.String("branch"),
		Status:     c.String("status"),
		Result:     c.String("result"),
		Output:     output,
		Load:       c.Bool("load"),
		Force:      c.Bool("force"),
	}, nil
}

// VersionOptions contains the options associated with the version
// command.
type VersionOptions struct {
	OutputJSON     bool
	BetaChannel    bool
	CheckForUpdate bool
}

// NewVersionOptions constructor
func NewVersionOptions(c *cli.Context, e *Environment) (*VersionOptions, error) {
	return &VersionOptions{
		OutputJSON:     c.Bool("json"),
		BetaChannel:    c.Bool("beta"),
		CheckForUpdate: !c.Bool("no-update-check"),
	}, nil
}
