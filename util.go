package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// exists is like python's os.path.exists and too many lines in Go
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// untargzip tries to untar-gzip stuff to a path
func untargzip(path string, r io.Reader) error {
	ungzipped, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tarball := tar.NewReader(ungzipped)

	defer ungzipped.Close()

	// We have to treat things differently for git-archives
	isGitArchive := false

	// Alright, things seem in order, let's make the base directory
	os.MkdirAll(path, 0755)
	for {
		hdr, err := tarball.Next()
		if err == io.EOF {
			// finished the tar
			break
		}
		if err != nil {
			return err
		}
		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		// If this was made with git-archive it will be in kinda an ugly
		// format, but we can identify it by the pax_global_header "file"
		name := hdr.Name
		if name == "pax_global_header" {
			isGitArchive = true
			continue
		}

		// It will also contain an extra subdir that we will automatically strip
		if isGitArchive {
			parts := strings.Split(name, "/")
			name = strings.Join(parts[1:], "/")
		}

		fpath := filepath.Join(path, name)
		if hdr.FileInfo().IsDir() {
			err = os.MkdirAll(fpath, 0755)
			if err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE, hdr.FileInfo().Mode())
		defer file.Close()
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarball)
		if err != nil {
			return err
		}
	}
	return nil
}
