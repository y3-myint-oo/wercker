package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
	"github.com/fsouza/go-dockerclient"
)

// Artificer collects artifacts from containers and uploads them.
type Artificer struct {
	options *PipelineOptions
	logger  *LogEntry
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
	Bucket        string
}

// NewArtificer returns an Artificer
func NewArtificer(options *PipelineOptions) *Artificer {
	logger := rootLogger.WithField("Logger", "Artificer")
	return &Artificer{options: options, logger: logger}
}

// URL returns the artifact's S3 url
func (art *Artifact) URL() string {
	return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", art.Bucket, art.RemotePath())
}

// RemotePath returns the S3 path for an artifact
func (art *Artifact) RemotePath() string {
	path := fmt.Sprintf("project-artifacts/%s", art.ApplicationID)
	if art.DeployID != "" {
		path = fmt.Sprintf("%s/deploy/%s", path, art.DeployID)
	} else {
		path = fmt.Sprintf("%s/build/%s", path, art.BuildID)
	}
	if art.BuildStepID != "" {
		path = fmt.Sprintf("%s/step/%s", path, art.BuildStepID)
	}
	path = fmt.Sprintf("%s/%s", path, filepath.Base(art.HostPath))
	return path
}

// Collect an artifact from the container, if it doesn't have any files in
// the tarball return ErrEmptyTarball
func (a *Artificer) Collect(artifact *Artifact) (*Artifact, error) {
	client, _ := NewDockerClient(a.options.DockerOptions)

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

	a.logger.Println("Uploading artifact:", artifact.RemotePath())
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
	parts, err := multi.PutAll(f, a.options.S3PartSize)
	if err != nil {
		return err
	}
	if err = multi.Complete(parts); err != nil {
		return err
	}

	return nil
}

// FileCollector gets files out of containers
type FileCollector interface {
	Collect(path string) (*Archive, chan error)
}

// DockerFileCollector impl of FileCollector
type DockerFileCollector struct {
	client      *DockerClient
	containerID string
	logger      *LogEntry
}

// NewDockerFileCollector constructor
func NewDockerFileCollector(client *DockerClient, containerID string) FileCollector {
	return &DockerFileCollector{
		client:      client,
		containerID: containerID,
		logger:      rootLogger.WithField("Logger", "DockerFileCollector"),
	}
}

// Collect grabs a path and returns an archive containing stream along with
// an error channel to select on
func (fc *DockerFileCollector) Collect(path string) (*Archive, chan error) {
	pipeReader, pipeWriter := io.Pipe()
	opts := docker.CopyFromContainerOptions{
		OutputStream: pipeWriter,
		Container:    fc.containerID,
		Resource:     path,
	}

	errs := make(chan error)

	go func() {
		defer close(errs)
		if err := fc.client.CopyFromContainer(opts); err != nil {
			switch err.(type) {
			case *docker.Error:
				derr := err.(*docker.Error)
				if derr.Status == 500 && strings.HasPrefix(derr.Message, "Could not find the file") {
					errs <- ErrEmptyTarball
				}
			default:
				errs <- err
			}
		}
		pipeWriter.Close()
	}()

	return NewArchive(pipeReader), errs
}
