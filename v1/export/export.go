// Package export writes a resolved command list as bash or JSON.
package export

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"kbvmaker/v1/compile"
	"kbvmaker/v1/version"
)

// Script is a bash program that runs the commands from the project directory.
func Script(cmds []compile.Command) string {
	return ScriptIn(cmds, "work")
}

// ScriptIn is Script with the intermediate files under work.
func ScriptIn(cmds []compile.Command, work string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#!/bin/bash\nset -euo pipefail\nmkdir -p %s\n", bashQuote(work))
	for i, c := range cmds {
		if c.Stage != "" {
			fmt.Fprintf(&b, "\n# %s\n", c.Stage)
		}
		if len(c.Body) > 0 {
			fmt.Fprintf(&b, "base64 -d >%s <<'KBVM_CARD_%d'\n", bashQuote(c.File), i)
			b.WriteString(wrapBase64(c.Body))
			fmt.Fprintf(&b, "KBVM_CARD_%d\n", i)
			continue
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

func wrapBase64(raw []byte) string {
	s := base64.StdEncoding.EncodeToString(raw)
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteByte('\n')
		s = s[76:]
	}
	b.WriteString(s)
	b.WriteByte('\n')
	return b.String()
}

// PlanFile is the JSON form of a command list. RenderPlan matches Montage's version.
type PlanFile struct {
	RenderPlan int               `json:"renderPlan"`
	Commands   [][]string        `json:"commands"`
	Files      map[string]string `json:"files,omitempty"`
}

// PlanJSON writes the command list as a renderPlan document.
func PlanJSON(cmds []compile.Command) ([]byte, error) {
	f := PlanFile{RenderPlan: version.Current}
	files := map[string]string{}
	for _, c := range cmds {
		if len(c.Body) > 0 && c.File != "" {
			files[c.File] = base64.StdEncoding.EncodeToString(c.Body)
			continue
		}
		f.Commands = append(f.Commands, c.Argv)
	}
	if len(files) > 0 {
		f.Files = files
	}
	buf, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}
