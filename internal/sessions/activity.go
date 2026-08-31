package sessions

import (
	"errors"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const recentActivityWindow time.Duration = 10 * time.Second

type activitySecond int64

func parseActivitySecond(encoded string) (activitySecond, error) {
	seconds, err := strconv.ParseInt(encoded, 10, 64)
	if err != nil || seconds <= 0 || strconv.FormatInt(seconds, 10) != encoded {
		return 0, errors.New("tmux window activity is invalid")
	}
	return activitySecond(seconds), nil
}

func deriveActivity(activity activitySecond, observedAt time.Time) (SessionActivity, error) {
	observedSecond := observedAt.Unix()
	activityValue := int64(activity)
	if activityValue > observedSecond {
		return "", errors.New("tmux window activity is after the observation clock")
	}
	if observedSecond-activityValue <= int64(recentActivityWindow/time.Second) {
		return SessionActivityActive, nil
	}
	return SessionActivityQuiet, nil
}

func deriveAgent(
	profiles []agentruntime.Profile,
	observed processinfo.Observation,
	registration string,
) *agentruntime.AgentRuntime {
	agent, found := agentruntime.Project(profiles, observed, registration)
	if !found {
		return nil
	}
	return &agent
}
