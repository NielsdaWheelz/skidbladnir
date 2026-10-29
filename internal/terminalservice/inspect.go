// Package terminalservice admits only the native terminal service's tmux lifetime.
package terminalservice

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const (
	DarwinLabel = "dev.niels.skidbladnir.terminals"
	LinuxUnit   = "skidbladnir-terminals.service"
)

type State string

const (
	Managed     State = "managed"
	Absent      State = "absent"
	Unavailable State = "unavailable"
	Unmanaged   State = "unmanaged"
)

// Inspect brackets the native job PID with kernel process identities. It does
// not start a service, stamp tmux options, or infer loss of a checkpoint source.
// The caller must separately establish loss of any previously known lifetime.
func Inspect(ctx context.Context, observed tmux.ServerObservation, present bool) (State, error) {
	var server process.Observation
	if present {
		pid, err := strconv.Atoi(observed.Identity.PID)
		if err != nil {
			return Unavailable, fmt.Errorf("terminal server pid: %w", err)
		}
		server, err = process.Observe(process.PID(pid))
		if err != nil {
			return observationFailure(err)
		}
	}
	pid, err := nativePID(ctx)
	if err != nil {
		return Unavailable, err
	}
	var job process.Observation
	if pid != 0 {
		job, err = process.Observe(pid)
		if err != nil {
			return observationFailure(err)
		}
	}
	closingPID, err := nativePID(ctx)
	if err != nil {
		return Unavailable, err
	}
	if closingPID != pid {
		return Unavailable, nil
	}
	if pid != 0 {
		after, err := process.Observe(pid)
		if err != nil {
			return observationFailure(err)
		}
		if after != job {
			return Unavailable, nil
		}
	}
	if !present {
		if pid == 0 {
			return Absent, nil
		}
		return Unavailable, nil
	}
	after, err := process.Observe(server.PID)
	if err != nil {
		return observationFailure(err)
	}
	if after != server {
		return Unavailable, nil
	}
	if pid == 0 || !ownsServer(job, server) {
		return Unmanaged, nil
	}
	return Managed, nil
}

func observationFailure(err error) (State, error) {
	if errors.Is(err, process.ErrProcessAbsent) {
		return Unavailable, nil
	}
	return Unavailable, fmt.Errorf("observe terminal service process: %w", err)
}
