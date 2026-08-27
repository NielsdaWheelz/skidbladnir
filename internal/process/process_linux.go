//go:build linux

package process

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func observeOnce(pid PID) (Observation, error) {
	root := filepath.Join("/proc", strconv.Itoa(int(pid)))
	contents, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return Observation{}, classifyProcError(err, "read process stat")
	}
	fields, err := statFields(contents)
	if err != nil {
		return Observation{}, err
	}
	parent, e1 := strconv.Atoi(fields[1])
	group, e2 := strconv.Atoi(fields[2])
	session, e3 := strconv.Atoi(fields[3])
	terminal, e4 := strconv.ParseUint(fields[4], 10, 64)
	foreground, e5 := strconv.Atoi(fields[5])
	start, e6 := strconv.ParseUint(fields[19], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || session <= 0 || e4 != nil || e5 != nil || e6 != nil || start == 0 {
		return Observation{}, errors.New("process stat has invalid identity")
	}
	executable, err := os.Readlink(filepath.Join(root, "exe"))
	if err != nil {
		return Observation{}, classifyProcError(err, "read process executable")
	}
	line, err := os.ReadFile(filepath.Join(root, "cmdline"))
	if err != nil {
		return Observation{}, classifyProcError(err, "read process command line")
	}
	argv := splitArgv(line)
	if len(argv) == 0 {
		return Observation{}, errors.New("process command line is empty")
	}
	return Observation{PID: pid, ParentPID: PID(parent), ProcessGroup: PID(group), SessionID: PID(session), TerminalDevice: TerminalDevice(terminal), ForegroundProcessGroup: PID(foreground), Executable: executable, Argv: argv, StartIdentity: StartIdentity(fields[19])}, nil
}

func classifyProcError(err error, action string) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return ErrProcessAbsent
	}
	if errors.Is(err, fs.ErrPermission) {
		return ErrProcessNotPermitted
	}
	return fmt.Errorf("%s: %w", action, err)
}

func foregroundProcessGroup(panePID PID) (PID, error) {
	contents, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(panePID)), "stat"))
	if err != nil {
		return 0, classifyProcError(err, "read pane process stat")
	}
	fields, err := statFields(contents)
	if err != nil {
		return 0, err
	}
	foreground, err := strconv.Atoi(fields[5])
	if err != nil || foreground <= 0 {
		return 0, errors.New("pane has no foreground process group")
	}
	return PID(foreground), nil
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
