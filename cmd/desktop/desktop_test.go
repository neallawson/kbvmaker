package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbvmaker/v1/media"
	"kbvmaker/web"
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
