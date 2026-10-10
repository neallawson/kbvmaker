package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/web"
)

const pieceJSON = `{
  "montage": 1,
  "audio": "song.wav",
  "slots": [
    {"src": "a.jpg", "mode": "in-center"},
    {"src": "b.jpg", "mode": "out-center", "dur": 3}
  ],
  "canvas": {"width": 640, "height": 360, "backdrop": {"kind": "color", "color": "#000000"}},
  "output": {"width": 320, "height": 180, "file": "out.mp4"},
  "defaults": {"join": {"type": "fade", "duration": 1}, "hold": 0.5, "zoomSpan": 0.25, "ease": "linear"},
  "timing": {"policy": "fit-audio"}
}`

// project is a directory with a Montage, empty media files, and a probe that knows them.
type project struct {
	dir   string
	doc   string
	probe *media.Fake
}

func newProject(t *testing.T) project {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "song.wav", "other.wav"} {
		write(t, filepath.Join(dir, name), "x")
	}
	doc := filepath.Join(dir, "piece.montage.json")
	write(t, doc, pieceJSON)
	return project{
		dir: dir,
		doc: doc,
		probe: &media.Fake{
			Durations: map[string]float64{
				filepath.Join(dir, "song.wav"):  8,
				filepath.Join(dir, "other.wav"): 10,
			},
			Images: map[string]media.Info{
				filepath.Join(dir, "a.jpg"): {Width: 3000, Height: 2000},
				filepath.Join(dir, "b.jpg"): {Width: 2000, Height: 3000},
			},
			Have: map[string]bool{"zoompan": true, "xfade": true, "gblur": true, "color": true},
		},
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type result struct {
	code   int
	stdout string
	stderr string
}

// call runs the CLI in p.dir with p's probe. stdin may be empty.
func (p project) call(t *testing.T, ctx context.Context, stdin string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	e := env{
		stdin:  strings.NewReader(stdin),
		stdout: &out,
		stderr: &errb,
		getwd:  func() (string, error) { return p.dir, nil },
		probe:  func(string, string) (media.Probe, error) { return p.probe, nil },
		listen: func(srv *http.Server) error { return srv.ListenAndServe() },
	}
	code := cli(ctx, args, e)
	return result{code: code, stdout: out.String(), stderr: errb.String()}
}

func (p project) run(t *testing.T, args ...string) result {
	t.Helper()
	return p.call(t, context.Background(), "", args...)
}

func expectCode(t *testing.T, r result, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout, r.stderr)
	}
}

// jsonError reads the single JSON error line on stderr.
func jsonError(t *testing.T, r result) failure {
	t.Helper()
	var f failure
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stderr)), &f); err != nil {
		t.Fatalf("stderr is not one JSON error: %q", r.stderr)
	}
	return f
}

// fakeFFmpeg writes a script that stands in for ffmpeg and ffprobe.
// It creates its last argument when that is a .png or .mp4, which is how
// every compiled command names its output.
func fakeFFmpeg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const touchLast = `for a in "$@"; do last="$a"; done
case "$last" in *.png|*.mp4) : > "$last" ;; esac`

func TestHelp(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "help")
	expectCode(t, r, exitOK)
	for _, want := range []string{"validate", "plan", "export-script", "export-plan", "render", "serve", "version", "130"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("help is missing %q:\n%s", want, r.stdout)
		}
	}
	for _, args := range [][]string{{"render", "-h"}, {"help", "render"}, {"plan", "--help"}} {
		r := p.run(t, args...)
		expectCode(t, r, exitOK)
		if !strings.Contains(r.stdout, "Usage: kbvmaker") || !strings.Contains(r.stdout, "-json") {
			t.Fatalf("%v help:\n%s", args, r.stdout)
		}
	}
	if r := p.run(t, "render", "-h"); !strings.Contains(r.stdout, "-progress") || !strings.Contains(r.stdout, "-clean") {
		t.Fatalf("render help:\n%s", r.stdout)
	}
	if r := p.run(t, "help", "serve"); r.code != exitOK || !strings.Contains(r.stdout, "-addr") {
		t.Fatalf("serve help %d:\n%s", r.code, r.stdout)
	}
}

func TestUsageErrors(t *testing.T) {
	p := newProject(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"unknown command", []string{"bogus"}},
		{"unknown help topic", []string{"help", "bogus"}},
		{"no file", []string{"plan"}},
		{"two files", []string{"plan", "a.json", "b.json"}},
		{"unknown flag", []string{"plan", "--nope", "piece.montage.json"}},
		{"bad number", []string{"plan", "--hold", "abc", "piece.montage.json"}},
		{"bad seed", []string{"plan", "--seed", "-1", "piece.montage.json"}},
		{"bad progress", []string{"render", "--progress", "fancy", "piece.montage.json"}},
		{"dir with a file", []string{"plan", "--dir", "/tmp", "piece.montage.json"}},
		{"version takes no file", []string{"version", "x"}},
		{"serve takes no file", []string{"serve", "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := p.run(t, c.args...)
			expectCode(t, r, exitUsage)
			if r.stdout != "" {
				t.Fatalf("usage error wrote stdout: %q", r.stdout)
			}
			if r.stderr == "" {
				t.Fatal("usage error wrote nothing to stderr")
			}
		})
	}
}

func TestUsageErrorAsJSON(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "plan", "--json", "--hold", "abc", "piece.montage.json")
	expectCode(t, r, exitUsage)
	if f := jsonError(t, r); f.Code != "usage" || !strings.Contains(f.Message, "hold") {
		t.Fatalf("error %+v", f)
	}
}

func TestValidate(t *testing.T) {
	p := newProject(t)
	calls := 0
	probe := p.probe
	var out, errb bytes.Buffer
	e := env{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errb,
		getwd: func() (string, error) { return p.dir, nil },
		probe: func(string, string) (media.Probe, error) { calls++; return probe, nil },
	}
	if code := cli(context.Background(), []string{"validate", "piece.montage.json"}, e); code != exitOK || out.String() != "ok\n" {
		t.Fatalf("validate %d %q %q", code, out.String(), errb.String())
	}
	if calls != 0 {
		t.Fatal("validate without -media probed media")
	}

	r := p.run(t, "validate", "--json", "piece.montage.json")
	expectCode(t, r, exitOK)
	if strings.TrimSpace(r.stdout) != `{"ok":true}` {
		t.Fatalf("json %q", r.stdout)
	}

	r = p.run(t, "validate", "--media", "piece.montage.json")
	expectCode(t, r, exitOK)
	if r.stdout != "ok: 2 pictures, 0:08\n" {
		t.Fatalf("media %q", r.stdout)
	}

	r = p.run(t, "validate", "--media", "--json", "piece.montage.json")
	expectCode(t, r, exitOK)
	var got struct {
		OK       bool    `json:"ok"`
		Slots    int     `json:"slots"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || !got.OK || got.Slots != 2 || got.Duration != 8 {
		t.Fatalf("media json %q %v", r.stdout, err)
	}
}

func TestRuleErrorsExit3WithTheSlot(t *testing.T) {
	p := newProject(t)
	write(t, p.doc, strings.Replace(pieceJSON, `"mode": "out-center"`, `"mode": "out-center", "motion": [{"zoom": 1}]`, 1))
	r := p.run(t, "validate", "piece.montage.json")
	expectCode(t, r, exitInvalid)
	if !strings.Contains(r.stderr, "slot 2") {
		t.Fatalf("stderr %q", r.stderr)
	}
	r = p.run(t, "validate", "--json", "piece.montage.json")
	expectCode(t, r, exitInvalid)
	if f := jsonError(t, r); f.Slot != 2 || f.Code == "" || f.Message == "" {
		t.Fatalf("error %+v", f)
	}
}

func TestOverrideBreakingARuleExits3(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "plan", "--json", "piece.montage.json", "--zoom-span", "0")
	expectCode(t, r, exitInvalid)
	if f := jsonError(t, r); f.Code != "bad-number" || !strings.Contains(f.Message, "zoomSpan") {
		t.Fatalf("error %+v", f)
	}
}

func TestMissingMediaExits4(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "validate", "--json", "nothing.montage.json")
	expectCode(t, r, exitMissing)
	if f := jsonError(t, r); f.Code != "missing-media" {
		t.Fatalf("missing document %+v", f)
	}

	os.Remove(filepath.Join(p.dir, "b.jpg"))
	r = p.run(t, "validate", "--media", "--json", "piece.montage.json")
	expectCode(t, r, exitMissing)
	if f := jsonError(t, r); f.Slot != 2 || f.Code != "missing-media" {
		t.Fatalf("missing picture %+v", f)
	}
	if r := p.run(t, "validate", "piece.montage.json"); r.code != exitOK {
		t.Fatalf("validate without -media should not read pictures: %d %s", r.code, r.stderr)
	}
}

func TestMissingToolExits4(t *testing.T) {
	p := newProject(t)
	var out, errb bytes.Buffer
	e := env{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errb,
		getwd: func() (string, error) { return p.dir, nil },
		probe: ffProbe,
	}
	code := cli(context.Background(), []string{"plan", "--json", "--ffmpeg", "/no/such/ffmpeg", "piece.montage.json"}, e)
	if code != exitMissing {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	var f failure
	if err := json.Unmarshal(errb.Bytes(), &f); err != nil || f.Code != "missing-tool" || !strings.Contains(f.Message, "/no/such/ffmpeg") {
		t.Fatalf("error %q", errb.String())
	}
}

func planOf(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("plan is not JSON: %v\n%s", err, s)
	}
	return m
}

func TestPlanFlagsMayFollowTheFile(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "plan", "piece.montage.json", "--join", "0.5")
	expectCode(t, r, exitOK)
	slots := planOf(t, r.stdout)["slots"].([]any)
	join := slots[0].(map[string]any)["join"].(map[string]any)
	if join["duration"] != 0.5 {
		t.Fatalf("join %v", join)
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	p := newProject(t)
	write(t, filepath.Join(p.dir, "-odd.montage.json"), pieceJSON)
	r := p.run(t, "validate", "--", "-odd.montage.json")
	expectCode(t, r, exitOK)
}

func TestPlanFromStdin(t *testing.T) {
	p := newProject(t)
	r := p.call(t, context.Background(), pieceJSON, "plan", "-")
	expectCode(t, r, exitOK)
	if planOf(t, r.stdout)["duration"] != 8.0 {
		t.Fatalf("plan %s", r.stdout)
	}

	elsewhere := t.TempDir()
	r = p.call(t, context.Background(), pieceJSON, "plan", "-dir", elsewhere, "-")
	expectCode(t, r, exitMissing)

	r = p.call(t, context.Background(), pieceJSON, "plan", "-dir", p.dir, "-")
	expectCode(t, r, exitOK)

	r = p.call(t, context.Background(), "{not json", "validate", "-")
	expectCode(t, r, exitInvalid)
}

func TestAudioOverrideIsRelativeToTheCurrentDirectory(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "plan", "--audio", "other.wav", "piece.montage.json")
	expectCode(t, r, exitOK)
	plan := planOf(t, r.stdout)
	if plan["audio"] != filepath.Join(p.dir, "other.wav") || plan["duration"] != 10.0 {
		t.Fatalf("audio %v duration %v", plan["audio"], plan["duration"])
	}
}

func TestPlanMatchesTheHTTPAPI(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "plan", "piece.montage.json")
	expectCode(t, r, exitOK)

	rec := httptest.NewRecorder()
	body := `{"document":` + quote(p.doc) + `}`
	web.Server{Probe: p.probe}.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/plan", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("http %d %s", rec.Code, rec.Body.String())
	}
	if cli, api := planOf(t, r.stdout), planOf(t, rec.Body.String()); !reflect.DeepEqual(cli, api) {
		t.Fatalf("CLI and /v1 plans differ\ncli: %v\napi: %v", cli, api)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestOutputFiles(t *testing.T) {
	p := newProject(t)
	out := filepath.Join(p.dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}

	r := p.run(t, "plan", "-o", "out/plan.json", "piece.montage.json")
	expectCode(t, r, exitOK)
	if r.stdout != "" {
		t.Fatalf("plan -o wrote stdout %q", r.stdout)
	}
	b, err := os.ReadFile(filepath.Join(out, "plan.json"))
	if err != nil || planOf(t, string(b))["duration"] != 8.0 {
		t.Fatalf("plan file %v %s", err, b)
	}

	r = p.run(t, "export-script", "--output", "out/render.sh", "piece.montage.json")
	expectCode(t, r, exitOK)
	info, err := os.Stat(filepath.Join(out, "render.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("script %v %v", err, info)
	}
	script, _ := os.ReadFile(filepath.Join(out, "render.sh"))
	if !strings.HasPrefix(string(script), "#!/bin/bash\nset -euo pipefail\nmkdir -p 'work'\n") {
		t.Fatalf("script head %q", script[:60])
	}

	entries, _ := os.ReadDir(out)
	if len(entries) != 2 {
		t.Fatalf("temporary files left behind: %v", entries)
	}

	r = p.run(t, "plan", "-o", "no/such/dir/plan.json", "piece.montage.json")
	expectCode(t, r, exitMissing)
}

func TestExportPlanAndScriptOptions(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "export-plan", "piece.montage.json")
	expectCode(t, r, exitOK)
	var rp struct {
		RenderPlan int        `json:"renderPlan"`
		Commands   [][]string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &rp); err != nil || rp.RenderPlan != 1 || len(rp.Commands) == 0 {
		t.Fatalf("renderPlan %v %s", err, r.stdout)
	}
	if !strings.Contains(r.stdout, `"-loglevel"`) || !strings.Contains(r.stdout, "work/clip-000.mp4") {
		t.Fatalf("default commands %s", r.stdout)
	}

	r = p.run(t, "export-plan", "--verbose", "--work", "scratch", "--ffmpeg", "/opt/ff/ffmpeg", "piece.montage.json")
	expectCode(t, r, exitOK)
	if strings.Contains(r.stdout, `"-loglevel"`) || !strings.Contains(r.stdout, "scratch/clip-000.mp4") || !strings.Contains(r.stdout, "/opt/ff/ffmpeg") {
		t.Fatalf("verbose scratch commands %s", r.stdout)
	}

	r = p.run(t, "export-script", "--work", "scratch", "piece.montage.json")
	expectCode(t, r, exitOK)
	if !strings.Contains(r.stdout, "mkdir -p 'scratch'") {
		t.Fatalf("script %s", r.stdout)
	}
}

func TestRenderWritesTheVideoAndReportsProgress(t *testing.T) {
	p := newProject(t)
	ff := fakeFFmpeg(t, touchLast)
	r := p.run(t, "render", "piece.montage.json", "--ffmpeg", ff, "--ffprobe", ff)
	expectCode(t, r, exitOK)
	video := filepath.Join(p.dir, "out.mp4")
	if strings.TrimSpace(r.stdout) != video {
		t.Fatalf("stdout %q", r.stdout)
	}
	if _, err := os.Stat(video); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.dir, ".out.partial.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the partial video was left behind")
	}
	for _, want := range []string{"[1/8] filters 1/1", "clip 2/2", "[8/8] mux 1/1", "wrote " + video} {
		if !strings.Contains(r.stderr, want) {
			t.Fatalf("progress is missing %q:\n%s", want, r.stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(p.dir, "work", "clip-000.mp4")); err != nil {
		t.Fatal("intermediates should stay without -clean")
	}
}

func TestRenderJSONProgressAndClean(t *testing.T) {
	p := newProject(t)
	ff := fakeFFmpeg(t, touchLast)
	write(t, filepath.Join(p.dir, "keep.txt"), "mine")
	r := p.run(t, "render", "--progress", "json", "--json", "--clean", "--work", ".", "-o", "final.mp4",
		"--ffmpeg", ff, "--ffprobe", ff, "piece.montage.json")
	expectCode(t, r, exitOK)

	var events []map[string]any
	sc := bufio.NewScanner(strings.NewReader(r.stderr))
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("progress line is not JSON: %q", sc.Text())
		}
		events = append(events, ev)
	}
	if len(events) != 17 {
		t.Fatalf("%d events:\n%s", len(events), r.stderr)
	}
	if events[0]["event"] != "start" || events[1]["event"] != "done" || events[16]["event"] != "wrote" {
		t.Fatalf("events %v", events)
	}
	video := filepath.Join(p.dir, "final.mp4")
	if events[16]["file"] != video {
		t.Fatalf("wrote %v", events[16])
	}
	var res struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &res); err != nil || res.File != video {
		t.Fatalf("stdout %q", r.stdout)
	}
	for _, name := range []string{"canvas-0.png", "clip-000.mp4", "join-000.mp4"} {
		if _, err := os.Stat(filepath.Join(p.dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("-clean left %s", name)
		}
	}
	for _, name := range []string{"keep.txt", "a.jpg", "piece.montage.json", "final.mp4"} {
		if _, err := os.Stat(filepath.Join(p.dir, name)); err != nil {
			t.Fatalf("-clean removed %s", name)
		}
	}
}

func TestRenderFailureExits5AndLeavesNoVideo(t *testing.T) {
	p := newProject(t)
	ok := fakeFFmpeg(t, touchLast)
	bad := fakeFFmpeg(t, `case "$*" in *-filters*) exit 0 ;; esac
echo "boom: no such filter" >&2; exit 1`)
	r := p.run(t, "render", "--json", "--progress", "none", "--ffmpeg", bad, "--ffprobe", ok, "piece.montage.json")
	expectCode(t, r, exitRender)
	f := jsonError(t, r)
	if f.Code != "render" || !strings.Contains(f.Message, "canvas") || !strings.Contains(f.Message, "boom") {
		t.Fatalf("error %+v", f)
	}
	for _, name := range []string{"out.mp4", ".out.partial.mp4"} {
		if _, err := os.Stat(filepath.Join(p.dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s exists after a failed render", name)
		}
	}
}

func TestInterruptStopsFFmpegAndExits130(t *testing.T) {
	p := newProject(t)
	slow := fakeFFmpeg(t, `case "$*" in *-filters*|*format=duration*) exit 0 ;; esac
exec sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	start := time.Now()
	r := p.call(t, ctx, "", "render", "--progress", "none", "--ffmpeg", slow, "--ffprobe", slow, "piece.montage.json")
	expectCode(t, r, exitInterrupted)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("interrupt took %v", took)
	}
	if _, err := os.Stat(filepath.Join(p.dir, "out.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an interrupted render left a video")
	}
}

func TestVersion(t *testing.T) {
	p := newProject(t)
	r := p.run(t, "version")
	expectCode(t, r, exitOK)
	if !strings.HasPrefix(r.stdout, "kbvmaker ") || !strings.Contains(r.stdout, "(Montage 1)") {
		t.Fatalf("version %q", r.stdout)
	}
	r = p.run(t, "--version", "--json")
	expectCode(t, r, exitOK)
	var v struct {
		Montage    int    `json:"montage"`
		RenderPlan int    `json:"renderPlan"`
		Version    string `json:"version"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || v.Montage != 1 || v.RenderPlan != 1 || v.Version == "" {
		t.Fatalf("version json %q", r.stdout)
	}
}

func TestServeStopsOnInterrupt(t *testing.T) {
	p := newProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	r := p.call(t, ctx, "", "serve", "--addr", "127.0.0.1:0")
	expectCode(t, r, exitOK)
	if !strings.Contains(r.stderr, "listening") {
		t.Fatalf("stderr %q", r.stderr)
	}
}

func TestRenderWithRealFFmpeg(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " is not installed")
		}
	}
	dir := t.TempDir()
	gen := func(args ...string) {
		cmd := exec.Command("ffmpeg", append([]string{"-loglevel", "error", "-y"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	gen("-f", "lavfi", "-i", "testsrc=s=640x480:d=1", "-frames:v", "1", "a.jpg")
	gen("-f", "lavfi", "-i", "testsrc2=s=480x640:d=1", "-frames:v", "1", "b.jpg")
	gen("-f", "lavfi", "-i", "sine=d=4", "song.wav")
	write(t, filepath.Join(dir, "piece.montage.json"), strings.Replace(pieceJSON, `, "dur": 3`, "", 1))

	var out, errb bytes.Buffer
	code := cli(context.Background(), []string{"render", "--progress", "none", "--clean", "piece.montage.json"}, env{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errb,
		getwd: func() (string, error) { return dir, nil },
		probe: ffProbe,
	})
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, errb.String())
	}
	video := strings.TrimSpace(out.String())
	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration:stream=width,height",
		"-of", "default=noprint_wrappers=1", video).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(probe), "width=320") || !strings.Contains(string(probe), "height=180") || !strings.Contains(string(probe), "duration=4.0") {
		t.Fatalf("video %s", probe)
	}
	if _, err := os.Stat(filepath.Join(dir, "work")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("-clean left the work directory")
	}
}
