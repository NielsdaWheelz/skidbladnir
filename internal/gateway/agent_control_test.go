package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestAgentControlInventoryDoesNotRequireTerminalActivity(t *testing.T) {
	card, err := mapSession(sessions.Session{
		TmuxID: "$7", TmuxName: "ordinary", IdentityToken: "v1-fixture",
		Character: catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"},
	}, testProfileCatalog())
	if err != nil {
		t.Fatalf("ordinary terminal without retired activity must remain visible: %v", err)
	}
	if _, present := encodedObject(t, card)["activity"]; present {
		t.Fatal("retired terminal activity remains on the agent-control wire")
	}
}

func TestAgentControlRejectsNullOrForeignOperationInputs(t *testing.T) {
	const target = `"identityToken":"v1-fixture","paneId":"%1","pid":12,"startIdentity":"42"`
	for _, test := range []struct{ operation, extra string }{
		{"read", `,"maxBytes":null`}, {"stop", `,"keys":null`},
		{"read", `,"text":"foreign"`}, {"send", `,"text":"fixture","maxBytes":3`},
		{"read", `,"maxBytes":0`}, {"stop", `,"mode":"terminal"`},
	} {
		request := httptest.NewRequest(http.MethodPost, "/v1/sessions/$1/agent/"+test.operation, strings.NewReader("{"+target+test.extra+"}"))
		request.Header.Set("Content-Type", "application/json")
		input, failure := decodeJSON[agentRequest](httptest.NewRecorder(), request)
		if failure == nil && input.valid(test.operation) {
			t.Fatal("null or another operation's fields were accepted")
		}
	}
}

func TestAgentControlReadBoundsEscapedJSONWithoutLosingCoverage(t *testing.T) {
	response := httptest.NewRecorder()
	writeAgentRead(response, agentcontrol.ReadResult{Text: strings.Repeat("\x01", 32768), Source: "terminal", Scope: "terminal_history"})
	var result agentcontrol.ReadResult
	if response.Code != http.StatusOK || response.Body.Len() > int(MaximumBodyBytes) || json.Unmarshal(response.Body.Bytes(), &result) != nil || !result.Truncated || result.Scope != "terminal_history" || result.Text == "" {
		t.Fatal("escaped terminal output exceeded its bound or lost coverage")
	}
}
