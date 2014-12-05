package main

import (
	"github.com/chuckpreslar/emission"
)

const (
	// Logs is the event when sentcli generate logs
	Logs = "Logs"

	// BuildStepsAdded is the event when sentcli has parsed the wercker.yml and
	// has valdiated that the steps exist.
	BuildStepsAdded = "BuildStepsAdded"

	// BuildStepStarted is the event when sentcli has started a new buildstep.
	BuildStepStarted = "BuildStepStarted"

	// BuildStepFinished is the event when sentcli has finished a buildstep.
	BuildStepFinished = "BuildStepFinished"
)

// LogsArgs contains the args associated with the "Logs" event.
type LogsArgs struct {
	Box     string
	Build   *Build
	Options *GlobalOptions
	Order   int
	Step    *Step
	Logs    string
	Stream  string
}

// BuildStepsAddedArgs contains the args associated with the
// "BuildStepsAdded" event.
type BuildStepsAddedArgs struct {
	Box     string
	Build   *Build
	Options *GlobalOptions
	Steps   []*Step
}

// BuildStepStartedArgs contains the args associated with the
// "BuildStepStarted" event.
type BuildStepStartedArgs struct {
	Box     string
	Build   *Build
	Options *GlobalOptions
	Order   int
	Step    *Step
}

// BuildStepFinishedArgs contains the args associated with the
// "BuildStepFinished" event.
type BuildStepFinishedArgs struct {
	Box        string
	Build      *Build
	Options    *GlobalOptions
	Order      int
	Step       *Step
	Successful bool
}

// emitter contains the singleton emitter.
var emitter = emission.NewEmitter()

// GetEmitter will return a singleton event emitter.
func GetEmitter() *emission.Emitter {
	return emitter
}
