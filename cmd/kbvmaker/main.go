// Command kbvmaker resolves a Montage document and renders it, or exports the plan.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"neal/kbvmaker/v1"
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
	if len(args) == 0 {
		return fmt.Errorf("usage: kbvmaker plan|export-script|export-plan|render file.montage.json | serve")
	}
	if args[0] == "serve" {
		return serve(args[1:])
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "keep ffmpeg's banner and warnings")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kbvmaker plan|export-script|export-plan|render [-verbose] file.montage.json | serve")
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		return err
	}
	d, err := v1.Load(fs.Arg(0))
	if err != nil {
		return err
	}
	eng := v1.New(probe)
	eng.Options.Verbose = *verbose
	ctx := context.Background()
	p, err := eng.Resolve(ctx, d)
	if err != nil {
		return err
	}
	switch args[0] {
	case "plan":
		buf, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", buf)
		return nil
	case "export-script", "export-plan", "render":
		cmds, err := eng.Compile(p)
		if err != nil {
			return err
		}
		switch args[0] {
		case "export-script":
			fmt.Print(v1.ExportScript(cmds))
		case "export-plan":
			buf, err := v1.ExportPlan(cmds)
			if err != nil {
				return err
			}
			fmt.Print(string(buf))
		case "render":
			return v1.Run(ctx, d.Dir, cmds)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	static := fs.String("static", "", "directory of the built client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		return err
	}
	srv := web.Server{Probe: probe}
	if *static != "" {
		srv.Static = os.DirFS(*static)
	}
	log.Printf("kbvmaker listening on http://%s", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}
