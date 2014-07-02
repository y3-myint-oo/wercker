package main

import (
  "fmt"
  "strings"
  "code.google.com/p/go-uuid/uuid"
)


type Step struct {
  id string
  owner string
  name string
  displayName string
  data RawStepData
  build *Build
}


// Convert a RawStep into a Step
func (s *RawStep) Step(build *Build) (*Step, error) {
  // There should only be one step in the internal map
  var stepId string
  var stepData RawStepData

  // Dereference ourself to get to our underlying data structure
  for id, data := range *s {
    stepId = id
    stepData = data
  }
  return CreateStep(stepId, stepData, build)
}


func CreateStep(stepId string, data RawStepData, build *Build) (*Step, error) {
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

  return &Step{id:stepId, owner:owner, name:name, data:data, displayName:displayName, build:build}, nil
}





