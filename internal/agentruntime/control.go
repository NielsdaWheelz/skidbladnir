package agentruntime

import (
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type Status struct {
	State  string `json:"state"`
	Source string `json:"source"`
}

type Methods struct {
	Read      string `json:"read"`
	SendPeer  string `json:"sendPeer"`
	SendUser  string `json:"sendUser"`
	QueueUser string `json:"queueUser"`
	Stop      string `json:"stop"`
}

type Conversation struct {
	Provider       Provider   `json:"provider"`
	ProfileKey     ProfileKey `json:"profileKey"`
	HistoryScope   string     `json:"historyScope"`
	ConversationID string     `json:"conversationId"`
}

type Binding struct {
	Conversation Conversation `json:"conversation"`
}

type Turn struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type Observation struct {
	Binding Binding `json:"binding"`
	Status  Status  `json:"status"`
	Turn    *Turn   `json:"turn,omitempty"`
}

type ConversationRuntime struct {
	Binding Binding `json:"binding"`
	Status  Status  `json:"status"`
	Methods Methods `json:"methods"`
	Turn    *Turn   `json:"turn,omitempty"`
}

func (status Status) Valid() bool {
	switch status.State {
	case "working", "blocked", "idle", "done", "failed", "stopped", "unknown":
	default:
		return false
	}
	return status.Source == "native" || status.Source == "unavailable" && status.State == "unknown"
}

func (methods Methods) Valid() bool {
	for _, method := range []string{methods.Read, methods.SendPeer, methods.SendUser} {
		if method != "native" && method != "unavailable" {
			return false
		}
	}
	return methods.QueueUser == "unavailable" && (methods.Stop == "native" || methods.Stop == "unavailable")
}

func (conversation Conversation) Valid() bool {
	if _, err := ParseProvider(conversation.Provider.String()); err != nil {
		return false
	}
	if _, err := ParseProfileKey(string(conversation.ProfileKey)); err != nil {
		return false
	}
	if len(conversation.HistoryScope) != 64 || !validProviderSessionID(conversation.ConversationID) {
		return false
	}
	for _, value := range conversation.HistoryScope {
		if value < '0' || value > '9' && (value < 'a' || value > 'f') {
			return false
		}
	}
	return true
}

func (binding Binding) Valid() bool              { return binding.Conversation.Valid() }
func (binding Binding) Equal(other Binding) bool { return binding.Conversation == other.Conversation }

func (turn Turn) Valid() bool {
	if !validProviderSessionID(turn.ID) {
		return false
	}
	switch turn.State {
	case "inProgress", "completed", "failed", "interrupted":
		return true
	default:
		return false
	}
}

// These records are also accepted from opaque client references. Null never
// means omission.
func requiredRecord(encoded []byte, fields ...string) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	if members == nil {
		return errors.New("record cannot be null")
	}
	for _, value := range members {
		if string(value) == "null" {
			return errors.New("record member cannot be null")
		}
	}
	for _, field := range fields {
		if _, found := members[field]; !found {
			return errors.New("record omits required member")
		}
	}
	return nil
}
func (value *Conversation) UnmarshalJSON(encoded []byte) error {
	if err := requiredRecord(encoded, "provider", "profileKey", "historyScope", "conversationId"); err != nil {
		return err
	}
	type wire Conversation
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*value = Conversation(decoded)
	if !value.Valid() {
		return errors.New("invalid conversation")
	}
	return nil
}
func (value *Binding) UnmarshalJSON(encoded []byte) error {
	if err := requiredRecord(encoded, "conversation"); err != nil {
		return err
	}
	type wire Binding
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*value = Binding(decoded)
	if !value.Valid() {
		return errors.New("invalid binding")
	}
	return nil
}
func (value *Turn) UnmarshalJSON(encoded []byte) error {
	if err := requiredRecord(encoded, "id", "state"); err != nil {
		return err
	}
	type wire Turn
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*value = Turn(decoded)
	if !value.Valid() {
		return errors.New("invalid turn")
	}
	return nil
}
