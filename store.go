package main

import "fmt"

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
