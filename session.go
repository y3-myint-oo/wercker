package main

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/fsouza/go-dockerclient"
	// "os"
	"strings"
	"time"
)

// Receiver is for reading from our session
type Receiver struct {
	queue chan string
}

// NewReceiver returns a new channel-based io.Writer
func NewReceiver(queue chan string) *Receiver {
	return &Receiver{queue: queue}
}

// Write writes to a channel
func (r *Receiver) Write(p []byte) (int, error) {
	buf := bytes.NewBuffer(p)
	r.queue <- buf.String()
	return buf.Len(), nil
}

// Sender is for sending to our session
type Sender struct {
	queue chan string
}

// NewSender gives us a new channel-based io.Reader
func NewSender(queue chan string) *Sender {
	return &Sender{queue: queue}
}

// Read reads from a channel
func (s *Sender) Read(p []byte) (int, error) {
	send := <-s.queue
	i := copy(p, []byte(send))
	return i, nil
}

// Session is our way to interact with the container
type Session struct {
	options     *GlobalOptions
	e           *emission.Emitter
	client      *DockerClient
	ContainerID string
	logsHidden  bool
	send        chan string
	recv        chan string
	exit        chan int
}

// NewSession returns a new interactive session to a container.
func NewSession(options *GlobalOptions, containerID string) (*Session, error) {
	client, err := NewDockerClient(options)
	if err != nil {
		return nil, err
	}
	return &Session{options: options, e: GetEmitter(), client: client, ContainerID: containerID, logsHidden: false}, nil
}

// Attach us to our container and set up read and write queues
func (s *Session) Attach() error {
	started := make(chan struct{})

	recv := make(chan string)
	outputStream := NewReceiver(recv)
	s.recv = recv

	send := make(chan string)
	inputStream := NewSender(send)
	s.send = send

	exit := make(chan int)
	s.exit = exit

	opts := docker.AttachToContainerOptions{
		Container:    s.ContainerID,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		Success:      started,
		InputStream:  inputStream,
		ErrorStream:  outputStream,
		OutputStream: outputStream,
		RawTerminal:  false,
	}

	go func() {
		status, err := s.client.WaitContainer(s.ContainerID)
		if err != nil {
			log.Errorln("Error waiting", err)
		}
		log.Debugln("Container finished with status code:", status)
		s.exit <- status
		close(s.exit)
	}()

	go func() {
		err := s.client.AttachToContainer(opts)
		if err != nil {
			log.Panicln(err)
		}
	}()

	// Wait for attach
	<-started
	started <- struct{}{}
	return nil
}

// HideLogs will emit Logs with args.Hidden set to true
func (s *Session) HideLogs() {
	s.logsHidden = true
}

// ShowLogs will emit Logs with args.Hidden set to false
func (s *Session) ShowLogs() {
	s.logsHidden = false
}

// Send an array of commands.
func (s *Session) Send(forceHidden bool, commands ...string) {
	for i := range commands {
		command := commands[i] + "\n"
		hidden := s.logsHidden
		if forceHidden {
			hidden = forceHidden
		}

		s.e.Emit(Logs, &LogsArgs{
			Hidden: hidden,
			Stream: "stdin",
			Logs:   command,
		})
		s.send <- command
	}
}

// CommandResult exists so that we can make a channel of them
type CommandResult struct {
	exitCode int
	recv     []string
	err      error
}

// SendChecked sends commands, waits for them to complete and returns the
// exit status and output
func (s *Session) SendChecked(commands ...string) (int, []string, error) {
	var exitCode int
	rand := uuid.NewRandom().String()
	check := false
	recv := []string{}

	s.Send(false, commands...)
	s.Send(true, fmt.Sprintf("echo %s $?", rand))

	c := make(chan CommandResult, 1)
	checkFunc := func() (int, []string, error) {
		// BUG(termie): This is relatively naive and will break if the messages
		// returned aren't complete lines, if this becomes a problem we'll have
		// to buffer it.
		for check != true {
			line := ""
			select {
			case myline, ok := <-s.recv:
				if !ok {
					return 1, recv, nil
				}
				line = myline
			// Exited "expectedly"
			case status := <-s.exit:
				return status, recv, nil
			// Timed out
			case <-time.After(time.Duration(s.options.NoResponseTimeout) * time.Minute):
				//close(s.ch)
				return 1, recv, fmt.Errorf("Timeout: no response seen for %d minutes", s.options.NoResponseTimeout)
			}

			if strings.HasPrefix(line, rand) {
				check = true
				_, err := fmt.Sscanf(line, "%s %d\n", &rand, &exitCode)
				if err != nil {
					s.e.Emit(Logs, &LogsArgs{
						Hidden: true,
						Logs:   line,
						Stream: "stdout",
					})
					return exitCode, recv, err
				}
			} else {
				s.e.Emit(Logs, &LogsArgs{
					Hidden: s.logsHidden,
					Logs:   line,
					Stream: "stdout",
				})
				recv = append(recv, line)
			}
		}
		return exitCode, recv, nil
	}

	// Timeout for the whole command
	go func() {
		exitCode, recv, err := checkFunc()
		c <- CommandResult{exitCode, recv, err}
	}()

	select {
	case r := <-c:
		return r.exitCode, r.recv, r.err
	case <-time.After(time.Duration(s.options.CommandTimeout) * time.Minute):
		//close(c)
		return 1, []string{}, fmt.Errorf("Command timed out after %d minutes", s.options.CommandTimeout)
	}
}
