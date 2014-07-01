package main

import (
  "fmt"
  "gopkg.in/yaml.v1"
)


type Config struct {
  box *Box
  build *Build
}


