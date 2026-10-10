// Package notifier owns the single background readiness observer.
package notifier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

type Config struct {
	Client    *fleetclient.Client
	Machine   machine.Handle
	Bearer    auth.FileVerifier
	StatePath string
	Publisher *Publisher
}
type observerFile struct {
	Schema              int                     `json:"schema"`
	Epoch               string                  `json:"epoch"`
	Revision            int64                   `json:"revision"`
	ReceiverRevision    int64                   `json:"receiverRevision"`
	Records             []attention.Record      `json:"records"`
	AndroidSubscription *attention.Subscription `json:"androidSubscription,omitempty"`
}

func (file *observerFile) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if attention.Decode(data, &fields) != nil {
		return attention.ErrUnavailable
	}
	for _, name := range []string{"schema", "epoch", "revision", "receiverRevision", "records"} {
		if fields[name] == nil {
			return attention.ErrUnavailable
		}
	}
	type wire observerFile
	var value wire
	if attention.Decode(data, &value) != nil {
		return attention.ErrUnavailable
	}
	*file = observerFile(value)
	return nil
}

type Observer struct {
	mutex       sync.Mutex
	client      *fleetclient.Client
	machine     machine.Handle
	bearer      auth.FileVerifier
	origin      string
	path        string
	lock        *os.File
	file        observerFile
	snapshot    attention.Snapshot
	unavailable bool
	changed     chan struct{}
	subscribers map[chan struct{}]struct{}
	publisher   *Publisher
}

func New(config Config) (*Observer, error) {
	notifications, configured := config.Client.NotificationConfig()
	if !configured || notifications.ObserverMachine != config.Machine.String() || !filepath.IsAbs(config.StatePath) {
		return nil, attention.ErrUnavailable
	}
	if _, err := config.Bearer.Read(); err != nil {
		return nil, attention.ErrUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(config.StatePath), 0700); err != nil {
		return nil, attention.ErrUnavailable
	}
	lock, err := os.OpenFile(config.StatePath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, attention.ErrUnavailable
	} // justify-ignore-error: lock admission failure is primary.
	observer := &Observer{client: config.Client, machine: config.Machine, bearer: config.Bearer, origin: notifications.NtfyOrigin, path: config.StatePath, lock: lock, changed: make(chan struct{}, 1), subscribers: map[chan struct{}]struct{}{}, publisher: config.Publisher}
	stored := observerFile{Schema: 1, Records: []attention.Record{}}
	data, err := readPrivate(config.StatePath, attention.MaximumSnapshotBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, observer.closeFailure()
	}
	if err == nil {
		if len(data) > attention.MaximumSnapshotBytes || attention.Decode(data, &stored) != nil || stored.Schema != 1 || !attention.EpochValid(stored.Epoch) || stored.Revision < 0 || stored.ReceiverRevision < 0 || stored.ReceiverRevision > stored.Revision || stored.Records == nil || stored.AndroidSubscription != nil && !stored.AndroidSubscription.Valid(observer.origin) {
			return nil, observer.closeFailure()
		}
		// Reuse snapshot admission to validate the same retained record constraints.
		slots := map[attention.Slot]bool{}
		for _, record := range stored.Records {
			if !attention.ValidRecord(record, stored.Revision) || slots[record.Key.Slot()] {
				return nil, observer.closeFailure()
			}
			slots[record.Key.Slot()] = true
		}
	} else {
		random := [16]byte{}
		if _, err := rand.Read(random[:]); err != nil {
			return nil, observer.closeFailure()
		}
		stored.Epoch = hex.EncodeToString(random[:])
	}
	if stored.Revision >= math.MaxInt64-1 {
		return nil, observer.closeFailure()
	}
	stored.Revision++
	observer.file = stored
	observer.snapshot = attention.Snapshot{Schema: 1, Epoch: stored.Epoch, Revision: stored.Revision, ReceiverTag: stored.Epoch + ":" + strconv.FormatInt(stored.ReceiverRevision, 10), Machines: []attention.MachineSnapshot{}}
	for _, host := range config.Client.Machines() {
		observer.snapshot.Machines = append(observer.snapshot.Machines, attention.MachineSnapshot{Machine: host.Handle, Availability: "gap"})
	}
	encoded, err := json.Marshal(stored)
	if err != nil || attention.WriteFile(config.StatePath, encoded) != nil {
		return nil, observer.closeFailure()
	}
	return observer, nil
}
func (observer *Observer) closeFailure() error {
	if observer.lock.Close() != nil {
		return attention.ErrUnavailable
	}
	return attention.ErrUnavailable
}
func (observer *Observer) Close() error { return observer.lock.Close() }

func (observer *Observer) commit(host string, peer fleetclient.Peer, err error) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable || observer.file.Revision >= math.MaxInt64-1 {
		observer.unavailable = true
		return attention.ErrUnavailable
	}
	file := observer.file
	file.Revision++
	after := attention.MachineSnapshot{Machine: host, Availability: "gap"}
	if err == nil {
		after.Availability = "fresh"
		after.ObservedAt = peer.ObservedAt
		after.Sessions = []attention.SessionSnapshot{}
		records := make([]attention.Record, 0, len(file.Records)+len(peer.Sessions))
		for _, record := range file.Records {
			if record.Key.Machine != host {
				records = append(records, record)
			}
		}
		for _, session := range peer.Sessions {
			ref, decodeErr := fleetclient.DecodeReference(session.Ref)
			if decodeErr != nil {
				panic("unadmitted inventory reference")
			} // justify-defect: fleetclient admitted every inventory reference.
			previous := attention.Record{}
			present := false
			for _, record := range file.Records {
				if record.Key.Slot() == fleetclient.NotificationKey(ref).Slot() {
					previous = record
					present = true
					break
				}
			}
			row := observe(previous, present, session, file.Revision)
			records = append(records, row.Record)
			after.Sessions = append(after.Sessions, row)
		}
		file.Records = records
	}
	data, encodeErr := json.Marshal(file)
	if encodeErr != nil || len(data) > attention.MaximumSnapshotBytes || attention.WriteFile(observer.path, data) != nil {
		observer.unavailable = true
		return attention.ErrUnavailable
	}
	before := attention.MachineSnapshot{}
	for i, current := range observer.snapshot.Machines {
		if current.Machine == host {
			before = current
			observer.snapshot.Machines[i] = after
			break
		}
	}
	observer.file = file
	observer.snapshot.Revision = file.Revision
	if meaningful(before, after) {
		observer.signal()
	}
	return nil
}
func (observer *Observer) signal() {
	select {
	case observer.changed <- struct{}{}:
	default:
	}
	for subscriber := range observer.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// Run owns independent five-second lanes and one coalesced current-hint publisher.
func (observer *Observer) Run(ctx context.Context) error {
	ctx, stop := context.WithCancel(ctx)
	// justify-polling: closed-client background attention needs bounded five-second inventories; cancellation owns every lane.
	outcomes := make(chan struct {
		host string
		peer fleetclient.Peer
		err  error
	}, len(observer.snapshot.Machines))
	var lanes sync.WaitGroup
	for _, host := range observer.client.Machines() {
		lanes.Go(func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				request, cancel := context.WithTimeout(ctx, 5*time.Second)
				peer, err := observer.client.InventoryMachine(request, host.Handle)
				cancel()
				select {
				case outcomes <- struct {
					host string
					peer fleetclient.Peer
					err  error
				}{host.Handle, peer, err}:
				case <-ctx.Done():
					return
				}
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	publisherDone := make(chan struct{})
	go func() { defer close(publisherDone); observer.publish(ctx) }()
	defer func() { stop(); lanes.Wait(); <-publisherDone }()
	for {
		select {
		case outcome := <-outcomes:
			if err := observer.commit(outcome.host, outcome.peer, outcome.err); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
func (observer *Observer) publish(ctx context.Context) {
	pulse := time.NewTicker(time.Minute)
	defer pulse.Stop()
	for {
		select {
		case <-observer.changed:
		case <-pulse.C:
			observer.mutex.Lock()
			for subscriber := range observer.subscribers {
				select {
				case subscriber <- struct{}{}:
				default:
				}
			}
			observer.mutex.Unlock()
		case <-ctx.Done():
			return
		}
		observer.mutex.Lock()
		hint := attention.Hint{Schema: 1, Epoch: observer.file.Epoch, Revision: observer.file.Revision}
		subscription := observer.file.AndroidSubscription
		unavailable := observer.unavailable
		observer.mutex.Unlock()
		if unavailable || subscription == nil || observer.publisher == nil {
			continue
		}
		request, cancel := context.WithTimeout(ctx, 3*time.Second)
		_ = observer.publisher.Publish(request, *subscription, hint) // justify-ignore-error: one hint has no history/retry; next minute retries current state and retains enrollment.
		cancel()
	}
}

func readPrivate(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, attention.ErrUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(data)) > limit {
		return nil, attention.ErrUnavailable
	}
	return data, nil
}
