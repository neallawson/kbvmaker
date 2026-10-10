package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/web"
)

// The document the editor sends after New (Classic), a song, and two pictures.
const editorDocument = `{
  "montage": 1,
  "slots": [
    {"src": "/photos/a.jpg", "mode": "in-center"},
    {"src": "/photos/b.jpg", "mode": "out-center", "dur": 3}
  ],
  "canvas": {"width": 3840, "height": 2160, "backdrop": {"kind": "color", "color": "#000000"}},
  "output": {"width": 1920, "height": 1080},
  "defaults": {"join": {"type": "fade", "duration": 1}, "hold": 0.5, "zoomSpan": 0.25, "ease": "linear"},
  "timing": {"policy": "fit-audio"},
  "audio": "/music/song.mp3"
}`

func TestAPISavesTheEditorDocument(t *testing.T) {
	api := &API{handler: web.Server{Probe: &media.Fake{}}.Handler()}
	path := filepath.Join(t.TempDir(), "piece.montage.json")
	body, _ := json.Marshal(map[string]any{"document": path, "body": json.RawMessage(editorDocument)})

	reply, err := api.Call("POST", "/v1/document", string(body))
	if err != nil || reply.Status != 200 {
		t.Fatalf("save %d %s %v", reply.Status, reply.Body, err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"dur": 3`) {
		t.Fatalf("saved %s", saved)
	}
}

func needDisplay(t *testing.T) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display for the GTK dialog")
	}
	if err := exec.Command("python3", "-c", "import gi; gi.require_version('Gtk', '3.0'); from gi.repository import Gtk").Run(); err != nil {
		t.Skip("python3 GTK is not installed")
	}
}

func TestPictureDialogReopensInTheLastFolder(t *testing.T) {
	needDisplay(t)
	first := t.TempDir()
	second := t.TempDir()
	a := filepath.Join(first, "a.jpg")
	b := filepath.Join(second, "b.jpg")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := &Dialogs{}

	t.Setenv("KBV_DIALOG_PICK", a)
	got, err := d.OpenImages()
	if err != nil || len(got) != 1 || got[0] != a {
		t.Fatalf("first pick %v %v", got, err)
	}
	if d.imageDir != first {
		t.Fatalf("remembered %q", d.imageDir)
	}

	t.Setenv("KBV_DIALOG_PICK", b)
	folder, paths := pickShown(t, "multi", d.imageDir)
	if folder != first {
		t.Fatalf("second dialog opened in %q, want %q", folder, first)
	}
	if len(paths) != 1 || paths[0] != b {
		t.Fatalf("second pick %v", paths)
	}
}

func TestSaveDialogReturnsANewMontageInTheLastFolder(t *testing.T) {
	needDisplay(t)
	dir := t.TempDir()
	d := &Dialogs{montageDir: dir}
	t.Setenv("KBV_DIALOG_PICK", "save")
	got, err := d.SaveMontage()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "piece.montage.json"); got != want {
		t.Fatalf("save path %q, want %q", got, want)
	}
}

// pickShown runs the dialog script and returns the folder on first paint and the paths it printed.
func pickShown(t *testing.T, mode, dir string) (string, []string) {
	t.Helper()
	cmd := exec.Command("python3", "-", mode, "Add pictures", dir, "", "Images", "*.jpg")
	cmd.Stdin = strings.NewReader(gtkFilePy)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("dialog %v: %s", err, stderr.String())
	}
	folder := ""
	for _, line := range strings.Split(stderr.String(), "\n") {
		if rest, ok := strings.CutPrefix(line, "folder: "); ok {
			folder = rest
		}
	}
	return folder, strings.Fields(string(out))
}
