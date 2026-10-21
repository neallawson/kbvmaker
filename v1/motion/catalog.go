package motion

import "kbvmaker/v1/doc"

func builtin(name string, hold float64) ([]doc.Segment, string, bool) {
	switch name {
	case "hold":
		return []doc.Segment{rest(doc.ZoomConst(doc.Sym("full")), doc.PanNamed("center"))}, "reveal", true
	case "in-center":
		return zoomIn(hold, doc.PanNamed("center"), doc.PanNamed("center")), "reveal", true
	case "in-left-right":
		return zoomIn(hold, doc.PanNamed("left"), doc.PanRamp("left", "right")), "reveal", true
	case "in-right-left":
		return zoomIn(hold, doc.PanNamed("right"), doc.PanRamp("right", "left")), "reveal", true
	case "in-top-bottom":
		return zoomIn(hold, doc.PanNamed("top"), doc.PanRamp("top", "bottom")), "reveal", true
	case "in-bottom-top":
		return zoomIn(hold, doc.PanNamed("bottom"), doc.PanRamp("bottom", "top")), "reveal", true
	case "out-center":
		return zoomOut(hold), "reveal", true
	default:
		return nil, "", false
	}
}

func zoomIn(hold float64, holdPan, travelPan *doc.PanSpec) []doc.Segment {
	travel := rest(zoomTravel(), travelPan)
	if hold <= 0 {
		travel.From = doc.Anchor("opaque-start", 0, false)
		return []doc.Segment{travel}
	}
	return []doc.Segment{
		{
			From:       doc.Anchor("opaque-start", 0, false),
			HasDur:     true,
			DurSeconds: hold,
			Zoom:       doc.ZoomConst(doc.Sym("full")),
			Pan:        holdPan,
		},
		travel,
	}
}

func zoomOut(hold float64) []doc.Segment {
	segs := []doc.Segment{{
		From: doc.Anchor("clip-start", 0, false),
		To:   doc.Anchor("opaque-end", -hold, true),
		Zoom: doc.ZoomRamp(doc.EndMin(doc.Sym("fill"), doc.Sym("limit")), doc.EndSym("full")),
		Pan:  doc.PanNamed("center"),
	}}
	segs = append(segs, rest(doc.ZoomConst(doc.Sym("full")), doc.PanNamed("center")))
	return segs
}

func zoomTravel() *doc.ZoomSpec {
	return doc.ZoomRamp(doc.EndSym("full"), doc.EndMin(doc.Sym("fill"), doc.Sym("limit")))
}

func rest(z *doc.ZoomSpec, p *doc.PanSpec) doc.Segment {
	return doc.Segment{HasDur: true, DurRest: true, Zoom: z, Pan: p}
}
