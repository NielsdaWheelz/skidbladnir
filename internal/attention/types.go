// Package attention owns producer evidence and device-local acknowledgement.
package attention

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const (
	MaximumSnapshotBytes     = 1024 * 1024
	MaximumHintBytes         = 256
	MaximumRegistrationBytes = 4096
)

var ErrUnavailable = errors.New("notifications unavailable")
var ErrEpochChanged = errors.New("notification memory reset required")

type Slot struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
}

type Key struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

func (key Key) Slot() Slot { return Slot{key.Machine, key.TmuxID, key.IdentityToken} }
func (slot Slot) Identifier() string {
	data, err := json.Marshal(slot)
	if err != nil {
		panic("encode attention slot")
	} // justify-defect: slot string fields always encode.
	return base64.RawURLEncoding.EncodeToString(data)
}
func (key Key) Valid() bool {
	_, err := machine.Parse(key.Machine)
	return err == nil && address(key.TmuxID, '$') && address(key.PaneID, '%') && key.IdentityToken != "" && utf8.ValidString(key.IdentityToken)
}
func address(value string, prefix byte) bool {
	return len(value) > 1 && value[0] == prefix && strings.Trim(value[1:], "0123456789") == ""
}

type Foreground struct {
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

func (foreground Foreground) Valid() bool {
	return (foreground.Provider == "Codex" || foreground.Provider == "Claude") && foreground.PID > 0 && foreground.StartIdentity != "" && utf8.ValidString(foreground.StartIdentity)
}
func SameForeground(left, right *Foreground) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

type Phase string

const (
	Quiet Phase = "quiet"
	Armed Phase = "armed"
	Ready Phase = "ready"
)

type Cause struct {
	Kind    string               `json:"kind"`
	Request sessions.Interaction `json:"request,omitempty"`
	Notice  sessions.Notice      `json:"notice,omitempty"`
}

func (cause Cause) Valid() bool {
	return cause.Kind == "ready" && cause.Request == "" && cause.Notice == "" || cause.Kind == "action" && (cause.Request == "" || requestValid(cause.Request)) && (cause.Notice == "" || cause.Notice == sessions.NoticeError || cause.Notice == sessions.NoticeInterrupted) && (cause.Request != "" || cause.Notice != "")
}
func requestValid(request sessions.Interaction) bool {
	return request.Valid() && request.Request()
}

func SameCause(left, right *Cause) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

type Record struct {
	Key              Key         `json:"key"`
	Foreground       *Foreground `json:"foreground,omitempty"`
	Phase            Phase       `json:"phase"`
	ReadyGeneration  int64       `json:"readyGeneration"`
	AttentionEpisode int64       `json:"attentionEpisode"`
	Cause            *Cause      `json:"cause,omitempty"`
}

type SessionSnapshot struct {
	Ref                string                  `json:"ref"`
	Name               string                  `json:"name"`
	TerminalStatus     sessions.TerminalStatus `json:"terminalStatus"`
	AttentionQualified bool                    `json:"attentionQualified"`
	Record             Record                  `json:"record"`
}

type MachineSnapshot struct {
	Machine      string            `json:"machine"`
	Availability string            `json:"availability"`
	ObservedAt   string            `json:"observedAt,omitempty"`
	Sessions     []SessionSnapshot `json:"sessions,omitempty"`
}

func (snapshot MachineSnapshot) MarshalJSON() ([]byte, error) {
	if snapshot.Availability == "gap" {
		return json.Marshal(struct {
			Machine      string `json:"machine"`
			Availability string `json:"availability"`
		}{snapshot.Machine, snapshot.Availability})
	}
	return json.Marshal(struct {
		Machine      string            `json:"machine"`
		Availability string            `json:"availability"`
		ObservedAt   string            `json:"observedAt"`
		Sessions     []SessionSnapshot `json:"sessions"`
	}{snapshot.Machine, snapshot.Availability, snapshot.ObservedAt, snapshot.Sessions})
}

type Snapshot struct {
	Schema      int               `json:"schema"`
	Epoch       string            `json:"epoch"`
	Revision    int64             `json:"revision"`
	ReceiverTag string            `json:"receiverTag"`
	Machines    []MachineSnapshot `json:"machines"`
}

func (snapshot Snapshot) Session(key Key) (SessionSnapshot, bool) {
	for _, host := range snapshot.Machines {
		if host.Availability != "fresh" || host.Machine != key.Machine {
			continue
		}
		for _, row := range host.Sessions {
			if row.Record.Key == key {
				return row, true
			}
		}
	}
	return SessionSnapshot{}, false
}

type Hint struct {
	Schema   int    `json:"schema"`
	Epoch    string `json:"epoch"`
	Revision int64  `json:"revision"`
}
type SubscriptionKeys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}
type Subscription struct {
	Endpoint string           `json:"endpoint"`
	Keys     SubscriptionKeys `json:"keys"`
}
type Config struct {
	ObserverMachine string `json:"observerMachine"`
	NtfyOrigin      string `json:"ntfyOrigin"`
}
type ReadyToken struct {
	Epoch      string `json:"epoch"`
	Generation int64  `json:"generation"`
}

type Presentation string

const (
	Claimed Presentation = "claimed"
	Posted  Presentation = "posted"
	Closed  Presentation = "closed"
)

type DeviceRecord struct {
	Key                         Key          `json:"key"`
	Foreground                  *Foreground  `json:"foreground,omitempty"`
	ReceivedReadyGeneration     int64        `json:"receivedReadyGeneration"`
	AcknowledgedReadyGeneration int64        `json:"acknowledgedReadyGeneration"`
	HandledAttentionEpisode     int64        `json:"handledAttentionEpisode"`
	Presentation                Presentation `json:"presentation"`
}
type DesiredRegistration struct {
	Generation   int64        `json:"generation"`
	Subscription Subscription `json:"subscription"`
}
type DeviceSnapshot struct {
	Schema              int                  `json:"schema"`
	ObserverEpoch       string               `json:"observerEpoch,omitempty"`
	AdmittedRevision    int64                `json:"admittedRevision"`
	LocalRevision       int64                `json:"localRevision"`
	Config              *Config              `json:"config,omitempty"`
	DesiredRegistration *DesiredRegistration `json:"desiredRegistration,omitempty"`
	Records             []DeviceRecord       `json:"records"`
}

func (snapshot DeviceSnapshot) Record(key Key) (DeviceRecord, bool) {
	for _, record := range snapshot.Records {
		if record.Key == key {
			return record, true
		}
	}
	return DeviceRecord{}, false
}
func EpochValid(epoch string) bool {
	return len(epoch) == 32 && strings.Trim(epoch, "0123456789abcdef") == ""
}
func NonIdle(status sessions.TerminalStatus) bool {
	return status.Interaction.Request() || status.Interaction == sessions.InteractionMenu || status.Activity == sessions.ActivityStarting || status.Activity == sessions.ActivityWorking
}
func Idle(status sessions.TerminalStatus) bool {
	return status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone
}
func RequestText(request sessions.Interaction) string {
	switch request {
	case sessions.InteractionPermission:
		return "needs permission"
	case sessions.InteractionQuestion:
		return "needs answer"
	case sessions.InteractionSetup:
		return "needs setup"
	case sessions.InteractionConfirmation:
		return "needs review"
	case sessions.InteractionInput:
		return "needs input"
	default:
		panic("request without content")
	} // justify-defect: only admitted request enums reach copy.
}
func NoticeText(notice sessions.Notice) string {
	switch notice {
	case sessions.NoticeInterrupted:
		return "interruption shown"
	case sessions.NoticeError:
		return "error shown"
	default:
		panic("notice without content")
	}
} // justify-defect: only admitted attention notices reach copy.
func (cause Cause) Text() string {
	if cause.Kind == "ready" {
		return "ready"
	}
	parts := []string{}
	if cause.Request != "" {
		parts = append(parts, RequestText(cause.Request))
	}
	if cause.Notice != "" {
		parts = append(parts, NoticeText(cause.Notice))
	}
	return strings.Join(parts, " · ")
}
func SingleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, text)
}

// Decode admits one strict, null-free transport shape.
func Decode(data []byte, value any) error {
	var tree any
	if strictjson.Decode(data, &tree) != nil || containsNull(tree) || strictjson.Decode(data, value) != nil {
		return ErrUnavailable
	}
	return nil
}
func containsNull(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case []any:
		for _, child := range typed {
			if containsNull(child) {
				return true
			}
		}
	case map[string]any:
		for _, child := range typed {
			if containsNull(child) {
				return true
			}
		}
	}
	return false
}
func ValidRecord(record Record, revision int64) bool {
	if !record.Key.Valid() || record.Foreground != nil && !record.Foreground.Valid() || record.ReadyGeneration < 0 || record.ReadyGeneration > revision || record.AttentionEpisode < 0 || record.AttentionEpisode > revision {
		return false
	}
	switch record.Phase {
	case Quiet:
	case Armed, Ready:
		if record.Foreground == nil {
			return false
		}
	default:
		return false
	}
	if record.Phase == Quiet && record.ReadyGeneration != 0 || record.Phase == Ready && record.ReadyGeneration == 0 || record.Cause != nil && (!record.Cause.Valid() || record.Foreground == nil || record.AttentionEpisode == 0 || record.Cause.Kind == "ready" && (record.Phase != Ready || record.AttentionEpisode < record.ReadyGeneration)) {
		return false
	}
	return true
}
func (snapshot Snapshot) Valid() bool {
	if snapshot.Schema != 1 || !EpochValid(snapshot.Epoch) || snapshot.Revision < 0 || snapshot.Revision >= math.MaxInt64 || snapshot.Machines == nil {
		return false
	}
	receiverRevision, valid := ReceiverRevision(snapshot.Epoch, snapshot.ReceiverTag)
	if !valid || receiverRevision > snapshot.Revision {
		return false
	}
	machines := map[string]bool{}
	slots := map[Slot]bool{}
	for _, host := range snapshot.Machines {
		if _, err := machine.Parse(host.Machine); err != nil || machines[host.Machine] {
			return false
		}
		machines[host.Machine] = true
		switch host.Availability {
		case "gap":
			if host.ObservedAt != "" || host.Sessions != nil {
				return false
			}
		case "fresh":
			if _, err := time.Parse(time.RFC3339Nano, host.ObservedAt); err != nil || host.Sessions == nil {
				return false
			}
		default:
			return false
		}
		for _, row := range host.Sessions {
			if row.Record.Key.Machine != host.Machine || !ValidRecord(row.Record, snapshot.Revision) || !row.TerminalStatus.Valid() || !utf8.ValidString(row.Name) || slots[row.Record.Key.Slot()] {
				return false
			}
			slots[row.Record.Key.Slot()] = true
			data, err := base64.RawURLEncoding.Strict().DecodeString(row.Ref)
			if err != nil || len(row.Ref) == 0 || len(row.Ref) > 4096 || base64.RawURLEncoding.EncodeToString(data) != row.Ref {
				return false
			}
			var key Key
			if Decode(data, &key) != nil || key != row.Record.Key {
				return false
			}
			if row.AttentionQualified && row.Record.Cause != nil && !Current(row) {
				return false
			}
		}
	}
	return true
}
func (hint Hint) Valid() bool {
	return hint.Schema == 1 && EpochValid(hint.Epoch) && hint.Revision >= 0 && hint.Revision < math.MaxInt64
}
func (snapshot DeviceSnapshot) Valid() bool {
	if snapshot.Schema != 3 || snapshot.Records == nil || snapshot.AdmittedRevision < 0 || snapshot.AdmittedRevision >= math.MaxInt64 || snapshot.LocalRevision < 0 || snapshot.LocalRevision >= math.MaxInt64 || snapshot.ObserverEpoch != "" && !EpochValid(snapshot.ObserverEpoch) || snapshot.ObserverEpoch == "" && (snapshot.AdmittedRevision != 0 || len(snapshot.Records) != 0) {
		return false
	}
	if snapshot.Config != nil && !snapshot.Config.Valid() {
		return false
	}
	if snapshot.DesiredRegistration != nil && (snapshot.Config == nil || snapshot.DesiredRegistration.Generation <= 0 || snapshot.DesiredRegistration.Generation >= math.MaxInt64 || !snapshot.DesiredRegistration.Subscription.Valid(snapshot.Config.NtfyOrigin)) {
		return false
	}
	slots := map[Slot]bool{}
	for _, record := range snapshot.Records {
		if !record.Key.Valid() || slots[record.Key.Slot()] || record.Foreground != nil && !record.Foreground.Valid() || record.ReceivedReadyGeneration < 0 || record.AcknowledgedReadyGeneration < 0 || record.AcknowledgedReadyGeneration > record.ReceivedReadyGeneration || record.HandledAttentionEpisode < 0 || record.ReceivedReadyGeneration > snapshot.AdmittedRevision || record.HandledAttentionEpisode > snapshot.AdmittedRevision || record.Foreground == nil && (record.ReceivedReadyGeneration > 0 || record.HandledAttentionEpisode > 0) {
			return false
		}
		switch record.Presentation {
		case Claimed, Posted:
			if record.HandledAttentionEpisode == 0 {
				return false
			}
		case Closed:
		default:
			return false
		}
		slots[record.Key.Slot()] = true
	}
	return true
}
func Current(row SessionSnapshot) bool {
	status := row.TerminalStatus
	record := row.Record
	if !row.AttentionQualified || record.Foreground == nil || status.Source != sessions.SourceTerminal || record.Cause == nil {
		return false
	}
	if record.Cause.Kind == "ready" {
		return record.Phase == Ready && Idle(status) && status.Notice == sessions.NoticeNone
	}
	return (record.Cause.Request == "" && !status.Interaction.Request() || record.Cause.Request == status.Interaction) && (record.Cause.Notice == "" && status.Notice != sessions.NoticeError && status.Notice != sessions.NoticeInterrupted || record.Cause.Notice == status.Notice)
}
func (snapshot DeviceSnapshot) Pending(row SessionSnapshot) bool {
	record, found := snapshot.Record(row.Record.Key)
	return found && SameForeground(record.Foreground, row.Record.Foreground) && record.ReceivedReadyGeneration > record.AcknowledgedReadyGeneration && row.Record.Phase == Ready && row.Record.ReadyGeneration == record.ReceivedReadyGeneration && row.AttentionQualified && row.TerminalStatus.Source == sessions.SourceTerminal && Idle(row.TerminalStatus)
}
