package hostconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
)

const maximumConfigBytes = 64 * 1024

var expectedProfiles = [...]struct {
	key      string
	provider agentruntime.Provider
}{
	{key: "personal", provider: agentruntime.ProviderCodex},
	{key: "work", provider: agentruntime.ProviderCodex},
	{key: "work2", provider: agentruntime.ProviderCodex},
	{key: "claude-personal", provider: agentruntime.ProviderClaude},
	{key: "claude-work", provider: agentruntime.ProviderClaude},
}

type Tmux struct {
	Path string
}

type Config struct {
	Platform platform.Kind
	Tmux     Tmux
	Profiles []agentruntime.Profile
}

type hostConfigFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

func Load(path string, runtime platform.Kind) (Config, error) {
	if path == "" {
		return Config{}, errors.New("host config path is empty")
	}
	// Host configuration is a deployment-owned local regular file. Nonblocking,
	// no-follow admission rejects FIFOs, devices, and symlinks before a hook can
	// wait on an unbounded filesystem producer; the capped read prevents a large
	// file from being normalized into memory before its size is rejected.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return Config{}, fmt.Errorf("read host config: %w", err)
	}
	return loadOpenedHostConfig(file, runtime)
}

func loadOpenedHostConfig(file hostConfigFile, runtime platform.Kind) (config Config, resultErr error) {
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			config = Config{}
			resultErr = errors.Join(resultErr, fmt.Errorf("close host config: %w", closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Config{}, errors.New("host config is not a regular file")
	}
	encoded, err := io.ReadAll(io.LimitReader(file, maximumConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read host config: %w", err)
	}
	if len(encoded) > maximumConfigBytes {
		return Config{}, errors.New("host config is too large")
	}
	config, err = parse(encoded, runtime)
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

func ValidateTmuxVersion(version string) error {
	if len(version) > 64 || !strings.HasPrefix(version, "tmux ") {
		return errors.New("tmux version is invalid")
	}
	for _, character := range []byte(version) {
		if character < 0x20 || character > 0x7e {
			return errors.New("tmux version is invalid")
		}
	}
	release := strings.TrimPrefix(version, "tmux ")
	if release == "" || strings.TrimSpace(release) != release {
		return errors.New("tmux version is invalid")
	}
	return nil
}

type configDTO struct {
	Platform stringField   `json:"platform"`
	Tmux     *tmuxDTO      `json:"tmux"`
	Profiles *[]profileDTO `json:"profiles"`
}

type tmuxDTO struct {
	Path          stringField `json:"path"`
	TestedVersion stringField `json:"testedVersion"`
}

type profileDTO struct {
	Key                  stringField               `json:"key"`
	Label                stringField               `json:"label"`
	Provider             stringField               `json:"provider"`
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
	if !wire.Platform.present || wire.Tmux == nil || wire.Profiles == nil {
		return Config{}, errors.New("host config omits a required member")
	}
	kind := platform.Kind(wire.Platform.value)
	if kind != platform.KindLinux && kind != platform.KindDarwin {
		return Config{}, errors.New("host config platform is unsupported")
	}
	if kind != runtime {
		return Config{}, fmt.Errorf("host config platform %q does not match runtime %q", kind, runtime)
	}
	if !wire.Tmux.Path.present || !wire.Tmux.TestedVersion.present || !validAbsolutePath(wire.Tmux.Path.value) || ValidateTmuxVersion(wire.Tmux.TestedVersion.value) != nil {
		return Config{}, errors.New("host config tmux entry is invalid")
	}
	profiles, err := mapProfiles(*wire.Profiles)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Platform: kind,
		Tmux:     Tmux{Path: filepath.Clean(wire.Tmux.Path.value)},
		Profiles: profiles,
	}, nil
}

func mapProfiles(wire []profileDTO) ([]agentruntime.Profile, error) {
	if len(wire) != len(expectedProfiles) {
		return nil, fmt.Errorf("host config must declare exactly %d profiles", len(expectedProfiles))
	}
	profiles := make([]agentruntime.Profile, len(wire))
	for index, candidate := range wire {
		expected := expectedProfiles[index]
		if !candidate.Key.present || !candidate.Label.present || !candidate.Provider.present || !candidate.Command.present || candidate.Environment == nil || candidate.ForegroundSignatures == nil || candidate.Arguments == nil {
			return nil, errors.New("host config profile omits a required member")
		}
		if candidate.Key.value != expected.key {
			return nil, fmt.Errorf("host config profile %d must be %q", index, expected.key)
		}
		provider, err := agentruntime.ParseProvider(candidate.Provider.value)
		if err != nil {
			return nil, fmt.Errorf("host config profile %s provider is invalid", candidate.Key.value)
		}
		if provider != expected.provider {
			return nil, fmt.Errorf("host config profile %s must use provider %s", candidate.Key.value, expected.provider)
		}
		environment, err := mapEnvironment(*candidate.Environment)
		if err != nil {
			return nil, err
		}
		signatures := make([]agentruntime.ForegroundSignature, len(*candidate.ForegroundSignatures))
		for signatureIndex, signature := range *candidate.ForegroundSignatures {
			signatures[signatureIndex] = agentruntime.ForegroundSignature{
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
		profiles[index] = agentruntime.Profile{
			Key:                  agentruntime.ProfileKey(candidate.Key.value),
			Label:                candidate.Label.value,
			Provider:             provider,
			Command:              candidate.Command.value,
			Environment:          environment,
			ForegroundSignatures: signatures,
			Arguments:            arguments,
		}
	}
	validated, err := agentruntime.ValidateProfiles(profiles)
	if err != nil {
		return nil, fmt.Errorf("validate host profiles: %w", err)
	}
	return validated, nil
}

func mapEnvironment(wire []environmentVariableDTO) ([]agentruntime.EnvironmentVariable, error) {
	environment := make([]agentruntime.EnvironmentVariable, len(wire))
	for index, candidate := range wire {
		if !candidate.Name.present || !candidate.Value.present {
			return nil, errors.New("host config environment entry omits a required member")
		}
		environment[index] = agentruntime.EnvironmentVariable{Name: candidate.Name.value, Value: candidate.Value.value}
	}
	return environment, nil
}

func validAbsolutePath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
