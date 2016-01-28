//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package dockerlocal

import (
	"path/filepath"
	"runtime"
	"time"

	"github.com/wercker/wercker/util"
)

// DockerOptions for our docker client
type DockerOptions struct {
	DockerHost      string
	DockerTLSVerify string
	DockerCertPath  string
	DockerDNS       []string
	DockerLocal     bool
}

func guessAndUpdateDockerOptions(opts *DockerOptions, e *util.Environment) {
	if opts.DockerHost != "" {
		return
	}

	logger := util.RootLogger().WithField("Logger", "docker")
	// f := &util.Formatter{opts.GlobalOptions.ShowColors}
	f := &util.Formatter{false}

	// Check the unix socket, default on linux
	// This will fail instantly so don't bother with the goroutine
	if runtime.GOOS == "linux" {
		unixSocket := "unix:///var/run/docker.sock"
		logger.Println(f.Info("No Docker host specified, checking", unixSocket))
		client, err := NewDockerClient(&DockerOptions{
			DockerHost: unixSocket,
		})
		if err == nil {
			_, err = client.Version()
			if err == nil {
				opts.DockerHost = unixSocket
				return
			}
		}
	}

	// Check the boot2docker port with default cert paths and such
	b2dCertPath := filepath.Join(e.Get("HOME"), ".boot2docker/certs/boot2docker-vm")
	b2dHost := "tcp://192.168.59.103:2376"

	logger.Printf(f.Info("No Docker host specified, checking for boot2docker", b2dHost))
	client, err := NewDockerClient(&DockerOptions{
		DockerHost:      b2dHost,
		DockerCertPath:  b2dCertPath,
		DockerTLSVerify: "1",
	})
	if err == nil {
		// This can take a long time if it isn't up, so toss it in a
		// goroutine so we can time it out
		result := make(chan bool)
		go func() {
			_, err = client.Version()
			if err == nil {
				result <- true
			} else {
				result <- false
			}
		}()
		select {
		case success := <-result:
			if success {
				opts.DockerHost = b2dHost
				opts.DockerCertPath = b2dCertPath
				opts.DockerTLSVerify = "1"
				return
			}
		case <-time.After(1 * time.Second):
		}
	}

	// Pick a default localhost port and hope for the best :/
	opts.DockerHost = "tcp://127.0.0.1:2375"
	logger.Println(f.Info("No Docker host found, falling back to default", opts.DockerHost))
}

// NewDockerOptions constructor
func NewDockerOptions(c util.Settings, e *util.Environment) (*DockerOptions, error) {
	dockerHost, _ := c.String("docker-host")
	dockerTLSVerify, _ := c.String("docker-tls-verify")
	dockerCertPath, _ := c.String("docker-cert-path")
	dockerDNS, _ := c.StringSlice("docker-dns")
	dockerLocal, _ := c.Bool("docker-local")

	speculativeOptions := &DockerOptions{
		DockerHost:      dockerHost,
		DockerTLSVerify: dockerTLSVerify,
		DockerCertPath:  dockerCertPath,
		DockerDNS:       dockerDNS,
		DockerLocal:     dockerLocal,
	}

	// We're going to try out a few settings and set DockerHost if
	// one of them works, it they don't we'll get a nice error when
	// requireDockerEndpoint triggers later on
	guessAndUpdateDockerOptions(speculativeOptions, e)
	return speculativeOptions, nil
}
