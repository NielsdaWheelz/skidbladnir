package logging

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLoggerOnlyEmitsClosedTypedFields(t *testing.T) {
	var out bytes.Buffer
	handle, err := ParseHandle("ga-3x5h7k9m2q")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	correlation, err := ParseCorrelation("cor_7f6d8g9h0j1k2l3m")
	if err != nil {
		t.Fatalf("correlation: %v", err)
	}
	event, err := NewEvent(handle, StateWorking, 25*time.Millisecond, 2, correlation, ErrorProfileUnavailable)
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := New(&out).Write(event); err != nil {
		t.Fatalf("write: %v", err)
	}
	line := out.String()
	for _, forbidden := range []string{"prompt", "message", "payload", "token", "secret", "terminal", "account"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log line contains forbidden field %q: %s", forbidden, line)
		}
	}
	if !strings.Contains(line, `"skidbladnir.agent.handle":"ga-3x5h7k9m2q"`) || !strings.Contains(line, `"skidbladnir.agent.state":"Working"`) {
		t.Fatalf("log line omitted typed fields: %s", line)
	}
}

func TestLoggerPropagatesWriterFailureAndRejectsZeroEvent(t *testing.T) {
	if err := New(failingWriter{}).Write(Event{}); err == nil {
		t.Fatal("zero event was emitted")
	}
	handle, err := ParseHandle("ga-3x5h7k9m2q")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	correlation, err := ParseCorrelation("cor_7f6d8g9h0j1k2l3m")
	if err != nil {
		t.Fatalf("correlation: %v", err)
	}
	event, err := NewEvent(handle, StateWorking, time.Millisecond, 1, correlation, ErrorProfileUnavailable)
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := New(failingWriter{}).Write(event); !errors.Is(err, errWrite) {
		t.Fatalf("write error = %v, want %v", err, errWrite)
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var _ io.Writer = failingWriter{}
