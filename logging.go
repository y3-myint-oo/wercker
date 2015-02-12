package main

import (
	"github.com/Sirupsen/logrus"
)

// Logger is a wrapper for logrus so that we don't have to keep referring
// to its types everywhere and can add helpers
type Logger struct {
	*logrus.Logger
}

type LogFields logrus.Fields

func NewLogger() *Logger {
	return &Logger{logrus.New()}
}

func (l *Logger) SetLevel(level string) {
	l.Level, _ = logrus.ParseLevel(level)
}

func (l *Logger) WithFields(fields LogFields) *LogEntry {
	return &LogEntry{l.Logger.WithFields(logrus.Fields(fields))}
}

func (l *Logger) WithField(key string, value interface{}) *LogEntry {
	return &LogEntry{l.Logger.WithField(key, value)}
}

type LogEntry struct {
	*logrus.Entry
}

func (e *LogEntry) WithField(key string, value interface{}) *LogEntry {
	return &LogEntry{e.Entry.WithField(key, value)}
}

func (e *LogEntry) WithFields(fields LogFields) *LogEntry {
	return &LogEntry{e.Entry.WithFields(logrus.Fields(fields))}
}

// Our root logger
var rootLogger = NewLogger()
