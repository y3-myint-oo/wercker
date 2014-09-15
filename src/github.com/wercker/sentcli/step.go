package main

import (
  "encoding/json"
  "errors"
  "fmt"
  "io/ioutil"
  "log"
  "os"
  "net/http"
  "path/filepath"
  "strings"
  "code.google.com/p/go-uuid/uuid"
  "github.com/termie/go-shutil"
  "gopkg.in/yaml.v1"
)


// StepConfig represents a wercker-step.yml
type StepConfig struct {
  Name string
  Version string
  Description string
  Keywords []string
  Properties map[string]StepConfigProperty
}


// This is structure of the values in the "properties" section of the config
type StepConfigProperty struct {
  Default string
  Required bool
  Type string
}


// Reads a file, expecting it to be parsed into a StepConfig.
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


// Return the default properties as a map
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


type Step struct {
  Env *Environment
  Id string
  SafeId string
  Owner string
  Name string
  Version string
  DisplayName string
  data RawStepData
  build *Build
  options *GlobalOptions
  stepConfig *StepConfig
}


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
        data[dataKey.(string)] = dataValue.(string)
      }
      s[key.(string)] = data
    }
    return &s, nil
  }
  return nil, errors.New(fmt.Sprintf("Invalid step data. %s", raw))
}


// Convert a RawStep into a Step
func (s *RawStep) ToStep(build *Build, options *GlobalOptions) (*Step, error) {
  // There should only be one step in the internal map
  var stepId string
  var stepData RawStepData

  // Dereference ourself to get to our underlying data structure
  for id, data := range *s {
    stepId = id
    stepData = data
  }
  return CreateStep(stepId, stepData, build, options)
}


func CreateStep(stepId string, data RawStepData, build *Build, options *GlobalOptions) (*Step, error) {
  var owner string
  var name string

  // TODO(termie): support other versions, "*" returns latest version
  version := "*"

  // Steps without an owner are owned by wercker
  if strings.Contains(stepId, "/") {
    _, err := fmt.Sscanf(stepId, "%s/%s", &owner, &name)
    if err != nil {
      return nil, err
    }
  } else {
    owner = "wercker"
    name = stepId
  }

  // Add a random number to the name to prevent collisions on disk
  stepSafeId := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

  // Script steps need unique IDs
  if name == "script" {
    stepId = uuid.NewRandom().String()
  }

  // If there is a name in data, make it our displayName and delete it
  displayName, ok := data["name"]
  if !ok {
    displayName = name
  }
  delete(data, "name")


  return &Step{Id:stepId, SafeId:stepSafeId, Owner:owner, Name:name, DisplayName:displayName, Version:version, data:data, build:build, options:options}, nil
}


func (s *Step) IsScript() (bool) {
  return s.Name == "script"
}


func normalizeCode(code string) string {
  if !strings.HasPrefix(code, "#!") {
     code = strings.Join([]string{"#!/bin/bash -xe", code}, "\n")
  }
  return code
}


func (s *Step) FetchScript() (string, error) {
  hostStepPath := s.build.HostPath(s.Id)
  scriptPath := s.build.HostPath(s.Id, "run.sh")
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


type StepApiInfo struct {
  TarballUrl string
  Version string
  Description string
}


func (s *Step) Fetch() (string, error) {
  // NOTE(termie): polymorphism based on kind, we could probably do something
  //               with interfaces here, but this is okay for now
  if s.IsScript() {
    return s.FetchScript()
  }

  stepPath := filepath.Join(s.options.StepDir, s.Id)
  stepExists, err := exists(stepPath)
  if err != nil {
    return "", err
  }
  if !stepExists {
    var stepInfo StepApiInfo

    // Grab the info about the step from the api
    client := CreateApiClient(s.options.WerckerEndpoint)
    apiBytes, err := client.Get("steps", s.Owner, s.Id, s.Version)
    if err != nil {
      return "", err
    }

    err = json.Unmarshal(apiBytes, &stepInfo)
    if err != nil {
      return "", err
    }

    // Grab the tarball and untar it
    resp, err := http.Get(stepInfo.TarballUrl)
    if err != nil {
      return "", err
    }
    if resp.StatusCode != 200 {
      return "", errors.New("Bad status code fetching tarball")
    }

    // Assuming we have a gzip'd tarball at this point
    err = untargzip(stepPath, resp.Body)
    if err != nil {
      return "", err
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


func (s *Step) SetupGuest(sess *Session) error {
  // TODO(termie): can this even fail? i.e. exit code != 0
 _, _, err := sess.SendChecked(fmt.Sprintf(`mkdir -p "%s"`, s.ReportPath("artifacts")))
 _, _, err = sess.SendChecked("set +e")
 _, _, err = sess.SendChecked(fmt.Sprintf(`cp -r "%s" "%s"`, s.MntPath(), s.GuestPath()))
 _, _, err = sess.SendChecked(fmt.Sprintf(`cd "%s"`, s.build.SourcePath()))
 return err
}


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


func (s *Step) InitEnv() {
  s.Env = &Environment{}
  m := map[string]string {
    "WERCKER_STEP_ROOT": s.GuestPath(),
    "WERCKER_STEP_ID": s.SafeId,
    "WERCKER_STEP_OWNER": s.Owner,
    "WERCKER_STEP_NAME": s.Name,
    "WERCKER_REPORT_NUMBERS_FILE": s.ReportPath("numbers.ini"),
    "WERCKER_REPORT_MESSAGE_FILE": s.ReportPath("message.txt"),
    "WERCKER_REPORT_ARTIFACTS_DIR": s.ReportPath("artifacts"),
  }
  s.Env.Update(m)

  u := map[string]string{}

  defaults := s.stepConfig.Defaults()

  for k, defaultValue := range defaults {
    value, ok := s.data[k]
    key := fmt.Sprintf("WERCKER_%s_%s", strings.Replace(s.Name, "-", "_", -1), k)
    key = strings.ToUpper(key)
    if !ok {
      u[key] = defaultValue
    } else {
      u[key] = value
    }
  }

  s.Env.Update(u)
}


func (s *Step) HostPath(p ...string) string {
  newArgs := append([]string{s.SafeId}, p...)
  return s.build.HostPath(newArgs...)
}


func (s *Step) GuestPath(p ...string) string {
  newArgs := append([]string{s.SafeId}, p...)
  return s.build.GuestPath(newArgs...)
}


func (s *Step) MntPath(p ...string) string {
  newArgs := append([]string{s.SafeId}, p...)
  return s.build.MntPath(newArgs...)
}


func (s *Step) ReportPath(p ...string) string {
  newArgs := append([]string{s.SafeId}, p...)
  return s.build.ReportPath(newArgs...)
}
