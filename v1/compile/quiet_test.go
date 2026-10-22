package compile

import (
	"strings"
	"testing"

	"kbvmaker/v1/model"
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

func TestMuxOmitsAudioWhenTheFilmHasNone(t *testing.T) {
	p := quietPlan()
	p.Audio = ""
	cmds, err := Commands(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		if c.Stage == "probe" {
			t.Fatalf("probe %v", c.Argv)
		}
	}
	mux := strings.Join(stageArgv(cmds, "mux"), " ")
	if !strings.Contains(mux, "-an") || strings.Contains(mux, "song.mp3") || strings.Contains(mux, "-shortest") {
		t.Fatalf("mux %s", mux)
	}
	withSong := quietPlan()
	cmds, err = Commands(withSong, Options{})
	if err != nil {
		t.Fatal(err)
	}
	mux = strings.Join(stageArgv(cmds, "mux"), " ")
	if !strings.Contains(mux, "-i song.mp3") || !strings.Contains(mux, "-shortest") {
		t.Fatalf("mux %s", mux)
	}
}

func stageArgv(cmds []Command, stage string) []string {
	for _, c := range cmds {
		if c.Stage == stage {
			return c.Argv
		}
	}
	return nil
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
