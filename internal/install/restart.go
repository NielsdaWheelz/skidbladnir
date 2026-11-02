package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalservice"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type restartSelection struct {
	observed tmux.ServerObservation
	present  bool
	process  process.Observation
}

type restartSettings struct {
	config  sessions.Config
	path    string
	encoded []byte
}

func inspectRestartSelection(ctx context.Context) (restartSelection, error) {
	client, err := terminalClient()
	if err != nil {
		return restartSelection{}, err
	}
	observed, present, err := client.InspectServer(ctx)
	if err != nil {
		return restartSelection{}, err
	}
	selection := restartSelection{observed: observed, present: present}
	if present {
		pid, err := strconv.Atoi(observed.Identity.PID)
		if err != nil {
			return restartSelection{}, err
		}
		selection.process, err = process.Observe(process.PID(pid))
		if err != nil {
			return restartSelection{}, err
		}
		if err := sessions.RefuseRuntimeAncestor(ctx, selection.process); err != nil {
			return restartSelection{}, err
		}
	}
	return selection, nil
}

func (i *installer) restartConfiguration() (restartSettings, error) {
	local, err := fleet.ReadLocal(i.dirs.State)
	if err != nil {
		return restartSettings{}, fmt.Errorf("installed settings cannot be read for terminal restart: %w", err)
	}
	handle, err := machine.Parse(local.Machine)
	if err != nil {
		return restartSettings{}, err
	}
	path := filepath.Join(i.dirs.State, "fleet.json")
	if local.Hub == local.Machine {
		path = filepath.Join(i.dirs.Config, "config.yaml")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return restartSettings{}, err
	}
	if local.Hub == local.Machine {
		parsed, err := hostconfig.Parse(encoded)
		if err != nil {
			return restartSettings{}, err
		}
		current, err := hostconfig.Resolve(parsed, local.Name, i.dirs.Home, local.Settings.Shell)
		if err != nil {
			return restartSettings{}, err
		}
		if current.Shell != local.Settings.Shell || !slices.Equal(current.Profiles, local.Settings.Profiles) {
			return restartSettings{}, errors.New("terminal settings differ from the installed gateway's accepted settings; apply them before restarting terminals")
		}
	}
	profiles, err := agentruntime.ResolveProfiles(local.Settings.Profiles)
	if err != nil {
		return restartSettings{}, err
	}
	directory, err := workdir.New(i.dirs.Home, "")
	if err != nil {
		return restartSettings{}, err
	}
	// The path is the same installed dependency selected for the runtime job.
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return restartSettings{}, err
	}
	return restartSettings{config: sessions.Config{Machine: handle, CheckpointPath: filepath.Join(i.dirs.State, "workspace.json"), TmuxPath: tmuxPath, ShellPath: local.Settings.Shell, Workdir: directory, Profiles: profiles}, path: path, encoded: encoded}, nil
}

func (i *installer) stopRestartSelection(ctx context.Context, terminal service, selected restartSelection, checkpoint *sessions.RuntimeRestart, settings restartSettings) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	current, err := os.ReadFile(settings.path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, settings.encoded) {
		return errors.New("terminal configuration changed during restart preparation")
	}
	if err := checkpoint.Recheck(ctx); err != nil {
		return err
	}
	state, err := terminalservice.Inspect(ctx, selected.observed, selected.present)
	if err != nil {
		return err
	}
	switch state {
	case terminalservice.Managed:
		return i.stopTerminal(ctx, terminal, selected.observed, true)
	case terminalservice.Absent:
		if selected.present {
			return errors.New("the selected terminal source changed")
		}
		return i.stopService(ctx, terminal)
	case terminalservice.Unavailable:
		return errors.New("the selected terminal runtime cannot be inspected for shutdown")
	case terminalservice.Unmanaged:
		if _, running, err := terminalservice.RuntimeProcess(ctx); err != nil {
			return err
		} else if running {
			return errors.New("the selected server conflicts with a running terminal service; repair the service before restarting terminals")
		}
	}
	client, err := terminalClient()
	if err != nil {
		return err
	}
	server := selected.observed.Identity
	// These are parsed epoch/digit values. The tmux queue guards the exact
	// lifetime; kill-server's deferred signal still requires the quiet interval.
	condition := fmt.Sprintf("#{&&:#{==:#{%s},%s},#{&&:#{==:#{pid},%s},#{==:#{start_time},%s}}}", tmux.ServerEpochOption, server.Epoch, server.PID, server.StartTime)
	output, err := client.Output(ctx, "checkpointed-terminal-restart", "if-shell", "-F", condition, "kill-server", "display-message -p -l 'SKID_RESTART_SOURCE_CHANGED'")
	if err != nil {
		return fmt.Errorf("terminal shutdown completion is uncertain; the checkpoint is retained: %w", err)
	}
	if output == "SKID_RESTART_SOURCE_CHANGED" {
		return errors.New("the selected terminal source changed before shutdown")
	}
	return waitStopped(ctx, checkpoint.Process)
}
