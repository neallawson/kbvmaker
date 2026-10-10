package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed gtkfile.py
var gtkFilePy string

// pickFiles runs the GTK dialog in its own process.
// A cancel is an empty list and a nil error.
func pickFiles(mode, title, dir, name, filter string, patterns []string) ([]string, error) {
	args := []string{"-", mode, title, dir, name, filter}
	args = append(args, patterns...)
	cmd := exec.Command("python3", args...)
	cmd.Stdin = strings.NewReader(gtkFilePy)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// montagePath keeps a .json name and adds .montage.json otherwise.
func montagePath(path string) string {
	if path == "" || strings.HasSuffix(strings.ToLower(path), ".json") {
		return path
	}
	return path + ".montage.json"
}

func withExt(path, ext string) string {
	if path == "" || strings.HasSuffix(strings.ToLower(path), ext) {
		return path
	}
	return path + ext
}
