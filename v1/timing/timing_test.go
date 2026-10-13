package timing

import (
	"math"
	"testing"

	"kbvmaker/v1/doc"
)

func defaults() doc.Defaults {
	return doc.Defaults{Join: doc.Join{Type: "fade", Duration: 1.5}, Hold: 1, ZoomSpan: 0.4, Ease: "linear"}
}

func TestFitAudioTwoClips(t *testing.T) {
	d := &doc.Document{
		Timing:   doc.Timing{Policy: "fit-audio"},
		Defaults: defaults(),
		Output:   doc.Output{FPS: 30},
		Slots: []doc.Slot{
			{Src: "a.jpg", Mode: "out-center"},
			{Src: "b.jpg", Mode: "in-left-right"},
		},
	}
	w, dur, err := Resolve(d, d.Slots, 20)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(dur-20) > 1e-9 {
		t.Fatalf("duration %v", dur)
	}
	if math.Abs(w[0].ClipDur-10.75) > 1e-9 || math.Abs(w[1].ClipDur-10.75) > 1e-9 {
		t.Fatalf("clip durs %v %v", w[0].ClipDur, w[1].ClipDur)
	}
	if math.Abs(w[0].Join.Start-9.25) > 1e-9 {
		t.Fatalf("fade start %v", w[0].Join.Start)
	}
	if math.Abs(w[0].OpaqueEnd-9.25) > 1e-9 {
		t.Fatalf("first opaque end %v", w[0].OpaqueEnd)
	}
	if math.Abs(w[1].OpaqueStart-1.5) > 1e-9 || math.Abs(w[1].OpaqueEnd-10.75) > 1e-9 {
		t.Fatalf("second opaque %v..%v", w[1].OpaqueStart, w[1].OpaqueEnd)
	}
}

func TestFitAudioPinSharesTheRest(t *testing.T) {
	pin := 8.0
	d := &doc.Document{
		Timing:   doc.Timing{Policy: "fit-audio"},
		Defaults: defaults(),
		Output:   doc.Output{FPS: 30},
		Slots: []doc.Slot{
			{Src: "a.jpg", Mode: "in-center", Dur: &pin},
			{Src: "b.jpg", Mode: "out-center"},
			{Src: "c.jpg", Mode: "hold"},
		},
	}
	w, dur, err := Resolve(d, d.Slots, 20)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(dur-20) > 1e-9 {
		t.Fatalf("duration %v", dur)
	}
	if math.Abs(w[0].ClipDur-8) > 1e-9 || math.Abs(w[1].ClipDur-7.5) > 1e-9 || math.Abs(w[2].ClipDur-7.5) > 1e-9 {
		t.Fatalf("clips %v %v %v", w[0].ClipDur, w[1].ClipDur, w[2].ClipDur)
	}
}

func TestFitAudioPinMustLeaveAShare(t *testing.T) {
	pin := 20.0
	d := &doc.Document{
		Timing:   doc.Timing{Policy: "fit-audio"},
		Defaults: defaults(),
		Slots: []doc.Slot{
			{Src: "a.jpg", Mode: "hold", Dur: &pin},
			{Src: "b.jpg", Mode: "hold"},
		},
	}
	_, _, err := Resolve(d, d.Slots, 20)
	if err == nil {
		t.Fatal("expected the remaining picture to be too short")
	}
}

func TestMarkers(t *testing.T) {
	pct := doc.TimePoint{Kind: "pct", Pct: 18, Of: "song"}
	sec := doc.TimePoint{Kind: "sec", Sec: 92.4, Of: "song"}
	d := &doc.Document{
		Timing:   doc.Timing{Policy: "markers"},
		Defaults: defaults(),
		Output:   doc.Output{FPS: 30},
		Slots: []doc.Slot{
			{Src: "a.jpg", Mode: "in-center", Until: &pct},
			{Src: "b.jpg", Mode: "out-center", Until: &sec},
			{Src: "c.jpg", Mode: "hold"},
		},
	}
	w, dur, err := Resolve(d, d.Slots, 100)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(dur-100) > 1e-6 {
		t.Fatalf("duration %v", dur)
	}
	if math.Abs(w[0].ClipDur-(18+0.75)) > 1e-6 {
		t.Fatalf("first dur %v", w[0].ClipDur)
	}
	// Middle: start 18-0.75, end 92.4+0.75.
	wantMid := (92.4 + 0.75) - (18 - 0.75)
	if math.Abs(w[1].ClipDur-wantMid) > 1e-6 {
		t.Fatalf("middle dur %v want %v", w[1].ClipDur, wantMid)
	}
	if math.Abs(w[0].Join.Mid-18) > 1e-6 {
		t.Fatalf("first cut midpoint %v", w[0].Join.Mid)
	}
}

func TestShortMarkerGap(t *testing.T) {
	a := doc.TimePoint{Kind: "pct", Pct: 40, Of: "song"}
	b := doc.TimePoint{Kind: "pct", Pct: 43, Of: "song"}
	d := &doc.Document{
		Timing:   doc.Timing{Policy: "markers"},
		Defaults: defaults(),
		Output:   doc.Output{FPS: 30},
		Slots: []doc.Slot{
			{Src: "a.jpg", Mode: "hold", Until: &a},
			{Src: "b.jpg", Mode: "hold", Until: &b},
			{Src: "c.jpg", Mode: "hold"},
		},
	}
	w, _, err := Resolve(d, d.Slots, 100)
	if err != nil {
		t.Fatal(err)
	}
	// 40% to 43% of 100s is 3s, join is 1.5s, so the middle picture is briefly opaque.
	if w[1].OpaqueEnd <= w[1].OpaqueStart {
		t.Fatalf("opaque %v..%v", w[1].OpaqueStart, w[1].OpaqueEnd)
	}
}
