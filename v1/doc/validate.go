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
	for i, t := range d.Tracks {
		if t.Src == "" || IsURL(t.Src) {
			return rule.E(0, "missing-field", "track %d needs a file", i+1)
		}
		if t.Gap < 0 {
			return rule.E(0, "bad-number", "the silence before track %d is %v seconds, and it must be zero or more", i+1, t.Gap)
		}
	}
	if d.Defaults.Join.Type != "fade" {
		return rule.E(0, "unknown-field", "join type %q is not supported", d.Defaults.Join.Type)
	}
	if d.Defaults.Join.Duration <= 0 {
		return rule.E(0, "bad-number", "the dissolve of %v seconds must be longer than zero", d.Defaults.Join.Duration)
	}
	if d.Defaults.Hold < 0 {
		return rule.E(0, "bad-number", "hold %v must be zero or more", d.Defaults.Hold)
	}
	if d.Defaults.ZoomSpan <= 0 {
		return rule.E(0, "bad-number", "zoom span %v must be greater than zero", d.Defaults.ZoomSpan)
	}
	if d.Defaults.Ease != "linear" {
		return rule.E(0, "unknown-field", "ease %q is not supported", d.Defaults.Ease)
	}
	if err := d.Canvas.Backdrop.finish(0); err != nil {
		return err
	}
	switch d.Timing.Policy {
	case "fit-audio", "fixed-clip", "explicit", "markers":
	default:
		return rule.E(0, "unknown-field", "timing policy %q is not supported", d.Timing.Policy)
	}
	if d.Timing.Policy == "fixed-clip" && (!d.Timing.HasClip || d.Timing.Clip <= 0) {
		return rule.E(0, "bad-number", "fixed-clip needs a time for each picture, longer than zero")
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
			return rule.E(0, "bad-number", "the film length %v must be longer than zero", d.Timing.Duration)
		}
		if len(d.Tracks) > 0 {
			return rule.E(0, "bad-time", "a soundtrack and a film length cannot both set the clock")
		}
	}
	if d.Schedule != nil && d.Pool == nil {
		return rule.E(0, "pool", "a schedule needs a pool of pictures")
	}
	if d.Pool != nil {
		if err := validatePool(d.Pool); err != nil {
			return err
		}
		if d.Schedule == nil {
			return rule.E(0, "missing-field", "a pool of pictures needs a schedule")
		}
		if err := validateSchedule(d.Schedule); err != nil {
			return err
		}
	}
	if d.Timing.Policy == "markers" && d.Pool != nil && len(d.Slots) == 0 {
		return rule.E(0, "pool", "markers needs the pictures listed in the file")
	}
	if d.Timing.Policy == "explicit" && d.Pool != nil && len(d.Slots) == 0 {
		return rule.E(0, "pool", "explicit timing needs the pictures listed in the file")
	}
	if d.Timing.HasCount && (d.Pool == nil || d.Schedule == nil || d.Schedule.Policy != "loop") {
		return rule.E(0, "bad-number", "timing.count is only valid with a loop schedule")
	}
	for name, def := range d.Modes {
		if !modeRe.MatchString(name) {
			return rule.E(0, "mode", "move name %q is not valid", name)
		}
		if def.Family != "reveal" && def.Family != "scan" {
			return rule.E(0, "mode", "move family %q is not supported", def.Family)
		}
		if len(def.Motion) == 0 {
			return rule.E(0, "mode", "move %s needs at least one motion", name)
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
			return rule.E(0, "mode", "move name %q is not valid", name)
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
	if len(d.Tracks) == 0 && d.needsAudio() {
		return rule.E(0, "missing-field", "this film needs a soundtrack")
	}
	if len(d.Slots) == 0 && d.Pool == nil {
		return rule.E(0, "missing-field", "this film needs pictures")
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
	return rule.E(len(d.Slots), "bad-time", "one picture must be left without its own length, so it can fill the rest of the %s", clockName(d))
}

func clockName(d *Document) string {
	if len(d.Tracks) == 0 {
		return "film"
	}
	return "song"
}

func validatePool(p *Pool) error {
	switch p.Kind {
	case "dir":
		if p.Dir == "" || IsURL(p.Dir) {
			return rule.E(0, "pool", "the picture pool needs a directory")
		}
		if p.Sort != "natural" && p.Sort != "lexical" {
			return rule.E(0, "pool", "sort %q is not supported", p.Sort)
		}
	case "list":
		if p.List == "" || IsURL(p.List) {
			return rule.E(0, "pool", "the picture pool needs a list file")
		}
	case "files":
		if len(p.Files) == 0 {
			return rule.E(0, "pool", "the picture pool lists no files")
		}
		for _, f := range p.Files {
			if f == "" || IsURL(f) {
				return rule.E(0, "pool", "pool file %q needs a path", f)
			}
		}
	default:
		return rule.E(0, "pool", "the picture pool is empty")
	}
	return nil
}

func validateSchedule(s *Schedule) error {
	switch s.Policy {
	case "given":
		if s.HasSeed {
			return rule.E(0, "bad-number", "an ordered pool does not take a seed")
		}
	case "shuffle", "loop":
		if !s.HasSeed {
			return rule.E(0, "missing-field", "this schedule needs a seed")
		}
	default:
		return rule.E(0, "unknown-field", "schedule policy %q is not supported", s.Policy)
	}
	return nil
}

func validateSlot(d *Document, s Slot, slot int) error {
	hasSrc := s.Src != ""
	hasCard := s.Card != nil
	if hasSrc && hasCard {
		return rule.E(slot, "unknown-field", "a picture cannot be both a file and a card")
	}
	if !hasSrc && !hasCard {
		return rule.E(slot, "missing-media", "this picture needs a file or a card")
	}
	if hasSrc && IsURL(s.Src) {
		return rule.E(slot, "missing-media", "a picture file must be a path on this computer")
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
	if hasMode && hasMotion {
		return rule.E(slot, "mode", "this picture has both a move and a custom motion list")
	}
	if !hasMode && !hasMotion {
		return rule.E(slot, "mode", "this picture needs a move")
	}
	if hasMode && !modeRe.MatchString(s.Mode) {
		return rule.E(slot, "mode", "move name %q is not valid", s.Mode)
	}
	if s.BasedOn != "" && !hasMotion {
		return rule.E(slot, "mode", "basedOn needs a custom motion list")
	}
	if s.Hold != nil && !hasMode {
		return rule.E(slot, "mode", "a custom still time needs a built-in move")
	}
	if s.Hold != nil && *s.Hold < 0 {
		return rule.E(slot, "bad-number", "still time %v must be zero or more", *s.Hold)
	}
	if s.Join != nil {
		if slot == len(d.Slots) {
			return rule.E(slot, "dissolve", "the last picture cannot have a dissolve after it")
		}
		if s.Join.Type != "fade" {
			return rule.E(slot, "unknown-field", "join type %q is not supported", s.Join.Type)
		}
		if s.Join.Duration <= 0 {
			return rule.E(slot, "bad-number", "the dissolve of %v seconds must be longer than zero", s.Join.Duration)
		}
	}
	switch d.Timing.Policy {
	case "explicit":
		if s.Dur == nil || *s.Dur <= 0 {
			return rule.E(slot, "bad-time", "each picture needs a length longer than zero")
		}
	case "fit-audio", "fixed-clip":
		if s.Dur != nil && *s.Dur <= 0 {
			return rule.E(slot, "bad-time", "picture length %v must be longer than zero", *s.Dur)
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
			return rule.E(slot, "bad-time", "the last picture runs to the end of the song and cannot have its own end time")
		}
		if !last && s.Until == nil {
			return rule.E(slot, "missing-field", "this picture needs an end time on the song")
		}
		if s.Until != nil && !s.Until.OnSong() {
			return rule.E(slot, "bad-time", "the end time must be a time on the song")
		}
	default:
		if s.Until != nil {
			return rule.E(slot, "bad-time", "an end time is only used with the markers clock")
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
	if s.To != nil && s.HasDur {
		return rule.E(slot, "bad-time", "a motion segment has both an end time and a length")
	}
	if s.To == nil && !s.HasDur {
		return rule.E(slot, "bad-time", "a motion segment needs an end time or a length")
	}
	if s.HasDur && !s.DurRest && s.DurSeconds <= 0 {
		return rule.E(slot, "bad-time", "motion length %v must be longer than zero", s.DurSeconds)
	}
	if s.Ease != "" && s.Ease != "linear" {
		return rule.E(slot, "unknown-field", "ease %q is not supported", s.Ease)
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
			return rule.E(slot, "bad-time", "clock %q is not supported", t.Of)
		}
	case "timecode", "frame":
		return nil
	case "anchor":
		if _, ok := anchors[t.Anchor]; !ok {
			return rule.E(slot, "bad-time", "anchor %q is not supported", t.Anchor)
		}
	default:
		return rule.E(slot, "bad-time", "time kind %q is not supported", t.Kind)
	}
	return nil
}

func validateZoom(z ZoomSpec, slot int) error {
	if z.From != nil || z.To != nil {
		if z.From == nil || z.To == nil {
			return rule.E(slot, "bad-number", "a zoom ramp needs a start and an end")
		}
		if err := validateEnd(*z.From, slot); err != nil {
			return err
		}
		return validateEnd(*z.To, slot)
	}
	if z.End == nil {
		return rule.E(slot, "bad-number", "zoom needs a value")
	}
	return validateEnd(*z.End, slot)
}

func validateEnd(e ZoomEnd, slot int) error {
	if len(e.Atoms) == 0 {
		return rule.E(slot, "bad-number", "a zoom endpoint needs a value")
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
			return rule.E(slot, "bad-number", "a pan ramp needs a start and an end")
		}
		if err := validatePanEnd(*p.From, slot); err != nil {
			return err
		}
		return validatePanEnd(*p.To, slot)
	}
	if p.End == nil {
		return rule.E(slot, "bad-number", "pan needs a value")
	}
	return validatePanEnd(*p.End, slot)
}

func validatePanEnd(e PanEnd, slot int) error {
	if e.HasUV {
		if e.U < 0 || e.U > 1 || e.V < 0 || e.V > 1 {
			return rule.E(slot, "bad-number", "pan %v,%v must stay inside 0 to 1", e.U, e.V)
		}
		return nil
	}
	if _, ok := panNames[e.Name]; !ok {
		return rule.E(slot, "bad-number", "pan %q is not supported", e.Name)
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
