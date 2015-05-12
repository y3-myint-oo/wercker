package main

import (
	"bufio"
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

// filterGitignore tries to exclude patterns defined in gitignore
func (s *WatchStep) filterGitignore(root string) []string {
	filters := []string{}
	gitignorePath := filepath.Join(root, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err == nil {
		s.logger.Debugln("Excluding file patterns in .gitignore")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			filters = append(filters, filepath.Join(root, scanner.Text()))
		}
	}
	return filters
}

func (s *WatchStep) watch(root string) (*fsnotify.Watcher, error) {
	// Set up the filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	filters := []string{
		fmt.Sprintf("%s*", s.options.StepDir),
		fmt.Sprintf("%s*", s.options.ProjectDir),
		fmt.Sprintf("%s*", s.options.BuildDir),
		".*",
		"_*",
	}

	watchCount := 0

	// import a .gitignore if it exists
	filters = append(filters, s.filterGitignore(root)...)

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if err != nil {
				return err
			}
			partialPath := filepath.Base(path)

			s.logger.Debugln("check path", path, partialPath)
			for _, pattern := range filters {
				matchFull, err := filepath.Match(pattern, path)
				if err != nil {
					s.logger.Warnln("Bad exclusion pattern: %s", pattern)
				}
				if matchFull {
					s.logger.Debugf("exclude (%s): %s\n", pattern, path)
					return filepath.SkipDir
				}
				matchPartial, _ := filepath.Match(pattern, partialPath)
				if matchPartial {
					s.logger.Debugf("exclude (%s): %s\n", pattern, partialPath)
					return filepath.SkipDir
				}
			}
			s.logger.Debugln("Watching:", path)
			watchCount = watchCount + 1
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Debugf("Watching %d directories\n", watchCount)
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
		err := sess.Send(ctx, false, "set +e", s.Code)
		if err != nil {
			return 0, err
		}
		for {
			// TODO(termie): this thing needs to be replaced with a watch for ctrl-c
			time.Sleep(1)
		}
		// return 0, err
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

	debounce := NewDebouncer(2 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			// TODO(termie): wait on os.SIGINT and end our loop, too
			select {
			case event := <-watcher.Events:
				s.logger.Debugln("fsnotify event", event.String())
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						s.logger.Debug(f.Info("Modified file", event.Name))
						debounce.Trigger()
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

	// Run build on first run
	debounce.Trigger()
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
