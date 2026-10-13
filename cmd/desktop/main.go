//go:build production || dev

// Command desktop opens the kbvmaker window on the shared HTTP API.
//
//	./scripts/build
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/web"
)

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
	handler := web.Server{Probe: probe}.Handler()
	dialogs := &Dialogs{}
	return wails.Run(&options.App{
		Title:  "kbvmaker - Montage (v1) editor, renderer",
		Width:  1100,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets:  os.DirFS(*static),
			Handler: handler,
		},
		OnStartup: dialogs.startup,
		Bind:      []interface{}{dialogs, &API{handler: handler}},
	})
}
