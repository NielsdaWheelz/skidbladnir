// Package notifier owns the active hub's background readiness observer.
package notifier

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

type Config struct {
	FleetID   string
	Client    *fleetclient.Client
	Machine   machine.Handle
	StatePath string
	Fleet     *fleet.Service
	Sender    *fcmpush.Client
	Push      *hostconfig.AndroidPush
	Members   []fleet.Member
}
type observerFile struct {
	Configuration string             `json:"configuration,omitempty"`
	FleetID       string             `json:"fleetId"`
	ImportDigest  string             `json:"importDigest,omitempty"`
	Schema        int                `json:"schema"`
	Epoch         string             `json:"epoch"`
	Revision      int64              `json:"revision"`
	Records       []attention.Record `json:"records"`
	Receivers     []receiver         `json:"receivers"`
}

func (file *observerFile) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if attention.Decode(data, &fields) != nil {
		return attention.ErrUnavailable
	}
	for _, name := range []string{"schema", "fleetId", "epoch", "revision", "records", "receivers"} {
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
func decodeFile(data []byte) (observerFile, error) {
	var stored observerFile
	if len(data) > attention.MaximumSnapshotBytes || attention.Decode(data, &stored) != nil || stored.Schema != 3 || stored.Configuration != "" && !digestValid(stored.Configuration) || !digestValid(stored.FleetID) || stored.ImportDigest != "" && !digestValid(stored.ImportDigest) || !attention.EpochValid(stored.Epoch) || stored.Revision < 0 || stored.Records == nil || stored.Receivers == nil {
		return stored, attention.ErrUnavailable
	}
	slots := map[attention.Slot]bool{}
	for _, record := range stored.Records {
		if !attention.ValidRecord(record, stored.Revision) || slots[record.Key.Slot()] {
			return stored, attention.ErrUnavailable
		}
		slots[record.Key.Slot()] = true
	}
	devices := map[string]bool{}
	for _, receiver := range stored.Receivers {
		if !receiver.valid() || devices[receiver.DeviceID] || receiver.Configuration != stored.Configuration {
			return stored, attention.ErrUnavailable
		}
		devices[receiver.DeviceID] = true
	}
	return stored, nil
}

type Observer struct {
	mutex            sync.Mutex
	client           *fleetclient.Client
	machine          machine.Handle
	fleet            *fleet.Service
	sender           *fcmpush.Client
	push             *hostconfig.AndroidPush
	sendCancel       context.CancelFunc
	path             string
	file             observerFile
	snapshot         attention.Snapshot
	unavailable      bool
	drained          bool
	cancel           context.CancelFunc
	done             chan struct{}
	changed          chan struct{}
	attentionPending bool
	streams          sync.WaitGroup
	subscribers      map[chan struct{}]struct{}
}

// New never calls fleet methods. Activation callbacks hold the fleet owner lock.
// Push setup is explicit and local; no sender credential enters durable state.
func New(config Config) (*Observer, error) {
	if !digestValid(config.FleetID) || config.Client == nil || config.Fleet == nil || !filepath.IsAbs(config.StatePath) || config.Machine.String() == "" {
		return nil, attention.ErrUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(config.StatePath), 0700); err != nil {
		return nil, attention.ErrUnavailable
	}
	// One gateway process owns this state directory through fleet's authority
	// lock, and composition serializes construction against drain and Restore.
	observer := &Observer{client: config.Client, machine: config.Machine, fleet: config.Fleet, sender: config.Sender, path: config.StatePath, changed: make(chan struct{}, 1), subscribers: map[chan struct{}]struct{}{}}
	stored := observerFile{Schema: 3, FleetID: config.FleetID, Records: []attention.Record{}, Receivers: []receiver{}}
	data, err := privatefile.Read(config.StatePath, attention.MaximumSnapshotBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, attention.ErrUnavailable
	}
	if err == nil {
		stored, err = decodeFile(data)
		if err != nil || stored.FleetID != config.FleetID {
			return nil, attention.ErrUnavailable
		}
	} else {
		random := [16]byte{}
		if _, err = rand.Read(random[:]); err != nil {
			return nil, attention.ErrUnavailable
		}
		stored.Epoch = hex.EncodeToString(random[:])
	}
	// Membership is captured by composition outside fleet callbacks.
	stored, _ = retireAbsent(stored, config.Members)
	stored.Revision++
	observer.file = stored
	observer.snapshot = attention.Snapshot{Schema: 1, Epoch: stored.Epoch, Revision: stored.Revision, Machines: []attention.MachineSnapshot{}}
	for _, host := range config.Client.Machines() {
		observer.snapshot.Machines = append(observer.snapshot.Machines, attention.MachineSnapshot{Machine: host.Handle, Availability: "gap"})
	}
	if observer.save(stored) != nil {
		return nil, attention.ErrUnavailable
	}
	if err := observer.ConfigurePush(config.Push, config.Sender); err != nil {
		return nil, attention.ErrUnavailable
	}
	return observer, nil
}
func (observer *Observer) save(file observerFile) error {
	encoded, err := json.Marshal(file)
	if err != nil || len(encoded) > attention.MaximumSnapshotBytes || attention.WriteFile(observer.path, encoded) != nil {
		observer.unavailable = true
		return attention.ErrUnavailable
	}
	observer.file = file
	return nil
}

// Drain closes writer admission before cancelling and joining every owned lane.
// It performs no fleet call and is safe under the fleet mutation lock.
func (observer *Observer) Drain() (map[string]json.RawMessage, error) {
	observer.mutex.Lock()
	observer.drained = true
	if observer.sendCancel != nil {
		observer.sendCancel()
	}
	if observer.cancel != nil {
		observer.cancel()
	}
	done := observer.done
	observer.signal(false)
	observer.mutex.Unlock()
	if done != nil {
		<-done
	}
	observer.streams.Wait()
	return observer.Export()
}
func (observer *Observer) Export() (map[string]json.RawMessage, error) {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if !observer.drained || observer.unavailable {
		return nil, attention.ErrUnavailable
	}
	data, err := json.Marshal(observer.file)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	return map[string]json.RawMessage{"notifications": data}, nil
}

// Restore installs transferred evidence, never freshness. New advances revision
// and starts every machine at gap. The caller must first drain the prior owner.
func digestValid(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
func Restore(path string, owned map[string]json.RawMessage) error {
	data, found := owned["notifications"]
	if !found || !filepath.IsAbs(path) {
		return attention.ErrUnavailable
	}
	imported, err := decodeFile(data)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	current, err := privatefile.Read(path, attention.MaximumSnapshotBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return attention.ErrUnavailable
	}
	if err == nil {
		existing, decodeErr := decodeFile(current)
		if decodeErr != nil {
			return decodeErr
		}
		// A retried activation must not reset writes made after its first import.
		if existing.ImportDigest == digest && existing.FleetID == imported.FleetID {
			// An earlier import may have renamed successfully but failed its
			// directory fsync. Confirm durability without overwriting later writes.
			directory, err := os.Open(filepath.Dir(path))
			if err != nil {
				return attention.ErrUnavailable
			}
			if err = errors.Join(directory.Sync(), directory.Close()); err != nil {
				return attention.ErrUnavailable
			}
			return nil
		}
		if existing.FleetID == imported.FleetID && (existing.Epoch != imported.Epoch || existing.Revision >= imported.Revision) {
			return attention.ErrUnavailable
		}
	}
	imported.ImportDigest = digest
	encoded, err := json.Marshal(imported)
	if err != nil {
		return attention.ErrUnavailable
	}
	return attention.WriteFile(path, encoded)
}
func (observer *Observer) commit(host string, peer fleetclient.Peer, err error) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained {
		return nil
	}
	if observer.unavailable {
		return attention.ErrUnavailable
	}
	index := slices.IndexFunc(observer.snapshot.Machines, func(current attention.MachineSnapshot) bool { return current.Machine == host })
	if index < 0 {
		return nil
	}
	file := observer.file
	file.Revision++
	after := attention.MachineSnapshot{Machine: host, Availability: "gap"}
	attentionAllocated := false
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
			attentionAllocated = attentionAllocated || row.Record.AttentionEpisode == file.Revision && row.Record.Cause != nil
			records = append(records, row.Record)
			after.Sessions = append(after.Sessions, row)
		}
		file.Records = records
	}
	if observer.save(file) != nil {
		return attention.ErrUnavailable
	}
	before := observer.snapshot.Machines[index]
	observer.snapshot.Machines[index] = after
	observer.snapshot.Revision = file.Revision
	if meaningful(before, after) {
		observer.signal(attentionAllocated)
	}
	return nil
}
func (observer *Observer) signal(attentionUrgency bool) {
	observer.attentionPending = observer.attentionPending || attentionUrgency
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
func (observer *Observer) membership(hosts []fleetclient.Machine) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.drained {
		return nil
	}
	machines := make([]attention.MachineSnapshot, 0, len(hosts))
	for _, host := range hosts {
		current := attention.MachineSnapshot{Machine: host.Handle, Availability: "gap"}
		for _, old := range observer.snapshot.Machines {
			if old.Machine == host.Handle {
				current = old
				break
			}
		}
		machines = append(machines, current)
	}
	if slices.EqualFunc(observer.snapshot.Machines, machines, func(a, b attention.MachineSnapshot) bool { return a.Machine == b.Machine }) {
		return nil
	}
	file := observer.file
	file.Revision++
	// Removed hosts leave the public roster. Retain exact lifetime counters:
	// a device may miss removal and the same computer can re-enroll unchanged.
	if observer.save(file) != nil {
		return attention.ErrUnavailable
	}
	observer.snapshot.Machines = machines
	observer.snapshot.Revision = file.Revision
	observer.signal(false)
	return nil
}

// Run owns independent inventory lanes; the existing five-second cadence also
// reconciles signed membership so additions and removals need no process restart.
func (observer *Observer) Run(parent context.Context) error {
	observer.mutex.Lock()
	if observer.drained || observer.done != nil {
		observer.mutex.Unlock()
		return attention.ErrUnavailable
	}
	ctx, stop := context.WithCancel(parent)
	observer.cancel = stop
	observer.done = make(chan struct{})
	done := observer.done
	observer.mutex.Unlock()
	type outcome struct {
		host string
		lane uint64
		peer fleetclient.Peer
		err  error
	}
	outcomes := make(chan outcome)
	type lane struct {
		id     uint64
		cancel context.CancelFunc
	}
	lanes := map[string]lane{}
	var workers sync.WaitGroup
	workers.Go(func() { observer.publish(ctx) })
	defer func() { stop(); workers.Wait(); close(done) }()
	var next uint64
	reconcile := func() error {
		if observer.client.Refresh() != nil {
			return nil
		} // Preserve admitted membership while its durable replacement is unavailable.
		hosts := observer.client.Machines()
		if err := observer.membership(hosts); err != nil {
			return err
		}
		for host, current := range lanes {
			if !slices.ContainsFunc(hosts, func(candidate fleetclient.Machine) bool { return candidate.Handle == host }) {
				current.cancel()
				delete(lanes, host)
			}
		}
		for _, host := range hosts {
			if _, found := lanes[host.Handle]; found {
				continue
			}
			next++
			current := next
			laneCtx, cancel := context.WithCancel(ctx)
			lanes[host.Handle] = lane{current, cancel}
			workers.Go(func() {
				// justify-polling: closed-client attention retains bounded five-second per-host inventories, cancelled on membership removal or drain.
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					request, cancel := context.WithTimeout(laneCtx, 5*time.Second)
					result := observer.client.InventoryMachine(request, host.Handle)
					cancel()
					value := outcome{host: host.Handle, lane: current, err: attention.ErrUnavailable}
					if result.OK {
						value.peer = result.Value.(fleetclient.Peer)
						value.err = nil
					} // justify-assertion: inventory result owns the admitted peer type.
					select {
					case outcomes <- value:
					case <-laneCtx.Done():
						return
					}
					select {
					case <-ticker.C:
					case <-laneCtx.Done():
						return
					}
				}
			})
		}
		return nil
	}
	if err := reconcile(); err != nil {
		return err
	}
	membership := time.NewTicker(5 * time.Second)
	defer membership.Stop()
	for {
		select {
		case result := <-outcomes:
			if current, found := lanes[result.host]; found && current.id == result.lane {
				if err := observer.commit(result.host, result.peer, result.err); err != nil {
					return err
				}
			}
		case <-membership.C:
			if err := reconcile(); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
