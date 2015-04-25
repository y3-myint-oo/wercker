package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/chuckpreslar/emission"
	"golang.org/x/net/context"
	"gopkg.in/fsnotify.v1"
)

// WatchStep needs to implemenet IStep
type WatchStep struct {
	*BaseStep
	Code       string
	reload     bool
	ProjectDir string
	data       map[string]string
	logger     *LogEntry
	e          *emission.Emitter
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

	options.DirectMount = true

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
		BaseStep:   baseStep,
		data:       stepConfig.Data,
		ProjectDir: options.ProjectPath,
		logger:     rootLogger.WithField("Logger", "WatchStep"),
		e:          GetEmitter(),
	}, nil
}

// InitEnv parses our data into our config
func (s *WatchStep) InitEnv(env *Environment) {
	if code, ok := s.data["code"]; ok {
		s.Code = code
	}
	if reload, ok := s.data["reload"]; ok {
		if v, err := strconv.ParseBool(reload); err != nil {
			s.reload = v
		}
	}
}

// Fetch NOP
func (s *WatchStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute commits the current container and pushes it to the configured
// registry
func (s *WatchStep) Execute(ctx context.Context, sess *Session) (int, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return 1, err
	}
	defer watcher.Close()
	if _, _, err = sess.SendChecked(ctx, s.env.Export()...); err != nil {
		s.logger.Println(err)
		return 1, err
	}
	cmd := fmt.Sprintf("set +e; %s", s.Code)
	// We don't care if this command succeeded
	sess.SendChecked(ctx, cmd)

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// s.logger.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						s.logger.Println("modified file:", event.Name)
						exit, _, err := sess.SendChecked(ctx, cmd)
						s.logger.Println(exit, err)
					}
				}
			case err := <-watcher.Errors:
				s.logger.Println("error:", err)
			}
		}
	}()

	err = filepath.Walk(s.ProjectDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if err != nil {
				return err
			}
			if !strings.HasPrefix(filepath.Base(path), ".") {
				if err := watcher.Add(path); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 1, err
	}
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

// DisplayName getter
func (s *WatchStep) DisplayName() string {
	return s.displayName
}

// Env getter
func (s *WatchStep) Env() *Environment {
	return s.env
}

// Cwd getter
func (s *WatchStep) Cwd() string {
	return s.cwd
}

// ID getter
func (s *WatchStep) ID() string {
	return s.id
}

// Name getter
func (s *WatchStep) Name() string {
	return s.name
}

// Owner getter
func (s *WatchStep) Owner() string {
	return s.owner
}

// SafeID getter
func (s *WatchStep) SafeID() string {
	return s.safeID
}

// Version getter
func (s *WatchStep) Version() string {
	return s.version
}
