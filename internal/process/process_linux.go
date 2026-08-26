//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func observeOnce(pid PID) (Observation, error) {
	root := filepath.Join("/proc", strconv.Itoa(int(pid)))
	contents, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return Observation{}, fmt.Errorf("read process stat: %w", err)
	}
	fields, err := statFields(contents)
	if err != nil {
		return Observation{}, err
	}
	parent, e1 := strconv.Atoi(fields[1])
	group, e2 := strconv.Atoi(fields[2])
	foreground, e3 := strconv.Atoi(fields[5])
	start, e4 := strconv.ParseUint(fields[19], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || start == 0 {
		return Observation{}, errors.New("process stat has invalid identity")
	}
	executable, err := os.Readlink(filepath.Join(root, "exe"))
	if err != nil {
		return Observation{}, fmt.Errorf("read process executable: %w", err)
	}
	line, err := os.ReadFile(filepath.Join(root, "cmdline"))
	if err != nil {
		return Observation{}, fmt.Errorf("read process command line: %w", err)
	}
	argv := splitArgv(line)
	if len(argv) == 0 {
		return Observation{}, errors.New("process command line is empty")
	}
	return Observation{PID: pid, ParentPID: PID(parent), ProcessGroup: PID(group), ForegroundProcessGroup: PID(foreground), Executable: executable, Argv: argv, StartIdentity: StartIdentity(fields[19])}, nil
}

func foregroundProcessGroup(panePID PID, _ string) (PID, error) {
	observation, err := observeOnce(panePID)
	if err != nil {
		return 0, err
	}
	if observation.ForegroundProcessGroup <= 0 {
		return 0, errors.New("pane has no foreground process group")
	}
	return observation.ForegroundProcessGroup, nil
}

func statFields(contents []byte) ([]string, error) {
	closing := strings.LastIndexByte(string(contents), ')')
	if closing < 0 {
		return nil, errors.New("process stat has no command terminator")
	}
	fields := strings.Fields(string(contents[closing+1:]))
	if len(fields) < 20 {
		return nil, errors.New("process stat is incomplete")
	}
	return fields, nil
}

func splitArgv(contents []byte) []string {
	trimmed := strings.TrimSuffix(string(contents), "\x00")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\x00")
}
