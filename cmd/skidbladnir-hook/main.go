package main

import (
	"flag"
	"io"
	"os"

	"github.com/NielsdaWheelz/skidbladnir/internal/hooks"
)

func main() {
	flags := flag.NewFlagSet("skidbladnir-hook", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pinnedCodex := flags.String("pinned-codex", "", "absolute pinned Codex executable")
	socketPath := flags.String("socket", "", "hook Unix socket")
	gapDirectory := flags.String("gap-directory", "", "durable hook gap marker directory")
	if flags.Parse(os.Args[1:]) != nil || *pinnedCodex == "" || *socketPath == "" || *gapDirectory == "" || flags.NArg() != 0 {
		os.Exit(2)
	}
	if err := hooks.Run(os.Stdin, os.Getpid(), *pinnedCodex, *socketPath, *gapDirectory); err != nil {
		os.Exit(1)
	}
}
