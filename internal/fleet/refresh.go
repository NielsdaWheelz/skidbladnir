package fleet

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Refresh follows signed membership through known gateways without making any
// gateway an execution proxy. The existing client tick owns refresh scheduling.
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	if e := s.admissionError(); e != nil {
		s.mu.Unlock()
		return e
	}
	association := s.identity.ActiveFleet
	if association == nil {
		s.mu.Unlock()
		return nil
	}
	a := *association
	origin, version := s.options.Origin, s.options.Version
	machines := append([]Machine{}, s.public.Machines...)
	hub := s.public.ActiveHub
	s.mu.Unlock()
	var failures []error
	// Prefer the active authority but allow any reachable member to relay its cache.
	ordered := []Machine{}
	for _, m := range machines {
		if m.Machine == hub {
			ordered = append(ordered, m)
		}
	}
	for _, m := range machines {
		if m.Machine != hub {
			ordered = append(ordered, m)
		}
	}
	for _, m := range ordered {
		if m.Machine == s.options.Machine || m.Origin == "" {
			continue
		}
		var state HostState
		e := s.request(ctx, m.Origin, m.Machine, a.Bearer, "GET", "/v1/fleet/host-state", nil, &state)
		if e != nil {
			var typed *Error
			if errors.As(e, &typed) && typed.Code == "DeviceRemoved" && typed.Removal != nil {
				return s.AcceptRemoval(*typed.Removal)
			}
			failures = append(failures, e)
			continue
		}
		if e = s.Receive(state); e != nil {
			failures = append(failures, e)
			continue
		}
		break
	}
	identity := s.Identity()
	if identity.ActiveFleet == nil {
		return nil
	}
	_, application := s.Settings()
	report := SelfReport{Origin: origin, Version: version, ApplyStatus: application}
	s.mu.Lock()
	selfHub := s.public.ActiveHub == s.options.Machine
	s.mu.Unlock()
	if selfHub {
		member := Member{DeviceID: identity.ActiveFleet.DeviceID, Kind: "computer", Machine: s.options.Machine}
		_, e := s.Report(member, report)
		return e
	}
	var status Status
	if e := s.RequestHub(ctx, "PUT", "/v1/fleet/self", report, &status); e != nil {
		failures = append(failures, e)
	}
	return errors.Join(failures...)
}
func (s *Service) ApplyContext(ctx context.Context) (Status, error) {
	s.mu.Lock()
	selfHub := s.public.ActiveHub == s.options.Machine
	s.mu.Unlock()
	if !selfHub {
		var status Status
		if e := s.RequestHub(ctx, "POST", "/v1/fleet/apply", nil, &status); e != nil {
			return s.Status(), e
		}
		if e := s.Refresh(ctx); e != nil {
			return status, e
		}
		return status, nil
	}
	status, e := s.Apply()
	if e != nil {
		return status, e
	}
	target := ""
	s.mu.Lock()
	desired := s.private.Settings.Hub
	for _, m := range s.public.Machines {
		if strings.EqualFold(m.Name, desired) && m.Machine != s.public.ActiveHub {
			target = m.Machine
		}
	}
	s.mu.Unlock()
	if target == "" {
		return status, nil
	}
	if e = s.TransferHub(ctx, target); e != nil {
		status = s.Status()
		status.Message = "config accepted; hub transfer pending"
		var credentialError *Error
		if errors.As(e, &credentialError) && credentialError.Code == androidPushCredentialsUnavailable {
			status.Message += ": " + androidPushCredentialsAction
		}
		return status, nil
	}
	return s.Status(), nil
}

// TransferHub drives the finite transfer; every retry reuses its durable bundle.
// A destination outage before acknowledgement can abort and leave the old hub
// active. After fencing, all uncertainty stays fenced until exact activation.
func (s *Service) TransferHub(ctx context.Context, target string) error {
	s.mu.Lock()
	if e := s.admissionError(); e != nil {
		s.mu.Unlock()
		return e
	}
	machine := Machine{}
	for _, m := range s.public.Machines {
		if m.Machine == target {
			machine = m
		}
	}
	identity := s.identity.ActiveFleet
	s.mu.Unlock()
	if identity == nil || machine.Origin == "" {
		return &Error{Code: "HubUnavailable", Message: "hub target offline; restore its tailscale connection", Status: 503}
	}

	s.mu.Lock()
	existing := s.authority != nil && s.authority.Transfer != nil && s.authority.Transfer.Target == target && (s.authority.Transfer.Phase == "preparing" || s.authority.Transfer.Phase == "fenced")
	s.mu.Unlock()
	if !existing {
		var state HostState
		if e := s.request(ctx, machine.Origin, machine.Machine, identity.Bearer, "GET", "/v1/fleet/host-state", nil, &state); e != nil {
			return e
		}
		key, e := ParsePublicKey(identity.PublicKey)
		if e != nil {
			return e
		}
		if _, _, e = VerifyHostState(key, state); e != nil {
			return e
		}
	}
	bundle, e := s.Prepare(target)
	if e != nil {
		return e
	}

	ack := bundle.Transfer
	s.mu.Lock()
	fenced := s.authority.Transfer.Phase == "fenced"
	s.mu.Unlock()
	if !fenced {

		if e = s.request(ctx, machine.Origin, machine.Machine, identity.Bearer, "POST", "/v1/fleet/handoff/prepare", bundle, &ack); e != nil {
			aborted, abortErr := s.Abort(bundle.Transfer.TransferID)
			if abortErr == nil {
				// The signed abort is durable before source writers resume. If staging's
				// response was lost, destination invalidation can finish when reachable.
				var ignored any
				_ = s.request(ctx, machine.Origin, machine.Machine, identity.Bearer, "POST", "/v1/fleet/handoff/abort", aborted, &ignored) // justify-ignore-error: pre-fence source abort is durable; stale staging has no activation authority.
			}
			return errors.Join(e, abortErr)
		}

	} else {
		ack.Phase = "staged"
	}

	if ack.TransferID != bundle.Transfer.TransferID || ack.BundleDigest != bundle.Transfer.BundleDigest || ack.Target != target || ack.Phase != "staged" && ack.Phase != "active" {
		_, abortErr := s.Abort(bundle.Transfer.TransferID)
		return errors.Join(errors.New("invalid hub staging acknowledgement"), abortErr)
	}
	activation, e := s.Fence(ack.TransferID, ack.BundleDigest)
	if e != nil {
		return e
	}
	var status Status
	if e = s.request(ctx, machine.Origin, machine.Machine, identity.Bearer, "POST", "/v1/fleet/handoff/activate", activation, &status); e != nil {
		return e
	}
	var state HostState
	if e = s.request(ctx, machine.Origin, machine.Machine, identity.Bearer, http.MethodGet, "/v1/fleet/host-state", nil, &state); e != nil {
		return e
	}
	return s.Receive(state)
}
