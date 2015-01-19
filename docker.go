package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"path"
)

// NewDockerClient based on options and env
func NewDockerClient(options *GlobalOptions) (*docker.Client, error) {
	dockerHost := options.DockerHost
	tlsVerify, ok := options.Env.Map["DOCKER_TLS_VERIFY"]
	if ok && tlsVerify == "1" {
		// We're using TLS, let's locate our certs and such
		// boot2docker puts its certs at...
		dockerCertPath := options.Env.Map["DOCKER_CERT_PATH"]

		// TODO(termie): maybe fast-fail if these don't exist?
		cert := path.Join(dockerCertPath, fmt.Sprintf("cert.pem"))
		ca := path.Join(dockerCertPath, fmt.Sprintf("ca.pem"))
		key := path.Join(dockerCertPath, fmt.Sprintf("key.pem"))
		log.Println("key path", key)
		return docker.NewVersionnedTLSClient(dockerHost, cert, key, ca, "")
	}
	return docker.NewClient(dockerHost)
}
