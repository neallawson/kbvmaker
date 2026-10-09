//go:build production || dev

// Command desktop opens the kbvmaker window on the shared HTTP API.
//
//	go build -tags production -o kbvmaker-window ./cmd/desktop
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/web"
)

// Dialogs is the only Wails binding. The screens call /v1 for everything else.
type Dialogs struct {
	ctx context.Context
}

func (d *Dialogs) startup(ctx context.Context) {
	d.ctx = ctx
}

// OpenMontage asks for a project file and returns its path.
// An empty path means the dialog was cancelled.
func (d *Dialogs) OpenMontage() (string, error) {
	return runtime.OpenFileDialog(d.ctx, runtime.OpenDialogOptions{
		Title: "Open Montage",
		Filters: []runtime.FileFilter{
			{DisplayName: "Montage", Pattern: "*.montage.json;*.json"},
		},
	})
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("desktop", flag.ContinueOnError)
	static := fs.String("static", "frontend/build", "directory of the built client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	info, err := os.Stat(*static)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("client build %s is missing; run pnpm --dir frontend build", *static)
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		return err
	}
	dialogs := &Dialogs{}
	return wails.Run(&options.App{
		Title:  "kbvmaker",
		Width:  1100,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets:  os.DirFS(*static),
			Handler: web.Server{Probe: probe}.Handler(),
		},
		OnStartup: dialogs.startup,
		Bind:      []interface{}{dialogs},
	})
}
