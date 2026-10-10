// Command kbvmaker resolves a Montage document and renders it, or exports the plan.
//
// It is built for scripts and pipelines: data goes to stdout, messages to
// stderr, and the exit code says what kind of failure happened. Run
// "kbvmaker help" for the commands.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli(ctx, os.Args[1:], osEnv())
	stop()
	os.Exit(code)
}
