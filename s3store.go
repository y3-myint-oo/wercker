package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
)

// NewS3Store creates a new S3Store
func NewS3Store(options *AWSOptions) *S3Store {
	if options == nil {
		log.Panic("options cannot be nil")
	}

	return &S3Store{options}
}

// S3Store stores files in S3
type S3Store struct {
	options *AWSOptions
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *S3Store) StoreFromFile(args *StoreFromFileArgs) error {
	log.WithFields(log.Fields{
		"Bucket": s.options.S3Bucket,
		"Path":   args.Path,
		"Region": s.options.AWSRegion,
		"S3Key":  args.Key,
	}).Info("Uploading file to S3")

	file, err := os.Open(args.Path)
	if err != nil {
		log.WithField("Error", err).Error("Unable to open input file")
		return err
	}
	defer file.Close()

	auth, err := aws.GetAuth(
		s.options.AWSAccessKeyID,
		s.options.AWSSecretAccessKey,
		"",
		time.Now().Add(time.Minute*10))
	if err != nil {
		log.WithField("Error", err).Error("Unable to create auth credentials")
		return err
	}

	region := aws.Regions[s.options.AWSRegion]
	bucket := s3.New(auth, region).Bucket(s.options.S3Bucket)

	log.Println("Creating multipart upload")

	multiOptions := s3.Options{
		SSE:  true,
		Meta: args.Meta,
	}
	multi, err := bucket.Multi(args.Key, args.ContentType, s3.Private, multiOptions)
	if err != nil {
		log.WithField("Error", err).Error("Unable to create multipart")
		return err
	}

	abort := true
	defer func() {
		if abort {
			log.Warn("Aborting multipart upload")
			multi.Abort()
		}
	}()

	log.Println("Starting to upload to S3")

	parts, err := multi.PutAll(file, fiveMegabytes)
	if err != nil {
		log.WithField("Error", err).Error("Unable to upload multiparts")
		return err
	}

	if err = multi.Complete(parts); err != nil {
		log.WithField("Error", err).Error("Unable to complete multipart upload")
		return err
	}

	// Reset abort flag
	abort = false

	log.Println("Upload to S3 complete")
	return nil
}
