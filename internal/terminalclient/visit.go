package terminalclient

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
)

// entryVisit owns optional preparation and one ordered notification visit.
// Neither preparation nor its RPC worker writes to or waits on the terminal.
type entryVisit struct {
	slot    attention.Slot
	hello   chan terminal.HelloFrame
	written chan attention.ReadySeed
	cancel  context.CancelFunc
	workers sync.WaitGroup
	mu      sync.Mutex
	seed    attention.ReadySeed
	err     error
}

func startEntryVisit(ctx context.Context, ref fleetclient.Reference, seed *attention.ReadySeed, nonce string) *entryVisit {
	ctx, cancel := context.WithCancel(ctx)
	visit := &entryVisit{slot: attention.Slot{Machine: ref.Machine, TmuxID: ref.TmuxID, IdentityToken: ref.IdentityToken}, hello: make(chan terminal.HelloFrame, 1), written: make(chan attention.ReadySeed, 1), cancel: cancel}
	if seed != nil {
		if seed.Valid() {
			visit.seed = copySeed(*seed)
		} else {
			visit.failed(attention.ErrUnavailable)
		}
	} else {
		visit.workers.Go(func() {
			preparationCtx, stop := context.WithTimeout(ctx, 3*time.Second)
			defer stop()
			var prepared attention.ReadySeed
			var err error
			if runtime.GOOS == "darwin" {
				prepared, err = macnotifications.Prepare(preparationCtx, visit.slot)
			} else {
				var store *fleetclient.NotificationStore
				store, err = fleetclient.DefaultNotificationStore()
				if err == nil {
					var device attention.DeviceSnapshot
					device, err = store.Read(preparationCtx)
					if err == nil {
						prepared = device.ReadySeed(visit.slot)
					}
				}
			}
			visit.mu.Lock()
			defer visit.mu.Unlock()
			if err != nil {
				if ctx.Err() == nil {
					visit.err = attention.ErrUnavailable
				}
			} else {
				visit.seed = prepared
			}
		})
	}
	visit.workers.Go(func() { visit.run(ctx, nonce) })
	return visit
}

func copySeed(seed attention.ReadySeed) attention.ReadySeed {
	if seed.Candidate != nil {
		candidate := *seed.Candidate
		if candidate.Foreground != nil {
			foreground := *candidate.Foreground
			candidate.Foreground = &foreground
		}
		seed.Candidate = &candidate
	}
	return seed
}

func (visit *entryVisit) failed(err error) {
	if err == nil {
		return
	}
	visit.mu.Lock()
	visit.err = attention.ErrUnavailable
	visit.mu.Unlock()
}

func (visit *entryVisit) greet(hello terminal.HelloFrame) { visit.hello <- hello }

// freeze runs immediately before the first nonempty payload's first write.
func (visit *entryVisit) freeze(hello terminal.HelloFrame) attention.ReadySeed {
	visit.mu.Lock()
	seed := copySeed(visit.seed)
	if !attention.EpochValid(seed.ResetID) {
		// Later preparation cannot repair this presentation's missed acknowledgement.
		visit.err = attention.ErrUnavailable
	}
	visit.mu.Unlock()
	key, foreground := visit.identity(hello)
	candidate := seed.Candidate
	if candidate != nil && (foreground == nil || candidate.Key != key || !attention.SameForeground(candidate.Foreground, foreground)) {
		seed.Candidate = nil
	}
	return seed
}

func (visit *entryVisit) identity(hello terminal.HelloFrame) (attention.Key, *attention.Foreground) {
	key := attention.Key{Machine: visit.slot.Machine, TmuxID: visit.slot.TmuxID, IdentityToken: visit.slot.IdentityToken, PaneID: hello.PaneID}
	var foreground *attention.Foreground
	if hello.Foreground != nil {
		foreground = &attention.Foreground{Provider: hello.Foreground.Provider, PID: hello.Foreground.PID, StartIdentity: hello.Foreground.StartIdentity}
	}
	return key, foreground
}

func (visit *entryVisit) run(ctx context.Context, nonce string) {
	var native *macnotifications.Visit
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if native != nil {
			visit.failed(native.End(cleanupCtx))
			visit.failed(native.Close())
		}
		visit.failed(abandonLaunch(cleanupCtx, nonce))
	}()
	var hello terminal.HelloFrame
	select {
	case hello = <-visit.hello:
	case <-ctx.Done():
		return
	}
	key, foreground := visit.identity(hello)
	var store *fleetclient.NotificationStore
	var err error
	if runtime.GOOS == "darwin" {
		native, err = macnotifications.Register(ctx, macnotifications.RegisterArgs{Key: key, Foreground: foreground, LaunchNonce: nonce})
	} else {
		store, err = fleetclient.DefaultNotificationStore()
	}
	if err != nil {
		if ctx.Err() == nil {
			visit.failed(err)
		}
		return
	}
	var frozen attention.ReadySeed
	outputWritten, ackAttempted := false, false
	var retry <-chan time.Time
	var retryTimer *time.Timer
	defer func() {
		if retryTimer != nil {
			retryTimer.Stop()
		}
	}()
	delay := time.Second
	for {
		if outputWritten && !ackAttempted {
			ackAttempted = true // An uncertain attempt is never replayed after reconnect.
			token := attention.ReadyToken{}
			if frozen.Candidate != nil {
				token = frozen.Candidate.Token
			}
			if native != nil {
				visit.failed(native.Presented(ctx, frozen.ResetID, token))
			} else if frozen.ResetID != "" {
				acknowledgementCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				_, err := store.Presented(acknowledgementCtx, frozen.ResetID, key, foreground, token)
				cancel()
				visit.failed(err)
			}
		}
		var disconnected <-chan struct{}
		if native != nil && retry == nil {
			disconnected = native.Disconnected()
		}
		select {
		case <-ctx.Done():
			return
		case frozen = <-visit.written:
			outputWritten = true
		case <-disconnected:
			visit.failed(attention.ErrUnavailable)
			retryTimer = time.NewTimer(delay)
			retry = retryTimer.C
		case <-retry:
			// Apply output already queued before reconstructing the visit.
			select {
			case frozen = <-visit.written:
				outputWritten = true
				retryTimer = time.NewTimer(0)
				retry = retryTimer.C
				continue
			default:
			}
			retry = nil
			if native.Reconnect(ctx) != nil {
				delay = min(2*delay, 30*time.Second)
				retryTimer = time.NewTimer(delay)
				retry = retryTimer.C
			} else {
				delay = time.Second
			}

		}
	}
}

func (visit *entryVisit) close() error {
	visit.cancel()
	visit.workers.Wait()
	visit.mu.Lock()
	defer visit.mu.Unlock()
	// The pump has finished; any remaining presentation was never attempted.
	select {
	case <-visit.written:
		visit.err = attention.ErrUnavailable
	default:
	}
	return visit.err
}

func abandonLaunch(ctx context.Context, nonce string) error {
	if runtime.GOOS != "darwin" || nonce == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return macnotifications.Abandon(ctx, nonce)
}
