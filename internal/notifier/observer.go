// Package notifier is the hub's attention owner. It observes every machine's
// inventory, keeps one record per session lifetime with the owner's
// acknowledgement, projects the records into notice verdicts and hints clients
// when that projection changes.
package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fcmpush"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
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
	mutex       sync.Mutex
	client      *fleetclient.Client
	fleet       *fleet.Service // learns the member kind; the gateway already admitted the request
	path        string
	file        observerFile    // the committed file
	hosts       []host          // roster order
	served      attention.State // the last published projection
	unavailable bool            // a save failed: 503 until restart
	sender      *fcmpush.Client // nil: push off or no credential
	urgent      bool            // an episode was allocated since the last fcm send
	send        chan struct{}   // wakes the fcm sender
	streams     map[chan struct{}]struct{}
}

type host struct {
	machine  string
	fresh    bool
	sessions []seen // the last fresh inventory, in inventory order
}

type seen struct {
	slot, ref, name string
	status          sessions.TerminalStatus
	qualified       bool
}

// New resumes this fleet's file, or starts a new one when it is missing or
// another fleet's: fleet deletes it before a new fleet commits, but the exiting
// process can write it once more.
func New(config Config) (*Observer, error) {
	file := observerFile{FleetID: config.FleetID}
	data, err := privatefile.Read(config.StatePath, 1<<20)
	if err == nil {
		err = strictjson.Decode(data, &file)
	}
	if err == nil && slices.ContainsFunc(file.Records, func(stored record) bool { return !stored.valid() }) {
		err = errors.New("notification observer file holds an unknown record")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if file.FleetID != config.FleetID {
		file = observerFile{FleetID: config.FleetID}
	}
	file.Receivers = phones(file.Receivers, config.Members)
	observer := &Observer{client: config.Client, fleet: config.Fleet, path: config.StatePath, send: make(chan struct{}, 1), streams: map[chan struct{}]struct{}{}}
	for _, machine := range config.Client.Machines() {
		observer.hosts = append(observer.hosts, host{machine: machine.Handle})
	}
	if err := observer.save(file); err != nil {
		return nil, err
	}
	observer.publish(false)
	if err := observer.ConfigurePush(config.Push, config.Sender); err != nil {
		return nil, err
	}
	return observer, nil
}

// save commits file, or latches the observer unavailable.
func (observer *Observer) save(file observerFile) error {
	data, _ := json.Marshal(file) // plain fields always encode
	if err := privatefile.WriteFile(observer.path, data); err != nil {
		observer.unavailable = true
		return err
	}
	observer.file = file
	return nil
}

// commit applies one machine's inventory read. A failed read is a gap: its
// records and notices stay as they are.
func (observer *Observer) commit(machine string, peer fleetclient.Peer, err error) error {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		return attention.ErrUnavailable
	}
	index := slices.IndexFunc(observer.hosts, func(current host) bool { return current.machine == machine })
	if index < 0 {
		return nil
	}
	if err != nil {
		observer.hosts[index] = host{machine: machine}
		observer.publish(false)
		return nil
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
		ref, err := fleetclient.DecodeReference(session.Ref)
		if err != nil {
			panic("fleetclient encoded an invalid session reference")
		}
		slot := slotID(ref)
		previous, present := before[slot]
		next, qualified := observe(previous, present, ref, session, file.Revision)
		changed = changed || !present || !reflect.DeepEqual(previous, next)
		allocated = allocated || next.Cause != nil && next.AttentionEpisode == file.Revision
		records = append(records, next)
		observed = append(observed, seen{slot, session.Ref, session.Name, session.TerminalStatus, qualified})
	}
	file.Records = records
	// An unchanged inventory needs no write: every value a record or client
	// holds was saved with a revision at least as high, so a restart still
	// allocates above all of them.
	if !changed {
		observer.file = file
	} else if err := observer.save(file); err != nil {
		return err
	}
	observer.hosts[index] = host{machine: machine, fresh: true, sessions: observed}
	observer.publish(allocated)
	return nil
}

// membership follows the roster. Records of removed machines stay, so a
// re-enrolled computer resumes its acknowledgements.
func (observer *Observer) membership(machines []fleetclient.Machine) {
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
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
// cadence, and owns the fcm sender. It stops with ctx or the first failed save.
func (observer *Observer) Run(parent context.Context) error {
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
	reconcile := func() {
		machines := observer.client.Machines()
		observer.membership(machines)
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
	reconcile()
	membership := time.NewTicker(5 * time.Second)
	defer membership.Stop()
	for {
		select {
		case result := <-outcomes:
			if current, found := lanes[result.machine]; found && current.id == result.lane {
				if err := observer.commit(result.machine, result.peer, result.err); err != nil {
					return err
				}
			}
		case <-membership.C:
			reconcile()
		case <-ctx.Done():
			return nil
		}
	}
}
