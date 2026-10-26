// Package run executes a compiled command list.
package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kbvmaker/v1/compile"
)

// Event reports one step starting or finishing.
// Step and Steps count every command. N and Of count commands of the same stage.
type Event struct {
	Done    bool
	Stage   string
	Step    int
	Steps   int
	N       int
	Of      int
	Seconds float64
}

// Options controls where intermediates go and who hears about progress.
type Options struct {
	// Work is the intermediate directory the commands were compiled with.
	// Relative paths are under dir. Empty means "work".
	Work string
	// Progress, if set, is called before and after each command.
	Progress func(Event)
	// Atomic writes the final video under a temporary name in the same
	// directory and renames it when the last command succeeds.
	Atomic bool
}

// StepError is a command that exited with an error. Output is its stdout and stderr.
type StepError struct {
	Stage  string
	Err    error
	Output string
}

func (e *StepError) Error() string {
	out := strings.TrimSpace(e.Output)
	if out == "" {
		return fmt.Sprintf("%s: %v", e.Stage, e.Err)
	}
	return fmt.Sprintf("%s: %v\n%s", e.Stage, e.Err, out)
}

func (e *StepError) Unwrap() error { return e.Err }

// Run starts each command in dir, in order, and stops at the first failure.
// Stdout and stderr of the failed command are part of the error.
func Run(ctx context.Context, dir string, cmds []compile.Command) error {
	return RunWith(ctx, dir, cmds, Options{})
}

// RunWith is Run with progress, a chosen work directory, and an atomic final write.
// When ctx is cancelled the running command is killed and ctx.Err() is returned.
func RunWith(ctx context.Context, dir string, cmds []compile.Command, opt Options) error {
	work := opt.Work
	if work == "" {
		work = "work"
	}
	if !filepath.IsAbs(work) {
		work = filepath.Join(dir, work)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	cmds, final, partial := atomicMux(cmds, dir, opt.Atomic)
	if partial != "" {
		defer os.Remove(partial)
	}
	of := map[string]int{}
	for _, c := range cmds {
		of[c.Stage]++
	}
	seen := map[string]int{}
	for i, c := range cmds {
		if err := ctx.Err(); err != nil {
			return err
		}
		seen[c.Stage]++
		ev := Event{Stage: c.Stage, Step: i + 1, Steps: len(cmds), N: seen[c.Stage], Of: of[c.Stage]}
		if opt.Progress != nil {
			opt.Progress(ev)
		}
		start := time.Now()
		if len(c.Body) > 0 {
			if c.File == "" {
				return fmt.Errorf("%s: write is missing a path", c.Stage)
			}
			path := abs(dir, c.File)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, c.Body, 0o644); err != nil {
				return err
			}
			if opt.Progress != nil {
				ev.Done = true
				ev.Seconds = time.Since(start).Seconds()
				opt.Progress(ev)
			}
			continue
		}
		if len(c.Argv) == 0 {
			return fmt.Errorf("%s: empty command", c.Stage)
		}
		cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
		cmd.Dir = dir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &StepError{Stage: c.Stage, Err: err, Output: out.String()}
		}
		if opt.Progress != nil {
			ev.Done = true
			ev.Seconds = time.Since(start).Seconds()
			opt.Progress(ev)
		}
	}
	if partial != "" {
		if err := os.Rename(partial, final); err != nil {
			return err
		}
	}
	return nil
}

// atomicMux points the mux command at a hidden file next to the output.
// It returns the commands, the final path, and the partial path ("" when unchanged).
func atomicMux(cmds []compile.Command, dir string, atomic bool) ([]compile.Command, string, string) {
	if !atomic {
		return cmds, "", ""
	}
	for i := len(cmds) - 1; i >= 0; i-- {
		c := cmds[i]
		if c.Stage != "mux" || len(c.Argv) < 2 {
			continue
		}
		out := c.Argv[len(c.Argv)-1]
		ext := filepath.Ext(out)
		tmp := filepath.Join(filepath.Dir(out), "."+strings.TrimSuffix(filepath.Base(out), ext)+".partial"+ext)
		argv := append([]string(nil), c.Argv...)
		argv[len(argv)-1] = tmp
		copied := append([]compile.Command(nil), cmds...)
		copied[i] = compile.Command{Stage: c.Stage, Argv: argv}
		return copied, abs(dir, out), abs(dir, tmp)
	}
	return cmds, "", ""
}

func abs(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// IsStep reports whether err is a command that failed.
func IsStep(err error) bool {
	var s *StepError
	return errors.As(err, &s)
}
