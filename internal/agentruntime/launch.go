package agentruntime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

const MaxLaunchBytes = 16 * 1024

// Launch is the one foreground action requested of a newly created shell.
type Launch struct {
	Command     string                `json:"command"`
	Arguments   []string              `json:"arguments"`
	Environment []EnvironmentVariable `json:"environment"`
}

func NewLaunch(profile Profile, name string) Launch {
	return Launch{Command: profile.Command, Arguments: LaunchArguments(profile, name), Environment: profile.Environment}
}

func EncodeLaunch(launch Launch) (string, error) {
	if err := validateLaunch(launch); err != nil {
		return "", err
	}
	value, err := json.Marshal(launch)
	if err != nil || len(value) > MaxLaunchBytes {
		return "", errors.New("agent launch is too large")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func DecodeLaunch(encoded string) (Launch, error) {
	if len(encoded) > base64.RawURLEncoding.EncodedLen(MaxLaunchBytes) {
		return Launch{}, errors.New("agent launch is too large")
	}
	value, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(value) > MaxLaunchBytes || base64.RawURLEncoding.EncodeToString(value) != encoded {
		return Launch{}, errors.New("invalid agent launch")
	}
	var launch Launch
	if err := json.Unmarshal(value, &launch); err != nil || validateLaunch(launch) != nil {
		return Launch{}, errors.New("invalid agent launch")
	}
	return launch, nil
}

func validateLaunch(launch Launch) error {
	if !filepath.IsAbs(launch.Command) || strings.ContainsRune(launch.Command, 0) {
		return errors.New("invalid agent command")
	}
	names := make(map[string]bool, len(launch.Environment))
	for _, variable := range launch.Environment {
		if !environmentPattern.MatchString(variable.Name) || names[variable.Name] || strings.ContainsRune(variable.Value, 0) {
			return errors.New("invalid agent environment")
		}
		names[variable.Name] = true
	}
	for _, argument := range launch.Arguments {
		if strings.ContainsRune(argument, 0) {
			return errors.New("invalid agent argument")
		}
	}
	return nil
}
