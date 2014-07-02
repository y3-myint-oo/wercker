package main

import (
  // "fmt"
  "io/ioutil"
  "gopkg.in/yaml.v1"
)


type RawBox string


type RawBuild struct {
  Steps []*RawStep
}


type RawConfig struct {
  Box *RawBox
  Build *RawBuild
}


type RawStep map[string]RawStepData


type RawStepData map[string]string





func ConfigFromYaml(filename string) (*RawConfig, error) {
  file, err := ioutil.ReadFile("projects/termie/farmboy/wercker.yml")
  if err != nil {
    return nil, err
  }

  var m RawConfig

  err = yaml.Unmarshal(file, &m)
  if err != nil {
    return nil, err
  }

  return &m, nil
}
