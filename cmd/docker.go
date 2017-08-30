package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/codegangsta/cli"
	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/kr/pty"
	"github.com/wercker/wercker/core"
)

func ensureWerckerCredentials(c *cli.Context, opts *core.WerckerDockerOptions) {
	fmt.Println("IMPLEMENT ME!!!")
	dockerConfig := config.LoadDefaultConfigFile(os.Stderr)
	werckerAuth, hasAuth := dockerConfig.AuthConfigs[opts.WerckerContainerRegistry.String()]
	if hasAuth {
		fmt.Printf("WE HAVE WERCKER AUTH!!!!\n%+v\n", werckerAuth)
	} else {
		dockerConfig.AuthConfigs[opts.WerckerContainerRegistry.String()] = types.AuthConfig{
			Username: "token",
			Password: opts.AuthToken,
		}
		err := dockerConfig.Save()
		if err != nil {
			fmt.Printf("Couldn't save docker config: %v", err)
		}
		fmt.Println(":-((((((((")

	}
}

func runDocker(args []string) error {
	dockerCmd := exec.Command("docker", args...)
	// run using a pseudo-terminal so that we get the nice docker output :)
	outFile, err := pty.Start(dockerCmd)
	if err != nil {
		return err
	}
	// Stream output of the command to stdout
	io.Copy(os.Stdout, outFile)
	return nil
}
