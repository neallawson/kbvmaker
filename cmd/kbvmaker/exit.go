package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"kbvmaker/v1"
	"kbvmaker/v1/rule"
)

// Exit codes. A script can tell a bad document from a missing photo from a
// failed ffmpeg step without reading the message.
const (
	exitOK          = 0
	exitFail        = 1
	exitUsage       = 2
	exitInvalid     = 3
	exitMissing     = 4
	exitRender      = 5
	exitInterrupted = 130
)

// usageError is a mistake on the command line.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return usageError{msg: fmt.Sprintf(format, args...)}
}

// toolError is ffmpeg or ffprobe missing from PATH or from the flag given.
type toolError struct{ err error }

func (e toolError) Error() string { return e.err.Error() }

// failure is the stable form of an error: exit code, slot, code, message.
type failure struct {
	Exit    int    `json:"-"`
	Slot    int    `json:"slot"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func classify(err error) failure {
	var use usageError
	var tool toolError
	switch {
	case errors.As(err, &use):
		return failure{Exit: exitUsage, Code: "usage", Message: use.msg}
	case errors.Is(err, context.Canceled):
		return failure{Exit: exitInterrupted, Code: "interrupted", Message: "interrupted"}
	case errors.As(err, &tool):
		return failure{Exit: exitMissing, Code: "missing-tool", Message: tool.Error()}
	case v1.IsStepFailure(err):
		return failure{Exit: exitRender, Code: "render", Message: err.Error()}
	}
	if re, ok := rule.As(err); ok {
		exit := exitInvalid
		if re.Code == "missing-media" {
			exit = exitMissing
		}
		return failure{Exit: exit, Slot: re.Slot, Code: re.Code, Message: re.Msg}
	}
	if errors.Is(err, os.ErrNotExist) {
		return failure{Exit: exitMissing, Code: "missing-media", Message: err.Error()}
	}
	return failure{Exit: exitFail, Code: "error", Message: err.Error()}
}

// report writes err to w as one line of text or one JSON object, and returns the exit code.
func report(w io.Writer, err error, asJSON bool) int {
	f := classify(err)
	if asJSON {
		b, _ := json.Marshal(f)
		fmt.Fprintf(w, "%s\n", b)
		return f.Exit
	}
	switch {
	case f.Slot > 0:
		fmt.Fprintf(w, "kbvmaker: slot %d: %s: %s\n", f.Slot, f.Code, f.Message)
	case f.Code == "usage":
		fmt.Fprintf(w, "kbvmaker: %s\n", f.Message)
	case f.Code == "error" || f.Code == "interrupted" || f.Code == "render" || f.Code == "missing-tool":
		fmt.Fprintf(w, "kbvmaker: %s\n", f.Message)
	default:
		fmt.Fprintf(w, "kbvmaker: %s: %s\n", f.Code, f.Message)
	}
	return f.Exit
}
