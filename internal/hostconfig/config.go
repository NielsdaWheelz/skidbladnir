package hostconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const DefaultYAML = `version: 1
profiles: []
# shell: /bin/zsh
# terminal: apple-terminal
# default_machine: laptop
# replace profiles: [] with:
#   - name: work
#     label: "codex · work"
#     command: codex-work
#     provider: codex
#     home: ~/.codex-work
#     resume_command: 'codex-work resume "$1"'
# resume_command is used only for recovery. $1 is the saved conversation id;
# quote it as "$1" and use the same provider home as command.
`

type Profile struct {
	Name          string `yaml:"name" json:"name"`
	Label         string `yaml:"label,omitempty" json:"label"`
	Command       string `yaml:"command" json:"command"`
	ResumeCommand string `yaml:"resume_command,omitempty" json:"resume_command,omitempty"`
	Provider      string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Home          string `yaml:"home,omitempty" json:"home,omitempty"`
}

// AndroidPush contains public Firebase client options, never sender credentials.
type AndroidPush struct {
	ProjectID     string `yaml:"project_id" json:"project_id"`
	ApplicationID string `yaml:"application_id" json:"application_id"`
	SenderID      string `yaml:"sender_id" json:"sender_id"`
	APIKey        string `yaml:"api_key" json:"api_key"`
}

func (p AndroidPush) ID() string {
	sum := sha256.Sum256([]byte("skidbladnir.android-push.v1\n" + p.ProjectID + "\n" + p.ApplicationID + "\n" + p.SenderID + "\n" + p.APIKey))
	return hex.EncodeToString(sum[:])
}

// Validate is the one check of these options. project_id also names the fcm send
// url, which fcmpush builds from it unchecked.
func (p AndroidPush) Validate() error {
	if !regexp.MustCompile(`^[a-z][a-z0-9-]{4,62}$`).MatchString(p.ProjectID) {
		return errors.New("android_push.project_id: expected a firebase project id: 5–63 lowercase letters, digits or hyphens, starting with a letter")
	}
	for _, field := range []struct{ name, value string }{
		{"application_id", p.ApplicationID}, {"sender_id", p.SenderID}, {"api_key", p.APIKey},
	} {
		valid := len(field.value) > 0 && len(field.value) <= 256
		for _, c := range []byte(field.value) {
			valid = valid && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == ':' || c == '_' || c == '.' || c == '-')
		}
		if !valid {
			return fmt.Errorf("android_push.%s: required 1–256 ascii identifier characters", field.name)
		}
	}
	return nil
}

// Config is config.yaml. Its json form is the fleet's accepted settings.
type Config struct {
	AndroidPush    *AndroidPush `yaml:"android_push,omitempty" json:"android_push,omitempty"`
	Version        int          `yaml:"version" json:"version"`
	Profiles       []Profile    `yaml:"profiles" json:"profiles"`
	Shell          string       `yaml:"shell,omitempty" json:"shell,omitempty"`
	Terminal       string       `yaml:"terminal,omitempty" json:"terminal,omitempty"`
	DefaultMachine string       `yaml:"default_machine,omitempty" json:"default_machine,omitempty"`
}

type Settings struct {
	Shell          string    `json:"shell"`
	Terminal       string    `json:"terminal"`
	DefaultMachine string    `json:"defaultMachine"`
	Profiles       []Profile `json:"profiles"`
}

// Parse admits config.yaml: at most 64 KiB, one yaml document decoded strictly
// into Config (unknown or duplicate keys and wrong value types are errors), then
// the rules the types cannot express. A scalar in a string field keeps its
// literal text, null or "" leaves a field unset, and aliases and merge keys expand.
func Parse(data []byte) (Config, error) {
	if len(data) > 64<<10 {
		return Config{}, errors.New("config: exceeds 64 kib")
	}
	var c Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err := decoder.Decode(&c)
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) {
		return Config{}, errors.New("config: " + strings.Join(typeErr.Errors, "; "))
	}
	if err != nil && !errors.Is(err, io.EOF) { // an empty file is an empty Config
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if !errors.Is(decoder.Decode(new(yaml.Node)), io.EOF) {
		return Config{}, errors.New("config: expected one yaml document")
	}
	if err := validate(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// validate holds the rules the types cannot express and fills each blank label
// with its profile name.
func validate(c *Config) error {
	if c.AndroidPush != nil {
		if err := c.AndroidPush.Validate(); err != nil {
			return err
		}
	}
	if c.Version != 1 {
		return errors.New("version: required integer 1")
	}
	if c.Profiles == nil {
		return errors.New("profiles: required ordered list")
	}
	if c.Shell != "" && !supportedShell(c.Shell) {
		return errors.New("config.shell: expected absolute bash or zsh path")
	}
	if c.Terminal != "" && c.Terminal != "apple-terminal" && c.Terminal != "ghostty" {
		return errors.New("config.terminal: expected apple-terminal or ghostty")
	}
	name := regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	names := map[string]bool{}
	for i := range c.Profiles {
		p := &c.Profiles[i]
		prefix := fmt.Sprintf("profiles[%d]", i)
		if !name.MatchString(p.Name) {
			return fmt.Errorf("%s.name: use 1–32 lowercase letters, digits, underscores or hyphens; start with a letter", prefix)
		}
		if names[p.Name] {
			return fmt.Errorf("%s.name: duplicate profile", prefix)
		}
		names[p.Name] = true
		if p.Label == "" {
			p.Label = p.Name
		}
		if !utf8.ValidString(p.Label) || utf8.RuneCountInString(p.Label) > 64 || strings.TrimSpace(p.Label) == "" || strings.IndexFunc(p.Label, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s.label: use 1–64 visible characters", prefix)
		}
		if strings.TrimSpace(p.Command) == "" || !utf8.ValidString(p.Command) || strings.ContainsRune(p.Command, 0) {
			return fmt.Errorf("%s.command: required command without nul bytes", prefix)
		}
		if p.Provider != "" && p.Provider != "codex" && p.Provider != "claude" {
			return fmt.Errorf("%s.provider: expected codex or claude", prefix)
		}
		if p.ResumeCommand != "" && (strings.TrimSpace(p.ResumeCommand) == "" || !utf8.ValidString(p.ResumeCommand) || strings.ContainsRune(p.ResumeCommand, 0)) {
			return fmt.Errorf("%s.resume_command: expected nonblank command without nul bytes", prefix)
		}
		if p.ResumeCommand != "" && p.Provider == "" {
			return fmt.Errorf("%s.resume_command: requires provider", prefix)
		}
		if p.Home != "" && p.Provider == "" {
			return fmt.Errorf("%s.home: requires provider", prefix)
		}
		if p.Home != "" && ((!filepath.IsAbs(p.Home) && !strings.HasPrefix(p.Home, "~/")) || !utf8.ValidString(p.Home) || strings.ContainsRune(p.Home, 0)) {
			return fmt.Errorf("%s.home: expected absolute path or ~/…", prefix)
		}
	}
	return nil
}

// supportedShell is the startup integration's rule: an absolute bash or zsh path.
func supportedShell(path string) bool {
	base := filepath.Base(path)
	return filepath.IsAbs(path) && (base == "bash" || base == "zsh") && !strings.ContainsRune(path, 0)
}

// ValidateMachines checks the config's one machine reference, default_machine,
// against the fleet's machine names with the fleet's case-insensitive rule.
func (c Config) ValidateMachines(names []string) error {
	if c.DefaultMachine != "" && !slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, c.DefaultMachine) }) {
		return errors.New("default_machine: unknown machine")
	}
	return nil
}

// Resolve derives this host's settings: an empty shell is the login shell, an
// empty terminal apple-terminal, an empty default this host. It is the one check
// of the shell panes exec: a supported path to a regular file this user can
// execute ("" when there is no login shell fails it). Provider homes default and
// expand against home.
func Resolve(c Config, name, home, loginShell string) (Settings, error) {
	s := Settings{Shell: c.Shell, Terminal: c.Terminal, DefaultMachine: c.DefaultMachine, Profiles: append([]Profile{}, c.Profiles...)}
	if s.Shell == "" {
		s.Shell = loginShell
	}
	if s.Terminal == "" {
		s.Terminal = "apple-terminal"
	}
	if s.DefaultMachine == "" {
		s.DefaultMachine = name
	}
	info, err := os.Stat(s.Shell)
	if !supportedShell(s.Shell) || err != nil || !info.Mode().IsRegular() || syscall.Access(s.Shell, 1) != nil { // 1 is X_OK
		return Settings{}, errors.New("local.shell: no executable bash or zsh on this host")
	}
	for i := range s.Profiles {
		p := &s.Profiles[i]
		if p.Provider == "" {
			continue
		}
		if p.Home == "" {
			if p.Provider == "codex" {
				p.Home = filepath.Join(home, ".codex")
			} else {
				p.Home = filepath.Join(home, ".claude")
			}
		} else if strings.HasPrefix(p.Home, "~/") {
			p.Home = filepath.Join(home, p.Home[2:])
		}
		if !filepath.IsAbs(p.Home) {
			return Settings{}, fmt.Errorf("profiles[%d].home: destination home unavailable", i)
		}
	}
	return s, nil
}
