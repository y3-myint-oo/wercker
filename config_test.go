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

func TestIfaceToString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"string input", "string input"},
		{int(1234), "1234"},
		{int32(123432), "123432"},
		{int64(123464), "123464"},
		{true, "true"},
		{false, "false"},

		// The following types are not supported, so a empty string is returned
		{nil, ""},
		{float32(123.123), ""},
		{float64(123.123), ""},
	}

	for _, test := range tests {
		actual := ifaceToString(test.input)
		assert.Equal(t, test.expected, actual, "")
	}
}
