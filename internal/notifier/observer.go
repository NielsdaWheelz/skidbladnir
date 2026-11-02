// Package notifier is the hub's attention owner. It observes every machine's
// inventory, keeps one record per session lifetime with the owner's
// acknowledgement, projects the records into notice verdicts and hints clients
// when that projection changes.
package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type Config struct {
	FleetID   string
	Client    *fleetclient.Client
	StatePath string
	Fleet     *fleet.Service
	Sender    *fcmpush.Client
	Push      *hostconfig.AndroidPush
	Members   []fleet.Member
	Logger    *slog.Logger
}

// observerFile is the hub's only notification state; its name is its version.
type observerFile struct {
	FleetID       string     `json:"fleetId"`
	Configuration string     `json:"configuration,omitempty"` // push options id; "" = push off
	Revision      int64      `json:"revision"`                // allocation counter, never served
	Records       []record   `json:"records"`
	Receivers     []receiver `json:"receivers"`
}

type Observer struct {
	mutex         sync.Mutex
	client        *fleetclient.Client
	fleet         *fleet.Service // learns the member kind; the gateway already admitted the request
	fleetID       string
	path          string
	logger        *slog.Logger
	file          observerFile    // the committed file
	hosts         []host          // roster order
	served        attention.State // the last published projection
	failure       *storeFailure
	failureWake   chan struct{} // asks Run to cancel inventory lanes
	members       []fleet.Member
	configuration string          // desired push options id
	sender        *fcmpush.Client // desired sender; nil: push off or no credential
	urgent        bool            // an episode was allocated since the last fcm send
	send          chan struct{}   // wakes the fcm sender
	streams       map[chan struct{}]struct{}
}

type storeFailure struct {
	phase, operation string
	errno            syscall.Errno
}

type host struct {
	machine  string
	fresh    bool
	sessions []seen // the last fresh inventory, in inventory order
}

type seen struct {
	slot, ref, name string
	status          api.TerminalStatus
	qualified       bool
}

// New resumes this fleet's file, or starts a new one when it is missing or
// another fleet's: fleet deletes it before a new fleet commits, but the exiting
// process can write it once more. Run retries failed admission.
func New(config Config) *Observer {
	observer := &Observer{
		client: config.Client, fleet: config.Fleet, fleetID: config.FleetID,
		path: config.StatePath, logger: config.Logger, members: slices.Clone(config.Members),
		send: make(chan struct{}, 1), failureWake: make(chan struct{}, 1),
		streams: map[chan struct{}]struct{}{},
	}
	if config.Push != nil {
		observer.configuration, observer.sender = config.Push.ID(), config.Sender
	}
	observer.mutex.Lock()
	observer.recoverStore(config.Client.Machines())
	observer.mutex.Unlock()
	return observer
}

// recoverStore rereads durable state because a failed save may have renamed it.
// The caller holds the mutex; only this path clears a storage failure.
func (observer *Observer) recoverStore(machines []fleetclient.Machine) {
	file := observerFile{FleetID: observer.fleetID}
	data, err := privatefile.Read(observer.path, 1<<20)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		observer.fail("read", err)
		return
	}
	if err == nil {
		if err := strictjson.Decode(data, &file); err != nil {
			observer.fail("decode", err)
			return
		}
		if slices.ContainsFunc(file.Records, func(stored record) bool { return !stored.valid() }) {
			observer.fail("admit", errors.New("notification observer file holds an unknown record"))
			return
		}
	}
	if file.FleetID != observer.fleetID {
		file = observerFile{FleetID: observer.fleetID}
	}
	file, _ = observer.reconcileConfiguration(file)
	if observer.save(file) != nil {
		return
	}
	observer.hosts = nil
	observer.membership(machines)
	if observer.failure != nil {
		observer.failure = nil
		observer.logger.Info("Notifications.Recovered")
	}
	wake(observer.send)
}

// reconcileConfiguration applies current desired settings to admitted state.
// The caller holds the mutex; phones clones before any destination is cleared.
func (observer *Observer) reconcileConfiguration(file observerFile) (observerFile, bool) {
	receivers := phones(file.Receivers, observer.members)
	changed := len(receivers) != len(file.Receivers) || file.Configuration != observer.configuration
	if file.Configuration != observer.configuration {
		file.Configuration = observer.configuration
		for index := range receivers {
			receivers[index].FID, receivers[index].Failed = "", false
		}
	}
	file.Receivers = receivers
	return file, changed
}

// fail withdraws availability and records content-free evidence. The caller
// holds the mutex; repeated classified failures stay silent.
func (observer *Observer) fail(phase string, err error) {
	failure := storeFailure{phase: phase}
	// privatefile joins the primary error before close and cleanup failures.
	for {
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			break
		}
		err = joined.Unwrap()[0]
	}
	var pathError *os.PathError
	var linkError *os.LinkError
	switch {
	case errors.As(err, &pathError):
		failure.operation, err = pathError.Op, pathError.Err
	case errors.As(err, &linkError):
		failure.operation, err = linkError.Op, linkError.Err
	}
	errors.As(err, &failure.errno)
	previous := observer.failure
	observer.failure = &failure
	observer.urgent = false
	for stream := range observer.streams {
		close(stream)
		delete(observer.streams, stream)
	}
	if previous == nil {
		wake(observer.failureWake)
	}
	if previous == nil || *previous != failure {
		attributes := []any{"skidbladnir.notifications.phase", failure.phase}
		if failure.operation != "" {
			attributes = append(attributes, "skidbladnir.notifications.operation", failure.operation)
		}
		if failure.errno != 0 {
			attributes = append(attributes, "skidbladnir.notifications.errno", int(failure.errno))
		}
		observer.logger.Info("Notifications.Unavailable", attributes...)
	}
}

// save commits file, or withdraws availability. The caller holds the mutex.
func (observer *Observer) save(file observerFile) error {
	data, _ := json.Marshal(file) // plain fields always encode
	if err := privatefile.WriteFile(observer.path, data); err != nil {
		observer.fail("write", err)
		return err
	}
	observer.file = file
	return nil
}

// commit applies one machine's inventory read. A failed read is a gap: its
// records and notices stay as they are. The caller holds the mutex and admits
// only the current lane of a healthy observer.
func (observer *Observer) commit(machine string, peer fleetclient.Peer, err error) {
	index := slices.IndexFunc(observer.hosts, func(current host) bool { return current.machine == machine })
	if index < 0 {
		return
	}
	if err != nil {
		observer.hosts[index] = host{machine: machine}
		observer.publish(false)
		return
	}
	file := observer.file
	file.Revision++
	before := map[string]record{}
	records := make([]record, 0, len(file.Records)+len(peer.Sessions))
	for _, stored := range file.Records {
		if stored.Key.Machine == machine {
			before[slotID(stored.Key)] = stored
		} else {
			records = append(records, stored)
		}
	}
	changed := len(before) != len(peer.Sessions)
	allocated := false
	observed := make([]seen, 0, len(peer.Sessions))
	for _, session := range peer.Sessions {
		ref := session.Ref.Reference
		slot := slotID(ref)
		previous, present := before[slot]
		next, qualified := observe(previous, present, ref, session, file.Revision)
		changed = changed || !present || !reflect.DeepEqual(previous, next)
		allocated = allocated || next.Cause != nil && next.AttentionEpisode == file.Revision
		records = append(records, next)
		observed = append(observed, seen{slot, session.Ref.Encode(), session.Name, session.TerminalStatus, qualified})
	}
	file.Records = records
	// An unchanged inventory needs no write: every value a record or client
	// holds was saved with a revision at least as high, so a restart still
	// allocates above all of them.
	if !changed {
		observer.file = file
	} else if err := observer.save(file); err != nil {
		return
	}
	observer.hosts[index] = host{machine: machine, fresh: true, sessions: observed}
	observer.publish(allocated)
}

// membership follows the roster. Records of removed machines stay, so a
// re-enrolled computer resumes its acknowledgements. The caller holds the mutex.
func (observer *Observer) membership(machines []fleetclient.Machine) {
	hosts := make([]host, 0, len(machines))
	for _, machine := range machines {
		next := host{machine: machine.Handle}
		if index := slices.IndexFunc(observer.hosts, func(current host) bool { return current.machine == machine.Handle }); index >= 0 {
			next = observer.hosts[index]
		}
		hosts = append(hosts, next)
	}
	observer.hosts = hosts
	observer.publish(false)
}

// publish recomputes the projection and wakes the fcm sender and every stream
// when it changed. The caller holds the mutex.
func (observer *Observer) publish(allocated bool) {
	observer.urgent = observer.urgent || allocated
	records := make(map[string]record, len(observer.file.Records))
	for _, stored := range observer.file.Records {
		records[slotID(stored.Key)] = stored
	}
	next := attention.State{Machines: []attention.Machine{}}
	for _, current := range observer.hosts {
		if !current.fresh {
			next.Machines = append(next.Machines, attention.Machine{Machine: current.machine, Availability: attention.Gap})
			continue
		}
		machine := attention.Machine{Machine: current.machine, Availability: attention.Fresh, Sessions: []attention.Session{}}
		for _, session := range current.sessions {
			machine.Sessions = append(machine.Sessions, project(records[session.slot], session))
		}
		next.Machines = append(next.Machines, machine)
	}
	if reflect.DeepEqual(next, observer.served) {
		return
	}
	observer.served = next
	wake(observer.send)
	for stream := range observer.streams {
		wake(stream)
	}
}

func wake(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

// Run reads every machine's inventory every five seconds, one lane per machine
// so a slow machine never delays another, reconciles the roster on the same
// cadence, and owns the fcm sender. Storage failure pauses inventory until a
// later tick recovers the file. Cancellation stops and joins every worker.
func (observer *Observer) Run(parent context.Context) {
	ctx, stop := context.WithCancel(parent)
	type outcome struct {
		machine string
		lane    uint64
		peer    fleetclient.Peer
		err     error
	}
	outcomes := make(chan outcome)
	type lane struct {
		id     uint64
		cancel context.CancelFunc
	}
	lanes := map[string]lane{}
	var workers sync.WaitGroup
	workers.Go(func() { observer.sendHints(ctx) })
	defer func() { stop(); workers.Wait() }()
	var next uint64
	suspend := func() {
		for machine, current := range lanes {
			current.cancel()
			delete(lanes, machine)
		}
	}
	reconcile := func(retry bool) {
		observer.mutex.Lock()
		defer observer.mutex.Unlock()
		machines := observer.client.Machines()
		if ctx.Err() != nil {
			return
		}
		if observer.failure != nil {
			suspend()
			if !retry {
				return
			}
			observer.recoverStore(machines)
			if observer.failure != nil || ctx.Err() != nil {
				return
			}
		} else {
			observer.membership(machines)
		}
		for machine, current := range lanes {
			if !slices.ContainsFunc(machines, func(candidate fleetclient.Machine) bool { return candidate.Handle == machine }) {
				current.cancel()
				delete(lanes, machine)
			}
		}
		for _, machine := range machines {
			if _, found := lanes[machine.Handle]; found {
				continue
			}
			next++
			id := next
			laneCtx, cancel := context.WithCancel(ctx)
			lanes[machine.Handle] = lane{id, cancel}
			workers.Go(func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					request, cancel := context.WithTimeout(laneCtx, 5*time.Second)
					result := observer.client.InventoryMachine(request, machine.Handle)
					cancel()
					value := outcome{machine: machine.Handle, lane: id, err: attention.ErrUnavailable}
					if result.OK {
						value.peer, value.err = result.Value.(fleetclient.Peer), nil
					}
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
	}
	reconcile(false)
	membership := time.NewTicker(5 * time.Second)
	defer membership.Stop()
	for {
		select {
		case result := <-outcomes:
			observer.mutex.Lock()
			if current, found := lanes[result.machine]; found && current.id == result.lane && observer.failure == nil && ctx.Err() == nil {
				observer.commit(result.machine, result.peer, result.err)
			}
			if observer.failure != nil {
				suspend()
			}
			observer.mutex.Unlock()
		case <-membership.C:
			reconcile(true)
		case <-observer.failureWake:
			observer.mutex.Lock()
			if observer.failure != nil {
				suspend()
			}
			observer.mutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
