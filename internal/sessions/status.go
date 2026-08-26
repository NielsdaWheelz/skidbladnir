package sessions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type foregroundProcess struct {
	pid            int
	startTime      string
	executableBase string
	argument0      string
	argument1      string
}

func (manager *Manager) deriveStatus(panePID int, lifecycleValue string, now time.Time) Status {
	now = now.UTC()
	process, err := observeForegroundProcess(panePID)
	if err != nil {
		return Status{Kind: StatusUnknown, Signal: StatusSignalPollFailure, SignalAt: now}
	}
	if !manager.matchesAgent(process) {
		return Status{Kind: StatusShell, Signal: StatusSignalProcess, SignalAt: now}
	}
	if lifecycle, valid := parseLifecycleStatus(lifecycleValue, process, now); valid {
		return lifecycle
	}
	return runningStatus(now)
}

func parseLifecycleStatus(value string, process foregroundProcess, now time.Time) (Status, bool) {
	fields := strings.Split(value, ":")
	if len(fields) != 5 || fields[0] != "v1" || process.pid <= 0 || process.startTime == "" {
		return Status{}, false
	}
	originPID, err := strconv.Atoi(fields[1])
	if err != nil || originPID != process.pid || fields[2] != process.startTime {
		return Status{}, false
	}
	seconds, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil || seconds <= 0 {
		return Status{}, false
	}
	signalAt := time.Unix(seconds, 0).UTC()
	if signalAt.After(now.UTC()) {
		return Status{}, false
	}
	status := Status{Signal: StatusSignalLifecycle, SignalAt: signalAt}
	switch fields[3] {
	case "working":
		status.Kind = StatusWorking
	case "idle":
		status.Kind = StatusIdle
	default:
		return Status{}, false
	}
	return status, true
}

func runningStatus(now time.Time) Status {
	return Status{Kind: StatusRunning, Signal: StatusSignalProcess, SignalAt: now.UTC()}
}

func (manager *Manager) matchesAgent(process foregroundProcess) bool {
	for _, profile := range manager.profiles {
		for _, signature := range profile.ForegroundSignatures {
			if (signature.ExecutableBase == "" || process.executableBase == signature.ExecutableBase) &&
				(signature.Argument0 == "" || process.argument0 == signature.Argument0) &&
				(signature.Argument1 == "" || process.argument1 == signature.Argument1) {
				return true
			}
		}
	}
	return false
}

func observeForegroundProcess(panePID int) (foregroundProcess, error) {
	contents, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(panePID), "stat"))
	if err != nil {
		return foregroundProcess{}, fmt.Errorf("read pane process stat: %w", err)
	}
	closingParenthesis := strings.LastIndexByte(string(contents), ')')
	if closingParenthesis < 0 {
		return foregroundProcess{}, errors.New("pane process stat has no command terminator")
	}
	fields := strings.Fields(string(contents[closingParenthesis+1:]))
	if len(fields) < 6 {
		return foregroundProcess{}, errors.New("pane process stat is incomplete")
	}
	foregroundPID, err := strconv.Atoi(fields[5])
	if err != nil || foregroundPID <= 0 {
		return foregroundProcess{}, errors.New("pane process has no foreground process group")
	}
	processRoot := filepath.Join("/proc", strconv.Itoa(foregroundPID))
	processStat, err := os.ReadFile(filepath.Join(processRoot, "stat"))
	if err != nil {
		return foregroundProcess{}, fmt.Errorf("read foreground process stat: %w", err)
	}
	startTime, err := parseProcessStartTime(processStat)
	if err != nil {
		return foregroundProcess{}, err
	}
	executable, err := os.Readlink(filepath.Join(processRoot, "exe"))
	if err != nil {
		return foregroundProcess{}, fmt.Errorf("read foreground executable: %w", err)
	}
	commandLine, err := os.ReadFile(filepath.Join(processRoot, "cmdline"))
	if err != nil {
		return foregroundProcess{}, fmt.Errorf("read foreground command line: %w", err)
	}
	arguments := strings.Split(strings.TrimSuffix(string(commandLine), "\x00"), "\x00")
	if len(arguments) == 0 || arguments[0] == "" {
		return foregroundProcess{}, errors.New("foreground command line is empty")
	}
	process := foregroundProcess{
		pid:            foregroundPID,
		startTime:      startTime,
		executableBase: filepath.Base(executable),
		argument0:      arguments[0],
	}
	if len(arguments) > 1 {
		process.argument1 = arguments[1]
	}
	return process, nil
}

func parseProcessStartTime(contents []byte) (string, error) {
	closingParenthesis := strings.LastIndexByte(string(contents), ')')
	if closingParenthesis < 0 {
		return "", errors.New("foreground process stat has no command terminator")
	}
	fields := strings.Fields(string(contents[closingParenthesis+1:]))
	if len(fields) < 20 {
		return "", errors.New("foreground process stat is incomplete")
	}
	startTime := fields[19]
	parsed, err := strconv.ParseUint(startTime, 10, 64)
	if err != nil || parsed == 0 {
		return "", errors.New("foreground process stat has an invalid start time")
	}
	return startTime, nil
}

func parseAttentionTime(value string, now time.Time) (time.Time, bool) {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	observed := time.Unix(seconds, 0).UTC()
	if observed.After(now) {
		return time.Time{}, false
	}
	return observed, true
}
