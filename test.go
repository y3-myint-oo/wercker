package main

import (
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
)

type TestLogWriter struct {
	t *testing.T
}

func NewTestLogWriter(t *testing.T) *TestLogWriter {
	return &TestLogWriter{t: t}
}

func (l *TestLogWriter) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}

type TestLogFormatter struct {
	*log.TextFormatter
}

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

type Stepper struct {
	stepper chan struct{}
}

func NewStepper() *Stepper {
	return &Stepper{stepper: make(chan struct{})}
}

func (s *Stepper) Wait() {
	s.stepper <- struct{}{}
	<-s.stepper
}

func (s *Stepper) Step(delay ...int) {
	<-s.stepper
	for _, d := range delay {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	s.stepper <- struct{}{}
}
