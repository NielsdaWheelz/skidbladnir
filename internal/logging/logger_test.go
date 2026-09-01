package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func TestLoggerEmitsOnlyClosedTmuxIdentityFields(t *testing.T) {
	var output bytes.Buffer
	launchProfile, err := agentruntime.ParseProfileKey("personal")
	if err != nil {
		t.Fatalf("parse launch profile: %v", err)
	}
	event, err := NewSessionCreated("$42", "skidbladnir-personal-1", launchProfile, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if err := New(&output).Write(event); err != nil {
		t.Fatalf("write event: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(output.Bytes(), &fields); err != nil {
		t.Fatalf("decode structured log: %v", err)
	}
	want := map[string]any{
		"event.name":                         "Session.Created",
		"skidbladnir.session.tmux_id":        "$42",
		"skidbladnir.session.tmux_name":      "skidbladnir-personal-1",
		"skidbladnir.session.launch_profile": "personal",
		"skidbladnir.duration.ms":            float64(25),
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("session-created structured fields = %#v, want %#v", fields, want)
	}
}

func TestRequestLogUsesRouteTemplatesAndClosedErrors(t *testing.T) {
	event, err := NewRequestCompleted(MethodDelete, RouteSession, 409, time.Millisecond, ErrorSessionIdentityMismatch)
	if err != nil {
		t.Fatalf("request event: %v", err)
	}
	var output bytes.Buffer
	if err := New(&output).Write(event); err != nil {
		t.Fatalf("write request event: %v", err)
	}
	line := output.String()
	if strings.Contains(line, "$42") || !strings.Contains(line, `"http.route":"/v1/sessions/{tmuxId}"`) {
		t.Fatalf("request log did not use a route template: %s", line)
	}
	if !strings.Contains(line, `"skidbladnir.error.code":"SessionIdentityMismatch"`) {
		t.Fatalf("request log omitted its closed error: %s", line)
	}

	if _, err := NewRequestCompleted(MethodGet, RouteSessions, 500, time.Millisecond, ErrorCode("Arbitrary")); err == nil {
		t.Fatal("request event accepted an open error code")
	}
	if _, err := NewRequestCompleted(MethodGet, Route("/v1/sessions/$42"), 200, time.Millisecond, ErrorNone); err == nil {
		t.Fatal("request event accepted a raw route")
	}
	if _, err := NewRequestCompleted(MethodGet, RouteSessions, 200, time.Millisecond, ErrorInternal); err == nil {
		t.Fatal("successful request accepted an error code")
	}
	if _, err := NewRequestCompleted(MethodOther, RouteUnmatched, 400, time.Millisecond, ErrorInvalidRequest); err != nil {
		t.Fatalf("closed unsupported-method event: %v", err)
	}
	if _, err := NewRequestCompleted(MethodPost, RouteDirectoryListings, 422, time.Millisecond, ErrorDirectoryListingTooLarge); err != nil {
		t.Fatalf("closed directory-listing event: %v", err)
	}

	patchEvent, err := NewRequestCompleted(MethodPatch, RouteSession, 204, time.Millisecond, ErrorNone)
	if err != nil {
		t.Fatal("PATCH request event was not in the closed method set")
	}
	output.Reset()
	if err := New(&output).Write(patchEvent); err != nil {
		t.Fatalf("write PATCH request event: %v", err)
	}
	patchLine := output.String()
	if !strings.Contains(patchLine, `"http.request.method":"PATCH"`) ||
		!strings.Contains(patchLine, `"http.route":"/v1/sessions/{tmuxId}"`) ||
		strings.Contains(patchLine, "tmux_name") {
		t.Fatal("PATCH request log did not remain a content-free route-template event")
	}
}

func TestLoggerRejectsInvalidEventsAndPropagatesWriterFailure(t *testing.T) {
	if err := New(io.Discard).Write(Event{}); err == nil {
		t.Fatal("zero event was emitted")
	}
	if _, err := NewSessionCreated("$42", "worker", agentruntime.ProfileKey("Work"), time.Millisecond); err == nil {
		t.Fatal("session-created event accepted a noncanonical launch profile")
	}
	event, err := NewSessionsListed(2, time.Millisecond)
	if err != nil {
		t.Fatalf("list event: %v", err)
	}
	if err := New(failingWriter{}).Write(event); !errors.Is(err, errWrite) {
		t.Fatalf("write error = %v, want %v", err, errWrite)
	} else {
		var writeError *WriteError
		if !errors.As(err, &writeError) {
			t.Fatalf("writer failure is not typed: %T %v", err, err)
		}
	}
}

func TestLoggerSafelyEncodesAnOpaqueLaptopSessionName(t *testing.T) {
	event, err := NewSessionKilled("$7", "laptop\nname", time.Millisecond)
	if err != nil {
		t.Fatalf("opaque tmux session name was rejected after a committed kill: %v", err)
	}
	var output bytes.Buffer
	if err := New(&output).Write(event); err != nil {
		t.Fatalf("write opaque session name: %v", err)
	}
	if strings.Contains(output.String(), "laptop\nname") || !strings.Contains(output.String(), `"skidbladnir.session.tmux_name":"laptop\nname"`) {
		t.Fatalf("session name was not JSON escaped: %q", output.String())
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var _ io.Writer = failingWriter{}
