package export

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"kbvmaker/v1/compile"
)

func TestScriptWritesCardBytesThenRunsFFmpeg(t *testing.T) {
	script := Script([]compile.Command{
		{Stage: "card", File: "work/card-0.png", Body: []byte("png-bytes")},
		{Stage: "clip", Argv: []string{"ffmpeg", "-i", "work/card-0.png", "work/clip-000.mp4"}},
	})
	if !strings.Contains(script, "base64 -d >'work/card-0.png'") {
		t.Fatalf("script %s", script)
	}
	if !strings.Contains(script, base64.StdEncoding.EncodeToString([]byte("png-bytes"))) {
		t.Fatalf("missing payload %s", script)
	}
	if !strings.Contains(script, "'ffmpeg' '-i' 'work/card-0.png'") {
		t.Fatalf("clip %s", script)
	}
}

func TestPlanJSONPutsCardBytesInFiles(t *testing.T) {
	raw, err := PlanJSON([]compile.Command{
		{Stage: "card", File: "work/card-0.png", Body: []byte("png-bytes")},
		{Stage: "clip", Argv: []string{"ffmpeg", "-i", "work/card-0.png", "out.mp4"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var file PlanFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if file.RenderPlan != 1 || len(file.Commands) != 1 || file.Files["work/card-0.png"] == "" {
		t.Fatalf("%s", raw)
	}
	got, err := base64.StdEncoding.DecodeString(file.Files["work/card-0.png"])
	if err != nil || string(got) != "png-bytes" {
		t.Fatalf("files %v %v", file.Files, err)
	}
}
