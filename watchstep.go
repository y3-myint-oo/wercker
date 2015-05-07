package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/chuckpreslar/emission"
	"golang.org/x/net/context"
	"gopkg.in/fsnotify.v1"
)

// test TODO (mh)
// 1. change multiple files simultaneously and show that build only happens
//    once
// 2. what happens when files written while build running. queue build?
//    make sure we don't run multiple builds in parallel

// WatchStep needs to implemenet IStep
type WatchStep struct {
	*BaseStep
	Code   string
	reload bool
	data   map[string]string
	logger *LogEntry
	e      *emission.Emitter
}

// NewWatchStep is a special step for doing docker pushes
func NewWatchStep(stepConfig *StepConfig, options *PipelineOptions) (*WatchStep, error) {
	name := "dev"
	displayName := "dev mode"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := &BaseStep{
		displayName: displayName,
		env:         &Environment{},
		id:          name,
		name:        name,
		options:     options,
		owner:       "wercker",
		safeID:      stepSafeID,
		version:     Version(),
	}

	return &WatchStep{
		BaseStep: baseStep,
		data:     stepConfig.Data,
		logger:   rootLogger.WithField("Logger", "WatchStep"),
		e:        GetEmitter(),
	}, nil
}

// InitEnv parses our data into our config
func (s *WatchStep) InitEnv(env *Environment) {
	if code, ok := s.data["code"]; ok {
		s.Code = code
	}
	if reload, ok := s.data["reload"]; ok {
		if v, err := strconv.ParseBool(reload); err == nil {
			s.reload = v
		} else {
			s.logger.Panic(err)
		}
	}
}

// Fetch NOP
func (s *WatchStep) Fetch() (string, error) {
	// nop
	return "", nil
}

func (s *WatchStep) watch(root string) (*fsnotify.Watcher, error) {
	// Set up the filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if err != nil {
				return err
			}
			if strings.HasPrefix(path, s.options.StepDir) {
				return nil
			}
			if strings.HasPrefix(path, s.options.ProjectDir) {
				return nil
			}
			if strings.HasPrefix(path, s.options.BuildDir) {
				return nil
			}
			checkPath := path[len(root):]
			if strings.HasPrefix(checkPath, "/") {
				checkPath = checkPath[1:]
			}
			if strings.HasPrefix(checkPath, ".") {
				return nil
			}
			if strings.HasPrefix(checkPath, "_") {
				return nil
			}
			if !strings.HasPrefix(filepath.Base(path), ".") {
				s.logger.Debugln("Watching:", path)
				if err := watcher.Add(path); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return watcher, nil
}

// killProcesses sends a signal to all the processes on the machine except
// for PID 1, somewhat naive but seems to work
func (s *WatchStep) killProcesses(containerID string, signal string) error {
	client, err := NewDockerClient(s.options.DockerOptions)
	if err != nil {
		return err
	}
	cmd := []string{`/bin/sh`, `-c`, fmt.Sprintf(`ps | grep -v PID | awk "{if (\$1 != 1) print \$1}" | xargs -n 1 kill -s %s`, signal)}
	err = client.ExecOne(containerID, cmd, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

// Execute commits the current container and pushes it to the configured
// registry
func (s *WatchStep) Execute(ctx context.Context, sess *Session) (int, error) {
	// Start watching our stdout
	go func() {
		for {
			line := <-sess.recv
			sess.e.Emit(Logs, &LogsArgs{
				Options: sess.options,
				Hidden:  sess.logsHidden,
				Logs:    line,
				Stream:  "stdout",
			})
		}
	}()

	// If we're not going to reload just run the thing once, synchronously
	if !s.reload {
		exit, _, err := sess.SendChecked(ctx, "set +e", s.Code)
		return exit, err
		// return 0, nil
	}
	f := Formatter{s.options.GlobalOptions}
	s.logger.Info(f.Info("Reloading on file changes"))
	doCmd := func() {
		err := sess.Send(ctx, false, "set +e", s.Code)
		if err != nil {
			s.logger.Errorln(err)
		}
	}

	// Otherwise set up a watcher and do some magic
	// cheating to get containerID
	// TODO(termie): we should deal with this eventually
	dt := sess.transport.(*DockerTransport)
	containerID := dt.containerID

	watcher, err := s.watch(s.options.ProjectPath)
	if err != nil {
		return -1, err
	}

	// connect(s.options.DockerOptions, sess)

	debounce := time.NewTimer(2 * time.Second)
	debounce.Stop()
	done := make(chan struct{})
	go func() {
		for {
			// TODO(termie): wait on os.SIGINT and end our loop, too
			select {
			case event := <-watcher.Events:
				// TODO(mh): we should pause this while build is running.
				// 		 	 python, for example, will generate .pyc files
				// 		 	 which will spawn multiple builds
				s.logger.Debugln("fsnotify event", event.String())
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						s.logger.Debug(f.Info("Modified file", event.Name))
						debounce.Reset(1 * time.Second)
					}
				}
			case <-debounce.C:
				err := s.killProcesses(containerID, "INT")
				if err != nil {
					s.logger.Panic(err)
					break
				}
				s.logger.Info(f.Info("Reloading"))
				go doCmd()
			case err := <-watcher.Errors:
				s.logger.Error(err)
				done <- struct{}{}
			}
		}
	}()

	go doCmd()
	<-done
	return 0, nil
}

// CollectFile NOP
func (s *WatchStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *WatchStep) CollectArtifact(string) (*Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *WatchStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}
