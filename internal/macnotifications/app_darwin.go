//go:build darwin && cgo

package macnotifications

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// Run starts the sole mac state writer. Its caller must remain on the original
// main goroutine; native callbacks and state work never synchronously wait on
// each other across the main-thread boundary.
func Run(ctx context.Context, configPath, skidPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !filepath.IsAbs(configPath) || !filepath.IsAbs(skidPath) {
		return errors.New("notification app paths must be absolute")
	}
	client, err := fleetclient.Open(configPath)
	if err != nil {
		client = nil
	}
	store, err := fleetclient.DefaultNotificationStore()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	owner, err := newOwner(ctx, client, store, configPath, skidPath)
	if err != nil {
		return err
	}
	listener, lock, err := listenSocket()
	if err != nil {
		return err
	}
	defer lock.Close() // justify-ignore-error: releasing process ownership happens after its joined work.
	var workers sync.WaitGroup
	var authorizationFailed atomic.Bool
	workers.Go(owner.run)
	workers.Go(func() { owner.serve(listener) })
	workers.Go(func() { owner.nativeEffects(&authorizationFailed) })
	workers.Go(func() { owner.nativeEvents(cancel, &authorizationFailed) })
	stop := context.AfterFunc(ctx, nativeStop)
	nativeLoop()
	stop()
	cancel()
	_ = listener.Close() // justify-ignore-error: cancellation closes admission before joining owned work.
	bridge.close()
	workers.Wait()
	return nil
}

func (owner *owner) nativeEffects(authorizationFailed *atomic.Bool) {
	for {
		select {
		case <-owner.ctx.Done():
			return
		case <-owner.effects:
		}
		var observation inspection
		observation.known = bridge.call(owner.ctx, "inspect", struct{}{}, &observation.notices) == nil
		needed, err := owner.request(owner.ctx, "focus-needed", 0, nil)
		if err != nil {
			return
		}
		if needed.(bool) { // justify-assertion: the owner returns this operation's boolean.
			var focus struct {
				ID string `json:"id"`
			}
			if bridge.call(owner.ctx, "focus", struct{}{}, &focus) == nil {
				observation.focus = focus.ID
			}
		}
		var permission struct {
			Allowed      bool `json:"allowed"`
			Undetermined bool `json:"undetermined"`
		}
		observation.permission = bridge.call(owner.ctx, "permission", struct{}{}, &permission) == nil
		observation.allowed = permission.Allowed
		value, err := owner.request(owner.ctx, "plan", 0, observation)
		if err != nil {
			continue
		}
		plan := value.([]nativeEffect) // justify-assertion: only the owning native planner returns this typed effect list.
		confirmReset := false
		for len(plan) != 0 {
			effect := plan[0]
			plan = plan[1:]
			if effect.rename {
				var notices []nativeNotice
				if bridge.call(owner.ctx, "inspect", struct{}{}, &notices) != nil {
					continue
				}
				present := false
				for _, notice := range notices {
					if notice.Slot == effect.notice.Slot && notice.Epoch == effect.notice.Epoch && notice.Episode == effect.notice.Episode {
						present = true
					}
				}
				if !present {
					_, _ = owner.request(owner.ctx, "absent", 0, nativeEvent{Slot: effect.notice.Slot, Epoch: effect.notice.Epoch, Episode: effect.notice.Episode}) // justify-ignore-error: the next inspection repeats absence admission if persistence fails.
					continue
				}
			}
			value, err := owner.request(owner.ctx, "submit", 0, effect)
			if err != nil {
				return
			}
			receipt := value.(*nativeReceipt) // justify-assertion: submission returns its completion receipt, or a typed nil when ineligible.
			if receipt == nil {
				continue
			}
			// Await completion even after a new intent supersedes this one. An
			// uncertain old add must finish before cancellation or a successor add.
			if err := bridge.await(owner.ctx, receipt, nil); err != nil {
				if errors.Is(err, errNativeRejected) && !effect.cancel {
					_, _ = owner.request(owner.ctx, "rejected", 0, effect) // justify-ignore-error: process shutdown discards the unresolved copy.
				}
				continue
			}
			value, err = owner.request(owner.ctx, "settle", 0, effect)
			if err != nil {
				continue
			}
			if value.(bool) { // justify-assertion: settlement reports whether that completed old effect needs cancellation.
				plan = append([]nativeEffect{{notice: effect.notice, cancel: true}}, plan...)
			}
			confirmReset = confirmReset || effect.confirmReset
		}
		if confirmReset {
			var confirmed inspection
			confirmed.known = bridge.call(owner.ctx, "inspect", struct{}{}, &confirmed.notices) == nil
			if confirmed.known {
				_, _ = owner.request(owner.ctx, "plan", 0, confirmed) // justify-ignore-error: one confirmation may finish reset; remaining cancellation waits for an existing explicit wake.
			}
		}
		health, err := owner.request(owner.ctx, "health", 0, nil)
		if err == nil {
			text := health.(string) // justify-assertion: health copy is owned by the state actor.
			if observation.permission && permission.Allowed {
				authorizationFailed.Store(false)
			}
			if !observation.permission || authorizationFailed.Load() {
				text = "notification delivery unavailable."
			} else if permission.Undetermined {
				text = "allow notifications for needs input."
			} else if !permission.Allowed {
				text = "notifications are blocked in system settings."
			}
			_ = bridge.call(owner.ctx, "health", struct {
				Text string `json:"text"`
			}{text}, nil) // justify-ignore-error: the next setup/effect cycle refreshes a quiet disclosure.
		}
	}
}

func (owner *owner) nativeEvents(cancel context.CancelFunc, authorizationFailed *atomic.Bool) {
	var launches sync.WaitGroup
	defer launches.Wait()
	for {
		select {
		case <-owner.ctx.Done():
			return
		case <-bridge.wake:
			for _, event := range bridge.drain() {
				switch event.Kind {
				case "ready", "wake", "setup", "permission":
					if event.Kind == "permission" {
						var result struct {
							Allowed     bool   `json:"allowed"`
							ErrorDomain string `json:"errorDomain"`
							ErrorCode   int    `json:"errorCode"`
						}
						if attention.Decode(event.Value, &result) != nil {
							authorizationFailed.Store(true)
							fmt.Fprintln(os.Stderr, "notification authorization completion unavailable")
						} else {
							authorizationFailed.Store(result.ErrorDomain != "")
							if result.ErrorDomain != "" {
								fmt.Fprintf(os.Stderr, "notification authorization failed: %s (%d)\n", result.ErrorDomain, result.ErrorCode)
							}
						}
					}
					if event.Kind == "setup" || event.Kind == "wake" {
						_, _ = owner.request(owner.ctx, "reload", 0, nil) // justify-ignore-error: reload failure is disclosed as setup required.
					}
					select {
					case owner.refresh <- struct{}{}:
					default:
					}
					owner.signalEffects()
				case "reset":
					_, _ = owner.request(owner.ctx, "reset", 0, nil) // justify-ignore-error: a failed reset leaves state intact and health remains unavailable.
				case "dismiss":
					_, _ = owner.request(owner.ctx, "dismiss", 0, event) // justify-ignore-error: inspection observes dismissal if its state write failed.
				case "click":
					_, _ = owner.request(owner.ctx, "dismiss", 0, event) // justify-ignore-error: click navigation is independent of notification memory availability.
					launches.Go(func() { owner.openSession(event.Ref) })
				case "quit":
					cancel()
				default:
					panic("unsupported native notification callback")
				}
			}
		}
	}
}

func (owner *owner) openSession(ref string) {
	value, err := owner.request(owner.ctx, "launch", 0, ref)
	if err != nil {
		_ = bridge.call(owner.ctx, "error", struct {
			Text string `json:"text"`
		}{"notification delivery unavailable."}, nil) // justify-ignore-error: a failed native dialog has no secondary surface.
		return
	}
	launch := value.(struct {
		Nonce  string
		Script string
	}) // justify-assertion: creator admission owns the launch shape.
	var surface struct {
		ID string `json:"id"`
	}
	err = bridge.call(owner.ctx, "launch", struct {
		Script string `json:"script"`
	}{launch.Script}, &surface)
	if err != nil {
		surface.ID = ""
	}
	_, _ = owner.request(owner.ctx, "launched", 0, launchResult{launch.Nonce, surface.ID}) // justify-ignore-error: app shutdown retires its ephemeral launch associations.
	if err != nil {
		_ = bridge.call(owner.ctx, "error", struct {
			Text string `json:"text"`
		}{"could not open ghostty. open the session from skid."}, nil) // justify-ignore-error: a failed native dialog has no secondary surface.
	}
}

func ghosttyScript(skidPath, configPath, ref, nonce string) (string, error) {
	directory, err := attention.StateDirectory()
	if err != nil {
		return "", ErrUnavailable
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	command := quote(skidPath) + " --config " + quote(configPath) + " enter --ref " + quote(ref)
	// AppleScript string escaping and shell argument escaping are separate
	// boundaries. Surface configuration launches no names or terminal text.
	return "with timeout of 2 seconds\ntell application id \"com.mitchellh.ghostty\"\n" +
		"set cfg to new surface configuration\nset command of cfg to " + strconv.Quote(command) + "\n" +
		"set environment variables of cfg to {" + strconv.Quote("SKID_NOTIFICATION_LAUNCH="+nonce) + ", " + strconv.Quote("XDG_STATE_HOME="+filepath.Dir(directory)) + "}\n" +
		"set wait after command of cfg to true\nset created to new window with configuration cfg\n" +
		"activate window created\nreturn id of focused terminal of selected tab of created\nend tell\nend timeout", nil
}

func DefaultPaths() (configPath, skidPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", ErrUnavailable
	}
	return filepath.Join(home, ".config", "skidbladnir", "client.json"), filepath.Join(home, ".local", "share", "skidbladnir", "current", "skidbladnir"), nil
}
