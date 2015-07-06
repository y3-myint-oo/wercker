package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/inconshreveable/go-keen"
)

// NewMetricsHandler will create a new NewMetricsHandler.
func NewMetricsHandler(opts *PipelineOptions) (*MetricsEventHandler, error) {
	if "" == opts.KeenProjectWriteKey {
		return nil, errors.New("No KeenProjectWriteKey specified")
	}

	if "" == opts.KeenProjectID {
		return nil, errors.New("No KeenProjectID specified")
	}

	keenInstance := &keen.Client{
		WriteKey:  opts.KeenProjectWriteKey,
		ProjectID: opts.KeenProjectID,
	}

	versions := GetVersions()

	return &MetricsEventHandler{
		keen:      keenInstance,
		versions:  versions,
		startStep: make(map[string]time.Time),
	}, nil
}

// A MetricsEventHandler reporting to keen.io.
type MetricsEventHandler struct {
	keen                *keen.Client
	startStep           map[string]time.Time
	startBuild          time.Time
	versions            *Versions
	numBuildSteps       int
	numBuildAfterSteps  int
	numDeploySteps      int
	numDeployAfterSteps int
}

// ListenTo will add eventhandlers to e.
func (h *MetricsEventHandler) ListenTo(e *NormalizedEmitter) {
	e.AddListener(BuildStepStarted, h.BuildStepStarted)
	e.AddListener(BuildStepFinished, h.BuildStepFinished)
	e.AddListener(BuildStepsAdded, h.BuildStepsAdded)

	e.AddListener(BuildStarted, h.BuildStarted)
	e.AddListener(BuildFinished, h.BuildFinished)
}

func newMetricsKeenPayload(now time.Time) *metricsKeenPayload {
	return &metricsKeenPayload{Timestamp: now.Format(time.RFC3339)}
}

// BuildStarted responds to the BuildStarted event.
func (h *MetricsEventHandler) BuildStarted(args *BuildStartedArgs) {
	now := time.Now()

	h.startBuild = now

	p := &MetricsPayload{}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		now:       now,
		eventName: "buildStarted",
	})
}

// BuildFinished responds to the BuildFinished event.
func (h *MetricsEventHandler) BuildFinished(args *BuildFinishedArgs) {
	now := time.Now()

	elapsed := now.Sub(h.startBuild)
	duration := int64(elapsed.Seconds())

	success := args.Result == "passed"

	p := &MetricsPayload{
		Duration: &duration,
		Success:  &success,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "buildFinished",
	})
}

// BuildStepStarted responds to the BuildStepStarted event.
func (h *MetricsEventHandler) BuildStepStarted(args *BuildStepStartedArgs) {
	now := time.Now()

	h.startStep[args.Step.SafeID()] = now

	p := &MetricsPayload{
		Step:      newMetricStepPayload(args.Step),
		StepName:  formatUniqueStepName(args.Step),
		StepOrder: args.Order,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "buildStepStarted",
	})
}

// BuildStepFinished responds to the BuildStepFinished event.
func (h *MetricsEventHandler) BuildStepFinished(args *BuildStepFinishedArgs) {
	now := time.Now()

	var duration int64
	begin, ok := h.startStep[args.Step.SafeID()]
	if ok {
		elapsed := now.Sub(begin)
		duration = int64(elapsed.Seconds())
		delete(h.startStep, args.Step.SafeID())
	}

	p := &MetricsPayload{
		Step:      newMetricStepPayload(args.Step),
		StepName:  formatUniqueStepName(args.Step),
		StepOrder: args.Order,
		Duration:  &duration,
		Success:   &args.Successful,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "buildStepFinished",
	})
}

// BuildStepsAdded handles the BuildStepsAdded event.
func (h *MetricsEventHandler) BuildStepsAdded(args *BuildStepsAddedArgs) {
	if args.Options.BuildID != "" {
		h.numBuildSteps = len(args.Steps)
		h.numBuildAfterSteps = len(args.AfterSteps)
	} else if args.Options.DeployID != "" {
		h.numDeploySteps = len(args.Steps)
		h.numDeployAfterSteps = len(args.AfterSteps)
	}
}

type sendPayloadArgs struct {
	p         *MetricsPayload
	options   *PipelineOptions
	box       *Box
	now       time.Time
	eventName string
}

func (h *MetricsEventHandler) sendPayload(args *sendPayloadArgs) {
	collection := getCollection(args.options)
	pipelineName := getPipelineName(args.options)
	boxName, boxTag := getBoxDetails(args.box)

	p := args.p

	p.Keen = newMetricsKeenPayload(args.now)
	p.Timestamp = args.now.Unix()
	p.BuildID = args.options.BuildID
	p.DeployID = args.options.DeployID
	p.Event = args.eventName
	p.StartedBy = args.options.ApplicationStartedByName
	p.MetricsApplicationPayload = &metricsApplicationPayload{
		ID:        args.options.ApplicationID,
		Name:      args.options.ApplicationName,
		OwnerName: args.options.ApplicationOwnerName,
	}
	p.SentCli = h.versions
	p.Stack = 5
	p.NumBuildSteps = h.numBuildSteps
	p.NumBuildAfterSteps = h.numBuildAfterSteps
	p.NumDeploySteps = h.numDeploySteps
	p.NumDeployAfterSteps = h.numDeployAfterSteps
	p.PipelineName = pipelineName
	p.BoxName = boxName
	p.BoxTag = boxTag

	h.keen.AddEvent(collection, p)
}

type metricsKeenPayload struct {
	Timestamp string `json:"timestamp"`
}

// metricsApplicationPayload is the app data we're reporting
// to keen. Part of MetricsPayload.
type metricsApplicationPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OwnerName string `json:"ownerName"`
}

func newMetricStepPayload(step Step) *metricStepPayload {
	return &metricStepPayload{
		Owner:      step.Owner(),
		Name:       step.Name(),
		Version:    step.Version(),
		FullName:   fmt.Sprintf("%s/%s", step.Owner(), step.Name()),
		UniqueName: formatUniqueStepName(step),
	}
}

type metricStepPayload struct {
	Owner   string `json:"owner,omitempty"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`

	FullName   string `json:"uniqueName,omitempty"` // Contains owner/name
	UniqueName string `json:"fullName,omitempty"`   // Contains owner/name@version
}

// MetricsPayload is the data we're sending to keen.
type MetricsPayload struct {
	Keen         *metricsKeenPayload `json:"keen"`
	Timestamp    int64               `json:"timestamp"`
	Event        string              `json:"event"`
	Stack        int                 `json:"stack,omitempty"`
	SentCli      *Versions           `json:"sentcli,omitempty"`
	Grappler     *Versions           `json:"grappler,omitempty"`
	PipelineName string              `json:"pipelineName,omitempty"`

	BuildID  string `json:"buildId,omitempty"`
	DeployID string `json:"deployId,omitempty"`

	NumBuildSteps       int `json:"numBuildSteps,omitempty"`
	NumBuildAfterSteps  int `json:"numBuildAfterSteps,omitempty"`
	NumDeploySteps      int `json:"numDeploySteps,omitempty"`
	NumDeployAfterSteps int `json:"numDeployAfterSteps,omitempty"`

	BoxName string `json:"box,omitempty"`
	BoxTag  string `json:"boxTag,omitempty"`

	Step *metricStepPayload `json:"step,omitempty"`

	// Required for backwards compatibility:
	StepName  string `json:"stepName,omitempty"` // <- owner/name@version
	StepOrder int    `json:"stepOrder,omitempty"`

	Success   *bool  `json:"success,omitempty"`
	Duration  *int64 `json:"duration,omitempty"`
	StartedBy string `json:"startedBy,omitempty"`

	VCS                       string                     `json:"versionControl,omitempty"`
	MetricsApplicationPayload *metricsApplicationPayload `json:"application,omitempty"`
}

func getPipelineName(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "build"
	}

	if options.DeployID != "" {
		return "deploy"
	}

	rootLogger.WithField("Logger", "Metrics").Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getCollection(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "build-events"
	}

	if options.DeployID != "" {
		return "deploy-events"
	}

	rootLogger.WithField("Logger", "Metrics").Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getBoxDetails(box *Box) (boxName string, boxTag string) {
	if box == nil {
		return
	}

	return box.Name, box.tag
}

func formatUniqueStepName(step Step) string {
	return fmt.Sprintf("%s/%s@%s", step.Owner(), step.Name(), step.Version())
}
