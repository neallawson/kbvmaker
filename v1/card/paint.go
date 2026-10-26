// Package card paints a title or credits screen onto a still.
package card

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"kbvmaker/v1/doc"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

var parsed = struct {
	sync.Mutex
	byName map[string]*opentype.Font
}{byName: map[string]*opentype.Font{}}

type line struct {
	text   string
	size   float64
	face   font.Face
	width  int
	credit bool
}

// Paint draws c onto a width-by-height RGBA still.
func Paint(c doc.Card, width, height int) (*image.RGBA, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("card size %dx%d", width, height)
	}
	if err := c.Finish(0); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fill := parseHex(c.Fill, color.RGBA{0, 0, 0, 255})
	ink := parseHex(c.Color, color.RGBA{255, 255, 255, 255})
	draw.Draw(img, img.Bounds(), &image.Uniform{fill}, image.Point{}, draw.Src)

	titleSize := titlePx(c.Size, height)
	lines, err := layout(c, titleSize, width)
	if err != nil {
		return nil, err
	}
	defer closeFaces(lines)
	if len(lines) == 0 {
		return img, nil
	}

	total := 0
	for i, ln := range lines {
		total += int(ln.size * 1.25)
		if i == len(lines)-1 {
			total -= int(ln.size * 0.25)
		}
	}
	hasHead, hasCredit := false, false
	for _, ln := range lines {
		if ln.credit {
			hasCredit = true
		} else {
			hasHead = true
		}
	}
	gap := 0
	if hasHead && hasCredit {
		gap = int(titleSize * 0.6)
		total += gap
	}

	marginY := int(0.18 * float64(height))
	y := 0
	switch c.VAlign {
	case "upper":
		y = marginY
	case "lower":
		y = height - marginY - total
	default:
		y = (height - total) / 2
	}
	if y < marginY/2 {
		y = marginY / 2
	}
	if y+total > height-marginY/2 {
		y = height - marginY/2 - total
		if y < marginY/2 {
			y = marginY / 2
		}
	}

	marginX := int(0.08 * float64(width))
	if c.Box {
		pad := int(titleSize * 0.45)
		box := image.Rect(marginX-pad, y-pad, width-marginX+pad, y+total+pad)
		if box.Min.X < 0 {
			box.Min.X = 0
		}
		if box.Min.Y < 0 {
			box.Min.Y = 0
		}
		if box.Max.X > width {
			box.Max.X = width
		}
		if box.Max.Y > height {
			box.Max.Y = height
		}
		overlay := color.RGBA{255, 255, 255, 40}
		if luma(fill) > 140 {
			overlay = color.RGBA{0, 0, 0, 40}
		}
		draw.Draw(img, box, &image.Uniform{overlay}, image.Point{}, draw.Over)
	}

	for i, ln := range lines {
		x := marginX
		switch c.Align {
		case "right":
			x = width - marginX - ln.width
		case "left":
			x = marginX
		default:
			x = (width - ln.width) / 2
		}
		baseline := y + int(ln.size*0.85)
		d := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: ln.face, Dot: fixed.P(x, baseline)}
		d.DrawString(ln.text)
		y += int(ln.size * 1.25)
		if gap > 0 && i+1 < len(lines) && !ln.credit && lines[i+1].credit {
			y += gap
		}
	}
	return img, nil
}

func titlePx(size string, height int) float64 {
	switch size {
	case "small":
		return 0.04 * float64(height)
	case "large":
		return 0.11 * float64(height)
	default:
		return 0.07 * float64(height)
	}
}

func layout(c doc.Card, titleSize float64, width int) ([]line, error) {
	maxW := int(0.84 * float64(width))
	if maxW < 8 {
		maxW = width
	}
	var out []line
	add := func(text string, size float64, credit bool) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		if size < 8 {
			size = 8
		}
		face, err := openFace(c.Face, c.Weight, size)
		if err != nil {
			return err
		}
		for _, part := range wrap(face, text, maxW) {
			out = append(out, line{
				text: part, size: size, face: face, credit: credit,
				width: font.MeasureString(face, part).Round(),
			})
		}
		return nil
	}
	if err := add(c.Title, titleSize, false); err != nil {
		return nil, err
	}
	if err := add(c.Subtitle, titleSize*0.55, false); err != nil {
		return nil, err
	}
	if err := add(c.Line, titleSize*0.45, false); err != nil {
		return nil, err
	}
	for _, cr := range c.Credits {
		if err := add(cr, titleSize*0.4, true); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func wrap(face font.Face, text string, maxW int) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		trial := cur + " " + w
		if font.MeasureString(face, trial).Round() <= maxW {
			cur = trial
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	return append(lines, cur)
}

func openFace(faceName, weight string, size float64) (font.Face, error) {
	if size < 1 {
		size = 1
	}
	file := "fonts/LiberationSans-Bold.ttf"
	switch {
	case faceName == "serif" && weight == "regular":
		file = "fonts/LiberationSerif-Regular.ttf"
	case faceName == "serif":
		file = "fonts/LiberationSerif-Bold.ttf"
	case weight == "regular":
		file = "fonts/LiberationSans-Regular.ttf"
	}
	ft, err := fontFile(file)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func fontFile(name string) (*opentype.Font, error) {
	parsed.Lock()
	defer parsed.Unlock()
	if f, ok := parsed.byName[name]; ok {
		return f, nil
	}
	raw, err := fontFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(raw)
	if err != nil {
		return nil, err
	}
	parsed.byName[name] = f
	return f, nil
}

func closeFaces(lines []line) {
	seen := map[font.Face]struct{}{}
	for _, ln := range lines {
		if _, ok := seen[ln.face]; ok {
			continue
		}
		seen[ln.face] = struct{}{}
		if c, ok := ln.face.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

func parseHex(s string, fallback color.RGBA) color.RGBA {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return fallback
	}
	n, err := parseRRGGBB(s[1:])
	if err != nil {
		return fallback
	}
	return n
}

func parseRRGGBB(s string) (color.RGBA, error) {
	var n uint32
	for i := 0; i < 6; i++ {
		c := s[i]
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return color.RGBA{}, fmt.Errorf("hex")
		}
		n = n<<4 | uint32(v)
	}
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, nil
}

func luma(c color.RGBA) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// PNG encodes a painted card.
func PNG(c doc.Card, width, height int) ([]byte, error) {
	img, err := Paint(c, width, height)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// JPEG encodes a painted card at quality 85.
func JPEG(c doc.Card, width, height int) ([]byte, error) {
	img, err := Paint(c, width, height)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
