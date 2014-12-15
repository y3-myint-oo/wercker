package main

import (
	"code.google.com/p/go-uuid/uuid"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
)

// Environment represents a shell environment and is implemented as something
// like an OrderedMap
type Environment struct {
	Map   map[string]string
	Order []string
}

// CreateEnvironment fills up an Environment from a []string
// Usually called like: env := CreateEnvironment(os.Environ())
func CreateEnvironment(env []string) *Environment {
	e := Environment{}
	for _, keyvalue := range env {
		pair := strings.SplitN(keyvalue, "=", 2)
		e.Add(pair[0], pair[1])
	}

	return &e
}

// Update adds new elements to the Environment data structure.
func (e *Environment) Update(a [][]string) {
	for _, keyvalue := range a {
		e.Add(keyvalue[0], keyvalue[1])
	}
}

// Add an idividual record.
func (e *Environment) Add(key, value string) {
	if e.Map == nil {
		e.Map = make(map[string]string)
	}
	if _, ok := e.Map[key]; !ok {
		e.Order = append(e.Order, key)
	}
	e.Map[key] = value
}

// Export the environment as shell commands for use with Session.Send*
func (e *Environment) Export() []string {
	s := []string{}
	for _, key := range e.Order {
		s = append(s, fmt.Sprintf(`export %s="%s"`, key, e.Map[key]))
	}
	return s
}

// Ordered returns a [][]string of the items in the env.
func (e *Environment) Ordered() [][]string {
	a := [][]string{}
	for _, k := range e.Order {
		a = append(a, []string{k, e.Map[k]})
	}
	return a
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

	ShouldReport bool
	WerckerHost  string
	WerckerToken string
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
			name = u.Name
		}
	}
	if name == "" {
		name = "wercker"
	}
	return name
}

// CreateGlobalOptions builds up GlobalOptions from the cli and environment.
func CreateGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
	env := CreateEnvironment(e)

	buildDir, _ := filepath.Abs(c.GlobalString("build-dir"))
	projectDir, _ := filepath.Abs(c.GlobalString("project-dir"))
	stepDir, _ := filepath.Abs(c.GlobalString("step-dir"))
	buildID := c.GlobalString("build-id")
	if buildID == "" {
		buildID = uuid.NewRandom().String()
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
		if buildID == "" {
			return nil, errors.New("build-id is required")
		}

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
		KeenProjectWriteKey:      keenProjectWriteKey,
		KeenProjectID:            keenProjectID,
		Tag:                      tag,
		Message:                  message,
		ShouldReport:             report,
		WerckerHost:              werckerHost,
		WerckerToken:             werckerToken,
	}, nil
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
