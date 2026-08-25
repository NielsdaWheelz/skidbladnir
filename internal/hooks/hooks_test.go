package hooks

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null,
		"unexpected":"no"
	}`))
	if err == nil {
		t.Fatal("decode accepted an unknown hook field")
	}
}

func TestDecodeProjectsSubagentOnlyAsActivity(t *testing.T) {
	fact, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"SubagentStop",
		"turn_id":"turn_123",
		"agent_id":"child_123",
		"agent_type":"worker",
		"agent_transcript_path":null,
		"stop_hook_active":false,
		"last_assistant_message":null,
		"permission_mode":"bypassPermissions"
	}`))
	if err != nil {
		t.Fatalf("decode subagent hook: %v", err)
	}
	if fact.Projection != activityObserved {
		t.Fatalf("subagent projection = %v, want activity only", fact.Projection)
	}
}

func TestReportFindsNearestExactPinnedCodexAncestor(t *testing.T) {
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	report, err := identityFor(os.Getpid(), pinned)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	if report.BindingID != "binding_123" || report.PID != os.Getpid() || report.StartTime == 0 {
		t.Fatalf("unexpected identity: %#v", report)
	}
}

func TestIdentityWalkFindsPinnedParentInsteadOfUnpinnedChild(t *testing.T) {
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	child := exec.Command("/bin/sh", "-c", "exec sleep 10")
	if err := child.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	time.Sleep(10 * time.Millisecond)
	report, err := identityFor(child.Process.Pid, pinned)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	if report.PID != os.Getpid() {
		t.Fatalf("nearest pinned process = %d, want parent %d", report.PID, os.Getpid())
	}
}

func TestRunWritesAtomicGapWhenAckFailsAndNeverWritesStdout(t *testing.T) {
	dir := t.TempDir()
	gapPath := filepath.Join(dir, "gap.json")
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	err = run(bytes.NewBufferString(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null
	}`), os.Getpid(), pinned, filepath.Join(dir, "missing.sock"), gapPath)
	if err != nil {
		t.Fatalf("delivery failure must not block Codex: %v", err)
	}
	gapBytes, err := os.ReadFile(gapPath)
	if err != nil {
		t.Fatalf("read gap marker: %v", err)
	}
	var gap gapMarker
	if err := json.Unmarshal(gapBytes, &gap); err != nil {
		t.Fatalf("decode gap marker: %v", err)
	}
	if gap.Hook != "Stop" || gap.BindingID != "binding_123" || gap.SessionID != "thr_123" || gap.TurnID != "turn_123" {
		t.Fatalf("unexpected gap marker: %#v", gap)
	}
}

func TestRunDeliversOnlySafeProjectionAfterAck(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "hook.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	received := make(chan deliveryMessage, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		var message deliveryMessage
		if json.NewDecoder(connection).Decode(&message) == nil {
			received <- message
			_ = json.NewEncoder(connection).Encode(struct {
				Type string `json:"type"`
			}{Type: "Ack"})
		}
	}()
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	err = run(bytes.NewBufferString(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"SubagentStart",
		"turn_id":"turn_123",
		"agent_id":"child_123",
		"agent_type":"worker",
		"permission_mode":"bypassPermissions"
	}`), os.Getpid(), pinned, socketPath, filepath.Join(dir, "gap.json"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	message := <-received
	if message.Projection != "ActivityObserved" || message.ToolName != "" || message.Hook != "SubagentStart" {
		t.Fatalf("subagent delivery = %#v, want activity-only projection", message)
	}
	if _, err := os.Stat(filepath.Join(dir, "gap.json")); !os.IsNotExist(err) {
		t.Fatalf("ACKed delivery wrote a gap marker: %v", err)
	}
}
