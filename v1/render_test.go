package v1_test

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"neal/kbvmaker/v1"
	"neal/kbvmaker/v1/frame"
	"neal/kbvmaker/v1/media"
)

func TestGeneratedImageAndShortRender(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip(err)
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	photo := filepath.Join(dir, "photo.png")
	if err := writePortrait(photo, 200, 400); err != nil {
		t.Fatal(err)
	}
	info, err := probe.Image(context.Background(), photo)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 200 || info.Height != 400 {
		t.Fatalf("probed size %dx%d", info.Width, info.Height)
	}
	got := frame.Fill(info.Width, info.Height, 320, 180)
	want := (320.0 / 180.0) / (200.0 / 400.0)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("fill from probed pixels %v, want %v", got, want)
	}

	if err := writeSilenceWAV(filepath.Join(dir, "tone.wav"), 8000, 0.5); err != nil {
		t.Fatal(err)
	}
	body := `{
	  "montage": 1,
	  "canvas": {"width": 320, "height": 180, "blur": 2},
	  "output": {"width": 160, "height": 90, "fps": 10, "preset": "ultrafast", "crf": 30, "file": "out.mp4"},
	  "audio": "tone.wav",
	  "slots": [{"src": "photo.png", "mode": "hold"}]
	}`
	path := filepath.Join(dir, "piece.montage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	eng := v1.New(probe)
	ctx := context.Background()
	p, err := eng.Resolve(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := v1.Run(ctx, dir, cmds); err != nil {
		t.Fatal(err)
	}
	out, err := probe.Image(ctx, filepath.Join(dir, "out.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 160 || out.Height != 90 {
		t.Fatalf("output frame %dx%d", out.Width, out.Height)
	}
	dur, err := probe.Duration(ctx, filepath.Join(dir, "out.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(dur-0.5) > 0.15 {
		t.Fatalf("output duration %v", dur)
	}
}

func TestColorBackdropPixels(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	if err := writePortrait(filepath.Join(dir, "photo.png"), 200, 400); err != nil {
		t.Fatal(err)
	}
	body := `{
	  "montage": 1,
	  "canvas": {"width": 320, "height": 180, "backdrop": {"kind": "color", "color": "#0000FF"}},
	  "output": {"width": 160, "height": 90, "fps": 10, "preset": "ultrafast", "crf": 30, "file": "out.mp4"},
	  "audio": "tone.wav",
	  "slots": [{"src": "photo.png", "mode": "hold"}]
	}`
	path := filepath.Join(dir, "piece.montage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSilenceWAV(filepath.Join(dir, "tone.wav"), 8000, 0.4); err != nil {
		t.Fatal(err)
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		t.Fatal(err)
	}
	d, err := v1.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v1.New(probe).Resolve(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := v1.New(probe).Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	var ran bool
	for _, c := range cmds {
		if c.Stage != "canvas" {
			continue
		}
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		ran = true
	}
	if !ran {
		t.Fatal("no canvas command")
	}
	f, err := os.Open(filepath.Join(dir, "work", "canvas-0.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
		t.Fatalf("canvas %v", img.Bounds())
	}
	br, bg, bb := rgb(img.At(0, 0))
	if br > 5 || bg > 5 || bb < 250 {
		t.Fatalf("bar pixel rgb(%d,%d,%d), want blue", br, bg, bb)
	}
	pr, pg, pb := rgb(img.At(160, 90))
	if pr < 150 || pg > 80 || pb > 80 {
		t.Fatalf("photo pixel rgb(%d,%d,%d), want the red portrait", pr, pg, pb)
	}
}

func rgb(c color.Color) (r, g, b int) {
	rr, gg, bb, _ := c.RGBA()
	return int(rr >> 8), int(gg >> 8), int(bb >> 8)
}

func writePortrait(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: 180, G: 40, B: 40, A: 255}
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

func writeSilenceWAV(path string, rate int, seconds float64) error {
	n := int(math.Round(seconds * float64(rate)))
	data := make([]byte, n*2)
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(data)))
	copy(hdr[8:], "WAVE")
	copy(hdr[12:], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], 1)
	binary.LittleEndian.PutUint32(hdr[24:], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(data)))
	return os.WriteFile(path, append(hdr, data...), 0o644)
}
