package fleetclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func handle(prefix string, tuple []string) string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(tuple)
	digest := sha256.Sum256(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}))
	return prefix + hex.EncodeToString(digest[:8])
}
func terminalHandle(ref Reference) string {
	return handle("t-", []string{"skid-terminal-handle", ref.Machine, ref.TmuxID, ref.IdentityToken})
}
func conversationHandle(machine string, conversation agentruntime.Conversation) string {
	return handle("c-", []string{"skid-conversation-handle", machine, string(conversation.Provider), string(conversation.ProfileKey), conversation.HistoryScope, conversation.ConversationID})
}
func validHandle(value, operation, mode string) bool {
	if len(value) != 18 || strings.Trim(value[2:], "0123456789abcdef") != "" {
		return false
	}
	terminal := operation == "info" || operation == "enter" || operation == "text" || operation == "keys" || operation == "shell" || operation == "group" || operation == "track" || operation == "untrack" || operation == "close" || (operation == "read" || operation == "stop") && mode == "terminal"
	if terminal {
		return strings.HasPrefix(value, "t-")
	}
	return (operation == "read" || operation == "send" || operation == "wait" || operation == "stop") && strings.HasPrefix(value, "c-")
}
