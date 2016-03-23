package dockerauth

import (
	"errors"
	"net/url"
	"strings"

	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/util"
)

type CheckAccessOptions struct {
	Username      string
	Password      string
	AwsSecretKey  string
	AwsAccessKey  string
	AwsRegion     string
	AwsStrictAuth bool
	AwsID         string
	AwsRegistryID string
	Registry      string
}

const (
	DockerRegistryV2 = "https://registry-1.docker.io"
)

var ErrNoAuthenticator = errors.New("Unable to make authenticator for this registry")

func NormalizeRegistry(address string) string {
	logger := util.RootLogger().WithField("Logger", "Docker")
	if address == "" {
		logger.Debugln("No registry address provided, using https://registry.hub.docker.com")
		return "https://registry.hub.docker.com/v1/"
	}

	parsed, err := url.Parse(address)
	if err != nil {
		logger.Errorln("Registry address is invalid, this will probably fail:", address)
		return address
	}
	if parsed.Scheme != "https" {
		logger.Warnln("Registry address is expected to begin with 'https://', forcing it to use https")
		parsed.Scheme = "https"
		address = parsed.String()
	}
	if strings.HasSuffix(address, "/") {
		address = address[:len(address)-1]
	}

	// since its not painfully obvious that someone specified a docker hub registry v2 url we need to check explitcly and return it
	if address == DockerRegistryV2 {
		ret := DockerRegistryV2 + "/v2/"
		return ret
	}
	parts := strings.Split(address, "/")
	possiblyAPIVersionStr := parts[len(parts)-1]

	// send them a v1 registry if they don't specify
	if possiblyAPIVersionStr != "v1" && possiblyAPIVersionStr != "v2" {
		newParts := append(parts, "v1")
		address = strings.Join(newParts, "/")
	}
	return address + "/"
}

func GetRegistryAuthenticator(opts CheckAccessOptions) (auth.Authenticator, error) {
	//calls to this function probably already have normalized registries, but call it again jic
	reg := NormalizeRegistry(opts.Registry)

	//try to get domain and check if you're pushing to ecr, so you can make an ecr auth checker
	if opts.AwsAccessKey != "" && opts.AwsSecretKey != "" && opts.AwsRegion != "" && opts.AwsRegistryID != "" {
		return auth.NewAmazonAuth(opts.AwsRegistryID, opts.AwsAccessKey, opts.AwsSecretKey, opts.AwsRegion, opts.AwsStrictAuth), nil
	}

	parts := strings.Split(reg, "/")
	apiVersion := parts[len(parts)-2]
	if apiVersion == "v1" {
		registryURL, err := url.Parse(reg)
		if err != nil {
			return nil, err
		}
		return auth.NewDockerAuthV1(registryURL, opts.Username, opts.Password), nil
	} else if apiVersion == "v2" {
		registryURL, err := url.Parse(reg)
		if err != nil {
			return nil, err
		}
		return auth.NewDockerAuth(registryURL, opts.Username, opts.Password), nil
	}
	return nil, ErrNoAuthenticator
}
