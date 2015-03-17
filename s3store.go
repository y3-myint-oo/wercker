package main

import (
	"os"
	"time"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
)

// NewS3Store creates a new S3Store
func NewS3Store(options *AWSOptions) *S3Store {
	logger := rootLogger.WithField("Logger", "S3Store")
	if options == nil {
		logger.Panic("options cannot be nil")
	}

	return &S3Store{options, logger}
}

// S3Store stores files in S3
type S3Store struct {
	options *AWSOptions
	logger  *LogEntry
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *S3Store) StoreFromFile(args *StoreFromFileArgs) error {
	s.logger.WithFields(LogFields{
		"Bucket": s.options.S3Bucket,
		"Path":   args.Path,
		"Region": s.options.AWSRegion,
		"S3Key":  args.Key,
	}).Info("Uploading file to S3")

	file, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open input file")
		return err
	}
	defer file.Close()

	auth, err := aws.GetAuth(
		s.options.AWSAccessKeyID,
		s.options.AWSSecretAccessKey,
		"",
		time.Now().Add(time.Minute*10))
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to create auth credentials")
		return err
	}

	region := aws.Regions[s.options.AWSRegion]
	bucket := s3.New(auth, region).Bucket(s.options.S3Bucket)

	s.logger.Println("Creating multipart upload")

	multiOptions := s3.Options{
		SSE:  true,
		Meta: args.Meta,
	}
	multi, err := bucket.Multi(args.Key, args.ContentType, s3.Private, multiOptions)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to create multipart")
		return err
	}

	abort := true
	defer func() {
		if abort {
			s.logger.Warn("Aborting multipart upload")
			multi.Abort()
		}
	}()

	s.logger.Println("Starting to upload to S3")

	parts, err := multi.PutAll(file, s.options.S3PartSize)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to upload multiparts")
		return err
	}

	if err = multi.Complete(parts); err != nil {
		s.logger.WithField("Error", err).Error("Unable to complete multipart upload")
		return err
	}

	// Reset abort flag
	abort = false

	s.logger.Println("Upload to S3 complete")
	return nil
}
