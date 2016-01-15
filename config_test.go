package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/sentcli/util"
)

type ConfigSuite struct {
	*util.TestSuite
}

func TestConfigSuite(t *testing.T) {
	suiteTester := &ConfigSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *ConfigSuite) TestConfigBoxStrings() {
	b, err := ioutil.ReadFile("./tests/box_strings.yml")
	s.Nil(err)
	config, err := ConfigFromYaml(b)
	s.Require().Nil(err)
	s.Equal("strings_box", config.Box.ID)
	s.Equal("strings_service", config.Services[0].ID)
}

func (s *ConfigSuite) TestConfigBoxStructs() {
	b, err := ioutil.ReadFile("./tests/box_structs.yml")
	s.Nil(err)
	config, err := ConfigFromYaml(b)
	s.Require().Nil(err)
	s.Equal("structs_box", config.Box.ID)
	s.Equal("structs_service", config.Services[0].ID)

	pipeline := config.PipelinesMap["pipeline"]
	s.Equal(pipeline.Box.ID, "blue")
	s.Equal(pipeline.Steps[0].ID, "string-step")
	s.Equal(pipeline.Steps[1].ID, "script")
	s.Equal(pipeline.Steps[2].ID, "script")
}

func (s *ConfigSuite) TestIfaceToString() {
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
		s.Equal(test.expected, actual, "")
	}
}
