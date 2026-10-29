package terminalservice

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const terminalJob = DarwinLabel
const gatewayJob = "dev.niels.skidbladnir"

var launchAgentRow = regexp.MustCompile(`^([^ \t]+)[ \t]+([^ \t]+)[ \t]+(.*)$`)

func nativePID(ctx context.Context, label string) (process.PID, error) {
	// launchctl(1) documents these three columns. Its print output is not API.
	output, err := exec.CommandContext(ctx, "/bin/launchctl", "list").Output()
	if err != nil {
		return 0, fmt.Errorf("inspect native launch agent %s: %w", label, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) == 0 || strings.Join(strings.Fields(lines[0]), " ") != "PID Status Label" {
		return 0, errors.New("invalid launch agent list header")
	}
	var pid process.PID
	found := false
	for _, line := range lines[1:] {
		// The first two columns are tokens; the remaining label may contain
		// whitespace. launchctl does not promise a particular column delimiter.
		fields := launchAgentRow.FindStringSubmatch(line)
		if fields == nil {
			return 0, errors.New("invalid launch agent list row")
		}
		if fields[3] != label {
			continue
		}
		if found {
			return 0, errors.New("duplicate native launch agent")
		}
		found = true
		if _, err := strconv.Atoi(fields[2]); err != nil {
			return 0, errors.New("invalid native launch agent status")
		}
		if fields[1] != "-" {
			value, err := strconv.Atoi(fields[1])
			if err != nil || value <= 0 {
				return 0, errors.New("invalid native launch agent pid")
			}
			pid = process.PID(value)
		}
	}
	return pid, nil
}

func ownsServer(job, server process.Observation) bool {
	return server.ParentPID == job.PID && server.ProcessGroup == job.ProcessGroup
}
