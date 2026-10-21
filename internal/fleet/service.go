package fleet

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"go.yaml.in/yaml/v3"
)

type Options struct {
	// Callbacks run under the fleet lock and must not call Service methods.
	PushChanged                                                    func(*hostconfig.AndroidPush)
	ValidateHub                                                    func(*hostconfig.AndroidPush) error
	HTTPClient                                                     *http.Client
	Directory, ConfigPath, Machine, Name, Origin, Home, LoginShell string
	Version                                                        string
	Resolve                                                        func(hostconfig.Settings) error
	CloseDevice                                                    func(string)
	DeviceRemoved                                                  func(string) error
	Recovery                                                       func(previousFleetID, newFleetID string) error
	Detach                                                         func() error
	Drain                                                          func() (map[string]json.RawMessage, error)
	Resume                                                         func() error
	Activate                                                       func(map[string]json.RawMessage) error
}
type Service struct {
	mu                 sync.Mutex
	options            Options
	identity           Identity
	authority          *authority
	state              HostState
	public             Public
	private            Private
	resolved           resolved
	changed            chan struct{}
	controlUncertain   bool
	admissionUncertain bool
	lockFile           *os.File
}
type authority struct {
	Schema             int                   `json:"schema"`
	ActivationComplete bool                  `json:"activationComplete"`
	Key                string                `json:"key"`
	Public             Public                `json:"public"`
	Private            Private               `json:"private"`
	YAML               string                `json:"yaml"`
	Invites            []inviteRecord        `json:"invites"`
	Reports            map[string]SelfReport `json:"reports"`
	Transfer           *Transfer             `json:"transfer,omitempty"`
	Activation         *Envelope             `json:"activation,omitempty"`
	Bundle             *TransferBundle       `json:"bundle,omitempty"`
	Aborted            []string              `json:"aborted"`
}
type inviteRecord struct {
	Verifier  string `json:"verifier"`
	ExpiresAt int64  `json:"expiresAt"`
}
type resolved struct {
	Schema         int                 `json:"schema"`
	Settings       hostconfig.Settings `json:"settings"`
	SourceRevision uint64              `json:"sourceRevision"`
	Digest         string              `json:"digest"`
	Application    ApplyStatus         `json:"application"`
}

type Error struct {
	Code    string
	Message string
	Status  int
	Routing *Envelope
	Removal *Envelope
}

func (e *Error) Error() string { return e.Message }
func conflict(message string) error {
	return &Error{Code: "FleetConflict", Message: message, Status: 409}
}
func Open(options Options) (service *Service, resultErr error) {
	if options.Directory == "" || options.ConfigPath == "" || !machine.ValidName(options.Name) {
		return nil, errors.New("fleet options are invalid")
	}
	if _, e := machine.Parse(options.Machine); e != nil {
		return nil, e
	}
	if options.Origin != "" && !machine.ValidOrigin(options.Origin) {
		return nil, errors.New("fleet origin must be a tailscale https origin on 8443")
	}
	if !filepath.IsAbs(options.Directory) || !filepath.IsAbs(options.ConfigPath) {
		return nil, errors.New("fleet state and config paths must be absolute")
	}
	if e := os.MkdirAll(options.Directory, 0700); e != nil {
		return nil, e
	}
	if e := os.Chmod(options.Directory, 0700); e != nil {
		return nil, e
	}

	lockFile, e := os.OpenFile(filepath.Join(options.Directory, "authority.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return nil, errors.Join(errors.New("fleet service already owns this state directory"), lockFile.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, lockFile.Close())
		}
	}()
	s := &Service{options: options, changed: make(chan struct{}), lockFile: lockFile}
	e = readPrivate(s.path("identity.json"), &s.identity)
	if errors.Is(e, os.ErrNotExist) {
		if e = s.bootstrap(nil); e != nil {
			return nil, e
		}
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if s.identity.Schema != 1 || s.identity.Machine != options.Machine || !machine.ValidName(s.identity.Name) {
		return nil, errors.New("stored fleet identity does not match installation")
	}
	s.options.Name = s.identity.Name
	if _, e := auth.ParseCredential(s.identity.LocalBearer); e != nil {
		return nil, e
	}
	if s.identity.ActiveFleet != nil {
		a := s.identity.ActiveFleet
		if _, e := auth.ParseCredential(a.Bearer); e != nil {
			return nil, e
		}
		key, e := ParsePublicKey(a.PublicKey)
		if e != nil || a.FleetID != Fingerprint(key) || !validDeviceID(a.DeviceID) {
			return nil, errors.New("stored fleet association is invalid")
		}

		e = readPrivate(s.path("host-state.json"), &s.state)
		if e == nil {
			s.public, s.private, e = VerifyHostState(key, s.state)
		}
		if e != nil {
			if s.identity.EnrollmentState == "" || filepath.Base(s.identity.EnrollmentState) != s.identity.EnrollmentState {
				return nil, e
			}
			if e = readPrivate(s.path(s.identity.EnrollmentState), &s.state); e != nil {
				return nil, e
			}
			s.public, s.private, e = VerifyHostState(key, s.state)
			if e != nil {
				return nil, e
			}
		}
		var stored authority
		e = readPrivate(s.path("authority.json"), &stored)
		if e == nil {
			if stored.Public.FleetID == a.FleetID {
				s.authority = &stored
				if e := s.validateAuthority(); e != nil {
					return nil, e
				}
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	e = readPrivate(s.path("resolved.json"), &s.resolved)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	if e = s.completeRecovery(); e != nil {
		return nil, e
	}
	if s.identity.ActiveFleet == nil {
		if options.Detach != nil {
			if e = options.Detach(); e != nil {
				return nil, e
			}
		}
		return s, nil
	}
	if s.authority != nil && s.authority.Transfer != nil && (s.authority.Transfer.Phase == "preparing" || s.authority.Transfer.Phase == "staged" || s.authority.Transfer.Phase == "fenced") {
		return s, nil
	}

	if s.authority != nil && s.authority.Public.ActiveHub == s.options.Machine {
		if t := s.authority.Transfer; t != nil && t.Phase == "active" {
			if e = s.completeActivation(); e != nil {
				return nil, e
			}
		}
		if e = s.publish(); e != nil {
			return nil, e
		}
	}
	if e = s.install(s.state); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Service) path(name string) string { return filepath.Join(s.options.Directory, name) }
func readPrivate(path string, target any) error {
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("fleet state must be a private regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	b, e := io.ReadAll(io.LimitReader(f, MaximumStateBytes*3+1))
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	if len(b) > MaximumStateBytes*3 {
		return errors.New("fleet state too large")
	}
	return strictjson.Decode(b, target)
}
func writePrivate(path string, value any) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return privatefile.WriteFile(path, b)
}
func randomID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return prefix + hex.EncodeToString(b), nil
}
func (s *Service) bootstrap(cached *hostconfig.Config) error {
	var settings hostconfig.Config
	var configYAML []byte
	var e error
	if cached != nil {
		settings = *cached
		configYAML, e = yaml.Marshal(settings)
	} else {
		configYAML, e = os.ReadFile(s.options.ConfigPath)
		if e == nil {
			settings, e = hostconfig.Parse(configYAML)
		}
	}
	if e != nil {
		return e
	}
	if e = settings.ValidateMachines([]string{s.options.Name}, nil); e != nil {
		return e
	}
	publicKey, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	device, e := randomID("d-")
	if e != nil {
		return e
	}
	credential, e := auth.NewCredential()
	if e != nil {
		return e
	}
	local, e := auth.NewCredential()
	if e != nil {
		return e
	}
	p := Public{AndroidPush: publicAndroidPush(settings.AndroidPush), Schema: 1, Kind: "fleet", FleetID: Fingerprint(publicKey), AuthorityEpoch: 1, StateRevision: 1, ConfigRevision: 1, ActiveHub: s.options.Machine, Machines: []Machine{{s.options.Machine, s.options.Name, s.options.Origin}}}
	h := Private{Schema: 1, Kind: "host", FleetID: p.FleetID, AuthorityEpoch: 1, StateRevision: 1, ConfigRevision: 1, Settings: settings, Members: []Member{{device, "computer", s.options.Machine, credential.Verifier()}}, Retired: []Retired{}}
	a := authority{Schema: 1, Key: encoding.EncodeToString(key), Public: p, Private: h, YAML: string(configYAML), Invites: []inviteRecord{}, Reports: map[string]SelfReport{}, Aborted: []string{}}
	// Build and stage the immutable adoption anchor before replacing authority.
	// A failed anchor must leave the previous durable signing authority intact.

	identity := Identity{Name: s.options.Name, Schema: 1, Machine: s.options.Machine, LocalBearer: local.CanonicalBearer(), ActiveFleet: &Association{p.FleetID, encoding.EncodeToString(publicKey), device, credential.CanonicalBearer()}}
	if cached != nil {
		previous := ""
		if s.identity.ActiveFleet != nil {
			previous = s.identity.ActiveFleet.FleetID
		}
		identity.Recovery = &RecoveryState{previous, p.FleetID}
	}

	pub, e := Sign(key, "fleet", a.Public)
	if e != nil {
		return e
	}
	priv, e := Sign(key, "host", a.Private)
	if e != nil {
		return e
	}
	state := HostState{pub, priv}
	filename, e := s.stageEnrollmentState(state)
	if e != nil {
		return e
	}

	identity.EnrollmentState = filename
	s.authority = &a
	if e = s.saveAuthority(); e != nil {
		return e
	}
	// Authority is durable before identity advertises the staged trust anchor.
	if e = s.storeIdentity(identity, true); e != nil {
		return e
	}
	admittedPublic, admittedPrivate, e := cloneState(state)
	if e != nil {
		return e
	}
	s.state, s.public, s.private = state, admittedPublic, admittedPrivate
	if e = s.completeRecovery(); e != nil {
		return e
	}
	if e = s.publish(); e != nil {
		return e
	}
	return s.install(state)
}
func (s *Service) validateAuthority() error {
	a := s.authority
	if a.Schema != 1 || a.Reports == nil || a.Invites == nil || a.Aborted == nil {
		return errors.New("invalid authority state")
	}
	key, e := encoding.DecodeString(a.Key)
	if e != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("invalid authority key")
	}
	if Fingerprint(ed25519.PublicKey(key[32:])) != a.Public.FleetID {
		return errors.New("authority key mismatch")
	}
	if a.Transfer != nil && a.Transfer.Phase != "preparing" && a.Transfer.Phase != "staged" && a.Transfer.Phase != "fenced" && a.Transfer.Phase != "active" && a.Transfer.Phase != "aborted" {
		return errors.New("invalid transfer phase")
	}
	return coherentAndroidPush(a.Public, a.Private)
}
func (s *Service) saveAuthority() error {
	if e := writePrivate(s.path("authority.json"), s.authority); e != nil {
		return s.fenceControl(e)
	}
	s.controlUncertain = false
	return nil
}
func (s *Service) key() (ed25519.PrivateKey, error) {
	if s.authority == nil {
		return nil, conflict("hub unavailable; contact the active hub")
	}
	b, e := encoding.DecodeString(s.authority.Key)
	if e != nil || len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid stored signing key")
	}
	return ed25519.PrivateKey(b), nil
}
func (s *Service) active() error {
	if s.controlUncertain || s.admissionUncertain {
		return conflict("control write uncertain; restart the service to reconcile durable state")
	}
	if s.authority == nil || s.identity.ActiveFleet == nil || s.authority.Public.ActiveHub != s.options.Machine || s.authority.Public.FleetID != s.identity.ActiveFleet.FleetID || s.public.ActiveHub != s.options.Machine {
		return s.wrongHub()
	}
	if s.identity.Recovery != nil {
		return conflict("new fleet recovery pending; restart to finish observer reset")
	}
	if t := s.authority.Transfer; t != nil && t.Phase != "active" && t.Phase != "aborted" {
		return conflict("hub transfer pending; retry the same transfer")
	}
	if s.authority.Transfer != nil && s.authority.Transfer.Phase == "active" && !s.authority.ActivationComplete {
		return conflict("hub activation incomplete; retry the same activation")
	}

	return nil
}
func (s *Service) wrongHub() error {
	routing := s.state.Public
	return &Error{Code: "WrongHub", Message: "request the active hub using verified fleet routing", Status: 409, Routing: &routing}
}

// IsHub reports whether this process currently owns writable hub authority.
func (s *Service) IsHub() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active() == nil
}
func (s *Service) publish() error {
	a := s.authority
	key, e := s.key()
	if e != nil {
		return e
	}
	pub, e := Sign(key, "fleet", a.Public)
	if e != nil {
		return e
	}
	priv, e := Sign(key, "host", a.Private)
	if e != nil {
		return e
	}
	state := HostState{pub, priv}

	admittedPublic, admittedPrivate, e := cloneState(state)
	if e != nil {
		return e
	}
	s.state, s.public, s.private = state, admittedPublic, admittedPrivate
	s.notify()
	return errors.Join(writePrivate(s.path("host-state.json"), state), writePrivate(s.path("fleet.json"), pub))
}
func (s *Service) notify() { close(s.changed); s.changed = make(chan struct{}) }
func (s *Service) Identity() Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.identity)
	var i Identity
	_ = json.Unmarshal(b, &i)
	return i
}
func (s *Service) Snapshot() (HostState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.admissionError(); e != nil {
		return HostState{}, e
	}

	if s.identity.ActiveFleet == nil {
		return HostState{}, conflict("local mode; join a fleet or run recover")
	}
	return s.state, nil
}
func (s *Service) Settings() (hostconfig.Settings, ApplyStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.resolved.Settings)
	var settings hostconfig.Settings
	_ = json.Unmarshal(b, &settings)
	return settings, s.resolved.Application
}
func (s *Service) Status() Status { s.mu.Lock(); defer s.mu.Unlock(); return s.status() }
func (s *Service) status() Status {
	if s.identity.ActiveFleet == nil {
		return Status{Version: s.options.Version, Members: []Member{}, Applications: []Application{}, PendingRevocation: []string{}, Message: "local mode; join a fleet or recover"}
	}
	p := s.public
	result := Status{Version: s.options.Version, FleetID: p.FleetID, ActiveHub: p.ActiveHub, AuthorityEpoch: p.AuthorityEpoch, StateRevision: p.StateRevision, ConfigRevision: p.ConfigRevision, Members: append([]Member{}, s.private.Members...), Applications: []Application{}, PendingRevocation: []string{}}
	// Verifiers are private authorization material, never status presentation.
	for i := range result.Members {
		result.Members[i].Verifier = ""
	}
	result.DesiredHub = s.private.Settings.Hub
	if result.DesiredHub == "" {
		for _, m := range p.Machines {
			if m.Machine == p.ActiveHub {
				result.DesiredHub = m.Name
			}
		}
	}
	for _, member := range s.private.Members {
		if member.Kind != "computer" {
			continue
		}
		name := ""
		for _, m := range p.Machines {
			if m.Machine == member.Machine {
				name = m.Name
			}
		}
		report := SelfReport{ApplyStatus: ApplyStatus{AuthorityEpoch: p.AuthorityEpoch, DesiredConfigRevision: p.ConfigRevision, Status: "pending_offline"}}
		if s.authority != nil {
			if v, ok := s.authority.Reports[member.DeviceID]; ok {
				report = v
			}
		}
		if member.Machine == s.options.Machine {
			report.ApplyStatus = s.resolved.Application
		}
		result.Applications = append(result.Applications, Application{member.DeviceID, name, report})
		if len(s.private.Retired) > 0 && (report.ApplyStatus.AuthorityEpoch < p.AuthorityEpoch || report.ApplyStatus.ReceivedStateRevision < p.StateRevision) {
			result.PendingRevocation = append(result.PendingRevocation, name)
		}
	}
	if s.authority != nil && s.authority.Transfer != nil {
		copy := *s.authority.Transfer
		result.Transfer = &copy
	}
	return result
}
func (s *Service) Apply() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return s.status(), e
	}
	b, e := os.ReadFile(s.options.ConfigPath)
	if e != nil {
		return s.status(), e
	}
	c, e := hostconfig.Parse(b)
	if e != nil {
		return s.status(), &Error{Code: "ConfigInvalid", Message: e.Error(), Status: 400}
	}
	active, retired := s.names()
	if e = c.ValidateMachines(active, retired); e != nil {
		return s.status(), &Error{Code: "ConfigInvalid", Message: e.Error(), Status: 400}
	}
	a := s.authority
	if string(a.YAML) != string(b) {

		previous, _ := json.Marshal(a.Private.Settings)
		next, _ := json.Marshal(c)
		a.YAML = string(b)
		if string(previous) != string(next) {
			a.Private.Settings = c
			a.Public.StateRevision++
			a.Public.ConfigRevision++
			s.syncRevisions()
		}

		if e = s.saveAuthority(); e != nil {
			return s.status(), e
		}
		if e = s.publish(); e != nil {
			return s.status(), e
		}
	}
	if e = s.install(s.state); e != nil {
		return s.status(), e
	}
	return s.status(), nil
}
func (s *Service) syncRevisions() {
	a := s.authority
	a.Public.AndroidPush = publicAndroidPush(a.Private.Settings.AndroidPush)
	a.Private.AuthorityEpoch = a.Public.AuthorityEpoch
	a.Private.StateRevision = a.Public.StateRevision
	a.Private.ConfigRevision = a.Public.ConfigRevision
}
func (s *Service) names() ([]string, []string) {
	active, retired := []string{}, []string{}
	for _, m := range s.public.Machines {
		active = append(active, m.Name)
	}
	for _, m := range s.private.Retired {
		if m.Name != "" {
			retired = append(retired, m.Name)
		}
	}
	return active, retired
}
func (s *Service) Receive(state HostState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.admissionError(); e != nil {
		return e
	}

	return s.install(state)
}
func (s *Service) install(state HostState) error {
	association := s.identity.ActiveFleet
	if association == nil {
		return conflict("local mode rejects old fleet state")
	}
	key, e := ParsePublicKey(association.PublicKey)
	if e != nil {
		return e
	}
	p, h, e := VerifyHostState(key, state)
	if e != nil {
		return e
	}
	if s.public.FleetID != "" {
		if e = newer(p, s.public); e != nil {
			return e
		}
		if p.AuthorityEpoch == s.public.AuthorityEpoch && p.StateRevision == s.public.StateRevision && (state.Public != s.state.Public || state.Private != s.state.Private) {
			return errors.New("conflicting same-revision fleet state")
		}
	}
	var self *Member
	for i := range h.Members {
		if h.Members[i].DeviceID == association.DeviceID {
			self = &h.Members[i]
		}
	}
	if self == nil {
		for _, r := range h.Retired {
			if r.DeviceID == association.DeviceID {
				return s.detach()
			}
		}
		return errors.New("current device missing from signed membership")
	}
	credential, e := auth.ParseCredential(association.Bearer)
	if e != nil || self.Verifier != credential.Verifier() || self.Machine != s.options.Machine {
		return errors.New("signed membership does not match this computer")
	}
	previous := s.private.Members
	// Membership is installed before launch settings are resolved. Revocation cannot
	// be blocked by an unavailable shell or an invalid host-local path.
	if e = writePrivate(s.path("host-state.json"), state); e != nil {
		return s.fenceAdmission(e)
	}

	controlChanged := s.public.AuthorityEpoch != p.AuthorityEpoch || s.public.StateRevision != p.StateRevision
	s.state, s.public, s.private = state, p, h
	for _, old := range previous {
		found := false
		for _, now := range h.Members {
			found = found || old.DeviceID == now.DeviceID
		}
		if !found && s.options.CloseDevice != nil {
			s.options.CloseDevice(old.DeviceID)
		}
	}
	if controlChanged {
		s.notify()
	}
	s.pushChanged(h.Settings.AndroidPush)
	name := s.options.Name
	for _, m := range p.Machines {
		if m.Machine == s.options.Machine {
			name = m.Name
		}
	}
	applied := s.resolved.SourceRevision
	application := ApplyStatus{AuthorityEpoch: p.AuthorityEpoch, ReceivedStateRevision: p.StateRevision, DesiredConfigRevision: p.ConfigRevision, AppliedConfigRevision: applied, Status: "applied"}
	local, e := hostconfig.Resolve(h.Settings, name, s.options.Home, s.options.LoginShell)
	if e == nil && s.options.Resolve != nil {
		e = s.options.Resolve(local)
	}
	if e != nil {
		application.Status = "invalid_host"
		application.Reason = "settings unavailable on this host; fix its shell or local paths"
		s.resolved.Application = application
		return writePrivate(s.path("resolved.json"), s.resolved)
	}
	active := map[string]bool{}
	for _, m := range p.Machines {
		active[strings.ToLower(m.Name)] = true
	}
	if !active[strings.ToLower(local.DefaultMachine)] {
		local.DefaultMachine = name
	}
	data, e := json.Marshal(local)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(data)
	application.AppliedConfigRevision = p.ConfigRevision
	s.resolved = resolved{1, local, p.ConfigRevision, hex.EncodeToString(sum[:]), application}
	return writePrivate(s.path("resolved.json"), s.resolved)
}
func (s *Service) detach() error {
	local, e := auth.NewCredential()
	if e != nil {
		return e
	}
	oldLocal, e := auth.ParseCredential(s.identity.LocalBearer)
	if e != nil {
		return e
	}
	identity := s.identity
	identity.LocalBearer = local.CanonicalBearer()
	identity.ActiveFleet = nil
	identity.Pending = nil
	if s.options.Drain != nil {
		if _, e = s.options.Drain(); e != nil {
			return e
		}
	}
	if e = s.storeIdentity(identity, true); e != nil {
		return e
	}
	s.identity = identity
	s.authority = nil
	if s.options.CloseDevice != nil {
		s.options.CloseDevice("local-" + oldLocal.Verifier())
		for _, member := range s.private.Members {
			s.options.CloseDevice(member.DeviceID)
		}
	}
	s.notify()
	if s.options.Detach != nil {
		return s.options.Detach()
	}
	return nil
}
func (s *Service) Recover() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.admissionError(); e != nil {
		return s.status(), e
	}

	if s.identity.Recovery != nil {
		if e := s.completeRecovery(); e != nil {
			return s.status(), e
		}
		if e := s.publish(); e != nil {
			return s.status(), e
		}
		return s.status(), s.install(s.state)
	}
	c := s.private.Settings
	if c.Version == 0 {
		c = hostconfig.Config{Version: 1, Profiles: s.resolved.Settings.Profiles, Shell: s.resolved.Settings.Shell, Terminal: s.resolved.Settings.Terminal}
	}
	c.Hub = s.options.Name
	c.Machines = nil
	c.DefaultMachine = s.options.Name
	if s.options.Drain != nil {
		if _, e := s.options.Drain(); e != nil {
			return s.status(), e
		}
	}
	if s.options.CloseDevice != nil {
		for _, m := range s.private.Members {
			s.options.CloseDevice(m.DeviceID)
		}
		oldLocal, e := auth.ParseCredential(s.identity.LocalBearer)
		if e != nil {
			return s.status(), e
		}
		s.options.CloseDevice("local-" + oldLocal.Verifier())
	}
	if e := s.bootstrap(&c); e != nil {
		return s.status(), s.fenceControl(e)
	}
	return s.status(), nil
}
func (s *Service) Report(member Member, report SelfReport) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.active(); e != nil {
		return s.status(), e
	}
	if member.Kind != "computer" {
		return s.status(), &Error{Code: "Forbidden", Message: "computer membership required", Status: 403}
	}
	if strings.HasPrefix(member.DeviceID, "local-") && member.Machine == s.options.Machine && s.identity.ActiveFleet != nil {
		member.DeviceID = s.identity.ActiveFleet.DeviceID
	}
	found := false
	for _, m := range s.private.Members {
		if m.DeviceID == member.DeviceID && m.Machine == member.Machine && m.Kind == "computer" {
			found = true
		}
	}
	if !found {
		return s.status(), &Error{Code: "Forbidden", Message: "current computer membership required", Status: 403}
	}

	if report.Origin != "" && !machine.ValidOrigin(report.Origin) || report.ApplyStatus.Status != "applied" && report.ApplyStatus.Status != "invalid_host" && report.ApplyStatus.Status != "pending_offline" {
		return s.status(), &Error{Code: "InvalidRequest", Message: "invalid host report", Status: 400}
	}
	a := s.authority
	if report.ApplyStatus.AuthorityEpoch > a.Public.AuthorityEpoch || report.ApplyStatus.ReceivedStateRevision > a.Public.StateRevision || report.ApplyStatus.AppliedConfigRevision > report.ApplyStatus.DesiredConfigRevision || report.ApplyStatus.DesiredConfigRevision > a.Public.ConfigRevision {
		return s.status(), conflict("host report exceeds published state")
	}
	changed := false
	for i := range a.Public.Machines {
		m := &a.Public.Machines[i]
		// An empty report has no new network evidence; it does not withdraw a known origin.
		if report.Origin != "" && m.Machine == member.Machine && m.Origin != report.Origin {
			for _, other := range a.Public.Machines {
				if other.Machine != m.Machine && other.Origin == report.Origin {
					return s.status(), conflict("origin already belongs to another computer")
				}
			}
			m.Origin = report.Origin
			changed = true
		}
	}
	a.Reports[member.DeviceID] = report
	if changed {
		a.Public.StateRevision++
		s.syncRevisions()
	}
	if e := s.saveAuthority(); e != nil {
		return s.status(), e
	}
	if changed {
		if e := s.publish(); e != nil {
			return s.status(), e
		}
	}
	return s.status(), nil
}

// stageEnrollmentState writes an immutable adoption anchor before the identity
// commit. Cache publication can then be completed after a crash without guessing.
func (s *Service) stageEnrollmentState(state HostState) (string, error) {
	b, e := json.Marshal(state)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	name := "enrollment-" + hex.EncodeToString(sum[:]) + ".json"
	if e = privatefile.WriteFile(s.path(name), b); e != nil {
		return "", e
	}
	return name, nil
}

// Close releases the single-writer lease after HTTP and Run have been stopped.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lockFile == nil {
		return nil
	}
	e := s.lockFile.Close()
	s.lockFile = nil
	return e
}

func (s *Service) pushChanged(push *hostconfig.AndroidPush) {
	if s.options.PushChanged == nil {
		return
	}
	if push != nil {
		value := *push
		push = &value
	}
	s.options.PushChanged(push)
}
