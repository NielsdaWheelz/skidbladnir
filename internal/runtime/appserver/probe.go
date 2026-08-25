// Package appserver contains the closed P0 Codex App Server probe exchange.
package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	pinnedCodexVersion              = "0.149.1"
	fixedCWD                        = "/home/niels/src"
	maximumFrameBytes               = 1 << 20
	threadListLimit                 = 100
	maximumNotificationsPerExchange = 256
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
func ProbeEmptyThread(ctx context.Context, connection Connection, codexHome string) (ThreadRef, error) {
	if err := initializeConnection(ctx, connection, codexHome); err != nil {
		return ThreadRef{}, fmt.Errorf("initialize App Server connection: %w", err)
	}

	started, err := exchange(ctx, connection, 2, "thread/start", threadStartParams{
		ApprovalPolicy: "never",
		CWD:            fixedCWD,
		Sandbox:        "danger-full-access",
	})
	if err != nil {
		return ThreadRef{}, fmt.Errorf("start empty App Server thread: %w", err)
	}
	ref, err := validateThreadStart(started)
	if err != nil {
		return ThreadRef{}, fmt.Errorf("validate empty App Server thread: %w", err)
	}

	unsubscribed, err := exchange(ctx, connection, 3, "thread/unsubscribe", threadUnsubscribeParams{ThreadID: ref.ThreadID})
	if err != nil {
		return ThreadRef{}, fmt.Errorf("unsubscribe empty App Server thread: %w", err)
	}
	if err := validateUnsubscribe(unsubscribed); err != nil {
		return ThreadRef{}, fmt.Errorf("validate App Server unsubscribe: %w", err)
	}
	return ref, nil
}

// ListThreadSummaries reads exactly one fixed-size App Server page. It never
// follows a cursor or exposes titles, previews, turns, or other content.
func ListThreadSummaries(ctx context.Context, connection Connection, codexHome, cwd string) ([]ThreadSummary, error) {
	if cwd != "" && !filepath.IsAbs(cwd) {
		return nil, errProtocol
	}
	if err := initializeConnection(ctx, connection, codexHome); err != nil {
		return nil, fmt.Errorf("initialize App Server connection: %w", err)
	}
	result, err := exchange(ctx, connection, 2, "thread/list", threadListParams{
		CWD:         cwd,
		Limit:       threadListLimit,
		SourceKinds: []string{"cli", "vscode"},
	})
	if err != nil {
		return nil, fmt.Errorf("list App Server threads: %w", err)
	}
	summaries, err := validateThreadList(result, cwd)
	if err != nil {
		return nil, fmt.Errorf("validate App Server thread list: %w", err)
	}
	return summaries, nil
}

// ReadThreadSummary reads one known App Server thread without turns.
func ReadThreadSummary(ctx context.Context, connection Connection, codexHome string, reference ThreadRef) (ThreadSummary, error) {
	if reference.ThreadID == "" || len(reference.ThreadID) > 256 {
		return ThreadSummary{}, errProtocol
	}
	if err := initializeConnection(ctx, connection, codexHome); err != nil {
		return ThreadSummary{}, fmt.Errorf("initialize App Server connection: %w", err)
	}
	result, err := exchange(ctx, connection, 2, "thread/read", threadReadParams{IncludeTurns: false, ThreadID: reference.ThreadID})
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("read App Server thread: %w", err)
	}
	summary, err := validateThreadRead(result, reference)
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("validate App Server thread: %w", err)
	}
	return summary, nil
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
	CWD         string   `json:"cwd,omitempty"`
	Limit       int      `json:"limit"`
	SourceKinds []string `json:"sourceKinds"`
}

type threadReadParams struct {
	IncludeTurns bool   `json:"includeTurns"`
	ThreadID     string `json:"threadId"`
}

func initializeConnection(ctx context.Context, connection Connection, codexHome string) error {
	if !filepath.IsAbs(codexHome) {
		return errProtocol
	}
	initialize, err := exchange(ctx, connection, 1, "initialize", initializeParams{
		ClientInfo: clientInfo{Name: "skidbladnir", Version: pinnedCodexVersion},
	})
	if err != nil {
		return err
	}
	if err := validateInitialize(initialize, codexHome); err != nil {
		return err
	}
	return sendNotification(ctx, connection, "initialized")
}

func exchange(ctx context.Context, connection Connection, id int64, method string, params any) (json.RawMessage, error) {
	encoded, err := json.Marshal(clientRequest{ID: id, Method: method, Params: params})
	if err != nil || len(encoded) > maximumFrameBytes {
		return nil, errProtocol
	}
	if err := connection.Send(ctx, encoded); err != nil {
		return nil, errTransport
	}

	for skipped := 0; skipped <= maximumNotificationsPerExchange; skipped++ {
		frame, err := connection.Receive(ctx)
		if err != nil || len(frame) == 0 || len(frame) > maximumFrameBytes {
			return nil, errTransport
		}
		fields, err := object(frame)
		if err != nil {
			return nil, fmt.Errorf("%w: response object", errProtocol)
		}
		if hasOnly(fields, "emittedAtMs", "method", "params") && hasRequired(fields, "method", "params") {
			if emittedAt, present := fields["emittedAtMs"]; present {
				if _, err := integer(emittedAt); err != nil {
					return nil, errProtocol
				}
			}
			if err := validateNotification(fields["method"], fields["params"]); err != nil {
				return nil, err
			}
			continue
		}
		if hasExactly(fields, "id", "error") {
			responseID, idErr := integer(fields["id"])
			errorFields, errorErr := object(fields["error"])
			if idErr != nil || responseID != id || errorErr != nil || !hasOnly(errorFields, "code", "data", "message") || !hasRequired(errorFields, "code", "message") {
				return nil, errProtocol
			}
			code, codeErr := integer(errorFields["code"])
			_, messageErr := stringValue(errorFields["message"])
			if codeErr != nil || messageErr != nil {
				return nil, errProtocol
			}
			return nil, fmt.Errorf("%w: remote error %d", errProtocol, code)
		}
		if !hasExactly(fields, "id", "result") {
			_, hasID := fields["id"]
			_, hasResult := fields["result"]
			_, hasMethod := fields["method"]
			_, hasParams := fields["params"]
			_, hasError := fields["error"]
			return nil, fmt.Errorf("%w: response shape fields=%d id=%t result=%t method=%t params=%t error=%t", errProtocol, len(fields), hasID, hasResult, hasMethod, hasParams, hasError)
		}
		responseID, err := integer(fields["id"])
		if err != nil || responseID != id {
			return nil, fmt.Errorf("%w: response id", errProtocol)
		}
		return fields["result"], nil
	}
	return nil, errTransport
}

func sendNotification(ctx context.Context, connection Connection, method string) error {
	encoded, err := json.Marshal(clientNotification{Method: method})
	if err != nil || len(encoded) > maximumFrameBytes {
		return errProtocol
	}
	if err := connection.Send(ctx, encoded); err != nil {
		return errTransport
	}
	return nil
}

func validateInitialize(result json.RawMessage, expectedCodexHome string) error {
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "codexHome", "platformFamily", "platformOs", "userAgent") {
		return errProtocol
	}
	codexHome, homeErr := stringValue(fields["codexHome"])
	platformFamily, familyErr := stringValue(fields["platformFamily"])
	platformOS, osErr := stringValue(fields["platformOs"])
	userAgent, agentErr := stringValue(fields["userAgent"])
	if homeErr != nil || codexHome != expectedCodexHome || familyErr != nil || platformFamily != "unix" || osErr != nil || platformOS != "linux" || agentErr != nil || !strings.HasPrefix(userAgent, "skidbladnir/"+pinnedCodexVersion+" ") {
		return errProtocol
	}
	return nil
}

func validateThreadStart(result json.RawMessage) (ThreadRef, error) {
	fields, err := object(result)
	if err != nil || !hasOnly(fields, "activePermissionProfile", "approvalPolicy", "approvalsReviewer", "cwd", "instructionSources", "model", "modelProvider", "multiAgentMode", "reasoningEffort", "runtimeWorkspaceRoots", "sandbox", "serviceTier", "thread") || !hasRequired(fields, "approvalPolicy", "approvalsReviewer", "cwd", "model", "modelProvider", "sandbox", "thread") {
		_, approval := fields["approvalPolicy"]
		_, reviewer := fields["approvalsReviewer"]
		_, cwd := fields["cwd"]
		_, model := fields["model"]
		_, provider := fields["modelProvider"]
		_, sandbox := fields["sandbox"]
		_, thread := fields["thread"]
		return ThreadRef{}, fmt.Errorf("%w: thread/start fields count=%d approval=%t reviewer=%t cwd=%t model=%t provider=%t sandbox=%t thread=%t", errProtocol, len(fields), approval, reviewer, cwd, model, provider, sandbox, thread)
	}
	approvalPolicy, err := stringValue(fields["approvalPolicy"])
	if err != nil || approvalPolicy != "never" {
		return ThreadRef{}, fmt.Errorf("%w: approval policy", errProtocol)
	}
	cwd, err := stringValue(fields["cwd"])
	if err != nil || cwd != fixedCWD {
		return ThreadRef{}, fmt.Errorf("%w: thread/start cwd", errProtocol)
	}
	if reviewer, err := stringValue(fields["approvalsReviewer"]); err != nil || reviewer != "user" {
		return ThreadRef{}, fmt.Errorf("%w: approvals reviewer", errProtocol)
	}
	if _, err := nonEmptyString(fields["model"]); err != nil {
		return ThreadRef{}, fmt.Errorf("%w: model", errProtocol)
	}
	if _, err := nonEmptyString(fields["modelProvider"]); err != nil {
		return ThreadRef{}, fmt.Errorf("%w: model provider", errProtocol)
	}
	if err := validateDangerFullAccess(fields["sandbox"]); err != nil {
		return ThreadRef{}, fmt.Errorf("%w: sandbox", errProtocol)
	}
	summary, err := validateThread(fields["thread"], fixedCWD, true, "vscode")
	if err != nil {
		return ThreadRef{}, err
	}
	if summary.ForkedFromID != nil || summary.ParentThreadID != nil {
		return ThreadRef{}, fmt.Errorf("%w: non-root thread", errProtocol)
	}
	return summary.ThreadRef, nil
}

func validateThread(raw json.RawMessage, expectedCWD string, requireEmpty bool, sources ...string) (ThreadSummary, error) {
	fields, err := object(raw)
	if err != nil || !hasOnly(fields, "agentNickname", "agentRole", "canAcceptDirectInput", "cliVersion", "createdAt", "cwd", "ephemeral", "extra", "forkedFromId", "gitInfo", "historyMode", "id", "modelProvider", "name", "parentThreadId", "path", "preview", "projectId", "recencyAt", "section", "sectionEnteredAt", "sessionId", "source", "status", "threadSource", "turns", "updatedAt") || !hasRequired(fields, "cliVersion", "createdAt", "cwd", "ephemeral", "id", "modelProvider", "preview", "projectId", "sessionId", "source", "status", "turns", "updatedAt") {
		return ThreadSummary{}, fmt.Errorf("%w: thread fields", errProtocol)
	}
	version, err := stringValue(fields["cliVersion"])
	if err != nil || version == "" || len(version) > 64 || requireEmpty && version != pinnedCodexVersion {
		return ThreadSummary{}, fmt.Errorf("%w: thread CLI version", errProtocol)
	}
	cwd, err := stringValue(fields["cwd"])
	if err != nil || !filepath.IsAbs(cwd) || expectedCWD != "" && cwd != expectedCWD {
		return ThreadSummary{}, fmt.Errorf("%w: thread cwd", errProtocol)
	}
	createdAt, err := integer(fields["createdAt"])
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: thread creation time", errProtocol)
	}
	if _, err := integer(fields["updatedAt"]); err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: thread update time", errProtocol)
	}
	if !isFalse(fields["ephemeral"]) {
		return ThreadSummary{}, fmt.Errorf("%w: ephemeral thread", errProtocol)
	}
	threadID, err := boundedIdentifier(fields["id"])
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: thread id", errProtocol)
	}
	sessionID, err := boundedIdentifier(fields["sessionId"])
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: session id", errProtocol)
	}
	if _, err := nonEmptyString(fields["modelProvider"]); err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: thread model provider", errProtocol)
	}
	preview, previewErr := stringValue(fields["preview"])
	nameValid := isAbsentOrNullOrString(fields, "name")
	nicknameValid := isAbsentOrNullOrString(fields, "agentNickname")
	roleValid := isAbsentOrNullOrString(fields, "agentRole")
	projectValid := isNullOrString(fields["projectId"])
	sourceValid := isSource(fields["source"], sources...)
	turnsEmpty := isEmptyArray(fields["turns"])
	contentValid := previewErr == nil && (!requireEmpty || preview == "" && isAbsentOrNull(fields, "name") && isAbsentOrNull(fields, "agentNickname") && isAbsentOrNull(fields, "agentRole"))
	if !contentValid || !nameValid || !nicknameValid || !roleValid || !projectValid || !sourceValid || !turnsEmpty {
		return ThreadSummary{}, fmt.Errorf("%w: thread metadata content=%t name=%t nickname=%t role=%t project=%t source=%t turns=%t", errProtocol, contentValid, nameValid, nicknameValid, roleValid, projectValid, sourceValid, turnsEmpty)
	}
	status, ok := parseStatus(fields["status"])
	if !ok {
		return ThreadSummary{}, fmt.Errorf("%w: thread status", errProtocol)
	}
	forkedFromID, err := nullableIdentifier(fields["forkedFromId"])
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: forked thread id", errProtocol)
	}
	parentThreadID, err := nullableIdentifier(fields["parentThreadId"])
	if err != nil {
		return ThreadSummary{}, fmt.Errorf("%w: parent thread id", errProtocol)
	}
	return ThreadSummary{ThreadRef: ThreadRef{ThreadID: threadID, SessionID: sessionID}, CWD: cwd, CreatedAt: createdAt, Status: status, ForkedFromID: forkedFromID, ParentThreadID: parentThreadID}, nil
}

func validateThreadList(result json.RawMessage, cwd string) ([]ThreadSummary, error) {
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
		summary, err := validateThread(raw, cwd, false, "cli", "vscode")
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func validateThreadRead(result json.RawMessage, reference ThreadRef) (ThreadSummary, error) {
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "thread") {
		return ThreadSummary{}, errProtocol
	}
	summary, err := validateThread(fields["thread"], fixedCWD, false, "cli", "vscode")
	if err != nil {
		return ThreadSummary{}, err
	}
	if summary.ThreadRef != reference {
		return ThreadSummary{}, fmt.Errorf("%w: thread/read identity", errProtocol)
	}
	return summary, nil
}

func validateNotification(methodRaw, paramsRaw json.RawMessage) error {
	method, err := nonEmptyString(methodRaw)
	if err != nil || len(method) > 256 {
		return errProtocol
	}
	params, err := object(paramsRaw)
	if err != nil {
		return errProtocol
	}
	switch method {
	case "thread/started":
		if !hasExactly(params, "thread") {
			return errProtocol
		}
		_, err := validateThread(params["thread"], fixedCWD, true, "vscode")
		return err
	case "thread/status/changed":
		if !hasExactly(params, "status", "threadId") {
			return errProtocol
		}
		if _, err := boundedIdentifier(params["threadId"]); err != nil {
			return errProtocol
		}
		if _, ok := parseStatus(params["status"]); !ok {
			return errProtocol
		}
		return nil
	case "remoteControl/status/changed":
		if !hasOnly(params, "environmentId", "installationId", "serverName", "status") || !hasRequired(params, "installationId", "serverName", "status") {
			return errProtocol
		}
		installationID, installationErr := nonEmptyString(params["installationId"])
		serverName, serverErr := nonEmptyString(params["serverName"])
		status, statusErr := stringValue(params["status"])
		if installationErr != nil || len(installationID) > 256 || serverErr != nil || len(serverName) > 256 || statusErr != nil || !containsString([]string{"disabled", "connecting", "connected", "errored"}, status) {
			return errProtocol
		}
		if environmentID, present := params["environmentId"]; present && !bytes.Equal(bytes.TrimSpace(environmentID), []byte("null")) {
			value, err := stringValue(environmentID)
			if err != nil || len(value) > 256 {
				return errProtocol
			}
		}
		return nil
	case "mcpServer/startupStatus/updated":
		if !hasOnly(params, "error", "failureReason", "name", "status", "threadId") || !hasRequired(params, "name", "status") {
			return errProtocol
		}
		name, nameErr := nonEmptyString(params["name"])
		status, statusErr := stringValue(params["status"])
		if nameErr != nil || len(name) > 256 || statusErr != nil || !containsString([]string{"starting", "ready", "failed", "cancelled"}, status) {
			return errProtocol
		}
		if raw, present := params["error"]; present && !nullableBoundedString(raw, 4096) {
			return errProtocol
		}
		if raw, present := params["threadId"]; present && !nullableBoundedString(raw, 256) {
			return errProtocol
		}
		if raw, present := params["failureReason"]; present && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			value, err := stringValue(raw)
			if err != nil || value != "reauthenticationRequired" {
				return errProtocol
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: notification method %q", errProtocol, method)
	}
}

func nullableBoundedString(raw json.RawMessage, maximum int) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true
	}
	value, err := stringValue(raw)
	return err == nil && len(value) <= maximum
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	if err != nil || status != "unsubscribed" {
		return errProtocol
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

func isSource(raw json.RawMessage, sources ...string) bool {
	value, err := stringValue(raw)
	if err != nil {
		return false
	}
	for _, source := range sources {
		if value == source {
			return true
		}
	}
	return false
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
