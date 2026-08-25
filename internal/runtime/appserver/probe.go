// Package appserver contains the closed P0 Codex App Server probe exchange.
package appserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	pinnedCodexVersion = "0.149.1"
	fixedCWD           = "/home/niels/src"
	maximumFrameBytes  = 1 << 20
	threadListLimit    = 100
)

var (
	errProtocol  = errors.New("Codex App Server protocol mismatch")
	errTransport = errors.New("Codex App Server transport failure")
)

// ThreadRef is the only thread data retained by the probe. Both fields are
// opaque App Server identifiers; the probe deliberately exposes no content.
type ThreadRef struct {
	ThreadID  string
	SessionID string
}

// ThreadStatus is the content-free status projection returned by the App
// Server. ActiveFlags is only populated for an active thread.
type ThreadStatus struct {
	Type        string
	ActiveFlags []string
}

// ThreadSummary is the only list/read projection retained by this adapter.
// Nil fork and parent references represent the App Server's nullable fields.
type ThreadSummary struct {
	ThreadRef
	CWD            string
	CreatedAt      int64
	Status         ThreadStatus
	ForkedFromID   *string
	ParentThreadID *string
}

// ProbeEmptyThread runs the fixed P0 App Server exchange. It only writes
// initialize, an empty thread/start, and thread/unsubscribe. It is a protocol
// adapter, not evidence that a live pinned Codex process or stock TUI resume
// has succeeded.
func ProbeEmptyThread(connection io.ReadWriter) (ThreadRef, error) {
	reader := bufio.NewReaderSize(connection, maximumFrameBytes+1)
	if err := initializeConnection(reader, connection); err != nil {
		return ThreadRef{}, err
	}

	started, err := exchange(reader, connection, 2, "thread/start", threadStartParams{
		ApprovalPolicy: "never",
		CWD:            fixedCWD,
		Sandbox:        "danger-full-access",
	})
	if err != nil {
		return ThreadRef{}, err
	}
	ref, err := validateThreadStart(started)
	if err != nil {
		return ThreadRef{}, err
	}

	unsubscribed, err := exchange(reader, connection, 3, "thread/unsubscribe", threadUnsubscribeParams{ThreadID: ref.ThreadID})
	if err != nil {
		return ThreadRef{}, err
	}
	if err := validateUnsubscribe(unsubscribed); err != nil {
		return ThreadRef{}, err
	}
	return ref, nil
}

// ListThreadSummaries reads exactly one fixed-size App Server page. It never
// follows a cursor or exposes titles, previews, turns, or other content.
func ListThreadSummaries(connection io.ReadWriter) ([]ThreadSummary, error) {
	reader := bufio.NewReaderSize(connection, maximumFrameBytes+1)
	if err := initializeConnection(reader, connection); err != nil {
		return nil, err
	}
	result, err := exchange(reader, connection, 2, "thread/list", threadListParams{
		CWD:         fixedCWD,
		Limit:       threadListLimit,
		SourceKinds: []string{"appServer"},
	})
	if err != nil {
		return nil, err
	}
	return validateThreadList(result)
}

// ReadThreadSummary reads one known App Server thread without turns.
func ReadThreadSummary(connection io.ReadWriter, reference ThreadRef) (ThreadSummary, error) {
	if reference.ThreadID == "" || len(reference.ThreadID) > 256 {
		return ThreadSummary{}, errProtocol
	}
	reader := bufio.NewReaderSize(connection, maximumFrameBytes+1)
	if err := initializeConnection(reader, connection); err != nil {
		return ThreadSummary{}, err
	}
	result, err := exchange(reader, connection, 2, "thread/read", threadReadParams{IncludeTurns: false, ThreadID: reference.ThreadID})
	if err != nil {
		return ThreadSummary{}, err
	}
	return validateThreadRead(result)
}

type clientRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type clientNotification struct {
	Method string `json:"method"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeParams struct {
	ClientInfo clientInfo `json:"clientInfo"`
}

type threadStartParams struct {
	ApprovalPolicy string `json:"approvalPolicy"`
	CWD            string `json:"cwd"`
	Sandbox        string `json:"sandbox"`
}

type threadUnsubscribeParams struct {
	ThreadID string `json:"threadId"`
}

type threadListParams struct {
	CWD         string   `json:"cwd"`
	Limit       int      `json:"limit"`
	SourceKinds []string `json:"sourceKinds"`
}

type threadReadParams struct {
	IncludeTurns bool   `json:"includeTurns"`
	ThreadID     string `json:"threadId"`
}

func initializeConnection(reader *bufio.Reader, connection io.Writer) error {
	initialize, err := exchange(reader, connection, 1, "initialize", initializeParams{
		ClientInfo: clientInfo{Name: "skidbladnir", Version: pinnedCodexVersion},
	})
	if err != nil {
		return err
	}
	if err := validateInitialize(initialize); err != nil {
		return err
	}
	return sendNotification(connection, "initialized")
}

func exchange(reader *bufio.Reader, writer io.Writer, id int64, method string, params any) (json.RawMessage, error) {
	encoded, err := json.Marshal(clientRequest{ID: id, Method: method, Params: params})
	if err != nil || len(encoded)+1 > maximumFrameBytes {
		return nil, errProtocol
	}
	encoded = append(encoded, '\n')
	if err := writeAll(writer, encoded); err != nil {
		return nil, errTransport
	}

	frame, err := readFrame(reader)
	if err != nil {
		return nil, errTransport
	}
	fields, err := object(frame)
	if err != nil || !hasExactly(fields, "id", "result") {
		return nil, errProtocol
	}
	responseID, err := integer(fields["id"])
	if err != nil || responseID != id {
		return nil, errProtocol
	}
	return fields["result"], nil
}

func sendNotification(writer io.Writer, method string) error {
	encoded, err := json.Marshal(clientNotification{Method: method})
	if err != nil || len(encoded)+1 > maximumFrameBytes {
		return errProtocol
	}
	if err := writeAll(writer, append(encoded, '\n')); err != nil {
		return errTransport
	}
	return nil
}

func validateInitialize(result json.RawMessage) error {
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "codexHome", "platformFamily", "platformOs", "userAgent") {
		return errProtocol
	}
	for _, name := range []string{"codexHome", "platformFamily", "platformOs", "userAgent"} {
		value, err := stringValue(fields[name])
		if err != nil || value == "" {
			return errProtocol
		}
	}
	return nil
}

func validateThreadStart(result json.RawMessage) (ThreadRef, error) {
	fields, err := object(result)
	if err != nil || !hasOnly(fields, "approvalPolicy", "approvalsReviewer", "cwd", "instructionSources", "model", "modelProvider", "reasoningEffort", "sandbox", "serviceTier", "thread") || !hasRequired(fields, "approvalPolicy", "approvalsReviewer", "cwd", "model", "modelProvider", "sandbox", "thread") {
		return ThreadRef{}, errProtocol
	}
	approvalPolicy, err := stringValue(fields["approvalPolicy"])
	if err != nil || approvalPolicy != "never" {
		return ThreadRef{}, errProtocol
	}
	cwd, err := stringValue(fields["cwd"])
	if err != nil || cwd != fixedCWD {
		return ThreadRef{}, errProtocol
	}
	if reviewer, err := stringValue(fields["approvalsReviewer"]); err != nil || reviewer != "user" {
		return ThreadRef{}, errProtocol
	}
	if _, err := nonEmptyString(fields["model"]); err != nil {
		return ThreadRef{}, errProtocol
	}
	if _, err := nonEmptyString(fields["modelProvider"]); err != nil {
		return ThreadRef{}, errProtocol
	}
	if err := validateDangerFullAccess(fields["sandbox"]); err != nil {
		return ThreadRef{}, errProtocol
	}
	summary, err := validateThread(fields["thread"])
	if err != nil || summary.ForkedFromID != nil || summary.ParentThreadID != nil {
		return ThreadRef{}, errProtocol
	}
	return summary.ThreadRef, nil
}

func validateThread(raw json.RawMessage) (ThreadSummary, error) {
	fields, err := object(raw)
	if err != nil || !hasOnly(fields, "agentNickname", "agentRole", "cliVersion", "createdAt", "cwd", "ephemeral", "forkedFromId", "gitInfo", "id", "modelProvider", "name", "parentThreadId", "path", "preview", "projectId", "recencyAt", "section", "sectionEnteredAt", "sessionId", "source", "status", "threadSource", "turns", "updatedAt") || !hasRequired(fields, "cliVersion", "createdAt", "cwd", "ephemeral", "id", "modelProvider", "preview", "projectId", "sessionId", "source", "status", "turns", "updatedAt") {
		return ThreadSummary{}, errProtocol
	}
	version, err := stringValue(fields["cliVersion"])
	if err != nil || version != pinnedCodexVersion {
		return ThreadSummary{}, errProtocol
	}
	cwd, err := stringValue(fields["cwd"])
	if err != nil || cwd != fixedCWD {
		return ThreadSummary{}, errProtocol
	}
	createdAt, err := integer(fields["createdAt"])
	if err != nil {
		return ThreadSummary{}, errProtocol
	}
	if _, err := integer(fields["updatedAt"]); err != nil {
		return ThreadSummary{}, errProtocol
	}
	if !isFalse(fields["ephemeral"]) {
		return ThreadSummary{}, errProtocol
	}
	threadID, err := boundedIdentifier(fields["id"])
	if err != nil {
		return ThreadSummary{}, errProtocol
	}
	sessionID, err := boundedIdentifier(fields["sessionId"])
	if err != nil {
		return ThreadSummary{}, errProtocol
	}
	if _, err := nonEmptyString(fields["modelProvider"]); err != nil {
		return ThreadSummary{}, errProtocol
	}
	if !isEmptyString(fields["preview"]) || !isAbsentOrNull(fields, "name") || !isAbsentOrNull(fields, "agentNickname") || !isAbsentOrNull(fields, "agentRole") || !isAbsentOrNull(fields, "gitInfo") || !isAbsentOrNull(fields, "path") || !isNullOrString(fields["projectId"]) || !isAppServerSource(fields["source"]) || !isEmptyArray(fields["turns"]) {
		return ThreadSummary{}, errProtocol
	}
	status, ok := parseStatus(fields["status"])
	if !ok {
		return ThreadSummary{}, errProtocol
	}
	forkedFromID, err := nullableIdentifier(fields["forkedFromId"])
	if err != nil {
		return ThreadSummary{}, errProtocol
	}
	parentThreadID, err := nullableIdentifier(fields["parentThreadId"])
	if err != nil {
		return ThreadSummary{}, errProtocol
	}
	return ThreadSummary{ThreadRef: ThreadRef{ThreadID: threadID, SessionID: sessionID}, CWD: cwd, CreatedAt: createdAt, Status: status, ForkedFromID: forkedFromID, ParentThreadID: parentThreadID}, nil
}

func validateThreadList(result json.RawMessage) ([]ThreadSummary, error) {
	fields, err := object(result)
	if err != nil || !hasOnly(fields, "backwardsCursor", "data", "nextCursor") || !hasRequired(fields, "data") || !isAbsentOrNullOrString(fields, "backwardsCursor") || !isAbsentOrNullOrString(fields, "nextCursor") {
		return nil, errProtocol
	}
	data, err := array(fields["data"])
	if err != nil || len(data) > threadListLimit {
		return nil, errProtocol
	}
	summaries := make([]ThreadSummary, 0, len(data))
	for _, raw := range data {
		summary, err := validateThread(raw)
		if err != nil {
			return nil, errProtocol
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func validateThreadRead(result json.RawMessage) (ThreadSummary, error) {
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "thread") {
		return ThreadSummary{}, errProtocol
	}
	return validateThread(fields["thread"])
}

func validateDangerFullAccess(raw json.RawMessage) error {
	fields, err := object(raw)
	if err != nil || !hasExactly(fields, "type") {
		return errProtocol
	}
	typeName, err := stringValue(fields["type"])
	if err != nil || typeName != "dangerFullAccess" {
		return errProtocol
	}
	return nil
}

func validateUnsubscribe(result json.RawMessage) error {
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "status") {
		return errProtocol
	}
	status, err := stringValue(fields["status"])
	if err != nil || (status != "notLoaded" && status != "notSubscribed" && status != "unsubscribed") {
		return errProtocol
	}
	return nil
}

func readFrame(reader *bufio.Reader) ([]byte, error) {
	frame, err := reader.ReadSlice('\n')
	if err != nil || len(frame) == 0 || len(frame) > maximumFrameBytes {
		return nil, errTransport
	}
	return frame[:len(frame)-1], nil
}

func writeAll(writer io.Writer, content []byte) error {
	for len(content) > 0 {
		count, err := writer.Write(content)
		if err != nil || count <= 0 {
			return errTransport
		}
		content = content[count:]
	}
	return nil
}

func object(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, errProtocol
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' { // justify-type-assertion: json.Decoder.Token returns delimiters through its interface result.
		return nil, errProtocol
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errProtocol
		}
		name, ok := token.(string) // justify-type-assertion: JSON object keys are strings by the encoding/json decoder contract.
		if !ok {
			return nil, errProtocol
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, errProtocol
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errProtocol
		}
		fields[name] = value
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, errProtocol
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' { // justify-type-assertion: json.Decoder.Token returns delimiters through its interface result.
		return nil, errProtocol
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errProtocol
	}
	return fields, nil
}

func hasExactly(fields map[string]json.RawMessage, names ...string) bool {
	return hasOnly(fields, names...) && len(fields) == len(names)
}

func hasOnly(fields map[string]json.RawMessage, names ...string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range fields {
		if !allowed[name] {
			return false
		}
	}
	return true
}

func hasRequired(fields map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		if _, present := fields[name]; !present {
			return false
		}
	}
	return true
}

func stringValue(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '"' {
		return "", errProtocol
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", errProtocol
	}
	return value, nil
}

func nonEmptyString(raw json.RawMessage) (string, error) {
	value, err := stringValue(raw)
	if err != nil || value == "" {
		return "", errProtocol
	}
	return value, nil
}

func boundedIdentifier(raw json.RawMessage) (string, error) {
	value, err := nonEmptyString(raw)
	if err != nil || len(value) > 256 {
		return "", errProtocol
	}
	return value, nil
}

func integer(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == 'n' || raw[0] == '"' {
		return 0, errProtocol
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, errProtocol
	}
	return value, nil
}

func isFalse(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return bytes.Equal(raw, []byte("false"))
}

func isEmptyString(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte(`""`))
}

func isAbsentOrNull(fields map[string]json.RawMessage, name string) bool {
	raw, present := fields[name]
	return !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func isAbsentOrNullOrString(fields map[string]json.RawMessage, name string) bool {
	raw, present := fields[name]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true
	}
	_, err := stringValue(raw)
	return err == nil
}

func isNullOrString(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return true
	}
	_, err := stringValue(raw)
	return err == nil
}

func isAppServerSource(raw json.RawMessage) bool {
	value, err := stringValue(raw)
	return err == nil && value == "appServer"
}

func parseStatus(raw json.RawMessage) (ThreadStatus, bool) {
	fields, err := object(raw)
	if err != nil || !hasRequired(fields, "type") {
		return ThreadStatus{}, false
	}
	status, err := stringValue(fields["type"])
	if err != nil {
		return ThreadStatus{}, false
	}
	switch status {
	case "notLoaded", "idle", "systemError":
		return ThreadStatus{Type: status}, hasExactly(fields, "type")
	case "active":
		if !hasExactly(fields, "type", "activeFlags") {
			return ThreadStatus{}, false
		}
		var flags []string
		if err := json.Unmarshal(fields["activeFlags"], &flags); err != nil {
			return ThreadStatus{}, false
		}
		for _, flag := range flags {
			if flag != "waitingOnApproval" && flag != "waitingOnUserInput" {
				return ThreadStatus{}, false
			}
		}
		return ThreadStatus{Type: status, ActiveFlags: flags}, true
	default:
		return ThreadStatus{}, false
	}
}

func isEmptyArray(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return false
	}
	var values []json.RawMessage
	return json.Unmarshal(raw, &values) == nil && len(values) == 0
}

func array(raw json.RawMessage) ([]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, errProtocol
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, errProtocol
	}
	return values, nil
}

func nullableIdentifier(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := boundedIdentifier(raw)
	if err != nil {
		return nil, errProtocol
	}
	return &value, nil
}
