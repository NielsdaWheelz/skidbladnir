package logging

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLoggerOnlyEmitsClosedTypedFields(t *testing.T) {
	var out bytes.Buffer
	handle, err := ParseHandle("ga-3x5h7k")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	correlation, err := ParseCorrelation("corr-7f6d")
	if err != nil {
		t.Fatalf("correlation: %v", err)
	}
	New(&out).Write(Event{
		Handle:      handle,
		State:       StateWorking,
		Timing:      25 * time.Millisecond,
		Count:       2,
		Correlation: correlation,
		ErrorCode:   ErrorSocketDelivery,
	})
	line := out.String()
	for _, forbidden := range []string{"prompt", "message", "payload", "token", "secret", "terminal", "account"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log line contains forbidden field %q: %s", forbidden, line)
		}
	}
	if !strings.Contains(line, `"handle":"ga-3x5h7k"`) || !strings.Contains(line, `"state":"Working"`) {
		t.Fatalf("log line omitted typed fields: %s", line)
	}
}
