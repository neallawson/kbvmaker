// Package run executes a compiled command list.
package run

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"neal/kbvmaker/v1/compile"
)

// Run starts each command in dir, in order, and stops at the first failure.
// Stdout and stderr of the failed command are part of the error.
func Run(ctx context.Context, dir string, cmds []compile.Command) error {
	if err := os.MkdirAll(filepath.Join(dir, "work"), 0o755); err != nil {
		return err
	}
	for _, c := range cmds {
		if len(c.Argv) == 0 {
			return fmt.Errorf("%s: empty command", c.Stage)
		}
		cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
		cmd.Dir = dir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w\n%s", c.Stage, err, out.String())
		}
	}
	return nil
}
