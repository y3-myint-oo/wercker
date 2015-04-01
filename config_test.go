package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigBoxStrings(t *testing.T) {
	b, err := ioutil.ReadFile("./tests/box_strings.yml")
	assert.Nil(t, err)
	config, err := ConfigFromYaml(b)
	assert.Nil(t, err)
	assert.Equal(t, "strings_box", config.Box.ID)
	assert.Equal(t, "strings_service", config.Services[0].ID)
}

func TestConfigBoxStructs(t *testing.T) {
	b, err := ioutil.ReadFile("./tests/box_structs.yml")
	assert.Nil(t, err)
	config, err := ConfigFromYaml(b)
	assert.Nil(t, err)
	assert.Equal(t, "structs_box", config.Box.ID)
	assert.Equal(t, "structs_service", config.Services[0].ID)
}
