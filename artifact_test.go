package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ArtifactSuite struct {
	*TestSuite
}

func TestArtifactSuite(t *testing.T) {
	suiteTester := &ArtifactSuite{&TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *ArtifactSuite) TestDockerFileCollectorSingle() {
	client := s.DockerOrSkip()

	container, err := tempBusybox(client)
	s.Nil(err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/etc/alpine-release")
	var b bytes.Buffer

	select {
	case err := <-archive.SingleBytes("alpine-release", &b):
		s.Nil(err)
	case err := <-errs:
		s.Nil(err)
		s.T().FailNow()
	}

	s.Equal("3.1.4\n", b.String())
}

func (s *ArtifactSuite) TestDockerFileCollectorSingleNotFound() {
	client := s.DockerOrSkip()

	container, err := tempBusybox(client)
	s.Nil(err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	// Fail first from docker client
	archive, errs := dfc.Collect("/notfound/file")
	var b bytes.Buffer
	select {
	case <-archive.SingleBytes("file", &b):
		s.T().FailNow()
	case err := <-errs:
		s.Equal(err, ErrEmptyTarball)
	}

	// Or from archive
	archive, errs = dfc.Collect("/etc/issue")
	var b2 bytes.Buffer
	select {
	case err := <-archive.SingleBytes("notfound", &b2):
		s.Equal(err, ErrEmptyTarball)
	case <-errs:
		s.T().FailNow()
	}
}

func (s *ArtifactSuite) TestDockerFileCollectorMulti() {
	client := s.DockerOrSkip()

	container, err := tempBusybox(client)
	s.Nil(err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/etc/apk")
	var b bytes.Buffer

	select {
	case err := <-archive.SingleBytes("apk/arch", &b):
		s.Nil(err)
	case <-errs:
		s.T().FailNow()
	}

	check := "x86_64\n"
	s.Equal(check, b.String())
}

func (s *ArtifactSuite) TestDockerFileCollectorMultiEmptyTarball() {
	client := s.DockerOrSkip()

	container, err := tempBusybox(client)
	s.Nil(err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/var/tmp")

	tmp, err := ioutil.TempDir("", "test-")
	s.Nil(err)
	defer os.RemoveAll(tmp)

	select {
	case err := <-archive.Multi("tmp", tmp, 1024*1024*1000):
		s.Equal(err, ErrEmptyTarball)
	case <-errs:
		s.FailNow()
	}
}

func (s *ArtifactSuite) TestDockerFileCollectorMultiNotFound() {
	client := s.DockerOrSkip()

	container, err := tempBusybox(client)
	s.Nil(err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/notfound")

	tmp, err := ioutil.TempDir("", "test-")
	s.Nil(err)
	defer os.RemoveAll(tmp)

	select {
	case <-archive.Multi("default", tmp, 1024*1024*1000):
		s.FailNow()
	case err := <-errs:
		s.Equal(err, ErrEmptyTarball)
	}
}
