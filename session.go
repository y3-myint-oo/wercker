package main

import (
	"code.google.com/p/go-uuid/uuid"
	"code.google.com/p/go.net/websocket"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"io"
	"strings"
	"time"
)

// Session is our class for interacting with a running Docker container.
type Session struct {
	wsURL       string
	ws          *websocket.Conn
	ch          chan string
	ContainerID string
	e           *emission.Emitter
	logsHidden  bool
	options     *GlobalOptions
}

// NewSession based on a docker api endpoint and container ID.
func NewSession(endpoint string, containerID string, options *GlobalOptions) *Session {
	wsEndpoint := strings.Replace(endpoint, "tcp://", "ws://", 1)
	wsQuery := "stdin=1&stderr=1&stdout=1&stream=1"
	wsURL := fmt.Sprintf("%s/containers/%s/attach/ws?%s",
		wsEndpoint, containerID, wsQuery)

	ch := make(chan string)

	return &Session{
		wsURL:       wsURL,
		ws:          nil,
		ch:          ch,
		ContainerID: containerID,
		e:           GetEmitter(),
		options:     options,
	}
}

// ReadToChan reads on a websocket forever, writing to a channel
func ReadToChan(ws *websocket.Conn, ch chan string) {
	var data string
	for {
		data = ""
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			if err != io.EOF {
				log.WithField("Error", err).Error("Error while reading from websocket")
			}
			close(ch)
			return
		}
		ch <- data
	}
}

// Attach begins reading on the websocket and writing to the internal channel.
func (s *Session) Attach() (*Session, error) {
	ws, err := websocket.Dial(s.wsURL, "", "http://localhost/")
	if err != nil {
		return s, err
	}
	s.ws = ws

	go ReadToChan(s.ws, s.ch)
	return s, nil
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

		err := websocket.Message.Send(s.ws, command)
		if err != nil {
			log.Panicln(err)
		}
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
			case myline, ok := <-s.ch:
				if !ok {
					return 1, recv, nil
				}
				line = myline
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

// HideLogs will emit Logs with args.Hidden set to true
func (s *Session) HideLogs() {
	s.logsHidden = true
}

// ShowLogs will emit Logs with args.Hidden set to false
func (s *Session) ShowLogs() {
	s.logsHidden = false
}
