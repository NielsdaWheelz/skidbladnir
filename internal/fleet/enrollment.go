package fleet

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

func ValidDeviceID(value string) bool { return validDeviceID(value) }
func tokenVerifier(token string) string {
	sum := sha256.Sum256([]byte(token))
	return encoding.EncodeToString(sum[:])
}
func (s *Service) Invite(now time.Time) (Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return Invitation{}, e
	}
	if now.IsZero() {
		return Invitation{}, errors.New("invitation time unavailable")
	}
	hub := Machine{}
	for _, m := range s.public.Machines {
		if m.Machine == s.public.ActiveHub {
			hub = m
		}
	}
	if hub.Origin == "" {
		return Invitation{}, conflict("tailscale network unavailable; configure its https origin before connect")
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return Invitation{}, e
	}
	token := encoding.EncodeToString(raw)
	expires := now.Add(5 * time.Minute).UTC()
	a := s.authority
	kept := []inviteRecord{}
	for _, invite := range a.Invites {
		if invite.ExpiresAt > now.Unix() {
			kept = append(kept, invite)
		}
	}
	if len(kept) >= 32 {
		return Invitation{}, conflict("too many pending invitations; wait five minutes")
	}
	a.Invites = append(kept, inviteRecord{tokenVerifier(token), expires.Unix()})
	if e := s.saveAuthority(); e != nil {
		return Invitation{}, e
	}

	key, e := s.key()
	if e != nil {
		return Invitation{}, e
	}
	proof, e := Sign(key, "invitation", InvitationClaims{1, "invitation", s.public.FleetID, hub, token, expires})
	if e != nil {
		return Invitation{}, e
	}
	return Invitation{Proof: proof, Schema: 1, FleetID: s.public.FleetID, PublicKey: s.identity.ActiveFleet.PublicKey, Hub: hub, Token: token, ExpiresAt: expires}, nil
}
func ValidateInvitation(i Invitation, now time.Time) error {
	key, e := ParsePublicKey(i.PublicKey)
	if e != nil || i.Schema != 1 || i.FleetID != Fingerprint(key) || !machine.ValidOrigin(i.Hub.Origin) || !machine.ValidName(i.Hub.Name) || !now.Before(i.ExpiresAt) || i.ExpiresAt.After(now.Add(5*time.Minute+time.Second)) {
		return errors.New("invitation invalid or expired")
	}

	var claims InvitationClaims
	if e = Verify(key, "invitation", i.Proof, &claims); e != nil {
		return errors.New("invitation proof is invalid")
	}
	if claims.Schema != 1 || claims.Kind != "invitation" || claims.FleetID != i.FleetID || claims.Hub != i.Hub || claims.Token != i.Token || !claims.ExpiresAt.Equal(i.ExpiresAt) {
		return errors.New("invitation proof does not match its fields")
	}
	if _, e = machine.Parse(i.Hub.Machine); e != nil {
		return e
	}
	token, e := encoding.DecodeString(i.Token)
	if e != nil || len(token) != 32 {
		return errors.New("invitation token invalid")
	}
	return nil
}
func (s *Service) StageJoin(invitation Invitation, name string) (JoinRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.admissionError(); e != nil {
		return JoinRequest{}, e
	}

	if name == "" {
		name = s.options.Name
	}
	if s.identity.ActiveFleet != nil && s.identity.ActiveFleet.FleetID == invitation.FleetID {
		return JoinRequest{}, conflict("already enrolled; authenticate with the current identity to repair discovery")
	}
	if s.identity.Pending != nil {
		p := s.identity.Pending
		if p.Invitation == invitation && p.Request.Name == name {
			return p.Request, nil
		}
	}
	if name == "" {
		name = s.options.Name
	}
	if e := ValidateInvitation(invitation, time.Now()); e != nil {
		return JoinRequest{}, e
	}
	if !machine.ValidName(name) {
		return JoinRequest{}, errors.New("name: use visible characters without surrounding spaces")
	}
	credential, e := auth.NewCredential()
	if e != nil {
		return JoinRequest{}, e
	}
	id, e := randomID("d-")
	if e != nil {
		return JoinRequest{}, e
	}
	request := JoinRequest{invitation.Token, id, "computer", name, s.options.Machine, s.options.Origin, credential.Verifier()}
	pending := PendingEnrollment{Association{invitation.FleetID, invitation.PublicKey, id, credential.CanonicalBearer()}, invitation, request}
	identity := s.identity
	identity.Pending = &pending
	if e = s.storeIdentity(identity, false); e != nil {
		return JoinRequest{}, e
	}
	s.identity = identity
	return request, nil
}
func (s *Service) Join(request JoinRequest, now time.Time) (HostState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return HostState{}, e
	}
	member := Member{request.DeviceID, request.Kind, request.Machine, request.Verifier}
	if e := validMember(member); e != nil {
		return HostState{}, &Error{Code: "InvalidRequest", Message: "invalid enrollment", Status: 400}
	}
	// A committed response is recovered with the pending bearer, never by replaying
	// this consumed token. Duplicate enrollment is refused even for the same tuple.
	for _, m := range s.private.Members {
		if m.DeviceID == request.DeviceID || m.Verifier == request.Verifier || m.Machine != "" && m.Machine == request.Machine {
			return HostState{}, conflict("device already enrolled; authenticate with the pending credential")
		}
	}
	for _, r := range s.private.Retired {
		if r.DeviceID == request.DeviceID || r.Verifier == request.Verifier {
			return HostState{}, conflict("removed device requires a new device identity")
		}
	}
	if request.Kind == "computer" {
		if !machine.ValidName(request.Name) || request.Origin != "" && !machine.ValidOrigin(request.Origin) {
			return HostState{}, &Error{Code: "InvalidRequest", Message: "invalid machine name or origin", Status: 400}
		}
		for _, m := range s.public.Machines {
			if strings.EqualFold(m.Name, request.Name) {
				return HostState{}, conflict("machine name already enrolled; choose an explicit new name")
			}
			if request.Origin != "" && m.Origin == request.Origin {
				return HostState{}, conflict("origin already enrolled")
			}
		}
	}
	a := s.authority
	match := -1
	digest := tokenVerifier(request.Token)
	for i, invite := range a.Invites {
		if subtle.ConstantTimeCompare([]byte(invite.Verifier), []byte(digest)) == 1 && now.Unix() < invite.ExpiresAt {
			match = i
		}
	}
	if match < 0 {
		return HostState{}, &Error{Code: "PairingInviteRejected", Message: "this fleet invite is invalid, expired, or already used", Status: 401}
	}
	a.Invites = append(a.Invites[:match], a.Invites[match+1:]...)
	a.Private.Members = append(a.Private.Members, member)
	if request.Kind == "computer" {
		a.Public.Machines = append(a.Public.Machines, Machine{request.Machine, request.Name, request.Origin})
	}
	a.Public.StateRevision++
	s.syncRevisions()
	if e := s.saveMembership(); e != nil {
		return HostState{}, e
	}
	if e := s.publish(); e != nil {
		return HostState{}, e
	}
	return s.state, nil
}
func (s *Service) ConfirmJoin(state HostState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.admissionError(); e != nil {
		return e
	}

	pending := s.identity.Pending
	if pending == nil {
		return conflict("no staged enrollment")
	}
	key, e := ParsePublicKey(pending.Association.PublicKey)
	if e != nil {
		return e
	}
	p, h, e := VerifyHostState(key, state)
	if e != nil {
		return e
	}
	if p.FleetID != pending.Association.FleetID {
		return errors.New("wrong enrolled fleet")
	}
	found := false
	for _, m := range h.Members {
		found = found || m.DeviceID == pending.Request.DeviceID && m.Kind == "computer" && m.Machine == s.options.Machine && m.Verifier == pending.Request.Verifier
	}
	if !found {
		return errors.New("staged enrollment was not confirmed")
	}
	// Save old human yaml as an inactive file before adopting another authority.
	old, e := os.ReadFile(s.options.ConfigPath)
	if e == nil {
		if e = privatefile.WriteFile(s.options.ConfigPath+".inactive", old); e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	previousMembers := append([]Member{}, s.private.Members...)
	filename, e := s.stageEnrollmentState(state)
	if e != nil {
		return e
	}

	if s.authority != nil && s.authority.Public.ActiveHub == s.options.Machine && s.options.Drain != nil {
		if _, e = s.options.Drain(); e != nil {
			return e
		}
	}

	oldLocal, e := auth.ParseCredential(s.identity.LocalBearer)
	if e != nil {
		return e
	}
	newLocal, e := auth.NewCredential()
	if e != nil {
		return e
	}
	identity := s.identity
	identity.LocalBearer = newLocal.CanonicalBearer()
	identity.Name = pending.Request.Name
	identity.ActiveFleet = &pending.Association
	identity.Pending = nil
	identity.EnrollmentState = filename
	if e = s.storeIdentity(identity, true); e != nil {
		return e
	}
	s.identity = identity
	s.options.Name = identity.Name
	s.notify()
	s.authority = nil
	s.state = state
	s.public = p
	s.private = h
	if s.options.CloseDevice != nil {
		s.options.CloseDevice("local-" + oldLocal.Verifier())
		for _, member := range previousMembers {
			s.options.CloseDevice(member.DeviceID)
		}
	}
	return s.install(state)
}
func (s *Service) Remove(deviceID string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return s.status(), e
	}
	a := s.authority
	index := -1
	for i, m := range a.Private.Members {
		if m.DeviceID == deviceID {
			index = i
		}
	}
	if index < 0 {
		for _, r := range a.Private.Retired {
			if r.DeviceID == deviceID {
				removalErr := s.removed(deviceID)
				publishErr := s.publish()
				return s.status(), errors.Join(removalErr, publishErr)
			}
		}
		return s.status(), conflict("device is not enrolled")
	}
	member := a.Private.Members[index]
	if member.Machine == a.Public.ActiveHub {
		return s.status(), conflict("active hub cannot be removed; transfer authority first")
	}
	a.Public.StateRevision++
	s.syncRevisions()
	key, e := s.key()
	if e != nil {
		return s.status(), e
	}
	proof, e := Sign(key, "member-removed", Removal{1, "member-removed", a.Public.FleetID, deviceID, a.Public.AuthorityEpoch, a.Public.StateRevision})
	if e != nil {
		return s.status(), e
	}
	retired := Retired{DeviceID: deviceID, Kind: member.Kind, Machine: member.Machine, Verifier: member.Verifier, Removal: proof}
	for i, m := range a.Public.Machines {
		if m.Machine == member.Machine {
			retired.Name = m.Name
			a.Public.Machines = append(a.Public.Machines[:i], a.Public.Machines[i+1:]...)
			break
		}
	}
	a.Private.Retired = append(a.Private.Retired, retired)
	a.Private.Members = append(a.Private.Members[:index], a.Private.Members[index+1:]...)
	delete(a.Reports, deviceID)
	if e = s.saveMembership(); e != nil {
		return s.status(), e
	}

	removalErr := s.removed(deviceID)
	publishErr := s.publish()
	if e = errors.Join(publishErr, removalErr); e != nil {
		return s.status(), e
	}

	status := s.status()
	status.Message = "device removed"
	if len(status.PendingRevocation) > 0 {
		status.Message += "; revocation pending on " + strings.Join(status.PendingRevocation, ", ")
	}
	if a.Private.Settings.Hub == retired.Name {
		status.Message += "; hub target removed; update config"
	}
	return status, nil
}
func (s *Service) Authenticate(request *http.Request) (Member, *Removal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authenticate(request)
}
func (s *Service) authenticate(r *http.Request) (Member, *Removal, error) {
	if e := s.admissionError(); e != nil {
		return Member{}, nil, e
	}
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 || strings.ContainsRune(headers[0], ',') {
		return Member{}, nil, &Error{Code: "Unauthenticated", Message: "authentication required", Status: 401}
	}
	// Stored installation identity authenticates local mode. Request source does
	// not grant local authority: tailscale serve also connects over loopback.
	credential, e := auth.ParseCredential(s.identity.LocalBearer)
	if e != nil {
		return Member{}, nil, e
	}
	if credential.Verify(headers[0]) {
		return Member{DeviceID: "local-" + credential.Verifier(), Kind: "computer", Machine: s.options.Machine, Verifier: credential.Verifier()}, nil, nil
	}
	if s.identity.ActiveFleet != nil {
		for _, m := range s.private.Members {
			if auth.VerifyDigest(m.Verifier, headers[0]) {
				return m, nil, nil
			}
		}
		for _, retired := range s.private.Retired {
			if auth.VerifyDigest(retired.Verifier, headers[0]) {
				key, e := ParsePublicKey(s.identity.ActiveFleet.PublicKey)
				if e != nil {
					return Member{}, nil, e
				}
				var removal Removal
				if e = Verify(key, "member-removed", retired.Removal, &removal); e != nil {
					return Member{}, nil, e
				}
				proof := retired.Removal
				return Member{}, &removal, &Error{Code: "DeviceRemoved", Message: "device removed; join again to restore access", Status: 403, Removal: &proof}
			}
		}
	}
	return Member{}, nil, &Error{Code: "Unauthenticated", Message: "authentication required", Status: 401}
}
func (s *Service) AcceptRemoval(envelope Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.identity.ActiveFleet
	if a == nil {
		return conflict("local mode rejects old removal proofs")
	}
	key, e := ParsePublicKey(a.PublicKey)
	if e != nil {
		return e
	}
	var removal Removal
	if e = Verify(key, "member-removed", envelope, &removal); e != nil {
		return e
	}
	if removal.Schema != 1 || removal.Kind != "member-removed" || removal.FleetID != a.FleetID || removal.DeviceID != a.DeviceID || removal.AuthorityEpoch < s.public.AuthorityEpoch || removal.StateRevision < s.public.StateRevision {
		return errors.New("removal proof does not match current enrollment")
	}
	return s.detach()
}
