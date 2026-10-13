package frame

import (
	"math"
	"testing"

	"kbvmaker/v1/model"
)

func TestFillPortraitAndLandscape(t *testing.T) {
	// 3000×4000 on 3840×2160. aspect 0.75, canvas 16/9, fill = (16/9)/0.75.
	got := Fill(3000, 4000, 3840, 2160)
	want := (3840.0 / 2160.0) / (3000.0 / 4000.0)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("portrait fill %v, want %v", got, want)
	}
	// 4000×1500 is wider than 16:9, so fill is image aspect over canvas aspect.
	wide := Fill(4000, 1500, 3840, 2160)
	wantWide := (4000.0 / 1500.0) / (3840.0 / 2160.0)
	if math.Abs(wide-wantWide) > 1e-12 {
		t.Fatalf("landscape fill %v, want %v", wide, wantWide)
	}
	if math.Abs(Fill(3840, 2160, 3840, 2160)-1) > 1e-12 {
		t.Fatal("matching aspect should fill at 1")
	}
}

func TestFitPortrait(t *testing.T) {
	f := FitRect(3000, 4000, 3840, 2160)
	if math.Abs(f.DW-1620) > 1e-9 || math.Abs(f.DH-2160) > 1e-9 {
		t.Fatalf("fitted size %v × %v", f.DW, f.DH)
	}
	if math.Abs(f.X0-1110) > 1e-9 || math.Abs(f.Y0) > 1e-9 {
		t.Fatalf("origin %v,%v", f.X0, f.Y0)
	}
}

func TestWindowAtZoomOneIsOrigin(t *testing.T) {
	f := FitRect(3000, 4000, 3840, 2160)
	points := [][2]float64{{0, 0}, {0, 0.5}, {0.5, 0.5}, {1, 1}, {1, 0}}
	for _, p := range points {
		x, y := Window(f, 3840, 2160, 1, p[0], p[1])
		if x != 0 || y != 0 {
			t.Fatalf("zoom 1 subject %v window %v,%v", p, x, y)
		}
	}
}

func TestWindowCenterTracksZoom(t *testing.T) {
	f := model.Fit{X0: 0, Y0: 0, DW: 3840, DH: 2160}
	x, y := Window(f, 3840, 2160, 2, 0.5, 0.5)
	// Centered window of half the canvas starts at a quarter of each side.
	if math.Abs(x-960) > 1e-9 || math.Abs(y-540) > 1e-9 {
		t.Fatalf("center at zoom 2 is %v,%v", x, y)
	}
}
