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

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/wercker/sentcli/util"
)

// Credentials holds credentials and auth scope to authenticate with api
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Scope    string `json:"oauthScope"`
}

// Response from authentication endpoint
type Response struct {
	Result  AuthResult `json:"result"`
	Success bool       `json:"success"`
}

// AuthResult holds the auth token
type AuthResult struct {
	Token string `json:"token"`
}

var authLogger = util.RootLogger().WithField("Logger", "Authentication")

func readUsername() string {
	print("Username: ")
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read username")
	}
	return input
}

func readPassword() string {
	var oldState *term.State
	var input string
	oldState, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to Set Raw Terminal")
	}

	print("Password: ")

	term.DisableEcho(os.Stdin.Fd(), oldState)
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	_, err = fmt.Scanln(&input)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read password")
	}

	if input == "" {
		authLogger.Println("Password required")
		os.Exit(1)
	}
	print("\n")
	return input
}

// retrieves a basic access token from the wercker API
func getAccessToken(username, password, url string) (string, error) {
	creds := Credentials{
		Username: username,
		Password: password,
		Scope:    "cli",
	}

	b, err := json.Marshal(creds)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to serialize credentials")
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to post request to wercker API")
		return "", err
	}

	req.SetBasicAuth(creds.Username, creds.Password)
	req.Header.Set("Content-Type", "application/json")
	AddRequestHeaders(req)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable read from wercker API")
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read response")
		return "", err
	}

	var response = &Response{}
	err = json.Unmarshal(body, response)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to serialize response")
		return "", err

	}
	if response.Success == false {
		err := errors.New("Invalid credentials")
		authLogger.WithField("Error", err).Debug("Authentication failed")
		return "", err
	}

	return strings.TrimSpace(response.Result.Token), nil
}

// creates directory when needed, overwrites file when it already exists
func saveToken(path, token string) error {
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to create auth store folder")
		return err
	}

	return ioutil.WriteFile(path, []byte(token), 0600)
}

func removeToken(tokenStore string) error {
	err := os.Remove(tokenStore)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
