package sentcli

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/sentcli/docker"
	"github.com/wercker/sentcli/util"
)

// Artificer collects artifacts from containers and uploads them.
type Artificer struct {
	options *PipelineOptions
	logger  *util.LogEntry
	store   Store
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
	Key           string
	ContentType   string
	Meta          map[string]*string
}

// NewArtificer returns an Artificer
func NewArtificer(options *PipelineOptions) *Artificer {
	logger := util.RootLogger().WithField("Logger", "Artificer")

	s3store := NewS3Store(options.AWSOptions)

	return &Artificer{
		options: options,
		logger:  logger,
		store:   s3store,
	}
}

// URL returns the artifact's S3 url
func (art *Artifact) URL() string {
	return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", art.Bucket, art.RemotePath())
}

// RemotePath returns the S3 path for an artifact
func (art *Artifact) RemotePath() string {
	if art.Key != "" {
		return art.Key
	}
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

// Cleanup removes files from the host
func (art *Artifact) Cleanup() error {
	return os.Remove(art.HostPath)
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
	return a.store.StoreFromFile(&StoreFromFileArgs{
		Path:        artifact.HostPath,
		Key:         artifact.RemotePath(),
		ContentType: artifact.ContentType,
		MaxTries:    3,
		Meta:        artifact.Meta,
	})
}

// FileCollector gets files out of containers
type FileCollector interface {
	Collect(path string) (*Archive, chan error)
}

// DockerFileCollector impl of FileCollector
type DockerFileCollector struct {
	client      *dockerlocal.DockerClient
	containerID string
	logger      *util.LogEntry
}

// NewDockerFileCollector constructor
func NewDockerFileCollector(client *dockerlocal.DockerClient, containerID string) FileCollector {
	return &DockerFileCollector{
		client:      client,
		containerID: containerID,
		logger:      util.RootLogger().WithField("Logger", "DockerFileCollector"),
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
