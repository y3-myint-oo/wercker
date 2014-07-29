package main

import (
  "fmt"
  "path/filepath"
  "strings"
  "code.google.com/p/go-uuid/uuid"
  "github.com/codegangsta/cli"
)

type Environment map[string]string


// Usually called like: env := CreateEnvironment(os.Environ())
func CreateEnvironment(env []string) (*Environment) {
  m := Environment{}
  for _, e := range env {
    pair := strings.SplitN(e, "=", 2)
    m[pair[0]] = pair[1]
  }
  return &m
}



type GlobalOptions struct {
  Env *Environment

  ProjectDir string
  StepDir string
  BuildDir string

  // Build ID for this operation
  BuildId string

  DockerEndpoint string

  // The read-write directory on the guest where all the work happens
  GuestRoot string

  // The read-only directory on the guest where volumes are mounted
  MntRoot string


  // Source path relative to checkout root
  SourceDir string

  // Timeout if no response is received from a script in this many minutes
  NoResponseTimeout int

  // Timeout if the command doesn't complete in this many minutes
  CommandTimeout int
}

func CreateGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
  env := CreateEnvironment(e)

  fmt.Println("buildDir", c.GlobalString("buildDir"))

  buildDir, _ := filepath.Abs(c.GlobalString("buildDir"))
  projectDir, _ := filepath.Abs(c.GlobalString("projectDir"))
  stepDir, _ := filepath.Abs(c.GlobalString("stepDir"))
  buildId := c.GlobalString("buildId")
  if buildId == "" {
    buildId = uuid.NewRandom().String()
  }

  return &GlobalOptions{
    Env:env,
    BuildDir:buildDir,
    BuildId:buildId,
    CommandTimeout:c.GlobalInt("commandTimeout"),
    DockerEndpoint:c.GlobalString("dockerEndpoint"),
    NoResponseTimeout:c.GlobalInt("noResponseTimeout"),
    ProjectDir:projectDir,
    SourceDir:c.GlobalString("sourceDir"),
    StepDir:stepDir,
    GuestRoot:c.GlobalString("guestRoot"),
    MntRoot:c.GlobalString("mntRoot"),
  }, nil
}



