package plan

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kbvmaker/v1/doc"
)

func TestTracksPlaceSilenceOnTheSongClock(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "one.mp3", "two.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := doc.Decode([]byte(`{
	  "montage": 1,
	  "tracks": [{"src": "one.mp3"}, {"src": "two.mp3", "gap": 4}],
	  "slots": [
	    {"src": "a.jpg", "mode": "hold"},
	    {"src": "b.jpg", "mode": "hold"}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	d.Dir = dir
	fake := fakeImages(dir)
	fake.Durations = map[string]float64{
		filepath.Join(dir, "one.mp3"): 10,
		filepath.Join(dir, "two.mp3"): 20,
	}
	p, err := Resolve(context.Background(), d, fake)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.Song-34) > 1e-9 || math.Abs(p.Duration-34) > 1e-9 {
		t.Fatalf("song %v duration %v", p.Song, p.Duration)
	}
	if len(p.Tracks) != 2 {
		t.Fatalf("tracks %d", len(p.Tracks))
	}
	if p.Tracks[0].Src != "one.mp3" || p.Tracks[0].Start != 0 || p.Tracks[0].Dur != 10 || p.Tracks[0].Gap != 0 {
		t.Fatalf("first %+v", p.Tracks[0])
	}
	if p.Tracks[1].Src != "two.mp3" || p.Tracks[1].Start != 14 || p.Tracks[1].Dur != 20 || p.Tracks[1].Gap != 4 {
		t.Fatalf("second %+v", p.Tracks[1])
	}
}
