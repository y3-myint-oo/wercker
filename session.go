package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"

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
	logger      *LogEntry
}

func NewDockerTransport(options *PipelineOptions, containerID string) (Transport, error) {
	client, err := NewDockerClient(options.DockerOptions)
	if err != nil {
		return nil, err
	}
	logger := rootLogger.WithField("Logger", "DockerTransport")
	return &DockerTransport{options: options, e: GetEmitter(), client: client, containerID: containerID, logger: logger}, nil
}

// Attach the given reader and writers to the transport, return a context
// that will be closed when the transport dies
func (t *DockerTransport) Attach(sessionCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (context.Context, error) {
	t.logger.Debugln("Attaching to container: ", t.containerID)
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
		Logs:         false,
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
			t.logger.Panicln(err)
		}
	}()

	// Wait for attach
	<-started
	go func() {
		defer cancel()
		status, err := t.client.WaitContainer(t.containerID)
		if err != nil {
			t.logger.Errorln("Error waiting", err)
		}
		t.logger.Warnln("Container finished with status code:", status, t.containerID)
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
	logger     *LogEntry
}

// NewSession returns a new interactive session to a container.
func NewSession(options *PipelineOptions, transport Transport) *Session {
	logger := rootLogger.WithField("Logger", "Session")
	return &Session{
		options:    options,
		e:          GetEmitter(),
		transport:  transport,
		logsHidden: false,
		logger:     logger,
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
	// Do a quick initial check whether we have a valid session first
	select {
	case <-sessionCtx.Done():
		s.logger.Errorln("Session finished before sending commands:", commands)
		return sessionCtx.Err()
	// Wait because if both cases are available golang will pick one randomly
	case <-time.After(1 * time.Millisecond):
		// Pass
	}

	for i := range commands {
		command := commands[i] + "\n"
		select {
		case <-sessionCtx.Done():
			s.logger.Errorln("Session finished before sending command:", command)
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

var randomSentinel = func() string {
	return uuid.NewRandom().String()
}

// CommandResult exists so that we can make a channel of them
type CommandResult struct {
	exit int
	recv []string
	err  error
}

func checkLine(line, sentinel string) (bool, int) {
	if !strings.HasPrefix(line, sentinel) {
		return false, -999
	}
	var rand string
	var exit int
	_, err := fmt.Sscanf(line, "%s %d\n", &rand, &exit)
	if err != nil {
		return false, -999
	}
	return true, exit
}

// SendChecked sends commands, waits for them to complete and returns the
// exit status and output
// Ways to know a command is done:
//	[ ] We received the sentinel echo
//  [ ] The container has exited and we've exhausted the incoming data
//  [ ] The session has closed and we've exhaused the incoming data
//  [ ] The command has timed out
// Ways for a command to be successful:
//  [ ] We received the sentinel echo with exit code 0
func (s *Session) SendChecked(sessionCtx context.Context, commands ...string) (int, []string, error) {
	recv := []string{}
	sentinel := randomSentinel()
	var err error

	sendCtx, _ := context.WithTimeout(sessionCtx, time.Duration(s.options.CommandTimeout)*time.Millisecond)

	commandComplete := make(chan CommandResult)

	// Signal channel to tell the reader to stop reading, this lets us
	// keep it reading for a small amount of time after we know something
	// has gone wrong, otherwise it misses some error messages.
	stopReading := make(chan struct{}, 1)

	// This is our main waiter, it will get an exit code, an error or a timeout
	// and then complete the command, anything
	exitChan := make(chan int)
	errChan := make(chan error)
	go func() {
		select {
		// We got an exit code because we got our sentinel, let's skiddaddle
		case exit := <-exitChan:
			err = nil
			if exit != 0 {
				err = fmt.Errorf("Command exited with exit code: %d", exit)
			}
			commandComplete <- CommandResult{exit: exit, recv: recv, err: err}
		case err = <-errChan:
			commandComplete <- CommandResult{exit: -1, recv: recv, err: err}
		case <-sendCtx.Done():
			// We timed out or something closed, try to read in the rest of the data
			// over the next 100 milliseconds and then return
			<-time.After(time.Duration(100) * time.Millisecond)
			// close(stopReading)
			stopReading <- struct{}{}
			commandComplete <- CommandResult{exit: -1, recv: recv, err: sendCtx.Err()}
		}
	}()

	// If we don't get a response in a certain amount of time, timeout
	noResponseTimeout := make(chan struct{})
	go func() {
		for {
			select {
			case <-noResponseTimeout:
				continue
			case <-time.After(time.Duration(s.options.NoResponseTimeout) * time.Millisecond):
				stopReading <- struct{}{}
				errChan <- fmt.Errorf("No response timeout")
				return
			}
		}
	}()

	// Read in data until we get our sentinel or are asked to stop
	go func() {
		for {
			select {
			case line := <-s.recv:
				// If we found a line reset the NoResponseTimeout timer
				noResponseTimeout <- struct{}{}
				// If we found the exit code, we're done
				foundExit, exit := checkLine(line, sentinel)
				if foundExit {
					s.e.Emit(Logs, &LogsArgs{
						Options: s.options,
						Hidden:  true,
						Logs:    line,
						Stream:  "stdout",
					})
					exitChan <- exit
					return
				}
				s.e.Emit(Logs, &LogsArgs{
					Options: s.options,
					Hidden:  s.logsHidden,
					Logs:    line,
					Stream:  "stdout",
				})
				recv = append(recv, line)
			case <-stopReading:
				return
			}
		}
	}()

	err = s.Send(sessionCtx, false, commands...)
	if err != nil {
		return -1, []string{}, err
	}
	err = s.Send(sessionCtx, true, fmt.Sprintf("echo %s $?", sentinel))
	if err != nil {
		return -1, []string{}, err
	}

	r := <-commandComplete
	return r.exit, r.recv, r.err
}
