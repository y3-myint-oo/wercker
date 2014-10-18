package main

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/termie/go-shutil"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// StepConfig represents a wercker-step.yml
type StepConfig struct {
	Name        string
	Version     string
	Description string
	Keywords    []string
	Properties  map[string]StepConfigProperty
}

// StepConfigProperty is the structure of the values in the "properties"
// section of the config
type StepConfigProperty struct {
	Default  string
	Required bool
	Type     string
}

// ReadStepConfig reads a file, expecting it to be parsed into a StepConfig.
func ReadStepConfig(configPath string) (*StepConfig, error) {
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var m StepConfig
	err = yaml.Unmarshal(file, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Defaults returns the default properties for a step as a map.
func (sc *StepConfig) Defaults() map[string]string {
	m := make(map[string]string)
	if sc == nil || sc.Properties == nil {
		return m
	}
	for k, v := range sc.Properties {
		m[k] = v.Default
	}
	return m
}

// Step is the holder of the Step methods.
type Step struct {
	Env         *Environment
	ID          string
	SafeID      string
	Owner       string
	Name        string
	Version     string
	DisplayName string
	url         string
	data        RawStepData
	build       *Build
	options     *GlobalOptions
	stepConfig  *StepConfig
}

// NormalizeStep attempts to make things like RawSteps into RawSteps.
// Steps unfortunately can come in a couple shapes in the yaml, this
// function attempts to normalize them all to a RawStep
func NormalizeStep(raw interface{}) (*RawStep, error) {
	s := make(RawStep)

	// If it was just a string, make a RawStep with empty data
	stringBase, ok := raw.(string)
	if ok {
		s[stringBase] = make(RawStepData)
		return &s, nil
	}

	// Otherwise it is a map[interface{}]map[interface{}]interface{},
	// and we will manually assert it into shape
	mapBase, ok := raw.(map[interface{}]interface{})
	if ok {
		for key, value := range mapBase {
			mapValue := value.(map[interface{}]interface{})
			data := make(RawStepData)
			for dataKey, dataValue := range mapValue {

				assertedValue, ok := dataValue.(string)
				if !ok {
					maybeBool, ok := dataValue.(bool)
					if ok && maybeBool {
						assertedValue = "true"
					} else {
						assertedValue = "false"
					}
				}

				data[dataKey.(string)] = assertedValue
			}
			s[key.(string)] = data
		}
		return &s, nil
	}
	return nil, fmt.Errorf("Invalid step data. %s", raw)
}

// ToStep converts a RawStep into a Step.
func (s *RawStep) ToStep(build *Build, options *GlobalOptions) (*Step, error) {
	// There should only be one step in the internal map
	var stepID string
	var stepData RawStepData

	// Dereference ourself to get to our underlying data structure
	for id, data := range *s {
		stepID = id
		stepData = data
	}
	return CreateStep(stepID, stepData, build, options)
}

// CreateStep sets up the basic parts of a Step.
// Step names can come in a couple forms (x means currently supported):
//   x setup-go-environment (fetches from api)
//   x wercker/hipchat-notify (fetches from api)
//   x wercker/hipchat-notify "http://someurl/thingee.tar" (downloads tarball)
//   x setup-go-environment "file:///some_path" (uses local path)
func CreateStep(stepID string, data RawStepData, build *Build, options *GlobalOptions) (*Step, error) {
	var identifier string
	var owner string
	var name string
	url := ""

	// TODO(termie): support other versions, "*" returns latest version
	version := "*"

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

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	// Script steps need unique IDs
	if name == "script" {
		stepID = uuid.NewRandom().String()
	}

	// If there is a name in data, make it our displayName and delete it
	displayName, ok := data["name"]
	if !ok {
		displayName = name
	}
	delete(data, "name")

	return &Step{ID: identifier, SafeID: stepSafeID, Owner: owner, Name: name, DisplayName: displayName, Version: version, url: url, data: data, build: build, options: options}, nil
}

// IsScript should probably not be exported.
func (s *Step) IsScript() bool {
	return s.Name == "script"
}

func normalizeCode(code string) string {
	if !strings.HasPrefix(code, "#!") {
		code = strings.Join([]string{"#!/bin/bash -xe", code}, "\n")
	}
	return code
}

// FetchScript turns the raw code in a step into a shell file.
func (s *Step) FetchScript() (string, error) {
	hostStepPath := s.build.HostPath(s.SafeID)
	scriptPath := s.build.HostPath(s.SafeID, "run.sh")
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

// StepAPIInfo is the data structure for the JSON returned by the wercker API.
type StepAPIInfo struct {
	TarballURL  string
	Version     string
	Description string
}

// Fetch grabs the Step content (or calls FetchScript for script steps).
func (s *Step) Fetch() (string, error) {
	// NOTE(termie): polymorphism based on kind, we could probably do something
	//               with interfaces here, but this is okay for now
	if s.IsScript() {
		return s.FetchScript()
	}

	stepPath := filepath.Join(s.options.StepDir, s.SafeID)
	stepExists, err := exists(stepPath)
	if err != nil {
		return "", err
	}

	if !stepExists {
		// If we don't have a url already
		if s.url == "" {
			var stepInfo StepAPIInfo

			// Grab the info about the step from the api
			client := CreateAPIClient(s.options.WerckerEndpoint)
			apiBytes, err := client.Get("steps", s.Owner, s.Name, s.Version)
			if err != nil {
				return "", err
			}

			err = json.Unmarshal(apiBytes, &stepInfo)
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
	cfg, err := ReadStepConfig(s.HostPath("wercker-step.yml"))
	if err != nil && !os.IsNotExist(err) {
		// TODO(termie): Log an error instead of printing
		log.Println("ERROR: Reading wercker-step.yml:", err)
	}
	if err == nil {
		s.stepConfig = cfg
	}
	return hostStepPath, nil
}

// SetupGuest ensures that the guest is ready to run a Step.
func (s *Step) SetupGuest(sess *Session) error {
	// TODO(termie): can this even fail? i.e. exit code != 0
	_, _, err := sess.SendChecked(fmt.Sprintf(`mkdir -p "%s"`, s.ReportPath("artifacts")))
	_, _, err = sess.SendChecked("set +e")
	_, _, err = sess.SendChecked(fmt.Sprintf(`cp -r "%s" "%s"`, s.MntPath(), s.GuestPath()))
	_, _, err = sess.SendChecked(fmt.Sprintf(`cd "%s"`, s.build.SourcePath()))
	return err
}

// Execute actually sends the commands for the step.
func (s *Step) Execute(sess *Session) (int, error) {
	err := s.SetupGuest(sess)
	if err != nil {
		return 1, err
	}
	_, _, err = sess.SendChecked(s.Env.Export()...)
	if err != nil {
		return 1, err
	}

	if yes, _ := exists(s.HostPath("init.sh")); yes {
		exit, _, err := sess.SendChecked(fmt.Sprintf(`source "%s"`, s.GuestPath("init.sh")))
		if exit != 0 {
			return exit, errors.New("Ack!")
		}
		if err != nil {
			return 1, err
		}
	}

	if yes, _ := exists(s.HostPath("run.sh")); yes {
		exit, _, err := sess.SendChecked(fmt.Sprintf(`source "%s"`, s.GuestPath("run.sh")))
		return exit, err
	}

	return 0, nil
}

// CollectArtifacts copies the artifacts associated with the Step.
func (s *Step) CollectArtifacts(sess *Session) ([]*Artifact, error) {
	artificer := CreateArtificer(s.options)

	// Ensure we have the host directory

	artifact := &Artifact{
		ContainerID: sess.ContainerID,
		GuestPath:   s.ReportPath("artifacts"),
		HostPath:    s.build.HostPath("artifacts", s.SafeID, "artifacts.tar"),
		ProjectID:   s.options.ProjectID,
		BuildID:     s.options.BuildID,
		BuildStepID: s.SafeID,
	}

	fullArtifact, err := artificer.Collect(artifact)
	if err != nil {
		if err == ErrEmptyTarball {
			return []*Artifact{}, nil
		}
		return nil, err
	}

	return []*Artifact{fullArtifact}, nil
}

// InitEnv sets up the internal environment for the Step.
func (s *Step) InitEnv() {
	s.Env = &Environment{}
	a := [][]string{
		[]string{"WERCKER_STEP_ROOT", s.GuestPath()},
		[]string{"WERCKER_STEP_ID", s.SafeID},
		[]string{"WERCKER_STEP_OWNER", s.Owner},
		[]string{"WERCKER_STEP_NAME", s.Name},
		[]string{"WERCKER_REPORT_NUMBERS_FILE", s.ReportPath("numbers.ini")},
		[]string{"WERCKER_REPORT_MESSAGE_FILE", s.ReportPath("message.txt")},
		[]string{"WERCKER_REPORT_ARTIFACTS_DIR", s.ReportPath("artifacts")},
	}
	s.Env.Update(a)

	defaults := s.stepConfig.Defaults()

	for k, defaultValue := range defaults {
		value, ok := s.data[k]
		key := fmt.Sprintf("WERCKER_%s_%s", s.Name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		if !ok {
			s.Env.Add(key, defaultValue)
		} else {
			s.Env.Add(key, value)
		}
	}

	for k, value := range s.data {
		if k == "code" || k == "name" {
			continue
		}
		key := fmt.Sprintf("WERCKER_%s_%s", s.Name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		s.Env.Add(key, value)
	}
}

// HostPath returns a path relative to the Step on the host.
func (s *Step) HostPath(p ...string) string {
	newArgs := append([]string{s.SafeID}, p...)
	return s.build.HostPath(newArgs...)
}

// GuestPath returns a path relative to the Step on the guest.
func (s *Step) GuestPath(p ...string) string {
	newArgs := append([]string{s.SafeID}, p...)
	return s.build.GuestPath(newArgs...)
}

// MntPath returns a path relative to the read-only mount of the Step on
// the guest.
func (s *Step) MntPath(p ...string) string {
	newArgs := append([]string{s.SafeID}, p...)
	return s.build.MntPath(newArgs...)
}

// ReportPath returns a path to the reports for the step on the guest.
func (s *Step) ReportPath(p ...string) string {
	newArgs := append([]string{s.SafeID}, p...)
	return s.build.ReportPath(newArgs...)
}
