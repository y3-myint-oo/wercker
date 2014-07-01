package main

import (
  "fmt"
  "strings"
  "code.google.com/p/go-uuid/uuid"
)


type Build struct {
  steps []Step
}


type Step struct {
  id string
  owner string
  name string
  data map[string]string
  build *Build
}


func CreateStep(stepId string, data map[string]string, build *Build) (*Step, error) {
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

  return &Step{id:stepId, owner:owner, name:name, data:data, build:build}, nil
}





