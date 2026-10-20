package fleet

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var encoding = base64.RawURLEncoding.Strict()

func Fingerprint(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:])
}
func ParsePublicKey(value string) (ed25519.PublicKey, error) {
	b, e := encoding.DecodeString(value)
	if e != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("invalid fleet public key")
	}
	return ed25519.PublicKey(b), nil
}
func Sign(key ed25519.PrivateKey, kind string, value any) (Envelope, error) {
	payload, e := json.Marshal(value)
	if e != nil {
		return Envelope{}, e
	}
	if len(payload) > MaximumStateBytes {
		return Envelope{}, errors.New("fleet state too large")
	}
	return Envelope{Payload: encoding.EncodeToString(payload), Signature: encoding.EncodeToString(ed25519.Sign(key, append([]byte("skidbladnir."+kind+".v1\x00"), payload...)))}, nil
}
func Verify(key ed25519.PublicKey, kind string, envelope Envelope, target any) error {
	if len(envelope.Payload) > encoding.EncodedLen(MaximumStateBytes) {
		return errors.New("fleet state too large")
	}
	payload, e := encoding.DecodeString(envelope.Payload)
	if e != nil {
		return errors.New("invalid signed fleet payload")
	}
	signature, e := encoding.DecodeString(envelope.Signature)
	if e != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, append([]byte("skidbladnir."+kind+".v1\x00"), payload...), signature) {
		return errors.New("invalid fleet signature")
	}
	return strictjson.Decode(payload, target)
}
func VerifyPublic(key ed25519.PublicKey, envelope Envelope) (Public, error) {
	var p Public
	if e := Verify(key, "fleet", envelope, &p); e != nil {
		return p, e
	}
	if p.Schema != 1 || p.Kind != "fleet" || p.FleetID != Fingerprint(key) || p.AuthorityEpoch == 0 || p.StateRevision == 0 || p.ConfigRevision == 0 || len(p.Machines) == 0 {
		return p, errors.New("invalid public fleet state")
	}
	if p.AndroidPush != nil {
		config := p.AndroidPush.config()
		if e := config.Validate(); e != nil || p.AndroidPush.ID != config.ID() {
			return p, errors.New("invalid public android push configuration")
		}
	}
	names := map[string]bool{}
	handles := map[string]bool{}
	origins := map[string]bool{}
	hub := false
	for _, m := range p.Machines {
		if _, e := machine.Parse(m.Machine); e != nil || !machine.ValidName(m.Name) || names[strings.ToLower(m.Name)] || handles[m.Machine] || m.Origin != "" && (!machine.ValidOrigin(m.Origin) || origins[m.Origin]) {
			return p, errors.New("invalid fleet machine")
		}
		names[strings.ToLower(m.Name)], handles[m.Machine] = true, true
		if m.Origin != "" {
			origins[m.Origin] = true
		}
		hub = hub || m.Machine == p.ActiveHub
	}
	if !hub {
		return p, errors.New("active hub is not enrolled")
	}
	return p, nil
}
func VerifyHostState(key ed25519.PublicKey, state HostState) (Public, Private, error) {
	p, e := VerifyPublic(key, state.Public)
	if e != nil {
		return p, Private{}, e
	}
	var h Private
	if e := Verify(key, "host", state.Private, &h); e != nil {
		return p, h, e
	}
	if h.Schema != 1 || h.Kind != "host" || h.FleetID != p.FleetID || h.AuthorityEpoch != p.AuthorityEpoch || h.StateRevision != p.StateRevision || h.ConfigRevision != p.ConfigRevision || h.Members == nil || h.Retired == nil {
		return p, h, errors.New("incoherent host state")
	}
	// Round-trip through the sole config decoder establishes the signed settings contract.
	yaml, e := json.Marshal(h.Settings)
	if e != nil {
		return p, h, e
	}
	if _, e = hostconfig.Parse(yaml); e != nil {
		return p, h, fmt.Errorf("invalid signed settings: %w", e)
	}
	if e := coherentAndroidPush(p, h); e != nil {
		return p, h, e
	}
	verifiers := map[string]bool{}
	members := map[string]bool{}
	computers := map[string]bool{}
	for _, m := range h.Members {
		if e := validMember(m); e != nil || members[m.DeviceID] || verifiers[m.Verifier] {
			return p, h, errors.New("invalid fleet member")
		}
		members[m.DeviceID] = true
		verifiers[m.Verifier] = true
		if m.Kind == "computer" {
			if computers[m.Machine] {
				return p, h, errors.New("duplicate computer membership")
			}
			computers[m.Machine] = true
		}
	}
	if len(computers) != len(p.Machines) {
		return p, h, errors.New("computer membership has no routing entry")
	}
	for _, m := range p.Machines {
		if !computers[m.Machine] {
			return p, h, errors.New("routing machine has no membership")
		}
	}
	for _, r := range h.Retired {
		m := Member{r.DeviceID, r.Kind, r.Machine, r.Verifier}
		if e := validMember(m); e != nil || members[m.DeviceID] || verifiers[m.Verifier] {
			return p, h, errors.New("invalid retired member")
		}
		members[m.DeviceID] = true
		verifiers[m.Verifier] = true
		var removal Removal
		if e := Verify(key, "member-removed", r.Removal, &removal); e != nil || removal.Schema != 1 || removal.Kind != "member-removed" || removal.FleetID != p.FleetID || removal.DeviceID != r.DeviceID || removal.AuthorityEpoch > p.AuthorityEpoch || removal.StateRevision > p.StateRevision {
			return p, h, errors.New("invalid removal proof")
		}
	}
	return p, h, nil
}
func validMember(m Member) error {
	if !validDeviceID(m.DeviceID) || !auth.ValidVerifier(m.Verifier) {
		return errors.New("invalid device identity")
	}
	if m.Kind == "phone" && m.Machine == "" {
		return nil
	}
	if m.Kind == "computer" {
		_, e := machine.Parse(m.Machine)
		return e
	}
	return errors.New("invalid device kind")
}
func validDeviceID(value string) bool {
	if !strings.HasPrefix(value, "d-") || len(value) != 34 {
		return false
	}
	_, e := hex.DecodeString(value[2:])
	return e == nil && strings.ToLower(value) == value
}
func newer(next, old Public) error {
	if next.FleetID != old.FleetID {
		return errors.New("wrong fleet")
	}
	if next.AuthorityEpoch == old.AuthorityEpoch && next.ActiveHub != old.ActiveHub {
		return errors.New("hub changed without authority epoch")
	}
	if next.AuthorityEpoch < old.AuthorityEpoch || next.StateRevision < old.StateRevision || next.ConfigRevision < old.ConfigRevision {
		return errors.New("fleet state regressed")
	}
	return nil
}

func coherentAndroidPush(p Public, h Private) error {
	expected := publicAndroidPush(h.Settings.AndroidPush)
	if expected == nil && p.AndroidPush == nil {
		return nil
	}
	if expected == nil || p.AndroidPush == nil || *expected != *p.AndroidPush {
		return errors.New("incoherent android push configuration")
	}
	return h.Settings.AndroidPush.Validate()
}
