package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

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
	Options *PipelineOptions
}

// BuildFinishedArgs contains the args associated with the "BuildFinished"
// event.
type BuildFinishedArgs struct {
	Box     *Box
	Options *PipelineOptions
	Result  string
}

// LogsArgs contains the args associated with the "Logs" event.
type LogsArgs struct {
	Build   Pipeline
	Options *PipelineOptions
	Order   int
	Step    *Step
	Logs    string
	Stream  string
	Hidden  bool
}

// BuildStepsAddedArgs contains the args associated with the
// "BuildStepsAdded" event.
type BuildStepsAddedArgs struct {
	Build      Pipeline
	Options    *PipelineOptions
	Steps      []*Step
	StoreStep  *Step
	AfterSteps []*Step
}

// BuildStepStartedArgs contains the args associated with the
// "BuildStepStarted" event.
type BuildStepStartedArgs struct {
	Options *PipelineOptions
	Box     *Box
	Build   Pipeline
	Order   int
	Step    *Step
}

// BuildStepFinishedArgs contains the args associated with the
// "BuildStepFinished" event.
type BuildStepFinishedArgs struct {
	Options     *PipelineOptions
	Box         *Box
	Build       Pipeline
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
	Options             *PipelineOptions
	MainSuccessful      bool
	RanAfterSteps       bool
	AfterStepSuccessful bool
}

type DebugHandler struct {
	murder *LogEntry
}

func NewDebugHandler() *DebugHandler {
	murder := rootLogger.WithField("Logger", "Events")
	return &DebugHandler{murder: murder}
}

// dumpEvent prints out some debug info about an event
func (h *DebugHandler) dumpEvent(event interface{}, indent ...string) {
	indent = append(indent, "  ")
	s := reflect.ValueOf(event).Elem()

	typeOfT := s.Type()
	names := []string{}
	for i := 0; i < s.NumField(); i++ {
		// f := s.Field(i)
		fieldName := typeOfT.Field(i).Name
		if fieldName != "Env" {
			names = append(names, fieldName)
		}
	}
	sort.Strings(names)

	for _, name := range names {

		r := reflect.ValueOf(event)
		f := reflect.Indirect(r).FieldByName(name)
		if name == "Options" {
			continue
		}
		if name[:1] == strings.ToLower(name[:1]) {
			// Not exported, skip it
			h.murder.Debugln(fmt.Sprintf("%s%s %s = %v", strings.Join(indent, ""), name, f.Type(), "<not exported>"))
			continue
		}
		if name == "Box" || name == "Step" {
			h.murder.Debugln(fmt.Sprintf("%s%s %s", strings.Join(indent, ""), name, f.Type()))
			if !f.IsNil() {
				h.dumpEvent(f.Interface(), indent...)
			}
		} else {
			h.murder.Debugln(fmt.Sprintf("%s%s %s = %v", strings.Join(indent, ""), name, f.Type(), f.Interface()))
		}
	}
}

func (h *DebugHandler) Handler(name string) func(interface{}) {
	return func(event interface{}) {
		h.murder.Debugln("Event: ", name)
		h.dumpEvent(event)
	}
}

func (h *DebugHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(BuildStarted, h.Handler("BuildStarted"))
	e.AddListener(BuildFinished, h.Handler("BuildFinished"))
	e.AddListener(BuildStepsAdded, h.Handler("BuildStepsAdded"))
	e.AddListener(BuildStepStarted, h.Handler("BuildStepStarted"))
	e.AddListener(BuildStepFinished, h.Handler("BuildStepFinished"))
	e.AddListener(FullPipelineFinished, h.Handler("FullPipelineFinished"))
}

// emitter contains the singleton emitter.
var emitter = emission.NewEmitter()

// GetEmitter will return a singleton event emitter.
func GetEmitter() *emission.Emitter {
	return emitter
}
