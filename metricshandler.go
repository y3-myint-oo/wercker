package main

import (
	"errors"
	"github.com/chuckpreslar/emission"
	"github.com/inconshreveable/go-keen"
	"time"
)

// A MetricsEventHandler reporting to keen.io.
type MetricsEventHandler struct {
	keen *keen.Client
}

type MetricsApplicationPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OwnerName string `json:"ownerName"`
}

// MetricsPayload is the data we're sending to keen.
// Identical to legacy pool but we've renamed
// `sentinel` -> `core`.
type MetricsPayload struct {
	*MetricsApplicationPayload `json:"application"`
	BuildID                    string `json:"buildId"`
	Event                      string `json:"event"`
	StepName                   string `json:"stepName"`
	StepOrder                  int    `json:"stepOrder"`
	Timestamp                  int32  `json:"timestamp"`
	VCS                        string `json:"versionControl"`
	// Box                        string `json:"box"`       // todo
	// Core                       string `json:"core"`      // todo
	// JobID                      string `json:"jobId"`     // todo
	// StartedBy                  string `json:"startedBy"` // todo
}

// NewMetricsHandler will create a new NewMetricsHandler.
func NewMetricsHandler(opts *GlobalOptions) (*MetricsEventHandler, error) {

	if "" == opts.KeenProjectWriteKey {
		return nil, errors.New("No KeenProjectWriteKey specified")
	}

	if "" == opts.KeenProjectID {
		return nil, errors.New("No KeenProjectID specified")
	}

	// todo(yoshuawuyts): replace with `keen batch client` + regular flushes
	keenInstance := &keen.Client{
		ApiKey:       opts.KeenProjectWriteKey,
		ProjectToken: opts.KeenProjectID}

	return &MetricsEventHandler{keen: keenInstance}, nil
}

// BuildStepStarted responds to the BuildStepStarted event.
func (h *MetricsEventHandler) BuildStepStarted(event *BuildStepStartedArgs) {
	h.keen.AddEvent("build-events-ewok", &MetricsPayload{
		MetricsApplicationPayload: &MetricsApplicationPayload{
			ID:        event.Options.ApplicationID,
			Name:      event.Options.ApplicationName,
			OwnerName: event.Options.ApplicationOwnerName,
		},
		BuildID:   event.Options.ApplicationID,
		Event:     "buildStepStarted",
		StepName:  event.Step.Name,
		StepOrder: event.Order,
		Timestamp: int32(time.Now().Unix()),
		VCS:       "git",
		// Box:     event.Box,
		// Core:      "",
		// JobID:     "",
		// StartedBy: "",
	})
}

// BuildStepFinished responds to the BuildStepFinished event.
func (h *MetricsEventHandler) BuildStepFinished(event *BuildStepFinishedArgs) {
	h.keen.AddEvent("build-events-ewok", &MetricsPayload{
		MetricsApplicationPayload: &MetricsApplicationPayload{
			ID:        event.Options.ApplicationID,
			Name:      event.Options.ApplicationName,
			OwnerName: event.Options.ApplicationOwnerName,
		},
		//Box:       event.Box,
		BuildID:   event.Options.ApplicationID,
		Event:     "buildStepFinished",
		StepName:  event.Step.Name,
		StepOrder: event.Order,
		Timestamp: int32(time.Now().Unix()),
		VCS:       "git",
		// Box:     event.Box,
		// Core:      "",
		// JobID:     "",
		// StartedBy: "",
	})
}

// ListenTo will add eventhandlers to e.
func (h *MetricsEventHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(BuildStepStarted, h.BuildStepStarted)
	e.AddListener(BuildStepFinished, h.BuildStepFinished)
}
