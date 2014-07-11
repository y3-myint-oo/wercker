package main

import (
  "path"
)






type Build struct {
  Env *Environment
  Steps []*Step
  options *GlobalOptions
}


func (b *RawBuild) ToBuild(options *GlobalOptions) (*Build, error) {
  var steps []*Step
  var build Build

  for _, rawStep := range b.RawSteps {
    step, err := rawStep.ToStep(&build, options)
    if err != nil {
      return nil, err
    }
    steps = append(steps, step)
  }

  build.options = options
  build.Steps = steps

  return &build, nil
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
