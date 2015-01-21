package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Flags for setting these options from the CLI
var (
	// These flags control where we store local files
	localPathFlags = []cli.Flag{
		cli.StringFlag{Name: "project-dir", Value: "./_projects", Usage: "path where downloaded projects live"},
		cli.StringFlag{Name: "step-dir", Value: "./_steps", Usage: "path where downloaded steps live"},
		cli.StringFlag{Name: "build-dir", Value: "./_builds", Usage: "path where created builds live"},
	}

	// These flags tell us where to go for operations
	endpointFlags = []cli.Flag{
		cli.StringFlag{Name: "docker-host", Value: "tcp://127.0.0.1:2375", Usage: "docker api host", EnvVar: "DOCKER_HOST"},
		cli.StringFlag{Name: "wercker-endpoint", Value: "https://app.wercker.com/api/v2", Usage: "wercker api endpoint"},
		cli.StringFlag{Name: "base-url", Value: "https://app.wercker.com/", Usage: "base url for the web app"},
		cli.StringFlag{Name: "registry", Value: "127.0.0.1:3000", Usage: "registry endpoint to push images to"},
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
		cli.StringFlag{Name: "application-id", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_ID"},
		cli.StringFlag{Name: "application-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_NAME"},
		cli.StringFlag{Name: "application-owner-name", Value: "", Usage: "application id", EnvVar: "WERCKER_APPLICATION_OWNER_NAME"},
		cli.StringFlag{Name: "application-started-by-name", Value: "", Usage: "application started by", EnvVar: "WERCKER_APPLICATION_STARTED_BY_NAME"},
	}

	// These flags affect our registry interactions
	registryFlags = []cli.Flag{
		cli.BoolFlag{Name: "push", Usage: "push the build result to registry"},
		cli.BoolFlag{Name: "commit", Usage: "commit the build result locally"},
		cli.StringFlag{Name: "tag", Value: "", Usage: "tag for this build", EnvVar: "WERCKER_GIT_BRANCH"},
		cli.StringFlag{Name: "message", Value: "", Usage: "message for this build"},
	}

	// These flags affect our artifact interactions
	artifactFlags = []cli.Flag{
		cli.BoolFlag{Name: "no-artifacts", Usage: "don't upload artifacts"},
	}

	// These flags affect our local execution environment
	devFlags = []cli.Flag{
		cli.StringFlag{Name: "environment", Value: "ENVIRONMENT", Usage: "specify additional environment variables in a file"},
		cli.BoolFlag{Name: "debug", Usage: "print additional debug information"},
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
		cli.BoolFlag{Name: "keen-metrics", Usage: "report metrics to keen.io"},
		cli.StringFlag{Name: "keen-project-write-key", Value: "", Usage: "keen write key"},
		cli.StringFlag{Name: "keen-project-id", Value: "", Usage: "keen project id"},
	}

	// Wercker Reporter settings
	reporterFlags = []cli.Flag{
		cli.BoolFlag{Name: "report", Usage: "Report logs back to wercker (requires build-id, wercker-host, wercker-token)"},
		cli.StringFlag{Name: "wercker-host", Usage: "Wercker host to use for wercker reporter"},
		cli.StringFlag{Name: "wercker-token", Usage: "Wercker token to use for wercker reporter"},
	}

	// These options might be overwritten by the wercker.yml
	configFlags = []cli.Flag{
		cli.StringFlag{Name: "source-dir", Value: "", Usage: "source path relative to checkout root"},
		cli.IntFlag{Name: "no-response-timeout", Value: 5, Usage: "timeout if no script output is received in this many minutes"},
		cli.IntFlag{Name: "command-timeout", Value: 10, Usage: "timeout if command does not complete in this many minutes"},
	}

	AllFlags = [][]cli.Flag{
		localPathFlags,
		endpointFlags,
		internalPathFlags,
		werckerFlags,
		registryFlags,
		artifactFlags,
		devFlags,
		awsFlags,
		keenFlags,
		reporterFlags,
		configFlags,
	}
)

func allFlags() []cli.Flag {
	all := []cli.Flag{}

	for _, x := range AllFlags {
		all = append(all, x...)
	}
	return all
}

// GlobalOptions is a shared data structure for global config.
type GlobalOptions struct {
	Env *Environment

	ProjectDir string
	StepDir    string
	BuildDir   string

	// Application ID for this operation
	ApplicationID string

	// Build ID for this operation
	BuildID string

	// Deploy ID for this operation
	DeployID string

	// Pipeline ID is either BuildID or DeployID dependent on which we got
	PipelineID string

	// Application name for this operation
	ApplicationName string

	// Application owner name for this operation
	ApplicationOwnerName string

	// Application starter name for this operation
	ApplicationStartedByName string

	// Base url template to see the results of this build
	BaseURL string

	DockerHost string

	// Base endpoint for wercker api
	WerckerEndpoint string

	// The read-write directory on the guest where all the work happens
	GuestRoot string

	// The read-only directory on the guest where volumes are mounted
	MntRoot string

	// The directory on the guest where reports will be written
	ReportRoot string

	// Source path relative to checkout root
	SourceDir string

	// Timeout if no response is received from a script in this many minutes
	NoResponseTimeout int

	// Timeout if the command doesn't complete in this many minutes
	CommandTimeout int

	// A path where the project lives
	ProjectPath string

	// For fetching code
	ProjectURL string

	// AWS Bits
	AWSSecretAccessKey string
	AWSAccessKeyID     string
	AWSRegion          string
	S3Bucket           string

	// Keen Bits
	ShouldKeenMetrics   bool
	KeenProjectWriteKey string
	KeenProjectID       string

	Registry     string
	ShouldPush   bool
	ShouldCommit bool
	Tag          string
	Message      string

	ShouldArtifacts bool

	ShouldReport bool
	WerckerHost  string
	WerckerToken string

	// Show stack traces on exit?
	Debug bool
}

// Some logic to guess the application name
func guessApplicationName(c *cli.Context, env *Environment) (string, error) {
	// If we explicitly were given an application name, use that
	applicationName, ok := env.Map["WERCKER_APPLICATION_NAME"]
	if ok {
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

func guessTag(c *cli.Context, env *Environment) string {
	tag := c.GlobalString("tag")
	return tag
}

func guessMessage(c *cli.Context, env *Environment) string {
	message := c.GlobalString("message")
	return message
}

func guessApplicationOwnerName(c *cli.Context, env *Environment) string {
	name := c.GlobalString("application-owner-name")
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

func guessBuildID(c *cli.Context, env *Environment) string {
	id := c.GlobalString("build-id")
	if id == "" {
		id, ok := env.Map["WERCKER_BUILD_ID"]
		if !ok {
			return ""
		}
		return id
	}
	return id
}

func guessDeployID(c *cli.Context, env *Environment) string {
	id := c.GlobalString("deploy-id")
	if id == "" {
		id, ok := env.Map["WERCKER_DEPLOY_ID"]
		if !ok {
			return ""
		}
		return id
	}
	return id
}

// dumpOptions prints out a sorted list of options
func dumpOptions(options *GlobalOptions) {
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

	for _, name := range names {
		r := reflect.ValueOf(options)
		f := reflect.Indirect(r).FieldByName(name)
		log.Debugln(fmt.Sprintf("  %s %s = %v", name, f.Type(), f.Interface()))
	}
}

// NewGlobalOptions builds up GlobalOptions from the cli and environment.
func NewGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
	env := NewEnvironment(e)

	buildDir, _ := filepath.Abs(c.GlobalString("build-dir"))
	projectDir, _ := filepath.Abs(c.GlobalString("project-dir"))
	stepDir, _ := filepath.Abs(c.GlobalString("step-dir"))
	buildID := guessBuildID(c, env)
	deployID := guessDeployID(c, env)

	pipelineID := ""
	if deployID != "" {
		pipelineID = deployID
	} else {
		pipelineID = buildID
	}

	applicationName, err := guessApplicationName(c, env)
	if err != nil {
		return nil, err
	}

	applicationOwnerName := guessApplicationOwnerName(c, env)

	applicationID, ok := env.Map["WERCKER_APPLICATION_ID"]
	if !ok {
		applicationID = applicationName
		log.Warnln("No ApplicationID specified, using", applicationID)
	}

	projectURL := ""
	target := c.Args().First()
	projectPath := target
	if projectPath == "" {
		projectPath = "."
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		projectURL = target
		projectPath = ""
	}

	projectID := c.GlobalString("project-id")
	if projectID == "" {
		projectID = strings.Replace(c.Args().First(), "/", "_", -1)
	}

	tag := guessTag(c, env)
	message := guessMessage(c, env)

	// AWS bits
	awsSecretAccessKey := c.GlobalString("aws-secret-key")
	awsAccessKeyID := c.GlobalString("aws-access-key")
	if awsSecretAccessKey == "" {
		if val, ok := env.Map["AWS_SECRET_ACCESS_KEY"]; ok {
			awsSecretAccessKey = val
		}
	}
	if awsAccessKeyID == "" {
		if val, ok := env.Map["AWS_ACCESS_KEY_ID"]; ok {
			awsAccessKeyID = val
		}
	}

	keenMetrics := c.GlobalBool("keen-metrics")
	keenProjectWriteKey := c.GlobalString("keen-project-write-key")
	keenProjectID := c.GlobalString("keen-project-id")

	if keenMetrics {
		if keenProjectWriteKey == "" {
			return nil, errors.New("keen-project-write-key is required")
		}

		if keenProjectID == "" {
			return nil, errors.New("keen-project-id is required")
		}
	}

	report := c.GlobalBool("report")
	werckerHost := c.GlobalString("wercker-host")
	werckerToken := c.GlobalString("wercker-token")

	if report {
		if werckerHost == "" {
			return nil, errors.New("wercker-host is required")
		}

		if werckerToken == "" {
			return nil, errors.New("wercker-token is required")
		}
	}

	return &GlobalOptions{
		Env:                      env,
		BuildDir:                 buildDir,
		BuildID:                  buildID,
		DeployID:                 deployID,
		PipelineID:               pipelineID,
		ApplicationID:            applicationID,
		ApplicationName:          applicationName,
		ApplicationOwnerName:     applicationOwnerName,
		ApplicationStartedByName: c.GlobalString("application-started-by-name"),
		BaseURL:                  c.GlobalString("base-url"),
		CommandTimeout:           c.GlobalInt("command-timeout"),
		DockerHost:               c.GlobalString("docker-host"),
		WerckerEndpoint:          c.GlobalString("wercker-endpoint"),
		NoResponseTimeout:        c.GlobalInt("no-response-timeout"),
		ProjectDir:               projectDir,
		SourceDir:                c.GlobalString("source-dir"),
		StepDir:                  stepDir,
		GuestRoot:                c.GlobalString("guest-root"),
		MntRoot:                  c.GlobalString("mnt-root"),
		ReportRoot:               c.GlobalString("report-root"),
		ProjectPath:              projectPath,
		ProjectURL:               projectURL,
		AWSSecretAccessKey:       awsSecretAccessKey,
		AWSAccessKeyID:           awsAccessKeyID,
		S3Bucket:                 c.GlobalString("s3-bucket"),
		AWSRegion:                c.GlobalString("aws-region"),
		Registry:                 c.GlobalString("registry"),
		ShouldPush:               c.GlobalBool("push"),
		ShouldCommit:             c.GlobalBool("commit"),
		ShouldKeenMetrics:        keenMetrics,
		ShouldArtifacts:          !c.GlobalBool("no-artifacts"),
		KeenProjectWriteKey:      keenProjectWriteKey,
		KeenProjectID:            keenProjectID,
		Tag:                      tag,
		Message:                  message,
		ShouldReport:             report,
		WerckerHost:              werckerHost,
		WerckerToken:             werckerToken,
		Debug:                    c.GlobalBool("debug"),
	}, nil
}

// SourcePath returns the path to the source dir
func (o *GlobalOptions) SourcePath() string {
	return o.GuestPath("source", o.SourceDir)
}

// HostPath returns a path relative to the build root on the host.
func (o *GlobalOptions) HostPath(s ...string) string {
	return path.Join(o.BuildDir, o.PipelineID, path.Join(s...))
}

// GuestPath returns a path relative to the build root on the guest.
func (o *GlobalOptions) GuestPath(s ...string) string {
	return path.Join(o.GuestRoot, path.Join(s...))
}

// MntPath returns a path relative to the read-only mount root on the guest.
func (o *GlobalOptions) MntPath(s ...string) string {
	return path.Join(o.MntRoot, path.Join(s...))
}

// ReportPath returns a path relative to the report root on the guest.
func (o *GlobalOptions) ReportPath(s ...string) string {
	return path.Join(o.ReportRoot, path.Join(s...))
}

// VersionOptions contains the options associated with the version
// command.
type VersionOptions struct {
	OutputJSON bool
}

func createVersionOptions(c *cli.Context) (*VersionOptions, error) {
	return &VersionOptions{
		OutputJSON: c.Bool("json"),
	}, nil
}
