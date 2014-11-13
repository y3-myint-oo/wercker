package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
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
	AWSBucket          string

	Registry       string
	PushToRegistry bool
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

// CreateGlobalOptions builds up GlobalOptions from the cli and environment.
func CreateGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
	env := CreateEnvironment(e)

	buildDir, _ := filepath.Abs(c.GlobalString("buildDir"))
	projectDir, _ := filepath.Abs(c.GlobalString("projectDir"))
	stepDir, _ := filepath.Abs(c.GlobalString("stepDir"))
	buildID := c.GlobalString("buildID")
	if buildID == "" {
		buildID = uuid.NewRandom().String()
	}

	applicationName, err := guessApplicationName(c, env)
	if err != nil {
		return nil, err
	}

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

	projectID := c.GlobalString("projectID")
	if projectID == "" {
		projectID = strings.Replace(c.Args().First(), "/", "_", -1)
	}

	// AWS bits
	awsSecretAccessKey := c.GlobalString("awsSecretAccessKey")
	awsAccessKeyID := c.GlobalString("awsAccessKeyID")
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

	return &GlobalOptions{
		Env:                  env,
		BuildDir:             buildDir,
		BuildID:              buildID,
		ApplicationID:        applicationID,
		ApplicationName:      applicationName,
		ApplicationOwnerName: c.GlobalString("applicationOwnerName"),
		BaseURL:              c.GlobalString("baseURL"),
		CommandTimeout:       c.GlobalInt("commandTimeout"),
		DockerHost:           c.GlobalString("dockerHost"),
		WerckerEndpoint:      c.GlobalString("werckerEndpoint"),
		NoResponseTimeout:    c.GlobalInt("noResponseTimeout"),
		ProjectDir:           projectDir,
		SourceDir:            c.GlobalString("sourceDir"),
		StepDir:              stepDir,
		GuestRoot:            c.GlobalString("guestRoot"),
		MntRoot:              c.GlobalString("mntRoot"),
		ReportRoot:           c.GlobalString("reportRoot"),
		ProjectPath:          projectPath,
		ProjectURL:           projectURL,
		AWSSecretAccessKey:   awsSecretAccessKey,
		AWSAccessKeyID:       awsAccessKeyID,
		AWSBucket:            c.GlobalString("awsBucket"),
		AWSRegion:            c.GlobalString("awsRegion"),
		Registry:             c.GlobalString("registry"),
		PushToRegistry:       c.GlobalBool("pushToRegistry"),
	}, nil
}
