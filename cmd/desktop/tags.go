//go:build !production && !dev

package main

import "fmt"
import "os"

func main() {
	fmt.Fprintln(os.Stderr, "build the GUI with ./scripts/build")
	fmt.Fprintln(os.Stderr, "Windows PowerShell: powershell -File scripts\\build.ps1")
	os.Exit(1)
}
