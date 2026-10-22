package fleetclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type LaunchKind string

const (
	LaunchAgent    LaunchKind = "agent"
	LaunchTerminal LaunchKind = "terminal"
)

// Request is shared by command parsing and the session browser. It is never a wire DTO.
type Request struct {
	Operation      string
	Name           string
	Handle         string
	Machine        string
	Ref            Reference // the exact captured target; zero means resolve Handle
	Kind           LaunchKind
	Profile        string
	CWD            string
	Text           string
	Keys           []string
	TerminalOnly   bool
	Explain        bool
	State          string
	WaitTimeout    time.Duration
	MaxBytes       int
	Group          group.Label
	GroupFilter    group.Filter
	ExpectedNaming *Naming
	Naming         *Naming
}

type Reference struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

func DecodeReference(encoded string) (Reference, error) {
	invalid := errors.New("invalid session reference")
	if len(encoded) == 0 || len(encoded) > 4096 {
		return Reference{}, invalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(data) != encoded {
		return Reference{}, invalid
	}
	var ref Reference
	if !nonNullJSON(data) || strictjson.Decode(data, &ref) != nil {
		return Reference{}, invalid
	}
	if _, err := machine.Parse(ref.Machine); err != nil {
		return Reference{}, invalid
	}
	if !tmuxAddress(ref.TmuxID, '$') || ref.IdentityToken == "" || !tmuxAddress(ref.PaneID, '%') {
		return Reference{}, invalid
	}
	return ref, nil
}

func (ref Reference) Encode() string {
	encoded, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// Handle names the session lifetime, independently of the selected pane.
func (ref Reference) Handle() string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode([]string{"skid-terminal-handle", ref.Machine, ref.TmuxID, ref.IdentityToken})
	digest := sha256.Sum256(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}))
	return "t-" + hex.EncodeToString(digest[:8])
}

// SessionEqual compares session lifetime, independently of pane selection and name.
func (ref Reference) SessionEqual(other Reference) bool {
	return ref.TmuxID != "" && ref.Machine == other.Machine && ref.TmuxID == other.TmuxID && ref.IdentityToken == other.IdentityToken
}

func (ref Reference) IsZero() bool { return ref == Reference{} }

// Ref is a Reference in the cli's json form: its encoded string, the value
// --ref accepts. Reference's own json is the notifier's persisted record key.
type Ref struct{ Reference }

func (ref Ref) MarshalJSON() ([]byte, error) { return json.Marshal(ref.Encode()) }

func tmuxAddress(value string, prefix byte) bool {
	return len(value) > 1 && value[0] == prefix && strings.Trim(value[1:], "0123456789") == ""
}
