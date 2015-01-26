package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/wercker/reporter"
	"io"
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

func mapBuildSteps(offset int, steps ...*Step) []*reporter.NewStep {
	buffer := make([]*reporter.NewStep, len(steps))
	for i, s := range steps {
		buffer[i] = &reporter.NewStep{
			DisplayName: s.DisplayName,
			Name:        s.Name,
			Order:       offset + i,
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

	h.reporter.StepStarted(args.Options.BuildID, args.Step.Name, args.Order)
}

// BuildStepFinished will handle the BuildStepFinished event.
func (h *ReportHandler) BuildStepFinished(args *BuildStepFinishedArgs) {
	h.currentStep = nil
	h.currentOrder = -1
	h.currentBuildID = ""

	h.reporter.StepFinished(
		args.Options.BuildID,
		args.Step.Name,
		args.Order,
		args.Successful,
		args.ArtifactURL,
		args.PackageURL,
		args.Message,
		args.WerckerYamlContents,
	)
}

// BuildStepsAdded will handle the BuildStepsAdded event.
func (h *ReportHandler) BuildStepsAdded(args *BuildStepsAddedArgs) {
	steps := mapBuildSteps(stepCounterOffset, args.Steps...)
	storeStep := mapBuildSteps(len(args.Steps)+stepCounterOffset+1, args.StoreStep)

	steps = append(steps, storeStep...)

	h.reporter.ReportNewSteps(args.Options.BuildID, steps)
}

// getStepOutputWriter will check h.writers for a writer for the step, otherwise
// it will create a new one.
func (h *ReportHandler) getStepOutputWriter(buildID, stepName string, order int) (io.WriteCloser, error) {
	key := fmt.Sprintf("%s_%s_%d", buildID, stepName, order)
	writer, ok := h.writers[key]

	if !ok {
		w, err := h.reporter.StepOutput(buildID, stepName, order)
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
	buildID := h.currentBuildID

	if step == nil {
		return
	}

	w, err := h.getStepOutputWriter(buildID, step.Name, order)
	if err != nil {
		log.WithField("Error", err).Error("Unable to create step output writer")
		return
	}
	w.Write([]byte(args.Logs))
}

// BuildFinished will handle the BuildFinished event. This will call h.Close.
func (h *ReportHandler) BuildFinished(args *BuildFinishedArgs) {
	h.reporter.BuildFinished(args.Options.BuildID, args.Result)

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
