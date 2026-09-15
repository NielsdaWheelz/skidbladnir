package fleetclient

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// Request is shared by command parsing and the session browser. It is never a wire DTO.
type Request struct {
	Operation   string
	Name        string
	Machine     string
	Ref         string
	Profile     string
	CWD         string
	Text        string
	Keys        []string
	Mode        string
	MaxBytes    int
	Space       space.Label
	SpaceFilter space.Filter
}

type ProcessReference struct {
	PaneID        string `json:"paneId"`
	PID           int    `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

type Reference struct {
	Machine       string            `json:"machine"`
	TmuxID        string            `json:"tmuxId"`
	IdentityToken string            `json:"identityToken"`
	Agent         *ProcessReference `json:"agent,omitempty"`
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
	if strictjson.Decode(data, &ref) != nil || !tmuxAddress(ref.TmuxID, '$') || ref.IdentityToken == "" {
		return Reference{}, invalid
	}
	if _, err := machine.Parse(ref.Machine); err != nil {
		return Reference{}, invalid
	}
	var fields map[string]json.RawMessage
	if strictjson.Decode(data, &fields) != nil {
		return Reference{}, invalid
	}
	for _, value := range fields {
		if string(value) == "null" {
			return Reference{}, invalid
		}
	}
	if ref.Agent != nil && (!tmuxAddress(ref.Agent.PaneID, '%') || ref.Agent.PID <= 0 || ref.Agent.StartIdentity == "") {
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
	if request.Operation != "start" && request.Operation != "space" && !request.Space.IsUnassigned() || request.Operation != "list" && request.SpaceFilter.Kind() != space.FilterAll {
		return false
	}
	if request.Mode != "" && request.Mode != "auto" && request.Mode != "terminal" {
		return false
	}
	if request.Operation != "read" && request.MaxBytes != 0 || request.Operation != "read" && request.Operation != "send" && request.Mode != "" {
		return false
	}
	if request.Operation != "send" && request.Text != "" || request.Operation != "keys" && len(request.Keys) != 0 {
		return false
	}
	if request.Operation != "start" && (request.Profile != "" || request.CWD != "") {
		return false
	}
	switch request.Operation {
	case "list":
		return request.Name == "" && request.Ref == ""
	case "start":
		return request.Name != "" && request.Machine != "" && request.Profile != "" && request.Ref == ""
	case "info", "enter", "read", "send", "keys", "interrupt", "stop", "kill", "space":
	default:
		return false
	}
	if request.Ref != "" {
		if request.Name != "" || request.Machine != "" {
			return false
		}
		if _, err := DecodeReference(request.Ref); err != nil {
			return false
		}
	} else if request.Name == "" {
		return false
	}
	switch request.Operation {
	case "read":
		return request.MaxBytes >= 0 && request.MaxBytes <= 32768
	case "send":
		return request.Text != "" && len(request.Text) <= 32768 && utf8.ValidString(request.Text)
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
