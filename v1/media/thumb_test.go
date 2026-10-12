package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestThumbnailCoversTheRequestedBox(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	path := filepath.Join(t.TempDir(), "wide.png")
	writePNG(t, path, 200, 40, color.NRGBA{R: 20, G: 80, B: 180, A: 255})

	jpegBytes, err := Thumbnail(context.Background(), path, 32, 18)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 32 || got.Bounds().Dy() != 18 {
		t.Fatalf("thumbnail is %dx%d, want 32x18", got.Bounds().Dx(), got.Bounds().Dy())
	}

	again, err := Thumbnail(context.Background(), path, 32, 18)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(jpegBytes) {
		t.Fatal("a second request did not reuse the thumbnail")
	}
}

func TestThumbnailMissingFile(t *testing.T) {
	_, err := Thumbnail(context.Background(), filepath.Join(t.TempDir(), "gone.jpg"), 16, 16)
	if err == nil {
		t.Fatal("expected an error for a missing picture")
	}
}

func writePNG(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
