package main

import (
	"fmt"
	"strings"
)

const (
	SuccessColor = "\x1b[32m"
	FailColor    = "\x1b[31m"
	VarColor     = "\x1b[33m"
	reset        = "\x1b[m"
)

// Formatter formats the messages, and optionally disabling colors. See
// FormatMessage for the structure of messages.
type Formatter struct {
	options *GlobalOptions
}

// Info uses no color.
func (f *Formatter) Info(messages ...string) string {
	return FormatMessage("", f.options.ShowColors, messages...)
}

// Success uses SuccessColor (green) as color.
func (f *Formatter) Success(messages ...string) string {
	return FormatMessage(SuccessColor, f.options.ShowColors, messages...)
}

// Fail uses FailColor (red) as color.
func (f *Formatter) Fail(messages ...string) string {
	return FormatMessage(FailColor, f.options.ShowColors, messages...)
}

// FormatMessage handles one or two messages. If more messages are used, those
// are ignore. If no messages are used, than it will return an empty string.
// 1 message : --> message[0]
// 2 messages: --> message[0]: message[1]
// color will be applied to the first message, VarColor will be used for the
// second message. If useColors is false, than color will be ignored.
func FormatMessage(color string, useColors bool, messages ...string) string {
	segments := []string{}

	l := len(messages)

	if l > 0 {
		segments = append(segments, "-->")
	}

	if l >= 1 {
		if useColors {
			segments = append(segments, fmt.Sprintf(" %s%s%s", color, messages[0], reset))
		} else {
			segments = append(segments, fmt.Sprintf(" %s", messages[0]))
		}
	}

	if l >= 2 {
		if useColors {
			segments = append(segments, fmt.Sprintf(": %s%s%s", VarColor, messages[1], reset))
		} else {
			segments = append(segments, fmt.Sprintf(": %s", messages[1]))
		}
	}

	return strings.Join(segments, "")
}
