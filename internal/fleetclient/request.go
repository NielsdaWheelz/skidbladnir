package fleetclient

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

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
	Ref            string
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
	return terminalHandle(ref)
}

// SessionEqual compares session lifetime, independently of pane selection and name.
func (ref Reference) SessionEqual(other Reference) bool {
	return ref.TmuxID != "" && ref.Machine == other.Machine && ref.TmuxID == other.TmuxID && ref.IdentityToken == other.IdentityToken
}

func (request Request) Valid() bool {
	if request.Operation == "rename" {
		if request.ExpectedNaming == nil || request.Naming == nil || !request.ExpectedNaming.valid() || !request.Naming.valid() || request.Naming.Mode == "manual" && !ValidSessionName(request.Naming.Name) {
			return false
		}
	} else if request.ExpectedNaming != nil || request.Naming != nil {
		return false
	}
	if request.Operation != "wait" && (request.State != "" || request.WaitTimeout != 0) || request.Operation != "close" && request.TerminalOnly || request.Operation != "info" && request.Explain {
		return false
	}
	if request.Operation != "start" && request.Operation != "group" && !request.Group.IsUnassigned() || request.Operation != "list" && request.GroupFilter.Kind() != group.FilterAll {
		return false
	}
	if request.Operation != "read" && request.MaxBytes != 0 {
		return false
	}
	if request.Operation != "send" && request.Operation != "text" && request.Operation != "start" && request.Text != "" || request.Operation != "keys" && len(request.Keys) != 0 {
		return false
	}
	if request.Operation != "start" && (request.Kind != "" || request.Profile != "" || request.CWD != "") {
		return false
	}
	switch request.Operation {
	case "list":
		return request.Name == "" && request.Handle == "" && request.Ref == ""
	case "start":
		return request.Machine != "" && request.Handle == "" && request.Ref == "" &&
			(request.Kind == LaunchAgent && request.Profile != "" && (request.Text == "" || validInputText(request.Text)) ||
				request.Kind == LaunchTerminal && request.Profile == "" && request.Text == "")
	case "info", "enter", "read", "send", "keys", "text", "stop", "close", "wait", "group", "rename", "shell":
	default:
		return false
	}
	if request.Name != "" {
		return false
	}
	if request.Handle != "" && !validHandle(request.Handle, request.Operation) {
		return false
	}
	if request.Ref != "" {
		if request.Handle != "" || request.Machine != "" {
			return false
		}
		if _, err := DecodeReference(request.Ref); err != nil {
			return false
		}
	} else if request.Handle == "" {
		return false
	}
	switch request.Operation {
	case "read":
		return request.MaxBytes >= 0 && request.MaxBytes <= 32768
	case "send", "text":
		return validInputText(request.Text)
	case "wait":
		// A request names its state and timeout; the cli owns skid's documented defaults.
		if request.WaitTimeout <= 0 || request.WaitTimeout > time.Hour {
			return false
		}
		switch request.State {
		case "idle", "working", "needs-input":
			return true
		default:
			return false
		}
	case "keys":
		if len(request.Keys) < 1 || len(request.Keys) > 16 {
			return false
		}
		for _, key := range request.Keys {
			switch key {
			case "enter", "escape", "ctrl-c", "up", "down", "left", "right", "tab", "backspace", "page-up", "page-down":
			default:
				return false
			}
		}
	}
	return true
}

func tmuxAddress(value string, prefix byte) bool {
	return len(value) > 1 && value[0] == prefix && strings.Trim(value[1:], "0123456789") == ""
}

func validInputText(text string) bool {
	return text != "" && len(text) <= 32768 && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}
