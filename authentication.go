package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
)

const werckerConfig = "config"
const werckerHome = ".wercker"

// Credentials holds credentials and auth scope to authenticate with api
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Scope    string `json:"oauthScope"`
}

// Response from authentication endpoint
type Response struct {
	Result  Result `json:"result"`
	Success bool   `json:"success"`
}

// Result holds the auth token
type Result struct {
	Token string `json:"token"`
}

func performLogin(url string) error {
	creds := Credentials{}
	creds.Username = readUsername()
	creds.Password = readPassword()
	creds.Scope = "cli"

	err := getAccessToken(creds, url)
	if err != nil {
		return err
	}
	return nil
}

func readUsername() (username string) {
	print("Username: ")
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to read username")
	}
	return input
}

func readPassword() (password string) {
	var oldState *term.State
	var input string
	oldState, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		log.WithField("Error", err).Debug("Unable to Set Raw Terminal")
	}

	print("Password: ")

	term.DisableEcho(os.Stdin.Fd(), oldState)

	_, err = fmt.Scanln(&input)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to read password")
	}

	term.RestoreTerminal(os.Stdin.Fd(), oldState)

	if input == "" {
		log.Println("Password required")
		os.Exit(1)
	}
	return input
}

// retrieves a basic access token from the wercker API
func getAccessToken(creds Credentials, url string) error {

	b, err := json.Marshal(creds)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to serialize credentials")
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		log.WithField("Error", err).Debug("Unable to post request to wercker API")
		return err
	}
	req.SetBasicAuth(creds.Username, creds.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.WithField("Error", err).Debug("Unable read from wercker API")
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to read response")
		return err
	}

	var response = Response{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to serialize response")
		return err

	}
	if response.Success == false {
		log.WithField("Error", err).Debug("Authentication failed")
		log.Println(err)
		return err
	}
	err = saveToken(response.Result.Token)
	if err != nil {
		return err
	}
	return nil
}

// saves a token to $HOME/.wercker/config
// creates directory when needed, overwrites file when it already exists
func saveToken(token string) error {
	homePath := os.Getenv("HOME")
	werckerHomePath := filepath.Join(homePath, werckerHome)
	werckerConfigPath := filepath.Join(werckerHomePath, werckerConfig)

	err := createWerckerDir(werckerHomePath)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to create wercker folder")
		return err
	}

	fp, err := os.Create(werckerConfigPath)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to create wercker config file")
		return err
	}
	defer fp.Close()
	_, err = io.WriteString(fp, token)
	if err != nil {
		log.WithField("Error", err).Debug("Unable to write wercker config")
		return err
	}
	log.Println("Stored wercker config in", werckerConfigPath)
	return nil
}

// helper function that creates the .wercker directory
func createWerckerDir(dir string) error {
	_, err := os.Stat(dir)
	if err != nil {
		err := os.Mkdir(dir, 0777)
		if err != nil {
			return err
		}
		return err
	}
	return err
}
