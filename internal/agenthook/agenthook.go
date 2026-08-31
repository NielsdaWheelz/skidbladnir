package agenthook

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const (
	maximumProcessAncestry = 128
	// Every installed hook entry declares a five-second provider timeout. The
	// helper closes its own tmux work inside that window so a wedged tmux
	// server degrades to omitted optional identity instead of blocking provider
	// startup. The CLI applies it across the whole command; deriving it again
	// here keeps Run bounded on its own.
	PublicationDeadline = 3 * time.Second
	hookSessionStart    = "SessionStart"
)

var (
	// ErrInvocationRejected reports a provider/event pair this build does not
	// accept, which is a deployment configuration defect. Every other failure
	// is a best-effort projection failure: the hook is a passive observer and
	// must never block a provider session from starting.
	ErrInvocationRejected = errors.New("unsupported provider hook event")

	// ErrProviderInput reports that the bounded SessionStart document could not
	// be read or admitted. It is deliberately content-free.
	ErrProviderInput = errors.New("invalid provider hook input")
)

type Config struct {
	TmuxPath string
	Profiles []agentruntime.Profile
}

type invocation struct {
	provider agentruntime.Provider
}

func Run(
	ctx context.Context,
	config Config,
	prepared Prepared,
) error {
	ctx, cancel := context.WithTimeout(ctx, PublicationDeadline)
	defer cancel()
	invocation := prepared.invocation

	pane := os.Getenv("TMUX_PANE")
	if !validPaneID(pane) {
		return nil
	}
	paneTerminal, err := readPaneTerminal(ctx, config.TmuxPath, pane)
	if err != nil {
		return err
	}
	ancestry, err := processinfo.ObserveAncestry(processinfo.PID(os.Getpid()), maximumProcessAncestry)
	if err != nil {
		return err
	}
	origin, found := agentruntime.HookOrigin(config.Profiles, ancestry, paneTerminal)
	if !found || origin.Provider != invocation.provider {
		return nil
	}

	profile, _ := agentruntime.MatchProfileEnvironment(config.Profiles, invocation.provider, os.LookupEnv)
	registration, err := agentruntime.EncodeRegistration(origin, profile, prepared.providerSessionID)
	if err != nil {
		return errors.New("encode agent runtime registration")
	}
	command := exec.CommandContext(
		ctx,
		config.TmuxPath,
		publicationArguments(pane, registration)...,
	)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return errors.New("publish tmux pane agent facts")
	}
	return nil
}

func parseInvocation(providerText, eventText string) (invocation, error) {
	provider, err := agentruntime.ParseProvider(providerText)
	if err != nil {
		return invocation{}, ErrInvocationRejected
	}
	if eventText != hookSessionStart ||
		(provider != agentruntime.ProviderCodex && provider != agentruntime.ProviderClaude) {
		return invocation{}, ErrInvocationRejected
	}
	return invocation{provider: provider}, nil
}

func publicationArguments(pane, registration string) []string {
	// Architecture §4: every valid SessionStart writes the option. The
	// HookOrigin ancestry and pane-tty check above already bind this write to a
	// provider that is live inside this exact pane, and inventory re-validates
	// the registration against the observed PID and kernel start identity, so
	// last-writer-wins is the correct projection of current identity. Argv form
	// carries no tmux command-string quoting.
	return []string{"set-option", "-p", "-t", pane, "--", agentruntime.PaneOption, registration}
}

func readPaneTerminal(ctx context.Context, tmuxPath, pane string) (processinfo.TerminalDevice, error) {
	command := exec.CommandContext(ctx, tmuxPath, "display-message", "-p", "-t", pane, "#{pane_tty}")
	command.Stderr = io.Discard
	encoded, err := command.Output()
	if err != nil || len(encoded) < 2 || encoded[len(encoded)-1] != '\n' || strings.ContainsRune(string(encoded[:len(encoded)-1]), '\n') {
		return 0, errors.New("read tmux pane terminal")
	}
	terminal, err := processinfo.TerminalDeviceAt(string(encoded[:len(encoded)-1]))
	if err != nil {
		return 0, errors.New("read tmux pane terminal")
	}
	return terminal, nil
}

func validPaneID(value string) bool {
	if len(value) < 2 || value[0] != '%' {
		return false
	}
	for _, symbol := range value[1:] {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return true
}
