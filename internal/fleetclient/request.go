package fleetclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
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

// DecodeReference admits exactly what Encode produced: a reference is valid when it re-encodes
// to itself.
func DecodeReference(encoded string) (Reference, error) {
	var ref Reference
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if len(encoded) > 4096 || err != nil || json.Unmarshal(data, &ref) != nil || !ref.valid() || ref.Encode() != encoded {
		return Reference{}, errors.New("invalid session reference")
	}
	return ref, nil
}

func (ref Reference) Encode() string {
	encoded, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// valid reports whether the ids form a reference; it is the client's one check of a session
// address, for cli input and gateway documents alike.
func (ref Reference) valid() bool {
	_, err := machine.Parse(ref.Machine)
	return err == nil && tmux.ValidID(ref.TmuxID, '$') && ref.IdentityToken != "" && tmux.ValidID(ref.PaneID, '%')
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
