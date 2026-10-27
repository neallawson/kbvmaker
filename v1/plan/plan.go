// Package plan resolves a Montage document into a timeline.
package plan

import (
	"context"
	"fmt"
	"os"
	"strings"

	"kbvmaker/v1/check"
	"kbvmaker/v1/clock"
	"kbvmaker/v1/doc"
	"kbvmaker/v1/frame"
	"kbvmaker/v1/media"
	"kbvmaker/v1/model"
	"kbvmaker/v1/motion"
	"kbvmaker/v1/rule"
	"kbvmaker/v1/schedule"
	"kbvmaker/v1/timing"
	"kbvmaker/v1/version"
)

// Resolve probes media, expands modes, and checks the timeline.
// Authored slots win over a pool. A pool is read only when slots are absent.
func Resolve(ctx context.Context, d *doc.Document, probe media.Probe) (*model.Plan, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	if err := requireFilters(ctx, probe); err != nil {
		return nil, err
	}
	var gaps []mediaGap
	song := 0.0
	var err error
	placed := make([]model.Track, 0, len(d.Tracks))
	for _, t := range d.Tracks {
		audio := doc.ResolvePath(d.Dir, t.Src)
		if err := exists(audio); err != nil {
			gaps = append(gaps, mediaGap{msg: fmt.Sprintf("Missing soundtrack: %s", t.Src)})
			continue
		}
		dur, err := probe.Duration(ctx, audio)
		if err != nil {
			return nil, rule.E(0, "missing-media", "Cannot read soundtrack: %s", t.Src)
		}
		if dur <= 0 {
			return nil, rule.E(0, "bad-number", "song duration %v", dur)
		}
		placed = append(placed, model.Track{Src: t.Src, Start: song + t.Gap, Dur: dur, Gap: t.Gap})
		song += t.Gap + dur
	}
	if len(d.Tracks) == 0 && d.Timing.Policy == "fit-audio" && d.Timing.HasDuration {
		song = d.Timing.Duration
	}
	slots := d.Slots
	if len(slots) == 0 && len(gaps) > 0 {
		return nil, reportMissing(gaps)
	}
	if len(slots) == 0 {
		slots, err = schedule.Generate(d, song)
		if err != nil {
			return nil, err
		}
		checkDoc := *d
		checkDoc.Slots = slots
		checkDoc.Pool = nil
		checkDoc.Schedule = nil
		checkDoc.Timing.HasCount = false
		if err := checkDoc.Validate(); err != nil {
			return nil, err
		}
	}
	for i, s := range slots {
		if s.Card != nil {
			continue
		}
		path := doc.ResolvePath(d.Dir, s.Src)
		if err := exists(path); err != nil {
			gaps = append(gaps, mediaGap{slot: i + 1, msg: fmt.Sprintf("Missing picture %d: %s", i+1, s.Src)})
		}
		bars := d.Canvas.Backdrop
		if s.Backdrop != nil {
			bars = *s.Backdrop
		}
		if bars.Kind == "image" {
			back := doc.ResolvePath(d.Dir, bars.Src)
			if err := exists(back); err != nil {
				gaps = append(gaps, mediaGap{slot: i + 1, msg: fmt.Sprintf("Missing backdrop for picture %d: %s", i+1, bars.Src)})
			}
		}
	}
	if len(gaps) > 0 {
		return nil, reportMissing(gaps)
	}
	windows, videoDur, err := timing.Resolve(d, slots, song)
	if err != nil {
		return nil, err
	}
	outFile := d.Output.File
	if outFile == "" {
		outFile = "out.mp4"
	}
	p := &model.Plan{
		Montage:    version.Current,
		Kind:       "resolved",
		Song:       song,
		Duration:   videoDur,
		Dir:        d.Dir,
		Tracks:     placed,
		OutputFile: outFile,
		Canvas: model.Canvas{
			Width: d.Canvas.Width, Height: d.Canvas.Height,
			Backdrop: modelBackdrop(d.Canvas.Backdrop),
		},
		Output: model.Output{
			Width: d.Output.Width, Height: d.Output.Height, FPS: d.Output.FPS,
			VideoCodec: d.Output.VideoCodec, CRF: d.Output.CRF, Preset: d.Output.Preset,
			PixFmt: d.Output.PixFmt, AudioCodec: d.Output.AudioCodec, AudioBitrate: d.Output.AudioBitrate,
		},
	}
	images := map[string]media.Info{}
	for i, s := range slots {
		w := windows[i]
		var info media.Info
		if s.Card != nil {
			info = media.Info{Width: d.Canvas.Width, Height: d.Canvas.Height}
		} else {
			path := doc.ResolvePath(d.Dir, s.Src)
			var ok bool
			info, ok = images[path]
			if !ok {
				info, err = probe.Image(ctx, path)
				if err != nil {
					return nil, rule.E(i+1, "missing-media", "image %s: %v", s.Src, err)
				}
				if info.Width < 1 || info.Height < 1 {
					return nil, rule.E(i+1, "missing-media", "image %s has no displayed size", s.Src)
				}
				images[path] = info
			}
		}
		fit := frame.FitRect(info.Width, info.Height, d.Canvas.Width, d.Canvas.Height)
		fill := frame.Fill(info.Width, info.Height, d.Canvas.Width, d.Canvas.Height)
		place := clock.Place{
			Song: song, ClipStart: w.ClipStart, ClipDur: w.ClipDur,
			OpaqueStart: w.OpaqueStart, OpaqueEnd: w.OpaqueEnd, FPS: d.Output.FPS,
		}
		family, mode, basedOn, spans, err := motion.Resolve(s, d.Modes, place, d.Defaults.ZoomSpan, d.Defaults.Hold, fill, i+1)
		if err != nil {
			return nil, err
		}
		far := 1.0
		for _, sp := range spans {
			if sp.Z0 > far {
				far = sp.Z0
			}
			if sp.Z1 > far {
				far = sp.Z1
			}
		}
		bars := d.Canvas.Backdrop
		if s.Backdrop != nil {
			bars = *s.Backdrop
		}
		p.Slots = append(p.Slots, model.Slot{
			Index: i + 1, Src: s.Src, Card: s.Card, Mode: mode, BasedOn: basedOn, Family: family,
			ClipStart: w.ClipStart, ClipDur: w.ClipDur,
			OpaqueStart: w.OpaqueStart, OpaqueEnd: w.OpaqueEnd,
			Fill: fill, Backdrop: modelBackdrop(bars), FarZoom: far, Join: w.Join, Spans: spans, Fit: fit,
		})
	}
	if err := requireBackdropFilters(ctx, probe, p); err != nil {
		return nil, err
	}
	if err := check.All(p); err != nil {
		return nil, err
	}
	return p, nil
}

func modelBackdrop(b doc.Backdrop) model.Backdrop {
	return model.Backdrop{Kind: b.Kind, Blur: b.Sigma(), Color: b.Color, Name: b.Name, Src: b.Src}
}

func requireBackdropFilters(ctx context.Context, probe media.Probe, p *model.Plan) error {
	need := map[string]bool{}
	for _, s := range p.Slots {
		switch s.Backdrop.Kind {
		case "color":
			need["color"] = true
		case "pattern":
			if s.Backdrop.Name == "grain" {
				need["color"] = true
				need["noise"] = true
			}
			if s.Backdrop.Name == "gradient" {
				need["gradients"] = true
			}
		}
	}
	if len(need) == 0 {
		return nil
	}
	have, err := probe.Filters(ctx)
	if err != nil {
		return err
	}
	for name := range need {
		if !have[name] {
			return rule.E(0, "missing-media", "ffmpeg has no %s filter", name)
		}
	}
	return nil
}

func requireFilters(ctx context.Context, probe media.Probe) error {
	have, err := probe.Filters(ctx)
	if err != nil {
		return err
	}
	for _, name := range media.RequiredFilters {
		if !have[name] {
			return rule.E(0, "missing-media", "ffmpeg has no %s filter", name)
		}
	}
	return nil
}

func exists(path string) error {
	_, err := os.Stat(path)
	return err
}

// mediaGap is one soundtrack, picture, or backdrop that is not on disk.
type mediaGap struct {
	slot int
	msg  string
}

// reportMissing returns every gap. One gap keeps that picture's slot. Several gaps are one document error so the list stays together.
func reportMissing(gaps []mediaGap) error {
	slot := 0
	if len(gaps) == 1 {
		slot = gaps[0].slot
	}
	msgs := make([]string, len(gaps))
	for i, gap := range gaps {
		msgs[i] = gap.msg
	}
	return rule.E(slot, "missing-media", "%s", strings.Join(msgs, "\n"))
}
