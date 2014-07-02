package main

import (
  // "fmt"
  "io/ioutil"
  "gopkg.in/yaml.v1"
)


type Box string


type Build struct {
  Steps []*RawStep
}


type Config struct {
  Box *Box
  Build *Build
}


type RawStep map[string]RawStepData


type RawStepData map[string]string


func ConfigFromYaml(filename string) (*Config, error) {
  file, err := ioutil.ReadFile("projects/termie/farmboy/wercker.yml")
  if err != nil {
    return nil, err
  }

  // m := make(map[string]interface{})
  var m Config

  err = yaml.Unmarshal(file, &m)
  if err != nil {
    return nil, err
  }

  // fmt.Println(m)
  // fmt.Println(m.Build)
  // fmt.Println(m.Build.Steps[0])

  // for _, v := range m.Build.Steps {
  //   fmt.Println(v)
  //   for id, data := range v {
  //     fmt.Println(id, data)
  //   }
  // }

  // Build a Box
  // box := CreateBoxFromYaml(m["box"].(string))

  // build := m["build"].(map[interface{}]interface{})
  // steps := build["steps"].([]interface{})
  // stepList := []StepTuple{}

  // for _, v := range steps {
  //   var stepId string
  //   stepData := make(map[string]string)

  //   // There is only one key in this array but can't just pop in golang
  //   for id, data := range v.(map[interface{}]interface{}) {
  //     stepId = id.(string)
  //     for prop, value := range data.(map[interface{}]interface{}) {
  //       stepData[prop.(string)] = value.(string)
  //     }
  //   }
  //   fmt.Println(stepId, stepData)
  //   stepList = append(stepList, StepTuple{stepId, stepData})
  // }
  // fmt.Println(stepList)

  return &m, nil
}
