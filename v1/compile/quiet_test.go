package compile

import (
	"strings"
	"testing"

	"neal/kbvmaker/v1/model"
)

func quietPlan() *model.Plan {
	p := &model.Plan{
		Audio: "song.mp3", OutputFile: "out.mp4",
		Canvas: model.Canvas{Width: 320, Height: 180, Backdrop: model.Backdrop{Kind: "blur", Blur: 40}},
		Output: model.Output{Width: 160, Height: 90, FPS: 10, VideoCodec: "libx264", Preset: "ultrafast", CRF: 30, AudioCodec: "aac", AudioBitrate: 128000},
		Slots: []model.Slot{
			holdSlot("a.jpg", model.Backdrop{Kind: "blur", Blur: 40}),
			holdSlot("b.jpg", model.Backdrop{Kind: "blur", Blur: 40}),
		},
	}
	p.Slots[0].Join = &model.Join{Type: "fade", Duration: 0.2}
	return p
}

func TestFFmpegPrintsOnlyErrorsAndProgress(t *testing.T) {
	cmds, err := Commands(quietPlan(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		if c.Stage == "filters" || c.Stage == "probe" {
			continue
		}
		line := strings.Join(c.Argv, " ")
		if !strings.HasPrefix(line, "ffmpeg -hide_banner -loglevel error -stats -y ") {
			t.Fatalf("%s: %s", c.Stage, line)
		}
	}
}

func TestVerboseKeepsFFmpegOutput(t *testing.T) {
	cmds, err := Commands(quietPlan(), Options{Verbose: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		line := strings.Join(c.Argv, " ")
		if strings.Contains(line, "-loglevel") {
			t.Fatalf("%s: %s", c.Stage, line)
		}
	}
}
