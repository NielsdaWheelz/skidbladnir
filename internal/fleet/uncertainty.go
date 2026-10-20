package fleet

import (
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// Authority uncertainty stops the hub's cached writers. It does not change the
// admitted terminal identities; a planned fence has this same narrow effect.
func (s *Service) fenceControl(cause error) error {
	s.controlUncertain = true
	if s.options.Drain != nil {
		_, e := s.options.Drain()
		cause = errors.Join(cause, e)
	}
	return cause
}

// Identity/membership uncertainty also closes every connection whose continued
// admission cannot be proved from the durable state. Restart reconciles storage.
func (s *Service) fenceAdmission(cause error) error {
	s.admissionUncertain = true
	cause = s.fenceControl(cause)
	s.closeConnections(s.identity, s.private.Members)
	s.notify()
	return cause
}
func (s *Service) closeConnections(identity Identity, members []Member) {
	if s.options.CloseDevice == nil {
		return
	}
	for _, member := range members {
		s.options.CloseDevice(member.DeviceID)
	}
	if credential, e := auth.ParseCredential(identity.LocalBearer); e == nil {
		s.options.CloseDevice("local-" + credential.Verifier())
	}
}
func (s *Service) storeIdentity(identity Identity, trustChanged bool) error {
	previous := s.identity
	if e := writePrivate(s.path("identity.json"), identity); e != nil {
		var durable Identity
		if readErr := readPrivate(s.path("identity.json"), &durable); readErr == nil && durable.Schema == 1 && durable.Machine == s.options.Machine {
			s.identity = durable
		} else {
			return s.fenceAdmission(errors.Join(e, readErr))
		}
		if trustChanged {
			s.closeConnections(previous, s.private.Members)
			return s.fenceAdmission(e)
		}
		return e
	}
	s.identity = identity
	return nil
}
func (s *Service) saveMembership() error {
	if e := s.saveAuthority(); e != nil {
		return s.fenceAdmission(e)
	}
	return nil
}
func (s *Service) removed(device string) error {
	if s.options.CloseDevice != nil {
		s.options.CloseDevice(device)
	}
	if s.options.DeviceRemoved != nil {
		return s.options.DeviceRemoved(device)
	}
	return nil
}

// Recover callbacks may be retried after a crash. The durable marker keeps new
// authority inactive until its old fleet-scoped observer state has been reset.
func (s *Service) completeRecovery() error {
	recovery := s.identity.Recovery
	if recovery == nil {
		return nil
	}
	if s.options.Recovery != nil {
		if e := s.options.Recovery(recovery.PreviousFleetID, recovery.FleetID); e != nil {
			return s.fenceControl(e)
		}
	}
	identity := s.identity
	identity.Recovery = nil

	if e := s.storeIdentity(identity, false); e != nil {
		return s.fenceControl(e)
	}
	if s.controlUncertain && s.authority != nil {
		return s.saveAuthority()
	}
	return nil
}

// cloneState materializes signed cache data separately from the mutable authority.
func cloneState(state HostState) (Public, Private, error) {
	publicBytes, e := encoding.DecodeString(state.Public.Payload)
	if e != nil {
		return Public{}, Private{}, e
	}
	privateBytes, e := encoding.DecodeString(state.Private.Payload)
	if e != nil {
		return Public{}, Private{}, e
	}
	var p Public
	var h Private
	if e = strictjson.Decode(publicBytes, &p); e != nil {
		return p, h, e
	}
	e = strictjson.Decode(privateBytes, &h)
	return p, h, e
}

func (s *Service) admissionError() error {
	if !s.admissionUncertain {
		return nil
	}
	return &Error{Code: "ControlUnavailable", Message: "identity or membership write uncertain; restart to reconcile durable state", Status: 503}
}
