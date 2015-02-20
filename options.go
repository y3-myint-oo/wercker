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
	"sort"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/codegangsta/cli"
)

// Flags for setting these options from the CLI
var (
	// These flags tell us where to go for operations
	endpointFlags = []cli.Flag{
		// deprecated
		cli.StringFlag{Name: "wercker-endpoint", Value: "", Usage: "deprecated", Hidden: true},
		cli.StringFlag{Name: "base-url", Value: "https://app.wercker.com", Usage: "base url for the wercker app", Hidden: true},
	}

	// These flags let us auth to wercker services
	authFlags = []cli.Flag{
		cli.StringFlag{Name: "auth-token", Usage: "authentication token to use"},
		cli.StringFlag{Name: "auth-token-store", Value: "~/.wercker/token", Usage: "where to store the token after a login", Hidden: true},
	}

	dockerFlags = []cli.Flag{
		cli.StringFlag{Name: "docker-host", Value: "tcp://127.0.0.1:2375", Usage: "docker api host", EnvVar: "DOCKER_HOST"},
		cli.StringFlag{Name: "docker-tls-verify", Value: "0", Usage: "docker api tls verify", EnvVar: "DOCKER_TLS_VERIFY"},
		cli.StringFlag{Name: "docker-cert-path", Value: "", Usage: "docker api cert path", EnvVar: "DOCKER_CERT_PATH"},
	}

	// These flags control where we store local files
	localPathFlags = []cli.Flag{
		cli.StringFlag{Name: "project-dir", Value: "./_projects", Usage: "path where downloaded projects live"},
		cli.StringFlag{Name: "step-dir", Value: "./_steps", Usage: "path where downloaded steps live"},
		cli.StringFlag{Name: "build-dir", Value: "./_builds", Usage: "path where created builds live"},
		cli.StringFlag{Name: "container-dir", Value: "./_containers", Usage: "path where exported containers live"},
	}

	// These flags control paths on the guest and probably shouldn't change
	internalPathFlags = []cli.Flag{
		cli.StringFlag{Name: "mnt-root", Value: "/mnt", Usage: "directory on the guest where volumes are mounted"},
		cli.StringFlag{Name: "guest-root", Value: "/pipeline", Usage: "directory on the guest where work is done"},
		cli.StringFlag{Name: "report-root", Value: "/report", Usage: "directory on the guest where reports will be written"},
	}

	// These flags are usually pulled from the env
	werckerFlags = []cli.Flag{
		cli.StringFlag{Name: "build-id", Value: "", Usage: "build id", EnvVar: "WERCKER_BUILD_ID"},
		cli.StringFlag{Name: "deploy-id", Value: "", Usage: "deploy id", EnvVar: "WERCKER_DEPLOY_ID"},
		cli.StringFlag{Name: "deploy-target", Value: "", Usage: "deploy target name", EnvVar: "WERCKER_DEPLOYTARGET_NAME"},
		cli.StringFlag{Name: "application-id", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_ID"},
		cli.StringFlag{Name: "application-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_NAME"},
		cli.StringFlag{Name: "application-owner-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_OWNER_NAME"},
		cli.StringFlag{Name: "application-started-by-name", Value: "", Usage: "application started by", EnvVar: "WERCKER_APPLICATION_STARTED_BY_NAME"},
	}

	gitFlags = []cli.Flag{
		cli.StringFlag{Name: "git-domain", Value: "", Usage: "git domain", EnvVar: "WERCKER_GIT_DOMAIN"},
		cli.StringFlag{Name: "git-owner", Value: "", Usage: "git owner", EnvVar: "WERCKER_GIT_OWNER"},
		cli.StringFlag{Name: "git-repository", Value: "", Usage: "git repository", EnvVar: "WERCKER_GIT_REPOSITORY"},
		cli.StringFlag{Name: "git-branch", Value: "", Usage: "git branch", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "git-commit", Value: "", Usage: "git commit", EnvVar: "WERCKER_GIT_COMMIT"},
	}

	// These flags affect our registry interactions
	registryFlags = []cli.Flag{
		cli.BoolFlag{Name: "commit", Usage: "commit the build result locally"},
		cli.StringFlag{Name: "tag", Value: "", Usage: "tag for this build", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "message", Value: "", Usage: "message for this build"},
	}

	// These flags affect our artifact interactions
	artifactFlags = []cli.Flag{
		cli.BoolFlag{Name: "artifacts", Usage: "store artifacts"},
		cli.BoolFlag{Name: "no-remove", Usage: "don't remove the containers"},
		cli.BoolFlag{Name: "store-local", Usage: "store artifacts and containers locally"},
		cli.BoolFlag{Name: "store-s3", Usage: "store artifacts and containers on s3"},
	}

	// These flags affect our local execution environment
	devFlags = []cli.Flag{
		cli.StringFlag{Name: "environment", Value: "ENVIRONMENT", Usage: "specify additional environment variables in a file"},
		cli.BoolFlag{Name: "debug", Usage: "print additional debug information"},
	}

	// These flags are advanced dev settings
	internalDevFlags = []cli.Flag{
		cli.BoolFlag{Name: "direct-mount", Usage: "mount our binds read-write to the pipeline path"},
	}

	// AWS bits
	awsFlags = []cli.Flag{
		cli.StringFlag{Name: "aws-secret-key", Value: "", Usage: "secret access key"},
		cli.StringFlag{Name: "aws-access-key", Value: "", Usage: "access key id"},
		cli.StringFlag{Name: "s3-bucket", Value: "wercker-development", Usage: "bucket for artifacts"},
		cli.StringFlag{Name: "aws-region", Value: "us-east-1", Usage: "region"},
	}

	// keen.io bits
	keenFlags = []cli.Flag{
		cli.BoolFlag{Name: "keen-metrics", Usage: "report metrics to keen.io", Hidden: true},
		cli.StringFlag{Name: "keen-project-write-key", Value: "", Usage: "keen write key", Hidden: true},
		cli.StringFlag{Name: "keen-project-id", Value: "", Usage: "keen project id", Hidden: true},
	}

	// Wercker Reporter settings
	reporterFlags = []cli.Flag{
		cli.BoolFlag{Name: "report", Usage: "Report logs back to wercker (requires build-id, wercker-host, wercker-token)", Hidden: true},
		cli.StringFlag{Name: "wercker-host", Usage: "Wercker host to use for wercker reporter", Hidden: true},
		cli.StringFlag{Name: "wercker-token", Usage: "Wercker token to use for wercker reporter", Hidden: true},
	}

	// These options might be overwritten by the wercker.yml
	configFlags = []cli.Flag{
		cli.StringFlag{Name: "source-dir", Value: "", Usage: "source path relative to checkout root"},
		cli.Float64Flag{Name: "no-response-timeout", Value: 5, Usage: "timeout if no script output is received in this many minutes"},
		cli.Float64Flag{Name: "command-timeout", Value: 10, Usage: "timeout if command does not complete in this many minutes"},
		cli.StringFlag{Name: "wercker-yml", Value: "", Usage: "specify a specific yaml file"},
	}

	pullFlags = [][]cli.Flag{
		[]cli.Flag{
			cli.StringFlag{Name: "build-id", Value: "", Usage: "build id to retrieve from wercker"},
		},
	}

	GlobalFlags = [][]cli.Flag{
		devFlags,
		endpointFlags,
		authFlags,
	}

	DockerFlags = [][]cli.Flag{
		dockerFlags,
	}

	PipelineFlags = [][]cli.Flag{
		localPathFlags,
		werckerFlags,
		dockerFlags,
		internalDevFlags,
		gitFlags,
		registryFlags,
		artifactFlags,
		awsFlags,
		configFlags,
	}

	WerckerInternalFlags = [][]cli.Flag{
		internalPathFlags,
		keenFlags,
		reporterFlags,
	}
)

func flagsFor(flagSets ...[][]cli.Flag) []cli.Flag {
	all := []cli.Flag{}
	for _, flagSet := range flagSets {
		for _, x := range flagSet {
			all = append(all, x...)
		}
	}
	return all
}

// GlobalOptions applicable to everything
type GlobalOptions struct {
	Debug   bool
	BaseURL string

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
	debug := c.GlobalBool("debug")

	baseURL := c.GlobalString("base-url")

	authTokenStore := expandHomePath(c.GlobalString("auth-token-store"), e.Get("HOME"))
	authToken := guessAuthToken(c, e, authTokenStore)

	return &GlobalOptions{
		Debug:   debug,
		BaseURL: baseURL,

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
	}, nil
}

// DockerOptions for our docker client
type DockerOptions struct {
	*GlobalOptions
	DockerHost      string
	DockerTLSVerify string
	DockerCertPath  string
}

// NewDockerOptions constructor
func NewDockerOptions(c *cli.Context, e *Environment, globalOpts *GlobalOptions) (*DockerOptions, error) {
	dockerHost := c.String("docker-host")
	dockerTLSVerify := c.String("docker-tls-verify")
	dockerCertPath := c.String("docker-cert-path")

	return &DockerOptions{
		GlobalOptions:   globalOpts,
		DockerHost:      dockerHost,
		DockerTLSVerify: dockerTLSVerify,
		DockerCertPath:  dockerCertPath,
	}, nil
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
	Env *Environment

	BuildID      string
	DeployID     string
	PipelineID   string
	DeployTarget string

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
	ProjectDir   string
	StepDir      string
	ContainerDir string

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

	DirectMount bool
	WerckerYml  string
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
	projectDir, _ := filepath.Abs(c.String("project-dir"))
	stepDir, _ := filepath.Abs(c.String("step-dir"))
	containerDir, _ := filepath.Abs(c.String("container-dir"))

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

	directMount := c.Bool("direct-mount")
	werckerYml := c.String("wercker-yml")

	return &PipelineOptions{
		GlobalOptions:   globalOpts,
		AWSOptions:      awsOpts,
		DockerOptions:   dockerOpts,
		GitOptions:      gitOpts,
		KeenOptions:     keenOpts,
		ReporterOptions: reporterOpts,

		Env: e,

		BuildID:      buildID,
		DeployID:     deployID,
		PipelineID:   pipelineID,
		DeployTarget: deployTarget,

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
		ProjectDir:   projectDir,
		StepDir:      stepDir,
		ContainerDir: containerDir,

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

		DirectMount: directMount,
		WerckerYml:  werckerYml,
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
		if fieldName != "Env" {
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

// PullOptions for the pull command
type PullOptions struct {
	*GlobalOptions
	*DockerOptions

	BuildID string
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
		return nil, errors.New("Pull requires buildId as the only argument: wercker [global options] pull [command options] <build id>")
	}
	buildID := c.Args().First()

	return &PullOptions{
		GlobalOptions: globalOpts,
		DockerOptions: dockerOpts,
		BuildID:       buildID,
	}, nil
}

// VersionOptions contains the options associated with the version
// command.
type VersionOptions struct {
	OutputJSON      bool
	UnstableChannel bool
}

// NewVersionOptions constructor
func NewVersionOptions(c *cli.Context, e *Environment) (*VersionOptions, error) {
	return &VersionOptions{
		OutputJSON:      c.Bool("json"),
		UnstableChannel: c.Bool("unstable"),
	}, nil
}
