// Package export writes a resolved command list as bash or JSON.
package export

import (
	"encoding/json"
	"fmt"
	"strings"

	"neal/kbvmaker/v1/compile"
	"neal/kbvmaker/v1/version"
)

// Script is a bash program that runs the commands from the project directory.
func Script(cmds []compile.Command) string {
	var b strings.Builder
	b.WriteString("#!/bin/bash\nset -euo pipefail\nmkdir -p work\n")
	for _, c := range cmds {
		if c.Stage != "" {
			fmt.Fprintf(&b, "\n# %s\n", c.Stage)
		}
		for i, a := range c.Argv {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(bashQuote(a))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func bashQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// PlanFile is the JSON form of a command list. RenderPlan matches Montage's version.
type PlanFile struct {
	RenderPlan int        `json:"renderPlan"`
	Commands   [][]string `json:"commands"`
}

// PlanJSON writes the command list as a renderPlan document.
func PlanJSON(cmds []compile.Command) ([]byte, error) {
	f := PlanFile{RenderPlan: version.Current, Commands: make([][]string, len(cmds))}
	for i, c := range cmds {
		f.Commands[i] = c.Argv
	}
	buf, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}
