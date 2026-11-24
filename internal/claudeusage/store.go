// Package claudeusage persists the latest quota fields for each Claude account.
package claudeusage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

// Dir is the private state shared by the quota store, machine bindings and native block.
func Dir(stateDir string) string {
	return filepath.Join(stateDir, "claude-quota")
}

func accountPath(stateDir, account string) (string, error) {
	if !filepath.IsLocal(account) || filepath.Base(account) != account {
		return "", errors.New("invalid claude usage account filename")
	}
	return filepath.Join(Dir(stateDir), "accounts", account+".json"), nil
}

// Load reads the last coherent record. A verified login can precede its first reading.
func Load(stateDir, account string) (api.ClaudeUsage, error) {
	if account == "" {
		return api.ClaudeUsage{}, nil
	}
	path, err := accountPath(stateDir, account)
	if err != nil {
		return api.ClaudeUsage{}, err
	}
	data, err := privatefile.Read(path, 16<<10)
	if errors.Is(err, os.ErrNotExist) {
		return api.ClaudeUsage{Account: account}, nil
	}
	if err != nil {
		return api.ClaudeUsage{}, err
	}
	var saved api.ClaudeUsage
	if err := json.Unmarshal(data, &saved); err != nil {
		return api.ClaudeUsage{}, err
	}
	if saved.Account != account {
		return api.ClaudeUsage{}, errors.New("stored claude usage belongs to another account")
	}
	return saved, nil
}

// Others returns saved accounts without a current profile row in filename order.
// A damaged record leaves all other readable accounts available.
func Others(stateDir string, represented map[string]bool) ([]api.ClaudeUsage, error) {
	entries, err := os.ReadDir(filepath.Join(Dir(stateDir), "accounts"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var records []api.ClaudeUsage
	for _, entry := range entries {
		account, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !entry.Type().IsRegular() || represented[account] {
			continue
		}
		record, readErr := Load(stateDir, account)
		if readErr != nil {
			err = errors.Join(err, readErr)
		} else if record.HasReading() {
			records = append(records, record)
		}
	}
	return records, err
}

// Remember serializes read-merge-replace across footer and gateway processes.
// A storage error still returns the best reading available to this caller.
func Remember(ctx context.Context, stateDir string, reading api.ClaudeUsage) (_ api.ClaudeUsage, err error) {
	if reading.Account == "" {
		return api.ClaudeUsage{}, nil
	}
	if !reading.HasReading() {
		saved, err := Load(stateDir, reading.Account)
		if err != nil {
			return reading, err
		}
		return saved, nil
	}
	path, err := accountPath(stateDir, reading.Account)
	if err != nil {
		return reading, err
	}
	lock, lockErr := privatefile.Lock(ctx, path+".lock")
	if lock != nil {
		defer func() { err = errors.Join(err, lock.Close()) }()
	}
	saved, err := Load(stateDir, reading.Account)
	if err != nil {
		return reading, errors.Join(lockErr, err)
	}
	merged := saved
	merged.Merge(reading)
	if lockErr != nil || merged == saved {
		return merged, lockErr
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return merged, err
	}
	return merged, privatefile.WriteFile(path, data)
}
