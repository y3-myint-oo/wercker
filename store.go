package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/fsouza/go-dockerclient"
	"github.com/mreiferson/go-snappystream"
	"github.com/pborman/uuid"
	"github.com/wercker/sentcli/util"

	"golang.org/x/net/context"
)

// Store is generic store interface
type Store interface {
	// StoreFromFile copies a file from local disk to the store
	StoreFromFile(*StoreFromFileArgs) error
}

// StoreFromFileArgs are the args for storing a file
type StoreFromFileArgs struct {
	// Path to the local file.
	Path string

	// Key of the file as stored in the store.
	Key string

	// ContentType hints to the content-type of the file (might be ignored)
	ContentType string

	// Meta data associated with the upload (might be ignored)
	Meta map[string]*string

	// MaxTries is the maximum that a store should retry should the store fail.
	MaxTries int
}

// GenerateBaseKey generates the base key based on ApplicationID and either
// DeployID or BuilID
func GenerateBaseKey(options *PipelineOptions) string {
	key := fmt.Sprintf("project-artifacts/%s", options.ApplicationID)
	if options.DeployID != "" {
		key = fmt.Sprintf("%s/deploy/%s", key, options.DeployID)
	} else {
		key = fmt.Sprintf("%s/build/%s", key, options.BuildID)
	}

	return key
}

// StoreContainerStep stores the container that was built
type StoreContainerStep struct {
	*BaseStep
	data     map[string]string
	logger   *util.LogEntry
	artifact *Artifact
}

// NewStoreContainerStep constructor
func NewStoreContainerStep(stepConfig *StepConfig, options *PipelineOptions) (*StoreContainerStep, error) {
	name := "store-container"
	displayName := "store container"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := &BaseStep{
		displayName: displayName,
		env:         &util.Environment{},
		id:          name,
		name:        name,
		options:     options,
		owner:       "wercker",
		safeID:      stepSafeID,
		version:     Version(),
	}

	return &StoreContainerStep{
		BaseStep: baseStep,
		data:     stepConfig.Data,
		logger:   util.RootLogger().WithField("Logger", "StoreContainerStep"),
	}, nil

}

// InitEnv preps our env
func (s *StoreContainerStep) InitEnv(env *util.Environment) {
	// NOP
}

// Fetch NOP
func (s *StoreContainerStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// DockerRepo calculates our repo name
func (s *StoreContainerStep) DockerRepo() string {
	if s.options.Repository != "" {
		return s.options.Repository
	}
	return fmt.Sprintf("build-%s", s.options.BuildID)
}

// DockerTag calculates our tag
func (s *StoreContainerStep) DockerTag() string {
	if s.options.Tag != "" {
		return s.options.Tag
	}
	return "latest"
}

// DockerMessage calculates our message
func (s *StoreContainerStep) DockerMessage() string {
	message := s.options.Message
	if message == "" {
		message = fmt.Sprintf("Build %s", s.options.BuildID)
	}
	return message
}

// Execute does the actual export and upload of the container
func (s *StoreContainerStep) Execute(ctx context.Context, sess *Session) (int, error) {
	e, err := EmitterFromContext(ctx)
	if err != nil {
		return -1, err
	}
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.options.DockerOptions)
	if err != nil {
		return -1, err
	}
	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.transport.(*DockerTransport)
	containerID := dt.containerID

	repoName := s.DockerRepo()
	tag := s.DockerTag()
	message := s.DockerMessage()

	commitOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: repoName,
		Tag:        tag,
		Author:     "wercker",
		Message:    message,
	}
	s.logger.Debugln("Commit container:", containerID)
	i, err := client.CommitContainer(commitOpts)
	if err != nil {
		return -1, err
	}
	s.logger.WithField("Image", i).Debug("Commit completed")

	e.Emit(Logs, &LogsArgs{
		Logs: "Exporting container\n",
	})

	file, err := ioutil.TempFile(s.options.BuildPath(), "export-image-")
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to create temporary file")
		return -1, err
	}

	hash := sha256.New()
	w := snappystream.NewWriter(io.MultiWriter(file, hash))

	exportImageOptions := docker.ExportImageOptions{
		Name:         repoName,
		OutputStream: w,
	}
	err = client.ExportImage(exportImageOptions)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to export image")
		return -1, err
	}

	// Copy is done now, so close temporary file and set the calculatedHash
	file.Close()

	calculatedHash := hex.EncodeToString(hash.Sum(nil))

	s.logger.WithFields(util.LogFields{
		"SHA256":            calculatedHash,
		"TemporaryLocation": file.Name(),
	}).Println("Export image successful")

	key := GenerateBaseKey(s.options)
	key = fmt.Sprintf("%s/%s", key, "docker.tar.sz")

	s.artifact = &Artifact{
		HostPath:    file.Name(),
		Key:         key,
		Bucket:      s.options.S3Bucket,
		ContentType: "application/x-snappy-framed",
		Meta: map[string]*string{
			"Sha256": &calculatedHash,
		},
	}

	return 0, nil
}

// CollectFile NOP
func (s *StoreContainerStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact return an artifact pointing at the exported thing we made
func (s *StoreContainerStep) CollectArtifact(string) (*Artifact, error) {
	return s.artifact, nil
}

// ReportPath NOP
func (s *StoreContainerStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *StoreContainerStep) ShouldSyncEnv() bool {
	return true
}
