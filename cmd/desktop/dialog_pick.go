package main

import "strings"

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
