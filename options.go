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

// LogOptions prints out a sorted list of options
func LogOptions(options *GlobalOptions) {
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
		log.Debugln(fmt.Sprintf("%s %s = %v", name, f.Type(), f.Interface()))
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
