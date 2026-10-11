//go:build !production && !dev

package main

import "fmt"
import "os"

func main() {
	fmt.Fprintln(os.Stderr, "build with: go build -tags production -o kbvmaker-gui ./cmd/desktop")
	os.Exit(1)
}
