package doc

import (
	"path/filepath"
	"regexp"

	"kbvmaker/v1/rule"
)

var modeRe = regexp.MustCompile(modeName)

var panNames = map[string]struct{}{
	"center": {}, "left": {}, "right": {}, "top": {}, "bottom": {},
	"top-left": {}, "top-right": {}, "bottom-left": {}, "bottom-right": {},
}

var clocks = map[string]struct{}{"song": {}, "clip": {}, "opaque": {}}

var anchors = map[string]struct{}{
	"song-start": {}, "song-end": {},
	"clip-start": {}, "clip-end": {},
	"opaque-start": {}, "opaque-end": {},
}

// Validate checks the document shape. It does not read media.
// A document that plan or render can run has a clock and at least one picture source.
// The clock is a song, a fit-audio film length, or authored picture lengths.
func (d *Document) Validate() error {
	if err := d.ValidateShape(); err != nil {
		return err
	}
	return d.ready()
}

// ValidateShape checks an edit the window may save before it can render.
// The song and the picture list may still be empty.
func (d *Document) ValidateShape() error {
	if d.Montage != 1 {
		return rule.E(0, "bad-number", "montage version %d", d.Montage)
	}
	if IsURL(d.Audio) {
		return rule.E(0, "missing-field", "audio")
	}
	if d.Defaults.Join.Type != "fade" {
		return rule.E(0, "unknown-field", "join type %q", d.Defaults.Join.Type)
	}
	if d.Defaults.Join.Duration <= 0 {
		return rule.E(0, "bad-number", "join duration %v", d.Defaults.Join.Duration)
	}
	if d.Defaults.Hold < 0 {
		return rule.E(0, "bad-number", "hold %v", d.Defaults.Hold)
	}
	if d.Defaults.ZoomSpan <= 0 {
		return rule.E(0, "bad-number", "zoomSpan %v", d.Defaults.ZoomSpan)
	}
	if d.Defaults.Ease != "linear" {
		return rule.E(0, "unknown-field", "ease %q", d.Defaults.Ease)
	}
	if err := d.Canvas.Backdrop.finish(0); err != nil {
		return err
	}
	switch d.Timing.Policy {
	case "fit-audio", "fixed-clip", "explicit", "markers":
	default:
		return rule.E(0, "unknown-field", "timing policy %q", d.Timing.Policy)
	}
	if d.Timing.Policy == "fixed-clip" && (!d.Timing.HasClip || d.Timing.Clip <= 0) {
		return rule.E(0, "bad-number", "fixed-clip needs clip > 0")
	}
	if d.Timing.HasCount && d.Timing.Policy != "fit-audio" {
		return rule.E(0, "bad-number", "timing.count is only valid for fit-audio")
	}
	if d.Timing.HasClip && d.Timing.Policy != "fixed-clip" {
		return rule.E(0, "bad-number", "timing.clip is only valid for fixed-clip")
	}
	if d.Timing.HasDuration {
		if d.Timing.Policy != "fit-audio" {
			return rule.E(0, "bad-number", "timing.duration is only valid for fit-audio")
		}
		if d.Timing.Duration <= 0 {
			return rule.E(0, "bad-number", "timing.duration %v", d.Timing.Duration)
		}
		if d.Audio != "" {
			return rule.E(0, "bad-time", "a soundtrack and a film length cannot both set the clock")
		}
	}
	if d.Schedule != nil && d.Pool == nil {
		return rule.E(0, "pool", "schedule requires a pool")
	}
	if d.Pool != nil {
		if err := validatePool(d.Pool); err != nil {
			return err
		}
		if d.Schedule == nil {
			return rule.E(0, "missing-field", "schedule")
		}
		if err := validateSchedule(d.Schedule); err != nil {
			return err
		}
	}
	if d.Timing.Policy == "markers" && d.Pool != nil && len(d.Slots) == 0 {
		return rule.E(0, "pool", "markers needs authored slots")
	}
	if d.Timing.Policy == "explicit" && d.Pool != nil && len(d.Slots) == 0 {
		return rule.E(0, "pool", "explicit needs authored slots")
	}
	if d.Timing.HasCount && (d.Pool == nil || d.Schedule == nil || d.Schedule.Policy != "loop") {
		return rule.E(0, "bad-number", "timing.count is only valid with a loop schedule")
	}
	for name, def := range d.Modes {
		if !modeRe.MatchString(name) {
			return rule.E(0, "mode", "mode name %q", name)
		}
		if def.Family != "reveal" && def.Family != "scan" {
			return rule.E(0, "mode", "family %q", def.Family)
		}
		if len(def.Motion) == 0 {
			return rule.E(0, "mode", "mode %s has no motion", name)
		}
		for i := range def.Motion {
			if err := validateSegment(def.Motion[i], 0); err != nil {
				return err
			}
		}
	}
	if err := validateMoves(d); err != nil {
		return err
	}
	for i, s := range d.Slots {
		if err := validateSlot(d, s, i+1); err != nil {
			return err
		}
	}
	return d.pins()
}

// BuiltinMode reports a mode the engine expands without a project definition.
func BuiltinMode(name string) bool {
	switch name {
	case "hold", "in-center", "out-center", "in-left-right", "in-right-left", "in-top-bottom", "in-bottom-top":
		return true
	default:
		return false
	}
}

func validateMoves(d *Document) error {
	seen := map[string]bool{}
	for _, name := range d.Moves {
		if !modeRe.MatchString(name) {
			return rule.E(0, "mode", "mode name %q", name)
		}
		if seen[name] {
			return rule.E(0, "mode", "move %s is listed twice", name)
		}
		seen[name] = true
		if _, ok := d.Modes[name]; ok || BuiltinMode(name) {
			continue
		}
		return rule.E(0, "mode", "unknown mode %q", name)
	}
	return nil
}

func (d *Document) ready() error {
	if d.Audio == "" && d.needsAudio() {
		return rule.E(0, "missing-field", "audio")
	}
	if len(d.Slots) == 0 && d.Pool == nil {
		return rule.E(0, "missing-field", "slots or pool")
	}
	return nil
}

// needsAudio reports a clock that still comes from the song.
// fit-audio can use timing.duration instead. fixed-clip and explicit can
// use authored slots. markers, a pool, and fit-audio without a duration cannot.
func (d *Document) needsAudio() bool {
	switch d.Timing.Policy {
	case "markers":
		return true
	case "fit-audio":
		return !d.Timing.HasDuration
	case "fixed-clip":
		return len(d.Slots) == 0
	case "explicit":
		return false
	default:
		return true
	}
}

func (d *Document) pins() error {
	if d.Timing.Policy != "fit-audio" || len(d.Slots) < 2 {
		return nil
	}
	for _, s := range d.Slots {
		if s.Dur == nil {
			return nil
		}
	}
	return rule.E(len(d.Slots), "bad-time", "leave one picture sharing the %s", clockName(d))
}

func clockName(d *Document) string {
	if d.Audio == "" {
		return "film"
	}
	return "song"
}

func validatePool(p *Pool) error {
	switch p.Kind {
	case "dir":
		if p.Dir == "" || IsURL(p.Dir) {
			return rule.E(0, "pool", "directory path")
		}
		if p.Sort != "natural" && p.Sort != "lexical" {
			return rule.E(0, "pool", "sort %q", p.Sort)
		}
	case "list":
		if p.List == "" || IsURL(p.List) {
			return rule.E(0, "pool", "list path")
		}
	case "files":
		if len(p.Files) == 0 {
			return rule.E(0, "pool", "files is empty")
		}
		for _, f := range p.Files {
			if f == "" || IsURL(f) {
				return rule.E(0, "pool", "file path %q", f)
			}
		}
	default:
		return rule.E(0, "pool", "empty pool")
	}
	return nil
}

func validateSchedule(s *Schedule) error {
	switch s.Policy {
	case "given":
		if s.HasSeed {
			return rule.E(0, "bad-number", "seed is not valid on given")
		}
	case "shuffle", "loop":
		if !s.HasSeed {
			return rule.E(0, "missing-field", "seed")
		}
	default:
		return rule.E(0, "unknown-field", "schedule policy %q", s.Policy)
	}
	return nil
}

func validateSlot(d *Document, s Slot, slot int) error {
	hasSrc := s.Src != ""
	hasCard := s.Card != nil
	if hasSrc && hasCard {
		return rule.E(slot, "unknown-field", "a slot has src and card")
	}
	if !hasSrc && !hasCard {
		return rule.E(slot, "missing-media", "src or card")
	}
	if hasSrc && IsURL(s.Src) {
		return rule.E(slot, "missing-media", "src")
	}
	if hasCard {
		if s.Backdrop != nil {
			return rule.E(slot, "unknown-field", "a card does not take a backdrop")
		}
		if err := s.Card.Finish(slot); err != nil {
			return err
		}
	}
	hasMode := s.Mode != ""
	hasMotion := len(s.Motion) > 0
	if hasMode == hasMotion {
		return rule.E(slot, "mode", "a slot needs a mode or a motion list, not both")
	}
	if hasMode && !modeRe.MatchString(s.Mode) {
		return rule.E(slot, "mode", "mode name %q", s.Mode)
	}
	if s.BasedOn != "" && !hasMotion {
		return rule.E(slot, "mode", "basedOn requires motion")
	}
	if s.Hold != nil && !hasMode {
		return rule.E(slot, "mode", "hold requires a mode")
	}
	if s.Hold != nil && *s.Hold < 0 {
		return rule.E(slot, "bad-number", "hold %v", *s.Hold)
	}
	if s.Join != nil {
		if slot == len(d.Slots) {
			return rule.E(slot, "dissolve", "the last slot has no outgoing join")
		}
		if s.Join.Type != "fade" {
			return rule.E(slot, "unknown-field", "join type %q", s.Join.Type)
		}
		if s.Join.Duration <= 0 {
			return rule.E(slot, "bad-number", "join duration %v", s.Join.Duration)
		}
	}
	switch d.Timing.Policy {
	case "explicit":
		if s.Dur == nil || *s.Dur <= 0 {
			return rule.E(slot, "bad-time", "explicit timing needs dur > 0")
		}
	case "fit-audio", "fixed-clip":
		if s.Dur != nil && *s.Dur <= 0 {
			return rule.E(slot, "bad-time", "dur %v", *s.Dur)
		}
	default:
		if s.Dur != nil {
			return rule.E(slot, "bad-time", "dur is only valid under fit-audio, fixed-clip, or explicit timing")
		}
	}
	switch d.Timing.Policy {
	case "markers":
		last := slot == len(d.Slots)
		if last && s.Until != nil {
			return rule.E(slot, "bad-time", "the last marker slot runs to the end of the song")
		}
		if !last && s.Until == nil {
			return rule.E(slot, "missing-field", "until")
		}
		if s.Until != nil && !s.Until.OnSong() {
			return rule.E(slot, "bad-time", "until is not on the song clock")
		}
	default:
		if s.Until != nil {
			return rule.E(slot, "bad-time", "until is only valid under markers")
		}
	}
	for i := range s.Motion {
		if err := validateSegment(s.Motion[i], slot); err != nil {
			return err
		}
	}
	if s.Backdrop != nil {
		if err := s.Backdrop.finish(slot); err != nil {
			return err
		}
	}
	return nil
}

func validateSegment(s Segment, slot int) error {
	if (s.To != nil) == s.HasDur {
		return rule.E(slot, "bad-time", "a segment needs to or dur, not both")
	}
	if s.HasDur && !s.DurRest && s.DurSeconds <= 0 {
		return rule.E(slot, "bad-time", "dur %v", s.DurSeconds)
	}
	if s.Ease != "" && s.Ease != "linear" {
		return rule.E(slot, "unknown-field", "ease %q", s.Ease)
	}
	if s.From != nil {
		if err := validateTime(*s.From, slot); err != nil {
			return err
		}
	}
	if s.To != nil {
		if err := validateTime(*s.To, slot); err != nil {
			return err
		}
	}
	if s.Zoom != nil {
		if err := validateZoom(*s.Zoom, slot); err != nil {
			return err
		}
	}
	if s.Pan != nil {
		if err := validatePan(*s.Pan, slot); err != nil {
			return err
		}
	}
	return nil
}

func validateTime(t TimePoint, slot int) error {
	switch t.Kind {
	case "sec", "pct":
		if _, ok := clocks[t.Of]; !ok {
			return rule.E(slot, "bad-time", "clock %q", t.Of)
		}
	case "timecode", "frame":
		return nil
	case "anchor":
		if _, ok := anchors[t.Anchor]; !ok {
			return rule.E(slot, "bad-time", "anchor %q", t.Anchor)
		}
	default:
		return rule.E(slot, "bad-time", "time kind %q", t.Kind)
	}
	return nil
}

func validateZoom(z ZoomSpec, slot int) error {
	if z.From != nil || z.To != nil {
		if z.From == nil || z.To == nil {
			return rule.E(slot, "bad-number", "zoom ramp needs from and to")
		}
		if err := validateEnd(*z.From, slot); err != nil {
			return err
		}
		return validateEnd(*z.To, slot)
	}
	if z.End == nil {
		return rule.E(slot, "bad-number", "zoom is empty")
	}
	return validateEnd(*z.End, slot)
}

func validateEnd(e ZoomEnd, slot int) error {
	if len(e.Atoms) == 0 {
		return rule.E(slot, "bad-number", "zoom endpoint is empty")
	}
	for _, a := range e.Atoms {
		if a.IsNum {
			if a.Num < 1 {
				return rule.E(slot, "bad-number", "zoom %v is below 1", a.Num)
			}
			continue
		}
		switch a.Sym {
		case "full", "fill", "limit":
		default:
			return rule.E(slot, "bad-number", "zoom %q", a.Sym)
		}
	}
	return nil
}

func validatePan(p PanSpec, slot int) error {
	if p.From != nil || p.To != nil {
		if p.From == nil || p.To == nil {
			return rule.E(slot, "bad-number", "pan ramp needs from and to")
		}
		if err := validatePanEnd(*p.From, slot); err != nil {
			return err
		}
		return validatePanEnd(*p.To, slot)
	}
	if p.End == nil {
		return rule.E(slot, "bad-number", "pan is empty")
	}
	return validatePanEnd(*p.End, slot)
}

func validatePanEnd(e PanEnd, slot int) error {
	if e.HasUV {
		if e.U < 0 || e.U > 1 || e.V < 0 || e.V > 1 {
			return rule.E(slot, "bad-number", "pan %v,%v", e.U, e.V)
		}
		return nil
	}
	if _, ok := panNames[e.Name]; !ok {
		return rule.E(slot, "bad-number", "pan %q", e.Name)
	}
	return nil
}

// ResolvePath joins a document-relative path onto dir.
func ResolvePath(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(dir, p))
}
