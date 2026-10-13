package run

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"kbvmaker/v1/compile"
)

// tool writes a shell script that creates its last argument and runs body first.
func tool(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool")
	script := "#!/bin/sh\n" + body + "\nfor a in \"$@\"; do last=\"$a\"; done\n: > \"$last\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunWithReportsEveryStepAndRenamesTheVideo(t *testing.T) {
	dir := t.TempDir()
	ok := tool(t, "")
	var seen []string
	var partialDuringMux bool
	cmds := []compile.Command{
		{Stage: "clip", Argv: []string{ok, "work/clip-000.mp4"}},
		{Stage: "clip", Argv: []string{ok, "work/clip-001.mp4"}},
		{Stage: "mux", Argv: []string{ok, "-i", "x", "out.mp4"}},
	}
	err := RunWith(context.Background(), dir, cmds, Options{
		Atomic: true,
		Progress: func(ev Event) {
			state := "start"
			if ev.Done {
				state = "done"
				if ev.Stage == "mux" {
					_, err := os.Stat(filepath.Join(dir, ".out.partial.mp4"))
					partialDuringMux = err == nil
				}
			}
			seen = append(seen, strings.Join([]string{state, ev.Stage, strconv.Itoa(ev.Step), strconv.Itoa(ev.Steps), strconv.Itoa(ev.N), strconv.Itoa(ev.Of)}, " "))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"start clip 1 3 1 2", "done clip 1 3 1 2",
		"start clip 2 3 2 2", "done clip 2 3 2 2",
		"start mux 3 3 1 1", "done mux 3 3 1 1",
	}
	if strings.Join(seen, "|") != strings.Join(want, "|") {
		t.Fatalf("events\n%v\nwant\n%v", seen, want)
	}
	if !partialDuringMux {
		t.Fatal("mux did not write the partial file")
	}
	if _, err := os.Stat(filepath.Join(dir, "out.mp4")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".out.partial.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial file left behind")
	}
}

func TestRunWithStepErrorKeepsTheOutput(t *testing.T) {
	dir := t.TempDir()
	bad := tool(t, `echo "boom" >&2; exit 3`)
	err := RunWith(context.Background(), dir, []compile.Command{
		{Stage: "mux", Argv: []string{bad, "out.mp4"}},
	}, Options{Atomic: true})
	var step *StepError
	if !errors.As(err, &step) || step.Stage != "mux" || !strings.Contains(step.Output, "boom") || !IsStep(err) {
		t.Fatalf("error %v", err)
	}
	for _, name := range []string{"out.mp4", ".out.partial.mp4"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s exists after a failure", name)
		}
	}
}

func TestRunWithCancelKillsTheCommand(t *testing.T) {
	dir := t.TempDir()
	slow := filepath.Join(t.TempDir(), "slow")
	if err := os.WriteFile(slow, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	err := RunWith(ctx, dir, []compile.Command{{Stage: "clip", Argv: []string{slow, "x.mp4"}}}, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if IsStep(err) {
		t.Fatal("a cancel is not a failed step")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("cancel took %v", took)
	}
}

func TestRunWithCreatesTheWorkDirectory(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "scratch")
	ok := tool(t, "")
	if err := RunWith(context.Background(), dir, []compile.Command{{Stage: "clip", Argv: []string{ok, filepath.Join(abs, "clip-000.mp4")}}}, Options{Work: abs}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(abs, "clip-000.mp4")); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "work")); err != nil {
		t.Fatal("Run should create work under the project directory")
	}
}
