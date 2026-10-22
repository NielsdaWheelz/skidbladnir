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
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const MaximumBytes = 64 * 1024
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
`

type Profile struct {
	Name     string `yaml:"name" json:"name"`
	Label    string `yaml:"label,omitempty" json:"label"`
	Command  string `yaml:"command" json:"command"`
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Home     string `yaml:"home,omitempty" json:"home,omitempty"`
}
type ProfileOverride struct {
	Command *string `yaml:"command,omitempty" json:"command,omitempty"`
	Home    *string `yaml:"home,omitempty" json:"home,omitempty"`
}
type Override struct {
	Shell          string                     `yaml:"shell,omitempty" json:"shell,omitempty"`
	Terminal       string                     `yaml:"terminal,omitempty" json:"terminal,omitempty"`
	DefaultMachine string                     `yaml:"default_machine,omitempty" json:"default_machine,omitempty"`
	Profiles       map[string]ProfileOverride `yaml:"profiles,omitempty" json:"profiles,omitempty"`
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
func (p AndroidPush) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"project_id", p.ProjectID}, {"application_id", p.ApplicationID}, {"sender_id", p.SenderID}, {"api_key", p.APIKey},
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

type Config struct {
	AndroidPush    *AndroidPush        `yaml:"android_push,omitempty" json:"android_push,omitempty"`
	Version        int                 `yaml:"version" json:"version"`
	Profiles       []Profile           `yaml:"profiles" json:"profiles"`
	Shell          string              `yaml:"shell,omitempty" json:"shell,omitempty"`
	Terminal       string              `yaml:"terminal,omitempty" json:"terminal,omitempty"`
	DefaultMachine string              `yaml:"default_machine,omitempty" json:"default_machine,omitempty"`
	Machines       map[string]Override `yaml:"machines,omitempty" json:"machines,omitempty"`
}
type Settings struct {
	Shell          string    `json:"shell"`
	Terminal       string    `json:"terminal"`
	DefaultMachine string    `json:"defaultMachine"`
	Profiles       []Profile `json:"profiles"`
}

func Parse(data []byte) (Config, error) {
	if len(data) > MaximumBytes {
		return Config{}, errors.New("config: exceeds 64 kib")
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if e := decoder.Decode(&document); e != nil {
		return Config{}, errors.New("config: invalid yaml")
	}
	var extra yaml.Node
	if e := decoder.Decode(&extra); !errors.Is(e, io.EOF) {
		return Config{}, errors.New("config: expected one yaml document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return Config{}, errors.New("config: expected a mapping")
	}
	if e := validateShape(document.Content[0], "config"); e != nil {
		return Config{}, e
	}
	if e := validateNode(document.Content[0], ""); e != nil {
		return Config{}, e
	}
	var config Config
	typed := yaml.NewDecoder(bytes.NewReader(data))
	typed.KnownFields(true)
	if e := typed.Decode(&config); e != nil {
		return Config{}, errors.New("config: unknown field or invalid field type")
	}
	if e := validate(&config); e != nil {
		return Config{}, e
	}
	return config, nil
}
func validateNode(node *yaml.Node, path string) error {
	if node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return fmt.Errorf("%s: yaml aliases and merge keys are unsupported", fieldPath(path, "config"))
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("%s: null is unsupported", fieldPath(path, "config"))
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" {
				return fmt.Errorf("%s: invalid mapping key", fieldPath(path, "config"))
			}
			child := fieldPath(path, key.Value)
			if seen[key.Value] {
				return fmt.Errorf("%s: duplicate field", child)
			}
			seen[key.Value] = true
			if e := validateNode(node.Content[i+1], child); e != nil {
				return e
			}
		}
	} else {
		for i, child := range node.Content {
			if e := validateNode(child, fmt.Sprintf("%s[%d]", path, i)); e != nil {
				return e
			}
		}
	}
	return nil
}
func fieldPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
func validate(c *Config) error {
	if c.AndroidPush != nil {
		if e := c.AndroidPush.Validate(); e != nil {
			return e
		}
	}
	if c.Version != 1 {
		return errors.New("version: required integer 1")
	}
	if c.Profiles == nil {
		return errors.New("profiles: required ordered list")
	}
	if e := validateOptions(c.Shell, c.Terminal, "config"); e != nil {
		return e
	}
	keys := map[string]bool{}
	for i := range c.Profiles {
		p := &c.Profiles[i]
		prefix := fmt.Sprintf("profiles[%d]", i)
		if !validKey(p.Name) {
			return fmt.Errorf("%s.name: use 1–32 lowercase letters, digits, underscores or hyphens; start with a letter", prefix)
		}
		if keys[p.Name] {
			return fmt.Errorf("%s.name: duplicate profile", prefix)
		}
		keys[p.Name] = true
		if p.Label == "" {
			p.Label = p.Name
		}
		if !validLabel(p.Label) {
			return fmt.Errorf("%s.label: use 1–64 visible characters", prefix)
		}
		if e := validateCommand(p.Command, prefix+".command"); e != nil {
			return e
		}
		if p.Provider != "" && p.Provider != "codex" && p.Provider != "claude" {
			return fmt.Errorf("%s.provider: expected codex or claude", prefix)
		}
		if p.Home != "" && p.Provider == "" {
			return fmt.Errorf("%s.home: requires provider", prefix)
		}
		if p.Home != "" && !validHome(p.Home) {
			return fmt.Errorf("%s.home: expected absolute path or ~/…", prefix)
		}
	}
	for name, o := range c.Machines {
		prefix := "machines." + name
		if e := validateOptions(o.Shell, o.Terminal, prefix); e != nil {
			return e
		}
		for key, p := range o.Profiles {
			if !keys[key] {
				return fmt.Errorf("%s.profiles.%s: unknown profile", prefix, key)
			}
			if p.Command != nil {
				if e := validateCommand(*p.Command, prefix+".profiles."+key+".command"); e != nil {
					return e
				}
			}
			if p.Home != nil {
				provider := ""
				for _, profile := range c.Profiles {
					if profile.Name == key {
						provider = profile.Provider
					}
				}
				if provider == "" || !validHome(*p.Home) {
					return fmt.Errorf("%s.profiles.%s.home: requires provider and absolute path or ~/…", prefix, key)
				}
			}
		}
	}
	return nil
}
func validateOptions(shell, terminal, path string) error {
	if shell != "" && (!filepath.IsAbs(shell) || (filepath.Base(shell) != "bash" && filepath.Base(shell) != "zsh") || strings.ContainsRune(shell, 0)) {
		return fmt.Errorf("%s.shell: expected absolute bash or zsh path", path)
	}
	if terminal != "" && terminal != "apple-terminal" && terminal != "ghostty" {
		return fmt.Errorf("%s.terminal: expected apple-terminal or ghostty", path)
	}
	return nil
}
func validateCommand(command, path string) error {
	if strings.TrimSpace(command) == "" || !utf8.ValidString(command) || strings.ContainsRune(command, 0) {
		return fmt.Errorf("%s: required command without nul bytes", path)
	}
	return nil
}
func validKey(v string) bool {
	if len(v) == 0 || len(v) > 32 {
		return false
	}
	for i, c := range []byte(v) {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if i > 0 && (c >= '0' && c <= '9' || c == '_' || c == '-') {
			continue
		}
		return false
	}
	return true
}
func validLabel(v string) bool {
	return utf8.ValidString(v) && utf8.RuneCountInString(v) <= 64 && strings.TrimSpace(v) != "" && strings.IndexFunc(v, unicode.IsControl) < 0
}
func validHome(v string) bool {
	return (filepath.IsAbs(v) || strings.HasPrefix(v, "~/")) && utf8.ValidString(v) && !strings.ContainsRune(v, 0)
}

// ValidateMachines admits sparse overrides against the fleet's machine names.
func (c Config) ValidateMachines(names []string) error {
	known := map[string]bool{}
	for _, n := range names {
		known[strings.ToLower(n)] = true
	}
	seen := map[string]bool{}
	for n, o := range c.Machines {
		if seen[strings.ToLower(n)] {
			return fmt.Errorf("machines.%s: duplicate machine override", n)
		}
		seen[strings.ToLower(n)] = true
		if !known[strings.ToLower(n)] {
			return fmt.Errorf("machines.%s: unknown machine", n)
		}
		if o.DefaultMachine != "" && !known[strings.ToLower(o.DefaultMachine)] {
			return fmt.Errorf("machines.%s.default_machine: unknown machine", n)
		}
	}
	if c.DefaultMachine != "" && !known[strings.ToLower(c.DefaultMachine)] {
		return errors.New("default_machine: unknown machine")
	}
	return nil
}
func Resolve(c Config, name, home, loginShell string) (Settings, error) {
	s := Settings{Shell: c.Shell, Terminal: c.Terminal, DefaultMachine: c.DefaultMachine, Profiles: append([]Profile{}, c.Profiles...)}
	var override Override
	hasOverride := false
	for key, value := range c.Machines {
		if strings.EqualFold(key, name) {
			override = value
			hasOverride = true
		}
	}
	if o, ok := override, hasOverride; ok {
		if o.Shell != "" {
			s.Shell = o.Shell
		}
		if o.Terminal != "" {
			s.Terminal = o.Terminal
		}
		if o.DefaultMachine != "" {
			s.DefaultMachine = o.DefaultMachine
		}
		for i := range s.Profiles {
			p := &s.Profiles[i]
			if override, ok := o.Profiles[p.Name]; ok {
				if override.Command != nil {
					p.Command = *override.Command
				}
				if override.Home != nil {
					p.Home = *override.Home
				}
			}
		}
	}
	if s.Shell == "" {
		s.Shell = loginShell
	}
	if s.Terminal == "" {
		s.Terminal = "apple-terminal"
	}
	if s.DefaultMachine == "" {
		s.DefaultMachine = name
	}
	if e := validateOptions(s.Shell, s.Terminal, "local"); e != nil {
		return Settings{}, e
	}
	if s.Shell == "" {
		return Settings{}, errors.New("local.shell: login shell unavailable; configure an absolute bash or zsh path")
	}
	info, e := os.Stat(s.Shell)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return Settings{}, errors.New("local.shell: executable unavailable; fix shell on this host")
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

// validateShape forbids yaml's implicit scalar conversions before typed decode.
func validateShape(n *yaml.Node, path string) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected mapping", path)
	}
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i].Value, n.Content[i+1]
		p := path + "." + k
		shape := "string"
		switch path {
		case "config":
			switch k {
			case "version":
				shape = "integer"
			case "profiles":
				shape = "profiles"
			case "machines":
				shape = "machines"
			case "android_push":
				shape = "android_push"
			case "shell", "terminal", "default_machine":
			default:
				return fmt.Errorf("%s: unknown field", p)
			}
		case "android_push":
			switch k {
			case "project_id", "application_id", "sender_id", "api_key":
			default:
				return fmt.Errorf("%s: unknown field", p)
			}
		case "profile":
			switch k {
			case "name", "label", "command", "provider", "home":
			default:
				return fmt.Errorf("%s: unknown field", p)
			}
		case "override":
			switch k {
			case "shell", "terminal", "default_machine":
			case "profiles":
				shape = "overrides"
			default:
				return fmt.Errorf("%s: unknown field", p)
			}
		case "profile override":
			if k != "command" && k != "home" {
				return fmt.Errorf("%s: unknown field", p)
			}
		}
		switch shape {
		case "android_push":
			if e := validateShape(v, "android_push"); e != nil {
				return e
			}
		case "string":
			if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				return fmt.Errorf("%s: expected string", p)
			}
			if v.Value == "" {
				return fmt.Errorf("%s: expected nonempty string", p)
			}

		case "integer":
			if v.Kind != yaml.ScalarNode || v.Tag != "!!int" {
				return fmt.Errorf("%s: expected integer", p)
			}
		case "profiles":
			if v.Kind != yaml.SequenceNode {
				return fmt.Errorf("%s: expected ordered list", p)
			}
			for j, c := range v.Content {
				if e := validateShape(c, "profile"); e != nil {
					return fmt.Errorf("profiles[%d]: %w", j, e)
				}
			}
		case "machines", "overrides":
			if v.Kind != yaml.MappingNode {
				return fmt.Errorf("%s: expected mapping", p)
			}
			for j := 0; j < len(v.Content); j += 2 {
				if v.Content[j].Tag != "!!str" {
					return fmt.Errorf("%s: expected string key", p)
				}
				next := "override"
				if shape == "overrides" {
					next = "profile override"
				}
				if e := validateShape(v.Content[j+1], next); e != nil {
					return fmt.Errorf("%s.%s: %w", p, v.Content[j].Value, e)
				}
			}
		}
	}
	return nil
}
