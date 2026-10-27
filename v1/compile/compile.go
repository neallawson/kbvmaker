// Package compile turns a resolved plan into ffmpeg and ffprobe argument lists.
package compile

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"kbvmaker/v1/card"
	"kbvmaker/v1/model"
)

// Command is one process the engine or an export would run.
// A card stage writes File from Body instead of running Argv.
type Command struct {
	Stage string
	Argv  []string
	File  string
	Body  []byte
}

// Options controls binary names and how large one xfade graph may be.
// Verbose keeps ffmpeg's banner and warnings; otherwise only errors and the
// progress line are printed.
type Options struct {
	FFmpeg        string
	FFProbe       string
	MaxJoinInputs int
	Verbose       bool
	// Work is the directory for canvases, clips, and joins, relative to the
	// project directory unless absolute. Empty means "work".
	Work string
}

// ffmpeg starts an ffmpeg argv that overwrites its output.
func (o Options) ffmpeg() []string {
	if o.Verbose {
		return []string{o.FFmpeg, "-y"}
	}
	return []string{o.FFmpeg, "-hide_banner", "-loglevel", "error", "-stats", "-y"}
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
	if o.Work == "" {
		o.Work = "work"
	}
	return o
}

// Commands is the render plan: a filter check, a soundtrack probe when the
// film has one, canvases, clips, joins, and the mux.
func Commands(p *model.Plan, opt Options) ([]Command, error) {
	opt = opt.norm()
	if len(p.Slots) == 0 {
		return nil, fmt.Errorf("plan has no slots")
	}
	var cmds []Command
	cmds = append(cmds, Command{Stage: "filters", Argv: []string{opt.FFmpeg, "-hide_banner", "-filters"}})
	for _, t := range p.Tracks {
		cmds = append(cmds, Command{Stage: "probe", Argv: []string{
			opt.FFProbe, "-v", "error", "-show_entries", "format=duration", "-of", "json", t.Src,
		}})
	}
	audio := ""
	if len(p.Tracks) == 1 && p.Tracks[0].Gap == 0 {
		audio = p.Tracks[0].Src
	} else if len(p.Tracks) > 0 {
		mix := soundtrackCommand(p, opt)
		cmds = append(cmds, mix)
		audio = filepath.Join(opt.Work, "soundtrack.wav")
	}
	canvasOf := map[string]string{}
	n := 0
	cards := 0
	for _, s := range p.Slots {
		key := canvasKey(s)
		if _, ok := canvasOf[key]; ok {
			continue
		}
		if s.Card != nil {
			path := filepath.Join(opt.Work, fmt.Sprintf("card-%d.png", cards))
			png, err := card.PNG(*s.Card, p.Canvas.Width, p.Canvas.Height)
			if err != nil {
				return nil, fmt.Errorf("slot %d card: %w", s.Index, err)
			}
			canvasOf[key] = path
			cards++
			cmds = append(cmds, Command{Stage: "card", File: path, Body: png})
			continue
		}
		path := filepath.Join(opt.Work, fmt.Sprintf("canvas-%d.png", n))
		canvasOf[key] = path
		n++
		args := append(opt.ffmpeg(), "-i", s.Src)
		if s.Backdrop.Kind == "image" {
			args = append(args, "-i", s.Backdrop.Src)
		}
		args = append(args, "-filter_complex", canvasFilter(p.Canvas, s.Backdrop), "-frames:v", "1", "-update", "1", path)
		cmds = append(cmds, Command{Stage: "canvas", Argv: args})
	}
	clips := make([]string, len(p.Slots))
	for i, s := range p.Slots {
		clips[i] = filepath.Join(opt.Work, fmt.Sprintf("clip-%03d.mp4", i))
		cmds = append(cmds, Command{Stage: "clip", Argv: append(opt.ffmpeg(),
			"-loop", "1", "-framerate", strconv.Itoa(p.Output.FPS),
			"-i", canvasOf[canvasKey(s)],
			"-vf", zoomFilter(s, p.Output),
			"-t", num(s.ClipDur),
			"-c:v", p.Output.VideoCodec, "-preset", p.Output.Preset, "-crf", num(p.Output.CRF),
			clips[i],
		)})
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
			path := filepath.Join(opt.Work, fmt.Sprintf("join-%03d.mp4", seq))
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
	mux := append(opt.ffmpeg(), "-i", video)
	if audio != "" {
		mux = append(mux, "-i", audio,
			"-map", "0:v:0", "-map", "1:a:0",
			"-c:v", "copy", "-c:a", p.Output.AudioCodec, "-b:a", strconv.Itoa(p.Output.AudioBitrate),
			"-shortest")
	} else {
		mux = append(mux, "-map", "0:v:0", "-c:v", "copy", "-an")
	}
	mux = append(mux, "-movflags", "+faststart", p.OutputFile)
	cmds = append(cmds, Command{Stage: "mux", Argv: mux})
	return cmds, nil
}

// soundtrackCommand concatenates the files and the silences before them into one wav.
func soundtrackCommand(p *model.Plan, opt Options) Command {
	args := opt.ffmpeg()
	var filter strings.Builder
	labels := make([]string, 0, len(p.Tracks)*2)
	input := 0
	for i, t := range p.Tracks {
		if t.Gap > 0 {
			label := fmt.Sprintf("g%d", i)
			fmt.Fprintf(&filter, "anullsrc=r=48000:cl=stereo,atrim=0:%s,asetpts=PTS-STARTPTS[%s];", num(t.Gap), label)
			labels = append(labels, "["+label+"]")
		}
		args = append(args, "-i", t.Src)
		label := fmt.Sprintf("a%d", i)
		fmt.Fprintf(&filter, "[%d:a]aformat=sample_fmts=s16:sample_rates=48000:channel_layouts=stereo,asetpts=PTS-STARTPTS[%s];", input, label)
		labels = append(labels, "["+label+"]")
		input++
	}
	fmt.Fprintf(&filter, "%sconcat=n=%d:v=0:a=1[a]", strings.Join(labels, ""), len(labels))
	args = append(args, "-filter_complex", filter.String(), "-map", "[a]", "-c:a", "pcm_s16le", filepath.Join(opt.Work, "soundtrack.wav"))
	return Command{Stage: "soundtrack", Argv: args}
}

func canvasKey(s model.Slot) string {
	if s.Card != nil {
		return "card\x00" + fmt.Sprintf("%#v", *s.Card)
	}
	b := s.Backdrop
	return s.Src + "\x00" + b.Kind + "\x00" + b.Color + "\x00" + b.Name + "\x00" + b.Src + "\x00" + num(b.Blur)
}

func canvasFilter(c model.Canvas, b model.Backdrop) string {
	w, h := c.Width, c.Height
	sharp := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,setsar=1[fitted]", w, h)
	over := "[bg][fitted]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)/2"
	switch b.Kind {
	case "color":
		return fmt.Sprintf("color=c=%s:s=%dx%d:d=1,setsar=1[bg];%s;%s", ffmpegColor(b.Color), w, h, sharp, over)
	case "pattern":
		if b.Name == "grain" {
			return fmt.Sprintf("color=c=%s:s=%dx%d:d=1,noise=alls=20:allf=u:all_seed=1,setsar=1[bg];%s;%s",
				ffmpegColor(b.Color), w, h, sharp, over)
		}
		return fmt.Sprintf("gradients=s=%dx%d:n=2:c0=%s:c1=0x000000:x0=0:y0=0:x1=0:y1=%d:type=linear:speed=0:seed=1:d=1,setsar=1[bg];%s;%s",
			w, h, ffmpegColor(b.Color), h, sharp, over)
	case "image":
		bg := fmt.Sprintf("[1:v]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d", w, h, w, h)
		if b.Blur > 0 {
			bg += ",gblur=sigma=" + num(b.Blur)
		}
		return bg + ",setsar=1[bg];" + sharp + ";" + over
	default:
		blur := fmt.Sprintf("[blur]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d", w, h, w, h)
		if b.Blur > 0 {
			blur += ",gblur=sigma=" + num(b.Blur)
		}
		return "[0:v]split[sharp][blur];" + blur + ",setsar=1[blurred];" +
			fmt.Sprintf("[sharp]scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,setsar=1[fitted];", w, h) +
			"[blurred][fitted]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)/2"
	}
}

func ffmpegColor(hex string) string {
	if len(hex) == 7 && hex[0] == '#' {
		return "0x" + hex[1:]
	}
	return "0x000000"
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
		return emitJoin(cmds, nodes, joins, name, p, opt.ffmpeg()), nil
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

func emitJoin(cmds *[]Command, nodes []node, joins []float64, name func() string, p *model.Plan, start []string) node {
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
	argv := start
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
