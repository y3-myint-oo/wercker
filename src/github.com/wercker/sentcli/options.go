package main

import (
  "strings"
  "github.com/codegangsta/cli"
)

type Environment map[string]string


// Usually called like: env := CreateEnvironment(os.Environ())
func CreateEnvironment(env []string) (*Environment) {
  var m Environment
  for _, e := range env {
    pair := strings.SplitN(e, "=", 2)
    m[pair[0]] = pair[1]
  }
  return &m
}



type GlobalOptions struct {
  Env *Environment

  projectDir string
  stepDir string
  buildDir string
  dockerEndpoint string

  // Source path relative to checkout root
  sourceDir string

  // Timeout if no response is received from a script in this many minutes
  noResponseTimeout int

  // Timeout if the command doesn't complete in this many minutes
  commandTimeout int
}

func CreateGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
  env := CreateEnvironment(e)

  return &GlobalOptions{Env:env}, nil
}
