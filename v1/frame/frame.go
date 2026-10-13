// Package frame is the geometry of one photograph on the working canvas.
package frame

import "kbvmaker/v1/model"

// Fill is the zoom that just covers the canvas aspect.
// w and h are the photograph's displayed size. W and H are the canvas.
func Fill(w, h, W, H int) float64 {
	if w*H >= h*W {
		return (float64(w) / float64(h)) / (float64(W) / float64(H))
	}
	return (float64(W) / float64(H)) / (float64(w) / float64(h))
}

// FitRect is the sharp copy centered inside the canvas, scaled to fit.
func FitRect(w, h, W, H int) model.Fit {
	scale := float64(W) / float64(w)
	if s := float64(H) / float64(h); s < scale {
		scale = s
	}
	dw := float64(w) * scale
	dh := float64(h) * scale
	return model.Fit{
		X0: (float64(W) - dw) / 2,
		Y0: (float64(H) - dh) / 2,
		DW: dw,
		DH: dh,
	}
}

// Window is the top-left of the zoompan window for subject (u, v) at zoom z.
// At zoom 1 the only legal window is the origin, for every subject point.
func Window(f model.Fit, canvasW, canvasH int, z, u, v float64) (x, y float64) {
	W := float64(canvasW)
	H := float64(canvasH)
	px := f.X0 + u*f.DW
	py := f.Y0 + v*f.DH
	x = clamp(px-W/(2*z), 0, W-W/z)
	y = clamp(py-H/(2*z), 0, H-H/z)
	return x, y
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
