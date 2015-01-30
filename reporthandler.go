package main

import (
	"fmt"
	"io"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/wercker/reporter"
)

const (
	stepCounterOffset = 2
)

// NewReportHandler will create a new ReportHandler.
func NewReportHandler(werckerHost, token string) (*ReportHandler, error) {
	r, err := reporter.New(werckerHost, token)
	if err != nil {
		return nil, err
	}

	writers := make(map[string]io.WriteCloser)

	h := &ReportHandler{
		reporter: r,
		writers:  writers,
	}
	return h, nil
}

func mapBuildSteps(counter *Counter, phase string, steps ...*Step) []*reporter.NewStep {
	buffer := make([]*reporter.NewStep, len(steps))
	for i, s := range steps {
		buffer[i] = &reporter.NewStep{
			DisplayName: s.DisplayName,
			Name:        s.Name,
			Order:       counter.Increment(),
			Phase:       phase,
		}
	}
	return buffer
}

// A ReportHandler logs all events to the wercker-api.
type ReportHandler struct {
	reporter        *reporter.Reporter
	writers         map[string]io.WriteCloser
	currentStep     *Step
	currentOrder    int
	currentBuildID  string
	currentDeployID string
}

// BuildStepStarted will handle the BuildStepStarted event.
func (h *ReportHandler) BuildStepStarted(args *BuildStepStartedArgs) {
	h.currentStep = args.Step
	h.currentOrder = args.Order
	h.currentBuildID = args.Options.BuildID
	h.currentDeployID = args.Options.DeployID

	opts := &reporter.PipelineStepStartedArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		StepName: args.Step.Name,
		Order:    args.Order,
	}

	h.reporter.PipelineStepStarted(opts)
}

// BuildStepFinished will handle the BuildStepFinished event.
func (h *ReportHandler) BuildStepFinished(args *BuildStepFinishedArgs) {
	h.currentStep = nil
	h.currentOrder = -1
	h.currentBuildID = ""

	opts := &reporter.PipelineStepFinishedArgs{
		BuildID:               args.Options.BuildID,
		DeployID:              args.Options.DeployID,
		StepName:              args.Step.Name,
		Order:                 args.Order,
		Successful:            args.Successful,
		ArtifactURL:           args.ArtifactURL,
		PackageURL:            args.PackageURL,
		Message:               args.Message,
		WerckerYamlContents:   args.WerckerYamlContents,
		WerckerConfigContents: args.WerckerYamlContents,
	}

	h.reporter.PipelineStepFinished(opts)
}

// BuildStepsAdded will handle the BuildStepsAdded event.
func (h *ReportHandler) BuildStepsAdded(args *BuildStepsAddedArgs) {
	stepCounter := &Counter{Current: 3}
	steps := mapBuildSteps(stepCounter, "mainSteps", args.Steps...)
	storeStep := mapBuildSteps(stepCounter, "mainSteps", args.StoreStep)
	afterSteps := mapBuildSteps(stepCounter, "finalSteps", args.AfterSteps...)
	steps = append(steps, storeStep...)
	steps = append(steps, afterSteps...)

	opts := &reporter.NewPipelineStepsArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		Steps:    steps,
	}

	h.reporter.NewPipelineSteps(opts)
}

// getStepOutputWriter will check h.writers for a writer for the step, otherwise
// it will create a new one.
func (h *ReportHandler) getStepOutputWriter(args *LogsArgs) (io.WriteCloser, error) {

	key := fmt.Sprintf("%s_%s_%d", args.Options.PipelineID, args.Step.Name, args.Order)

	opts := &reporter.PipelineStepReporterArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		StepName: args.Step.Name,
		Order:    args.Order,
	}

	writer, ok := h.writers[key]
	if !ok {
		w, err := h.reporter.PipelineStepReporter(opts)
		if err != nil {
			return nil, err
		}
		h.writers[key] = w
		writer = w
	}

	return writer, nil
}

// Logs will handle the Logs event.
func (h *ReportHandler) Logs(args *LogsArgs) {
	if args.Hidden {
		return
	}

	step := h.currentStep
	order := h.currentOrder

	args.Step = step
	args.Order = order

	if step == nil {
		return
	}

	w, err := h.getStepOutputWriter(args)
	if err != nil {
		log.WithField("Error", err).Error("Unable to create step output writer")
		return
	}
	w.Write([]byte(args.Logs))
}

// BuildFinished will handle the BuildFinished event. This will call h.Close.
func (h *ReportHandler) BuildFinished(args *BuildFinishedArgs) {
	opts := &reporter.PipelineFinishedArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		Result:   args.Result,
	}
	h.reporter.PipelineFinished(opts)

	h.Close()
}

// Close will call close on any log writers that have been created.
func (h *ReportHandler) Close() error {
	for _, w := range h.writers {
		w.Close()
	}

	return nil
}

// ListenTo will add eventhandlers to e.
func (h *ReportHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(BuildFinished, h.BuildFinished)
	e.AddListener(BuildStepsAdded, h.BuildStepsAdded)
	e.AddListener(BuildStepStarted, h.BuildStepStarted)
	e.AddListener(BuildStepFinished, h.BuildStepFinished)
	e.AddListener(Logs, h.Logs)
}
