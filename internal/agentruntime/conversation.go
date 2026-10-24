package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

type ConversationSource string

const (
	ConversationProcessRecord ConversationSource = "process_record"
	ConversationTitleMatch    ConversationSource = "title_match"
)

type ConversationReason string

const (
	ConversationProfileUnavailable  ConversationReason = "profile_unavailable"
	ConversationMetadataUnavailable ConversationReason = "metadata_unavailable"
	ConversationNoMatch             ConversationReason = "no_match"
	ConversationTiedMatches         ConversationReason = "tied_matches"
	ConversationObservationChanged  ConversationReason = "observation_changed"
)

type ConversationQuery struct {
	PaneKey       string
	ForegroundPID processinfo.PID
	RawTitle      string
	CWD           string
}

// ConversationResult carries either ID/Source or Reason. The selected name stays
// transient and private, solely to close the pane-title observation bracket.
type ConversationResult struct {
	PaneKey string
	ID      string
	Source  ConversationSource
	Reason  ConversationReason
	name    string
}

type conversationCandidate struct {
	id      string
	name    string
	cwd     string
	updated time.Time
}

const conversationRegistryLimit = 64 << 10

var errConversationMetadata = errors.New("conversation metadata unavailable")
var errConversationMetadataChanged = errors.New("conversation metadata changed")

type conversationBudget struct {
	ctx       context.Context
	remaining int64
	entries   int
}

// DiscoverConversations reads one complete, bounded metadata set for the profile.
// Sessions must close native observations before accepting these results.
func DiscoverConversations(ctx context.Context, profile Profile, queries []ConversationQuery) []ConversationResult {
	results := make([]ConversationResult, len(queries))
	for i, query := range queries {
		results[i] = ConversationResult{PaneKey: query.PaneKey, Reason: ConversationMetadataUnavailable}
	}
	if _, err := AccountScope(profile); err != nil {
		for i := range results {
			results[i].Reason = ConversationProfileUnavailable
		}
		return results
	}
	budget := conversationBudget{ctx: ctx, remaining: 64 << 20}
	pending := make([]int, 0, len(queries))
	for i, query := range queries {
		if profile.Provider == ProviderClaude {
			if id, err := readClaudeRegistry(&budget, profile.Home, query.ForegroundPID); err == nil && budget.remaining >= conversationRegistryLimit {
				// Reserve the closing read inside this pass's total byte allowance.
				budget.remaining -= conversationRegistryLimit
				results[i] = ConversationResult{PaneKey: query.PaneKey, ID: id, Source: ConversationProcessRecord}
				continue
			}
		}
		pending = append(pending, i)
	}
	if len(pending) == 0 {
		return results
	}
	var candidates []conversationCandidate
	var err error
	switch profile.Provider {
	case ProviderClaude:
		candidates, err = claudeConversationCandidates(&budget, profile.Home)
	case ProviderCodex:
		candidates, err = codexConversationCandidates(&budget, profile.Home, queries)
	default:
		err = errConversationMetadata
	}
	if err != nil || ctx.Err() != nil {
		return results
	}
	for _, i := range pending {
		query := queries[i]
		queryCWD := conversationCWD(query.CWD)
		bestFull, bestCWD := -1, -1
		var best *conversationCandidate
		tied := false
		for j := range candidates {
			if ctx.Err() != nil {
				return results
			}
			candidate := &candidates[j]
			full, matches := conversationTitleMatch(profile.Provider, candidate.name, query.RawTitle)
			if !matches {
				continue
			}
			fullRank := 0
			if full {
				fullRank = 1
			}
			cwdRank := 1
			if candidate.cwd != "" && queryCWD != "" {
				cwdRank = 0
				if candidate.cwd == queryCWD {
					cwdRank = 2
				}
			}
			if best == nil || fullRank > bestFull || fullRank == bestFull && (cwdRank > bestCWD || cwdRank == bestCWD && candidate.updated.After(best.updated)) {
				best, bestFull, bestCWD, tied = candidate, fullRank, cwdRank, false
			} else if fullRank == bestFull && cwdRank == bestCWD && candidate.updated.Equal(best.updated) && candidate.id != best.id {
				tied = true
			}
		}
		switch {
		case best == nil:
			results[i].Reason = ConversationNoMatch
		case tied:
			results[i].Reason = ConversationTiedMatches
		default:
			results[i] = ConversationResult{PaneKey: query.PaneKey, ID: best.id, Source: ConversationTitleMatch, name: best.name}
		}
	}
	return results
}

// CloseConversationObservations rereads only native records, outside the manager
// gate. A changed or unreadable record withdraws the earlier selected id.
func CloseConversationObservations(ctx context.Context, profile Profile, queries []ConversationQuery, results []ConversationResult) []ConversationResult {
	closed := append([]ConversationResult(nil), results...)
	budget := conversationBudget{ctx: ctx, remaining: 64 << 20}
	for i, result := range closed {
		if result.Source != ConversationProcessRecord {
			continue
		}
		id, err := readClaudeRegistry(&budget, profile.Home, queries[i].ForegroundPID)
		if err != nil || id != result.ID {
			closed[i] = ConversationResult{PaneKey: result.PaneKey, Reason: ConversationObservationChanged}
		}
	}
	return closed
}

func ConversationTitleMatches(provider Provider, result ConversationResult, title string) bool {
	_, matches := conversationTitleMatch(provider, result.name, title)
	return matches
}

func conversationTitleMatch(provider Provider, name, title string) (bool, bool) {
	if name == "" || !utf8.ValidString(title) {
		return false, false
	}
	if provider == ProviderClaude {
		return true, title == "✳ "+name
	}
	return codexConversationTitleMatch(name, title)
}

func ValidConversationID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := range len(id) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if id[i] != '-' {
				return false
			}
		} else if !(id[i] >= '0' && id[i] <= '9' || id[i] >= 'a' && id[i] <= 'f') {
			return false
		}
	}
	return true
}

func conversationCWD(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}
	return canonicalHome(path)
}

func (budget *conversationBudget) read(file *os.File, offset, size int64) ([]byte, error) {
	if err := budget.ctx.Err(); err != nil {
		return nil, err
	}
	if size < 0 || size > budget.remaining {
		return nil, errConversationMetadata
	}
	budget.remaining -= size
	data := make([]byte, size)
	_, err := file.ReadAt(data, offset)
	return data, err
}

func openConversationFile(path string) (*os.File, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errConversationMetadata
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		file.Close()
		return nil, nil, errConversationMetadata
	}
	if !os.SameFile(info, opened) {
		file.Close()
		return nil, nil, errConversationMetadataChanged
	}
	return file, opened, nil
}

func stableConversationFile(path string, file *os.File, before os.FileInfo) bool {
	after, err := file.Stat()
	if err != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	current, err := os.Stat(path)
	return err == nil && os.SameFile(before, current) && before.Size() == current.Size() && before.ModTime().Equal(current.ModTime())
}

func (budget *conversationBudget) wholeFile(path string, limit int64) ([]byte, error) {
	if err := budget.ctx.Err(); err != nil {
		return nil, err
	}
	file, before, err := openConversationFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if before.Size() > limit {
		return nil, errConversationMetadata
	}
	data, err := budget.read(file, 0, before.Size())
	if err != nil || !stableConversationFile(path, file, before) {
		return nil, errConversationMetadata
	}
	return data, nil
}

func (budget *conversationBudget) directory(path string) ([]os.DirEntry, error) {
	if err := budget.ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(16385 - budget.entries)
	budget.entries += len(entries)
	if budget.entries > 16384 || err != nil && !errors.Is(err, io.EOF) {
		return nil, errConversationMetadata
	}
	return entries, nil
}

func decodeConversationRecord(line []byte, target any) error {
	line = bytes.TrimSpace(line)
	if !utf8.Valid(line) || len(line) == 0 || line[0] != '{' {
		return errConversationMetadata
	}
	return json.Unmarshal(line, target)
}

func conversationLines(data []byte, partialStart, partialEnd bool) [][]byte {
	if partialStart {
		_, data, _ = bytes.Cut(data, []byte{'\n'})
	}
	if partialEnd {
		end := bytes.LastIndexByte(data, '\n')
		if end < 0 {
			return nil
		}
		data = data[:end]
	}
	return bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
}
