package util

import (
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kardianos/osext"
)

// InstalledWithHomebrew tries to determine if the cli was installed with homebrew
// Returns true if installed and currently running, empty string if not installed
//
// Heuristic (bail if any step fails):
//   1. see if homebrew is installed
//   2. get installed packages from homebrew, look for `wercker-cli`
//   3. find symlink installed by homebrew, follow it to binary
//   4. get path of currently running executable
//   5. compare path with homebrew path
func InstalledWithHomebrew() (bool, error) {
	// Check if homebrew is installed
	cmd := exec.Command("which", "brew")
	cmdOut, err := cmd.Output()
	if err != nil {
		return false, err
	}

	if string(cmdOut) == "homebrew not found" {
		// Homebrew not installed on machine
		return false, nil
	}

	// Check if wercker was installed with homebrew
	cmd = exec.Command("brew", "list", "--versions")
	cmdOut, err = cmd.Output()
	if err != nil {
		return false, err
	}

	version := string(cmdOut)
	r := regexp.MustCompile(`(?m)^wercker-cli\s(.*)$`)
	matches := r.FindAllStringSubmatch(version, -1)

	if len(matches) == 0 {
		// wercker not installed in homebrew
		return false, nil
	}

	// resolve path to installed homebrew binary
	cmd = exec.Command("brew", "--prefix")
	cmdOut, err = cmd.Output()
	if err != nil {
		return false, err
	}

	brewPath := strings.Trim(string(cmdOut), "\n ")
	brewBinary := path.Join(brewPath, "/bin/wercker")
	brewBinPath, err := filepath.EvalSymlinks(brewBinary)

	if err != nil {
		// could not resolve symlink
		// could happen if {brew prefix}/bin/wercker doesn't exist
		return false, err
	}

	// get path of currently running executable
	curPath, err := osext.Executable()
	if err != nil {
		return false, err
	}

	if curPath == brewBinPath {
		// current executable path matches binary installed by homebrew
		return true, nil
	}

	// path didn't match -> this binary was not from homebrew
	return false, nil
}
