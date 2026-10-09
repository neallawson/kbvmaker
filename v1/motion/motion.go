// Package motion expands modes into zoom and pan spans.
package motion

import (
	"math"
	"strconv"

	"neal/kbvmaker/v1/clock"
	"neal/kbvmaker/v1/doc"
	"neal/kbvmaker/v1/model"
	"neal/kbvmaker/v1/rule"
)

// Resolve expands a slot's mode or motion into spans that cover the clip.
// fill is the photograph's fill zoom. holdDefault is the project hold.
func Resolve(s doc.Slot, modes doc.ModeMap, place clock.Place, zoomSpan, holdDefault, fill float64, slot int) (family, mode, basedOn string, spans []model.Span, err error) {
	hold := s.HoldOr(holdDefault)
	family, segs, builtin, err := expand(s, modes, hold, slot)
	if err != nil {
		return "", "", "", nil, err
	}
	if builtin && traveling(s.Mode) && hold > place.OpaqueEnd-place.OpaqueStart+1e-9 {
		return "", "", "", nil, rule.E(slot, "no-travel", "hold does not fit inside the opaque range")
	}
	mode, basedOn = s.Mode, s.BasedOn
	if len(s.Motion) > 0 {
		mode = ""
	}
	spans, err = spansFrom(segs, place, zoomSpan, fill, slot)
	return family, mode, basedOn, spans, err
}

func traveling(name string) bool {
	switch name {
	case "in-center", "in-left-right", "in-top-bottom", "out-center":
		return true
	default:
		return false
	}
}

func expand(s doc.Slot, modes doc.ModeMap, hold float64, slot int) (string, []doc.Segment, bool, error) {
	if len(s.Motion) > 0 {
		return "reveal", s.Motion, false, nil
	}
	if def, ok := modes[s.Mode]; ok {
		return def.Family, def.Motion, false, nil
	}
	segs, family, ok := builtin(s.Mode, hold)
	if !ok {
		return "", nil, false, rule.E(slot, "mode", "unknown mode %q", s.Mode)
	}
	return family, segs, true, nil
}

func spansFrom(segs []doc.Segment, place clock.Place, zoomSpan, fill float64, slot int) ([]model.Span, error) {
	type piece struct {
		from, to float64
		z0, z1   float64
		u0, u1   float64
		v0, v1   float64
		hasZoom  bool
		hasPan   bool
	}
	raw := make([]piece, 0, len(segs))
	cursor := 0.0
	for _, seg := range segs {
		from := cursor
		var err error
		if seg.From != nil {
			from, err = clock.Clip(*seg.From, place, slot)
			if err != nil {
				return nil, err
			}
		}
		var to float64
		switch {
		case seg.DurRest:
			to = place.ClipDur
		case seg.HasDur:
			to = from + seg.DurSeconds
		case seg.To != nil:
			to, err = clock.Clip(*seg.To, place, slot)
			if err != nil {
				return nil, err
			}
		default:
			return nil, rule.E(slot, "bad-time", "segment has no end")
		}
		if from < -1e-6 || to > place.ClipDur+1e-6 {
			return nil, rule.E(slot, "span", "segment %s..%s is outside the clip", n(from), n(to))
		}
		if from < 0 {
			from = 0
		}
		if to > place.ClipDur {
			to = place.ClipDur
		}
		if to < from-1e-9 {
			return nil, rule.E(slot, "span", "segment ends before it starts")
		}
		p := piece{from: from, to: to, hasZoom: seg.Zoom != nil, hasPan: seg.Pan != nil}
		if seg.Zoom != nil {
			limit := 1.0
			if place.ClipDur > 0 {
				limit = 1 + (zoomSpan/place.ClipDur)*(to-from)
			}
			p.z0, p.z1, err = zoomRange(*seg.Zoom, fill, limit, slot)
			if err != nil {
				return nil, err
			}
		}
		if seg.Pan != nil {
			p.u0, p.v0, p.u1, p.v1, err = panRange(*seg.Pan, slot)
			if err != nil {
				return nil, err
			}
		}
		changes := (p.hasZoom && math.Abs(p.z0-p.z1) > 1e-9) || (p.hasPan && (math.Abs(p.u0-p.u1) > 1e-9 || math.Abs(p.v0-p.v1) > 1e-9))
		if to <= from+1e-9 {
			if changes {
				return nil, rule.E(slot, "no-travel", "a move has no duration")
			}
			continue
		}
		if from+1e-9 < cursor {
			return nil, rule.E(slot, "span", "segments overlap")
		}
		raw = append(raw, p)
		cursor = to
	}
	z, u, v := 1.0, 0.5, 0.5
	var out []model.Span
	at := 0.0
	for _, p := range raw {
		if p.from > at+1e-9 {
			out = append(out, model.Span{From: at, To: p.from, Z0: z, Z1: z, U0: u, U1: u, V0: v, V1: v})
		}
		if p.hasZoom {
			z = p.z0
		} else {
			p.z0, p.z1 = z, z
		}
		if p.hasPan {
			u, v = p.u0, p.v0
		} else {
			p.u0, p.u1, p.v0, p.v1 = u, u, v, v
		}
		out = append(out, model.Span{From: p.from, To: p.to, Z0: p.z0, Z1: p.z1, U0: p.u0, U1: p.u1, V0: p.v0, V1: p.v1})
		z, u, v = p.z1, p.u1, p.v1
		at = p.to
	}
	if at < place.ClipDur-1e-9 {
		out = append(out, model.Span{From: at, To: place.ClipDur, Z0: z, Z1: z, U0: u, U1: u, V0: v, V1: v})
	}
	if len(out) == 0 {
		return nil, rule.E(slot, "span", "clip has no spans")
	}
	return out, nil
}

func zoomRange(z doc.ZoomSpec, fill, limit float64, slot int) (float64, float64, error) {
	if z.From != nil || z.To != nil {
		a, err := zoomEnd(*z.From, fill, limit, slot)
		if err != nil {
			return 0, 0, err
		}
		b, err := zoomEnd(*z.To, fill, limit, slot)
		return a, b, err
	}
	v, err := zoomEnd(*z.End, fill, limit, slot)
	return v, v, err
}

func zoomEnd(e doc.ZoomEnd, fill, limit float64, slot int) (float64, error) {
	if len(e.Atoms) == 0 {
		return 0, rule.E(slot, "bad-number", "zoom endpoint is empty")
	}
	m := math.Inf(1)
	for _, a := range e.Atoms {
		v, err := zoomAtom(a, fill, limit, slot)
		if err != nil {
			return 0, err
		}
		if v < m {
			m = v
		}
	}
	if m < 1 {
		m = 1
	}
	return m, nil
}

func zoomAtom(a doc.ZoomAtom, fill, limit float64, slot int) (float64, error) {
	if a.IsNum {
		if a.Num < 1 {
			return 0, rule.E(slot, "bad-number", "zoom %v is below 1", a.Num)
		}
		return a.Num, nil
	}
	switch a.Sym {
	case "full":
		return 1, nil
	case "fill":
		return fill, nil
	case "limit":
		return limit, nil
	default:
		return 0, rule.E(slot, "bad-number", "zoom %q", a.Sym)
	}
}

func panRange(p doc.PanSpec, slot int) (u0, v0, u1, v1 float64, err error) {
	if p.From != nil || p.To != nil {
		u0, v0, err = panEnd(*p.From, slot)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		u1, v1, err = panEnd(*p.To, slot)
		return u0, v0, u1, v1, err
	}
	u0, v0, err = panEnd(*p.End, slot)
	return u0, v0, u0, v0, err
}

func panEnd(e doc.PanEnd, slot int) (float64, float64, error) {
	if e.HasUV {
		return e.U, e.V, nil
	}
	if uv, ok := panUV[e.Name]; ok {
		return uv[0], uv[1], nil
	}
	return 0, 0, rule.E(slot, "bad-number", "pan %q", e.Name)
}

func n(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

var panUV = map[string][2]float64{
	"top-left": {0, 0}, "top": {0.5, 0}, "top-right": {1, 0},
	"left": {0, 0.5}, "center": {0.5, 0.5}, "right": {1, 0.5},
	"bottom-left": {0, 1}, "bottom": {0.5, 1}, "bottom-right": {1, 1},
}
