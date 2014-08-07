package main

import (
  "errors"
  "fmt"
  "io/ioutil"
  "os"
  "gopkg.in/yaml.v1"
)


type RawBox string


type RawBuild struct {
  RawSteps []*RawStep `yaml:"steps"`
}


type RawConfig struct {
  RawBox *RawBox `yaml:"box"`
  RawBuild *RawBuild `yaml:"build"`
}


type RawStep map[string]RawStepData


type RawStepData map[string]string


// exists is like python's os.path.exists and too many lines in Go
func exists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil {
      return true, nil
    }
    if os.IsNotExist(err) {
      return false, nil
    }
    return false, err
}

// ReadWerckerYaml will try to find a wercker.yml file and return its bytes.
// TODO(termie): If allowDefault is true it will try to generate a
// default yaml file by inspecting the project.
func ReadWerckerYaml(searchDirs []string, allowDefault bool) ([]byte, error) {
  var foundYaml string
  found := false

  for _, v := range searchDirs {
    possibleYaml := fmt.Sprintf("%s/wercker.yml", v)
    ymlExists, err := exists(possibleYaml)
    if err != nil {
      return nil, err
    }
    if !ymlExists {
      continue
    }
    found = true
    foundYaml = possibleYaml
  }

  // TODO(termie): If allowDefault, we'd generate something here
  if !allowDefault && !found {
    return nil, errors.New("No wercker.yml found and no defaults allowed.")
  }

  return ioutil.ReadFile(foundYaml)
}


// Read a []byte as yaml and turn it into a RawConfig object
func ConfigFromYaml(file []byte) (*RawConfig, error) {
  var m RawConfig

  err := yaml.Unmarshal(file, &m)
  if err != nil {
    return nil, err
  }

  return &m, nil
}
