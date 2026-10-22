package fleet

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

var errRestart = errors.New("fleet identity changed; restarting skid")

type Options struct {
	Directory, ConfigPath, HandlePath, ObserverPath string
	Name                                            string // hostname: bootstrap display name only
	Origin                                          string // tailscale origin discovered at start; "" when unavailable
	Home, LoginShell, Version                       string
}

// Service is the single writer of this installation's fleet.json. Every mutation
// edits a draft decoded from the committed bytes and swaps it in only after the
// write succeeds, so no request observes unpersisted state.
type Service struct {
	mu      sync.Mutex
	options Options
	machine machine.Handle  // process-lifetime constant
	local   auth.Credential // process-lifetime constant
	origin  string          // tailscale origin; skid connect replaces it
	path    string
	doc     document // committed
	data    []byte   // committed bytes
	changed chan struct{}
	wake    chan struct{} // a member's local request reached the hub: follow it now
	stopped context.Context
	stop    context.CancelCauseFunc
	lock    *os.File
}

// Open takes the state directory's single-writer lease for the process lifetime,
// then loads fleet.json or bootstraps a new fleet with this computer as its hub.
func Open(o Options) (_ *Service, err error) {
	if err := os.MkdirAll(o.Directory, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(o.Directory, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(o.Directory, "authority.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.Join(errors.New("fleet service already owns this state directory"), lock.Close())
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, lock.Close())
		}
	}()
	handle, err := loadHandle(o.HandlePath)
	if err != nil {
		return nil, err
	}
	s := &Service{options: o, machine: handle, origin: o.Origin, path: filepath.Join(o.Directory, "fleet.json"), changed: make(chan struct{}), wake: make(chan struct{}, 1), lock: lock}
	s.stopped, s.stop = context.WithCancelCause(context.Background())
	var next document
	data, err := privatefile.Read(s.path, maximumStateBytes)
	switch {
	case errors.Is(err, os.ErrNotExist):
		data, err := os.ReadFile(o.ConfigPath)
		if err != nil {
			return nil, err
		}
		c, err := hostconfig.Parse(data)
		if err != nil {
			return nil, err
		}
		if !machine.ValidName(o.Name) {
			return nil, errors.New("hostname is not a valid machine name")
		}
		if err := c.ValidateMachines([]string{o.Name}); err != nil {
			return nil, err
		}
		local, _ := newBearer()
		if next, err = s.newFleet(document{Machine: handle.String(), Name: o.Name, LocalBearer: local}, c); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &s.doc); err != nil {
			return nil, err
		}
		s.data = data
		if s.doc.Machine != handle.String() {
			return nil, errors.New("stored fleet identity does not match installation")
		}
		next = s.draft()
		if next.Authority != nil {
			// An invalid config.yaml keeps the accepted settings; skid apply reports it.
			_ = s.acceptConfig(&next)
		}
	}
	if err := s.commit(next); err != nil {
		return nil, err
	}
	if s.local, err = auth.ParseCredential(s.doc.LocalBearer); err != nil {
		return nil, err
	}
	if s.doc.Applied.ApplyStatus.AppliedConfigRevision == 0 {
		return nil, errors.New("local settings invalid; fix config.yaml and restart skid")
	}
	return s, nil
}

// loadHandle reads the write-once machine handle, creating it on first start.
func loadHandle(path string) (machine.Handle, error) {
	data, err := privatefile.Read(path, 64)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte(randomID("mh-") + "\n")
		err = privatefile.WriteFile(path, data)
	}
	if err != nil {
		return machine.Handle{}, err
	}
	return machine.Parse(strings.TrimSuffix(string(data), "\n"))
}

// newFleet makes base the hub of a new fleet. The previous fleet's observer state
// is deleted first, so the commit that follows never pairs it with the new fleet.
func (s *Service) newFleet(base document, c hostconfig.Config) (document, error) {
	id := make([]byte, 32)
	_, _ = rand.Read(id) // crypto/rand cannot fail
	bearer, verifier := newBearer()
	device := randomID("d-")
	base.Fleet = membership{DeviceID: device, Bearer: bearer, State: state{
		FleetID: hex.EncodeToString(id), Hub: base.Machine, StateRevision: 1, ConfigRevision: 1,
		Machines: []Machine{{base.Machine, base.Name, s.origin}},
		Members:  []Member{{device, "computer", base.Machine, verifier}},
		Settings: &c,
	}}
	base.Authority = &authority{Invites: []invite{}, Reports: map[string]SelfReport{}}
	return base, resetObserver(s.options.ObserverPath)
}

// resetObserver durably deletes the notification observer's fleet-scoped state.
func resetObserver(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

// draft decodes the committed bytes into a private copy for one mutation.
func (s *Service) draft() document {
	var d document
	_ = json.Unmarshal(s.data, &d) // committed bytes always decode
	return d
}

// resolve derives this host's applied settings from the fleet's accepted settings.
// It applies the same predicate serve applies, so a persisted snapshot always
// launches. Failure keeps the previous applied settings.
func (s *Service) resolve(d *document) {
	accepted := d.Fleet.State
	d.Applied.ApplyStatus = ApplyStatus{DesiredConfigRevision: accepted.ConfigRevision, AppliedConfigRevision: d.Applied.ApplyStatus.AppliedConfigRevision, Status: "invalid_host", Reason: "settings unavailable on this host; fix its shell or local paths"}
	settings, err := hostconfig.Resolve(*accepted.Settings, d.Name, s.options.Home, s.options.LoginShell)
	if err == nil {
		_, err = agentruntime.ResolveProfiles(settings.Profiles)
	}
	if err != nil {
		return
	}
	if !slices.ContainsFunc(accepted.Machines, func(m Machine) bool { return strings.EqualFold(m.Name, settings.DefaultMachine) }) {
		settings.DefaultMachine = d.Name
	}
	d.Applied = applied{settings, ApplyStatus{DesiredConfigRevision: accepted.ConfigRevision, AppliedConfigRevision: accepted.ConfigRevision, Status: "applied"}}
}

// commit persists next with one atomic replace. A failed write stops the service:
// the replace may or may not have happened, and only a restart can tell. The
// caller holds mu.
func (s *Service) commit(next document) error {
	if s.stopped.Err() != nil {
		return context.Cause(s.stopped)
	}
	s.resolve(&next)
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if bytes.Equal(data, s.data) {
		return nil
	}
	if err := privatefile.WriteFile(s.path, data); err != nil {
		s.stop(err)
		return err
	}
	s.doc, s.data = next, data
	s.notify()
	return nil
}
func (s *Service) notify() { close(s.changed); s.changed = make(chan struct{}) }

// Machine is this installation's handle, fixed for the process.
func (s *Service) Machine() machine.Handle { return s.machine }

// Watch returns the committed fleet and a channel closed when it is superseded.
func (s *Service) Watch() (Local, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.local(), s.changed
}

// Done is closed when the process must restart: a committed trust change or a
// failed write. Err is then errRestart or the write error; nil before.
func (s *Service) Done() <-chan struct{} { return s.stopped.Done() }
func (s *Service) Err() error            { return context.Cause(s.stopped) }

// Close releases the single-writer lease after HTTP and Run have stopped.
func (s *Service) Close() error { return s.lock.Close() }

// Authenticate admits the installed local bearer as this computer, or a current
// member's bearer. Request source never grants authority: tailscale serve also
// connects over loopback.
func (s *Service) Authenticate(r *http.Request) (Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authenticate(r)
}
func (s *Service) authenticate(r *http.Request) (Member, error) {
	headers := r.Header.Values("Authorization")
	if len(headers) == 1 && !strings.ContainsRune(headers[0], ',') {
		if s.local.Verify(headers[0]) {
			return Member{DeviceID: "local-" + s.local.Verifier(), Kind: "computer", Machine: s.machine.String(), Verifier: s.local.Verifier()}, nil
		}
		for _, m := range s.doc.Fleet.State.Members {
			if auth.VerifyDigest(m.Verifier, headers[0]) {
				return m, nil
			}
		}
	}
	return Member{}, &Error{Code: "Unauthenticated", Message: "authentication required", Status: 401}
}

// status presents membership and each computer's last report; the hub alone holds
// reports, so a member shows the other computers as pending_offline.
func (s *Service) status() Status {
	current := s.doc.Fleet.State
	result := Status{Version: s.options.Version, FleetID: current.FleetID, Hub: current.Hub, StateRevision: current.StateRevision, ConfigRevision: current.ConfigRevision, Members: []Member{}, Applications: []Application{}}
	for _, m := range current.Members {
		result.Members = append(result.Members, Member{DeviceID: m.DeviceID, Kind: m.Kind, Machine: m.Machine})
		if m.Kind != "computer" {
			continue
		}
		name := ""
		for _, entry := range current.Machines {
			if entry.Machine == m.Machine {
				name = entry.Name
			}
		}
		report := SelfReport{ApplyStatus: ApplyStatus{DesiredConfigRevision: current.ConfigRevision, Status: "pending_offline"}}
		if s.doc.Authority != nil {
			if stored, ok := s.doc.Authority.Reports[m.DeviceID]; ok {
				report = stored
			}
		}
		if m.Machine == s.machine.String() {
			report.ApplyStatus = s.doc.Applied.ApplyStatus
		}
		result.Applications = append(result.Applications, Application{m.DeviceID, name, report})
	}
	if !slices.ContainsFunc(current.Members, func(m Member) bool { return m.DeviceID == s.doc.Fleet.DeviceID }) {
		result.Message = "removed from the fleet; run skid join with a fresh invitation"
	}
	return result
}
