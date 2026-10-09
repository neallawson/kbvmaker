// Package check applies the Montage rule table to a resolved plan.
package check

import (
	"math"

	"neal/kbvmaker/v1/model"
	"neal/kbvmaker/v1/rule"
)

// All rejects a plan that breaks a render rule.
// scan slots skip the reveal rule and still run the others.
func All(p *model.Plan) error {
	for i, s := range p.Slots {
		if s.OpaqueEnd <= s.OpaqueStart+1e-9 {
			return rule.E(s.Index, "dissolve", "the picture is never fully opaque")
		}
		if s.Join != nil && s.Join.Duration >= s.ClipDur-1e-9 {
			return rule.E(s.Index, "dissolve", "join is not shorter than the clip")
		}
		if i+1 < len(p.Slots) && s.Join != nil && s.Join.Duration >= p.Slots[i+1].ClipDur-1e-9 {
			return rule.E(s.Index, "dissolve", "join is not shorter than the next clip")
		}
		if s.Family == "reveal" && minZoom(s) > 1.001 {
			return rule.E(s.Index, "reveal", "zoom stays above the whole photograph while the picture is opaque")
		}
		for _, sp := range s.Spans {
			if sp.Z0 > float64(p.Canvas.Width) || sp.Z1 > float64(p.Canvas.Width) {
				return rule.E(s.Index, "bad-number", "zoom exceeds the canvas width")
			}
			if sp.Z0 < 1-1e-9 || sp.Z1 < 1-1e-9 {
				return rule.E(s.Index, "bad-number", "zoom drops below 1")
			}
		}
	}
	return nil
}

func minZoom(s model.Slot) float64 {
	m := math.Inf(1)
	for _, sp := range s.Spans {
		a := math.Max(sp.From, s.OpaqueStart)
		b := math.Min(sp.To, s.OpaqueEnd)
		if b < a-1e-9 {
			continue
		}
		m = math.Min(m, zoomAt(sp, a))
		m = math.Min(m, zoomAt(sp, b))
	}
	return m
}

func zoomAt(sp model.Span, t float64) float64 {
	if sp.To <= sp.From {
		return sp.Z0
	}
	u := (t - sp.From) / (sp.To - sp.From)
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return sp.Z0 + (sp.Z1-sp.Z0)*u
}
