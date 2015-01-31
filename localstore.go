package main

import (
	"io"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
)

// NewLocalStore creates a new LocalStore.
func NewLocalStore(baseDirectory string) *LocalStore {
	return &LocalStore{baseDirectory}
}

// LocalStore stores content in base.
type LocalStore struct {
	base string
}

// StoreFromFile copies the file from args.Path to s.base + args.Key.
func (s *LocalStore) StoreFromFile(args *StoreFromFileArgs) error {
	// NOTE(bvdberg): For now only linux paths are supported, since
	// GenerateBaseKey is expected to return / separators.
	outputPath := path.Join(s.base, args.Key)
	inputFile, err := os.Open(args.Path)
	if err != nil {
		log.WithField("Error", err).Error("Unable to open image")
		return err
	}
	defer inputFile.Close()

	outputDirectory := path.Dir(outputPath)
	log.WithField("Directory", outputDirectory).
		Debug("Creating output directory")
	err = os.MkdirAll(outputDirectory, 0777)
	if err != nil {
		log.WithField("Error", err).
			Error("Unable to create container directory")
		return err
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		log.WithField("Error", err).Error("Unable to create output file")
		return err
	}
	defer outputFile.Close()

	log.Println("Starting to copy to container directory")

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		log.WithField("Error", err).
			Error("Unable to copy input file to container directory")
		return err
	}

	log.WithField("Path", outputFile.Name()).
		Println("Copied container to container directory")
	return nil
}
