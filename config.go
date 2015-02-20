package main

import (
	"fmt"
	"io/ioutil"
	"path"

	"gopkg.in/yaml.v1"
)

// RawBox is the data type for a box in the wercker.yml
type RawBox string

// RawServices is a list of auxilliary boxes to boot in the wercker.yml
type RawServices []RawBox

// RawPipeline is the data type for builds and deploys in the wercker.yml
type RawPipeline map[string]interface{}

func (r RawPipeline) GetBox() *RawBox {
	if s, ok := r["box"]; ok {
		box := RawBox(s.(string))
		return &box
	}
	return nil
}

// GetSteps retrieves the steps section for the build or deploy. Return nil if
// not found.
func (r RawPipeline) GetSteps(section string) []interface{} {
	if s, ok := r[section]; ok {
		return s.([]interface{})
	}
	return nil
}

// RawSteps retrieves the "steps" section for the build or deploy.
func (r *RawPipeline) RawSteps() []interface{} {
	return r.GetSteps("steps")
}

// RawAfterSteps retrieves the "after-steps" section for the build or deploy.
func (r *RawPipeline) RawAfterSteps() []interface{} {
	return r.GetSteps("after-steps")
}

// RawConfig is the data type for wercker.yml
type RawConfig struct {
	SourceDir   string       `yaml:"source-dir"`
	RawBox      *RawBox      `yaml:"box"`
	RawServices RawServices  `yaml:"services"`
	RawBuild    *RawPipeline `yaml:"build"`
	RawDeploy   *RawPipeline `yaml:"deploy"`
}

// RawStep is the data type for a step in wercker.yml
type RawStep map[string]RawStepData

// RawStepData is the data type for the contents of a step in wercker.yml
type RawStepData map[string]string

func findYaml(searchDirs []string) (string, error) {
	possibleYaml := []string{"ewok.yml", "wercker.yml", ".wercker.yml"}

	for _, v := range searchDirs {
		for _, y := range possibleYaml {
			possibleYaml := path.Join(v, y)
			ymlExists, err := exists(possibleYaml)
			if err != nil {
				return "", err
			}
			if !ymlExists {
				continue
			}
			return possibleYaml, nil
		}
	}
	return "", fmt.Errorf("No wercker.yml found")
}

// ReadWerckerYaml will try to find a wercker.yml file and return its bytes.
// TODO(termie): If allowDefault is true it will try to generate a
// default yaml file by inspecting the project.
func ReadWerckerYaml(searchDirs []string, allowDefault bool) ([]byte, error) {
	foundYaml, err := findYaml(searchDirs)
	if err != nil {
		return nil, err
	}

	// TODO(termie): If allowDefault, we'd generate something here
	// if !allowDefault && !found {
	//   return nil, errors.New("No wercker.yml found and no defaults allowed.")
	// }

	return ioutil.ReadFile(foundYaml)
}

// ConfigFromYaml reads a []byte as yaml and turn it into a RawConfig object
func ConfigFromYaml(file []byte) (*RawConfig, error) {
	var m RawConfig

	err := yaml.Unmarshal(file, &m)
	if err != nil {
		errStr := err.Error()
		err = fmt.Errorf(`Error parsing your wercker.yml:
  %s
`, errStr)
		return nil, err
	}

	return &m, nil
}
