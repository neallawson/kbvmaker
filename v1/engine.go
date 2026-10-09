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
	Options compile.Options
}

// New returns an engine that probes media through p.
func New(p media.Probe) *Engine {
	return &Engine{Probe: p}
}

// Load reads a Montage 1 document from path.
func Load(path string) (*doc.Document, error) {
	return doc.Load(path)
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

// ExportPlan writes the commands as a renderPlan JSON document.
func ExportPlan(cmds []compile.Command) ([]byte, error) {
	return export.PlanJSON(cmds)
}

// Run executes the commands with dir as the working directory.
func Run(ctx context.Context, dir string, cmds []compile.Command) error {
	return run.Run(ctx, dir, cmds)
}
