package agentruntime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apparentlymart/go-textseg/v16/textseg"
)

func codexConversationCandidates(budget *conversationBudget, home string, queries []ConversationQuery) ([]conversationCandidate, error) {
	data, err := budget.wholeFile(filepath.Join(home, "session_index.jsonl"), 8<<20)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	indexed := make(map[string]conversationCandidate)
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if budget.ctx.Err() != nil {
			return nil, budget.ctx.Err()
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record struct {
			ID        string  `json:"id"`
			Name      *string `json:"thread_name"`
			UpdatedAt string  `json:"updated_at"`
		}
		if err := decodeConversationRecord(line, &record); err != nil {
			return nil, err
		}
		id := strings.ToLower(record.ID)
		if !ValidConversationID(id) || record.Name == nil {
			return nil, errConversationMetadata
		}
		var updated time.Time
		if record.UpdatedAt != "" && record.UpdatedAt != "unknown" {
			updated, err = time.Parse(time.RFC3339Nano, record.UpdatedAt)
			if err != nil {
				return nil, errConversationMetadata
			}
		}
		indexed[id] = conversationCandidate{id: id, name: *record.Name, updated: updated}
	}
	matching := make(map[string]conversationCandidate)
	for id, candidate := range indexed {
		for _, query := range queries {
			if budget.ctx.Err() != nil {
				return nil, budget.ctx.Err()
			}
			if _, matches := codexConversationTitleMatch(candidate.name, query.RawTitle); matches {
				matching[id] = candidate
				break
			}
		}
	}
	if len(matching) == 0 {
		return nil, nil
	}
	cwdComplete := true
	seen := make(map[string]bool)
	for _, directory := range []string{"sessions", "archived_sessions"} {
		if err := codexRolloutCWDs(budget, filepath.Join(home, directory), matching, seen, &cwdComplete); err != nil {
			return nil, err
		}
	}
	if len(seen) != len(matching) {
		cwdComplete = false
	}
	candidates := make([]conversationCandidate, 0, len(matching))
	for _, candidate := range matching {
		if !cwdComplete {
			candidate.cwd = ""
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func codexRolloutCWDs(budget *conversationBudget, directory string, candidates map[string]conversationCandidate, seen map[string]bool, complete *bool) error {
	entries, err := budget.directory(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if entry.IsDir() {
			if err := codexRolloutCWDs(budget, path, candidates, seen, complete); err != nil {
				return err
			}
			continue
		}
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") || len(name) < 36+len("rollout-.jsonl") {
			continue
		}
		id := strings.ToLower(name[len(name)-len(".jsonl")-36 : len(name)-len(".jsonl")])
		candidate, wanted := candidates[id]
		if !wanted {
			continue
		}
		cwd, err := readCodexRolloutCWD(budget, path, id)
		if errors.Is(err, errConversationMetadataChanged) {
			return err
		}
		if err != nil || seen[id] || cwd == "" {
			*complete = false
		}
		seen[id] = true
		candidate.cwd = cwd
		candidates[id] = candidate
	}
	return nil
}

func readCodexRolloutCWD(budget *conversationBudget, path, id string) (string, error) {
	file, before, err := openConversationFile(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var line []byte
	found := false
	limit := min(before.Size(), 1<<20)
	for offset := int64(0); offset < limit; {
		data, err := budget.read(file, offset, min(limit-offset, 4096))
		if err != nil {
			if !stableConversationFile(path, file, before) {
				return "", errConversationMetadataChanged
			}
			return "", err
		}
		offset += int64(len(data))
		var record []byte
		record, _, found = bytes.Cut(data, []byte{'\n'})
		line = append(line, record...)
		if found {
			break
		}
	}
	if !stableConversationFile(path, file, before) {
		return "", errConversationMetadataChanged
	}
	if !found && before.Size() > limit {
		return "", errConversationMetadata
	}
	var record struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"payload"`
	}
	if err := decodeConversationRecord(line, &record); err != nil || record.Type != "session_meta" || strings.ToLower(record.Payload.ID) != id {
		return "", errConversationMetadata
	}
	return conversationCWD(record.Payload.CWD), nil
}

const codexSpinnerRunes = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

func codexConversationTitleMatch(name, title string) (bool, bool) {
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) >= 240 {
		return false, false
	}
	name = strings.TrimSpace(name)
	structuralName := strings.Contains(name, " | ") || strings.ContainsAny(name, "●"+codexSpinnerRunes)
	data := []byte(name)
	count, shortenedName := 0, ""
	for offset := 0; offset < len(data); {
		advance, _, _ := textseg.ScanGraphemeClusters(data[offset:], true)
		offset += advance
		count++
		if count == 45 {
			shortenedName = name[:offset] + "..."
		}
		if count > 48 {
			break
		}
	}
	full := count <= 48
	if !full {
		name = shortenedName
	}
	name = sanitizeCodexConversationName(name)
	if name == "" {
		return full, false
	}
	if structuralName {
		return full, title == name
	}
notices:
	for _, prefix := range []string{"[ ! ] Action Required", "[ . ] Action Required"} {
		for _, decorated := range []string{prefix, "● " + prefix} {
			if title == decorated {
				title = ""
				break notices
			}
			if rest, found := strings.CutPrefix(title, decorated+" | "); found {
				title = rest
				break notices
			}
		}
	}
	if title == name {
		return full, true
	}
	for _, segment := range strings.Split(title, " | ") {
		words := strings.Split(segment, " ")
		start := 0
		for i := 0; i <= len(words); i++ {
			activity := i < len(words) && (words[i] == "●" || isCodexSpinner(words[i]))
			if i != len(words) && !activity {
				continue
			}
			if strings.Join(words[start:i], " ") == name {
				return full, true
			}
			if activity && words[i] == "●" && i+1 < len(words) && isCodexSpinner(words[i+1]) {
				i++ // The microphone plus spinner is one activity token.
			}
			start = i + 1
		}
	}
	return full, false
}

func isCodexSpinner(value string) bool {
	return utf8.RuneCountInString(value) == 1 && strings.Contains(codexSpinnerRunes, value)
}

// Pinned to codex 0.163.0's Unicode terminal-title sanitizer. Name shortening
// precedes this sanitation; changing the order changes provider-visible names.
func sanitizeCodexConversationName(value string) string {
	var result strings.Builder
	count, pendingSpace := 0, false
	for _, r := range value {
		if unicode.IsSpace(r) {
			pendingSpace = result.Len() != 0
			continue
		}
		if unicode.IsControl(r) || r == '\u00ad' || r == '\u034f' || r == '\u061c' || r == '\u180e' ||
			r >= '\u200b' && r <= '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2060' && r <= '\u206f' ||
			r >= '\ufe00' && r <= '\ufe0f' || r == '\ufeff' || r >= '\ufff9' && r <= '\ufffb' ||
			r >= '\U0001bca0' && r <= '\U0001bca3' || r >= '\U000e0100' && r <= '\U000e01ef' {
			continue
		}
		if pendingSpace && 240-count > 1 {
			result.WriteByte(' ')
			count++
			pendingSpace = false
		}
		if count >= 240 {
			break
		}
		result.WriteRune(r)
		count++
	}
	return result.String()
}
