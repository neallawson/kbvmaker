// Package identity is the release name, version, copyright, and URL shared by
// the command line and the editor.
package identity

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed identity.json
var raw []byte

// release is the product version. A release build sets it from the git tag
// with -ldflags "-X kbvmaker/identity.release=v1.0.0". It is not the Montage
// document number. An unstamped build says dev.
var release = "dev"

// built is when this build was made, UTC, such as 2026-11-22T22:08:05Z.
// The build stamps it with -X kbvmaker/identity.built=... Dev builds included.
var built = "unknown"

// Info is the name, copyright, and URL. The version is the git tag, stamped
// at build time.
type Info struct {
	Name      string `json:"name"`
	Copyright string `json:"copyright"`
	URL       string `json:"url"`
}

// Current is the embedded identity.
var Current = load()

func load() Info {
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		panic(err)
	}
	return info
}

// Version is the stamped git tag, or dev when this build was not tagged.
func Version() string {
	if release == "" {
		return "dev"
	}
	return release
}

// Built is the UTC time this build was made.
func Built() string {
	if built == "" {
		return "unknown"
	}
	return built
}

// Text is the two lines both programs show.
func Text() string {
	return fmt.Sprintf("%s version %s, %s, built %s\n%s\n", Current.Name, Version(), Current.Copyright, Built(), Current.URL)
}
