//go:build system

package hooks

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
		_ = child.Process.Kill() // justify-ignore-error: cleanup accepts an already-exited fixture process.
		_ = child.Wait()         // justify-ignore-error: cleanup accepts the killed fixture process exit status.
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

func TestIdentityReportsNestedExactPinnedProcess(t *testing.T) {
	if os.Getenv("SKIDBLADNIR_TEST_NESTED_PINNED") == "1" {
		select {}
	}
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	child := exec.Command(pinned, "-test.run=^TestIdentityReportsNestedExactPinnedProcess$")
	child.Env = append(os.Environ(), "SKIDBLADNIR_TEST_NESTED_PINNED=1")
	if err := child.Start(); err != nil {
		t.Fatalf("start nested pinned child: %v", err)
	}
	defer func() {
		_ = child.Process.Kill() // justify-ignore-error: cleanup accepts an already-exited fixture process.
		_ = child.Wait()         // justify-ignore-error: cleanup accepts the killed fixture process exit status.
	}()
	time.Sleep(10 * time.Millisecond)
	report, err := identityFor(child.Process.Pid, pinned)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	if report.PID != child.Process.Pid || report.StartTime == 0 {
		t.Fatalf("nested pinned identity = %#v, want child PID %d and start time", report, child.Process.Pid)
	}
}

func TestRunWritesAtomicGapWhenAckFailsAndNeverWritesStdout(t *testing.T) {
	dir := t.TempDir()
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
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null
	}`), os.Getpid(), pinned, filepath.Join(dir, "missing.sock"), dir)
	if err != nil {
		t.Fatalf("delivery failure must not block Codex: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read gap directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("gap marker count = %d, want 1", len(entries))
	}
	markerInfo, err := entries[0].Info()
	if err != nil {
		t.Fatalf("stat gap marker: %v", err)
	}
	if markerInfo.Mode().Perm() != 0o600 {
		t.Fatalf("gap marker mode = %o, want 600", markerInfo.Mode().Perm())
	}
	gapBytes, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
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

func TestRunWritesDistinctGapMarkersWithoutOverwrite(t *testing.T) {
	directory := t.TempDir()
	pinned, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("SKIDBLADNIR_BINDING_ID", "binding_123")
	input := bytes.NewBufferString(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null
	}`)
	if err := run(input, os.Getpid(), pinned, filepath.Join(directory, "missing.sock"), directory); err != nil {
		t.Fatalf("first failed delivery: %v", err)
	}
	input = bytes.NewBuffer(inputBytesForGapTest())
	if err := run(input, os.Getpid(), pinned, filepath.Join(directory, "missing.sock"), directory); err != nil {
		t.Fatalf("second failed delivery: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read gap directory: %v", err)
	}
	if len(entries) != 2 || entries[0].Name() == entries[1].Name() {
		t.Fatalf("gap markers = %#v, want two distinct markers", entries)
	}
}

func inputBytesForGapTest() []byte {
	return []byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null
	}`)
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
			_ = json.NewEncoder(connection).Encode(struct { // justify-ignore-error: the negative-path fixture intentionally allows the client to close first.
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
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions"
	}`), os.Getpid(), pinned, socketPath, dir)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	message := <-received
	if message.Projection != "ActivityObserved" || message.ToolName != "" || message.Hook != "SubagentStart" {
		t.Fatalf("subagent delivery = %#v, want activity-only projection", message)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read gap directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "gap-") {
			t.Fatalf("ACKed delivery wrote gap marker: %s", entry.Name())
		}
	}
}

func TestDeliverRejectsTrailingAckData(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "hook.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		var received deliveryMessage
		if json.NewDecoder(connection).Decode(&received) == nil {
			_, _ = connection.Write([]byte("{\"type\":\"Ack\"} {\"type\":\"Ack\"}\n")) // justify-ignore-error: the malformed-response fixture intentionally allows the client to close first.
		}
	}()

	err = deliver(socketPath, deliveryMessage{Type: "HookFact"})
	if err == nil {
		t.Fatal("delivery accepted trailing ACK JSON")
	}
}
