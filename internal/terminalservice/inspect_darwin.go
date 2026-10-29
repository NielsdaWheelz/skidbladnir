package terminalservice

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func nativePID(ctx context.Context) (process.PID, error) {
	// launchctl(1) documents these three columns. Its print output is not API.
	output, err := exec.CommandContext(ctx, "/bin/launchctl", "list").Output()
	if err != nil {
		return 0, fmt.Errorf("inspect terminal launch agent: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || strings.Join(strings.Fields(lines[0]), " ") != "PID Status Label" {
		return 0, errors.New("invalid launch agent list header")
	}
	var pid process.PID
	found := false
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, errors.New("invalid launch agent list row")
		}
		if fields[2] != DarwinLabel {
			continue
		}
		if found {
			return 0, errors.New("duplicate terminal launch agent")
		}
		found = true
		if _, err := strconv.Atoi(fields[1]); err != nil {
			return 0, errors.New("invalid terminal launch agent status")
		}
		if fields[0] != "-" {
			value, err := strconv.Atoi(fields[0])
			if err != nil || value <= 0 {
				return 0, errors.New("invalid terminal launch agent pid")
			}
			pid = process.PID(value)
		}
	}
	return pid, nil
}

func ownsServer(job, server process.Observation) bool {
	return server.ParentPID == job.PID && server.ProcessGroup == job.ProcessGroup
}
