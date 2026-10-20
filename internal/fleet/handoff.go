package fleet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

func bundleDigest(authority json.RawMessage, owned map[string]json.RawMessage) (string, error) {
	b, e := json.Marshal(struct {
		Authority  json.RawMessage            `json:"authority"`
		OwnedState map[string]json.RawMessage `json:"ownedState"`
	}{authority, owned})
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func (s *Service) Prepare(target string) (TransferBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authority != nil && s.authority.Transfer != nil {
		t := s.authority.Transfer
		if t.Target == target && (t.Phase == "preparing" || t.Phase == "fenced") && s.authority.Bundle != nil {
			return *s.authority.Bundle, nil
		}
	}

	a := s.authority
	retry := a != nil && a.Transfer != nil && a.Transfer.Target == target && a.Transfer.Source == s.options.Machine && a.Transfer.Phase == "preparing"
	if !retry {
		if e := s.active(); e != nil {
			return TransferBundle{}, e
		}
		if target == s.options.Machine {
			return TransferBundle{}, conflict("hub target is already active")
		}
		found := false
		for _, m := range s.public.Machines {
			found = found || m.Machine == target
		}
		if !found {
			return TransferBundle{}, conflict("hub target is not an enrolled computer")
		}

	}
	var e error
	owned := map[string]json.RawMessage{}
	if s.options.Drain != nil {
		owned, e = s.options.Drain()
		if e == nil && len(owned) == 0 {
			e = errors.New("notification state unavailable; hub transfer was not prepared")
		}
		if e != nil {
			if retry {
				_, abortErr := s.abort(a.Transfer.TransferID)
				return TransferBundle{}, errors.Join(e, abortErr)
			}
			if s.options.Resume != nil {
				e = errors.Join(e, s.options.Resume())
			}
			return TransferBundle{}, e
		}
	}
	if !retry {
		id, e := randomID("x-")
		if e != nil {
			return TransferBundle{}, e
		}
		a.Bundle = nil
		a.ActivationComplete = false
		a.Activation = nil
		a.Transfer = &Transfer{id, s.options.Machine, target, a.Public.AuthorityEpoch + 1, "", "preparing"}
		if e = s.saveAuthority(); e != nil {
			return TransferBundle{}, e
		}
	}
	// No source mutation/writer may proceed between this capture and fencing.
	bytes, e := json.Marshal(a)
	if e != nil {
		return TransferBundle{}, e
	}
	digest, e := bundleDigest(bytes, owned)
	if e != nil {
		return TransferBundle{}, e
	}
	a.Transfer.BundleDigest = digest
	bundle := TransferBundle{*a.Transfer, bytes, owned}
	a.Bundle = &bundle
	a.ActivationComplete = false
	if e = s.saveAuthority(); e != nil {
		return TransferBundle{}, e
	}
	return bundle, nil
}
func (s *Service) Stage(source Member, bundle TransferBundle) (Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if source.Kind != "computer" || source.Machine != bundle.Transfer.Source || bundle.Transfer.Target != s.options.Machine {
		return Transfer{}, &Error{Code: "Forbidden", Message: "designated source and destination required", Status: 403}
	}
	if s.identity.ActiveFleet == nil || source.Machine != s.public.ActiveHub {
		return Transfer{}, conflict("transfer source is not the active hub")
	}
	digest, e := bundleDigest(bundle.Authority, bundle.OwnedState)
	if e != nil || digest != bundle.Transfer.BundleDigest {
		return Transfer{}, errors.New("transfer bundle digest mismatch")
	}
	t := bundle.Transfer
	if t.Phase != "preparing" || t.NextAuthorityEpoch != s.public.AuthorityEpoch+1 {
		return Transfer{}, conflict("invalid transfer epoch or phase")
	}
	if s.authority != nil {
		for _, id := range s.authority.Aborted {
			if id == t.TransferID {
				return Transfer{}, conflict("transfer was aborted")
			}
		}
		if old := s.authority.Transfer; old != nil && old.TransferID == t.TransferID {
			if old.BundleDigest != digest {
				return Transfer{}, conflict("conflicting transfer bundle")
			}
			if e = s.validateHub(s.authority.Private.Settings.AndroidPush); e != nil {
				return Transfer{}, e
			}
			return *old, nil
		}
		if s.authority.Public.ActiveHub == s.options.Machine {
			return Transfer{}, conflict("destination already owns active authority")
		}
	}
	var a authority
	if e = strictjson.Decode(bundle.Authority, &a); e != nil {
		return Transfer{}, e
	}
	if a.Public.FleetID != s.public.FleetID || a.Public.ActiveHub != source.Machine || a.Public.AuthorityEpoch != s.public.AuthorityEpoch || a.Public.StateRevision < s.public.StateRevision {
		return Transfer{}, conflict("stale transfer bundle")
	}
	old := s.authority
	s.authority = &a
	if e = s.validateAuthority(); e != nil {
		s.authority = old
		return Transfer{}, e
	}
	if e = s.validateHub(a.Private.Settings.AndroidPush); e != nil {
		s.authority = old
		return Transfer{}, e
	}
	t.Phase = "staged"
	a.Transfer = &t
	a.Bundle = &bundle
	if e = s.saveAuthority(); e != nil {
		return Transfer{}, e
	}
	return t, nil
}

// Fence is legal only after the destination's durable staging acknowledgement.
// A write error leaves this service fenced in memory too: rename may have happened.
func (s *Service) Fence(transferID, digest string) (Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.authority
	if a == nil || a.Transfer == nil {
		return Envelope{}, conflict("no prepared transfer")
	}
	t := a.Transfer
	if t.TransferID != transferID || t.BundleDigest != digest || t.Source != s.options.Machine || t.Phase != "preparing" && t.Phase != "fenced" {
		return Envelope{}, conflict("transfer acknowledgement mismatch")
	}
	if a.Bundle == nil {
		return Envelope{}, conflict("transfer bundle was not captured")
	}
	if a.Activation == nil {
		copy := *t
		copy.Phase = "fenced"
		key, e := s.key()
		if e != nil {
			return Envelope{}, e
		}
		signed, e := Sign(key, "handoff-activation", Activation{1, "handoff-activation", a.Public.FleetID, copy})
		if e != nil {
			return Envelope{}, e
		}
		a.Activation = &signed
	}
	t.Phase = "fenced"
	if e := s.saveAuthority(); e != nil {
		return Envelope{}, e
	}
	return *a.Activation, nil
}
func (s *Service) ActivateTransfer(source Member, activation Envelope) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity.ActiveFleet == nil {
		return s.status(), conflict("no enrolled fleet")
	}
	key, e := ParsePublicKey(s.identity.ActiveFleet.PublicKey)
	if e != nil {
		return s.status(), e
	}
	var record Activation
	if e = Verify(key, "handoff-activation", activation, &record); e != nil {
		return s.status(), e
	}
	t := record.Transfer
	if record.Schema != 1 || record.Kind != "handoff-activation" || record.FleetID != s.public.FleetID || source.Kind != "computer" || source.Machine != t.Source || t.Target != s.options.Machine || t.Phase != "fenced" {
		return s.status(), conflict("activation does not match designated transfer")
	}
	a := s.authority
	if a == nil || a.Transfer == nil || a.Transfer.TransferID != t.TransferID || a.Transfer.BundleDigest != t.BundleDigest || a.Transfer.NextAuthorityEpoch != t.NextAuthorityEpoch {
		return s.status(), conflict("no matching durable staging")
	}

	if a.Transfer.Phase == "active" {
		if e = s.saveAuthority(); e != nil {
			return s.status(), e
		}
		if e = s.completeActivation(); e != nil {
			return s.status(), e
		}
		if e = s.publish(); e != nil {
			return s.status(), e
		}
		if e = s.install(s.state); e != nil {
			return s.status(), e
		}
		return s.status(), nil
	}

	if a.Transfer.Phase != "staged" || a.Bundle == nil {
		return s.status(), conflict("staging cannot activate")
	}
	for _, id := range a.Aborted {
		if id == t.TransferID {
			return s.status(), conflict("transfer was aborted")
		}
	}
	if e = s.validateHub(a.Private.Settings.AndroidPush); e != nil {
		return s.status(), e
	}
	a.Transfer.Phase = "active"
	a.Activation = &activation
	a.Public.ActiveHub = s.options.Machine
	a.Public.AuthorityEpoch = t.NextAuthorityEpoch
	a.Public.StateRevision++
	s.syncRevisions()
	// Activation is durable before owned writers start or new routing is published.
	if e = s.saveAuthority(); e != nil {
		return s.status(), e
	}
	if e = s.completeActivation(); e != nil {
		return s.status(), e
	}
	if e = s.publish(); e != nil {
		return s.status(), e
	}
	if e = s.install(s.state); e != nil {
		return s.status(), e
	}
	return s.status(), nil
}
func (s *Service) Abort(transferID string) (Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.abort(transferID)
}
func (s *Service) abort(transferID string) (Envelope, error) {
	a := s.authority
	if a == nil || a.Transfer == nil || a.Transfer.TransferID != transferID || a.Transfer.Source != s.options.Machine || a.Transfer.Phase != "preparing" {
		return Envelope{}, conflict("only a pre-fence transfer can abort")
	}
	t := *a.Transfer
	t.Phase = "aborted"
	key, e := s.key()
	if e != nil {
		return Envelope{}, e
	}
	signed, e := Sign(key, "handoff-abort", Activation{1, "handoff-abort", a.Public.FleetID, t})
	if e != nil {
		return Envelope{}, e
	}
	a.Transfer = &t
	a.Aborted = append(a.Aborted, transferID)
	a.Bundle = nil
	if e = s.saveAuthority(); e != nil {
		return Envelope{}, e
	}
	if s.options.Resume != nil {
		if e = s.options.Resume(); e != nil {
			return Envelope{}, e
		}
	}
	return signed, nil
}
func (s *Service) AbortStaged(source Member, abort Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity.ActiveFleet == nil {
		return conflict("no enrolled fleet")
	}
	key, e := ParsePublicKey(s.identity.ActiveFleet.PublicKey)
	if e != nil {
		return e
	}
	var record Activation
	if e = Verify(key, "handoff-abort", abort, &record); e != nil {
		return e
	}
	a := s.authority
	if a == nil || a.Transfer == nil || record.Schema != 1 || record.Kind != "handoff-abort" || record.FleetID != s.public.FleetID || record.Transfer.TransferID != a.Transfer.TransferID || record.Transfer.BundleDigest != a.Transfer.BundleDigest || record.Transfer.Phase != "aborted" || source.Kind != "computer" || source.Machine != a.Transfer.Source || a.Transfer.Target != s.options.Machine || a.Transfer.Phase != "staged" {
		return conflict("abort does not match staged transfer")
	}
	a.Aborted = append(a.Aborted, a.Transfer.TransferID)
	a.Transfer.Phase = "aborted"
	a.Bundle = nil
	return s.saveAuthority()
}
func (s *Service) TransferStatus() (*Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authority == nil || s.authority.Transfer == nil {
		return nil, nil
	}
	t := *s.authority.Transfer
	return &t, nil
}

// ResumeAuthority restores durable owned state after a process restart. A fenced
// source is never resumed. Composition calls this before starting hub writers.
func (s *Service) ResumeAuthority() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return e
	}
	a := s.authority
	if a.Transfer != nil && a.Transfer.Phase == "active" {
		if e := s.completeActivation(); e != nil {
			return e
		}
		if e := s.publish(); e != nil {
			return e
		}
		return s.install(s.state)
	}
	if s.options.Resume != nil {
		return s.options.Resume()
	}
	return nil
}

// Owned-state import must be idempotent across a crash between import and this
// completion commit. Its owner preserves newer state for the same transfer.
func (s *Service) completeActivation() error {
	a := s.authority
	if a.ActivationComplete {
		return nil
	}
	if a.Bundle == nil {
		return errors.New("active transfer lost its staged bundle")
	}
	if e := privatefile.WriteFile(s.options.ConfigPath, []byte(a.YAML)); e != nil {
		return e
	}
	s.pushChanged(a.Private.Settings.AndroidPush)
	if s.options.Activate != nil {
		if e := s.options.Activate(a.Bundle.OwnedState); e != nil {
			return e
		}
	}
	a.ActivationComplete = true
	return s.saveAuthority()
}

// InactiveConfigPath is retained for explicit operator recovery; never merged.
func (s *Service) InactiveConfigPath() string { return s.options.ConfigPath + ".inactive" }

const androidPushCredentialsUnavailable = "AndroidPushCredentialsUnavailable"
const androidPushCredentialsAction = "android push credentials unavailable on destination; install matching android-push-credentials.json there and apply, or disable android_push"

func (s *Service) validateHub(push *hostconfig.AndroidPush) error {
	if push == nil {
		return nil
	}
	if s.options.ValidateHub != nil {
		value := *push
		if s.options.ValidateHub(&value) == nil {
			return nil
		}
	}
	return &Error{Code: androidPushCredentialsUnavailable, Message: androidPushCredentialsAction, Status: 503}
}
