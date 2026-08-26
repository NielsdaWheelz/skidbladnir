package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/statushook"
)

const (
	exitFailure = 1
	exitUsage   = 64
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		_, _ = io.WriteString(stderr, "usage: skidbladnir {gateway|bearer mint|status-hook EVENT}\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
		return exitUsage
	}
	if arguments[0] == "status-hook" {
		if len(arguments) != 2 {
			_, _ = io.WriteString(stderr, "usage: skidbladnir status-hook {SessionStart|UserPromptSubmit|Stop}\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitUsage
		}
		if err := statushook.Run(context.Background(), arguments[1], os.Stdin, stdout); err != nil {
			_, _ = io.WriteString(stderr, "status-hook failed\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitFailure
		}
		return 0
	}
	if arguments[0] != "bearer" && arguments[0] != "gateway" {
		_, _ = io.WriteString(stderr, "usage: skidbladnir {gateway|bearer mint|status-hook EVENT}\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
		return exitUsage
	}
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve service home: %v\n", err) // justify-ignore-error: a broken CLI output stream cannot be recovered.
		return exitFailure
	}
	if arguments[0] == "bearer" {
		if len(arguments) == 1 || arguments[1] != "mint" {
			_, _ = io.WriteString(stderr, "usage: skidbladnir bearer mint [--file=PATH]\n") // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitUsage
		}
		flags := flag.NewFlagSet("bearer mint", flag.ContinueOnError)
		flags.SetOutput(stderr)
		path := flags.String("file", filepath.Join(home, ".config", "skidbladnir", "bearer"), "bearer file")
		if err := flags.Parse(arguments[2:]); err != nil || flags.NArg() != 0 {
			return exitUsage
		}
		bearer, err := auth.Mint(auth.MintOptions{Path: *path})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "mint bearer: %v\n", err) // justify-ignore-error: a broken CLI output stream cannot be recovered.
			return exitFailure
		}
		if _, err := fmt.Fprintln(stdout, bearer); err != nil {
			_, _ = io.WriteString(stderr, "write minted bearer: output failed\n") // justify-ignore-error: both CLI output streams are unavailable.
			return exitFailure
		}
		return 0
	}
	flags := flag.NewFlagSet("gateway", flag.ContinueOnError)
	flags.SetOutput(stderr)
	listen := flags.String("listen", "127.0.0.1:7341", "numeric loopback listen address")
	bearerPath := flags.String("bearer-file", filepath.Join(home, ".config", "skidbladnir", "bearer"), "bearer file")
	cataloguePath := flags.String("catalogue-path", filepath.Join(home, ".local", "share", "skidbladnir", "characters.json"), "Dvergatal catalogue")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return exitUsage
	}
	if err := serveGateway(*listen, *bearerPath, *cataloguePath, home, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "gateway: %v\n", err) // justify-ignore-error: a broken CLI output stream cannot be recovered.
		return exitFailure
	}
	return 0
}

func serveGateway(listen, bearerPath, cataloguePath, home string, logOutput io.Writer) error {
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      "/usr/bin/tmux",
		Home:          home,
		CataloguePath: cataloguePath,
		Profiles:      gatewayProfiles(home),
	})
	if err != nil {
		return fmt.Errorf("initialize tmux sessions: %w", err)
	}
	monitor := pressure.NewMonitor()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go monitor.Run(ctx)
	handler := gateway.New(gateway.Config{
		Sessions: manager,
		Pressure: monitor,
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Logger:   logging.New(logOutput),
	})
	if err := gateway.ListenAndServe(ctx, listen, handler); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func gatewayProfiles(home string) []sessions.Profile {
	return []sessions.Profile{
		codexProfile(home, "personal", "Codex · Personal", ".codex-personal"),
		codexProfile(home, "work", "Codex · Work", ".codex-work"),
		codexProfile(home, "work2", "Codex · Work 2", ".codex-work2"),
		claudeProfile(home),
	}
}

func codexProfile(home, key, label, codexHomeName string) sessions.Profile {
	return sessions.Profile{
		Key:     key,
		Label:   label,
		Command: filepath.Join(home, "bin", "codex-"+key),
		Environment: []sessions.EnvironmentVariable{
			{Name: "CODEX_HOME", Value: filepath.Join(home, codexHomeName)},
		},
		ForegroundSignatures: []sessions.ForegroundSignature{
			{ExecutableBase: "codex"},
			{ExecutableBase: "node", Argument1: filepath.Join(home, ".local", "bin", "codex")},
		},
		Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
	}
}

func claudeProfile(home string) sessions.Profile {
	return sessions.Profile{
		Key:     "claude-work",
		Label:   "Claude · Work",
		Command: filepath.Join(home, "bin", "claude-work"),
		Environment: []sessions.EnvironmentVariable{
			{Name: "CLAUDE_CONFIG_DIR", Value: filepath.Join(home, ".claude-work")},
		},
		ForegroundSignatures: []sessions.ForegroundSignature{
			{Argument0: filepath.Join(home, ".local", "bin", "claude")},
		},
		Arguments: []string{"--permission-mode", "auto"},
	}
}
