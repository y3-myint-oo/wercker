package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/wercker/reporter"
)

// NewLiteralLogHandler will create a new LiteralLogHandler.
func NewLiteralLogHandler() (*LiteralLogHandler, error) {
	logger := log.New()

	logger.Formatter = &reporter.LiteralFormatter{}
	logger.Level = log.InfoLevel

	return &LiteralLogHandler{l: logger}, nil
}

// A LiteralLogHandler logs all events using Logrus.
type LiteralLogHandler struct {
	l *log.Logger
}

// Logs will handle the Logs event.
func (h *LiteralLogHandler) Logs(args *LogsArgs) {
	if !args.Hidden {
		h.l.Print(args.Logs)
	}
}

// ListenTo will add eventhandlers to e.
func (h *LiteralLogHandler) ListenTo(e *emission.Emitter) {
	e.AddListener(Logs, h.Logs)
}
