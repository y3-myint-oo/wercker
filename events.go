package main

import (
	"github.com/chuckpreslar/emission"
)

const (
	// Logs is the event when sentcli generate logs
	Logs = "Logs"

	// BuildStarted is the event when sentcli has started a build.
	BuildStarted = "BuildStarted"

	// BuildFinished occures when a pipeline finishes the main phase. It is
	// possible that after-steps are run after this event.
	BuildFinished = "BuildFinished"

	// BuildStepsAdded is the event when sentcli has parsed the wercker.yml and
	// has valdiated that the steps exist.
	BuildStepsAdded = "BuildStepsAdded"

	// BuildStepStarted is the event when sentcli has started a new buildstep.
	BuildStepStarted = "BuildStepStarted"

	// BuildStepFinished is the event when sentcli has finished a buildstep.
	BuildStepFinished = "BuildStepFinished"

	// FullPipelineFinished occurs when a pipeline finishes all it's steps,
	// included after-steps.
	FullPipelineFinished = "FullPipelineFinished"
)

// BuildStartedArgs contains the args associated with the "BuildStarted" event.
type BuildStartedArgs struct {
	Options *GlobalOptions
}

// BuildFinishedArgs contains the args associated with the "BuildFinished"
// event.
type BuildFinishedArgs struct {
	Options *GlobalOptions
	Result  string
}

// LogsArgs contains the args associated with the "Logs" event.
type LogsArgs struct {
	Box     string
	Build   Pipeline
	Options *GlobalOptions
	Order   int
	Step    *Step
	Logs    string
	Stream  string
	Hidden  bool
}

// BuildStepsAddedArgs contains the args associated with the
// "BuildStepsAdded" event.
type BuildStepsAddedArgs struct {
	Box        string
	Build      Pipeline
	Options    *GlobalOptions
	Steps      []*Step
	StoreStep  *Step
	AfterSteps []*Step
}

// BuildStepStartedArgs contains the args associated with the
// "BuildStepStarted" event.
type BuildStepStartedArgs struct {
	Box     string
	Build   Pipeline
	Options *GlobalOptions
	Order   int
	Step    *Step
}

// BuildStepFinishedArgs contains the args associated with the
// "BuildStepFinished" event.
type BuildStepFinishedArgs struct {
	Box         string
	Build       Pipeline
	Options     *GlobalOptions
	Order       int
	Step        *Step
	Successful  bool
	Message     string
	ArtifactURL string
	// Only applicable to the store step
	PackageURL string
	// Only applicable to the setup environment step
	WerckerYamlContents string
}

// FullPipelineFinishedArgs contains the args associated with the
// "FullPipelineFinished" event.
type FullPipelineFinishedArgs struct {
	Options             *GlobalOptions
	MainSuccessful      bool
	RanAfterSteps       bool
	AfterStepSuccessful bool
}

// emitter contains the singleton emitter.
var emitter = emission.NewEmitter()

// GetEmitter will return a singleton event emitter.
func GetEmitter() *emission.Emitter {
	return emitter
}
