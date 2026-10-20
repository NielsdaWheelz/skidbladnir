package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/notifier"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type runtimeHostConfig struct {
	TmuxPath, ZoxidePath, NativeControlPath string
	Profiles                                []agentruntime.Profile
}

func loadRuntimeHostConfig(ctx context.Context) (runtimeHostConfig, error) {
	directories, err := install.Paths()
	if err != nil {
		return runtimeHostConfig{}, err
	}
	settings, _, err := fleet.LoadResolved(directories.State)
	if err != nil {
		return runtimeHostConfig{}, err
	}
	profiles, err := agentruntime.ResolveProfiles(settings.Profiles)
	if err != nil {
		return runtimeHostConfig{}, err
	}
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return runtimeHostConfig{}, errors.New("tmux unavailable; run skid install")
	}
	version, err := exec.CommandContext(ctx, tmux, "-V").Output()
	if err != nil || len(version) < 2 || version[len(version)-1] != '\n' || bytes.ContainsRune(version[:len(version)-1], '\n') {
		return runtimeHostConfig{}, errors.New("tmux version unavailable")
	}
	if err := hostconfig.ValidateTmuxVersion(string(version[:len(version)-1])); err != nil {
		return runtimeHostConfig{}, err
	}
	zoxide, lookupErr := exec.LookPath("zoxide")
	if lookupErr != nil {
		zoxide = ""
	} // justify-ignore-error: directory search explicitly supports an absent optional zoxide.
	return runtimeHostConfig{tmux, zoxide, filepath.Join(directories.Data, "current", "providers", "native-control"), profiles}, nil
}

func serveStandalone(logOutput io.Writer) (result error) {
	directories, err := install.Paths()
	if err != nil {
		return err
	}
	home, config, state := directories.Home, directories.Config, directories.State
	handle, err := machine.Init(filepath.Join(config, "machine-handle"))
	if err != nil {
		return err
	}
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	name = strings.TrimSuffix(name, ".local")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logger := logging.New(logOutput)
	credentialPath := filepath.Join(config, "android-push-credentials.json")
	host := &notificationHost{credentialPath: credentialPath, logger: &logger, path: filepath.Join(state, "notification-observer-v1.json"), changed: make(chan struct{}, 1)}
	var manager atomic.Pointer[sessions.Manager]
	var handler atomic.Pointer[gateway.Gateway]
	shell, err := loginShell(ctx, home)
	if err != nil {
		return err
	}
	networkCtx, stopNetwork := context.WithTimeout(ctx, 2*time.Second)
	origin, networkErr := networkOrigin(networkCtx)
	stopNetwork()
	if networkErr != nil {
		origin = ""
	} // justify-ignore-error: local access requires no tailnet login.
	control, err := fleet.Open(fleet.Options{
		Directory: state, ConfigPath: filepath.Join(config, "config.yaml"), Machine: handle.String(), Name: name, Home: home, LoginShell: shell, Origin: origin, Version: releaseVersion,
		PushChanged: host.applyPush,
		ValidateHub: func(push *hostconfig.AndroidPush) error {
			if push == nil {
				return nil
			}
			_, err := fcmpush.Load(credentialPath, push.ProjectID)
			return err
		},
		Resolve: func(settings hostconfig.Settings) error {
			profiles, err := agentruntime.ResolveProfiles(settings.Profiles)
			if err != nil {
				return err
			}
			if current := manager.Load(); current != nil {
				if err := current.ApplySettings(ctx, profiles, settings.Shell); err != nil {
					return err
				}
			}
			host.signal()
			return nil
		},
		CloseDevice: func(device string) {
			if current := handler.Load(); current != nil {
				current.CloseDevice(device)
			}
		},
		DeviceRemoved: host.revoke,
		Recovery:      host.recover,
		Drain:         host.drain,
		Resume:        func() error { host.signal(); return nil },
		Activate: func(owned map[string]json.RawMessage) error {
			if err := notifier.Restore(host.path, owned); err != nil {
				return err
			}
			host.signal()
			return nil
		},
		Detach: func() error { _, err := host.drain(); return err },
	})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, control.Close()) }()
	settings, application := control.Settings()
	if application.AppliedConfigRevision == 0 {
		return errors.New("local settings invalid; fix config.yaml and restart skid")
	}
	runtime, err := loadRuntimeHostConfig(ctx)
	if err != nil {
		return err
	}
	workingDirectories, err := workdir.New(home, runtime.ZoxidePath)
	if err != nil {
		return err
	}
	sessionManager, err := sessions.New(sessions.Config{Machine: handle, CheckpointPath: filepath.Join(state, "workspace.json"), TmuxPath: runtime.TmuxPath, ShellPath: settings.Shell, Workdir: workingDirectories, CataloguePath: filepath.Join(directories.Data, "current", "characters.json"), Profiles: runtime.Profiles})
	if err != nil {
		return err
	}
	manager.Store(sessionManager)
	agents, err := agentcontrol.New(sessionManager, runtime.NativeControlPath)
	if err != nil {
		return err
	}
	client, err := fleetclient.Open(state)
	if err != nil {
		return err
	}
	monitor := pressure.NewMonitor()
	gatewayHandler := gateway.New(gateway.Config{Agents: agents, Sessions: sessionManager, Workdir: workingDirectories, Pressure: monitor, Fleet: control, Notifications: host, Logger: logger, Machine: handle, Platform: platform.Current()})
	handler.Store(gatewayHandler)
	if err := host.reconcile(ctx, control, client, handle); err != nil {
		host.failed(err)
	}
	workers := make(chan error, 3)
	go func() { monitor.Run(ctx); workers <- nil }()
	go func() {
		err := control.Run(ctx)
		if err != nil {
			cancel()
		}
		workers <- err
	}()
	go func() { workers <- host.run(ctx, control, client, handle) }()
	result = gateway.ListenAndServe(ctx, "127.0.0.1:7341", gatewayHandler)
	cancel()
	for range 3 {
		result = errors.Join(result, <-workers)
	}
	return result
}
