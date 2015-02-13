package main

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

// Logger is a wrapper for logrus so that we don't have to keep referring
// to its types everywhere and can add helpers
type Logger struct {
	*logrus.Logger
}

type LogFields logrus.Fields

func NewLogger() *Logger {
	l := &Logger{logrus.New()}
	l.Formatter = &TerseFormatter{}
	return l
}

func (l *Logger) SetLevel(level string) {
	l.Level, _ = logrus.ParseLevel(level)
}

func (l *Logger) WithFields(fields LogFields) *LogEntry {
	return &LogEntry{l.Logger.WithFields(logrus.Fields(fields))}
}

func (l *Logger) WithField(key string, value interface{}) *LogEntry {
	return &LogEntry{l.Logger.WithField(key, value)}
}

type LogEntry struct {
	*logrus.Entry
}

func (e *LogEntry) WithField(key string, value interface{}) *LogEntry {
	return &LogEntry{e.Entry.WithField(key, value)}
}

func (e *LogEntry) WithFields(fields LogFields) *LogEntry {
	return &LogEntry{e.Entry.WithFields(logrus.Fields(fields))}
}

// Our root logger
var rootLogger = NewLogger()

// NOTE(termie): Pretty much everything below here is slightly modified
//               copy-paste from logrus, it doesn't offer a very easy way
//               to modify the output template

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 34
)

var (
	baseTimestamp time.Time
	isTerminal    bool
	noQuoteNeeded *regexp.Regexp
)

func init() {
	baseTimestamp = time.Now()
	isTerminal = logrus.IsTerminal()
}

// This is to not silently overwrite `time`, `msg` and `level` fields when
// dumping it. If this code wasn't there doing:
//
//  logrus.WithField("level", 1).Info("hello")
//
// Would just silently drop the user provided level. Instead with this code
// it'll logged as:
//
//  {"level": "info", "fields.level": 1, "msg": "hello", "time": "..."}
//
// It's not exported because it's still using Data in an opinionated way. It's to
// avoid code duplication between the two default formatters.
func prefixFieldClashes(data logrus.Fields) {
	_, ok := data["time"]
	if ok {
		data["fields.time"] = data["time"]
	}

	_, ok = data["msg"]
	if ok {
		data["fields.msg"] = data["msg"]
	}

	_, ok = data["level"]
	if ok {
		data["fields.level"] = data["level"]
	}
}

func miniTS() int {
	return int(time.Since(baseTimestamp) / time.Second)
}

type TerseFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors   bool
	DisableColors bool
	// Set to true to disable timestamp logging (useful when the output
	// is redirected to a logging system already adding a timestamp)
	DisableTimestamp bool
}

func (f *TerseFormatter) Format(entry *logrus.Entry) ([]byte, error) {

	var keys []string
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b := &bytes.Buffer{}

	prefixFieldClashes(entry.Data)

	isColored := (f.ForceColors || isTerminal) && !f.DisableColors

	if isColored {
		printColored(b, entry, keys)
	} else {
		if !f.DisableTimestamp {
			f.appendKeyValue(b, "time", entry.Time.Format(time.RFC3339))
		}
		f.appendKeyValue(b, "level", entry.Level.String())
		f.appendKeyValue(b, "msg", entry.Message)
		for _, key := range keys {
			f.appendKeyValue(b, key, entry.Data[key])
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func printColored(b *bytes.Buffer, entry *logrus.Entry, keys []string) {
	var levelColor int
	switch entry.Level {
	case logrus.WarnLevel:
		levelColor = yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	levelText := strings.ToUpper(entry.Level.String())[0:4]

	var source string
	source, ok := entry.Data["Logger"].(string)
	if !ok {
		source = "root"
	}
	source = strings.ToLower(source)
	fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %8.8s| %-44s ", levelColor, levelText, miniTS(), source, entry.Message)
	for _, k := range keys {
		if k != "Error" {
			continue
		}
		v := entry.Data[k]
		fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=%v", levelColor, k, v)
	}
}

func needsQuoting(text string) bool {
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch < '9') ||
			ch == '-' || ch == '.') {
			return false
		}
	}
	return true
}

func (f *TerseFormatter) appendKeyValue(b *bytes.Buffer, key, value interface{}) {
	switch value.(type) {
	case string:
		if needsQuoting(value.(string)) {
			fmt.Fprintf(b, "%v=%s ", key, value)
		} else {
			fmt.Fprintf(b, "%v=%q ", key, value)
		}
	case error:
		if needsQuoting(value.(error).Error()) {
			fmt.Fprintf(b, "%v=%s ", key, value)
		} else {
			fmt.Fprintf(b, "%v=%q ", key, value)
		}
	default:
		fmt.Fprintf(b, "%v=%v ", key, value)
	}
}
