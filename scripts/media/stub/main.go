// Command stub stands in for a provider in staged demos. skid recognizes a
// provider by its executable's base name, so stage installs this binary as
// codex and as claude. It prints one pre-painted frame and waits until the
// terminal hangs up or it is told to stop.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: codex|claude FRAME.ansi")
		os.Exit(2)
	}
	frame, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGHUP, syscall.SIGTERM)
	os.Stdout.Write(frame)
	<-stop
}
