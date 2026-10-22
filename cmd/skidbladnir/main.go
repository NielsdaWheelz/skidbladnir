package main

import (
	"context"
	"fmt"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentcli"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"io"
	"os"
)

const (
	exitFailure = 1
	exitUsage   = 64
)

var (
	releaseVersion = "dev"
	releaseSHA     = "unknown"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
func run(arguments []string, stdin *os.File, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		return agentcli.Run(context.Background(), arguments, stdin, stdout, stderr)
	}
	switch arguments[0] {
	case "terminal-context":
		if err := terminalContext(arguments[1:]); err != nil {
			fmt.Fprintln(stderr, "terminal context unavailable")
			return exitFailure
		}
		return 0
	case "agent-exec":
		if err := agentExec(arguments[1:]); err != nil {
			fmt.Fprintln(stderr, "agent startup failed")
			return exitFailure
		}
		return 0
	case "terminal-exec":
		if err := terminalExec(arguments[1:]); err != nil {
			fmt.Fprintln(stderr, "terminal startup failed")
			return exitFailure
		}
		return 0
	case "version":
		if len(arguments) != 1 {
			return exitUsage
		}
		if _, err := fmt.Fprintf(stdout, "%s %s\n", releaseVersion, releaseSHA); err != nil {
			return exitFailure
		}
		return 0
	case "serve":
		if len(arguments) != 1 {
			return exitUsage
		}
		if err := serveStandalone(stdout); err != nil {
			fmt.Fprintf(stderr, "skid service: %v\n", err)
			return exitFailure
		}
		return 0
	case "connect", "join", "remove", "recover", "status", "apply":
		return fleetCommand(arguments, stdin, stdout, stderr)
	case "install", "update", "uninstall":
		if err := install.Run(context.Background(), arguments, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "skid %s: %v\n", arguments[0], err)
			return exitFailure
		}
		return 0
	default:
		return agentcli.Run(context.Background(), arguments, stdin, stdout, stderr)
	}
}
