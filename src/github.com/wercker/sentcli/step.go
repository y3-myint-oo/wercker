package main

import (
  "fmt"
  "io/ioutil"
  "os"
  "path/filepath"
  "strings"
  "code.google.com/p/go-uuid/uuid"
  "github.com/termie/go-shutil"
)


type Step struct {
  Id string
  Owner string
  Name string
  DisplayName string
  data RawStepData
  Build *Build
  options *GlobalOptions
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

  return &Step{Id:stepId, Owner:owner, Name:name, DisplayName:displayName, data:data, Build:build, options:options}, nil
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
  hostStepPath := s.Build.HostPath(s.Id)
  scriptPath := s.Build.HostPath(s.Id, "run.sh")
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


func (s *Step) Fetch() (string, error) {
  // NOTE(termie): polymorphism based on kind, we could probably do something
  //               with interfaces here, but this is okay for now
  if s.IsScript() {
    return s.FetchScript()
  }

  // TODO(termie): Actually fetch the step!
  stepPath := filepath.Join(s.options.StepDir, s.Id)
  hostStepPath := s.Build.HostPath(s.Id)

  err := shutil.CopyTree(stepPath, hostStepPath, nil)
  if err != nil {
    return "", nil
  }

  return hostStepPath, nil
}





