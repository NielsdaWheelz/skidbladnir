package agentcontrol

import (
	"context"
	"os/exec"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func (service *Service) PrepareLaunch(parent context.Context, profileKey agentruntime.ProfileKey) error {
	profile, found := service.sessions.Profile(profileKey)
	if !found || profile.Provider != agentruntime.ProviderCodex {
		return ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, profile.Command, "app-server", "daemon", "start")
	command.Env = service.nativeEnvironment(profile)
	command.WaitDelay = 250 * time.Millisecond
	if err := command.Run(); err != nil {
		return &UnavailableError{Dispatch: "not_sent"}
	}
	return nil
}
