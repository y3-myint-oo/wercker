package main

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/wercker/reporter"
)

// NewLiteralLogHandler will create a new LiteralLogHandler.
func NewLiteralLogHandler(options *PipelineOptions) (*LiteralLogHandler, error) {
	logger := log.New()

	if options.Debug {
		logger.Formatter = new(log.TextFormatter)
	} else {
		logger.Formatter = &reporter.LiteralFormatter{}
	}
	logger.Level = log.InfoLevel

	return &LiteralLogHandler{l: logger, options: options}, nil
}

// A LiteralLogHandler logs all events using Logrus.
type LiteralLogHandler struct {
	l       *log.Logger
	options *PipelineOptions
}

// Logs will handle the Logs event.
func (h *LiteralLogHandler) Logs(args *LogsArgs) {
	if h.options.Debug {
		streamInfo := fmt.Sprintf("%6s: ", args.Stream)
		shown := "[x] "
		if args.Hidden {
			shown = "[ ] "
		}
		h.l.Print(shown, streamInfo, fmt.Sprintf("%q", args.Logs))
	} else if !args.Hidden {
		h.l.Print(args.Logs)
	}
}

// ListenTo will add eventhandlers to e.
func (h *LiteralLogHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(Logs, h.Logs)
}
