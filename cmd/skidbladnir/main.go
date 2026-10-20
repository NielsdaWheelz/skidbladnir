package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentcli"
	"github.com/NielsdaWheelz/skidbladnir/internal/agenthook"
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
		return installationCommand(arguments, stdout, stderr)
	case "agent-hook":
	default:
		return agentcli.Run(context.Background(), arguments, stdin, stdout, stderr)
	}
	if arguments[0] == "agent-hook" {
		// One budget covers argument admission, bounded SessionStart input,
		// accepted-settings admission, and publication. Closing stdin is the
		// cancellation boundary when a provider never reaches EOF.
		ctx, cancel := context.WithTimeout(context.Background(), agenthook.PublicationDeadline)
		defer cancel()
		flags := flag.NewFlagSet("agent-hook", flag.ContinueOnError)
		flags.SetOutput(stderr)
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 2 || flags.Arg(0) != "Claude" || flags.Arg(1) != "SessionStart" {
			_, _ = io.WriteString(stderr, "usage: skid agent-hook Claude SessionStart\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitUsage
		}
		if os.Getenv("SKIDBLADNIR_AGENT") != "1" {
			return 0
		}
		preparedInput := prepareAgentHookInput(flags.Arg(0), flags.Arg(1), stdin)
		prepared, prepareErr := awaitAgentHookInput(ctx, stdin, preparedInput)
		if errors.Is(prepareErr, agenthook.ErrInvocationRejected) {
			// Prepare admits the closed argv union before reading stdin; deployment
			// state is intentionally still untouched here.
			_, _ = io.WriteString(stderr, "agent-hook did not publish\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitUsage
		}
		if prepareErr != nil {
			// Invalid or nonterminating provider input is projection-local and
			// cannot prevent the provider session from starting.
			_, _ = io.WriteString(stderr, "agent-hook did not publish\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return 0
		}
		// Without a pane there is no projection to publish. Input admission and
		// draining still precede this no-op; host tooling is irrelevant to it.
		if os.Getenv("TMUX_PANE") == "" {
			return 0
		}
		config, configErr := loadRuntimeHostConfig(ctx)
		if configErr != nil {
			// A host-settings defect is the operator's to fix, not the
			// provider's to pay for. This command stays out of the provider's way.
			_, _ = io.WriteString(stderr, "agent-hook did not publish\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return 0
		}
		if err := agenthook.Run(ctx, agenthook.Config{
			TmuxPath: config.TmuxPath,
			Profiles: config.Profiles,
		}, prepared); err != nil {
			// The hook is a passive projection of pane-local facts. Publication
			// failure must not prevent the provider session from starting, so only
			// the argv rejection above exits non-zero. A failed projection reports
			// itself content-free and leaves the provider's own work untouched.
			_, _ = io.WriteString(stderr, "agent-hook did not publish\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return 0
		}
		return 0
	}
	return exitUsage
}

type agentHookInputResult struct {
	prepared agenthook.Prepared
	err      error
}

type agentHookInput interface {
	io.Reader
	io.Closer
}

var errAgentHookInputClose = errors.New("close provider hook input")

func prepareAgentHookInput(provider, event string, input io.Reader) <-chan agentHookInputResult {
	result := make(chan agentHookInputResult, 1)
	go func() {
		prepared, err := agenthook.Prepare(provider, event, input)
		result <- agentHookInputResult{prepared: prepared, err: err}
	}()
	return result
}

func awaitAgentHookInput(
	ctx context.Context,
	input agentHookInput,
	result <-chan agentHookInputResult,
) (agenthook.Prepared, error) {
	select {
	case prepared := <-result:
		return prepared.prepared, prepared.err
	case <-ctx.Done():
		// The command owns this hook-only stdin. Closing a provider pipe is the
		// cancellation boundary for a writer that never reaches EOF; the worker
		// is then joined so run never leaks input work in tests or production.
		closeErr := input.Close()
		prepared := <-result
		causes := []error{ctx.Err()}
		if closeErr != nil {
			if errors.Is(closeErr, os.ErrClosed) {
				// justify-ignore-error: an already-closed owned input has already
				// established the cancellation boundary and the worker was joined.
			} else {
				causes = append(causes, errAgentHookInputClose)
			}
		}
		if prepared.err != nil {
			causes = append(causes, prepared.err)
		}
		return prepared.prepared, errors.Join(causes...)
	}
}
