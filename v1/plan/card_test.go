package plan

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kbvmaker/v1/doc"
)

func TestResolveCardWithoutAPictureFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"defaults": {"hold": 0.5, "join": {"type": "fade", "duration": 1}, "zoomSpan": 0.25, "ease": "linear"},
		"timing": {"policy": "fixed-clip", "clip": 5},
		"slots": [
			{"card": {"title": "Greenhouse", "subtitle": "a short film"}, "mode": "hold"},
			{"src": "a.jpg", "mode": "hold"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	d.Dir = dir
	p, err := Resolve(context.Background(), d, fakeImages(dir))
	if err != nil {
		t.Fatal(err)
	}
	if p.Slots[0].Card == nil || p.Slots[0].Card.Title != "Greenhouse" || p.Slots[0].Src != "" {
		t.Fatalf("card %+v", p.Slots[0])
	}
	if math.Abs(p.Slots[0].Fill-1) > 1e-9 {
		t.Fatalf("fill %v", p.Slots[0].Fill)
	}
	if p.Slots[1].Src == "" {
		t.Fatal("picture lost src")
	}
}
