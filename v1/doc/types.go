// Package doc loads a Montage document and checks its shape.
package doc

import "strings"

// ModeName matches a catalog key.
const modeName = `^[a-z0-9]+(-[a-z0-9]+)*$`

// Document is a Montage project after defaults have been applied.
type Document struct {
	Montage  int       `json:"montage"`
	Canvas   Canvas    `json:"canvas"`
	Output   Output    `json:"output"`
	Defaults Defaults  `json:"defaults"`
	Audio    string    `json:"audio"`
	Dir      string    `json:"-"`
	Timing   Timing    `json:"timing"`
	Pool     *Pool     `json:"pool,omitempty"`
	Schedule *Schedule `json:"schedule,omitempty"`
	Modes    ModeMap   `json:"modes,omitempty"`
	Moves    []string  `json:"moves,omitempty"`
	Slots    []Slot    `json:"slots,omitempty"`
}

// Canvas is the working still.
type Canvas struct {
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Backdrop Backdrop `json:"backdrop"`
}

// Output is the encoded movie.
type Output struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	FPS          int     `json:"fps"`
	File         string  `json:"file,omitempty"`
	VideoCodec   string  `json:"videoCodec"`
	CRF          float64 `json:"crf"`
	Preset       string  `json:"preset"`
	PixFmt       string  `json:"pixFmt"`
	AudioCodec   string  `json:"audioCodec"`
	AudioBitrate int     `json:"audioBitrate"`
}

// Defaults apply when a slot omits the value.
type Defaults struct {
	Join     Join    `json:"join"`
	Hold     float64 `json:"hold"`
	ZoomSpan float64 `json:"zoomSpan"`
	Ease     string  `json:"ease"`
}

// Join is a dissolve.
type Join struct {
	Type     string  `json:"type"`
	Duration float64 `json:"duration"`
}

// Timing chooses how slot durations are derived.
// Duration is the finished film length for fit-audio when there is no soundtrack.
type Timing struct {
	Policy      string  `json:"-"`
	Clip        float64 `json:"-"`
	Duration    float64 `json:"-"`
	Count       int     `json:"-"`
	HasClip     bool    `json:"-"`
	HasDuration bool    `json:"-"`
	HasCount    bool    `json:"-"`
}

// Pool is a directory, a list file, or an authored file list.
type Pool struct {
	Dir   string   `json:"-"`
	Sort  string   `json:"-"`
	List  string   `json:"-"`
	Files []string `json:"-"`
	Kind  string   `json:"-"`
}

// Schedule turns a pool into an ordered slot list.
type Schedule struct {
	Policy  string `json:"-"`
	Seed    uint64 `json:"-"`
	HasSeed bool   `json:"-"`
}

// ModeMap is a project's extra modes. JSON object key order is sorted on write.
type ModeMap map[string]ModeDef

// ModeDef is a family plus the segments it expands to.
type ModeDef struct {
	Family string    `json:"family"`
	Motion []Segment `json:"motion"`
}

// Slot is one picture or card in the authored timeline.
type Slot struct {
	Src      string     `json:"src,omitempty"`
	Card     *Card      `json:"card,omitempty"`
	Mode     string     `json:"mode,omitempty"`
	Motion   []Segment  `json:"motion,omitempty"`
	BasedOn  string     `json:"basedOn,omitempty"`
	Hold     *float64   `json:"hold,omitempty"`
	Join     *Join      `json:"join,omitempty"`
	Dur      *float64   `json:"dur,omitempty"`
	Until    *TimePoint `json:"until,omitempty"`
	Backdrop *Backdrop  `json:"backdrop,omitempty"`
}

// HoldOr returns the slot hold, or def when the slot does not set one.
func (s Slot) HoldOr(def float64) float64 {
	if s.Hold == nil {
		return def
	}
	return *s.Hold
}

// Segment is one span of zoom and pan.
type Segment struct {
	From       *TimePoint `json:"-"`
	To         *TimePoint `json:"-"`
	DurSeconds float64    `json:"-"`
	DurRest    bool       `json:"-"`
	HasDur     bool       `json:"-"`
	Zoom       *ZoomSpec  `json:"-"`
	Pan        *PanSpec   `json:"-"`
	Ease       string     `json:"-"`
}

// TimePoint is one spelling of a position.
type TimePoint struct {
	Kind      string
	Sec       float64
	Pct       float64
	Of        string
	Timecode  string
	Frame     int
	Anchor    string
	Offset    float64
	HasOffset bool
}

// OnSong reports whether the point is measured on the song clock.
func (t TimePoint) OnSong() bool {
	switch t.Kind {
	case "timecode", "frame":
		return true
	case "sec", "pct":
		return t.Of == "song"
	case "anchor":
		return t.Anchor == "song-start" || t.Anchor == "song-end"
	default:
		return false
	}
}

// ZoomSpec is a constant zoom or a linear ramp.
type ZoomSpec struct {
	From *ZoomEnd
	To   *ZoomEnd
	End  *ZoomEnd
}

// ZoomEnd is one endpoint, possibly the min of several atoms.
type ZoomEnd struct {
	Min   bool
	Atoms []ZoomAtom
}

// ZoomAtom is a number or a symbol: full, fill, or limit.
type ZoomAtom struct {
	Num   float64
	Sym   string
	IsNum bool
}

// PanSpec is a constant subject or a linear ramp.
type PanSpec struct {
	From *PanEnd
	To   *PanEnd
	End  *PanEnd
}

// PanEnd is a named point or an explicit subject.
type PanEnd struct {
	Name  string
	U     float64
	V     float64
	HasUV bool
}

// Sym is a symbolic zoom atom.
func Sym(s string) ZoomAtom { return ZoomAtom{Sym: s} }

// Num is a numeric zoom atom.
func Num(n float64) ZoomAtom { return ZoomAtom{IsNum: true, Num: n} }

// ZoomConst is a zoom that does not change across the segment.
func ZoomConst(a ZoomAtom) *ZoomSpec {
	return &ZoomSpec{End: &ZoomEnd{Atoms: []ZoomAtom{a}}}
}

// ZoomRamp interpolates between two endpoints.
func ZoomRamp(from, to ZoomEnd) *ZoomSpec {
	return &ZoomSpec{From: &from, To: &to}
}

// EndSym is a symbolic endpoint.
func EndSym(s string) ZoomEnd { return ZoomEnd{Atoms: []ZoomAtom{Sym(s)}} }

// EndMin is the smaller of the atoms, then raised to 1 if needed.
func EndMin(atoms ...ZoomAtom) ZoomEnd { return ZoomEnd{Min: true, Atoms: atoms} }

// PanNamed is a constant named subject.
func PanNamed(name string) *PanSpec {
	return &PanSpec{End: &PanEnd{Name: name}}
}

// PanRamp interpolates between two named subjects.
func PanRamp(from, to string) *PanSpec {
	return &PanSpec{
		From: &PanEnd{Name: from},
		To:   &PanEnd{Name: to},
	}
}

// Anchor is a time point at an anchor plus an optional offset.
func Anchor(name string, offset float64, hasOffset bool) *TimePoint {
	return &TimePoint{Kind: "anchor", Anchor: name, Offset: offset, HasOffset: hasOffset}
}

// IsURL reports a path the document is not allowed to carry.
func IsURL(p string) bool {
	return strings.Contains(p, "://")
}
