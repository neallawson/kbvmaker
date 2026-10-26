// Package model holds the resolved timeline. It has no behavior.
package model

import "kbvmaker/v1/doc"

// Plan is the resolved timeline for one Montage document.
type Plan struct {
	Montage    int     `json:"montage"`
	Kind       string  `json:"kind"`
	Song       float64 `json:"song"`
	Duration   float64 `json:"duration"`
	Dir        string  `json:"-"`
	Audio      string  `json:"audio"`
	OutputFile string  `json:"outputFile"`
	Canvas     Canvas  `json:"canvas"`
	Output     Output  `json:"output"`
	Slots      []Slot  `json:"slots"`
}

// Canvas is the working still that zoompan reads.
type Canvas struct {
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Backdrop Backdrop `json:"backdrop"`
}

// Backdrop is the resolved bars behind the fitted photograph.
type Backdrop struct {
	Kind  string  `json:"kind"`
	Blur  float64 `json:"blur,omitempty"`
	Color string  `json:"color,omitempty"`
	Name  string  `json:"name,omitempty"`
	Src   string  `json:"src,omitempty"`
}

// Output is the encoded frame.
type Output struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	FPS          int     `json:"fps"`
	VideoCodec   string  `json:"videoCodec"`
	CRF          float64 `json:"crf"`
	Preset       string  `json:"preset"`
	PixFmt       string  `json:"pixFmt"`
	AudioCodec   string  `json:"audioCodec"`
	AudioBitrate int     `json:"audioBitrate"`
}

// Slot is one picture or card on the resolved timeline.
type Slot struct {
	Index       int       `json:"index"`
	Src         string    `json:"src,omitempty"`
	Card        *doc.Card `json:"card,omitempty"`
	Mode        string    `json:"mode,omitempty"`
	BasedOn     string    `json:"basedOn,omitempty"`
	Family      string    `json:"family"`
	ClipStart   float64   `json:"clipStart"`
	ClipDur     float64   `json:"clipDur"`
	OpaqueStart float64   `json:"opaqueStart"`
	OpaqueEnd   float64   `json:"opaqueEnd"`
	Fill        float64   `json:"fill"`
	Backdrop    Backdrop  `json:"backdrop"`
	FarZoom     float64   `json:"farZoom"`
	Join        *Join     `json:"join,omitempty"`
	Spans       []Span    `json:"-"`
	Fit         Fit       `json:"-"`
}

// Join is the dissolve that follows a slot.
type Join struct {
	Type     string  `json:"type"`
	Duration float64 `json:"duration"`
	Start    float64 `json:"start"`
	Mid      float64 `json:"mid"`
}

// Window is one slot's place on the output timeline.
type Window struct {
	ClipStart   float64
	ClipDur     float64
	OpaqueStart float64
	OpaqueEnd   float64
	Join        *Join
}

// Span is one linear piece of zoom and pan, in clip-local seconds.
type Span struct {
	From float64
	To   float64
	Z0   float64
	Z1   float64
	U0   float64
	U1   float64
	V0   float64
	V1   float64
}

// Fit is the sharp photograph's rectangle inside the canvas.
type Fit struct {
	X0 float64
	Y0 float64
	DW float64
	DH float64
}

// Image is the displayed size of a photograph, after rotation.
type Image struct {
	Width  int
	Height int
}
