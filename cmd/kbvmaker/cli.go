package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"kbvmaker/v1"
	"kbvmaker/v1/doc"
	"kbvmaker/v1/media"
	"kbvmaker/v1/model"
	"kbvmaker/web"
)

// env is everything the CLI touches outside its arguments, so tests can replace it.
type env struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	getwd  func() (string, error)
	probe  func(ffmpeg, ffprobe string) (media.Probe, error)
	listen func(srv *http.Server) error
}

func osEnv() env {
	return env{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		getwd:  os.Getwd,
		probe:  ffProbe,
		listen: func(srv *http.Server) error { return srv.ListenAndServe() },
	}
}

func ffProbe(ffmpeg, ffprobe string) (media.Probe, error) {
	ff, err := exec.LookPath(ffmpeg)
	if err != nil {
		return nil, toolError{fmt.Errorf("ffmpeg not found: %s", ffmpeg)}
	}
	fp, err := exec.LookPath(ffprobe)
	if err != nil {
		return nil, toolError{fmt.Errorf("ffprobe not found: %s", ffprobe)}
	}
	return &media.FFProbe{Probe: fp, FFmpeg: ff}, nil
}

// opts holds every flag a document command may register.
type opts struct {
	json     bool
	dir      string
	output   string
	ffmpeg   string
	ffprobe  string
	verbose  bool
	work     string
	clean    bool
	progress string
	media    bool
	audio    string
	ov       v1.Override
}

// command is one document command: plan, validate, export-script, export-plan, render.
type command struct {
	name    string
	summary string
	usage   string
	flags   func(fs *flag.FlagSet, o *opts)
	run     func(ctx context.Context, j *job) error
}

var commands = map[string]*command{}

var order = []string{"validate", "plan", "export-script", "export-plan", "render"}

func init() {
	for _, c := range []*command{
		{
			name:    "validate",
			summary: "check a Montage document",
			usage:   "kbvmaker validate [flags] FILE|-",
			flags: func(fs *flag.FlagSet, o *opts) {
				docFlags(fs, o)
				fs.BoolVar(&o.media, "media", false, "also probe the song, the pictures, and the ffmpeg filters")
			},
			run: validate,
		},
		{
			name:    "plan",
			summary: "print the resolved timeline as JSON",
			usage:   "kbvmaker plan [flags] FILE|-",
			flags: func(fs *flag.FlagSet, o *opts) {
				docFlags(fs, o)
				outputFlag(fs, o, "write the plan to `FILE` instead of stdout")
			},
			run: planCmd,
		},
		{
			name:    "export-script",
			summary: "print the render as a bash script",
			usage:   "kbvmaker export-script [flags] FILE|-",
			flags: func(fs *flag.FlagSet, o *opts) {
				docFlags(fs, o)
				outputFlag(fs, o, "write the script to `FILE` (mode 0755) instead of stdout")
				renderFlags(fs, o)
			},
			run: exportScript,
		},
		{
			name:    "export-plan",
			summary: "print the render commands as renderPlan JSON",
			usage:   "kbvmaker export-plan [flags] FILE|-",
			flags: func(fs *flag.FlagSet, o *opts) {
				docFlags(fs, o)
				outputFlag(fs, o, "write the renderPlan to `FILE` instead of stdout")
				renderFlags(fs, o)
			},
			run: exportPlan,
		},
		{
			name:    "render",
			summary: "run ffmpeg and write the video",
			usage:   "kbvmaker render [flags] FILE|-",
			flags: func(fs *flag.FlagSet, o *opts) {
				docFlags(fs, o)
				outputFlag(fs, o, "write the video to `FILE` (overrides output.file)")
				renderFlags(fs, o)
				fs.BoolVar(&o.clean, "clean", false, "delete the intermediate files after a successful render")
				fs.StringVar(&o.progress, "progress", "text", "progress on stderr, `MODE` text, json, or none")
			},
			run: render,
		},
	} {
		commands[c.name] = c
	}
}

func docFlags(fs *flag.FlagSet, o *opts) {
	fs.BoolVar(&o.json, "json", false, "print errors (and results that are not files) as JSON")
	fs.StringVar(&o.dir, "dir", "", "project `DIR` for a document read from stdin (default: current directory)")
	fs.StringVar(&o.ffmpeg, "ffmpeg", "ffmpeg", "ffmpeg `BINARY`")
	fs.StringVar(&o.ffprobe, "ffprobe", "ffprobe", "ffprobe `BINARY`")
	fs.StringVar(&o.audio, "audio", "", "use this song `FILE` instead of the document's")
	floatFlag(fs, "join", "dissolve length in `SECONDS`", &o.ov.Join)
	floatFlag(fs, "hold", "still time at each end of a move, in `SECONDS`", &o.ov.Hold)
	floatFlag(fs, "zoom-span", "how far a move zooms (`N`, above 0)", &o.ov.ZoomSpan)
	floatFlag(fs, "blur", "blur sigma for a blurred backdrop (`N`)", &o.ov.Blur)
	fs.Func("seed", "shuffle `SEED` for a picture pool", func(s string) error {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("not a whole number: %s", s)
		}
		o.ov.Seed = &n
		return nil
	})
	fs.StringVar(&o.ov.Schedule, "schedule", "", "pool schedule `POLICY`: given, shuffle, or loop")
}

func outputFlag(fs *flag.FlagSet, o *opts, usage string) {
	fs.StringVar(&o.output, "o", "", usage)
	fs.StringVar(&o.output, "output", "", "same as -o `FILE`")
}

func renderFlags(fs *flag.FlagSet, o *opts) {
	fs.BoolVar(&o.verbose, "verbose", false, "keep ffmpeg's banner and warnings")
	fs.StringVar(&o.work, "work", "work", "intermediate `DIR`, relative to the project directory unless absolute")
}

func floatFlag(fs *flag.FlagSet, name, usage string, dst **float64) {
	fs.Func(name, usage, func(s string) error {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("not a number: %s", s)
		}
		*dst = &v
		return nil
	})
}

// cli runs one invocation and returns the exit code.
func cli(ctx context.Context, args []string, e env) int {
	if len(args) == 0 {
		fmt.Fprint(e.stderr, topHelp())
		return exitUsage
	}
	name, rest := args[0], args[1:]
	switch name {
	case "help", "-h", "-help", "--help":
		return help(rest, e)
	case "version", "-version", "--version":
		return versionCmd(rest, e)
	case "serve":
		return serve(ctx, rest, e)
	}
	c, ok := commands[name]
	if !ok {
		return report(e.stderr, usagef("unknown command %q; run kbvmaker help", name), wantsJSON(rest))
	}
	return runCommand(ctx, c, rest, e)
}

func newOpts() *opts {
	return &opts{ffmpeg: "ffmpeg", ffprobe: "ffprobe", work: "work", progress: "text"}
}

func flagSet(c *command, o *opts) *flag.FlagSet {
	fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	c.flags(fs, o)
	return fs
}

func runCommand(ctx context.Context, c *command, args []string, e env) int {
	o := newOpts()
	fs := flagSet(c, o)
	pos, err := parse(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		commandHelp(e.stdout, c, fs)
		return exitOK
	}
	asJSON := o.json || wantsJSON(args)
	if err != nil {
		return report(e.stderr, usagef("%s: %v (run kbvmaker help %s)", c.name, err, c.name), asJSON)
	}
	if len(pos) != 1 {
		return report(e.stderr, usagef("%s needs one Montage file, or - for stdin (run kbvmaker help %s)", c.name, c.name), asJSON)
	}
	if o.progress != "text" && o.progress != "json" && o.progress != "none" {
		return report(e.stderr, usagef("%s: -progress is text, json, or none, not %q", c.name, o.progress), asJSON)
	}
	j := &job{env: e, o: o, src: pos[0]}
	if err := c.run(ctx, j); err != nil {
		return report(e.stderr, err, asJSON)
	}
	return exitOK
}

// parse accepts flags before and after the file. Arguments after "--" are never flags.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var tail []string
	for i, a := range args {
		if a == "--" {
			tail = append(tail, args[i+1:]...)
			args = args[:i]
			break
		}
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
	return append(pos, tail...), nil
}

// wantsJSON finds -json before flag parsing, so a parse error is reported in the asked-for form.
func wantsJSON(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		switch a {
		case "-json", "--json", "-json=true", "--json=true":
			return true
		}
	}
	return false
}

// job is one document command in progress.
type job struct {
	env env
	o   *opts
	src string
}

func (j *job) cwd() (string, error) {
	wd, err := j.env.getwd()
	if err != nil {
		return "", err
	}
	return wd, nil
}

// abs makes a command-line path absolute against the current directory.
func (j *job) abs(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return path, nil
	}
	wd, err := j.cwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, path), nil
}

// load reads the document and applies the override flags.
func (j *job) load() (*doc.Document, error) {
	var d *doc.Document
	if j.src == "-" {
		b, err := io.ReadAll(j.env.stdin)
		if err != nil {
			return nil, err
		}
		dir := j.o.dir
		if dir == "" {
			if dir, err = j.cwd(); err != nil {
				return nil, err
			}
		}
		if dir, err = j.abs(dir); err != nil {
			return nil, err
		}
		if d, err = v1.Decode(b, dir); err != nil {
			return nil, err
		}
	} else {
		if j.o.dir != "" {
			return nil, usagef("-dir is only for a document read from stdin")
		}
		path, err := j.abs(j.src)
		if err != nil {
			return nil, err
		}
		if d, err = v1.Load(path); err != nil {
			return nil, err
		}
	}
	ov := j.o.ov
	var err error
	if ov.Audio, err = j.abs(j.o.audio); err != nil {
		return nil, err
	}
	if ov.OutputFile, err = j.abs(ov.OutputFile); err != nil {
		return nil, err
	}
	v1.ApplyOverrides(d, ov)
	return d, nil
}

func (j *job) engine() (*v1.Engine, error) {
	probe, err := j.env.probe(j.o.ffmpeg, j.o.ffprobe)
	if err != nil {
		return nil, err
	}
	eng := v1.New(probe)
	eng.Options = v1.CompileOptions{FFmpeg: j.o.ffmpeg, FFProbe: j.o.ffprobe, Verbose: j.o.verbose, Work: j.o.work}
	return eng, nil
}

func (j *job) resolve(ctx context.Context) (*v1.Engine, *doc.Document, *model.Plan, error) {
	d, err := j.load()
	if err != nil {
		return nil, nil, nil, err
	}
	eng, err := j.engine()
	if err != nil {
		return nil, nil, nil, err
	}
	p, err := eng.Resolve(ctx, d)
	if err != nil {
		return nil, nil, nil, err
	}
	return eng, d, p, nil
}

// write sends data to stdout, or to path through a temporary file and a rename.
func (j *job) write(path string, data []byte, mode os.FileMode) error {
	if path == "" || path == "-" {
		_, err := j.env.stdout.Write(data)
		return err
	}
	path, err := j.abs(path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (j *job) result(text string, value any) error {
	if j.o.json {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(j.env.stdout, "%s\n", b)
		return err
	}
	_, err := fmt.Fprintln(j.env.stdout, text)
	return err
}

func validate(ctx context.Context, j *job) error {
	if !j.o.media {
		d, err := j.load()
		if err != nil {
			return err
		}
		if err := v1.Validate(d); err != nil {
			return err
		}
		return j.result("ok", map[string]any{"ok": true})
	}
	_, _, p, err := j.resolve(ctx)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("ok: %d pictures, %s", len(p.Slots), clock(p.Duration))
	return j.result(text, map[string]any{"ok": true, "slots": len(p.Slots), "duration": p.Duration})
}

func planCmd(ctx context.Context, j *job) error {
	_, _, p, err := j.resolve(ctx)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return j.write(j.o.output, append(b, '\n'), 0o644)
}

func exportScript(ctx context.Context, j *job) error {
	eng, _, p, err := j.resolve(ctx)
	if err != nil {
		return err
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		return err
	}
	return j.write(j.o.output, []byte(v1.ExportScriptIn(cmds, j.o.work)), 0o755)
}

func exportPlan(ctx context.Context, j *job) error {
	eng, _, p, err := j.resolve(ctx)
	if err != nil {
		return err
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		return err
	}
	b, err := v1.ExportPlan(cmds)
	if err != nil {
		return err
	}
	return j.write(j.o.output, b, 0o644)
}

func render(ctx context.Context, j *job) error {
	j.o.ov.OutputFile = j.o.output
	eng, d, p, err := j.resolve(ctx)
	if err != nil {
		return err
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		return err
	}
	start := time.Now()
	err = v1.RunWith(ctx, d.Dir, cmds, v1.RunOptions{
		Work:     j.o.work,
		Atomic:   true,
		Progress: j.progress(),
	})
	if err != nil {
		return err
	}
	video := p.OutputFile
	if !filepath.IsAbs(video) {
		video = filepath.Join(d.Dir, video)
	}
	if j.o.clean {
		cleanWork(d.Dir, j.o.work, cmds)
	}
	seconds := time.Since(start).Seconds()
	switch j.o.progress {
	case "text":
		fmt.Fprintf(j.env.stderr, "wrote %s in %s\n", video, clock(seconds))
	case "json":
		b, _ := json.Marshal(map[string]any{"event": "wrote", "file": video, "seconds": seconds})
		fmt.Fprintf(j.env.stderr, "%s\n", b)
	}
	return j.result(video, map[string]any{"file": video, "seconds": seconds})
}

func (j *job) progress() func(v1.RunEvent) {
	w := j.env.stderr
	switch j.o.progress {
	case "text":
		return func(ev v1.RunEvent) {
			if !ev.Done {
				fmt.Fprintf(w, "[%d/%d] %s %d/%d\n", ev.Step, ev.Steps, ev.Stage, ev.N, ev.Of)
			}
		}
	case "json":
		return func(ev v1.RunEvent) {
			line := map[string]any{
				"event": "start", "stage": ev.Stage,
				"step": ev.Step, "steps": ev.Steps, "n": ev.N, "of": ev.Of,
			}
			if ev.Done {
				line["event"] = "done"
				line["seconds"] = ev.Seconds
			}
			b, _ := json.Marshal(line)
			fmt.Fprintf(w, "%s\n", b)
		}
	}
	return nil
}

// cleanWork removes the intermediate files the commands wrote, then the work
// directory if nothing else is in it. Other files there are left alone.
func cleanWork(dir, work string, cmds []v1.Command) {
	root := work
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	for _, c := range cmds {
		for _, a := range c.Argv {
			p := a
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			if filepath.Dir(p) == filepath.Clean(root) && intermediate.MatchString(filepath.Base(p)) {
				os.Remove(p)
			}
		}
	}
	if filepath.Clean(root) != filepath.Clean(dir) {
		os.Remove(root)
	}
}

// intermediate matches the names the compiler gives canvases, clips, and joins.
var intermediate = regexp.MustCompile(`^(canvas-\d+\.png|clip-\d{3}\.mp4|join-\d{3}\.mp4)$`)

func clock(seconds float64) string {
	total := int(seconds + 0.5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func serve(ctx context.Context, args []string, e env) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "127.0.0.1:8787", "listen `ADDRESS`")
	static := fs.String("static", "", "serve the built client from `DIR`")
	ffmpeg := fs.String("ffmpeg", "ffmpeg", "ffmpeg `BINARY`")
	ffprobe := fs.String("ffprobe", "ffprobe", "ffprobe `BINARY`")
	pos, err := parse(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(e.stdout, "Usage: kbvmaker serve [flags]\n\nServe the /v1 HTTP API (and the client with -static) until interrupted.\n\nFlags:\n")
		fs.SetOutput(e.stdout)
		fs.PrintDefaults()
		return exitOK
	}
	if err != nil {
		return report(e.stderr, usagef("serve: %v (run kbvmaker help serve)", err), false)
	}
	if len(pos) != 0 {
		return report(e.stderr, usagef("serve takes no file"), false)
	}
	probe, err := e.probe(*ffmpeg, *ffprobe)
	if err != nil {
		return report(e.stderr, err, false)
	}
	srv := web.Server{Probe: probe}
	if *static != "" {
		srv.Static = os.DirFS(*static)
	}
	hs := &http.Server{Addr: *addr, Handler: srv.Handler()}
	done := make(chan error, 1)
	go func() { done <- e.listen(hs) }()
	fmt.Fprintf(e.stderr, "kbvmaker listening on http://%s\n", *addr)
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return report(e.stderr, err, false)
		}
		return exitOK
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(shut)
		return exitOK
	}
}
