package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	maxHookInputBytes     = 4 * 1024 * 1024
	maxProjectedHookBytes = 16 * 1024
	maxObjectiveRunes     = 240
	maxAncestryDepth      = 256
	deliveryTimeout       = 250 * time.Millisecond
)

var runtimeIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type projection uint8

const (
	sessionStarted projection = iota + 1
	promptSubmitted
	activityObserved
	stopObserved
	sessionEnded
)

type fact struct {
	Hook          string
	Projection    projection
	ThreadID      string
	SessionID     string
	TurnID        string
	ToolName      string
	EffectiveCWD  string
	Model         string
	SessionSource string
	Objective     string
}

type processIdentity struct {
	RuntimeID string
	PID       int
	StartTime uint64
	TTY       string
}

type deliveryMessage struct {
	Type          string `json:"type"`
	Hook          string `json:"hook"`
	Projection    string `json:"projection"`
	RuntimeID     string `json:"runtime_id"`
	PID           int    `json:"pid"`
	StartTime     uint64 `json:"start_time"`
	TTY           string `json:"tty"`
	ThreadID      string `json:"thread_id"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	EffectiveCWD  string `json:"effective_cwd,omitempty"`
	Model         string `json:"model,omitempty"`
	SessionSource string `json:"session_source,omitempty"`
	Objective     string `json:"objective,omitempty"`
}

type gapMarker struct {
	Hook          string `json:"hook"`
	Projection    string `json:"projection"`
	RuntimeID     string `json:"runtime_id"`
	PID           int    `json:"pid"`
	StartTime     uint64 `json:"start_time"`
	TTY           string `json:"tty"`
	ThreadID      string `json:"thread_id"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	EffectiveCWD  string `json:"effective_cwd,omitempty"`
	Model         string `json:"model,omitempty"`
	SessionSource string `json:"session_source,omitempty"`
	Objective     string `json:"objective,omitempty"`
}

func Run(input io.Reader, pid int, pinnedExecutable, socketPath, gapDirectory string) error {
	return run(input, pid, pinnedExecutable, socketPath, gapDirectory)
}

func run(input io.Reader, pid int, pinnedExecutable, socketPath, gapDirectory string) error {
	raw, err := readBounded(input, maxHookInputBytes)
	if err != nil {
		return err
	}
	fact, err := decode(raw)
	if err != nil {
		return err
	}
	identity, err := identityFor(pid, pinnedExecutable)
	if err != nil {
		return err
	}
	message := deliveryMessage{
		Type:          "HookFact",
		Hook:          fact.Hook,
		Projection:    fact.Projection.String(),
		RuntimeID:     identity.RuntimeID,
		PID:           identity.PID,
		StartTime:     identity.StartTime,
		TTY:           identity.TTY,
		ThreadID:      fact.ThreadID,
		SessionID:     fact.SessionID,
		TurnID:        fact.TurnID,
		ToolName:      fact.ToolName,
		EffectiveCWD:  fact.EffectiveCWD,
		Model:         fact.Model,
		SessionSource: fact.SessionSource,
		Objective:     fact.Objective,
	}
	if err := deliver(socketPath, message); err != nil {
		if gapErr := writeGap(gapDirectory, gapMarker{
			Hook:          fact.Hook,
			Projection:    fact.Projection.String(),
			RuntimeID:     identity.RuntimeID,
			PID:           identity.PID,
			StartTime:     identity.StartTime,
			TTY:           identity.TTY,
			ThreadID:      fact.ThreadID,
			SessionID:     fact.SessionID,
			TurnID:        fact.TurnID,
			ToolName:      fact.ToolName,
			EffectiveCWD:  fact.EffectiveCWD,
			Model:         fact.Model,
			SessionSource: fact.SessionSource,
			Objective:     fact.Objective,
		}); gapErr != nil {
			return fmt.Errorf("deliver hook fact: %w; write gap marker: %v", err, gapErr)
		}
		return nil
	}
	return nil
}

func readBounded(input io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(input, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read hook input: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("hook input exceeds %d bytes", limit)
	}
	return data, nil
}

func decode(data []byte) (fact, error) {
	fields, err := decodeObject(data)
	if err != nil {
		return fact{}, err
	}
	event, err := requiredString(fields, "hook_event_name")
	if err != nil {
		return fact{}, err
	}
	allowed, projectionFor, err := allowedFor(event)
	if err != nil {
		return fact{}, err
	}
	for name := range fields {
		if !allowed[name] {
			return fact{}, fmt.Errorf("unknown %s field %q", event, name)
		}
	}
	sessionID, err := requiredString(fields, "session_id")
	if err != nil {
		return fact{}, err
	}
	if err := validateCommon(fields, event); err != nil {
		return fact{}, err
	}
	threadID, err := canonicalTranscriptThreadID(fields)
	if err != nil {
		return fact{}, err
	}
	decoded := fact{Hook: event, Projection: projectionFor, ThreadID: threadID, SessionID: sessionID}
	if requiresTurn(event) {
		decoded.TurnID, err = requiredString(fields, "turn_id")
		if err != nil {
			return fact{}, err
		}
	}
	if event == "PostToolUse" {
		decoded.ToolName, err = requiredString(fields, "tool_name")
		if err != nil {
			return fact{}, err
		}
	}
	if event == "UserPromptSubmit" && hasSubagentDiscriminator(fields) {
		decoded.Projection = activityObserved
	}
	if err := validateEvent(fields, event); err != nil {
		return fact{}, err
	}
	if event == "SessionStart" {
		decoded.EffectiveCWD, err = requiredString(fields, "cwd")
		if err != nil {
			return fact{}, err
		}
		decoded.Model, err = requiredString(fields, "model")
		if err != nil {
			return fact{}, err
		}
		decoded.SessionSource, err = requiredString(fields, "source")
		if err != nil {
			return fact{}, err
		}
	}
	if event == "UserPromptSubmit" && !hasSubagentDiscriminator(fields) {
		prompt, promptErr := requiredLargeString(fields, "prompt")
		if promptErr != nil {
			return fact{}, promptErr
		}
		decoded.Objective = objectivePreview(prompt)
	}
	return decoded, nil
}

func decodeObject(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode hook JSON: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' { // justify-type-assertion: json.Decoder.Token exposes delimiters through its required interface return.
		return nil, errors.New("hook input must be one JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("decode hook field name: %w", err)
		}
		name, ok := token.(string) // justify-type-assertion: object keys returned by json.Decoder.Token are strings by the JSON decoder contract.
		if !ok {
			return nil, errors.New("hook field name is not a string")
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, fmt.Errorf("duplicate hook field %q", name)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("decode hook field %q: %w", name, err)
		}
		fields[name] = raw
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("close hook JSON object: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' { // justify-type-assertion: json.Decoder.Token exposes delimiters through its required interface return.
		return nil, errors.New("hook input has invalid JSON object closure")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("hook input contains multiple JSON values")
	}
	return fields, nil
}

func allowedFor(event string) (map[string]bool, projection, error) {
	common := map[string]bool{
		"session_id": true, "transcript_path": true, "cwd": true, "hook_event_name": true,
	}
	with := func(names ...string) map[string]bool {
		allowed := make(map[string]bool, len(common)+len(names))
		for name := range common {
			allowed[name] = true
		}
		for _, name := range names {
			allowed[name] = true
		}
		return allowed
	}
	switch event {
	case "SessionStart":
		return with("model", "permission_mode", "source"), sessionStarted, nil
	case "UserPromptSubmit":
		return with("model", "permission_mode", "turn_id", "prompt", "agent_id", "agent_type"), promptSubmitted, nil
	case "PostToolUse":
		return with("model", "permission_mode", "turn_id", "tool_name", "tool_use_id", "tool_input", "tool_response", "agent_id", "agent_type"), activityObserved, nil
	case "SubagentStart":
		return with("model", "permission_mode", "turn_id", "agent_id", "agent_type"), activityObserved, nil
	case "SubagentStop":
		return with("model", "permission_mode", "turn_id", "agent_id", "agent_type", "agent_transcript_path", "stop_hook_active", "last_assistant_message"), activityObserved, nil
	case "Stop":
		return with("model", "permission_mode", "turn_id", "stop_hook_active", "last_assistant_message"), stopObserved, nil
	case "SessionEnd":
		return with("reason"), sessionEnded, nil
	default:
		return nil, 0, fmt.Errorf("unknown hook event %q", event)
	}
}

func validateCommon(fields map[string]json.RawMessage, event string) error {
	if _, err := requiredString(fields, "cwd"); err != nil {
		return err
	}
	if event == "SessionEnd" {
		return nil
	}
	if _, err := requiredString(fields, "model"); err != nil {
		return err
	}
	permissionMode, err := requiredString(fields, "permission_mode")
	if err != nil {
		return err
	}
	if !knownPermissionMode(permissionMode) {
		return errors.New("invalid permission_mode")
	}
	return nil
}

var canonicalRolloutBase = regexp.MustCompile(`^rollout-([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2})-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:_([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}))?\.jsonl$`)

func canonicalTranscriptThreadID(fields map[string]json.RawMessage) (string, error) {
	raw, ok := fields["transcript_path"]
	if !ok {
		return "", errors.New("missing transcript_path")
	}
	var transcriptPath string
	if err := json.Unmarshal(raw, &transcriptPath); err != nil || transcriptPath == "" {
		return "", errors.New("transcript_path must be a canonical rollout path")
	}
	if !filepath.IsAbs(transcriptPath) || filepath.Clean(transcriptPath) != transcriptPath {
		return "", errors.New("transcript_path must be a canonical absolute path")
	}
	match := canonicalRolloutBase.FindStringSubmatch(filepath.Base(transcriptPath))
	if match == nil {
		return "", errors.New("transcript_path must be a canonical rollout path")
	}
	if _, err := time.Parse("2006-01-02T15-04-05", match[1]); err != nil {
		return "", errors.New("transcript_path must contain a valid rollout timestamp")
	}
	return match[2], nil
}

func hasSubagentDiscriminator(fields map[string]json.RawMessage) bool {
	_, hasAgentID := fields["agent_id"]
	_, hasAgentType := fields["agent_type"]
	return hasAgentID || hasAgentType
}

func validateSubagentDiscriminator(fields map[string]json.RawMessage) error {
	for _, name := range []string{"agent_id", "agent_type"} {
		if raw, ok := fields[name]; ok {
			if _, err := decodeString(raw, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEvent(fields map[string]json.RawMessage, event string) error {
	switch event {
	case "SessionStart":
		source, err := requiredString(fields, "source")
		if err != nil || !knownSessionSource(source) {
			return errors.New("invalid SessionStart source")
		}
	case "SessionEnd":
		reason, err := requiredString(fields, "reason")
		if err != nil || reason != "other" {
			return errors.New("invalid SessionEnd reason")
		}
	case "UserPromptSubmit":
		if _, err := requiredLargeString(fields, "prompt"); err != nil {
			return err
		}
		if err := validateSubagentDiscriminator(fields); err != nil {
			return err
		}
	case "PostToolUse":
		if _, err := requiredString(fields, "tool_use_id"); err != nil {
			return err
		}
		if err := validJSONValue(fields["tool_input"], "tool_input"); err != nil {
			return err
		}
		if err := validJSONValue(fields["tool_response"], "tool_response"); err != nil {
			return err
		}
		if err := validateSubagentDiscriminator(fields); err != nil {
			return err
		}
	case "SubagentStart":
		if _, err := requiredString(fields, "agent_id"); err != nil {
			return err
		}
		if _, err := requiredString(fields, "agent_type"); err != nil {
			return err
		}
	case "SubagentStop":
		if _, err := requiredString(fields, "agent_id"); err != nil {
			return err
		}
		if _, err := requiredString(fields, "agent_type"); err != nil {
			return err
		}
		if err := requiredNullableString(fields, "agent_transcript_path"); err != nil {
			return err
		}
		if err := requiredBool(fields, "stop_hook_active"); err != nil {
			return err
		}
		if err := requiredNullableLargeString(fields, "last_assistant_message"); err != nil {
			return err
		}
	case "Stop":
		if err := requiredBool(fields, "stop_hook_active"); err != nil {
			return err
		}
		if err := requiredNullableLargeString(fields, "last_assistant_message"); err != nil {
			return err
		}
	}
	return nil
}

func validJSONValue(raw json.RawMessage, name string) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func requiredBool(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing %s", name)
	}
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("invalid %s", name)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func requiredNullableString(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing %s", name)
	}
	if bytes.Equal(raw, []byte("null")) {
		return nil
	}
	_, err := decodeString(raw, name)
	return err
}

func requiredNullableLargeString(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing %s", name)
	}
	if bytes.Equal(raw, []byte("null")) {
		return nil
	}
	_, err := decodeLargeString(raw, name)
	return err
}

func knownSessionSource(value string) bool {
	return value == "startup" || value == "resume" || value == "clear" || value == "compact"
}

func knownPermissionMode(value string) bool {
	switch value {
	case "default", "acceptEdits", "plan", "dontAsk", "bypassPermissions":
		return true
	default:
		return false
	}
}

func requiredString(fields map[string]json.RawMessage, name string) (string, error) {
	raw, ok := fields[name]
	if !ok {
		return "", fmt.Errorf("missing %s", name)
	}
	return decodeString(raw, name)
}

func requiredLargeString(fields map[string]json.RawMessage, name string) (string, error) {
	raw, ok := fields[name]
	if !ok {
		return "", fmt.Errorf("missing %s", name)
	}
	return decodeLargeString(raw, name)
}

func decodeString(raw json.RawMessage, name string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" || len(value) > 4096 {
		return "", fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func decodeLargeString(raw json.RawMessage, name string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" || len(value) > maxHookInputBytes {
		return "", fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func objectivePreview(value string) string {
	normalized := norm.NFC.String(value)
	preview := make([]rune, 0, maxObjectiveRunes)
	spacePending := false
	for _, current := range normalized {
		if objectiveSeparator(current) {
			spacePending = len(preview) > 0
			continue
		}
		if spacePending && len(preview) < maxObjectiveRunes {
			preview = append(preview, ' ')
		}
		spacePending = false
		if len(preview) == maxObjectiveRunes {
			break
		}
		preview = append(preview, current)
	}
	return strings.TrimSpace(string(preview))
}

func objectiveSeparator(value rune) bool {
	if unicode.IsSpace(value) || unicode.IsControl(value) || value == '\u2028' || value == '\u2029' {
		return true
	}
	switch value {
	case '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
		'\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

func requiresTurn(event string) bool {
	switch event {
	case "UserPromptSubmit", "PostToolUse", "SubagentStart", "SubagentStop", "Stop":
		return true
	default:
		return false
	}
}

func (value projection) String() string {
	switch value {
	case sessionStarted:
		return "SessionStarted"
	case promptSubmitted:
		return "PromptSubmitted"
	case activityObserved:
		return "ActivityObserved"
	case stopObserved:
		return "StopObserved"
	case sessionEnded:
		return "SessionEnded"
	default:
		panic("invalid hook projection") // justify-defect: the closed hook decoder validated the projection kind before dispatch.
	}
}

func identityFor(pid int, pinnedExecutable string) (processIdentity, error) {
	runtimeID := os.Getenv("SKIDBLADNIR_RUNTIME_ID")
	if !runtimeIDPattern.MatchString(runtimeID) {
		return processIdentity{}, errors.New("missing or invalid SKIDBLADNIR_RUNTIME_ID")
	}
	pinned, err := filepath.EvalSymlinks(pinnedExecutable)
	if err != nil {
		return processIdentity{}, fmt.Errorf("resolve pinned Codex executable: %w", err)
	}
	for depth, candidate := 0, pid; depth < maxAncestryDepth && candidate > 0; depth++ {
		executable, err := filepath.EvalSymlinks(filepath.Join("/proc", strconv.Itoa(candidate), "exe"))
		if err == nil && executable == pinned {
			startTime, err := processStartTime(candidate)
			if err != nil {
				return processIdentity{}, err
			}
			tty, err := processTTY(candidate)
			if err != nil {
				return processIdentity{}, err
			}
			return processIdentity{RuntimeID: runtimeID, PID: candidate, StartTime: startTime, TTY: tty}, nil
		}
		parent, err := parentPID(candidate)
		if err != nil {
			return processIdentity{}, err
		}
		if parent == candidate {
			break
		}
		candidate = parent
	}
	return processIdentity{}, errors.New("no exact pinned Codex ancestor")
}

func processTTY(pid int) (string, error) {
	tty, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", "0"))
	if err != nil {
		return "", fmt.Errorf("read Codex stdin TTY: %w", err)
	}
	if !filepath.IsAbs(tty) || filepath.Clean(tty) != tty {
		return "", errors.New("Codex stdin TTY is not canonical")
	}
	return tty, nil
}

func parentPID(pid int) (int, error) {
	fields, err := procStatFields(pid)
	if err != nil {
		return 0, err
	}
	if len(fields) < 2 {
		return 0, errors.New("short proc stat")
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("parse parent pid: %w", err)
	}
	return parent, nil
}

func processStartTime(pid int) (uint64, error) {
	fields, err := procStatFields(pid)
	if err != nil {
		return 0, err
	}
	if len(fields) < 20 {
		return 0, errors.New("short proc stat")
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || startTime == 0 {
		return 0, errors.New("invalid proc start time")
	}
	return startTime, nil
}

func procStatFields(pid int) ([]string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, fmt.Errorf("read proc stat: %w", err)
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 || end+2 >= len(data) {
		return nil, errors.New("invalid proc stat")
	}
	return strings.Fields(string(data[end+2:])), nil
}

func deliver(socketPath string, message deliveryMessage) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxProjectedHookBytes {
		return fmt.Errorf("projected hook message exceeds %d bytes", maxProjectedHookBytes)
	}
	connection, err := net.DialTimeout("unix", socketPath, deliveryTimeout)
	if err != nil {
		return err
	}
	defer connection.Close()
	deadline := time.Now().Add(deliveryTimeout)
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	if _, err := io.Copy(connection, bytes.NewReader(append(encoded, '\n'))); err != nil {
		return err
	}
	var ack struct {
		Type string `json:"type"`
	}
	decoder := json.NewDecoder(io.LimitReader(connection, 256))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ack); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("hook ACK contains multiple JSON values")
	}
	if ack.Type != "Ack" {
		return errors.New("missing hook ACK")
	}
	return nil
}

func writeGap(directory string, marker gapMarker) error {
	encoded, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxProjectedHookBytes {
		return fmt.Errorf("projected gap marker exceeds %d bytes", maxProjectedHookBytes)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("gap path is not a directory")
	}
	temporary, err := os.CreateTemp(directory, ".gap-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }() // justify-ignore-error: cleanup targets only this unpublished temporary marker.
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close() // justify-ignore-error: returning the chmod error is authoritative.
		return err
	}
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		_ = temporary.Close() // justify-ignore-error: returning the write error is authoritative.
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close() // justify-ignore-error: returning the file sync error is authoritative.
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	finalName := filepath.Join(directory, "gap-"+strings.TrimPrefix(filepath.Base(temporaryName), ".gap-")+".json")
	if _, err := os.Lstat(finalName); err == nil {
		return errors.New("gap marker name collision")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporaryName, finalName); err != nil {
		return err
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return err
	}
	if err := directoryHandle.Sync(); err != nil {
		_ = directoryHandle.Close() // justify-ignore-error: returning the directory sync error is authoritative.
		return err
	}
	return directoryHandle.Close()
}
