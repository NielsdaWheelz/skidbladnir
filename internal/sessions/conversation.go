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

// Recorded native metadata takes precedence; registration only supplies
// otherwise absent Claude identity, without native status/history calls.
func (manager *Manager) projectClaudeConversation(session *Session) {
	agent := session.Agent
	if session.Conversation != nil || agent == nil || agent.Provider != agentruntime.ProviderClaude || agent.ProviderSession == nil || agent.ProviderSession.ID() == "" {
		return
	}
	profile, found := manager.Profile(agent.Profile)
	if !found || profile.Provider != agentruntime.ProviderClaude {
		return
	}
	scope, err := agentruntime.HistoryScope(profile)
	if err != nil {
		return
	} // justify-ignore-error: unavailable account metadata omits optional recorded native identity.
	conversation := agentruntime.Conversation{Provider: profile.Provider, ProfileKey: profile.Key, HistoryScope: scope, ConversationID: agent.ProviderSession.ID()}
	if conversation.Valid() {
		session.Conversation = &conversation
	}
}
