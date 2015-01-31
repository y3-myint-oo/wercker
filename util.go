package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
)

func expanduser(path string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if path[:2] == "~/" {
		path = strings.Replace(path, "~", dir, 1)
	}
	return path
}

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

// fetchTarball tries to fetch a tarball
// For now this is pretty naive and useless, but we are doing it in a couple
// places and this is a fine stub to expand upon.
func fetchTarball(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return resp, fmt.Errorf("Bad status code fetching tarball: %s", url)
	}

	return resp, nil
}

// Write the contents up a single file to dst
func untarOne(name string, dst io.Writer, src io.ReadCloser) error {
	// ungzipped, err := gzip.NewReader(src)
	// if err != nil {
	//   return err
	// }
	tarball := tar.NewReader(src)
	defer src.Close()
	// defer tarball.Close()

	for {
		hdr, err := tarball.Next()
		if err == io.EOF {
			// finished the tar
			break
		}
		if err != nil {
			return err
		}

		if hdr.Name != name {
			continue
		}

		// We found the file we care about
		_, err = io.Copy(dst, tarball)
		break
	}
	return nil
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
		file.Close()
	}
	return nil
}

// Finisher is a helper class for running something either right away or
// at `defer` time.
type Finisher struct {
	callback   func(interface{})
	isFinished bool
}

// NewFinisher returns a new Finisher with a callback.
func NewFinisher(callback func(interface{})) *Finisher {
	return &Finisher{callback: callback, isFinished: false}
}

// Finish executes the callback if it hasn't been run yet.
func (f *Finisher) Finish(result interface{}) {
	if f.isFinished {
		return
	}
	f.isFinished = true
	f.callback(result)
}

// Retrieving user input utility functions

func askForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatal(err)
	}
	response = strings.ToLower(response)
	if strings.HasPrefix(response, "y") {
		return true
	} else if strings.HasPrefix(response, "n") {
		return false
	} else {
		log.Println("Please type yes or no and then press enter:")
		return askForConfirmation()
	}
}

// Counter is a simple struct
type Counter struct {
	Current int
	l       sync.Mutex
}

// Increment will return current and than increment c.Current.
func (c *Counter) Increment() int {
	c.l.Lock()
	defer c.l.Unlock()

	current := c.Current
	c.Current = current + 1

	return current
}

// ContainsString checks if the array items contains the string target.
// TODO(bvdberg): write units tests
func ContainsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
