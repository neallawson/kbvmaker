// Package timing turns a slot list and a song duration into clip windows.
package timing

import (
	"math"

	"kbvmaker/v1/clock"
	"kbvmaker/v1/doc"
	"kbvmaker/v1/model"
	"kbvmaker/v1/rule"
)

// Resolve returns one window per slot and the output video duration.
func Resolve(d *doc.Document, slots []doc.Slot, song float64) ([]model.Window, float64, error) {
	if len(slots) == 0 {
		return nil, 0, rule.E(0, "missing-field", "slots")
	}
	var durs []float64
	var joins []float64
	var err error
	switch d.Timing.Policy {
	case "fit-audio":
		durs, joins, err = fitAudio(d, slots, song)
	case "fixed-clip":
		durs, joins, err = equal(d, slots, song, d.Timing.Clip, false)
	case "explicit":
		durs, joins, err = explicit(d, slots)
	case "markers":
		durs, joins, err = markers(d, slots, song)
	default:
		err = rule.E(0, "unknown-field", "timing policy %q", d.Timing.Policy)
	}
	if err != nil {
		return nil, 0, err
	}
	return windows(durs, joins)
}

func fitAudio(d *doc.Document, slots []doc.Slot, song float64) ([]float64, []float64, error) {
	x, err := uniform(d, slots)
	if err != nil {
		return nil, nil, err
	}
	n := len(slots)
	clock := "song"
	if d.Audio == "" {
		clock = "film"
	}
	if n == 1 {
		if slots[0].Dur != nil && math.Abs(*slots[0].Dur-song) > 1e-6 {
			return nil, nil, rule.E(1, "bad-time", "the only picture runs for the whole %s", clock)
		}
		return []float64{song}, nil, nil
	}
	sumP := 0.0
	pinned := 0
	lastPin := 0
	for i, s := range slots {
		if s.Dur == nil {
			continue
		}
		if *s.Dur <= x {
			return nil, nil, rule.E(i+1, "bad-time", "picture length %v is not longer than the dissolve %v", *s.Dur, x)
		}
		sumP += *s.Dur
		pinned++
		lastPin = i + 1
	}
	if pinned == n {
		return nil, nil, rule.E(n, "bad-time", "leave one picture sharing the %s", clock)
	}
	shared := (song + float64(n-1)*x - sumP) / float64(n-pinned)
	if shared <= x {
		return nil, nil, rule.E(lastPin, "bad-time", "the other pictures would be no longer than the dissolve")
	}
	durs := make([]float64, n)
	for i, s := range slots {
		if s.Dur != nil {
			durs[i] = *s.Dur
			continue
		}
		durs[i] = shared
	}
	joins := make([]float64, n-1)
	for i := range joins {
		joins[i] = x
	}
	return durs, joins, nil
}

func equal(d *doc.Document, slots []doc.Slot, song, clip float64, fromSong bool) ([]float64, []float64, error) {
	x, err := uniform(d, slots)
	if err != nil {
		return nil, nil, err
	}
	n := len(slots)
	if fromSong {
		if n == 1 {
			clip = song
		} else {
			clip = (song + float64(n-1)*x) / float64(n)
		}
	}
	if n > 1 && clip <= x {
		return nil, nil, rule.E(0, "bad-number", "clip %v is not longer than the join %v", clip, x)
	}
	durs := make([]float64, n)
	for i, s := range slots {
		dur := clip
		if !fromSong && s.Dur != nil {
			dur = *s.Dur
		}
		if n > 1 && dur <= x {
			return nil, nil, rule.E(i+1, "bad-time", "picture length %v is not longer than the dissolve %v", dur, x)
		}
		if dur <= 0 {
			return nil, nil, rule.E(i+1, "bad-time", "dur %v", dur)
		}
		durs[i] = dur
	}
	joins := make([]float64, n-1)
	for i := range joins {
		joins[i] = x
	}
	return durs, joins, nil
}

func explicit(d *doc.Document, slots []doc.Slot) ([]float64, []float64, error) {
	durs := make([]float64, len(slots))
	for i, s := range slots {
		if s.Dur == nil || *s.Dur <= 0 {
			return nil, nil, rule.E(i+1, "bad-time", "dur")
		}
		durs[i] = *s.Dur
	}
	joins, err := eachJoin(d, slots)
	if err != nil {
		return nil, nil, err
	}
	return durs, joins, nil
}

func markers(d *doc.Document, slots []doc.Slot, song float64) ([]float64, []float64, error) {
	n := len(slots)
	if n == 1 {
		return []float64{song}, nil, nil
	}
	cuts := make([]float64, n+1)
	cuts[0] = 0
	cuts[n] = song
	for i := 0; i < n-1; i++ {
		if slots[i].Until == nil {
			return nil, nil, rule.E(i+1, "missing-field", "until")
		}
		c, err := clock.Song(*slots[i].Until, song, d.Output.FPS, i+1)
		if err != nil {
			return nil, nil, err
		}
		cuts[i+1] = c
	}
	for i := 1; i < n; i++ {
		if cuts[i] <= cuts[i-1]+1e-9 || cuts[i] >= song-1e-9 {
			return nil, nil, rule.E(i, "cuts", "cut at %v is not strictly inside the song", cuts[i])
		}
	}
	joins, err := eachJoin(d, slots)
	if err != nil {
		return nil, nil, err
	}
	durs := make([]float64, n)
	for i := 0; i < n; i++ {
		switch {
		case i == 0:
			durs[i] = cuts[1] + joins[0]/2
		case i == n-1:
			durs[i] = cuts[n] - (cuts[n-1] - joins[i-1]/2)
		default:
			start := cuts[i] - joins[i-1]/2
			end := cuts[i+1] + joins[i]/2
			durs[i] = end - start
		}
		if durs[i] <= 0 {
			return nil, nil, rule.E(i+1, "cuts", "clip duration %v", durs[i])
		}
	}
	return durs, joins, nil
}

func uniform(d *doc.Document, slots []doc.Slot) (float64, error) {
	if len(slots) < 2 {
		return d.Defaults.Join.Duration, nil
	}
	x := joinDur(d, slots[0])
	for i := 1; i < len(slots)-1; i++ {
		xi := joinDur(d, slots[i])
		if math.Abs(xi-x) > 1e-9 {
			return 0, rule.E(i+1, "dissolve", "this timing policy needs one join duration")
		}
	}
	return x, nil
}

func eachJoin(d *doc.Document, slots []doc.Slot) ([]float64, error) {
	if len(slots) < 2 {
		return nil, nil
	}
	out := make([]float64, len(slots)-1)
	for i := range out {
		out[i] = joinDur(d, slots[i])
		if out[i] <= 0 {
			return nil, rule.E(i+1, "bad-number", "join duration")
		}
	}
	return out, nil
}

func joinDur(d *doc.Document, s doc.Slot) float64 {
	if s.Join != nil {
		return s.Join.Duration
	}
	return d.Defaults.Join.Duration
}

func windows(durs, joins []float64) ([]model.Window, float64, error) {
	starts := make([]float64, len(durs))
	t := 0.0
	for i, dur := range durs {
		starts[i] = t
		if i < len(durs)-1 {
			t += dur - joins[i]
		} else {
			t += dur
		}
	}
	out := make([]model.Window, len(durs))
	for i, dur := range durs {
		in := 0.0
		if i > 0 {
			in = joins[i-1]
		}
		outDur := 0.0
		if i < len(durs)-1 {
			outDur = joins[i]
		}
		w := model.Window{
			ClipStart:   starts[i],
			ClipDur:     dur,
			OpaqueStart: in,
			OpaqueEnd:   dur - outDur,
		}
		if i < len(durs)-1 {
			w.Join = &model.Join{
				Type:     "fade",
				Duration: joins[i],
				Start:    starts[i+1],
				Mid:      starts[i+1] + joins[i]/2,
			}
		}
		out[i] = w
	}
	return out, t, nil
}
