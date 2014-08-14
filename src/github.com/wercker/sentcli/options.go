package main

import (
  "fmt"
  "path/filepath"
  "strings"
  "code.google.com/p/go-uuid/uuid"
  "github.com/codegangsta/cli"
)

// This represents a shell environment and is implemented as something
// like an OrderedMap
type Environment struct {
  Map map[string]string
  Order []string
}


// Usually called like: env := CreateEnvironment(os.Environ())
func CreateEnvironment(env []string) (*Environment) {
  var m map[string]string
  m = make(map[string]string)
  for _, e := range env {
    pair := strings.SplitN(e, "=", 2)
    m[pair[0]] = pair[1]
  }

  e := Environment{}
  e.Update(m)
  return &e
}


func (e *Environment) Update(m map[string]string) {
  if e.Map == nil {
    e.Map = make(map[string]string)
  }
  for k, v := range m {
    _, ok := e.Map[k]
    if !ok {
      e.Order = append(e.Order, k)
    }
    e.Map[k] = v
  }
}


// Export the environment as shell commands for use with Session.Send*
func (e *Environment) Export() []string {
  s := []string{}
  for _, key := range e.Order {
    s = append(s, fmt.Sprintf(`export %s="%s"`, key, e.Map[key]))
  }
  return s
}


type GlobalOptions struct {
  Env *Environment

  ProjectDir string
  StepDir string
  BuildDir string

  // Build ID for this operation
  BuildId string

  DockerEndpoint string

  // Base endpoint for wercker api
  WerckerEndpoint string

  // The read-write directory on the guest where all the work happens
  GuestRoot string

  // The read-only directory on the guest where volumes are mounted
  MntRoot string

  // The directory on the guest where reports will be written
  ReportRoot string


  // Source path relative to checkout root
  SourceDir string

  // Timeout if no response is received from a script in this many minutes
  NoResponseTimeout int

  // Timeout if the command doesn't complete in this many minutes
  CommandTimeout int
}


func CreateGlobalOptions(c *cli.Context, e []string) (*GlobalOptions, error) {
  env := CreateEnvironment(e)

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
    WerckerEndpoint:c.GlobalString("werckerEndpoint"),
    NoResponseTimeout:c.GlobalInt("noResponseTimeout"),
    ProjectDir:projectDir,
    SourceDir:c.GlobalString("sourceDir"),
    StepDir:stepDir,
    GuestRoot:c.GlobalString("guestRoot"),
    MntRoot:c.GlobalString("mntRoot"),
    ReportRoot:c.GlobalString("reportRoot"),
  }, nil
}



