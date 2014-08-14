package main

import (
  "fmt"
  "path"
  "code.google.com/p/go-uuid/uuid"
)


type Build struct {
  Env *Environment
  Steps []*Step
  Id string
  options *GlobalOptions
}


var MirroredEnv = [...]string{"WERCKER_GIT_DOMAIN",
                              "WERCKER_GIT_OWNER",
                              "WERCKER_GIT_REPOSITORY",
                              "WERCKER_GIT_BRANCH",
                              "WERCKER_GIT_COMMIT",
                              "WERCKER_STARTED_BY",
                              "WERCKER_MAIN_PIPELINE_STARTED",
                              "WERCKER_APPLICATION_URL",
                              "WERCKER_APPLICATION_ID",
                              "WERCKER_APPLICATION_NAME",
                              "WERCKER_APPLICATION_OWNER_NAME"}


func (b *RawBuild) ToBuild(options *GlobalOptions) (*Build, error) {
  var steps []*Step
  var build Build

  // Start with the secret step, wercker-init that runs before everything
  rawStepData := RawStepData{}
  initStep, err := CreateStep("wercker-init", rawStepData, &build, options)
  if err != nil {
    return nil, err
  }
  steps = append(steps, initStep)

  for _, extraRawStep := range b.RawSteps {
    rawStep, err := NormalizeStep(extraRawStep)
    if err != nil {
      return nil, err
    }
    step, err := rawStep.ToStep(&build, options)
    if err != nil {
      return nil, err
    }
    steps = append(steps, step)
  }

  build.options = options
  build.Steps = steps

  id, ok := build.options.Env.Map["WERCKER_BUILD_ID"]
  if !ok {
    id = uuid.NewRandom().String()
  }
  build.Id = id

  build.InitEnv()

  return &build, nil
}


func (b *Build) InitEnv() {
  b.Env = &Environment{}
  // TODO(termie): deal with PASSTHRU args from the user here
  b.Env.Update(b.getMirrorEnv())

  // Add all of our basic env vars
  m := map[string]string {
    "WERCKER": "true",
    "BUILD": "true",
    "CI": "true",
    "WERCKER_BUILD_ID": b.Id,
    "WERCKER_ROOT": b.GuestPath("source"),
    "WERCKER_SOURCE_DIR": b.GuestPath("source", b.options.SourceDir),
    "WERCKER_CACHE_DIR": "/cache",
    "WERCKER_OUTPUT_DIR": b.GuestPath("output"),
    "WERCKER_PIPELINE_DIR": b.GuestPath(),
    "WERCKER_REPORT_DIR": b.GuestPath("report"),
    "TERM": "xterm-256color",
  }
  b.Env.Update(m)
}


func (b *Build) getMirrorEnv() map[string]string {
  var m = make(map[string]string)
  for _, key := range MirroredEnv {
    value, ok := b.options.Env.Map[key]
    if ok {
      m[key] = value
    }
  }
  return m
}


func (b *Build) SourcePath() string {
  return b.GuestPath("source", b.options.SourceDir)
}


func (b *Build) HostPath(s ...string) (string) {
  hostPath := path.Join(b.options.BuildDir, b.options.BuildId)
  for _, v := range s {
    hostPath = path.Join(hostPath, v)
  }
  return hostPath
}


func (b *Build) GuestPath(s ...string) (string) {
  guestPath := b.options.GuestRoot
  for _, v := range s {
    guestPath = path.Join(guestPath, v)
  }
  return guestPath
}


func (b *Build) MntPath(s ...string) (string) {
  mntPath := b.options.MntRoot
  for _, v := range s {
    mntPath = path.Join(mntPath, v)
  }
  return mntPath
}


func (b *Build) ReportPath(s ...string) (string) {
  reportPath := b.options.ReportRoot
  for _, v := range s {
    reportPath = path.Join(reportPath, v)
  }
  return reportPath
}


func (b *Build) SetupGuest(sess *Session) error {
  // sess.Start("setup guest")

  // Make sure our guest path exists
  exit, recv, err := sess.SendChecked(fmt.Sprintf(`mkdir "%s"`, b.GuestPath()))

  // And the cache path
  exit, recv, err = sess.SendChecked(fmt.Sprintf(`mkdir "%s"`, "/cache"))

  // Copy the source dir to the guest path
  exit, recv, err = sess.SendChecked(fmt.Sprintf(`cp -r "%s" "%s"`, b.MntPath("source"), b.GuestPath("source")))

  fmt.Println(exit, recv, err)

  // exit, recv, err = sess.SendChecked(
  // sess.Commit()
  return nil
}
