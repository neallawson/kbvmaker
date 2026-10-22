package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kbvmaker/v1"
	"kbvmaker/v1/media"
)

// These tests ask a real ffmpeg whether the graphs we compile are legal.
// They stay short on purpose: one frame per backdrop, one brief clip per move,
// one dissolve, and one probed file. Document rules stay in the unit tests.

func TestFFmpegParsesEveryBackdrop(t *testing.T) {
	probe := needFFmpeg(t)
	cases := []struct {
		name   string
		canvas string
	}{
		{"blur", `"blur": 2`},
		{"color", `"backdrop": {"kind": "color", "color": "#0000FF"}`},
		{"grain", `"backdrop": {"kind": "pattern", "name": "grain", "color": "#202020"}`},
		{"gradient", `"backdrop": {"kind": "pattern", "name": "gradient", "color": "#FFFFFF"}`},
		{"image", `"backdrop": {"kind": "image", "src": "bars.png"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := smokeMedia(t)
			cmds := compileSmoke(t, probe, dir, smokeBody(tc.canvas, "", smokeSlot("hold"), ""))
			runStages(t, dir, cmds, "canvas")
			img := decodeStagePNG(t, dir, cmds, "canvas")
			if img.Bounds().Dx() != 160 || img.Bounds().Dy() != 90 {
				t.Fatalf("canvas %v", img.Bounds())
			}
		})
	}
}

func TestFFmpegParsesEveryMove(t *testing.T) {
	probe := needFFmpeg(t)
	cases := []struct {
		name  string
		modes string
		slot  string
	}{
		{"hold", "", smokeSlot("hold")},
		{"in-center", "", smokeSlot("in-center")},
		{"out-center", "", smokeSlot("out-center")},
		{"in-left-right", "", smokeSlot("in-left-right")},
		{"in-right-left", "", smokeSlot("in-right-left")},
		{"in-top-bottom", "", smokeSlot("in-top-bottom")},
		{"in-bottom-top", "", smokeSlot("in-bottom-top")},
		{
			"scan",
			`, "modes": {"drift": {"family": "scan", "motion": [{"dur": "rest", "zoom": "fill", "pan": {"from": "left", "to": "right"}}]}}`,
			`{"src": "photo.png", "mode": "drift"}`,
		},
		{
			"motion",
			"",
			`{"src": "photo.png", "motion": [{"dur": "rest", "zoom": "full", "pan": {"from": "left", "to": "right"}}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := smokeMedia(t)
			cmds := compileSmoke(t, probe, dir, smokeBody(`"blur": 2`, tc.modes, tc.slot, ""))
			runStages(t, dir, cmds, "canvas", "clip")
			streams := probeFile(t, filepath.Join(dir, stageOutput(cmds, "clip")))
			if streams.video().PixFmt == "" || streams.video().CodecName == "" {
				t.Fatalf("clip streams %+v", streams.Streams)
			}
		})
	}
}

func TestFFmpegParsesADissolve(t *testing.T) {
	probe := needFFmpeg(t)
	dir := smokeMedia(t)
	slots := smokeSlot("hold") + ", " + smokeSlot("hold")
	cmds := compileSmoke(t, probe, dir, smokeBody(`"blur": 2`, "", slots, ""))
	runStages(t, dir, cmds, "canvas", "clip", "join")
	if probeFile(t, filepath.Join(dir, stageOutput(cmds, "join"))).video().CodecName == "" {
		t.Fatal("join produced no video stream")
	}
}

func TestFFmpegHonorsOutputKnobs(t *testing.T) {
	probe := needFFmpeg(t)
	dir := smokeMedia(t)
	output := `"width": 80, "height": 48, "fps": 8, "videoCodec": "libx264", "preset": "ultrafast", "crf": 32, "pixFmt": "yuv422p", "audioCodec": "aac", "audioBitrate": 64000, "file": "out.mp4"`
	body := smokeBody(`"blur": 2`, "", smokeSlot("hold")+", "+smokeSlot("hold"), output)
	path := filepath.Join(dir, "piece.montage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	eng := v1.New(probe)
	p, err := eng.Resolve(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(stageArgv(cmds, "join"), " ")
	for _, want := range []string{"-preset ultrafast", "-crf 32", "-pix_fmt yuv422p"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("join argv missing %q\n%s", want, joined)
		}
	}
	muxed := strings.Join(stageArgv(cmds, "mux"), " ")
	for _, want := range []string{"-c:a aac", "-b:a 64000"} {
		if !strings.Contains(muxed, want) {
			t.Fatalf("mux argv missing %q\n%s", want, muxed)
		}
	}
	if err := v1.Run(context.Background(), dir, cmds); err != nil {
		t.Fatal(err)
	}
	streams := probeFile(t, filepath.Join(dir, "out.mp4"))
	if streams.video().PixFmt != "yuv422p" {
		t.Fatalf("pix_fmt %q", streams.video().PixFmt)
	}
	if streams.video().CodecName != "h264" {
		t.Fatalf("video codec %q", streams.video().CodecName)
	}
	if streams.audio().CodecName != "aac" {
		t.Fatalf("audio codec %q", streams.audio().CodecName)
	}
}

func TestFFmpegRendersVideoWithoutAudio(t *testing.T) {
	probe := needFFmpeg(t)
	dir := smokeMedia(t)
	body := fmt.Sprintf(`{
		"montage": 1,
		"canvas": {"width": 160, "height": 90, "blur": 2},
		"output": {"width": 80, "height": 48, "fps": 8, "preset": "ultrafast", "crf": 32, "file": "out.mp4"},
		"defaults": {"hold": 0.1, "join": {"type": "fade", "duration": 0.15}, "zoomSpan": 0.2, "ease": "linear"},
		"timing": {"policy": "fixed-clip", "clip": 0.4},
		"slots": [%s, %s]
	}`, smokeSlot("hold"), smokeSlot("hold"))
	cmds := compileSmoke(t, probe, dir, body)
	mux := strings.Join(stageArgv(cmds, "mux"), " ")
	if !strings.Contains(mux, "-an") || strings.Contains(mux, "tone.wav") {
		t.Fatalf("mux %s", mux)
	}
	if err := v1.Run(context.Background(), dir, cmds); err != nil {
		t.Fatal(err)
	}
	streams := probeFile(t, filepath.Join(dir, "out.mp4"))
	if streams.video().CodecName == "" {
		t.Fatal("silent film produced no video stream")
	}
	if streams.audio().CodecName != "" {
		t.Fatalf("audio codec %q", streams.audio().CodecName)
	}
}

func smokeBody(canvas, modes, slots, output string) string {
	if output == "" {
		output = `"width": 80, "height": 48, "fps": 8, "preset": "ultrafast", "crf": 32, "file": "out.mp4"`
	}
	return fmt.Sprintf(`{
		"montage": 1,
		"canvas": {"width": 160, "height": 90, %s},
		"output": {%s},
		"defaults": {"hold": 0.1, "join": {"type": "fade", "duration": 0.15}, "zoomSpan": 0.2, "ease": "linear"},
		"timing": {"policy": "fixed-clip", "clip": 0.4},
		"audio": "tone.wav"%s,
		"slots": [%s]
	}`, canvas, output, modes, slots)
}

func smokeSlot(mode string) string {
	return fmt.Sprintf(`{"src": "photo.png", "mode": %q}`, mode)
}

func needFFmpeg(t *testing.T) *media.FFProbe {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip(err)
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		t.Skip(err)
	}
	return probe
}

func smokeMedia(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := writePortrait(filepath.Join(dir, "photo.png"), 200, 400); err != nil {
		t.Fatal(err)
	}
	if err := writeSolid(filepath.Join(dir, "bars.png"), 400, 200, color.RGBA{G: 180, A: 255}); err != nil {
		t.Fatal(err)
	}
	if err := writeSilenceWAV(filepath.Join(dir, "tone.wav"), 8000, 1); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeSolid(path string, w, h int, c color.RGBA) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func compileSmoke(t *testing.T, probe *media.FFProbe, dir, body string) []v1.Command {
	t.Helper()
	path := filepath.Join(dir, "piece.montage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	eng := v1.New(probe)
	p, err := eng.Resolve(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	return cmds
}

func runStages(t *testing.T, dir string, cmds []v1.Command, stages ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, s := range stages {
		want[s] = true
	}
	ran := map[string]int{}
	for _, c := range cmds {
		if !want[c.Stage] {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", c.Stage, err, out)
		}
		ran[c.Stage]++
	}
	for _, s := range stages {
		if ran[s] == 0 {
			t.Fatalf("no %s command", s)
		}
	}
}

func stageOutput(cmds []v1.Command, stage string) string {
	var path string
	for _, c := range cmds {
		if c.Stage == stage && len(c.Argv) > 0 {
			path = c.Argv[len(c.Argv)-1]
		}
	}
	return path
}

func stageArgv(cmds []v1.Command, stage string) []string {
	for _, c := range cmds {
		if c.Stage == stage {
			return c.Argv
		}
	}
	return nil
}

func decodeStagePNG(t *testing.T, dir string, cmds []v1.Command, stage string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, stageOutput(cmds, stage)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

type probeStreams struct {
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	PixFmt    string `json:"pix_fmt"`
}

func (p probeStreams) video() probeStream { return p.kind("video") }
func (p probeStreams) audio() probeStream { return p.kind("audio") }

func (p probeStreams) kind(kind string) probeStream {
	for _, s := range p.Streams {
		if s.CodecType == kind {
			return s
		}
	}
	return probeStream{}
}

func probeFile(t *testing.T, path string) probeStreams {
	t.Helper()
	bin, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-v", "error", "-show_entries", "stream=codec_type,codec_name,pix_fmt", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v\n%s", path, err, out)
	}
	var streams probeStreams
	if err := json.Unmarshal(out, &streams); err != nil {
		t.Fatal(err)
	}
	return streams
}
