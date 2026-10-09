package clock

import (
	"math"
	"testing"

	"neal/kbvmaker/v1/doc"
)

func TestSongPercentAndNegativeSecond(t *testing.T) {
	got, err := Song(doc.TimePoint{Kind: "pct", Pct: 18, Of: "song"}, 100, 30, 1)
	if err != nil || math.Abs(got-18) > 1e-12 {
		t.Fatalf("pct: %v %v", got, err)
	}
	got, err = Song(doc.TimePoint{Kind: "sec", Sec: -2, Of: "song"}, 100, 30, 1)
	if err != nil || math.Abs(got-98) > 1e-12 {
		t.Fatalf("negative sec: %v %v", got, err)
	}
	_, err = Song(doc.TimePoint{Kind: "anchor", Anchor: "opaque-start"}, 100, 30, 1)
	if err == nil {
		t.Fatal("opaque anchor accepted as a song marker")
	}
}

func TestTimecodeAndFrame(t *testing.T) {
	got, err := Song(doc.TimePoint{Kind: "timecode", Timecode: "00:01:02.5"}, 1000, 30, 0)
	if err != nil || math.Abs(got-62.5) > 1e-12 {
		t.Fatalf("timecode: %v %v", got, err)
	}
	got, err = Song(doc.TimePoint{Kind: "frame", Frame: 30}, 1000, 30, 0)
	if err != nil || math.Abs(got-1) > 1e-12 {
		t.Fatalf("frame: %v %v", got, err)
	}
}

func TestOpaqueAnchor(t *testing.T) {
	p := Place{Song: 20, ClipStart: 9.25, ClipDur: 10.75, OpaqueStart: 1.5, OpaqueEnd: 10.75, FPS: 30}
	got, err := Clip(doc.TimePoint{Kind: "anchor", Anchor: "opaque-start", Offset: 1, HasOffset: true}, p, 2)
	if err != nil || math.Abs(got-2.5) > 1e-12 {
		t.Fatalf("opaque-start+1s: %v %v", got, err)
	}
	got, err = Clip(doc.TimePoint{Kind: "pct", Pct: 50, Of: "opaque"}, p, 2)
	// Opaque length is 9.25, halfway is 1.5+4.625 = 6.125.
	if err != nil || math.Abs(got-(1.5+9.25/2)) > 1e-12 {
		t.Fatalf("opaque percent: %v %v", got, err)
	}
}

func TestSongPointMustFallInsideSlot(t *testing.T) {
	p := Place{Song: 20, ClipStart: 9.25, ClipDur: 10.75, OpaqueStart: 1.5, OpaqueEnd: 10.75, FPS: 30}
	_, err := Clip(doc.TimePoint{Kind: "sec", Sec: 1, Of: "song"}, p, 2)
	if err == nil {
		t.Fatal("song time before the slot was accepted")
	}
	got, err := Clip(doc.TimePoint{Kind: "sec", Sec: 10, Of: "song"}, p, 2)
	if err != nil || math.Abs(got-(10-9.25)) > 1e-12 {
		t.Fatalf("song time inside slot: %v %v", got, err)
	}
}
