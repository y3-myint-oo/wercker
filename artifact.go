package main

import (
	"archive/tar"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
	"github.com/fsouza/go-dockerclient"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	fiveMegabytes = 5 * 1024 * 1024
)

// Artificer collects artifacts from containers and uploads them.
type Artificer struct {
	options *GlobalOptions
}

// Artifact holds the information required to extract a folder
// from a container and eventually upload it to S3.
type Artifact struct {
	ContainerID   string
	GuestPath     string
	HostPath      string
	ApplicationID string
	BuildID       string
	DeployID      string
	BuildStepID   string
}

var (
	// ErrEmptyTarball is returned when the tarball has no files in it
	ErrEmptyTarball = errors.New("empty tarball")
)

// NewArtificer returns an Artificer
func NewArtificer(options *GlobalOptions) *Artificer {
	return &Artificer{options: options}
}

// RemotePath returns the S3 path for an artifact
func (art *Artifact) RemotePath() string {
	path := fmt.Sprintf("project-artifacts/project-%s", art.ApplicationID)
	if art.DeployID != "" {
		path = fmt.Sprintf("%s/deploys/deploy-%s", path, art.DeployID)
	} else {
		path = fmt.Sprintf("%s/builds/build-%s", path, art.BuildID)
	}
	if art.BuildStepID != "" {
		path = fmt.Sprintf("%s/buildStep-%s", path, art.BuildStepID)
	}
	path = fmt.Sprintf("%s/%s", path, filepath.Base(art.HostPath))
	return path
}

// Collect an artifact from the container, if it doesn't have any files in
// the tarball return ErrEmptyTarball
func (a *Artificer) Collect(artifact *Artifact) (*Artifact, error) {
	client, _ := NewDockerClient(a.options)

	if err := os.MkdirAll(filepath.Dir(artifact.HostPath), 0755); err != nil {
		return nil, err
	}

	outputFile, err := os.Create(artifact.HostPath)
	defer outputFile.Close()
	if err != nil {
		return nil, err
	}

	opts := docker.CopyFromContainerOptions{
		OutputStream: outputFile,
		Container:    artifact.ContainerID,
		Resource:     artifact.GuestPath,
	}
	if err = client.CopyFromContainer(opts); err != nil {
		return nil, err
	}

	if _, err = outputFile.Seek(0, 0); err != nil {
		return nil, err
	}

	hasFiles := false
	tarReader := tar.NewReader(outputFile)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !header.FileInfo().IsDir() {
			hasFiles = true
			break
		}
	}
	if !hasFiles {
		return nil, ErrEmptyTarball
	}

	return artifact, nil
}

// Upload an artifact to S3
func (a *Artificer) Upload(artifact *Artifact) error {

	auth, err := aws.GetAuth(
		a.options.AWSAccessKeyID,
		a.options.AWSSecretAccessKey,
		"",
		time.Now().Add(time.Minute*10))
	if err != nil {
		return err
	}

	log.Println("Uploading artifact:", artifact.RemotePath())
	region := aws.Regions[a.options.AWSRegion]

	s := s3.New(auth, region)
	b := s.Bucket(a.options.S3Bucket)

	f, err := os.Open(artifact.HostPath)
	if err != nil {
		return err
	}
	defer f.Close()

	multi, err := b.Multi(artifact.RemotePath(), "application/x-tar", s3.Private, s3.Options{SSE: true})
	if err != nil {
		return err
	}
	parts, err := multi.PutAll(f, fiveMegabytes)
	if err != nil {
		return err
	}
	if err = multi.Complete(parts); err != nil {
		return err
	}

	return nil
}
