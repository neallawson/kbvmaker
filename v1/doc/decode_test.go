package doc

import (
	"strings"
	"testing"

	"kbvmaker/v1/rule"
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

func TestMoveList(t *testing.T) {
	d, err := Decode([]byte(`{
		"montage": 1,
		"audio": "a.mp3",
		"moves": ["in-center", "in-right-left", "in-bottom-top"],
		"slots": [{ "src": "a.jpg", "mode": "in-right-left" }]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(d.Moves) != 3 || d.Moves[1] != "in-right-left" || d.Moves[2] != "in-bottom-top" {
		t.Fatalf("moves %v", d.Moves)
	}

	d, err = Decode([]byte(`{"montage":1,"audio":"a.mp3","moves":["nope"]}`))
	if err != nil {
		t.Fatal(err)
	}
	err = d.ValidateShape()
	re, ok := rule.As(err)
	if !ok || re.Code != "mode" || !strings.Contains(re.Msg, "unknown mode") {
		t.Fatalf("unknown move: %v", err)
	}

	d, err = Decode([]byte(`{"montage":1,"audio":"a.mp3","moves":["hold","hold"]}`))
	if err != nil {
		t.Fatal(err)
	}
	err = d.ValidateShape()
	re, ok = rule.As(err)
	if !ok || re.Code != "mode" || !strings.Contains(re.Msg, "twice") {
		t.Fatalf("duplicate move: %v", err)
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

func TestDraftOmitsSongAndPictures(t *testing.T) {
	d, err := Decode([]byte(`{"montage":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ValidateShape(); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected a draft to be unready")
	}
}

func TestFitAudioRejectsAPinOnEveryPicture(t *testing.T) {
	d, err := Decode([]byte(`{
	  "montage": 1,
	  "audio": "song.mp3",
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "dur": 8},
	    {"src": "b.jpg", "mode": "hold", "dur": 8}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = d.Validate()
	re, ok := rule.As(err)
	if !ok || re.Code != "bad-time" || re.Msg != "leave one picture sharing the song" {
		t.Fatalf("pins: %v", err)
	}
}

func TestSilentClocks(t *testing.T) {
	fixed, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "dur": 8},
	    {"src": "b.jpg", "mode": "hold"}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := fixed.Validate(); err != nil {
		t.Fatal(err)
	}
	locked, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fit-audio", "duration": 41},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold"},
	    {"src": "b.jpg", "mode": "hold", "dur": 8}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := locked.Validate(); err != nil {
		t.Fatal(err)
	}
	if !locked.Timing.HasDuration || locked.Timing.Duration != 41 {
		t.Fatalf("duration %+v", locked.Timing)
	}
	body, err := locked.Timing.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"policy":"fit-audio","duration":41}` {
		t.Fatalf("timing json %s", body)
	}
	both, err := Decode([]byte(`{
	  "montage": 1,
	  "audio": "song.mp3",
	  "timing": {"policy": "fit-audio", "duration": 41},
	  "slots": [{"src": "a.jpg", "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = both.Validate()
	re, ok := rule.As(err)
	if !ok || re.Msg != "a soundtrack and a film length cannot both set the clock" {
		t.Fatalf("both clocks: %v", err)
	}
	filmPins, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fit-audio", "duration": 20},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "dur": 8},
	    {"src": "b.jpg", "mode": "hold", "dur": 8}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = filmPins.Validate()
	re, ok = rule.As(err)
	if !ok || re.Msg != "leave one picture sharing the film" {
		t.Fatalf("film pins: %v", err)
	}
	needsSong, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fit-audio"},
	  "slots": [{"src": "a.jpg", "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = needsSong.Validate()
	re, ok = rule.As(err)
	if !ok || re.Code != "missing-field" || re.Msg != "audio" {
		t.Fatalf("needs song: %v", err)
	}
}

func TestFixedSlot(t *testing.T) {
	d, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "fixed": true},
	    {"card": {"title": "End"}, "mode": "hold", "fixed": true}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if !d.Slots[0].Fixed || !d.Slots[1].Fixed {
		t.Fatalf("fixed %+v %+v", d.Slots[0].Fixed, d.Slots[1].Fixed)
	}

	open, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"src": "a.jpg", "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if open.Slots[0].Fixed {
		t.Fatal("omitted fixed was set")
	}

	_, err = Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"src": "a.jpg", "mode": "hold", "fixed": "yes"}]
	}`))
	re, ok := rule.As(err)
	if !ok || re.Code != "bad-number" {
		t.Fatalf("bad fixed: %v", err)
	}
}

func TestDecodeCard(t *testing.T) {
	d, err := Decode([]byte(`{
	  "montage": 1,
	  "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [
	    {"card": {"title": "Greenhouse", "subtitle": "a short film"}, "mode": "hold"},
	    {"src": "a.jpg", "mode": "hold"}
	  ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.Slots[0].Card == nil || d.Slots[0].Card.Title != "Greenhouse" || d.Slots[0].Src != "" {
		t.Fatalf("card %+v", d.Slots[0])
	}
}

func TestCardRules(t *testing.T) {
	both, err := Decode([]byte(`{
	  "montage": 1, "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"src": "a.jpg", "card": {"title": "X"}, "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = both.Validate()
	re, ok := rule.As(err)
	if !ok || re.Code != "unknown-field" {
		t.Fatalf("both: %v", err)
	}

	empty, err := Decode([]byte(`{
	  "montage": 1, "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"card": {"subtitle": "no title"}, "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = empty.Validate()
	re, ok = rule.As(err)
	if !ok || re.Code != "missing-field" {
		t.Fatalf("empty card: %v", err)
	}

	credits, err := Decode([]byte(`{
	  "montage": 1, "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"card": {"credits": ["Ada — Camera"]}, "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := credits.Validate(); err != nil {
		t.Fatal(err)
	}

	face, err := Decode([]byte(`{
	  "montage": 1, "timing": {"policy": "fixed-clip", "clip": 5},
	  "slots": [{"card": {"title": "X", "face": "comic"}, "mode": "hold"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	err = face.Validate()
	re, ok = rule.As(err)
	if !ok || re.Code != "unknown-field" {
		t.Fatalf("face: %v", err)
	}
}
