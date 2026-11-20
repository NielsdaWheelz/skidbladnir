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

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

// ReadClaudeProfile takes the canonical Claude home for attribution, including for the
// default profile, whose global config actually lives one directory higher.
func ReadClaudeProfile(configDir string) api.ClaudeUsage {
	if home, err := os.UserHomeDir(); err == nil {
		defaultDir := filepath.Join(home, ".claude")
		resolved, _ := filepath.EvalSymlinks(defaultDir)
		if configDir == defaultDir || configDir == resolved {
			return readClaudeCache("", true)
		}
	}
	return ReadClaudeCache(configDir)
}

// ReadClaudeCache reads the standard Claude Code global config for the given
// CLAUDE_CONFIG_DIR. An empty argument selects the native default layout.
// A verified login survives a missing or mismatched usage sample.
func ReadClaudeCache(configDir string) api.ClaudeUsage {
	return readClaudeCache(configDir, false)
}

func readClaudeCache(configDir string, gatewayDefault bool) api.ClaudeUsage {
	var reading api.ClaudeUsage
	globalDir := configDir
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return reading
		}
		globalDir, configDir = home, filepath.Join(home, ".claude")
	}
	if !filepath.IsAbs(globalDir) {
		return reading
	}
	path := filepath.Join(globalDir, ".claude.json")
	legacy := filepath.Join(configDir, ".config.json")
	if info, err := os.Stat(legacy); err == nil {
		if info.Mode().IsRegular() && info.Size() > 0 {
			path = legacy
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return reading
	}
	// A FIFO at either selected path must not stall the quota request or footer.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return reading
	}
	defer file.Close()
	const maxBytes = 8 << 20 // Claude Code's own global-config backup bound.
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBytes {
		return reading
	}
	// The gateway knows the profile home but cannot see whether a command set
	// CLAUDE_CONFIG_DIR to the default directory. Claude then uses the inner
	// .claude.json. A shared legacy file or the same inode resolves both cases.
	if gatewayDefault && path != legacy {
		other, err := os.Stat(filepath.Join(configDir, ".claude.json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return reading
		}
		if other != nil && !os.SameFile(info, other) {
			return reading
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || len(data) > maxBytes {
		return reading
	}
	var config struct {
		Account struct {
			UUID string `json:"accountUuid"`
		} `json:"oauthAccount"`
		Cache json.RawMessage `json:"cachedUsageUtilization"`
	}
	if json.Unmarshal(data, &config) != nil || strings.TrimSpace(config.Account.UUID) == "" {
		return reading
	}
	reading.Account = fmt.Sprintf("%x", sha256.Sum256([]byte("claude-account\x00"+config.Account.UUID)))
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
		return reading
	}
	fetchedAt := time.UnixMilli(cache.FetchedAtMS).UTC()
	planWindow := func(raw json.RawMessage) api.ClaudeWindow {
		var window struct {
			Used  json.RawMessage `json:"utilization"`
			Reset json.RawMessage `json:"resets_at"`
		}
		if json.Unmarshal(raw, &window) != nil {
			return api.ClaudeWindow{}
		}
		return claudeWindow(window.Used, window.Reset, fetchedAt)
	}
	reading.FiveHour = planWindow(cache.Utilization.FiveHour)
	reading.SevenDay = planWindow(cache.Utilization.SevenDay)
	reading.FableSevenDay = fableWindow(cache.Utilization.Limits, fetchedAt)
	return reading
}

// Claude Code recognizes the family by its model display name, including versions.
var fableModel = regexp.MustCompile(`(?i)^fable\b`)

func fableWindow(raw json.RawMessage, fetchedAt time.Time) api.ClaudeWindow {
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil {
		return api.ClaudeWindow{}
	}
	var found bool
	var window api.ClaudeWindow
	for _, row := range rows {
		var limit struct {
			Kind  string `json:"kind"`
			Scope struct {
				Model struct {
					DisplayName string `json:"display_name"`
				} `json:"model"`
				Surface json.RawMessage `json:"surface"`
			} `json:"scope"`
			Percent json.RawMessage `json:"percent"`
			Reset   json.RawMessage `json:"resets_at"`
		}
		if json.Unmarshal(row, &limit) != nil || limit.Kind != "weekly_scoped" ||
			!fableModel.MatchString(limit.Scope.Model.DisplayName) || !absent(limit.Scope.Surface) {
			continue
		}
		if found {
			return api.ClaudeWindow{}
		}
		found = true
		window = claudeWindow(limit.Percent, limit.Reset, fetchedAt)
	}
	return window
}

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
