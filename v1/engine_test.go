package v1_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbvmaker/v1"
	"kbvmaker/v1/compile"
	"kbvmaker/v1/media"
)

func TestResolveCompileAndExports(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "song.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	body := `{
	  "montage": 1,
	  "audio": "song.mp3",
	  "slots": [
	    {"src": "a.jpg", "mode": "out-center"},
	    {"src": "b.jpg", "mode": "in-left-right"}
	  ]
	}`
	if err := os.WriteFile(filepath.Join(dir, "piece.montage.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(filepath.Join(dir, "piece.montage.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "a.jpg")
	b := filepath.Join(dir, "b.jpg")
	song := filepath.Join(dir, "song.mp3")
	eng := v1.New(&media.Fake{
		Durations: map[string]float64{song: 20},
		Images: map[string]media.Info{
			a: {Width: 3000, Height: 4000},
			b: {Width: 3000, Height: 4000},
		},
	})
	p, err := eng.Resolve(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.Duration-20) > 1e-9 || math.Abs(p.Slots[0].ClipDur-10.75) > 1e-9 {
		t.Fatalf("timing song %v clip %v", p.Duration, p.Slots[0].ClipDur)
	}
	if math.Abs(p.Slots[0].Join.Start-9.25) > 1e-9 {
		t.Fatalf("join %v", p.Slots[0].Join.Start)
	}
	// Far zoom is limit, not the portrait fill (~2.37).
	if p.Slots[0].FarZoom >= p.Slots[0].Fill || p.Slots[0].FarZoom < 1.3 || p.Slots[0].FarZoom > 1.31 {
		t.Fatalf("far %v fill %v", p.Slots[0].FarZoom, p.Slots[0].Fill)
	}
	if p.Slots[0].OpaqueStart != 0 || math.Abs(p.Slots[1].OpaqueStart-1.5) > 1e-9 {
		t.Fatalf("opaque %v %v", p.Slots[0].OpaqueStart, p.Slots[1].OpaqueStart)
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	script := v1.ExportScript(cmds)
	plan, err := v1.ExportPlan(cmds)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		RenderPlan int        `json:"renderPlan"`
		Commands   [][]string `json:"commands"`
	}
	if err := json.Unmarshal(plan, &file); err != nil {
		t.Fatal(err)
	}
	if file.RenderPlan != v1.Version || len(file.Commands) != len(cmds) {
		t.Fatalf("renderPlan %d commands %d", file.RenderPlan, len(file.Commands))
	}
	for i, c := range cmds {
		if strings.Join(file.Commands[i], "\x00") != strings.Join(c.Argv, "\x00") {
			t.Fatalf("json argv %d differs", i)
		}
		for _, arg := range c.Argv {
			if !strings.Contains(script, "'"+arg+"'") && !strings.Contains(script, bashSafe(arg)) {
				t.Fatalf("script missing %q", arg)
			}
		}
	}
	var join string
	for _, c := range cmds {
		if c.Stage == "join" {
			join = strings.Join(c.Argv, " ")
		}
		if c.Stage == "clip" && strings.Contains(strings.Join(c.Argv, " "), "clip-000") {
			vf := argAfter(c.Argv, "-vf")
			if !strings.Contains(vf, "1.306977") || !strings.Contains(vf, "iw/2-(iw/zoom/2)") {
				t.Fatalf("out-center filter %s", vf)
			}
		}
	}
	if !strings.Contains(join, "offset=9.25") || !strings.Contains(join, "duration=1.5") {
		t.Fatalf("join %s", join)
	}
}

func TestBatchJoins(t *testing.T) {
	dir := t.TempDir()
	names := []string{"a.jpg", "b.jpg", "c.jpg", "song.mp3"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	body := `{
	  "montage": 1,
	  "audio": "song.mp3",
	  "defaults": {"hold": 0.2, "join": {"type": "fade", "duration": 0.4}},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold"},
	    {"src": "b.jpg", "mode": "hold"},
	    {"src": "c.jpg", "mode": "hold"}
	  ]
	}`
	path := filepath.Join(dir, "p.montage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	img := media.Info{Width: 1000, Height: 1000}
	eng := v1.New(&media.Fake{
		Durations: map[string]float64{filepath.Join(dir, "song.mp3"): 30},
		Images: map[string]media.Info{
			filepath.Join(dir, "a.jpg"): img,
			filepath.Join(dir, "b.jpg"): img,
			filepath.Join(dir, "c.jpg"): img,
		},
	})
	eng.Options = compile.Options{MaxJoinInputs: 2}
	p, err := eng.Resolve(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	joins := 0
	for _, c := range cmds {
		if c.Stage == "join" {
			joins++
		}
	}
	if joins != 2 {
		t.Fatalf("joins %d, want 2 for three clips batched by twos", joins)
	}
}

func argAfter(argv []string, flag string) string {
	for i, a := range argv {
		if a == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

func bashSafe(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}
