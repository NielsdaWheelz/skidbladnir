package sessions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

const conversationOption = "@skid_conversation_b64"

var ErrConversationDispatchUnknown = errors.New("conversation association completion is unknown")

func encodeConversation(conversation agentruntime.Conversation) (string, error) {
	encoded, err := json.Marshal(conversation)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

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

// SetConversation changes only session metadata. It never selects a provider view.
func (manager *Manager) SetConversation(ctx context.Context, tmuxID, identityToken string, conversation *agentruntime.Conversation) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()
	server, _, err := manager.sessionLifetimeIdentity(ctx, tmuxID, identityToken)
	if err != nil {
		return err
	}
	assignment := "set-option -u -t '" + tmuxID + "' -- " + conversationOption
	if conversation != nil {
		encoded, err := encodeConversation(*conversation)
		if err != nil {
			return err
		}
		assignment = "set-option -t '" + tmuxID + "' -- " + conversationOption + " " + encoded
	}
	condition := "#{&&:#{==:#{session_id}," + tmuxID + "},#{&&:#{==:#{@skid_server_epoch}," + server.Epoch + "},#{&&:#{==:#{pid}," + server.PID + "},#{==:#{start_time}," + server.StartTime + "}}}}"
	output, err := manager.tmux.Output(ctx, "set-conversation-if-identity", "if-shell", "-F", "-t", tmuxID, condition, assignment+" ; display-message -p -l 'associated'", "display-message -p -l 'stale'")
	if err != nil {
		return ErrConversationDispatchUnknown
	}
	if output == "stale" {
		return sessionIdentityMismatch()
	}
	if output != "associated" {
		return ErrConversationDispatchUnknown
	}
	return nil
}
