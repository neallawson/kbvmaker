package compile

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"kbvmaker/v1/doc"
	"kbvmaker/v1/model"
)

func TestCardStageWritesAPNGTheClipReads(t *testing.T) {
	c := doc.Card{Title: "Greenhouse"}
	p := &model.Plan{
		OutputFile: "out.mp4",
		Canvas:     model.Canvas{Width: 64, Height: 36, Backdrop: model.Backdrop{Kind: "blur", Blur: 40}},
		Output:     model.Output{Width: 64, Height: 36, FPS: 10, VideoCodec: "libx264", Preset: "ultrafast", CRF: 30},
		Slots: []model.Slot{
			{
				Index: 1, Card: &c, ClipDur: 2, Backdrop: model.Backdrop{Kind: "blur", Blur: 40},
				Spans: []model.Span{{From: 0, To: 2, Z0: 1, Z1: 1, U0: 0.5, U1: 0.5, V0: 0.5, V1: 0.5}},
			},
		},
	}
	cmds, err := Commands(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var cardCmd *Command
	var clip string
	for i := range cmds {
		if cmds[i].Stage == "card" {
			cardCmd = &cmds[i]
		}
		if cmds[i].Stage == "canvas" {
			t.Fatal("card used the photograph canvas")
		}
		if cmds[i].Stage == "clip" {
			clip = strings.Join(cmds[i].Argv, " ")
		}
	}
	if cardCmd == nil || cardCmd.File != "work/card-0.png" || len(cardCmd.Body) == 0 {
		t.Fatalf("card %+v", cardCmd)
	}
	img, err := png.Decode(bytes.NewReader(cardCmd.Body))
	if err != nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 36 {
		t.Fatalf("png %v %v", err, img)
	}
	if !strings.Contains(clip, "-i work/card-0.png") {
		t.Fatalf("clip %s", clip)
	}
}

func TestIdenticalCardsShareOnePNG(t *testing.T) {
	c := doc.Card{Title: "Title"}
	p := &model.Plan{
		OutputFile: "out.mp4",
		Canvas:     model.Canvas{Width: 64, Height: 36, Backdrop: model.Backdrop{Kind: "color", Color: "#000000"}},
		Output:     model.Output{Width: 64, Height: 36, FPS: 10, VideoCodec: "libx264", Preset: "ultrafast", CRF: 30},
		Slots: []model.Slot{
			holdCard(c), holdCard(c),
		},
	}
	p.Slots[0].Join = &model.Join{Type: "fade", Duration: 0.2}
	p.Slots[0].Index = 1
	p.Slots[1].Index = 2
	cmds, err := Commands(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cmds {
		if c.Stage == "card" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("card stages %d", n)
	}
}

func holdCard(c doc.Card) model.Slot {
	cp := c
	return model.Slot{
		Card: &cp, ClipDur: 1, Backdrop: model.Backdrop{Kind: "color", Color: "#000000"},
		Spans: []model.Span{{From: 0, To: 1, Z0: 1, Z1: 1, U0: 0.5, U1: 0.5, V0: 0.5, V1: 0.5}},
	}
}
