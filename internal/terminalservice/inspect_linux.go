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

const terminalJob = LinuxUnit
const gatewayJob = "skidbladnir.service"

func nativePID(ctx context.Context, unit string) (process.PID, error) {
	output, err := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=MainPID", unit).Output()
	if err != nil {
		return 0, fmt.Errorf("inspect native user service %s: %w", unit, err)
	}
	value, ok := strings.CutPrefix(strings.TrimSuffix(string(output), "\n"), "MainPID=")
	pid, err := strconv.Atoi(value)
	if !ok || err != nil || pid < 0 {
		return 0, errors.New("invalid native user service pid")
	}
	return process.PID(pid), nil
}

func ownsServer(job, server process.Observation) bool {
	return job.PID == server.PID && job.StartIdentity == server.StartIdentity
}
