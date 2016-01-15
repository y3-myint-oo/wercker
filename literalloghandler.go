package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/wercker/reporter"
	"github.com/wercker/sentcli/util"
)

// NewLiteralLogHandler will create a new LiteralLogHandler.
func NewLiteralLogHandler(options *PipelineOptions) (*LiteralLogHandler, error) {
	var logger *util.Logger

	if options.Debug {
		logger = util.RootLogger()
	} else {
		logger = util.NewLogger()
		logger.Formatter = &reporter.LiteralFormatter{}
		logger.Level = log.InfoLevel
	}

	return &LiteralLogHandler{l: logger, options: options}, nil
}

// A LiteralLogHandler logs all events using Logrus.
type LiteralLogHandler struct {
	l       *util.Logger
	options *PipelineOptions
}

// Logs will handle the Logs event.
func (h *LiteralLogHandler) Logs(args *LogsArgs) {
	if args.Stream == "" {
		args.Stream = "stdout"
	}
	if h.options.Debug {
		shown := "[x]"
		if args.Hidden {
			shown = "[ ]"
		}
		h.l.WithFields(util.LogFields{
			"Logger": "Literal",
			"Hidden": args.Hidden,
			"Stream": args.Stream,
		}).Printf("%s %6s %q", shown, args.Stream, args.Logs)
	} else if h.shouldPrintLog(args) {
		h.l.Print(args.Logs)
	}
}

func (h *LiteralLogHandler) shouldPrintLog(args *LogsArgs) bool {
	if args.Hidden {
		return false
	}

	// Do not show stdin stream is verbose is false
	if args.Stream == "stdin" && !h.options.Verbose {
		return false
	}

	return true
}

// ListenTo will add eventhandlers to e.
func (h *LiteralLogHandler) ListenTo(e *NormalizedEmitter) {
	e.AddListener(Logs, h.Logs)
}
