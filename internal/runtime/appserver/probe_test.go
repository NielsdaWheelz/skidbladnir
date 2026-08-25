package appserver

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestProbeEmptyThreadUsesOnlyPinnedSafeMethods(t *testing.T) {
	connection := scriptedProbe(validThreadResult(), true)
	ref, err := ProbeEmptyThread(connection)
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
	if _, err := ProbeEmptyThread(scriptedProbe(threadResultWithTurns(), false)); err == nil {
		t.Fatal("probe accepted a thread response carrying turns")
	}
}

func TestProbeRejectsUnknownPinnedResponseField(t *testing.T) {
	result := `{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/home/niels/src","model":"gpt","modelProvider":"openai","sandbox":{"type":"dangerFullAccess"},"thread":{"cliVersion":"0.149.1","createdAt":1,"cwd":"/home/niels/src","ephemeral":false,"id":"thread_123","modelProvider":"openai","preview":"","projectId":null,"sessionId":"session_123","source":"appServer","status":{"type":"notLoaded"},"turns":[],"updatedAt":1},"unexpected":"drift"}`
	if _, err := ProbeEmptyThread(scriptedProbe(result, false)); err == nil {
		t.Fatal("probe accepted an unknown thread/start response field")
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
			if _, err := ProbeEmptyThread(connection); err == nil {
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
	summaries, err := ListThreadSummaries(connection)
	if err != nil {
		t.Fatalf("list thread summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ThreadID != "thread_123" || summaries[0].SessionID != "session_123" || summaries[0].CWD != "/home/niels/src" || summaries[0].CreatedAt != 1 || summaries[0].Status.Type != "notLoaded" {
		t.Fatalf("summaries = %#v", summaries)
	}
	want := strings.Join([]string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"skidbladnir","version":"0.149.1"}}}`,
		`{"method":"initialized"}`,
		`{"id":2,"method":"thread/list","params":{"cwd":"/home/niels/src","limit":100,"sourceKinds":["appServer"]}}`,
	}, "\n") + "\n"
	if got := connection.requests.String(); got != want {
		t.Fatalf("request transcript = %q, want %q", got, want)
	}
}

func TestReadThreadSummaryDisablesTurns(t *testing.T) {
	connection := scriptedResponses(
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":{"thread":%s}}`, validThread()),
	)
	summary, err := ReadThreadSummary(connection, ThreadRef{ThreadID: "thread_123", SessionID: "session_123"})
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

func TestListAndReadRejectForeignOrMalformedThread(t *testing.T) {
	for name, thread := range map[string]string{
		"foreign cwd":    strings.Replace(validThread(), `"cwd":"/home/niels/src"`, `"cwd":"/tmp"`, 1),
		"foreign source": strings.Replace(validThread(), `"source":"appServer"`, `"source":"cli"`, 1),
		"unknown field":  strings.Replace(validThread(), `"id":"thread_123"`, `"unexpected":"drift","id":"thread_123"`, 1),
		"duplicate id":   strings.Replace(validThread(), `"id":"thread_123"`, `"id":"thread_456","id":"thread_123"`, 1),
	} {
		t.Run("list "+name, func(t *testing.T) {
			connection := scriptedResponses(initializeResponse(), fmt.Sprintf(`{"id":2,"result":{"data":[%s]}}`, thread))
			if _, err := ListThreadSummaries(connection); err == nil {
				t.Fatal("list accepted a foreign or malformed thread")
			}
		})
		t.Run("read "+name, func(t *testing.T) {
			connection := scriptedResponses(initializeResponse(), fmt.Sprintf(`{"id":2,"result":{"thread":%s}}`, thread))
			if _, err := ReadThreadSummary(connection, ThreadRef{ThreadID: "thread_123", SessionID: "session_123"}); err == nil {
				t.Fatal("read accepted a foreign or malformed thread")
			}
		})
	}
}

type scriptedConnection struct {
	responses *bytes.Reader
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
	return &scriptedConnection{responses: bytes.NewReader([]byte(strings.Join(responses, "\n") + "\n"))}
}

func scriptedResponses(responses ...string) *scriptedConnection {
	return &scriptedConnection{responses: bytes.NewReader([]byte(strings.Join(responses, "\n") + "\n"))}
}

func initializeResponse() string {
	return `{"id":1,"result":{"codexHome":"/home/niels/.codex","platformFamily":"unix","platformOs":"linux","userAgent":"codex/0.149.1"}}`
}

func (connection *scriptedConnection) Read(buffer []byte) (int, error) {
	return connection.responses.Read(buffer)
}

func (connection *scriptedConnection) Write(buffer []byte) (int, error) {
	return connection.requests.Write(buffer)
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
	return `{"cliVersion":"0.149.1","createdAt":1,"cwd":"/home/niels/src","ephemeral":false,"id":"thread_123","modelProvider":"openai","preview":"","projectId":null,"sessionId":"session_123","source":"appServer","status":{"type":"notLoaded"},"turns":[],"updatedAt":1}`
}
