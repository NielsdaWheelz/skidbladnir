package hostconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const maximumConfigBytes = 64 * 1024

var (
	expectedProfileKeys = [...]string{"personal", "work", "work2", "claude-personal", "claude-work"}
)

type Tmux struct {
	Path    string
	Version string
}

type Config struct {
	Platform            platform.Kind
	Tmux                Tmux
	CodexNodeEntrypoint string
	Profiles            []sessions.Profile
}

func Load(path string, runtime platform.Kind) (Config, error) {
	if path == "" {
		return Config{}, errors.New("host config path is empty")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read host config: %w", err)
	}
	if len(encoded) > maximumConfigBytes {
		return Config{}, errors.New("host config is too large")
	}
	config, err := parse(encoded, runtime)
	if err != nil {
		return Config{}, fmt.Errorf("parse host config: %w", err)
	}
	return config, nil
}

func parse(encoded []byte, runtime platform.Kind) (Config, error) {
	if len(encoded) == 0 || len(encoded) > maximumConfigBytes {
		return Config{}, errors.New("host config has invalid size")
	}
	if runtime != platform.KindLinux && runtime != platform.KindDarwin {
		return Config{}, errors.New("runtime platform is unsupported")
	}
	wire, err := decodeConfig(encoded)
	if err != nil {
		return Config{}, err
	}
	return wire.validate(runtime)
}

func (config Config) ValidateTmuxVersion(observed string) error {
	if observed != config.Tmux.Version {
		return fmt.Errorf("tmux version is %q, want %q", observed, config.Tmux.Version)
	}
	return nil
}

type configDTO struct {
	Platform            stringField   `json:"platform"`
	Tmux                *tmuxDTO      `json:"tmux"`
	CodexNodeEntrypoint stringField   `json:"codexNodeEntrypoint"`
	Profiles            *[]profileDTO `json:"profiles"`
}

type tmuxDTO struct {
	Path    stringField `json:"path"`
	Version stringField `json:"version"`
}

type profileDTO struct {
	Key                  stringField               `json:"key"`
	Label                stringField               `json:"label"`
	Command              stringField               `json:"command"`
	Environment          *[]environmentVariableDTO `json:"environment"`
	ForegroundSignatures *[]foregroundSignatureDTO `json:"foregroundSignatures"`
	Arguments            *[]stringField            `json:"arguments"`
}

type environmentVariableDTO struct {
	Name  stringField `json:"name"`
	Value stringField `json:"value"`
}

type foregroundSignatureDTO struct {
	ExecutableBase stringField `json:"executableBase"`
	Argument0      stringField `json:"argument0"`
	Argument1      stringField `json:"argument1"`
}

func (wire configDTO) validate(runtime platform.Kind) (Config, error) {
	if !wire.Platform.present || wire.Tmux == nil || !wire.CodexNodeEntrypoint.present || wire.Profiles == nil {
		return Config{}, errors.New("host config omits a required member")
	}
	kind := platform.Kind(wire.Platform.value)
	if kind != platform.KindLinux && kind != platform.KindDarwin {
		return Config{}, errors.New("host config platform is unsupported")
	}
	if kind != runtime {
		return Config{}, fmt.Errorf("host config platform %q does not match runtime %q", kind, runtime)
	}
	if !wire.Tmux.Path.present || !wire.Tmux.Version.present || !validAbsolutePath(wire.Tmux.Path.value) || !safeText(wire.Tmux.Version.value, 64) || !strings.HasPrefix(wire.Tmux.Version.value, "tmux ") {
		return Config{}, errors.New("host config tmux entry is invalid")
	}
	if !validAbsolutePath(wire.CodexNodeEntrypoint.value) {
		return Config{}, errors.New("host config Codex entrypoint must be absolute")
	}
	profiles, err := mapProfiles(*wire.Profiles)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Platform:            kind,
		Tmux:                Tmux{Path: filepath.Clean(wire.Tmux.Path.value), Version: wire.Tmux.Version.value},
		CodexNodeEntrypoint: filepath.Clean(wire.CodexNodeEntrypoint.value),
		Profiles:            profiles,
	}, nil
}

func mapProfiles(wire []profileDTO) ([]sessions.Profile, error) {
	if len(wire) != len(expectedProfileKeys) {
		return nil, fmt.Errorf("host config must declare exactly %d profiles", len(expectedProfileKeys))
	}
	profiles := make([]sessions.Profile, len(wire))
	for index, candidate := range wire {
		if !candidate.Key.present || !candidate.Label.present || !candidate.Command.present || candidate.Environment == nil || candidate.ForegroundSignatures == nil || candidate.Arguments == nil {
			return nil, errors.New("host config profile omits a required member")
		}
		if candidate.Key.value != expectedProfileKeys[index] {
			return nil, fmt.Errorf("host config profile %d must be %q", index, expectedProfileKeys[index])
		}
		environment, err := mapEnvironment(*candidate.Environment)
		if err != nil {
			return nil, err
		}
		signatures := make([]sessions.ForegroundSignature, len(*candidate.ForegroundSignatures))
		for signatureIndex, signature := range *candidate.ForegroundSignatures {
			signatures[signatureIndex] = sessions.ForegroundSignature{
				ExecutableBase: signature.ExecutableBase.value,
				Argument0:      signature.Argument0.value,
				Argument1:      signature.Argument1.value,
			}
		}
		arguments := make([]string, len(*candidate.Arguments))
		for argumentIndex, argument := range *candidate.Arguments {
			if !argument.present {
				return nil, errors.New("host config profile argument is null")
			}
			arguments[argumentIndex] = argument.value
		}
		profiles[index] = sessions.Profile{
			Key:                  candidate.Key.value,
			Label:                candidate.Label.value,
			Command:              candidate.Command.value,
			Environment:          environment,
			ForegroundSignatures: signatures,
			Arguments:            arguments,
		}
	}
	validated, err := sessions.ValidateProfiles(profiles)
	if err != nil {
		return nil, fmt.Errorf("validate host profiles: %w", err)
	}
	return validated, nil
}

func mapEnvironment(wire []environmentVariableDTO) ([]sessions.EnvironmentVariable, error) {
	environment := make([]sessions.EnvironmentVariable, len(wire))
	for index, candidate := range wire {
		if !candidate.Name.present || !candidate.Value.present {
			return nil, errors.New("host config environment entry omits a required member")
		}
		environment[index] = sessions.EnvironmentVariable{Name: candidate.Name.value, Value: candidate.Value.value}
	}
	return environment, nil
}

func safeText(value string, maximumRunes int) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumRunes {
		return false
	}
	for _, character := range value {
		if isUnsafeTextRune(character) {
			return false
		}
	}
	return true
}

func validAbsolutePath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func isUnsafeTextRune(value rune) bool {
	return value <= '\u001f' || value >= '\u007f' && value <= '\u009f' || value == '\u2028' || value == '\u2029' || value >= '\u202a' && value <= '\u202e' || value >= '\u2066' && value <= '\u2069'
}
