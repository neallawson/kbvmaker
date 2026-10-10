package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"neal/kbvmaker/v1/version"
)

func topHelp() string {
	var b strings.Builder
	b.WriteString(`kbvmaker makes a Ken Burns video from a Montage document, a song, and photos.

Usage:
  kbvmaker COMMAND [flags] FILE|-
  kbvmaker help [COMMAND]

Commands:
`)
	for _, name := range order {
		fmt.Fprintf(&b, "  %-14s %s\n", name, commands[name].summary)
	}
	b.WriteString(`  serve          serve the /v1 HTTP API
  version        print the build and the Montage version
  help           show this help, or one command's flags

FILE is a Montage document (name.montage.json). "-" reads it from stdin;
relative paths in it then resolve against -dir (default: current directory).
Flags may come before or after FILE.

Output:
  Results go to stdout (or to -o FILE). Progress and errors go to stderr.
  -json prints errors as {"slot","code","message"}, the same as the HTTP API.

Exit codes:
  0    success
  1    unexpected failure (I/O and the like)
  2    usage: unknown command, bad flag, wrong number of files
  3    the document breaks a Montage rule
  4    missing media or tool: document, song, picture, ffmpeg, ffprobe, or a filter
  5    an ffmpeg or ffprobe step failed during render
  130  interrupted (SIGINT or SIGTERM); the running ffmpeg is stopped
`)
	return b.String()
}

func help(args []string, e env) int {
	if len(args) == 0 {
		fmt.Fprint(e.stdout, topHelp())
		return exitOK
	}
	switch args[0] {
	case "serve":
		return serve(context.Background(), []string{"-h"}, e)
	case "version":
		fmt.Fprint(e.stdout, "Usage: kbvmaker version [-json]\n\nPrint the build and the Montage version.\n")
		return exitOK
	case "help":
		fmt.Fprint(e.stdout, topHelp())
		return exitOK
	}
	c, ok := commands[args[0]]
	if !ok {
		return report(e.stderr, usagef("unknown command %q; run kbvmaker help", args[0]), false)
	}
	commandHelp(e.stdout, c, flagSet(c, newOpts()))
	return exitOK
}

func commandHelp(w io.Writer, c *command, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: %s\n\n%s.\n\nFlags:\n", c.usage, strings.ToUpper(c.summary[:1])+c.summary[1:])
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	v := info.Main.Version
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if rev != "" && (v == "" || v == "(devel)") {
		return rev + dirty
	}
	if v == "" {
		return "unknown"
	}
	return v
}

func versionCmd(args []string, e env) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parse(fs, args)
	if err == flag.ErrHelp {
		return help([]string{"version"}, e)
	}
	if err != nil || len(pos) != 0 {
		return report(e.stderr, usagef("usage: kbvmaker version [-json]"), wantsJSON(args))
	}
	if *asJSON {
		b, _ := json.Marshal(map[string]any{"version": buildVersion(), "montage": version.Current, "renderPlan": version.Current})
		fmt.Fprintf(e.stdout, "%s\n", b)
		return exitOK
	}
	fmt.Fprintf(e.stdout, "kbvmaker %s (Montage %d)\n", buildVersion(), version.Current)
	return exitOK
}
