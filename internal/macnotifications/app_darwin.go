//go:build darwin && cgo

package macnotifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// app is the notification app's state. Only loop touches its fields, except
// hints and hinted, which the event stream signals.
type app struct {
	configPath, skidPath string
	client               *fleetclient.Client // nil: setup required
	alerted              map[string]int64    // slot -> episode audibly posted by this process
	authorizationFailed  bool
	hints                chan struct{} // cap 1
	hinted               atomic.Uint64 // bumped by the stream on every event
}

// Run is the notification app. Its caller stays on the original main thread,
// which AppKit owns until the app quits or ctx ends.
func Run(ctx context.Context, configPath, skidPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !filepath.IsAbs(configPath) || !filepath.IsAbs(skidPath) {
		return errors.New("notification app paths must be absolute")
	}
	listener, lock, err := listenSocket()
	if err != nil {
		return err
	}
	defer lock.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	app := &app{configPath: configPath, skidPath: skidPath, alerted: make(map[string]int64), hints: make(chan struct{}, 1)}
	var workers sync.WaitGroup
	workers.Go(func() { serve(ctx, listener) })
	workers.Go(func() { app.loop(ctx, cancel) })
	stop := context.AfterFunc(ctx, nativeStop)
	nativeLoop()
	stop()
	cancel()
	bridge.close()
	workers.Wait()
	return nil
}

// loop is the app's single owner: it syncs after every hint and native event,
// one sync at a time, and owns the event stream and click launches.
func (app *app) loop(ctx context.Context, quit context.CancelFunc) {
	var workers sync.WaitGroup
	defer workers.Wait()
	stopStream := func() {}
	open := func() {
		stopStream()
		app.client, _ = fleetclient.Open(app.configPath)
		if app.client == nil {
			return
		}
		streamCtx, cancel := context.WithCancel(ctx)
		stopStream = cancel
		client := app.client
		workers.Go(func() { app.stream(streamCtx, client) })
	}
	open()
	for {
		due := false
		select {
		case <-ctx.Done():
			return
		case <-app.hints:
			due = true
		case <-bridge.wake:
			for _, event := range bridge.drain() {
				switch event.Kind {
				case "ready": // the app finished launching
				case "permission":
					var result struct {
						ErrorDomain string `json:"errorDomain"`
						ErrorCode   int    `json:"errorCode"`
					}
					app.authorizationFailed = json.Unmarshal(event.Value, &result) != nil || result.ErrorDomain != ""
					if app.authorizationFailed {
						fmt.Fprintf(os.Stderr, "notification authorization failed: %s (%d)\n", result.ErrorDomain, result.ErrorCode)
					}
				case "wake", "setup":
					open()
				case "dismiss", "click":
					if event.Kind == "click" {
						workers.Go(func() { app.openSession(ctx, event.Ref) })
					}
					// The owner has seen this episode: this process never posts it again,
					// even when it was an earlier process's notice or the ack fails.
					app.alerted[event.Slot] = event.Episode
					if app.client != nil && app.client.NotificationHandled(ctx, attention.Handled{Slot: event.Slot, Episode: event.Episode}) != nil {
						fmt.Fprintln(os.Stderr, "notification acknowledgement unavailable")
					}
				case "quit":
					quit()
				default:
					panic("unsupported native notification callback")
				}
				due = true
			}
		}
		if due {
			app.sync(ctx)
		}
	}
}

// sync reconciles the notification center with one hub state read and
// refreshes the setup window's health line. It inspects the notification
// center before reading the hub: a notice dismissed during the read is then
// still a same-episode copy, never an absent one that would post again.
func (app *app) sync(ctx context.Context) {
	var delivered []notice
	inspectErr := bridge.call(ctx, "inspect", struct{}{}, &delivered)
	var permission struct {
		Allowed      bool `json:"allowed"`
		Undetermined bool `json:"undetermined"`
	}
	permissionErr := bridge.call(ctx, "permission", struct{}{}, &permission)
	var state attention.State
	var fetchErr error
	if app.client != nil {
		hinted := app.hinted.Load()
		state, fetchErr = app.client.NotificationState(ctx)
		if fetchErr == nil && app.hinted.Load() != hinted {
			return // overtaken by a hint, whose pending token syncs again
		}
	}
	if app.client != nil && fetchErr == nil && inspectErr == nil && permissionErr == nil {
		for _, effect := range reconcile(state, delivered, app.alerted, permission.Allowed) {
			if effect.cancel {
				if bridge.call(ctx, "cancel", struct {
					Slot string `json:"slot"`
				}{effect.notice.Slot}, nil) == nil {
					delete(app.alerted, effect.notice.Slot)
				}
				continue
			}
			if bridge.call(ctx, "post", struct {
				notice
				Silent bool `json:"silent"`
			}{effect.notice, !effect.audible}, nil) == nil && effect.audible {
				app.alerted[effect.notice.Slot] = effect.notice.Episode
			}
		}
	}
	if permissionErr == nil && permission.Allowed {
		app.authorizationFailed = false
	}
	health := "allow notifications for needs input."
	switch {
	case permissionErr != nil || app.authorizationFailed:
		health = "notification delivery unavailable."
	case permission.Undetermined: // the allow button is the next step
	case !permission.Allowed:
		health = "notifications are blocked in system settings."
	case app.client == nil:
		health = "notification setup required."
	case fetchErr != nil || inspectErr != nil:
		health = "notification delivery unavailable."
	}
	_ = bridge.call(ctx, "health", struct {
		Text string `json:"text"`
	}{health}, nil)
}

// stream holds the hub's notification event stream open, so a changed state
// syncs within one round trip. It reconnects after 1 s, doubling to 30 s and
// resetting after a stream that lasted 30 s, and stops with ctx.
func (app *app) stream(ctx context.Context, client *fleetclient.Client) {
	const maximumDelay = 30 * time.Second
	backoff := time.Second
	for {
		started := time.Now()
		_ = client.NotificationEvents(ctx, func() {
			app.hinted.Add(1)
			select {
			case app.hints <- struct{}{}:
			default:
			}
		})
		if time.Since(started) >= maximumDelay {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maximumDelay)
	}
}

// openSession opens the clicked session's exact lifetime in the configured
// terminal, or explains why it could not.
func (app *app) openSession(ctx context.Context, ref string) {
	text := "notification delivery unavailable."
	if local, err := fleet.ReadLocal(app.configPath); err == nil {
		// AppleScript string escaping and shell argument escaping are separate boundaries.
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
		command := strconv.Quote(quote(app.skidPath) + " --config " + quote(app.configPath) + " enter --ref " + quote(ref))
		script := ""
		switch local.Settings.Terminal {
		case "ghostty":
			text = "could not open ghostty. open the session from skid."
			script = "with timeout of 2 seconds\ntell application id \"com.mitchellh.ghostty\"\n" +
				"set cfg to new surface configuration\nset command of cfg to " + command + "\n" +
				"set wait after command of cfg to true\nset created to new window with configuration cfg\n" +
				"activate window created\nend tell\nend timeout"
		case "apple-terminal":
			text = "could not open terminal. open the session from skid."
			script = "with timeout of 2 seconds\ntell application id \"com.apple.Terminal\"\n" +
				"do script " + command + "\nactivate\nend tell\nend timeout"
		}
		if script != "" && bridge.call(ctx, "launch", struct {
			Script string `json:"script"`
		}{script}, nil) == nil {
			return
		}
	}
	_ = bridge.call(ctx, "error", struct {
		Text string `json:"text"`
	}{text}, nil)
}

func DefaultPaths() (configPath, skidPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", attention.ErrUnavailable
	}
	directory, err := attention.StateDirectory()
	if err != nil {
		return "", "", err
	}
	return directory, filepath.Join(home, ".local", "share", "skidbladnir", "current", "skidbladnir"), nil
}
