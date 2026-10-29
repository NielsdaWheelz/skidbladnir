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
	output, err := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=MainPID", LinuxUnit).Output()
	if err != nil {
		return 0, fmt.Errorf("inspect terminal user service: %w", err)
	}
	value, ok := strings.CutPrefix(strings.TrimSuffix(string(output), "\n"), "MainPID=")
	pid, err := strconv.Atoi(value)
	if !ok || err != nil || pid < 0 {
		return 0, errors.New("invalid terminal user service pid")
	}
	return process.PID(pid), nil
}

func ownsServer(job, server process.Observation) bool {
	return job.PID == server.PID && job.StartIdentity == server.StartIdentity
}
