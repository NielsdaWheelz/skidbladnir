package fleet

import (
	"context"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

// SetOrigin records the locally discovered tailscale endpoint. Only the installed
// credential admits this operation; it is never a human configuration override.
func (s *Service) SetOrigin(ctx context.Context, origin string) (Status, error) {
	if !machine.ValidOrigin(origin) {
		return s.Status(), &Error{Code: "InvalidRequest", Message: "tailscale https origin on port 8443 required", Status: 400}
	}
	s.mu.Lock()
	s.options.Origin = origin
	s.notify()
	identity := s.identity
	application := s.resolved.Application
	selfHub := s.public.ActiveHub == s.options.Machine
	version := s.options.Version
	s.mu.Unlock()
	if identity.ActiveFleet == nil {
		return s.Status(), nil
	}
	report := SelfReport{Origin: origin, Version: version, ApplyStatus: application}
	if selfHub {
		return s.Report(Member{DeviceID: identity.ActiveFleet.DeviceID, Kind: "computer", Machine: identity.Machine}, report)
	}
	var status Status
	if e := s.RequestHub(ctx, "PUT", "/v1/fleet/self", report, &status); e != nil {
		return s.Status(), e
	}
	if e := s.Refresh(ctx); e != nil {
		return s.Status(), e
	}
	return s.Status(), nil
}
