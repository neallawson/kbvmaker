package plan

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kbvmaker/v1/doc"
	"kbvmaker/v1/media"
	"kbvmaker/v1/rule"
)

func TestResolveFixedClipWithoutAudio(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"defaults": {"hold": 0.5, "join": {"type": "fade", "duration": 1}, "zoomSpan": 0.25, "ease": "linear"},
		"timing": {"policy": "fixed-clip", "clip": 5},
		"slots": [
			{"src": "a.jpg", "mode": "hold", "dur": 8},
			{"src": "b.jpg", "mode": "hold"}
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
	if len(p.Tracks) != 0 || p.Song != 0 {
		t.Fatalf("tracks %d song %v", len(p.Tracks), p.Song)
	}
	if math.Abs(p.Duration-12) > 1e-9 {
		t.Fatalf("duration %v", p.Duration)
	}
	if math.Abs(p.Slots[0].ClipDur-8) > 1e-9 || math.Abs(p.Slots[1].ClipDur-5) > 1e-9 {
		t.Fatalf("clips %v %v", p.Slots[0].ClipDur, p.Slots[1].ClipDur)
	}
}

func TestResolveFilmLengthWithoutAudio(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"defaults": {"hold": 0.5, "join": {"type": "fade", "duration": 1}, "zoomSpan": 0.25, "ease": "linear"},
		"timing": {"policy": "fit-audio", "duration": 9},
		"slots": [
			{"src": "a.jpg", "mode": "hold"},
			{"src": "b.jpg", "mode": "hold"}
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
	if math.Abs(p.Duration-9) > 1e-9 {
		t.Fatalf("duration %v", p.Duration)
	}
	if math.Abs(p.Slots[0].ClipDur-5) > 1e-9 || math.Abs(p.Slots[1].ClipDur-5) > 1e-9 {
		t.Fatalf("clips %v %v", p.Slots[0].ClipDur, p.Slots[1].ClipDur)
	}
}

func TestResolveFitAudioWithoutAClock(t *testing.T) {
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"slots": [{"src": "a.jpg", "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(context.Background(), d, &media.Fake{})
	re, ok := rule.As(err)
	if !ok || re.Code != "missing-field" || re.Msg != "tracks" {
		t.Fatalf("error %+v", err)
	}
}

func fakeImages(dir string) *media.Fake {
	info := media.Info{Width: 400, Height: 300}
	return &media.Fake{Images: map[string]media.Info{
		filepath.Join(dir, "a.jpg"): info,
		filepath.Join(dir, "b.jpg"): info,
	}}
}
