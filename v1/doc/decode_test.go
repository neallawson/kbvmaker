package doc

import (
	"strings"
	"testing"

	"neal/kbvmaker/v1/rule"
)

const twoSlots = `{
  "montage": 1,
  "audio": "Love_Ruins-Greenhouse.mp3",
  "slots": [
    { "src": "Greenhouse3.jpg", "mode": "out-center" },
    { "src": "Greenhouse8.jpg", "mode": "in-left-right" }
  ]
}`

func TestDecodeDefaults(t *testing.T) {
	d, err := Decode([]byte(twoSlots))
	if err != nil {
		t.Fatal(err)
	}
	if d.Montage != 1 || d.Canvas.Width != 3840 || d.Canvas.Backdrop.Kind != "blur" || d.Canvas.Backdrop.Sigma() != 40 {
		t.Fatalf("defaults %+v", d.Canvas)
	}
	if d.Timing.Policy != "fit-audio" || d.Defaults.Hold != 1 || d.Defaults.Join.Duration != 1.5 {
		t.Fatalf("timing %+v defaults %+v", d.Timing, d.Defaults)
	}
	if len(d.Slots) != 2 || d.Slots[0].Mode != "out-center" {
		t.Fatalf("slots %+v", d.Slots)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnknownFieldAndWrongVersion(t *testing.T) {
	_, err := Decode([]byte(`{"montage":1,"audio":"a.mp3","slots":[],"nope":true}`))
	re, ok := rule.As(err)
	if !ok || re.Code != "unknown-field" {
		t.Fatalf("unknown: %v", err)
	}
	_, err = Decode([]byte(`{"montage":2,"audio":"a.mp3"}`))
	re, ok = rule.As(err)
	if !ok || re.Code != "bad-number" {
		t.Fatalf("version: %v", err)
	}
}

func TestRejectsDuplicateKey(t *testing.T) {
	_, err := Decode([]byte(`{"montage":1,"montage":1,"audio":"a.mp3"}`))
	re, ok := rule.As(err)
	if !ok || re.Code != "duplicate-key" {
		t.Fatalf("dup: %v", err)
	}
}

func TestMotionRoundTrip(t *testing.T) {
	in := `{
	  "montage": 1,
	  "audio": "song.mp3",
	  "slots": [{
	    "src": "portrait.jpg",
	    "basedOn": "in-left-right",
	    "motion": [
	      {"from": {"anchor": "opaque-start"}, "dur": 1.0, "zoom": "full", "pan": "center"},
	      {"dur": "rest", "zoom": {"from": "full", "to": {"min": ["fill", "limit"]}}, "pan": {"from": "left", "to": "right"}}
	    ]
	  }]
	}`
	d, err := Decode([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	seg := d.Slots[0].Motion
	if len(seg) != 2 || !seg[1].DurRest || seg[1].Zoom.From == nil || seg[1].Pan.From.Name != "left" {
		t.Fatalf("motion %+v", seg)
	}
	if strings.Contains(seg[0].Zoom.End.Atoms[0].Sym, "nope") {
		t.Fatal("unexpected")
	}
}

func TestBackdropKinds(t *testing.T) {
	d, err := Decode([]byte(`{
	  "montage": 1,
	  "audio": "song.mp3",
	  "canvas": {"backdrop": {"kind": "color", "color": "#0000FF"}},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "backdrop": {"kind": "pattern", "name": "grain"}},
	    {"src": "b.jpg", "mode": "hold"}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Canvas.Backdrop.Kind != "color" || d.Canvas.Backdrop.Color != "#0000FF" {
		t.Fatalf("canvas %+v", d.Canvas.Backdrop)
	}
	if d.Slots[0].Backdrop == nil || d.Slots[0].Backdrop.Name != "grain" || d.Slots[0].Backdrop.Color != "#000000" {
		t.Fatalf("slot %+v", d.Slots[0].Backdrop)
	}
	if d.Slots[1].Backdrop != nil {
		t.Fatal("second slot should inherit the canvas backdrop")
	}
	_, err = Decode([]byte(`{"montage":1,"audio":"a.mp3","canvas":{"blur":40,"backdrop":{"kind":"blur"}}}`))
	re, ok := rule.As(err)
	if !ok || re.Code != "unknown-field" {
		t.Fatalf("both: %v", err)
	}
	_, err = Decode([]byte(`{"montage":1,"audio":"a.mp3","canvas":{"backdrop":{"kind":"pattern","name":"stripes"}}}`))
	re, ok = rule.As(err)
	if !ok || re.Code != "unknown-field" {
		t.Fatalf("pattern: %v", err)
	}
}
