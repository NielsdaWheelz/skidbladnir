package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
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
	"github.com/NielsdaWheelz/skidbladnir/internal/notifier"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

// serveStandalone runs one gateway process. Machine identity and hub role are
// constants of the process: when fleet commits a trust change or fails a write,
// serve returns its error and the supervisor restarts it.
func serveStandalone(stdout io.Writer) (result error) {
	directories, err := install.Paths()
	if err != nil {
		return err
	}
	home, config, state := directories.Home, directories.Config, directories.State
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	name = strings.TrimSuffix(name, ".local")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// systemd journals serve's stdout; launchd discards it, so the mac keeps a bounded file.
	logOutput := stdout
	if runtime.GOOS == "darwin" {
		logOutput = logging.BoundedFile(filepath.Join(state, "serve.log"))
	}
	logger := logging.New(logOutput)
	networkCtx, stopNetwork := context.WithTimeout(ctx, 2*time.Second)
	origin, _, networkErr := networkOrigin(networkCtx)
	stopNetwork()
	if networkErr != nil {
		origin = "" // local access needs no tailnet login
	}
	observerPath := filepath.Join(state, "notification-observer-v1.json")
	control, err := fleet.Open(fleet.Options{
		Directory: state, ConfigPath: filepath.Join(config, "config.yaml"), HandlePath: filepath.Join(config, "machine-handle"), ObserverPath: observerPath,
		Name: name, Origin: origin, Home: home, LoginShell: loginShell(ctx, home), Version: releaseVersion,
	})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, control.Close()) }()
	local, changed := control.Watch()
	handle := control.Machine()
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("tmux unavailable; run skid install")
	}
	zoxide, lookupErr := exec.LookPath("zoxide")
	if lookupErr != nil {
		zoxide = "" // directory search works without zoxide
	}
	profiles, err := agentruntime.ResolveProfiles(local.Settings.Profiles)
	if err != nil {
		return err
	}
	workingDirectories, err := workdir.New(home, zoxide)
	if err != nil {
		return err
	}
	manager, err := sessions.New(sessions.Config{Machine: handle, CheckpointPath: filepath.Join(state, "workspace.json"), TmuxPath: tmux, ShellPath: local.Settings.Shell, Workdir: workingDirectories, Profiles: profiles})
	if err != nil {
		return err
	}
	agents := agentcontrol.New(manager, filepath.Join(directories.Data, "current", "providers", "quota-read"))
	client := fleetclient.New(state, local)
	credentials := filepath.Join(config, "android-push-credentials.json")
	notifications := &notificationHost{logger: logger}
	configuredPush := pushKey(credentials, local.Push)
	if local.Hub == local.Machine {
		observer, err := notifier.New(notifier.Config{FleetID: local.FleetID, Client: client, Machine: handle, StatePath: observerPath, Fleet: control, Push: local.Push, Sender: pushSender(credentials, local.Push), Members: local.Members})
		if err != nil {
			notifications.fail()
		} else {
			notifications.observer = observer
		}
	}
	monitor := pressure.NewMonitor()
	gatewayHandler := gateway.New(gateway.Config{Agents: agents, Sessions: manager, Workdir: workingDirectories, Pressure: monitor, Fleet: control, Notifications: notifications, Logger: logger, Machine: handle})
	workers := make(chan error, 4)
	go func() { monitor.Run(ctx); workers <- nil }()
	go func() { control.Run(ctx); workers <- nil }()
	go func() {
		if notifications.observer != nil && notifications.observer.Run(ctx) != nil {
			notifications.fail()
		}
		workers <- nil
	}()
	// Every committed fleet change reaches the process here, as one snapshot.
	go func() {
		previous := local
		for {
			select {
			case <-ctx.Done():
				workers <- nil
				return
			case <-control.Done():
				cancel()
				workers <- nil
				return
			case <-changed:
			}
			var next fleet.Local
			next, changed = control.Watch()
			for _, member := range previous.Members {
				if !slices.ContainsFunc(next.Members, func(m fleet.Member) bool { return m.DeviceID == member.DeviceID }) {
					gatewayHandler.CloseDevice(member.DeviceID)
				}
			}
			profiles, err := agentruntime.ResolveProfiles(next.Settings.Profiles)
			if err == nil {
				err = manager.ApplySettings(ctx, profiles, next.Settings.Shell)
			}
			if err != nil {
				if ctx.Err() != nil {
					err = nil // shutdown interrupted the apply; nothing failed
				}
				cancel()
				workers <- err
				return
			}
			client.Adopt(next)
			if observer := notifications.observer; observer != nil {
				if observer.ReconcileMembers(next.Members) != nil {
					notifications.fail()
				}
				// Reconfiguring cancels a send in flight, so only a changed transport does.
				if key := pushKey(credentials, next.Push); key != configuredPush {
					configuredPush = key
					if observer.ConfigurePush(next.Push, pushSender(credentials, next.Push)) != nil {
						notifications.fail()
					}
				}
			}
			previous = next
		}
	}()
	result = gateway.ListenAndServe(ctx, fleet.LocalAddress, gatewayHandler)
	cancel()
	for range 4 {
		result = errors.Join(result, <-workers)
	}
	return errors.Join(result, control.Err())
}

// notificationHost serves the hub's observer. The observer is fixed before the
// gateway serves; a failed observer answers 503 until the process restarts.
type notificationHost struct {
	observer *notifier.Observer
	failed   atomic.Bool
	logger   *slog.Logger
}

func (host *notificationHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if host.observer != nil && !host.failed.Load() {
		host.observer.ServeHTTP(w, r)
		return
	}
	message := "notification hub unavailable; check skid status and service logs"
	if host.failed.Load() {
		message = "notification state unavailable; repair its state and restart the skid service"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, "{\"code\":\"NotificationsUnavailable\",\"message\":\""+message+"\"}\n") // a disconnected client cannot be answered
}
func (host *notificationHost) fail() {
	if !host.failed.Swap(true) {
		host.logger.Info("Notifications.Unavailable")
	}
}

// pushSender loads the hub's firebase credential; nil when push is off or the
// credential is missing, which affects android delivery only.
func pushSender(path string, push *hostconfig.AndroidPush) *fcmpush.Client {
	if push == nil {
		return nil
	}
	sender, err := fcmpush.Load(path, push.ProjectID)
	if err != nil {
		return nil
	}
	return sender
}

// pushKey identifies the push transport: the accepted options and the credential
// file's bytes. skid apply after replacing the credential changes it.
func pushKey(path string, push *hostconfig.AndroidPush) string {
	if push == nil {
		return ""
	}
	data, _ := os.ReadFile(path) // a missing credential is a transport without a sender
	sum := sha256.Sum256(data)
	return push.ID() + hex.EncodeToString(sum[:])
}

// loginShell is the account's shell, or "" when unavailable: config.yaml `shell`
// then supplies it, and resolution reports the gap otherwise.
func loginShell(ctx context.Context, home string) string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.CommandContext(ctx, "/usr/bin/dscl", ".", "-read", "/Users/"+filepath.Base(home), "UserShell")
	} else {
		command = exec.CommandContext(ctx, "getent", "passwd", fmt.Sprint(os.Getuid()))
	}
	data, err := command.Output()
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(data))
	if runtime.GOOS == "darwin" {
		_, value, _ = strings.Cut(value, ": ")
	} else {
		fields := strings.Split(value, ":")
		if len(fields) != 7 {
			return ""
		}
		value = fields[6]
	}
	if !filepath.IsAbs(value) {
		return ""
	}
	return value
}
