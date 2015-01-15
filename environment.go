package main

import (
	"fmt"
	"strings"
)

// Environment represents a shell environment and is implemented as something
// like an OrderedMap
type Environment struct {
	Map   map[string]string
	Order []string
}

// NewEnvironment fills up an Environment from a []string
// Usually called like: env := NewEnvironment(os.Environ())
func NewEnvironment(env []string) *Environment {
	e := Environment{}
	for _, keyvalue := range env {
		pair := strings.SplitN(keyvalue, "=", 2)
		e.Add(pair[0], pair[1])
	}

	return &e
}

// Update adds new elements to the Environment data structure.
func (e *Environment) Update(a [][]string) {
	for _, keyvalue := range a {
		e.Add(keyvalue[0], keyvalue[1])
	}
}

// Add an individual record.
func (e *Environment) Add(key, value string) {
	if e.Map == nil {
		e.Map = make(map[string]string)
	}
	if _, ok := e.Map[key]; !ok {
		e.Order = append(e.Order, key)
	}
	e.Map[key] = value
}

// Export the environment as shell commands for use with Session.Send*
func (e *Environment) Export() []string {
	s := []string{}
	for _, key := range e.Order {
		s = append(s, fmt.Sprintf(`export %s="%s"`, key, e.Map[key]))
	}
	return s
}

// Ordered returns a [][]string of the items in the env.
func (e *Environment) Ordered() [][]string {
	a := [][]string{}
	for _, k := range e.Order {
		a = append(a, []string{k, e.Map[k]})
	}
	return a
}

var mirroredEnv = [...]string{
	"WERCKER_GIT_DOMAIN",
	"WERCKER_GIT_OWNER",
	"WERCKER_GIT_REPOSITORY",
	"WERCKER_GIT_BRANCH",
	"WERCKER_GIT_COMMIT",
	"WERCKER_STARTED_BY",
	"WERCKER_MAIN_PIPELINE_STARTED",
	// "WERCKER_APPLICATION_ID",
	// "WERCKER_APPLICATION_NAME",
	// "WERCKER_APPLICATION_OWNER_NAME",
}

// Collect passthru variables from the project
func (e *Environment) getPassthru() [][]string {
	a := [][]string{}
	for key, value := range e.Map {
		if strings.HasPrefix(key, "X_") {
			a = append(a, []string{strings.TrimPrefix(key, "X_"), value})
		}
	}
	return a
}

func (e *Environment) getMirror() [][]string {
	a := [][]string{}
	for _, key := range mirroredEnv {
		value, ok := e.Map[key]
		if ok {
			a = append(a, []string{key, value})
		}
	}
	return a
}
