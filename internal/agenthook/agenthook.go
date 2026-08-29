package agenthook

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const (
	maximumProcessAncestry = 128
	lifecycleOption        = "@skid_lifecycle"
	attentionOption        = "@skid_attention"
)

type Config struct {
	TmuxPath string
	Profiles []agentruntime.Profile
}

type hookEvent string

const (
	hookSessionStart     hookEvent = "SessionStart"
	hookUserPromptSubmit hookEvent = "UserPromptSubmit"
	hookStop             hookEvent = "Stop"
)

type invocation struct {
	provider agentruntime.Provider
	event    hookEvent
}

func Run(
	ctx context.Context,
	config Config,
	providerText string,
	eventText string,
	input io.Reader,
	output io.Writer,
) error {
	invocation, err := parseInvocation(providerText, eventText)
	if err != nil {
		return err
	}
	providerSessionID := ""
	if invocation.event == hookSessionStart {
		providerSessionID, err = readSessionStartID(input)
	} else {
		_, err = io.Copy(io.Discard, input)
	}
	if err != nil {
		return errors.New("read provider hook input")
	}

	pane := os.Getenv("TMUX_PANE")
	if !validPaneID(pane) {
		return writeSuccess(output, invocation.event)
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
		return writeSuccess(output, invocation.event)
	}

	registration := ""
	if invocation.event == hookSessionStart {
		profile, _ := agentruntime.MatchProfileEnvironment(config.Profiles, invocation.provider, os.LookupEnv)
		registration, err = agentruntime.EncodeRegistration(origin, profile, providerSessionID)
		if err != nil {
			return errors.New("encode agent runtime registration")
		}
	}
	command := exec.CommandContext(
		ctx,
		config.TmuxPath,
		publicationArguments(pane, invocation, origin, registration, time.Now())...,
	)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return errors.New("publish tmux pane agent runtime")
	}
	return writeSuccess(output, invocation.event)
}

func parseInvocation(providerText, eventText string) (invocation, error) {
	provider, err := agentruntime.ParseProvider(providerText)
	if err != nil {
		return invocation{}, errors.New("unsupported provider hook event")
	}
	event := hookEvent(eventText)
	valid := provider == agentruntime.ProviderCodex &&
		(event == hookSessionStart || event == hookUserPromptSubmit || event == hookStop) ||
		provider == agentruntime.ProviderClaude && event == hookSessionStart
	if !valid {
		return invocation{}, errors.New("unsupported provider hook event")
	}
	return invocation{provider: provider, event: event}, nil
}

func publicationArguments(
	pane string,
	hook invocation,
	origin agentruntime.Foreground,
	registration string,
	now time.Time,
) []string {
	var arguments []string
	if hook.event == hookSessionStart {
		arguments = append(arguments, "set-option", "-p", "-t", pane, "--", agentruntime.PaneOption, registration)
	}
	if hook.provider != agentruntime.ProviderCodex {
		return arguments
	}
	if len(arguments) != 0 {
		arguments = append(arguments, ";")
	}
	arguments = append(
		arguments,
		"set-option", "-p", "-t", pane, "--", lifecycleOption, lifecycleValue(hook.event, origin, now),
	)
	if hook.event == hookUserPromptSubmit {
		arguments = append(arguments, ";", "set-option", "-pqu", "-t", pane, "--", attentionOption)
	}
	return arguments
}

func lifecycleValue(event hookEvent, origin agentruntime.Foreground, now time.Time) string {
	state := "idle"
	if event == hookUserPromptSubmit {
		state = "working"
	}
	return "v1:" + strconv.Itoa(int(origin.PID)) + ":" + string(origin.StartIdentity) + ":" + state + ":" + strconv.FormatInt(now.UTC().Unix(), 10)
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

func writeSuccess(output io.Writer, event hookEvent) error {
	if event != hookStop {
		return nil
	}
	if _, err := io.WriteString(output, "{}\n"); err != nil {
		return errors.New("write provider hook response")
	}
	return nil
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
