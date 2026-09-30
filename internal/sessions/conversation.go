package sessions

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

const conversationOption = "@skid_conversation_b64"

func decodeConversation(encoded string) (agentruntime.Conversation, error) {
	var conversation agentruntime.Conversation
	if len(encoded) > 2048 {
		return conversation, errors.New("conversation metadata too large")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return conversation, errors.New("invalid conversation metadata")
	}
	err = json.Unmarshal(decoded, &conversation)
	return conversation, err
}
