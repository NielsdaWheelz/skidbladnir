// Package fleet owns this installation's fleet: its identity file, membership,
// the fixed hub's authority, the fleet routes and the member's copy loop.
//
// The hub is the computer that created the fleet (bootstrap or recover). It is the
// one writer of fleet state and serves it as plain json to authenticated members;
// members copy it. Identity and hub role are constants of one serve process: a
// committed trust change or a failed write stops the service, and its supervisor
// restarts it from fleet.json.
package fleet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

const (
	LocalAddress = "127.0.0.1:7341"
	LocalOrigin  = "http://" + LocalAddress

	maximumStateBytes = 4 << 20
)

type Machine struct {
	Machine string `json:"machine"`
	Name    string `json:"name"`
	Origin  string `json:"origin,omitempty"`
}
type Member struct {
	DeviceID string `json:"deviceId"`
	Kind     string `json:"kind"` // computer | phone
	Machine  string `json:"machine,omitempty"`
	Verifier string `json:"verifier,omitempty"` // cleared in Status
}

// AndroidPush is the public firebase client configuration the phone reads.
type AndroidPush struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	ApplicationID string `json:"applicationId"`
	SenderID      string `json:"senderId"`
	APIKey        string `json:"apiKey"`
}
type Invitation struct {
	FleetID   string    `json:"fleetId"`
	Hub       Machine   `json:"hub"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type ApplyStatus struct {
	DesiredConfigRevision uint64 `json:"desiredConfigRevision"`
	AppliedConfigRevision uint64 `json:"appliedConfigRevision"`
	Status                string `json:"status"` // applied | invalid_host | pending_offline
	Reason                string `json:"reason,omitempty"`
}
type SelfReport struct {
	Origin      string      `json:"origin,omitempty"`
	Version     string      `json:"version"`
	ApplyStatus ApplyStatus `json:"application"`
}
type Application struct {
	DeviceID   string     `json:"deviceId"`
	Name       string     `json:"name"`
	SelfReport SelfReport `json:"report"`
}
type Status struct {
	Version        string        `json:"version"`
	FleetID        string        `json:"fleetId"`
	Hub            string        `json:"hub"`
	StateRevision  uint64        `json:"stateRevision"`
	ConfigRevision uint64        `json:"configRevision"`
	Members        []Member      `json:"members"`
	Applications   []Application `json:"applications"`
	Message        string        `json:"message,omitempty"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func conflict(message string) error {
	return &Error{Code: "FleetConflict", Message: message, Status: 409}
}

// document is fleet.json, the one machine file. Authority is present only on the
// hub; its presence is the hub role.
type document struct {
	Machine     string     `json:"machine"`
	Name        string     `json:"name"`
	LocalBearer string     `json:"localBearer"`
	Fleet       membership `json:"fleet"`
	Applied     applied    `json:"applied"`
	Authority   *authority `json:"authority,omitempty"`
}
type membership struct {
	DeviceID string `json:"deviceId"`
	Bearer   string `json:"bearer"`
	State    state  `json:"state"`
}

// applied is history, not a cache: a host keeps its last launchable settings when
// newly accepted ones do not resolve locally.
type applied struct {
	Settings    hostconfig.Settings `json:"settings"`
	ApplyStatus ApplyStatus         `json:"application"`
}
type authority struct {
	Invites []invite              `json:"invites"`
	Reports map[string]SelfReport `json:"reports"`
}
type invite struct {
	Verifier  string `json:"verifier"`
	ExpiresAt int64  `json:"expiresAt"`
}

// state is the one wire-and-cache type: the hub's authority, the GET /v1/fleet and
// join reply, and every member's cache. Phones receive public().
type state struct {
	FleetID        string             `json:"fleetId"`
	Hub            string             `json:"hub"`
	StateRevision  uint64             `json:"stateRevision"`
	ConfigRevision uint64             `json:"configRevision"`
	Machines       []Machine          `json:"machines"` // hub first, then join order
	AndroidPush    *AndroidPush       `json:"androidPush,omitempty"`
	Members        []Member           `json:"members,omitempty"`
	Settings       *hostconfig.Config `json:"settings,omitempty"`
}

func (s state) public() state {
	s.Members, s.Settings = nil, nil
	return s
}
func publicAndroidPush(p *hostconfig.AndroidPush) *AndroidPush {
	if p == nil {
		return nil
	}
	return &AndroidPush{p.ID(), p.ProjectID, p.ApplicationID, p.SenderID, p.APIKey}
}

// Local is this installation's fleet as one value. ReadLocal returns it out of
// process; Service.Watch returns it in process. Same bytes, same projection.
type Local struct {
	Machine, Name, LocalBearer string
	FleetID, Hub, Bearer       string
	Machines                   []Machine // hub first, then join order
	Members                    []Member
	Settings                   hostconfig.Settings     // applied on this host
	Push                       *hostconfig.AndroidPush // accepted android push options
}
type Target struct{ Origin, Machine, Bearer string }

// Target is the one routing rule: self -> LocalOrigin + local bearer; any other
// machine -> its origin (possibly "") + this host's fleet bearer.
func (l Local) Target(handle string) Target {
	if handle == l.Machine {
		return Target{LocalOrigin, handle, l.LocalBearer}
	}
	for _, m := range l.Machines {
		if m.Machine == handle {
			return Target{m.Origin, handle, l.Bearer}
		}
	}
	return Target{"", handle, l.Bearer}
}

// ReadLocal decodes fleet.json tolerantly. serve is its only writer.
func ReadLocal(directory string) (Local, error) {
	data, err := privatefile.Read(filepath.Join(directory, "fleet.json"), maximumStateBytes)
	if err != nil {
		return Local{}, err
	}
	var d document
	if err := json.Unmarshal(data, &d); err != nil {
		return Local{}, err
	}
	return d.local(), nil
}
func (d document) local() Local {
	l := Local{Machine: d.Machine, Name: d.Name, LocalBearer: d.LocalBearer, FleetID: d.Fleet.State.FleetID, Hub: d.Fleet.State.Hub, Bearer: d.Fleet.Bearer, Machines: d.Fleet.State.Machines, Members: d.Fleet.State.Members, Settings: d.Applied.Settings}
	if d.Fleet.State.Settings != nil {
		l.Push = d.Fleet.State.Settings.AndroidPush
	}
	return l
}
func ValidDeviceID(value string) bool {
	rest, ok := strings.CutPrefix(value, "d-")
	decoded, err := hex.DecodeString(rest)
	return ok && err == nil && len(decoded) == 16 && hex.EncodeToString(decoded) == rest
}

// randomID is prefix + 32 lowercase hex.
func randomID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand cannot fail
	return prefix + hex.EncodeToString(b)
}

// newBearer mints a 256-bit bearer and its sha256 verifier.
func newBearer() (bearer, verifier string) {
	credential, _ := auth.NewCredential() // crypto/rand cannot fail
	return credential.CanonicalBearer(), credential.Verifier()
}
