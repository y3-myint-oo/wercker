package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
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

type Transport interface {
	Attach(context.Context, io.Reader, io.Writer, io.Writer) (context.Context, error)
}

type DockerTransport struct {
	options     *PipelineOptions
	e           *emission.Emitter
	client      *DockerClient
	containerID string
}

func NewDockerTransport(options *PipelineOptions, containerID string) (Transport, error) {
	client, err := NewDockerClient(options.DockerOptions)
	if err != nil {
		return nil, err
	}
	return &DockerTransport{options: options, e: GetEmitter(), client: client, containerID: containerID}, nil
}

// Attach the given reader and writers to the transport, return a context
// that will be closed when the transport dies
func (t *DockerTransport) Attach(sessionCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (context.Context, error) {
	log.Debugln("Attaching to container: ", t.containerID)
	started := make(chan struct{})
	transportCtx, cancel := context.WithCancel(sessionCtx)

	// exit := make(chan int)
	// t.exit = exit

	opts := docker.AttachToContainerOptions{
		Container:    t.containerID,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		Logs:         true,
		Success:      started,
		InputStream:  stdin,
		ErrorStream:  stdout,
		OutputStream: stderr,
		RawTerminal:  false,
	}

	go func() {
		defer cancel()
		err := t.client.AttachToContainer(opts)
		if err != nil {
			log.Panicln(err)
		}
	}()

	// Wait for attach
	<-started
	go func() {
		defer cancel()
		status, err := t.client.WaitContainer(t.containerID)
		if err != nil {
			log.Errorln("Error waiting", err)
		}
		log.Warnln("Container finished with status code:", status, t.containerID)
		// t.exit <- status
		// close(t.exit)
	}()
	started <- struct{}{}
	return transportCtx, nil
}

// DockerSession is our way to interact with the docker container
type Session struct {
	options    *PipelineOptions
	e          *emission.Emitter
	transport  Transport
	logsHidden bool
	send       chan string
	recv       chan string
	exit       chan int
}

// NewSession returns a new interactive session to a container.
func NewSession(options *PipelineOptions, transport Transport) *Session {
	return &Session{
		options:    options,
		e:          GetEmitter(),
		transport:  transport,
		logsHidden: false,
	}
}

// Attach us to our container and set up read and write queues.
// Returns a context object for the transport so we can propagate cancels
// on errors and closed connections.
func (s *Session) Attach(runnerCtx context.Context) (context.Context, error) {
	recv := make(chan string)
	outputStream := NewReceiver(recv)
	s.recv = recv

	send := make(chan string)
	inputStream := NewSender(send)
	s.send = send

	// We treat the transport context as the session context everywhere
	return s.transport.Attach(runnerCtx, inputStream, outputStream, outputStream)
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
func (s *Session) Send(sessionCtx context.Context, forceHidden bool, commands ...string) error {
	for i := range commands {
		command := commands[i] + "\n"

		select {
		case <-sessionCtx.Done():
			log.Errorln("Session finished before sending command:", command)
			// log.Errorln("Err: ", sessionCtx.Err())
			return sessionCtx.Err()
		case s.send <- command:
			hidden := s.logsHidden
			if forceHidden {
				hidden = forceHidden
			}

			s.e.Emit(Logs, &LogsArgs{
				Options: s.options,
				Hidden:  hidden,
				Stream:  "stdin",
				Logs:    command,
			})
		}
	}
	return nil
}

// CommandResult exists so that we can make a channel of them
type CommandResult struct {
	exitCode int
	recv     []string
	err      error
}

// SendChecked sends commands, waits for them to complete and returns the
// exit status and output
func (s *Session) SendChecked(sessionCtx context.Context, commands ...string) (int, []string, error) {
	var exitCode int
	rand := uuid.NewRandom().String()
	check := false
	recv := []string{}

	err := s.Send(sessionCtx, false, commands...)
	if err != nil {
		return 1, []string{}, err
	}
	s.Send(sessionCtx, true, fmt.Sprintf("echo %s $?", rand))
	if err != nil {
		return 1, []string{}, err
	}

	sendCtx, cancelSend := context.WithTimeout(sessionCtx, time.Duration(s.options.CommandTimeout)*time.Minute)

	c := make(chan CommandResult, 1)
	checkFunc := func() (int, []string, error) {
		// BUG(termie): This is relatively naive and will break if the messages
		// returned aren't complete lines, if this becomes a problem we'll have
		// to buffer it.
		// Cancel the timeout if we finish before it gets called
		defer cancelSend()
		for check != true {
			checkCtx, cancelCheck := context.WithTimeout(sendCtx, time.Duration(s.options.NoResponseTimeout)*time.Minute)
			line := ""
			select {
			case myline := <-s.recv:
				// We got data so cancel the no-response timeout
				cancelCheck()
				line = myline
			case <-checkCtx.Done():
				log.Errorln("Session finished before receiving output.")
				return 1, recv, checkCtx.Err()
			// Exited "expectedly"
			case status := <-s.exit:
				return status, recv, nil
				// Timed out
				// case <-time.After(time.Duration(s.options.NoResponseTimeout) * time.Minute):
				//   //close(s.ch)
				//   return 1, recv, fmt.Errorf("Timeout: no response seen for %d minutes", s.options.NoResponseTimeout)
			}

			if strings.HasPrefix(line, rand) {
				check = true
				_, err := fmt.Sscanf(line, "%s %d\n", &rand, &exitCode)
				if err != nil {
					s.e.Emit(Logs, &LogsArgs{
						Options: s.options,
						Hidden:  true,
						Logs:    line,
						Stream:  "stdout",
					})
					return exitCode, recv, err
				}
			} else {
				s.e.Emit(Logs, &LogsArgs{
					Options: s.options,
					Hidden:  s.logsHidden,
					Logs:    line,
					Stream:  "stdout",
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

	r := <-c
	return r.exitCode, r.recv, r.err

	// select {
	// case r := <-c:
	//   return r.exitCode, r.recv, r.err
	// case <-time.After(time.Duration(s.options.CommandTimeout) * time.Minute):
	//   //close(c)
	//   return 1, []string{}, fmt.Errorf("Command timed out after %d minutes", s.options.CommandTimeout)
	// }
}
