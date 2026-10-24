package agentruntime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func readClaudeRegistry(budget *conversationBudget, home string, pid processinfo.PID) (string, error) {
	if pid <= 0 {
		return "", errConversationMetadata
	}
	data, err := budget.wholeFile(filepath.Join(home, "sessions", strconv.Itoa(int(pid))+".json"), conversationRegistryLimit)
	if err != nil {
		return "", err
	}
	var record struct {
		PID       processinfo.PID `json:"pid"`
		SessionID string          `json:"sessionId"`
	}
	if err := decodeConversationRecord(data, &record); err != nil || record.PID != pid {
		return "", errConversationMetadata
	}
	id := strings.ToLower(record.SessionID)
	if !ValidConversationID(id) {
		return "", errConversationMetadata
	}
	return id, nil
}

func claudeConversationCandidates(budget *conversationBudget, home string) ([]conversationCandidate, error) {
	projects := filepath.Join(home, "projects")
	directories, err := budget.directory(projects)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var candidates []conversationCandidate
	for _, directory := range directories {
		if !directory.IsDir() {
			continue
		}
		project := filepath.Join(projects, directory.Name())
		entries, err := budget.directory(project)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			id := strings.ToLower(strings.TrimSuffix(entry.Name(), ".jsonl"))
			if !strings.HasSuffix(entry.Name(), ".jsonl") || !ValidConversationID(id) || !entry.Type().IsRegular() {
				continue
			}
			candidate, sidechain, err := readClaudeConversation(budget, filepath.Join(project, entry.Name()), id)
			if err != nil {
				return nil, err
			}
			if sidechain {
				continue
			}
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

func readClaudeConversation(budget *conversationBudget, path, id string) (conversationCandidate, bool, error) {
	file, before, err := openConversationFile(path)
	if err != nil {
		return conversationCandidate{}, false, err
	}
	defer file.Close()
	const window = 64 << 10
	headSize := min(before.Size(), window)
	head, err := budget.read(file, 0, headSize)
	if err != nil {
		return conversationCandidate{}, false, err
	}
	var tail []byte
	tailOffset := max(headSize, before.Size()-window)
	if tailOffset < before.Size() {
		tail, err = budget.read(file, tailOffset, before.Size()-tailOffset)
		if err != nil {
			return conversationCandidate{}, false, err
		}
	}
	if !stableConversationFile(path, file, before) {
		return conversationCandidate{}, false, errConversationMetadata
	}
	var lines [][]byte
	if tailOffset == headSize {
		lines = conversationLines(append(head, tail...), false, false)
	} else {
		lines = append(conversationLines(head, false, true), conversationLines(tail, true, false)...)
	}
	candidate := conversationCandidate{id: id, updated: before.ModTime()}
	var customTitle, aiTitle string
	var hasCustomTitle bool
	sidechain := false
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record struct {
			Type        string  `json:"type"`
			CustomTitle *string `json:"customTitle"`
			AITitle     *string `json:"aiTitle"`
			CWD         string  `json:"cwd"`
			Sidechain   bool    `json:"isSidechain"`
		}
		if err := decodeConversationRecord(line, &record); err != nil {
			return conversationCandidate{}, false, err
		}
		sidechain = sidechain || record.Sidechain
		if record.CWD != "" {
			candidate.cwd = conversationCWD(record.CWD)
		}
		switch record.Type {
		case "custom-title":
			if record.CustomTitle == nil {
				return conversationCandidate{}, false, errConversationMetadata
			}
			customTitle, hasCustomTitle = *record.CustomTitle, true
		case "ai-title":
			if record.AITitle == nil {
				return conversationCandidate{}, false, errConversationMetadata
			}
			aiTitle = *record.AITitle
		}
	}
	candidate.name = aiTitle
	if hasCustomTitle {
		candidate.name = customTitle
	}
	return candidate, sidechain, nil
}
