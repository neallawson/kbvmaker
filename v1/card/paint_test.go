package card

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"kbvmaker/v1/doc"
	"kbvmaker/v1/rule"
)

func TestPaintTitleFillsTheCornersAndUsesWhiteInk(t *testing.T) {
	img, err := Paint(doc.Card{Title: "Greenhouse"}, 320, 180)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
		t.Fatalf("size %v", img.Bounds())
	}
	for _, p := range []image.Point{{0, 0}, {319, 0}, {0, 179}, {319, 179}} {
		if got := img.RGBAAt(p.X, p.Y); got != (color.RGBA{0, 0, 0, 255}) {
			t.Fatalf("corner %v %v", p, got)
		}
	}
	if !hasNear(img, color.RGBA{255, 255, 255, 255}, 40) {
		t.Fatal("no white title ink")
	}
}

func TestPaintCreditsSitOnTheLowerLeft(t *testing.T) {
	img, err := Paint(doc.Card{
		Credits: []string{"Camera — Ada"},
		Align:   "left",
		VAlign:  "lower",
		Size:    "small",
		Weight:  "regular",
		Color:   "#FFFF00",
		Fill:    "#112233",
	}, 320, 180)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got != (color.RGBA{0x11, 0x22, 0x33, 255}) {
		t.Fatalf("fill %v", got)
	}
	fill := color.RGBA{0x11, 0x22, 0x33, 255}
	top := countNot(img, fill, image.Rect(0, 0, 320, 70))
	bot := countNot(img, fill, image.Rect(0, 90, 180, 180))
	right := countNot(img, fill, image.Rect(220, 0, 320, 180))
	if bot == 0 || top >= bot || right >= bot {
		t.Fatalf("ink top %d bot %d right %d", top, bot, right)
	}
}

func TestPaintSerifAndBox(t *testing.T) {
	img, err := Paint(doc.Card{
		Title: "Credits",
		Face:  "serif",
		Box:   true,
		Fill:  "#000000",
		Color: "#FFFFFF",
	}, 200, 120)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNear(img, color.RGBA{255, 255, 255, 255}, 80) {
		t.Fatal("no serif ink")
	}
}

func TestPaintRejectsEmptyText(t *testing.T) {
	_, err := Paint(doc.Card{Subtitle: "only a subtitle"}, 64, 36)
	re, ok := rule.As(err)
	if !ok || re.Code != "missing-field" {
		t.Fatalf("empty: %v", err)
	}
}

func TestPNGAndJPEGRoundTrip(t *testing.T) {
	c := doc.Card{Title: "Title", Subtitle: "a short film"}
	raw, err := PNG(c, 64, 36)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 36 {
		t.Fatalf("png %v %v", err, img)
	}
	jpg, err := JPEG(c, 64, 36)
	if err != nil {
		t.Fatal(err)
	}
	j, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil || j.Bounds().Dx() != 64 || j.Bounds().Dy() != 36 {
		t.Fatalf("jpeg %v %v", err, j)
	}
}

func hasNear(img *image.RGBA, want color.RGBA, slop int) bool {
	return countNear(img, want, slop, img.Bounds()) > 0
}

func countNear(img *image.RGBA, want color.RGBA, slop int, r image.Rectangle) int {
	n := 0
	b := r.Intersect(img.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			got := img.RGBAAt(x, y)
			if abs(int(got.R)-int(want.R)) <= slop && abs(int(got.G)-int(want.G)) <= slop && abs(int(got.B)-int(want.B)) <= slop {
				n++
			}
		}
	}
	return n
}

func countNot(img *image.RGBA, skip color.RGBA, r image.Rectangle) int {
	n := 0
	b := r.Intersect(img.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != skip {
				n++
			}
		}
	}
	return n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
