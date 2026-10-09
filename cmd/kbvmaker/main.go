// Command kbvmaker resolves a Montage document and renders it, or exports the plan.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"neal/kbvmaker/v1"
	"neal/kbvmaker/v1/media"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: kbvmaker plan|export-script|export-plan|render file.montage.json")
	}
	probe, err := media.NewFFProbe()
	if err != nil {
		return err
	}
	d, err := v1.Load(args[1])
	if err != nil {
		return err
	}
	eng := v1.New(probe)
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
