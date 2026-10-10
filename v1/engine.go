package v1

import (
	"context"

	"neal/kbvmaker/v1/compile"
	"neal/kbvmaker/v1/doc"
	"neal/kbvmaker/v1/export"
	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/v1/model"
	"neal/kbvmaker/v1/plan"
	"neal/kbvmaker/v1/run"
)

// Engine resolves Montage 1 documents and compiles render commands.
type Engine struct {
	Probe   media.Probe
	Options CompileOptions
}

// Command is one ffmpeg or ffprobe argument list.
type Command = compile.Command

// CompileOptions names the ffmpeg binaries, the work directory, and ffmpeg's verbosity.
type CompileOptions = compile.Options

// New returns an engine that probes media through p.
func New(p media.Probe) *Engine {
	return &Engine{Probe: p}
}

// Load reads a Montage 1 document from path.
func Load(path string) (*doc.Document, error) {
	return doc.Load(path)
}

// Decode parses a Montage 1 document. Relative paths in it resolve against dir.
func Decode(b []byte, dir string) (*doc.Document, error) {
	d, err := doc.Decode(b)
	if err != nil {
		return nil, err
	}
	d.Dir = dir
	return d, nil
}

// Override is a value from the command line that wins over the document.
type Override = doc.Override

// ApplyOverrides writes flag values onto a loaded document.
func ApplyOverrides(d *doc.Document, o Override) {
	doc.ApplyOverrides(d, o)
}

// Validate checks a document without reading media.
func Validate(d *doc.Document) error {
	return d.Validate()
}

// Resolve builds the timeline. It does not encode.
func (e *Engine) Resolve(ctx context.Context, d *doc.Document) (*model.Plan, error) {
	return plan.Resolve(ctx, d, e.Probe)
}

// Compile turns a plan into the ffmpeg argument lists.
func (e *Engine) Compile(p *model.Plan) ([]compile.Command, error) {
	return compile.Commands(p, e.Options)
}

// ExportScript writes the commands as bash.
func ExportScript(cmds []compile.Command) string {
	return export.Script(cmds)
}

// ExportScriptIn writes the commands as bash that creates work first.
func ExportScriptIn(cmds []compile.Command, work string) string {
	return export.ScriptIn(cmds, work)
}

// ExportPlan writes the commands as a renderPlan JSON document.
func ExportPlan(cmds []compile.Command) ([]byte, error) {
	return export.PlanJSON(cmds)
}

// Run executes the commands with dir as the working directory.
func Run(ctx context.Context, dir string, cmds []compile.Command) error {
	return run.Run(ctx, dir, cmds)
}

// RunOptions controls progress, the work directory, and the atomic final write.
type RunOptions = run.Options

// RunEvent is one step starting or finishing.
type RunEvent = run.Event

// RunWith executes the commands with progress and an atomic final write.
func RunWith(ctx context.Context, dir string, cmds []compile.Command, opt RunOptions) error {
	return run.RunWith(ctx, dir, cmds, opt)
}

// IsStepFailure reports whether err is an ffmpeg or ffprobe command that failed.
func IsStepFailure(err error) bool {
	return run.IsStep(err)
}
