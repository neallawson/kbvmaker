// Package clock resolves a Montage time point onto the song, clip, or opaque clock.
package clock

import (
	"math"
	"regexp"
	"strconv"

	"neal/kbvmaker/v1/doc"
	"neal/kbvmaker/v1/rule"
)

// Place is the resolved clip a segment is measured against.
type Place struct {
	Song        float64
	ClipStart   float64
	ClipDur     float64
	OpaqueStart float64
	OpaqueEnd   float64
	FPS         int
}

var timecode = regexp.MustCompile(`^(\d+):([0-5]\d):([0-5]\d)(\.\d+)?$`)

// Song returns seconds from the start of the audio.
// The point must be a song-clock spelling.
func Song(tp doc.TimePoint, song float64, fps int, slot int) (float64, error) {
	if !tp.OnSong() {
		return 0, rule.E(slot, "bad-time", "marker %s is not on the song clock", tp.Kind)
	}
	return songPoint(tp, song, fps, slot)
}

// Clip returns seconds from the start of the clip.
// A song-clock point is legal when it falls inside this slot's output extent.
func Clip(tp doc.TimePoint, p Place, slot int) (float64, error) {
	if tp.OnSong() {
		s, err := songPoint(tp, p.Song, p.FPS, slot)
		if err != nil {
			return 0, err
		}
		if s < p.ClipStart-1e-9 || s > p.ClipStart+p.ClipDur+1e-9 {
			return 0, rule.E(slot, "bad-time", "song time %s is outside the slot", num(s))
		}
		return s - p.ClipStart, nil
	}
	switch tp.Kind {
	case "sec", "pct":
		switch tp.Of {
		case "clip":
			return onClock(tp, 0, p.ClipDur, slot)
		case "opaque":
			return onClock(tp, p.OpaqueStart, p.OpaqueEnd-p.OpaqueStart, slot)
		default:
			return 0, rule.E(slot, "bad-time", "clock %q", tp.Of)
		}
	case "anchor":
		var base float64
		switch tp.Anchor {
		case "clip-start":
			base = 0
		case "clip-end":
			base = p.ClipDur
		case "opaque-start":
			base = p.OpaqueStart
		case "opaque-end":
			base = p.OpaqueEnd
		default:
			return 0, rule.E(slot, "bad-time", "anchor %q", tp.Anchor)
		}
		return base + tp.Offset, nil
	default:
		return 0, rule.E(slot, "bad-time", "time kind %q", tp.Kind)
	}
}

func songPoint(tp doc.TimePoint, song float64, fps, slot int) (float64, error) {
	switch tp.Kind {
	case "sec":
		return onClock(tp, 0, song, slot)
	case "pct":
		return onClock(tp, 0, song, slot)
	case "timecode":
		m := timecode.FindStringSubmatch(tp.Timecode)
		if m == nil {
			return 0, rule.E(slot, "bad-time", "timecode %q", tp.Timecode)
		}
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		sec, _ := strconv.Atoi(m[3])
		frac := 0.0
		if m[4] != "" {
			frac, _ = strconv.ParseFloat(m[4], 64)
		}
		return float64(h*3600+min*60+sec) + frac, nil
	case "frame":
		if fps <= 0 {
			return 0, rule.E(slot, "bad-time", "fps is zero")
		}
		return float64(tp.Frame) / float64(fps), nil
	case "anchor":
		switch tp.Anchor {
		case "song-start":
			return tp.Offset, nil
		case "song-end":
			return song + tp.Offset, nil
		default:
			return 0, rule.E(slot, "bad-time", "anchor %q", tp.Anchor)
		}
	default:
		return 0, rule.E(slot, "bad-time", "time kind %q", tp.Kind)
	}
}

func onClock(tp doc.TimePoint, start, length float64, slot int) (float64, error) {
	if tp.Kind == "pct" {
		if tp.Pct < 0 || tp.Pct > 100 || math.IsNaN(tp.Pct) {
			return 0, rule.E(slot, "bad-time", "percent %v", tp.Pct)
		}
		return start + (tp.Pct/100)*length, nil
	}
	if math.IsNaN(tp.Sec) {
		return 0, rule.E(slot, "bad-time", "seconds are not a number")
	}
	if tp.Sec < 0 {
		return start + length + tp.Sec, nil
	}
	return start + tp.Sec, nil
}

func num(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
