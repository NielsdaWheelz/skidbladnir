package appserver

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

const testCodexHome = "/home/niels/.codex"

func TestProbeEmptyThreadUsesOnlyPinnedSafeMethods(t *testing.T) {
	connection := scriptedProbe(validThreadResult(), true)
	ref, err := ProbeEmptyThread(context.Background(), connection, testCodexHome)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if ref.ThreadID != "thread_123" || ref.SessionID != "session_123" {
		t.Fatalf("thread ref = %#v", ref)
	}
	want := strings.Join([]string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"skidbladnir","version":"0.149.1"}}}`,
		`{"method":"initialized"}`,
		`{"id":2,"method":"thread/start","params":{"approvalPolicy":"never","cwd":"/home/niels/src","sandbox":"danger-full-access"}}`,
		`{"id":3,"method":"thread/unsubscribe","params":{"threadId":"thread_123"}}`,
	}, "\n") + "\n"
	if got := connection.requests.String(); got != want {
		t.Fatalf("request transcript = %q, want %q", got, want)
	}
}

func TestProbeRejectsThreadResponseWithTurns(t *testing.T) {
	if _, err := ProbeEmptyThread(context.Background(), scriptedProbe(threadResultWithTurns(), false), testCodexHome); err == nil {
		t.Fatal("probe accepted a thread response carrying turns")
	}
}

func TestProbeRejectsUnknownPinnedResponseField(t *testing.T) {
	result := `{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/home/niels/src","model":"gpt","modelProvider":"openai","sandbox":{"type":"dangerFullAccess"},"thread":{"cliVersion":"0.149.1","createdAt":1,"cwd":"/home/niels/src","ephemeral":false,"id":"thread_123","modelProvider":"openai","preview":"","projectId":null,"sessionId":"session_123","source":"vscode","status":{"type":"notLoaded"},"turns":[],"updatedAt":1},"unexpected":"drift"}`
	if _, err := ProbeEmptyThread(context.Background(), scriptedProbe(result, false), testCodexHome); err == nil {
		t.Fatal("probe accepted an unknown thread/start response field")
	}
}

func TestProbeRejectsWrongServerIdentity(t *testing.T) {
	for name, initialize := range map[string]string{
		"profile home": strings.Replace(initializeResponse(), testCodexHome, "/home/niels/.codex-other", 1),
		"runtime pin":  strings.Replace(initializeResponse(), "skidbladnir/0.149.1", "skidbladnir/0.150.0", 1),
		"platform":     strings.Replace(initializeResponse(), `"platformOs":"linux"`, `"platformOs":"macos"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			connection := scriptedResponses(initialize)
			if _, err := ProbeEmptyThread(context.Background(), connection, testCodexHome); err == nil {
				t.Fatalf("probe accepted wrong %s", name)
			}
		})
	}
}

func TestProbeDrainsNotificationsBeforeMatchingResponse(t *testing.T) {
	connection := scriptedResponses(
		initializeResponse(),
		`{"method":"remoteControl/status/changed","params":{"environmentId":null,"installationId":"install_123","serverName":"devbox","status":"disabled"}}`,
		`{"method":"mcpServer/startupStatus/updated","params":{"error":null,"failureReason":null,"name":"example","status":"ready","threadId":null}}`,
		fmt.Sprintf(`{"emittedAtMs":1234,"method":"thread/started","params":{"thread":%s}}`, validThread()),
		fmt.Sprintf(`{"id":2,"result":%s}`, validThreadResult()),
		`{"method":"thread/status/changed","params":{"status":{"type":"notLoaded"},"threadId":"thread_123"}}`,
		`{"id":3,"result":{"status":"unsubscribed"}}`,
	)
	if _, err := ProbeEmptyThread(context.Background(), connection, testCodexHome); err != nil {
		t.Fatalf("probe with interleaved notifications: %v", err)
	}
}

func TestProbeRejectsUnknownOrMalformedNotification(t *testing.T) {
	for name, notification := range map[string]string{
		"unknown method":   `{"method":"thread/name/updated","params":{"threadId":"thread_123"}}`,
		"malformed params": `{"method":"thread/status/changed","params":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			connection := scriptedResponses(
				initializeResponse(),
				notification,
				fmt.Sprintf(`{"id":2,"result":%s}`, validThreadResult()),
				`{"id":3,"result":{"status":"unsubscribed"}}`,
			)
			if _, err := ProbeEmptyThread(context.Background(), connection, testCodexHome); err == nil {
				t.Fatalf("probe accepted %s", name)
			}
		})
	}
}

func TestProbeRequiresFirstUnsubscribeToReleaseSubscription(t *testing.T) {
	for _, status := range []string{"notLoaded", "notSubscribed"} {
		t.Run(status, func(t *testing.T) {
			connection := scriptedResponses(
				initializeResponse(),
				fmt.Sprintf(`{"id":2,"result":%s}`, validThreadResult()),
				fmt.Sprintf(`{"id":3,"result":{"status":%q}}`, status),
			)
			if _, err := ProbeEmptyThread(context.Background(), connection, testCodexHome); err == nil {
				t.Fatalf("probe accepted first unsubscribe status %q", status)
			}
		})
	}
}

func TestProbeRejectsNonRootOrContentThread(t *testing.T) {
	for name, thread := range map[string]string{
		"ephemeral": strings.Replace(validThreadResult(), `"ephemeral":false`, `"ephemeral":true`, 1),
		"forked":    strings.Replace(validThreadResult(), `"id":"thread_123"`, `"forkedFromId":"thread_456","id":"thread_123"`, 1),
		"parent":    strings.Replace(validThreadResult(), `"id":"thread_123"`, `"parentThreadId":"thread_456","id":"thread_123"`, 1),
		"title":     strings.Replace(validThreadResult(), `"id":"thread_123"`, `"name":"title","id":"thread_123"`, 1),
		"preview":   strings.Replace(validThreadResult(), `"preview":""`, `"preview":"content"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			connection := scriptedProbe(thread, false)
			if _, err := ProbeEmptyThread(context.Background(), connection, testCodexHome); err == nil {
				t.Fatal("probe accepted a non-root or content-bearing thread")
			}
			if strings.Contains(connection.requests.String(), `"method":"thread/unsubscribe"`) {
				t.Fatal("probe unsubscribed an invalid thread")
			}
		})
	}
}

func TestListThreadSummariesUsesOneBoundedAppServerPage(t *testing.T) {
	connection := scriptedResponses(
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":{"data":[%s],"nextCursor":"ignored"}}`, validThread()),
	)
	summaries, err := ListThreadSummaries(context.Background(), connection, testCodexHome, "/home/niels/src")
	if err != nil {
		t.Fatalf("list thread summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ThreadID != "thread_123" || summaries[0].SessionID != "session_123" || summaries[0].CWD != "/home/niels/src" || summaries[0].CreatedAt != 1 || summaries[0].Status.Type != "notLoaded" {
		t.Fatalf("summaries = %#v", summaries)
	}
	want := strings.Join([]string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"skidbladnir","version":"0.149.1"}}}`,
		`{"method":"initialized"}`,
		`{"id":2,"method":"thread/list","params":{"cwd":"/home/niels/src","limit":100,"sourceKinds":["cli","vscode"]}}`,
	}, "\n") + "\n"
	if got := connection.requests.String(); got != want {
		t.Fatalf("request transcript = %q, want %q", got, want)
	}
}

func TestListAcceptsStoredThreadCreatedByOlderCodexVersion(t *testing.T) {
	thread := strings.Replace(validThread(), `"cliVersion":"0.149.1"`, `"cliVersion":"0.148.0"`, 1)
	connection := scriptedResponses(
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":{"data":[%s]}}`, thread),
	)
	if _, err := ListThreadSummaries(context.Background(), connection, testCodexHome, "/home/niels/src"); err != nil {
		t.Fatalf("pinned server rejected an older stored thread: %v", err)
	}
}

func TestReadThreadSummaryDisablesTurns(t *testing.T) {
	connection := scriptedResponses(
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":{"thread":%s}}`, validThread()),
	)
	summary, err := ReadThreadSummary(context.Background(), connection, testCodexHome, ThreadRef{ThreadID: "thread_123", SessionID: "session_123"})
	if err != nil {
		t.Fatalf("read thread summary: %v", err)
	}
	if summary.ThreadID != "thread_123" || summary.SessionID != "session_123" || summary.CWD != "/home/niels/src" || summary.CreatedAt != 1 || summary.Status.Type != "notLoaded" {
		t.Fatalf("summary = %#v", summary)
	}
	want := strings.Join([]string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"skidbladnir","version":"0.149.1"}}}`,
		`{"method":"initialized"}`,
		`{"id":2,"method":"thread/read","params":{"includeTurns":false,"threadId":"thread_123"}}`,
	}, "\n") + "\n"
	if got := connection.requests.String(); got != want {
		t.Fatalf("request transcript = %q, want %q", got, want)
	}
}

func TestReadThreadSummaryRejectsMismatchedIdentity(t *testing.T) {
	for name, thread := range map[string]string{
		"thread id":  strings.Replace(validThread(), `"id":"thread_123"`, `"id":"thread_456"`, 1),
		"session id": strings.Replace(validThread(), `"sessionId":"session_123"`, `"sessionId":"session_456"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			connection := scriptedResponses(
				initializeResponse(),
				fmt.Sprintf(`{"id":2,"result":{"thread":%s}}`, thread),
			)
			if _, err := ReadThreadSummary(context.Background(), connection, testCodexHome, ThreadRef{ThreadID: "thread_123", SessionID: "session_123"}); err == nil {
				t.Fatalf("read accepted mismatched %s", name)
			}
		})
	}
}

func TestListAndReadRejectForeignOrMalformedThread(t *testing.T) {
	for name, thread := range map[string]string{
		"foreign cwd":    strings.Replace(validThread(), `"cwd":"/home/niels/src"`, `"cwd":"/tmp"`, 1),
		"foreign source": strings.Replace(validThread(), `"source":"vscode"`, `"source":"exec"`, 1),
		"unknown field":  strings.Replace(validThread(), `"id":"thread_123"`, `"unexpected":"drift","id":"thread_123"`, 1),
		"duplicate id":   strings.Replace(validThread(), `"id":"thread_123"`, `"id":"thread_456","id":"thread_123"`, 1),
	} {
		t.Run("list "+name, func(t *testing.T) {
			connection := scriptedResponses(initializeResponse(), fmt.Sprintf(`{"id":2,"result":{"data":[%s]}}`, thread))
			if _, err := ListThreadSummaries(context.Background(), connection, testCodexHome, "/home/niels/src"); err == nil {
				t.Fatal("list accepted a foreign or malformed thread")
			}
		})
		t.Run("read "+name, func(t *testing.T) {
			connection := scriptedResponses(initializeResponse(), fmt.Sprintf(`{"id":2,"result":{"thread":%s}}`, thread))
			if _, err := ReadThreadSummary(context.Background(), connection, testCodexHome, ThreadRef{ThreadID: "thread_123", SessionID: "session_123"}); err == nil {
				t.Fatal("read accepted a foreign or malformed thread")
			}
		})
	}
}

type scriptedConnection struct {
	responses *bufio.Reader
	requests  bytes.Buffer
}

func scriptedProbe(startResult string, unsubscribe bool) *scriptedConnection {
	responses := []string{
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":%s}`, startResult),
	}
	if unsubscribe {
		responses = append(responses, `{"id":3,"result":{"status":"unsubscribed"}}`)
	}
	return &scriptedConnection{responses: bufio.NewReader(strings.NewReader(strings.Join(responses, "\n") + "\n"))}
}

func scriptedResponses(responses ...string) *scriptedConnection {
	return &scriptedConnection{responses: bufio.NewReader(strings.NewReader(strings.Join(responses, "\n") + "\n"))}
}

func initializeResponse() string {
	return `{"id":1,"result":{"codexHome":"/home/niels/.codex","platformFamily":"unix","platformOs":"linux","userAgent":"skidbladnir/0.149.1 (Linux 6; x86_64) codex_cli_rs/0.149.1"}}`
}

func (connection *scriptedConnection) Send(_ context.Context, message []byte) error {
	connection.requests.Write(message)
	connection.requests.WriteByte('\n')
	return nil
}

func (connection *scriptedConnection) Receive(_ context.Context) ([]byte, error) {
	message, err := connection.responses.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(message) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return message[:len(message)-1], nil
}

func validThreadResult() string {
	return fmt.Sprintf(`{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/home/niels/src","model":"gpt","modelProvider":"openai","sandbox":{"type":"dangerFullAccess"},"thread":%s}`, validThread())
}

func threadResultWithTurns() string {
	return validThreadResultWith(strings.Replace(validThread(), `"turns":[]`, `"turns":[{}]`, 1))
}

func validThreadResultWith(thread string) string {
	return fmt.Sprintf(`{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/home/niels/src","model":"gpt","modelProvider":"openai","sandbox":{"type":"dangerFullAccess"},"thread":%s}`, thread)
}

func validThread() string {
	return `{"cliVersion":"0.149.1","createdAt":1,"cwd":"/home/niels/src","ephemeral":false,"id":"thread_123","modelProvider":"openai","preview":"","projectId":null,"sessionId":"session_123","source":"vscode","status":{"type":"notLoaded"},"turns":[],"updatedAt":1}`
}
