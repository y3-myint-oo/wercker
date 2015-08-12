package main

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// NewS3Store creates a new S3Store
func NewS3Store(options *AWSOptions) *S3Store {
	logger := rootLogger.WithField("Logger", "S3Store")
	if options == nil {
		logger.Panic("options cannot be nil")
	}

	client := s3.New(&aws.Config{Region: &options.AWSRegion})

	return &S3Store{
		client:  client,
		logger:  logger,
		options: options,
	}
}

// S3Store stores files in S3
type S3Store struct {
	client  *s3.S3
	logger  *LogEntry
	options *AWSOptions
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *S3Store) StoreFromFile(args *StoreFromFileArgs) error {
	if args.MaxTries == 0 {
		args.MaxTries = 1
	}

	s.logger.WithFields(LogFields{
		"Bucket":   s.options.S3Bucket,
		"Path":     args.Path,
		"Region":   s.options.AWSRegion,
		"S3Key":    args.Key,
		"MaxTries": args.MaxTries,
	}).Info("Uploading file to S3")

	file, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open input file")
		return err
	}
	defer file.Close()

	var outerErr error
	uploadManager := s3manager.NewUploader(&s3manager.UploadOptions{
		S3:       s.client,
		PartSize: s.options.S3PartSize,
	})
	for try := 1; try <= args.MaxTries; try++ {

		_, err = uploadManager.Upload(&s3manager.UploadInput{
			ACL:                  aws.String("private"),
			Body:                 file,
			Bucket:               aws.String(s.options.S3Bucket),
			Key:                  aws.String(args.Key),
			Metadata:             args.Meta,
			ServerSideEncryption: aws.String("AES256"),
		})

		if err != nil {
			s.logger.WithFields(LogFields{
				"Bucket":   s.options.S3Bucket,
				"Path":     args.Path,
				"Region":   s.options.AWSRegion,
				"S3Key":    args.Key,
				"Try":      try,
				"MaxTries": args.MaxTries,
			}).Error("Unable to upload file to S3")
			outerErr = err
			continue
		}

		s.logger.WithFields(LogFields{
			"Bucket":   s.options.S3Bucket,
			"Path":     args.Path,
			"Region":   s.options.AWSRegion,
			"S3Key":    args.Key,
			"Try":      try,
			"MaxTries": args.MaxTries,
		}).Info("Uploading file to S3 complete")

		return nil
	}

	return outerErr
}
