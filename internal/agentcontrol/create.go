package agentcontrol

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func (service *Service) CreateConversation(parent context.Context, profileKey agentruntime.ProfileKey, name, cwd string) (agentruntime.Conversation, error) {
	profile, found := service.sessions.Profile(profileKey)
	if !found || profile.Provider != agentruntime.ProviderCodex {
		return agentruntime.Conversation{}, ErrInvalidInput
	}
	if len(profile.Arguments) != 0 && (len(profile.Arguments) != 1 || profile.Arguments[0] != "--yolo") {
		return agentruntime.Conversation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	scope, err := agentruntime.HistoryScope(profile)
	if err != nil {
		return agentruntime.Conversation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, profile.Command, "app-server", "daemon", "start")
	command.Env = service.nativeEnvironment(profile)
	command.WaitDelay = 250 * time.Millisecond
	if err := command.Run(); err != nil {
		return agentruntime.Conversation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	err = service.native(ctx, profile, "create", nil, struct {
		Name              string `json:"name"`
		CWD               string `json:"cwd"`
		BypassPermissions bool   `json:"bypassPermissions"`
	}{name, cwd, slices.Contains(profile.Arguments, "--yolo")}, &created)
	if err != nil {
		var partial *nativeCreateError
		if !errors.As(err, &partial) {
			return agentruntime.Conversation{}, err
		}
		created.SessionID = partial.sessionID
	}
	conversation := agentruntime.Conversation{Provider: profile.Provider, ProfileKey: profile.Key, HistoryScope: scope, ConversationID: created.SessionID}
	if !conversation.Valid() || !nativeCodexIDPattern.MatchString(created.SessionID) {
		return agentruntime.Conversation{}, &UnavailableError{Dispatch: "unknown"}
	}
	return conversation, err
}

func (service *Service) Associate(parent context.Context, tmuxID, identityToken string, conversation agentruntime.Conversation) error {
	if conversation.Provider != agentruntime.ProviderCodex {
		return ErrInvalidInput
	}
	if _, err := service.Inspect(parent, conversation); err != nil {
		return err
	}
	return service.sessions.SetConversation(parent, tmuxID, identityToken, &conversation)
}
