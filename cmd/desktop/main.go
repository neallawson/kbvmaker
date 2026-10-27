//go:build production || dev

// Command desktop opens the kbvmaker window on the shared HTTP API.
//
//	./scripts/build
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"kbvmaker/v1/media"
	"kbvmaker/web"
)

//go:embed all:client
var embeddedClient embed.FS

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("kbvmaker-gui takes no arguments")
	}
	client, err := clientAssets()
	if err != nil {
		return err
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
			Assets:  client,
			Handler: handler,
		},
		OnStartup:     dialogs.startup,
		OnBeforeClose: dialogs.beforeClose,
		Bind:          []interface{}{dialogs, &API{handler: handler}},
	})
}

func clientAssets() (fs.FS, error) {
	client, err := fs.Sub(embeddedClient, "client")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(client, "index.html"); err != nil {
		return nil, fmt.Errorf("the GUI was built without its client; run ./scripts/build")
	}
	return client, nil
}
