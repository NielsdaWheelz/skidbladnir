package fleetclient

import (
	"encoding/json"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type target struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
	PID           int    `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

type operationInput struct {
	Target   target    `json:"target"`
	Mode     *string   `json:"mode,omitempty"`
	MaxBytes *int      `json:"maxBytes,omitempty"`
	Text     *string   `json:"text,omitempty"`
	Keys     *[]string `json:"keys,omitempty"`
}

func (client *Client) prepare(operation string, encoded []byte) (peer, string, []byte, *Failure) {
	invalid := &Failure{Code: "invalid_input", Dispatch: "not_sent"}
	if operation == "start" {
		var input *struct {
			Machine string  `json:"machine"`
			Profile string  `json:"profile"`
			CWD     string  `json:"cwd"`
			Name    *string `json:"name,omitempty"`
		}
		if strictjson.Decode(encoded, &input) != nil || input == nil || input.Machine == "" || input.Profile == "" || input.CWD == "" || input.Name != nil && *input.Name == "" {
			return peer{}, "", nil, invalid
		}
		var members map[string]json.RawMessage
		if strictjson.Decode(encoded, &members) != nil || members["name"] != nil && input.Name == nil {
			return peer{}, "", nil, invalid
		}
		for _, selected := range client.peers {
			if selected.Label != input.Machine {
				continue
			}
			body, _ := json.Marshal(struct {
				CWD     string  `json:"cwd"`
				Profile string  `json:"profile"`
				Name    *string `json:"optionalTmuxName,omitempty"`
			}{input.CWD, input.Profile, input.Name})
			return selected, "/v1/sessions", body, nil
		}
		return peer{}, "", nil, &Failure{Code: "machine_unknown", Dispatch: "not_sent"}
	}
	var input *operationInput
	if strictjson.Decode(encoded, &input) != nil || input == nil {
		return peer{}, "", nil, invalid
	}
	// Optional fields are operation-specific: reject them even when explicitly null.
	var members map[string]json.RawMessage
	if strictjson.Decode(encoded, &members) != nil {
		return peer{}, "", nil, invalid
	}
	for key, value := range members {
		if string(value) == "null" {
			return peer{}, "", nil, invalid
		}
		allowed := key == "target" || operation == "read" && (key == "mode" || key == "maxBytes") || operation == "send" && (key == "mode" || key == "text") || operation == "keys" && key == "keys"
		if !allowed {
			return peer{}, "", nil, invalid
		}
	}
	selectedTarget := input.Target
	if !tmuxAddress(selectedTarget.TmuxID, '$') || !tmuxAddress(selectedTarget.PaneID, '%') || selectedTarget.PID <= 0 || selectedTarget.IdentityToken == "" || selectedTarget.StartIdentity == "" {
		return peer{}, "", nil, invalid
	}
	body := map[string]any{
		"identityToken": selectedTarget.IdentityToken, "paneId": selectedTarget.PaneID,
		"pid": selectedTarget.PID, "startIdentity": selectedTarget.StartIdentity,
	}
	switch operation {
	case "read", "send":
		mode := "auto"
		if input.Mode != nil {
			mode = *input.Mode
		}
		if mode != "auto" && mode != "terminal" {
			return peer{}, "", nil, invalid
		}
		body["mode"] = mode
		if operation == "read" {
			maximum := 16384
			if input.MaxBytes != nil {
				maximum = *input.MaxBytes
			}
			if maximum < 1 || maximum > 32768 {
				return peer{}, "", nil, invalid
			}
			body["maxBytes"] = maximum
		} else {
			if input.Text == nil || *input.Text == "" || len(*input.Text) > 32768 {
				return peer{}, "", nil, invalid
			}
			body["text"] = *input.Text
		}
	case "keys":
		if input.Keys == nil || len(*input.Keys) < 1 || len(*input.Keys) > 16 {
			return peer{}, "", nil, invalid
		}
		for _, key := range *input.Keys {
			switch key {
			case "enter", "escape", "ctrl-c", "up", "down", "left", "right", "tab", "backspace", "page-up", "page-down":
			default:
				return peer{}, "", nil, invalid
			}
		}
		body["keys"] = *input.Keys
	case "interrupt", "stop":
	default:
		return peer{}, "", nil, &Failure{Code: "operation_unknown", Dispatch: "not_sent"}
	}
	for _, selected := range client.peers {
		if selected.Machine != selectedTarget.Machine {
			continue
		}
		encodedBody, _ := json.Marshal(body)
		if len(encodedBody) > MaximumInputBytes {
			return peer{}, "", nil, &Failure{Code: "input_limit", Dispatch: "not_sent"}
		}
		return selected, "/v1/sessions/" + selectedTarget.TmuxID + "/agent/" + operation, encodedBody, nil
	}
	return peer{}, "", nil, &Failure{Code: "machine_unknown", Dispatch: "not_sent"}
}

func tmuxAddress(value string, prefix byte) bool {
	return len(value) > 1 && value[0] == prefix && strings.Trim(value[1:], "0123456789") == ""
}
