package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalservice"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// ActionRequired is an expected refusal the owner must resolve before retrying.
type ActionRequired struct{ Err error }

func (e *ActionRequired) Error() string { return e.Err.Error() }
func (e *ActionRequired) Unwrap() error { return e.Err }

const drainInstructions = "finish work, close the tmux sessions and pause new session creation while rerunning this operation"
const conflictInstructions = "the default tmux server was started outside the terminal service. finish work, close its sessions and pause new session creation. stop the empty server, then rerun the installer."

func (i *installer) stopTerminal(ctx context.Context, s service, observed tmux.ServerObservation, present bool) error {
	job, running, err := terminalservice.RuntimeProcess(ctx)
	if err != nil {
		return err
	}
	if running != present {
		return errors.New("the drained terminal runtime changed before shutdown; " + drainInstructions)
	}
	var server process.Observation
	if present {
		pid, err := strconv.Atoi(observed.Identity.PID)
		if err != nil {
			return err
		}
		server, err = process.Observe(process.PID(pid))
		if err != nil {
			return err
		}
		owned := server.PID == job.PID && server.StartIdentity == job.StartIdentity
		if runtime.GOOS == "darwin" {
			owned = server.ParentPID == job.PID && server.ProcessGroup == job.ProcessGroup
		}
		if !owned {
			return errors.New("the drained terminal runtime changed before shutdown; " + drainInstructions)
		}
	}
	if err = i.stopService(ctx, s); err != nil {
		return err
	}
	if running {
		return waitStopped(ctx, job, server)
	}
	return nil
}

func terminalClient() (tmux.Client, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return tmux.Client{}, err
	}
	return tmux.New(path, "")
}

func (i *installer) inspectTerminal(ctx context.Context, drain bool) (tmux.ServerObservation, bool, error) {
	client, err := terminalClient()
	if err != nil {
		return tmux.ServerObservation{}, false, err
	}
	observed, present, err := client.InspectServer(ctx)
	if err != nil {
		return observed, present, fmt.Errorf("the terminal server cannot be inspected: %w", err)
	}
	state, err := terminalservice.Inspect(ctx, observed, present)
	if err != nil {
		return observed, present, fmt.Errorf("the terminal service cannot be inspected: %w", err)
	}
	switch state {
	case terminalservice.Unmanaged:
		return observed, present, &ActionRequired{errors.New(conflictInstructions)}
	case terminalservice.Unavailable:
		return observed, present, &ActionRequired{errors.New("the terminal service is unavailable on this machine; repair terminal-service setup before rerunning this operation")}
	case terminalservice.Managed, terminalservice.Absent:
	}
	if drain && observed.SessionCount != 0 {
		return observed, present, &ActionRequired{errors.New("tmux sessions are still running; " + drainInstructions)}
	}
	if drain {
		if err = i.checkCheckpoint(observed, present); err != nil {
			return observed, present, err
		}
	}
	return observed, present, nil
}

func (i *installer) checkCheckpoint(observed tmux.ServerObservation, present bool) error {
	path := filepath.Join(i.dirs.State, "workspace.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	local, err := fleet.ReadLocal(i.dirs.State)
	if err != nil {
		return fmt.Errorf("the workspace checkpoint cannot be checked without readable installed state: %w", err)
	}
	handle, err := machine.Parse(local.Machine)
	if err != nil {
		return err
	}
	if err = sessions.CheckDrainedCheckpoint(path, handle, i.dirs.Home, observed, present); err != nil {
		return fmt.Errorf("the workspace checkpoint is not drained; repair it using the current installation: %w", err)
	}
	return nil
}

func (i *installer) waitTerminal(ctx context.Context) error {
	ready, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client, err := terminalClient()
	if err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		observed, present, inspectErr := client.InspectServer(ready)
		if inspectErr == nil {
			state, stateErr := terminalservice.Inspect(ready, observed, present)
			if stateErr == nil && state == terminalservice.Managed {
				options, optionErr := client.Output(ready, "terminal-service-readiness", "display-message", "-p", "#{exit-empty}|#{exit-unattached}")
				if optionErr == nil {
					if strings.TrimSpace(options) != "0|0" {
						return errors.New("the terminal service requires exit-empty off and exit-unattached off; correct those settings in your tmux configuration, then rerun the installer")
					}
					after, ok, afterErr := client.InspectServer(ready)
					if afterErr == nil && ok && after.Identity == observed.Identity {
						return nil
					}
				}
				last = optionErr
			} else if state == terminalservice.Unmanaged {
				return errors.New(conflictInstructions)
			} else {
				last = stateErr
			}
		} else {
			last = inspectErr
		}
		select {
		case <-ready.Done():
			return fmt.Errorf("the terminal service did not become ready within 5 s; check tmux configuration and set exit-empty off and exit-unattached off: %w", errors.Join(ready.Err(), last))
		case <-ticker.C:
		}
	}
}

func (i *installer) runtimeChanged(ctx context.Context, payload string, s service) (bool, error) {
	sameRuntime := true
	if runtime.GOOS == "darwin" {
		installedApp := filepath.Join(i.dirs.Data, "terminal-runtime", "SkidTerminals.app")
		if _, err := os.Stat(installedApp); errors.Is(err, os.ErrNotExist) {
			sameRuntime = false
		} else if err != nil {
			return false, err
		} else {
			var err error
			sameRuntime, err = sameMacRuntime(ctx, installedApp, filepath.Join(payload, "terminal-runtime", "SkidTerminals.app"))
			if err != nil {
				return false, fmt.Errorf("the installed terminal runtime cannot be verified; drain and stop it, then repair its files from a verified release before rerunning: %w", err)
			}
		}
	}
	installed, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !sameRuntime || !bytes.Equal(installed, []byte(s.text)), nil
}

func (i *installer) replaceRuntime(payload, stage string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	path := filepath.Join(i.dirs.Data, "terminal-runtime")
	previous := filepath.Join(stage, "previous-runtime")
	if err := os.Rename(path, previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(filepath.Join(payload, "terminal-runtime"), path); err != nil {
		if restore := os.Rename(previous, path); !errors.Is(restore, os.ErrNotExist) {
			err = errors.Join(err, restore)
		}
		return err
	}
	return nil
}
