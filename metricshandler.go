package main

import (
	"errors"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
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
		keen:     keenInstance,
		versions: versions,
		start:    make(map[string]time.Time),
	}, nil
}

// A MetricsEventHandler reporting to keen.io.
type MetricsEventHandler struct {
	keen                *keen.Client
	start               map[string]time.Time
	versions            *Versions
	numBuildSteps       int
	numBuildAfterSteps  int
	numDeploySteps      int
	numDeployAfterSteps int
}

func newMetricsKeenPayload(now time.Time) *metricsKeenPayload {
	return &metricsKeenPayload{Timestamp: now.Format(time.RFC3339)}
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

func newMetricStepPayload(step *Step) *metricStepPayload {
	return &metricStepPayload{
		Owner:      step.Owner,
		Name:       step.Name,
		Version:    step.Version,
		FullName:   fmt.Sprintf("%s/%s", step.Owner, step.Name),
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

	BuildID  string `json:"buildID,omitempty"`
	DeployID string `json:"deployID,omitempty"`

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

	Success   bool   `json:"success,omitempty"`
	Duration  int64  `json:"duration,omitempty"`
	StartedBy string `json:"startedBy,omitempty"`

	VCS                       string                     `json:"versionControl,omitempty"`
	MetricsApplicationPayload *metricsApplicationPayload `json:"application,omitempty"`
}

// BuildStepStarted responds to the BuildStepStarted event.
func (h *MetricsEventHandler) BuildStepStarted(args *BuildStepStartedArgs) {
	now := time.Now()
	h.start[args.Step.SafeID] = now

	pipelineName := getPipelineName(args.Options)
	collection := getCollection(args.Options)
	eventName := getStartEventName(args.Options)
	boxName, boxTag := getBoxDetails(args.Box)

	h.keen.AddEvent(collection, &MetricsPayload{
		Keen:      newMetricsKeenPayload(now),
		Timestamp: now.Unix(),
		BuildID:   args.Options.BuildID,
		DeployID:  args.Options.DeployID,
		Event:     eventName,
		StartedBy: args.Options.ApplicationStartedByName,
		MetricsApplicationPayload: &metricsApplicationPayload{
			ID:        args.Options.ApplicationID,
			Name:      args.Options.ApplicationName,
			OwnerName: args.Options.ApplicationOwnerName,
		},
		Step:                newMetricStepPayload(args.Step),
		StepName:            formatUniqueStepName(args.Step),
		StepOrder:           args.Order,
		SentCli:             h.versions,
		Stack:               5,
		BoxName:             boxName,
		BoxTag:              boxTag,
		NumBuildSteps:       h.numBuildSteps,
		NumBuildAfterSteps:  h.numBuildAfterSteps,
		NumDeploySteps:      h.numDeploySteps,
		NumDeployAfterSteps: h.numDeployAfterSteps,
		PipelineName:        pipelineName,
	})
}

// BuildStepFinished responds to the BuildStepFinished event.
func (h *MetricsEventHandler) BuildStepFinished(args *BuildStepFinishedArgs) {
	now := time.Now()

	pipelineName := getPipelineName(args.Options)
	collection := getCollection(args.Options)
	eventName := getFinishEventName(args.Options)
	boxName, boxTag := getBoxDetails(args.Box)

	var duration int64
	begin, ok := h.start[args.Step.SafeID]
	if ok {
		elapsed := now.Sub(begin)
		duration = elapsed.Nanoseconds() / 1000000
		delete(h.start, args.Step.SafeID)
	}

	h.keen.AddEvent(collection, &MetricsPayload{
		Keen:      newMetricsKeenPayload(now),
		Timestamp: now.Unix(),
		BuildID:   args.Options.BuildID,
		DeployID:  args.Options.DeployID,
		Duration:  duration,
		Event:     eventName,
		StartedBy: args.Options.ApplicationStartedByName,
		MetricsApplicationPayload: &metricsApplicationPayload{
			ID:        args.Options.ApplicationID,
			Name:      args.Options.ApplicationName,
			OwnerName: args.Options.ApplicationOwnerName,
		},
		Step:                newMetricStepPayload(args.Step),
		StepName:            formatUniqueStepName(args.Step),
		StepOrder:           args.Order,
		Success:             args.Successful,
		SentCli:             h.versions,
		Stack:               5,
		BoxName:             boxName,
		BoxTag:              boxTag,
		NumBuildSteps:       h.numBuildSteps,
		NumBuildAfterSteps:  h.numBuildAfterSteps,
		NumDeploySteps:      h.numDeploySteps,
		NumDeployAfterSteps: h.numDeployAfterSteps,
		PipelineName:        pipelineName,
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

// ListenTo will add eventhandlers to e.
func (h *MetricsEventHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(BuildStepStarted, h.BuildStepStarted)
	e.AddListener(BuildStepFinished, h.BuildStepFinished)
	e.AddListener(BuildStepsAdded, h.BuildStepsAdded)
}

func getPipelineName(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "build"
	}

	if options.DeployID != "" {
		return "deploy"
	}

	log.Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getCollection(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "build-events"
	}

	if options.DeployID != "" {
		return "deploy-events"
	}

	log.Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getStartEventName(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "buildStepStarted"
	}

	if options.DeployID != "" {
		return "deployStepStarted"
	}

	log.Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getFinishEventName(options *PipelineOptions) string {
	if options.BuildID != "" {
		return "buildStepFinished"
	}

	if options.DeployID != "" {
		return "deployStepFinished"
	}

	log.Panic("Metrics is only able to send metrics for builds or deploys")
	return ""
}

func getBoxDetails(box *Box) (boxName string, boxTag string) {
	if box == nil {
		return
	}

	return box.Name, box.tag
}

func formatUniqueStepName(step *Step) string {
	return fmt.Sprintf("%s/%s@%s", step.Owner, step.Name, step.Version)
}
