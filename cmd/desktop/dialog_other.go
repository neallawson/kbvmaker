//go:build !linux

package main

import (
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// pickFiles uses the Wails dialogs, which honor the starting folder on Windows and macOS.
// A cancel is an empty list.
func (d *Dialogs) pickFiles(mode, title, dir, name, filter string, patterns []string) ([]string, error) {
	opts := runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: dir,
		Filters:          []runtime.FileFilter{{DisplayName: filter, Pattern: strings.Join(patterns, ";")}},
	}
	switch mode {
	case "multi":
		return runtime.OpenMultipleFilesDialog(d.ctx, opts)
	case "save":
		path, err := runtime.SaveFileDialog(d.ctx, runtime.SaveDialogOptions{
			Title:            title,
			DefaultDirectory: dir,
			DefaultFilename:  name,
			Filters:          opts.Filters,
		})
		return nonEmpty(path), err
	default:
		path, err := runtime.OpenFileDialog(d.ctx, opts)
		return nonEmpty(path), err
	}
}

func nonEmpty(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}
