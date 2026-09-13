package agentcli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func TestFailureIsOneBoundedJSONEnvelope(t *testing.T) {
	for _, input := range []string{`{}`, strings.Repeat("x", fleetclient.MaximumInputBytes+1)} {
		var stdout bytes.Buffer
		exit := Run(context.Background(), "/missing/client.json", "send", strings.NewReader(input), &stdout)
		var result fleetclient.Result
		if json.Unmarshal(stdout.Bytes(), &result) != nil || exit != 1 || result.OK || result.Error.Dispatch != "not_sent" || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
			t.Fatal("cli failure was not a single not-sent json envelope")
		}
		if strings.Contains(stdout.String(), "/missing") {
			t.Fatal("configuration detail escaped into tool output")
		}
	}
}
