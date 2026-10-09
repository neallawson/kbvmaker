// Package compile turns a resolved plan into ffmpeg and ffprobe argument lists.
package compile

import (
	"fmt"
	"strconv"
	"strings"

	"neal/kbvmaker/v1/model"
)

// Command is one process the engine or an export would run.
type Command struct {
	Stage string
	Argv  []string
}

// Options controls binary names and how large one xfade graph may be.
type Options struct {
	FFmpeg        string
	FFProbe       string
	MaxJoinInputs int
}

func (o Options) norm() Options {
	if o.FFmpeg == "" {
		o.FFmpeg = "ffmpeg"
	}
	if o.FFProbe == "" {
		o.FFProbe = "ffprobe"
	}
	if o.MaxJoinInputs < 2 {
		o.MaxJoinInputs = 32
	}
	return o
}

// Commands is the render plan: a filter check, a duration probe, canvases,
// clips, joins, and the audio mux.
func Commands(p *model.Plan, opt Options) ([]Command, error) {
	opt = opt.norm()
	if len(p.Slots) == 0 {
		return nil, fmt.Errorf("plan has no slots")
	}
	var cmds []Command
	cmds = append(cmds, Command{Stage: "filters", Argv: []string{opt.FFmpeg, "-hide_banner", "-filters"}})
	cmds = append(cmds, Command{Stage: "probe", Argv: []string{
		opt.FFProbe, "-v", "error", "-show_entries", "format=duration", "-of", "json", p.Audio,
	}})
	canvasOf := map[string]string{}
	n := 0
	for _, s := range p.Slots {
		if _, ok := canvasOf[s.Src]; ok {
			continue
		}
		path := fmt.Sprintf("work/canvas-%d.png", n)
		canvasOf[s.Src] = path
		n++
		cmds = append(cmds, Command{Stage: "canvas", Argv: []string{
			opt.FFmpeg, "-y", "-i", s.Src,
			"-filter_complex", canvasFilter(p.Canvas),
			"-frames:v", "1", "-update", "1", path,
		}})
	}
	clips := make([]string, len(p.Slots))
	for i, s := range p.Slots {
		clips[i] = fmt.Sprintf("work/clip-%03d.mp4", i)
		cmds = append(cmds, Command{Stage: "clip", Argv: []string{
			opt.FFmpeg, "-y", "-loop", "1", "-framerate", strconv.Itoa(p.Output.FPS),
			"-i", canvasOf[s.Src],
			"-vf", zoomFilter(s, p.Output),
			"-t", num(s.ClipDur),
			"-c:v", p.Output.VideoCodec, "-preset", p.Output.Preset, "-crf", num(p.Output.CRF),
			clips[i],
		}})
	}
	video := clips[0]
	if len(p.Slots) > 1 {
		nodes := make([]node, len(p.Slots))
		joins := make([]float64, len(p.Slots)-1)
		for i, s := range p.Slots {
			nodes[i] = node{path: clips[i], dur: s.ClipDur}
			if i < len(joins) {
				if s.Join == nil {
					return nil, fmt.Errorf("slot %d is missing a join", s.Index)
				}
				joins[i] = s.Join.Duration
			}
		}
		var joinCmds []Command
		seq := 0
		name := func() string {
			path := fmt.Sprintf("work/join-%03d.mp4", seq)
			seq++
			return path
		}
		final, err := joinAll(&joinCmds, nodes, joins, opt, name, p)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, joinCmds...)
		video = final.path
	}
	cmds = append(cmds, Command{Stage: "mux", Argv: []string{
		opt.FFmpeg, "-y", "-i", video, "-i", p.Audio,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "copy", "-c:a", p.Output.AudioCodec, "-b:a", strconv.Itoa(p.Output.AudioBitrate),
		"-shortest", "-movflags", "+faststart",
		p.OutputFile,
	}})
	return cmds, nil
}

func canvasFilter(c model.Canvas) string {
	return fmt.Sprintf(
		"[0:v]split[sharp][blur];"+
			"[blur]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,gblur=sigma=%s,setsar=1[blurred];"+
			"[sharp]scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,setsar=1[fitted];"+
			"[blurred][fitted]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)/2",
		c.Width, c.Height, c.Width, c.Height, num(c.Blur), c.Width, c.Height)
}

func zoomFilter(s model.Slot, out model.Output) string {
	z := pieceExpr(s.Spans, func(sp model.Span) (float64, float64) { return sp.Z0, sp.Z1 })
	x, y := "iw/2-(iw/zoom/2)", "ih/2-(ih/zoom/2)"
	if !centerPan(s.Spans) {
		u := pieceExpr(s.Spans, func(sp model.Span) (float64, float64) { return sp.U0, sp.U1 })
		v := pieceExpr(s.Spans, func(sp model.Span) (float64, float64) { return sp.V0, sp.V1 })
		x = fmt.Sprintf("min(max(%s+(%s)*%s-iw/(2*zoom),0),iw-iw/zoom)", num(s.Fit.X0), u, num(s.Fit.DW))
		y = fmt.Sprintf("min(max(%s+(%s)*%s-ih/(2*zoom),0),ih-ih/zoom)", num(s.Fit.Y0), v, num(s.Fit.DH))
	}
	return fmt.Sprintf("zoompan=z='%s':x='%s':y='%s':d=1:s=%dx%d:fps=%d,format=yuv420p",
		z, x, y, out.Width, out.Height, out.FPS)
}

func centerPan(spans []model.Span) bool {
	for _, sp := range spans {
		if sp.U0 != 0.5 || sp.U1 != 0.5 || sp.V0 != 0.5 || sp.V1 != 0.5 {
			return false
		}
	}
	return true
}

func pieceExpr(spans []model.Span, ends func(model.Span) (float64, float64)) string {
	_, z1 := ends(spans[len(spans)-1])
	expr := num(z1)
	for i := len(spans) - 1; i >= 0; i-- {
		a, b := ends(spans[i])
		inner := num(a)
		if a != b {
			inner = ramp(spans[i].From, spans[i].To, a, b)
		}
		expr = fmt.Sprintf("if(lt(time,%s),%s,%s)", num(spans[i].To), inner, expr)
	}
	return expr
}

func ramp(from, to, a, b float64) string {
	dur := to - from
	if from == 0 {
		return fmt.Sprintf("%s+(%s-%s)*(time/%s)", num(a), num(b), num(a), num(dur))
	}
	return fmt.Sprintf("%s+(%s-%s)*((time-%s)/%s)", num(a), num(b), num(a), num(from), num(dur))
}

func num(f float64) string {
	s := strconv.FormatFloat(f, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

type node struct {
	path string
	dur  float64
}

func joinAll(cmds *[]Command, nodes []node, joins []float64, opt Options, name func() string, p *model.Plan) (node, error) {
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	if len(nodes) <= opt.MaxJoinInputs {
		return emitJoin(cmds, nodes, joins, name, p, opt.FFmpeg), nil
	}
	var mids []node
	var midJoins []float64
	for start := 0; start < len(nodes); {
		end := start + opt.MaxJoinInputs
		if end > len(nodes) {
			end = len(nodes)
		}
		var subJoins []float64
		if end-start > 1 {
			subJoins = joins[start : end-1]
		}
		sub, err := joinAll(cmds, nodes[start:end], subJoins, opt, name, p)
		if err != nil {
			return node{}, err
		}
		if len(mids) > 0 {
			midJoins = append(midJoins, joins[start-1])
		}
		mids = append(mids, sub)
		start = end
	}
	return joinAll(cmds, mids, midJoins, opt, name, p)
}

func emitJoin(cmds *[]Command, nodes []node, joins []float64, name func() string, p *model.Plan, ff string) node {
	starts := make([]float64, len(nodes))
	t := 0.0
	for i := range nodes {
		starts[i] = t
		if i < len(nodes)-1 {
			t += nodes[i].dur - joins[i]
		} else {
			t += nodes[i].dur
		}
	}
	var b strings.Builder
	for i := 0; i < len(nodes)-1; i++ {
		if i > 0 {
			b.WriteByte(';')
		}
		left := fmt.Sprintf("[%d:v]", i)
		if i > 0 {
			left = fmt.Sprintf("[x%d]", i)
		}
		label := fmt.Sprintf("[x%d]", i+1)
		if i == len(nodes)-2 {
			label = "[v]"
		}
		fmt.Fprintf(&b, "%s[%d:v]xfade=transition=fade:duration=%s:offset=%s%s",
			left, i+1, num(joins[i]), num(starts[i+1]), label)
	}
	out := name()
	argv := []string{ff, "-y"}
	for _, n := range nodes {
		argv = append(argv, "-i", n.path)
	}
	argv = append(argv,
		"-filter_complex", b.String(),
		"-map", "[v]",
		"-c:v", p.Output.VideoCodec, "-preset", p.Output.Preset, "-crf", num(p.Output.CRF),
		"-pix_fmt", p.Output.PixFmt,
		"-t", num(t),
		out,
	)
	*cmds = append(*cmds, Command{Stage: "join", Argv: argv})
	return node{path: out, dur: t}
}
