package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

// NewLogHandler will create a new LogHandler.
func NewLogHandler() (*LogHandler, error) {
	return &LogHandler{}, nil
}

// A LogHandler logs all events using Logrus.
type LogHandler struct{}

// BuildStepStarted will handle the BuildStepStarted event.
func (h *LogHandler) BuildStepStarted(args *BuildStepStartedArgs) {
	log.WithFields(log.Fields{
	// "BuildID":  args.BuildID,
	// "StepName": args.StepName,
	// "Order":    args.Order,
	}).Debug("BuildStep started")
}

// BuildStepFinished will handle the BuildStepFinished event.
func (h *LogHandler) BuildStepFinished(args *BuildStepFinishedArgs) {
	log.WithFields(log.Fields{
	// "BuildID":    args.BuildID,
	// "StepName":   args.StepName,
	// "Order":      args.Order,
	// "Successful": args.Successful,
	}).Debug("BuildStep finished")
}

// BuildStepsAdded will handle the BuildStepsAdded event.
func (h *LogHandler) BuildStepsAdded(args *BuildStepsAddedArgs) {
	log.WithFields(log.Fields{
	// "BuildID": args.BuildID,
	// "Step":    args.Steps,
	}).Debug("BuildSteps added")
}

// ListenTo will add eventhandlers to e.
func (h *LogHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(BuildStepStarted, h.BuildStepStarted)
	e.AddListener(BuildStepFinished, h.BuildStepFinished)
	e.AddListener(BuildStepsAdded, h.BuildStepsAdded)
}
