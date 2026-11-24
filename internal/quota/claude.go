package quota

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

type ClaudeSample struct {
	Reading      api.ClaudeUsage
	ReadState    string
	FetchedAt    time.Time // valid even when Claude supplied no meter values
	path         string    // selected native config; an atomic replacement keeps its path
	organization string
	plan         claudePlan
}

type claudePlan uint8

const (
	unsupportedPlan claudePlan = iota
	personalPlan
	teamPlan
)

func quotaPlan(subscription string) claudePlan {
	switch subscription {
	case "pro", "max":
		return personalPlan
	case "team":
		return teamPlan
	default:
		return unsupportedPlan
	}
}

func ReadClaudeProfile(profile agentruntime.Profile) ClaudeSample {
	dir := profile.ClaudeConfigDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ClaudeSample{ReadState: "unavailable"}
		}
		dir = filepath.Join(home, ".claude")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	// The default global config lives outside .claude and can outlast it.
	if errors.Is(err, os.ErrNotExist) && profile.ClaudeConfigDir == "" {
		resolved = dir
	} else if err != nil {
		return ClaudeSample{ReadState: "unavailable"}
	}
	if resolved != profile.Home {
		return ClaudeSample{ReadState: "unavailable"}
	}
	return readClaudeCache(profile.ClaudeConfigDir, profile.ClaudeConfigDir == "")
}

// The footer knows exactly which CLAUDE_CONFIG_DIR its caller supplied.
func ReadClaudeCache(configDir string) api.ClaudeUsage {
	return readClaudeCache(configDir, false).Reading
}

func readClaudeCache(configDir string, gatewayDefault bool) ClaudeSample {
	sample := ClaudeSample{ReadState: "unavailable"}
	globalDir := configDir
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return sample
		}
		globalDir, configDir = home, filepath.Join(home, ".claude")
	}
	if !filepath.IsAbs(globalDir) {
		return sample
	}
	path := filepath.Join(globalDir, ".claude.json")
	alternate := filepath.Join(configDir, ".config.json")
	if info, err := os.Stat(alternate); err == nil {
		if info.Mode().IsRegular() && info.Size() > 0 {
			path = alternate
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return sample
	}
	// A FIFO at either selected path must not stall the quota request or footer.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return sample
	}
	defer file.Close()
	const maxBytes = 8 << 20 // Claude Code's own global-config backup bound.
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBytes {
		return sample
	}
	// The gateway knows the profile home but cannot see whether a command set
	// CLAUDE_CONFIG_DIR to the default directory. Claude then uses the inner
	// .claude.json. A shared .config.json or the same inode resolves both cases.
	if gatewayDefault && path != alternate {
		other, err := os.Stat(filepath.Join(configDir, ".claude.json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return sample
		}
		if other != nil && !os.SameFile(info, other) {
			return sample
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || len(data) > maxBytes {
		return sample
	}
	// A JSON null config, or a malformed login object, is unreadable. Only an
	// absent/null login in a valid config clears the binding.
	var config *struct {
		Account *struct {
			UUID             string `json:"accountUuid"`
			Organization     string `json:"organizationUuid"`
			OrganizationType string `json:"organizationType"`
		} `json:"oauthAccount"`
		Cache json.RawMessage `json:"cachedUsageUtilization"`
	}
	if json.Unmarshal(data, &config) != nil || config == nil {
		return sample
	}
	if config.Account == nil {
		sample.ReadState = "no_login"
		return sample
	}
	if strings.TrimSpace(config.Account.UUID) == "" {
		return sample
	}
	sample.path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return sample
	}
	sample.ReadState = "ok"
	sample.organization = config.Account.Organization
	if subscription, ok := strings.CutPrefix(config.Account.OrganizationType, "claude_"); ok {
		sample.plan = quotaPlan(subscription)
	}
	sample.Reading.Account = fmt.Sprintf("%x", sha256.Sum256([]byte("claude-account\x00"+config.Account.UUID)))
	var cache struct {
		AccountUUID string `json:"accountUuid"`
		FetchedAtMS int64  `json:"fetchedAtMs"`
		Utilization *struct {
			FiveHour json.RawMessage `json:"five_hour"`
			SevenDay json.RawMessage `json:"seven_day"`
			Limits   json.RawMessage `json:"limits"`
		} `json:"utilization"`
	}
	if json.Unmarshal(config.Cache, &cache) != nil || cache.AccountUUID != config.Account.UUID ||
		cache.Utilization == nil || cache.FetchedAtMS <= 0 || cache.FetchedAtMS > time.Now().UnixMilli() {
		return sample
	}
	sample.FetchedAt = time.UnixMilli(cache.FetchedAtMS).UTC()
	rows := claudeMeterRows(cache.Utilization.Limits, sample.FetchedAt)
	sample.Reading.FiveHour = withNamedFields(rows["session"], cache.Utilization.FiveHour, sample.FetchedAt)
	sample.Reading.SevenDay = withNamedFields(rows["weekly_all"], cache.Utilization.SevenDay, sample.FetchedAt)
	sample.Reading.FableSevenDay = rows["weekly_scoped"]
	return sample
}

func claudeMeterRows(raw json.RawMessage, fetchedAt time.Time) map[string]api.ClaudeWindow {
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	windows := make(map[string]api.ClaudeWindow)
	for _, raw := range rows {
		var row struct {
			Kind    string                     `json:"kind"`
			Scope   map[string]json.RawMessage `json:"scope"`
			Percent json.RawMessage            `json:"percent"`
			Reset   json.RawMessage            `json:"resets_at"`
		}
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		for key, value := range row.Scope {
			if absent(value) {
				delete(row.Scope, key)
			}
		}
		switch row.Kind {
		case "session", "weekly_all":
			if len(row.Scope) != 0 {
				continue
			}
		case "weekly_scoped":
			var model struct {
				DisplayName string `json:"display_name"`
			}
			if len(row.Scope) != 1 || json.Unmarshal(row.Scope["model"], &model) != nil || !fableModel.MatchString(model.DisplayName) {
				continue
			}
		default:
			continue
		}
		if _, duplicate := windows[row.Kind]; duplicate {
			windows[row.Kind] = api.ClaudeWindow{}
		} else {
			windows[row.Kind] = claudeWindow(row.Percent, row.Reset, fetchedAt)
		}
	}
	return windows
}

func withNamedFields(row api.ClaudeWindow, raw json.RawMessage, fetchedAt time.Time) api.ClaudeWindow {
	var named struct {
		Used  json.RawMessage `json:"utilization"`
		Reset json.RawMessage `json:"resets_at"`
	}
	if json.Unmarshal(raw, &named) != nil {
		return row
	}
	fallback := claudeWindow(named.Used, named.Reset, fetchedAt)
	if row.Usage == nil {
		row.Usage = fallback.Usage
	}
	if row.Reset == nil {
		row.Reset = fallback.Reset
	}
	return row
}

// Claude identifies the family by its display name, including version labels.
var fableModel = regexp.MustCompile(`(?i)^fable\b`)

func claudeWindow(percent, reset json.RawMessage, fetchedAt time.Time) api.ClaudeWindow {
	var window api.ClaudeWindow
	var used *float64
	if json.Unmarshal(percent, &used) == nil && used != nil && *used >= 0 && *used <= 100 {
		window.Usage = &api.ClaudeAmount{UsedPercent: *used, FetchedAt: fetchedAt}
	}
	var date time.Time
	if json.Unmarshal(reset, &date) == nil && !date.IsZero() {
		utc := date.UTC()
		if utc.Year() >= 1 && utc.Year() <= 9999 {
			window.Reset = &api.ClaudeReset{ResetsAt: utc, FetchedAt: fetchedAt}
		}
	}
	return window
}
