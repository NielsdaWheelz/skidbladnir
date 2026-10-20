// Package fleet owns one user's signed fleet configuration and membership.
package fleet

import (
	"encoding/json"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"time"
)

const MaximumStateBytes = 4 * 1024 * 1024

type Machine struct {
	Machine string `json:"machine"`
	Name    string `json:"name"`
	Origin  string `json:"origin,omitempty"`
}
type AndroidPush struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	ApplicationID string `json:"applicationId"`
	SenderID      string `json:"senderId"`
	APIKey        string `json:"apiKey"`
}

func publicAndroidPush(p *hostconfig.AndroidPush) *AndroidPush {
	if p == nil {
		return nil
	}
	return &AndroidPush{p.ID(), p.ProjectID, p.ApplicationID, p.SenderID, p.APIKey}
}
func (p AndroidPush) config() hostconfig.AndroidPush {
	return hostconfig.AndroidPush{ProjectID: p.ProjectID, ApplicationID: p.ApplicationID, SenderID: p.SenderID, APIKey: p.APIKey}
}

type Public struct {
	AndroidPush    *AndroidPush `json:"androidPush,omitempty"`
	Schema         int          `json:"schema"`
	Kind           string       `json:"kind"`
	FleetID        string       `json:"fleetId"`
	AuthorityEpoch uint64       `json:"authorityEpoch"`
	StateRevision  uint64       `json:"stateRevision"`
	ConfigRevision uint64       `json:"configRevision"`
	ActiveHub      string       `json:"activeHub"`
	Machines       []Machine    `json:"machines"`
}
type Member struct {
	DeviceID string `json:"deviceId"`
	Kind     string `json:"kind"`
	Machine  string `json:"machine,omitempty"`
	Verifier string `json:"verifier"`
}
type Retired struct {
	DeviceID string   `json:"deviceId"`
	Kind     string   `json:"kind"`
	Machine  string   `json:"machine,omitempty"`
	Name     string   `json:"name,omitempty"`
	Verifier string   `json:"verifier"`
	Removal  Envelope `json:"removal"`
}
type Private struct {
	Schema         int               `json:"schema"`
	Kind           string            `json:"kind"`
	FleetID        string            `json:"fleetId"`
	AuthorityEpoch uint64            `json:"authorityEpoch"`
	StateRevision  uint64            `json:"stateRevision"`
	ConfigRevision uint64            `json:"configRevision"`
	Settings       hostconfig.Config `json:"settings"`
	Members        []Member          `json:"members"`
	Retired        []Retired         `json:"retired"`
}
type Envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}
type HostState struct {
	Public  Envelope `json:"public"`
	Private Envelope `json:"private"`
}
type Removal struct {
	Schema         int    `json:"schema"`
	Kind           string `json:"kind"`
	FleetID        string `json:"fleetId"`
	DeviceID       string `json:"deviceId"`
	AuthorityEpoch uint64 `json:"authorityEpoch"`
	StateRevision  uint64 `json:"stateRevision"`
}
type Association struct {
	FleetID   string `json:"fleetId"`
	PublicKey string `json:"publicKey"`
	DeviceID  string `json:"deviceId"`
	Bearer    string `json:"bearer"`
}
type PendingEnrollment struct {
	Association Association `json:"association"`
	Invitation  Invitation  `json:"invitation"`
	Request     JoinRequest `json:"request"`
}
type RecoveryState struct {
	PreviousFleetID string `json:"previousFleetId"`
	FleetID         string `json:"fleetId"`
}
type Identity struct {
	Recovery        *RecoveryState     `json:"recovery,omitempty"`
	Name            string             `json:"name"`
	EnrollmentState string             `json:"enrollmentState"`
	Schema          int                `json:"schema"`
	Machine         string             `json:"machine"`
	LocalBearer     string             `json:"localBearer"`
	ActiveFleet     *Association       `json:"activeFleet"`
	Pending         *PendingEnrollment `json:"pending,omitempty"`
}
type InvitationClaims struct {
	Schema    int       `json:"schema"`
	Kind      string    `json:"kind"`
	FleetID   string    `json:"fleetId"`
	Hub       Machine   `json:"hub"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type Invitation struct {
	Proof     Envelope  `json:"proof"`
	Schema    int       `json:"schema"`
	FleetID   string    `json:"fleetId"`
	PublicKey string    `json:"publicKey"`
	Hub       Machine   `json:"hub"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type JoinRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Machine  string `json:"machine,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Verifier string `json:"verifier"`
}
type ApplyStatus struct {
	AuthorityEpoch        uint64 `json:"authorityEpoch"`
	ReceivedStateRevision uint64 `json:"receivedStateRevision"`
	DesiredConfigRevision uint64 `json:"desiredConfigRevision"`
	AppliedConfigRevision uint64 `json:"appliedConfigRevision"`
	Status                string `json:"status"`
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
	Version           string        `json:"version"`
	FleetID           string        `json:"fleetId,omitempty"`
	ActiveHub         string        `json:"activeHub,omitempty"`
	DesiredHub        string        `json:"desiredHub,omitempty"`
	AuthorityEpoch    uint64        `json:"authorityEpoch"`
	StateRevision     uint64        `json:"stateRevision"`
	ConfigRevision    uint64        `json:"configRevision"`
	Members           []Member      `json:"members"`
	Applications      []Application `json:"applications"`
	PendingRevocation []string      `json:"pendingRevocation"`
	Message           string        `json:"message,omitempty"`
	Transfer          *Transfer     `json:"transfer,omitempty"`
}
type Transfer struct {
	TransferID         string `json:"transferId"`
	Source             string `json:"source"`
	Target             string `json:"target"`
	NextAuthorityEpoch uint64 `json:"nextAuthorityEpoch"`
	BundleDigest       string `json:"bundleDigest"`
	Phase              string `json:"phase"`
}
type TransferBundle struct {
	Transfer   Transfer                   `json:"transfer"`
	Authority  json.RawMessage            `json:"authority"`
	OwnedState map[string]json.RawMessage `json:"ownedState"`
}
type Activation struct {
	Schema   int      `json:"schema"`
	Kind     string   `json:"kind"`
	FleetID  string   `json:"fleetId"`
	Transfer Transfer `json:"transfer"`
}
