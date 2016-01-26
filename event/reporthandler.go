//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package event

import (
	"fmt"

	"github.com/wercker/reporter"
	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
)

// NewReportHandler will create a new ReportHandler.
func NewReportHandler(werckerHost, token string) (*ReportHandler, error) {
	r, err := reporter.New(werckerHost, token)
	if err != nil {
		return nil, err
	}

	writers := make(map[string]*reporter.LogWriter)
	logger := util.RootLogger().WithField("Logger", "Reporter")
	h := &ReportHandler{
		reporter: r,
		writers:  writers,
		logger:   logger,
	}
	return h, nil
}

func mapBuildSteps(counter *util.Counter, phase string, steps ...core.Step) []*reporter.NewStep {
	buffer := make([]*reporter.NewStep, len(steps))
	for i, s := range steps {
		buffer[i] = &reporter.NewStep{
			DisplayName: s.DisplayName(),
			Name:        s.Name(),
			Order:       counter.Increment(),
			Phase:       phase,
		}
	}
	return buffer
}

// A ReportHandler reports all events to the wercker-api.
type ReportHandler struct {
	reporter *reporter.Reporter
	writers  map[string]*reporter.LogWriter
	logger   *util.LogEntry
}

// BuildStepStarted will handle the BuildStepStarted event.
func (h *ReportHandler) BuildStepStarted(args *core.BuildStepStartedArgs) {
	opts := &reporter.PipelineStepStartedArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		StepName: args.Step.Name(),
		Order:    args.Order,
	}

	h.reporter.PipelineStepStarted(opts)
}

func (h *ReportHandler) generateKey(pipelineID, stepName string, order int) string {
	return fmt.Sprintf("%s_%s_%d", pipelineID, stepName, order)
}

func (h *ReportHandler) flushLogs(pipelineID, stepName string, order int) error {
	key := h.generateKey(pipelineID, stepName, order)

	if writer, ok := h.writers[key]; ok {
		return writer.Flush()
	}

	return nil
}

// BuildStepFinished will handle the BuildStepFinished event.
func (h *ReportHandler) BuildStepFinished(args *core.BuildStepFinishedArgs) {
	h.flushLogs(args.Options.PipelineID, args.Step.Name(), args.Order)

	opts := &reporter.PipelineStepFinishedArgs{
		BuildID:               args.Options.BuildID,
		DeployID:              args.Options.DeployID,
		StepName:              args.Step.Name(),
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
func (h *ReportHandler) BuildStepsAdded(args *core.BuildStepsAddedArgs) {
	stepCounter := &util.Counter{Current: 3}
	steps := mapBuildSteps(stepCounter, "mainSteps", args.Steps...)

	if args.StoreStep != nil {
		storeStep := mapBuildSteps(stepCounter, "mainSteps", args.StoreStep)
		steps = append(steps, storeStep...)
	}

	afterSteps := mapBuildSteps(stepCounter, "finalSteps", args.AfterSteps...)
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
func (h *ReportHandler) getStepOutputWriter(args *core.LogsArgs) (*reporter.LogWriter, error) {
	key := h.generateKey(args.Options.PipelineID, args.Step.Name(), args.Order)

	opts := &reporter.PipelineStepReporterArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		StepName: args.Step.Name(),
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
func (h *ReportHandler) Logs(args *core.LogsArgs) {
	if args.Hidden {
		return
	}
	if args.Step == nil {
		return
	}

	w, err := h.getStepOutputWriter(args)
	if err != nil {
		h.logger.WithField("Error", err).Error("Unable to create step output writer")
		return
	}
	w.Write([]byte(args.Logs))
}

// BuildFinished will handle the BuildFinished event.
func (h *ReportHandler) BuildFinished(args *core.BuildFinishedArgs) {
	opts := &reporter.PipelineFinishedArgs{
		BuildID:  args.Options.BuildID,
		DeployID: args.Options.DeployID,
		Result:   args.Result,
	}
	h.reporter.PipelineFinished(opts)
}

// FullPipelineFinished closes current writers, making sure they have flushed
// their logs.
func (h *ReportHandler) FullPipelineFinished(args *core.FullPipelineFinishedArgs) {
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
func (h *ReportHandler) ListenTo(e *core.NormalizedEmitter) {
	e.AddListener(core.BuildFinished, h.BuildFinished)
	e.AddListener(core.BuildStepsAdded, h.BuildStepsAdded)
	e.AddListener(core.BuildStepStarted, h.BuildStepStarted)
	e.AddListener(core.BuildStepFinished, h.BuildStepFinished)
	e.AddListener(core.FullPipelineFinished, h.FullPipelineFinished)
	e.AddListener(core.Logs, h.Logs)
}
