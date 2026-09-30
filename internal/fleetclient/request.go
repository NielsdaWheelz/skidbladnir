package fleetclient

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
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
	ConversationID string
	Operation      string
	Name           string
	Machine        string
	Ref            string
	Kind           LaunchKind
	Profile        string
	CWD            string
	Text           string
	Keys           []string
	Mode           string
	Scope          string
	Input          string
	Delivery       string
	TerminalOnly   bool
	State          string
	WaitTimeout    time.Duration
	MaxBytes       int
	Group          group.Label
	GroupFilter    group.Filter
}

type ProcessReference struct {
	PaneID        string `json:"paneId"`
	PID           int    `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

type Reference struct {
	Machine       string                            `json:"machine"`
	TmuxID        string                            `json:"tmuxId,omitempty"`
	IdentityToken string                            `json:"identityToken,omitempty"`
	Agent         *ProcessReference                 `json:"agent,omitempty"`
	Conversation  *agentruntime.ConversationRuntime `json:"conversation,omitempty"`
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
	if ref.Agent != nil && (!tmuxAddress(ref.Agent.PaneID, '%') || ref.Agent.PID <= 0 || ref.Agent.StartIdentity == "") {
		return Reference{}, invalid
	}
	if ref.TmuxID == "" {
		if ref.IdentityToken != "" || ref.Agent != nil || ref.Conversation == nil {
			return Reference{}, invalid
		}
	} else if !tmuxAddress(ref.TmuxID, '$') || ref.IdentityToken == "" {
		return Reference{}, invalid
	}
	if ref.Conversation != nil && !validConversationRuntime(*ref.Conversation) {
		return Reference{}, invalid
	}
	return ref, nil
}

func (ref Reference) Encode() string {
	encoded, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// SessionEqual deliberately ignores the observed foreground process and name.
func (ref Reference) SessionEqual(other Reference) bool {
	return ref.Machine == other.Machine && ref.TmuxID == other.TmuxID && ref.IdentityToken == other.IdentityToken
}

func (request Request) Valid() bool {
	if request.Operation != "read" && request.Scope != "" || request.Operation != "send" && (request.Input != "" || request.Delivery != "") || request.Operation != "wait" && (request.State != "" || request.WaitTimeout != 0) || request.Operation != "close" && request.TerminalOnly {
		return false
	}
	if request.Operation != "start" && request.Operation != "group" && !request.Group.IsUnassigned() || request.Operation != "list" && request.GroupFilter.Kind() != group.FilterAll {
		return false
	}
	if request.Mode != "" && request.Mode != "native" && request.Mode != "terminal" {
		return false
	}
	if request.Operation != "read" && request.MaxBytes != 0 || request.Operation != "read" && request.Operation != "stop" && request.Mode != "" {
		return false
	}
	if request.Operation != "send" && request.Operation != "text" && request.Text != "" || request.Operation != "keys" && len(request.Keys) != 0 {
		return false
	}
	if request.Operation != "start" && (request.Kind != "" || request.Profile != "" && request.ConversationID == "" || request.CWD != "") {
		return false
	}
	switch request.Operation {
	case "list":
		return request.Name == "" && request.Ref == "" && request.ConversationID == ""
	case "start":
		return request.Machine != "" && request.Ref == "" && request.ConversationID == "" &&
			(request.Kind == LaunchAgent && request.Profile != "" || request.Kind == LaunchTerminal && request.Profile == "")
	case "info", "enter", "read", "send", "keys", "text", "stop", "close", "wait", "group", "shell", "inspect":
	default:
		return false
	}
	if request.Ref != "" {
		if request.ConversationID != "" {
			return false
		}
		if request.Name != "" || request.Machine != "" {
			return false
		}
		if ref, err := DecodeReference(request.Ref); err != nil || ref.TmuxID == "" && (request.Operation == "enter" || request.Operation == "close" || request.Operation == "text" || request.Operation == "keys" || request.Operation == "shell" || request.Operation == "group" || request.Mode == "terminal") {
			return false
		}
	} else if request.ConversationID != "" {
		if request.Ref != "" || request.Mode == "terminal" || request.Machine == "" || request.Profile == "" || !validReplyID(request.ConversationID) || request.Name != "" {
			return false
		}
		if request.Operation != "read" && request.Operation != "send" && request.Operation != "wait" && request.Operation != "stop" && request.Operation != "inspect" {
			return false
		}
	} else if request.Name == "" {
		return false
	}
	switch request.Operation {
	case "read":
		return request.MaxBytes >= 0 && request.MaxBytes <= 32768 && (request.Mode == "terminal" && request.Scope == "" || request.Mode != "terminal" && (request.Scope == "" || request.Scope == "latest" || request.Scope == "history"))
	case "send":
		return validInputText(request.Text) && (request.Input == "peer" || request.Input == "user") && (request.Delivery == "direct" || request.Delivery == "queue")
	case "text":
		return validInputText(request.Text)
	case "wait":
		return (request.State == "" || request.State == "idle" || request.State == "blocked" || request.State == "done" || request.State == "failed" || request.State == "stopped") && request.WaitTimeout >= 0 && request.WaitTimeout <= time.Hour
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
