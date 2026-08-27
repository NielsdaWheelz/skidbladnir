package statushook

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const (
	maximumProcessAncestry = 128
	lifecycleOption        = "@skid_lifecycle"
)

type Config struct {
	TmuxPath            string
	CodexNodeEntrypoint string
}

type HookEvent string

const (
	HookSessionStart     HookEvent = "SessionStart"
	HookUserPromptSubmit HookEvent = "UserPromptSubmit"
	HookStop             HookEvent = "Stop"
)

func Run(ctx context.Context, config Config, eventText string, input io.Reader, output io.Writer) error {
	event, err := parseHookEvent(eventText)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, input); err != nil {
		return errors.New("read Codex hook input")
	}
	pane := os.Getenv("TMUX_PANE")
	if !validPaneID(pane) {
		_, err := io.WriteString(output, successOutput(event))
		return err
	}
	paneTerminalCommand := exec.CommandContext(
		ctx,
		config.TmuxPath,
		"display-message", "-p", "-t", pane, "#{pane_tty}",
	)
	paneTerminalCommand.Stderr = io.Discard
	paneTerminalPath, err := paneTerminalCommand.Output()
	if err != nil {
		return errors.New("read tmux pane terminal")
	}
	if len(paneTerminalPath) < 2 || paneTerminalPath[len(paneTerminalPath)-1] != '\n' ||
		strings.ContainsRune(string(paneTerminalPath[:len(paneTerminalPath)-1]), '\n') {
		return errors.New("read tmux pane terminal")
	}
	paneTerminal, err := processinfo.TerminalDeviceAt(string(paneTerminalPath[:len(paneTerminalPath)-1]))
	if err != nil {
		return errors.New("read tmux pane terminal")
	}
	ancestry, err := processinfo.ObserveAncestry(processinfo.PID(os.Getpid()), maximumProcessAncestry)
	if err != nil {
		return err
	}
	if origin, valid := foregroundCodexOrigin(ancestry, paneTerminal, config.CodexNodeEntrypoint); valid {
		command := exec.CommandContext(
			ctx,
			config.TmuxPath,
			lifecycleTmuxArguments(pane, event, origin, time.Now())...,
		)
		command.Stdout = io.Discard
		command.Stderr = io.Discard
		if err := command.Run(); err != nil {
			return errors.New("publish tmux pane lifecycle")
		}
	}
	if _, err := io.WriteString(output, successOutput(event)); err != nil {
		return errors.New("write Codex hook response")
	}
	return nil
}

func parseHookEvent(value string) (HookEvent, error) {
	event := HookEvent(value)
	switch event {
	case HookSessionStart, HookUserPromptSubmit, HookStop:
		return event, nil
	default:
		return "", errors.New("unsupported Codex lifecycle hook event")
	}
}

func lifecycleValue(event HookEvent, origin processinfo.Observation, now time.Time) string {
	state := "idle"
	if event == HookUserPromptSubmit {
		state = "working"
	}
	return "v1:" + strconv.Itoa(int(origin.PID)) + ":" + string(origin.StartIdentity) + ":" + state + ":" + strconv.FormatInt(now.UTC().Unix(), 10)
}

func lifecycleTmuxArguments(pane string, event HookEvent, origin processinfo.Observation, now time.Time) []string {
	arguments := []string{
		"set-option", "-p", "-t", pane, "--", lifecycleOption, lifecycleValue(event, origin, now),
	}
	// Only a submitted prompt proves the user has seen the pane; SessionStart
	// resume/clear must not silently drop a pending attention badge.
	if event == HookUserPromptSubmit {
		arguments = append(arguments, ";", "set-option", "-pqu", "-t", pane, "--", "@skid_attention")
	}
	return arguments
}

func successOutput(event HookEvent) string {
	if event == HookStop {
		return "{}\n"
	}
	return ""
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

func foregroundCodexOrigin(ancestry []processinfo.Observation, paneTerminal processinfo.TerminalDevice, codexNodeEntrypoint string) (processinfo.Observation, bool) {
	terminalAncestry, valid := exactTerminalSessionAncestry(ancestry, paneTerminal)
	if !valid || terminalAncestry[0].ForegroundProcessGroup <= 0 {
		return processinfo.Observation{}, false
	}
	foregroundProcessGroup := terminalAncestry[0].ForegroundProcessGroup
	codexAncestors := make([]processinfo.Observation, 0, 2)
	for _, process := range terminalAncestry[1:] {
		if isCodexProcess(process, codexNodeEntrypoint) {
			codexAncestors = append(codexAncestors, process)
		}
	}
	var origin processinfo.Observation
	switch len(codexAncestors) {
	case 1:
		origin = codexAncestors[0]
	case 2:
		native := codexAncestors[0]
		wrapper := codexAncestors[1]
		if native.ExecutableBase() != "codex" ||
			wrapper.ExecutableBase() != "node" || wrapper.Argument(1) != codexNodeEntrypoint ||
			native.ParentPID != wrapper.PID {
			return processinfo.Observation{}, false
		}
		origin = wrapper
	default:
		return processinfo.Observation{}, false
	}
	if origin.PID != foregroundProcessGroup || origin.StartIdentity == "" {
		return processinfo.Observation{}, false
	}
	return origin, true
}

func exactTerminalSessionAncestry(ancestry []processinfo.Observation, paneTerminal processinfo.TerminalDevice) ([]processinfo.Observation, bool) {
	if len(ancestry) == 0 || paneTerminal == 0 || ancestry[0].SessionID <= 0 || ancestry[0].TerminalDevice != paneTerminal {
		return nil, false
	}
	session := ancestry[0].SessionID
	for index, observation := range ancestry {
		if observation.SessionID != session || observation.TerminalDevice != paneTerminal {
			return nil, false
		}
		if observation.PID == session {
			return ancestry[:index+1], true
		}
	}
	return nil, false
}

func isCodexProcess(process processinfo.Observation, codexNodeEntrypoint string) bool {
	return process.ExecutableBase() == "codex" ||
		process.ExecutableBase() == "node" && process.Argument(1) == codexNodeEntrypoint
}
