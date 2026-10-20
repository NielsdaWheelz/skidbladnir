package agentruntime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"strings"
	"unicode/utf8"
)

const MaxLaunchBytes = 16 * 1024

// Launch carries command text as data until the initialized shell evaluates it.
type Launch struct {
	Command string `json:"command"`
}

func NewLaunch(profile Profile) Launch { return Launch{Command: profile.Command} }
func validCommand(command string) bool {
	return strings.TrimSpace(command) != "" && utf8.ValidString(command) && !strings.ContainsRune(command, 0)
}
func EncodeLaunch(launch Launch) (string, error) {
	if !validCommand(launch.Command) {
		return "", errors.New("invalid agent command")
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
	if err := strictjson.Decode(value, &launch); err != nil || !validCommand(launch.Command) {
		return Launch{}, errors.New("invalid agent launch")
	}
	return launch, nil
}
