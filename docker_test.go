package main

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/sentcli/util"
)

type DockerSuite struct {
	*util.TestSuite
}

func TestDockerSuite(t *testing.T) {
	suiteTester := &DockerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *DockerSuite) TestNormalizeRegistry() {
	quay := "https://quay.io/v1/"
	dock := "https://registry.hub.docker.com/v1/"
	s.Equal(quay, normalizeRegistry("https://quay.io"))
	s.Equal(quay, normalizeRegistry("https://quay.io/v1"))
	s.Equal(quay, normalizeRegistry("http://quay.io/v1"))
	s.Equal(quay, normalizeRegistry("https://quay.io/v1/"))
	s.Equal(quay, normalizeRegistry("quay.io"))

	s.Equal(dock, normalizeRegistry(""))
	s.Equal(dock, normalizeRegistry("https://registry.hub.docker.com"))
	s.Equal(dock, normalizeRegistry("http://registry.hub.docker.com"))
	s.Equal(dock, normalizeRegistry("registry.hub.docker.com"))
}

func (s *DockerSuite) TestNormalizeRepo() {
	s.Equal("gox-mirror", normalizeRepo("example.com/gox-mirror"))
	s.Equal("termie/gox-mirror", normalizeRepo("quay.io/termie/gox-mirror"))
	s.Equal("termie/gox-mirror", normalizeRepo("termie/gox-mirror"))
	s.Equal("mongo", normalizeRepo("mongo"))
}

func (s *DockerSuite) TestPing() {
	client := DockerOrSkip(s.T())
	err := client.Ping()
	s.Nil(err)
}

func (s *DockerSuite) TestGenerateDockerID() {
	id, err := GenerateDockerID()
	s.Require().NoError(err, "Unable to generate Docker ID")

	// The ID needs to be a valid hex value
	b, err := hex.DecodeString(id)
	s.Require().NoError(err, "Generated Docker ID was not a hex value")

	// The ID needs to be 256 bits
	s.Equal(256, len(b)*8)
}
