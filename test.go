package main

import (
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
)

// TestLogWriter writes our logs to the test output
type TestLogWriter struct {
	t *testing.T
}

// NewTestLogWriter constructor
func NewTestLogWriter(t *testing.T) *TestLogWriter {
	return &TestLogWriter{t: t}
}

// Write for io.Writer
func (l *TestLogWriter) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}

// TestLogFormatter removes the last newline character
type TestLogFormatter struct {
	*log.TextFormatter
}

// NewTestLogFormatter constructor
func NewTestLogFormatter() *TestLogFormatter {
	return &TestLogFormatter{&log.TextFormatter{}}
}

// Format like a text log but strip the last newline
func (f *TestLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	b, err := f.TextFormatter.Format(entry)
	if err == nil {
		b = b[:len(b)-1]
	}
	return b, err
}

func setup(t *testing.T) {
	writer := NewTestLogWriter(t)
	log.SetOutput(writer)
	log.SetFormatter(NewTestLogFormatter())
}

// Stepper lets use step and sync goroutines
type Stepper struct {
	stepper chan struct{}
}

// NewStepper constructor
func NewStepper() *Stepper {
	return &Stepper{stepper: make(chan struct{})}
}

// Wait until Step has been called
func (s *Stepper) Wait() {
	s.stepper <- struct{}{}
	<-s.stepper
}

// Step through a waiting goroutine with optional delay
func (s *Stepper) Step(delay ...int) {
	<-s.stepper
	for _, d := range delay {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	s.stepper <- struct{}{}
}
