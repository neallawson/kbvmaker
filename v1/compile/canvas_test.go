package compile

import (
	"strings"
	"testing"

	"neal/kbvmaker/v1/model"
)

func TestCanvasFilters(t *testing.T) {
	canvas := model.Canvas{Width: 320, Height: 180, Backdrop: model.Backdrop{Kind: "blur", Blur: 40}}
	p := &model.Plan{
		Audio: "song.mp3", OutputFile: "out.mp4", Canvas: canvas,
		Output: model.Output{Width: 160, Height: 90, FPS: 10, VideoCodec: "libx264", Preset: "ultrafast", CRF: 30, AudioCodec: "aac", AudioBitrate: 128000},
		Slots: []model.Slot{
			holdSlot("a.jpg", model.Backdrop{Kind: "color", Color: "#0000FF"}),
			holdSlot("a.jpg", model.Backdrop{Kind: "blur", Blur: 40}),
			holdSlot("b.jpg", model.Backdrop{Kind: "pattern", Name: "gradient", Color: "#FFFFFF"}),
		},
	}
	p.Slots[0].Join = &model.Join{Type: "fade", Duration: 0.2}
	p.Slots[1].Join = &model.Join{Type: "fade", Duration: 0.2}
	cmds, err := Commands(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var canvases []string
	for _, c := range cmds {
		if c.Stage == "canvas" {
			canvases = append(canvases, strings.Join(c.Argv, " "))
		}
	}
	if len(canvases) != 3 {
		t.Fatalf("canvases %d: %v", len(canvases), canvases)
	}
	if !strings.Contains(canvases[0], "color=c=0x0000FF:s=320x180") {
		t.Fatalf("color filter %s", canvases[0])
	}
	if !strings.Contains(canvases[1], "gblur=sigma=40") {
		t.Fatalf("blur filter %s", canvases[1])
	}
	if !strings.Contains(canvases[2], "gradients=") || !strings.Contains(canvases[2], "speed=0") {
		t.Fatalf("gradient filter %s", canvases[2])
	}
}

func holdSlot(src string, b model.Backdrop) model.Slot {
	return model.Slot{
		Src: src, ClipDur: 1, Backdrop: b,
		Spans: []model.Span{{From: 0, To: 1, Z0: 1, Z1: 1, U0: 0.5, U1: 0.5, V0: 0.5, V1: 0.5}},
	}
}
