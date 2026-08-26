package statushook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maximumProcessAncestry = 128
	codexNodeEntrypoint    = "/home/niels/.local/bin/codex"
	lifecycleOption        = "@skid_lifecycle"
)

type HookEvent string

const (
	HookSessionStart     HookEvent = "SessionStart"
	HookUserPromptSubmit HookEvent = "UserPromptSubmit"
	HookStop             HookEvent = "Stop"
)

type processObservation struct {
	pid                    int
	parentPID              int
	foregroundProcessGroup int
	startTime              string
	executableBase         string
	argument1              string
}

func Run(ctx context.Context, eventText string, input io.Reader, output io.Writer) error {
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
	ancestry, err := observeProcessAncestry("/proc", os.Getpid())
	if err != nil {
		return err
	}
	if origin, valid := foregroundCodexOrigin(ancestry); valid {
		command := exec.CommandContext(
			ctx,
			"/usr/bin/tmux",
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

func lifecycleValue(event HookEvent, origin processObservation, now time.Time) string {
	state := "idle"
	if event == HookUserPromptSubmit {
		state = "working"
	}
	return "v1:" + strconv.Itoa(origin.pid) + ":" + origin.startTime + ":" + state + ":" + strconv.FormatInt(now.UTC().Unix(), 10)
}

func lifecycleTmuxArguments(pane string, event HookEvent, origin processObservation, now time.Time) []string {
	arguments := []string{
		"set-option", "-p", "-t", pane, "--", lifecycleOption, lifecycleValue(event, origin, now),
	}
	if event != HookStop {
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

func observeProcessAncestry(procRoot string, initialPID int) ([]processObservation, error) {
	if initialPID <= 0 {
		return nil, errors.New("invalid hook process id")
	}
	ancestry := make([]processObservation, 0, 8)
	seen := make(map[int]struct{}, 8)
	pid := initialPID
	for len(ancestry) < maximumProcessAncestry {
		if _, duplicate := seen[pid]; duplicate {
			return nil, errors.New("process ancestry contains a cycle")
		}
		seen[pid] = struct{}{}
		observation, err := observeProcess(procRoot, pid)
		if err != nil {
			return nil, err
		}
		ancestry = append(ancestry, observation)
		if observation.parentPID <= 0 || observation.pid == 1 {
			return ancestry, nil
		}
		pid = observation.parentPID
	}
	return nil, errors.New("process ancestry exceeds its closed bound")
}

func observeProcess(procRoot string, pid int) (processObservation, error) {
	processRoot := filepath.Join(procRoot, strconv.Itoa(pid))
	contents, err := os.ReadFile(filepath.Join(processRoot, "stat"))
	if err != nil {
		return processObservation{}, fmt.Errorf("read hook process stat: %w", err)
	}
	closingParenthesis := strings.LastIndexByte(string(contents), ')')
	if closingParenthesis < 0 {
		return processObservation{}, errors.New("hook process stat has no command terminator")
	}
	fields := strings.Fields(string(contents[closingParenthesis+1:]))
	if len(fields) < 20 {
		return processObservation{}, errors.New("hook process stat is incomplete")
	}
	parentPID, parentErr := strconv.Atoi(fields[1])
	foregroundProcessGroup, foregroundErr := strconv.Atoi(fields[5])
	startTime, startTimeErr := strconv.ParseUint(fields[19], 10, 64)
	if parentErr != nil || parentPID < 0 || foregroundErr != nil || startTimeErr != nil || startTime == 0 {
		return processObservation{}, errors.New("hook process stat has invalid ancestry")
	}
	executable, err := os.Readlink(filepath.Join(processRoot, "exe"))
	if err != nil {
		return processObservation{}, fmt.Errorf("read hook process executable: %w", err)
	}
	commandLine, err := os.ReadFile(filepath.Join(processRoot, "cmdline"))
	if err != nil {
		return processObservation{}, fmt.Errorf("read hook process command line: %w", err)
	}
	arguments := strings.Split(strings.TrimSuffix(string(commandLine), "\x00"), "\x00")
	observation := processObservation{
		pid:                    pid,
		parentPID:              parentPID,
		foregroundProcessGroup: foregroundProcessGroup,
		startTime:              fields[19],
		executableBase:         filepath.Base(executable),
	}
	if len(arguments) > 1 {
		observation.argument1 = arguments[1]
	}
	return observation, nil
}

func foregroundCodexOrigin(ancestry []processObservation) (processObservation, bool) {
	if len(ancestry) == 0 || ancestry[0].foregroundProcessGroup <= 0 {
		return processObservation{}, false
	}
	foregroundProcessGroup := ancestry[0].foregroundProcessGroup
	codexAncestors := make([]processObservation, 0, 2)
	for _, process := range ancestry[1:] {
		if isCodexProcess(process) {
			codexAncestors = append(codexAncestors, process)
		}
	}
	var origin processObservation
	switch len(codexAncestors) {
	case 1:
		origin = codexAncestors[0]
	case 2:
		native := codexAncestors[0]
		wrapper := codexAncestors[1]
		if native.executableBase != "codex" ||
			wrapper.executableBase != "node" || wrapper.argument1 != codexNodeEntrypoint ||
			native.parentPID != wrapper.pid {
			return processObservation{}, false
		}
		origin = wrapper
	default:
		return processObservation{}, false
	}
	if origin.pid != foregroundProcessGroup || origin.startTime == "" {
		return processObservation{}, false
	}
	return origin, true
}

func isCodexProcess(process processObservation) bool {
	return process.executableBase == "codex" ||
		process.executableBase == "node" && process.argument1 == codexNodeEntrypoint
}
