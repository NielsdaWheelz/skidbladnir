package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/notifier"
)

// Fleet callbacks hold its mutation lock. This owner never calls fleet while
// holding mu; generation invalidates authority captured across drain or recovery.
type notificationHost struct {
	push           *hostconfig.AndroidPush
	sender         *fcmpush.Client
	credentialPath string
	mu             sync.Mutex
	observer       *notifier.Observer
	worker         *notificationWorker
	generation     uint64
	owned          map[string]json.RawMessage
	failure        error
	path           string
	changed        chan struct{}
	logger         *logging.Logger
}
type notificationWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error // published by closing done; completion is broadcast, never consumed.
}

func (host *notificationHost) signal() {
	select {
	case host.changed <- struct{}{}:
	default:
	}
}
func (host *notificationHost) drain() (map[string]json.RawMessage, error) {
	host.mu.Lock()
	defer host.mu.Unlock()
	host.generation++
	if host.observer == nil {
		return host.owned, nil
	}
	if host.worker != nil {
		host.worker.cancel()
		<-host.worker.done
	}
	owned, err := host.observer.Drain()
	if host.worker != nil {
		err = errors.Join(err, host.worker.err)
	}
	err = errors.Join(err, host.observer.Close())
	host.observer, host.worker = nil, nil
	host.owned = nil
	if err == nil {
		host.owned = owned
	}
	return owned, err
}
func (host *notificationHost) revoke(device string) error {
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.observer == nil {
		return nil
	} // New reconciles durable membership before Run.
	return host.observer.Revoke(device)
}
func (host *notificationHost) recover(previous, next string) error {
	host.mu.Lock()
	defer host.mu.Unlock()
	host.generation++
	if host.observer != nil {
		return errors.New("notification owner must be drained before recovery")
	}
	data, err := os.ReadFile(host.path)
	if errors.Is(err, os.ErrNotExist) {
		host.owned = nil
		directory, openErr := os.Open(filepath.Dir(host.path))
		if openErr != nil {
			return openErr
		}
		return errors.Join(directory.Sync(), directory.Close())
	}
	if err != nil {
		return err
	}
	var scope struct {
		FleetID string `json:"fleetId"`
	}
	if json.Unmarshal(data, &scope) != nil {
		return errors.New("notification recovery state invalid")
	}
	if scope.FleetID == next {
		return nil
	} // Completed callback replay after restart.
	digest, decodeErr := hex.DecodeString(scope.FleetID)
	if decodeErr != nil || len(digest) != 32 || hex.EncodeToString(digest) != scope.FleetID {
		return errors.New("notification recovery fleet invalid")
	}
	if previous != "" && scope.FleetID != previous {
		return errors.New("notification recovery fleet mismatch")
	}
	archive := host.path + ".recovered-" + scope.FleetID
	existing, readErr := os.ReadFile(archive)
	if readErr == nil && !bytes.Equal(existing, data) {
		return errors.New("notification recovery archive already differs")
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err = os.Rename(host.path, archive); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(host.path))
	if err != nil {
		return err
	}
	if err = errors.Join(directory.Sync(), directory.Close()); err != nil {
		return err
	}
	host.owned = nil
	host.failure = nil
	host.signal()
	return nil
}
func (host *notificationHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host.mu.Lock()
	observer := host.observer
	failure := host.failure
	host.mu.Unlock()
	if observer == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		message := "notification hub unavailable; check skid status and service logs"
		if failure != nil {
			message = "notification state unavailable; repair its state and restart the skid service"
		}
		_, _ = io.WriteString(w, "{\"code\":\"NotificationsUnavailable\",\"message\":\""+message+"\"}\n") // justify-ignore-error: transport failure cannot be repaired here.
		return
	}
	observer.ServeHTTP(w, r)
}
func (host *notificationHost) failed(err error) {
	host.mu.Lock()
	host.failure = err
	host.mu.Unlock()
	if host.logger != nil {
		_ = host.logger.Write(logging.NewNotificationsUnavailable())
	} // justify-ignore-error: secondary diagnostics must not interrupt local terminals.
}

// applyPush is independent of local shell resolution. Missing credentials affect
// Android delivery only; ConfigurePush preserves the observer's attention state.
func (host *notificationHost) applyPush(push *hostconfig.AndroidPush) {
	host.mu.Lock()
	if push != nil {
		value := *push
		push = &value
	}
	host.push = push
	host.sender = nil
	if push != nil {
		sender, err := fcmpush.Load(host.credentialPath, push.ProjectID)
		if err == nil {
			host.sender = sender
		}
	}
	var err error
	if host.observer != nil {
		err = host.observer.ConfigurePush(host.push, host.sender)
	}
	host.mu.Unlock()
	if err != nil {
		host.failed(err)
	}
	host.signal()
}
func (host *notificationHost) reconcile(ctx context.Context, control *fleet.Service, client *fleetclient.Client, handle machine.Handle) error {
	host.mu.Lock()
	generation := host.generation
	present := host.observer != nil
	host.mu.Unlock()
	active := control.IsHub()
	status := control.Status()
	if !active {
		if present {
			_, err := host.drain()
			return err
		}
		return nil // Non-hubs and fenced authorities own no notification writer.
	}
	host.mu.Lock()
	if generation != host.generation || host.observer != nil {
		host.mu.Unlock()
		return nil
	}
	host.owned = nil
	observer, err := notifier.New(notifier.Config{Client: client, Machine: handle, StatePath: host.path, Fleet: control, Push: host.push, Sender: host.sender, Members: status.Members, FleetID: status.FleetID})
	if err != nil {
		host.mu.Unlock()
		return err
	}
	host.observer = observer // Removal/drain callbacks can now fence construction.
	host.mu.Unlock()
	active = control.IsHub()
	current := control.Status()
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.observer != observer {
		return nil
	}
	if !active || current.FleetID != status.FleetID || current.AuthorityEpoch != status.AuthorityEpoch || generation != host.generation || ctx.Err() != nil {
		_, drainErr := observer.Drain()
		closeErr := observer.Close()
		host.observer = nil
		return errors.Join(drainErr, closeErr)
	}
	if err := observer.ReconcileMembers(current.Members); err != nil {
		_, drainErr := observer.Drain()
		closeErr := observer.Close()
		host.observer = nil
		return errors.Join(err, drainErr, closeErr)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	worker := &notificationWorker{cancel: cancel, done: make(chan struct{})}
	host.worker = worker
	host.failure = nil
	go func() { worker.err = observer.Run(workerCtx); close(worker.done) }()
	return nil
}
func (host *notificationHost) run(ctx context.Context, control *fleet.Service, client *fleetclient.Client, handle machine.Handle) (result error) {
	defer func() { _, err := host.drain(); result = errors.Join(result, err) }()
	for {
		if err := host.reconcile(ctx, control, client, handle); err != nil {
			host.failed(err)
		}
		host.mu.Lock()
		worker := host.worker
		var done <-chan struct{}
		if worker != nil {
			done = worker.done
		}
		host.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-host.changed:
		case <-done:
			host.mu.Lock()
			current := host.worker == worker
			host.mu.Unlock()
			if !current {
				continue
			} // A normal handoff/abort replaces the owner.
			_, drainErr := host.drain()
			err := errors.Join(worker.err, drainErr)
			if err == nil {
				err = errors.New("notification observer stopped unexpectedly")
			}
			host.failed(err)
			// Retry only after an explicit control/configuration change, never spin.
			select {
			case <-ctx.Done():
				return nil
			case <-host.changed:
			}
		}
	}
}
