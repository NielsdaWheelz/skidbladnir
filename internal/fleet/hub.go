package fleet

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

type joinRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Machine  string `json:"machine,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Verifier string `json:"verifier"`
}

var errNotHub = conflict("this computer is not the hub")

func (s *Service) isHub() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Authority != nil
}

// acceptConfig makes the hub's config.yaml the fleet's accepted settings.
func (s *Service) acceptConfig(next *document) error {
	data, err := os.ReadFile(s.options.ConfigPath)
	if err != nil {
		return err
	}
	c, err := hostconfig.Parse(data)
	if err == nil {
		names := []string{}
		for _, m := range next.Fleet.State.Machines {
			names = append(names, m.Name)
		}
		err = c.ValidateMachines(names)
	}
	if err != nil {
		return &Error{Code: "ConfigInvalid", Message: err.Error(), Status: 400}
	}
	previous, _ := json.Marshal(next.Fleet.State.Settings)
	accepted, _ := json.Marshal(c)
	if !bytes.Equal(previous, accepted) {
		current := &next.Fleet.State
		current.Settings, current.AndroidPush = &c, publicAndroidPush(c.AndroidPush)
		current.StateRevision++
		current.ConfigRevision++
	}
	return nil
}

// apply always signals a change, even with equal settings, so serve re-reads a
// replaced push credential. Revision hints go out only for a new state revision.
func (s *Service) apply() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Authority == nil {
		return Status{}, errNotHub
	}
	next := s.draft()
	if err := s.acceptConfig(&next); err != nil {
		return Status{}, err
	}
	if err := s.commit(next); err != nil {
		return Status{}, err
	}
	s.notify()
	return s.status(), nil
}

// invite mints a one-use token valid for five minutes; only its verifier is kept.
func (s *Service) invite(now time.Time) (Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Authority == nil {
		return Invitation{}, errNotHub
	}
	next := s.draft()
	current := next.Fleet.State
	hub := current.Machines[slices.IndexFunc(current.Machines, func(m Machine) bool { return m.Machine == current.Hub })]
	if hub.Origin == "" {
		return Invitation{}, conflict("tailscale network unavailable; configure its https origin before connect")
	}
	live := []invite{}
	for _, v := range next.Authority.Invites {
		if v.ExpiresAt > now.Unix() {
			live = append(live, v)
		}
	}
	if len(live) >= 32 {
		return Invitation{}, conflict("too many pending invitations; wait five minutes")
	}
	token, verifier := newBearer()
	expires := now.Add(5 * time.Minute).UTC()
	next.Authority.Invites = append(live, invite{verifier, expires.Unix()})
	if err := s.commit(next); err != nil {
		return Invitation{}, err
	}
	return Invitation{current.FleetID, hub, token, expires}, nil
}

// join admits a device with a live token. Conflicts are checked before the token,
// and the token is consumed only by a successful join. A computer that is already
// a member is re-keyed in place, keeping its position: one rule for a lost reply,
// a re-join after removal and a changed origin.
func (s *Service) join(request joinRequest, now time.Time) (state, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Authority == nil {
		return state{}, errNotHub
	}
	computer := request.Kind == "computer"
	_, handleErr := machine.Parse(request.Machine)
	shaped := computer && handleErr == nil || request.Kind == "phone" && request.Machine == "" && request.Origin == ""
	if !shaped || !ValidDeviceID(request.DeviceID) || !auth.ValidVerifier(request.Verifier) {
		return state{}, &Error{Code: "InvalidRequest", Message: "invalid enrollment", Status: 400}
	}
	if computer && (!machine.ValidName(request.Name) || request.Origin != "" && !machine.ValidOrigin(request.Origin)) {
		return state{}, &Error{Code: "InvalidRequest", Message: "invalid machine name or origin", Status: 400}
	}
	next := s.draft()
	current := &next.Fleet.State
	for _, m := range current.Members {
		if m.DeviceID == request.DeviceID || m.Verifier == request.Verifier {
			return state{}, conflict("device already enrolled")
		}
	}
	if computer {
		if request.Machine == current.Hub {
			return state{}, conflict("this computer is the hub")
		}
		for _, m := range current.Machines {
			if m.Machine != request.Machine && strings.EqualFold(m.Name, request.Name) {
				return state{}, conflict("machine name already enrolled; choose an explicit new name")
			}
			if m.Machine != request.Machine && request.Origin != "" && m.Origin == request.Origin {
				return state{}, conflict("origin already enrolled")
			}
		}
	}
	used := slices.IndexFunc(next.Authority.Invites, func(v invite) bool {
		return now.Unix() < v.ExpiresAt && auth.VerifyDigest(v.Verifier, "Bearer "+request.Token)
	})
	if used < 0 {
		return state{}, &Error{Code: "PairingInviteRejected", Message: "this fleet invite is invalid, expired, or already used", Status: 401}
	}
	next.Authority.Invites = slices.Delete(next.Authority.Invites, used, used+1)
	member := Member{request.DeviceID, request.Kind, request.Machine, request.Verifier}
	existing := -1
	if computer {
		existing = slices.IndexFunc(current.Members, func(m Member) bool { return m.Machine == request.Machine })
	}
	if existing >= 0 {
		delete(next.Authority.Reports, current.Members[existing].DeviceID)
		current.Members[existing] = member
		entry := &current.Machines[slices.IndexFunc(current.Machines, func(m Machine) bool { return m.Machine == request.Machine })]
		entry.Name = request.Name
		if request.Origin != "" {
			entry.Origin = request.Origin
		}
	} else {
		current.Members = append(current.Members, member)
		if computer {
			current.Machines = append(current.Machines, Machine{request.Machine, request.Name, request.Origin})
		}
	}
	current.StateRevision++
	if err := s.commit(next); err != nil {
		return state{}, err
	}
	return next.Fleet.State, nil
}

func (s *Service) remove(device string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Authority == nil {
		return Status{}, errNotHub
	}
	next := s.draft()
	current := &next.Fleet.State
	index := slices.IndexFunc(current.Members, func(m Member) bool { return m.DeviceID == device })
	if index < 0 {
		return Status{}, conflict("device is not enrolled")
	}
	removed := current.Members[index]
	if removed.Kind == "computer" && removed.Machine == current.Hub {
		return Status{}, conflict("the hub cannot be removed")
	}
	current.Members = slices.Delete(current.Members, index, index+1)
	if removed.Kind == "computer" {
		current.Machines = slices.DeleteFunc(current.Machines, func(m Machine) bool { return m.Machine == removed.Machine })
	}
	delete(next.Authority.Reports, device)
	current.StateRevision++
	if err := s.commit(next); err != nil {
		return Status{}, err
	}
	status := s.status()
	status.Message = "device removed"
	return status, nil
}

// report records a computer's own application status and network origin. An
// empty origin carries no network evidence and never withdraws a known one.
func (s *Service) report(member Member, report SelfReport) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Authority == nil {
		return Status{}, errNotHub
	}
	if member.Kind != "computer" {
		return Status{}, &Error{Code: "Forbidden", Message: "computer membership required", Status: 403}
	}
	next := s.draft()
	current := &next.Fleet.State
	if !slices.ContainsFunc(current.Members, func(m Member) bool {
		return m.DeviceID == member.DeviceID && m.Kind == "computer" && m.Machine == member.Machine
	}) {
		return Status{}, &Error{Code: "Forbidden", Message: "current computer membership required", Status: 403}
	}
	if report.Origin != "" && !machine.ValidOrigin(report.Origin) || !slices.Contains([]string{"applied", "invalid_host", "pending_offline"}, report.ApplyStatus.Status) {
		return Status{}, &Error{Code: "InvalidRequest", Message: "invalid host report", Status: 400}
	}
	if report.Origin != "" {
		for _, entry := range current.Machines {
			if entry.Machine != member.Machine && entry.Origin == report.Origin {
				return Status{}, conflict("origin already belongs to another computer")
			}
		}
		for i := range current.Machines {
			if entry := &current.Machines[i]; entry.Machine == member.Machine && entry.Origin != report.Origin {
				entry.Origin = report.Origin
				current.StateRevision++
			}
		}
	}
	next.Authority.Reports[member.DeviceID] = report
	if err := s.commit(next); err != nil {
		return Status{}, err
	}
	return s.status(), nil
}

// recover makes this computer the hub of a new fleet holding only itself, from
// its cached accepted settings, then stops the service so it restarts in that role.
func (s *Service) recover() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.draft()
	c := *previous.Fleet.State.Settings
	c.Machines, c.DefaultMachine = nil, previous.Name
	next, err := s.newFleet(document{Machine: previous.Machine, Name: previous.Name, LocalBearer: previous.LocalBearer, Applied: previous.Applied}, c)
	if err != nil {
		return Status{}, err
	}
	if err := s.commit(next); err != nil {
		return Status{}, err
	}
	s.stop(errRestart)
	return s.status(), nil
}
