package main

import (
	"code.google.com/p/go-uuid/uuid"
	"code.google.com/p/go.net/websocket"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"io"
	"strings"
)

// Session is our class for interacting with a running Docker container.
type Session struct {
	wsURL       string
	ws          *websocket.Conn
	ch          chan string
	ContainerID string
	e           *emission.Emitter
	logsHidden  bool
}

// CreateSession based on a docker api endpoint and container ID.
func CreateSession(endpoint string, containerID string) *Session {
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
				log.Errorln(err)
				close(ch)
				return
			} else {
				close(ch)
				return
			}
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

// SendChecked sends commands, waits for them to complete and returns the
// exit status and output
func (s *Session) SendChecked(commands ...string) (int, []string, error) {
	var exitCode int
	rand := uuid.NewRandom().String()
	check := false
	recv := []string{}

	s.Send(false, commands...)
	s.Send(true, fmt.Sprintf("echo %s $?", rand))

	// BUG(termie): This is relatively naive and will break if the messages
	// returned aren't complete lines, if this becomes a problem we'll have
	// to buffer it.
	for check != true {
		line, ok := <-s.ch
		if !ok {
			return 1, recv, nil
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

func (s *Session) HideLogs() {
	s.logsHidden = true
}

func (s *Session) ShowLogs() {
	s.logsHidden = false
}
