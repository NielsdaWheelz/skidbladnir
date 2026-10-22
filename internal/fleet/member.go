package fleet

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

// errMemberView is a hub reply without the settings every computer reads: a lost
// response, never a crash.
var errMemberView = errors.New("hub reply lacks fleet settings")

// Run makes a member follow the hub: fetch, adopt, report, then wait on the
// hub's revision stream. The hub has nothing to follow and returns at once.
// The cached state has no expiry: a member keeps serving through hub outages.
// Failures back off from 1 s to 30 s; a local request that reached the hub
// resets the backoff.
func (s *Service) Run(ctx context.Context) {
	if s.isHub() {
		return
	}
	delay := time.Second
	for ctx.Err() == nil && s.stopped.Err() == nil {
		if s.sync(ctx) == nil {
			delay = time.Second
			continue
		}
		timer := time.NewTimer(delay)
		delay = min(2*delay, 30*time.Second)
		select {
		case <-ctx.Done():
		case <-s.stopped.Done():
		case <-s.wake:
			delay = time.Second
		case <-timer.C:
		}
		timer.Stop()
	}
}

// sync returns nil when the hub's revision moved past the adopted state, so Run
// fetches again at once; any failure returns an error and Run backs off.
func (s *Service) sync(ctx context.Context) error {
	hub, err := s.hubTarget()
	if err != nil {
		return err
	}
	var current state
	if err := hubCall(ctx, hub, "GET", "/v1/fleet", nil, &current); err != nil {
		var failure *Error
		if errors.As(err, &failure) && failure.Code == "Unauthenticated" {
			// The hub no longer admits this computer, so no later removal can reach
			// it: stop admitting this fleet's devices. The local bearer still works.
			s.mu.Lock()
			defer s.mu.Unlock()
			next := s.draft()
			next.Fleet.State.Members = nil
			return errors.Join(err, s.commit(next))
		}
		return err
	}
	if current.Settings == nil {
		return errMemberView
	}
	if err := s.adopt(current); err != nil {
		return err
	}
	s.mu.Lock()
	report := SelfReport{Origin: s.origin, Version: s.options.Version, ApplyStatus: s.doc.Applied.ApplyStatus}
	revision := s.doc.Fleet.State.StateRevision
	s.mu.Unlock()
	if err := hubCall(ctx, hub, "PUT", "/v1/fleet/self", report, nil); err != nil {
		return err
	}
	request, err := NewRequest(ctx, hub, "GET", "/v1/fleet/events", nil)
	if err != nil {
		return err
	}
	response, err := peerClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("fleet revision stream unavailable")
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		var hint struct {
			StateRevision uint64 `json:"stateRevision"`
		}
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if ok && json.Unmarshal([]byte(data), &hint) == nil && hint.StateRevision != revision {
			return nil
		}
	}
	return errors.Join(errors.New("fleet revision stream closed"), scanner.Err())
}

// adopt replaces the cached membership and settings with the hub's state.
func (s *Service) adopt(current state) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.draft()
	next.Fleet.State = current
	return s.commit(next)
}

func (s *Service) hubTarget() (Target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hub := s.doc.local().Target(s.doc.Fleet.State.Hub)
	if hub.Origin == "" {
		return Target{}, &Error{Code: "HubUnavailable", Message: "hub offline; reconnect its tailscale network", Status: 503}
	}
	return hub, nil
}

func hubCall(ctx context.Context, hub Target, method, path string, body, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return Call(ctx, hub, method, path, body, result)
}

// enroll joins this computer to the invitation's fleet with a new device
// credential, then stops the service so it restarts as that fleet's member. A
// failed join changes nothing.
func (s *Service) enroll(ctx context.Context, invitation Invitation, name string) (Status, error) {
	s.mu.Lock()
	if name == "" {
		name = s.doc.Name
	}
	origin := s.origin
	s.mu.Unlock()
	if !machine.ValidName(name) {
		return Status{}, &Error{Code: "InvalidRequest", Message: "name: use visible characters without surrounding spaces", Status: 400}
	}
	device := randomID("d-")
	bearer, verifier := newBearer()
	request := joinRequest{Token: invitation.Token, DeviceID: device, Kind: "computer", Name: name, Machine: s.machine.String(), Origin: origin, Verifier: verifier}
	var reply state
	if err := hubCall(ctx, Target{Origin: invitation.Hub.Origin, Machine: invitation.Hub.Machine}, "POST", "/v1/fleet/join", request, &reply); err != nil {
		return Status{}, err
	}
	if reply.Settings == nil {
		return Status{}, errMemberView
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.draft()
	next.Name = name
	next.Fleet = membership{DeviceID: device, Bearer: bearer, State: reply}
	next.Authority = nil
	if err := s.commit(next); err != nil {
		return Status{}, err
	}
	s.stop(errRestart)
	return s.status(), nil
}

// localApply accepts the hub's config.yaml: the hub applies it, a member asks
// the hub and returns the hub's status.
func (s *Service) localApply(ctx context.Context) (Status, error) {
	if s.isHub() {
		return s.apply()
	}
	hub, err := s.hubTarget()
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := hubCall(ctx, hub, "POST", "/v1/fleet/apply", nil, &status); err != nil {
		return Status{}, err
	}
	s.followNow()
	return status, nil
}

// followNow cuts a member's backoff short: its hub just answered.
func (s *Service) followNow() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// setOrigin records the tailscale origin skid connect configured and reports it
// to the hub.
func (s *Service) setOrigin(ctx context.Context, origin string) (Status, error) {
	if !machine.ValidOrigin(origin) {
		return Status{}, &Error{Code: "InvalidRequest", Message: "tailscale https origin on port 8443 required", Status: 400}
	}
	s.mu.Lock()
	s.origin = origin
	self := Member{DeviceID: s.doc.Fleet.DeviceID, Kind: "computer", Machine: s.machine.String()}
	report := SelfReport{Origin: origin, Version: s.options.Version, ApplyStatus: s.doc.Applied.ApplyStatus}
	hub := s.doc.Authority != nil
	s.mu.Unlock()
	if hub {
		return s.report(self, report)
	}
	target, err := s.hubTarget()
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := hubCall(ctx, target, "PUT", "/v1/fleet/self", report, &status); err != nil {
		return Status{}, err
	}
	s.followNow()
	return status, nil
}
