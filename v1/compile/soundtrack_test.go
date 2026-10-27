package compile

import (
	"strings"
	"testing"

	"kbvmaker/v1/model"
)

func TestOneTrackIsMuxedDirectly(t *testing.T) {
	cmds, err := Commands(quietPlan(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		if c.Stage == "soundtrack" {
			t.Fatal("a single file with no silence was mixed")
		}
	}
	mux := strings.Join(stage(cmds, "mux"), " ")
	if !strings.Contains(mux, "song.mp3") {
		t.Fatalf("mux %s", mux)
	}
}

func TestSoundtrackMixesFilesAndSilence(t *testing.T) {
	p := quietPlan()
	p.Tracks = []model.Track{
		{Src: "one.mp3", Dur: 10},
		{Src: "two.mp3", Start: 14, Dur: 20, Gap: 4},
	}
	cmds, err := Commands(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	mix := strings.Join(stage(cmds, "soundtrack"), " ")
	if !strings.Contains(mix, "-i one.mp3") || !strings.Contains(mix, "-i two.mp3") || !strings.Contains(mix, "anullsrc") || !strings.Contains(mix, "atrim=0:4") {
		t.Fatalf("mix %s", mix)
	}
	mux := strings.Join(stage(cmds, "mux"), " ")
	if !strings.Contains(mux, "soundtrack.wav") || strings.Contains(mux, "one.mp3") {
		t.Fatalf("mux %s", mux)
	}
}

func stage(cmds []Command, name string) []string {
	for _, c := range cmds {
		if c.Stage == name {
			return c.Argv
		}
	}
	return nil
}
