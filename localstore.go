package main

import (
	"io"
	"os"
	"path"
)

// NewLocalStore creates a new LocalStore.
func NewLocalStore(baseDirectory string) *LocalStore {
	logger := rootLogger.WithField("Logger", "LocalStore")
	return &LocalStore{base: baseDirectory, logger: logger}
}

// LocalStore stores content in base.
type LocalStore struct {
	base   string
	logger *LogEntry
}

// StoreFromFile copies the file from args.Path to s.base + args.Key.
func (s *LocalStore) StoreFromFile(args *StoreFromFileArgs) error {
	// NOTE(bvdberg): For now only linux paths are supported, since
	// GenerateBaseKey is expected to return / separators.
	outputPath := path.Join(s.base, args.Key)
	inputFile, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open image")
		return err
	}
	defer inputFile.Close()

	outputDirectory := path.Dir(outputPath)
	s.logger.WithField("Directory", outputDirectory).
		Debug("Creating output directory")
	err = os.MkdirAll(outputDirectory, 0777)
	if err != nil {
		s.logger.WithField("Error", err).
			Error("Unable to create container directory")
		return err
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to create output file")
		return err
	}
	defer outputFile.Close()

	s.logger.Println("Starting to copy to container directory")

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		s.logger.WithField("Error", err).
			Error("Unable to copy input file to container directory")
		return err
	}

	s.logger.WithField("Path", outputFile.Name()).
		Println("Copied container to container directory")
	return nil
}
