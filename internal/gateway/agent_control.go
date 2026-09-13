package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var (
	errorAgentTargetStale  = apiError{Code: "AgentTargetStale", Message: "The agent changed. Refresh and try again.", Status: http.StatusConflict, logCode: logging.ErrorAgentTargetStale, Dispatch: "not_sent"}
	errorAgentUnavailable  = apiError{Code: "AgentUnavailable", Message: "That agent method is unavailable.", Status: http.StatusServiceUnavailable, logCode: logging.ErrorAgentUnavailable, Dispatch: "not_sent"}
	errorAgentBlocked      = apiError{Code: "AgentBlocked", Message: "Inspect the terminal and send a deliberate reply.", Status: http.StatusConflict, logCode: logging.ErrorAgentBlocked, Dispatch: "not_sent"}
	errorAgentInputInvalid = apiError{Code: "AgentInputInvalid", Message: "The agent input is not valid.", Status: http.StatusBadRequest, logCode: logging.ErrorAgentInputInvalid, Dispatch: "not_sent"}
)

type agentRequest struct {
	IdentityToken stringField `json:"identityToken"`
	PaneID        stringField `json:"paneId"`
	PID           *int        `json:"pid"`
	StartIdentity stringField `json:"startIdentity"`
	Mode          stringField `json:"mode"`
	Text          stringField `json:"text"`
	MaxBytes      *int        `json:"maxBytes"`
	Keys          *[]string   `json:"keys"`
}

func (input *agentRequest) UnmarshalJSON(encoded []byte) error {
	var fields map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &fields); err != nil {
		return err
	}
	for _, value := range fields {
		if bytes.Equal(value, []byte("null")) {
			return errors.New("agent request fields cannot be null")
		}
	}
	type wire agentRequest
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*input = agentRequest(decoded)
	return nil
}

func (input agentRequest) valid(operation string) bool {
	if input.IdentityToken.value == "" || input.PaneID.value == "" || input.PID == nil || *input.PID <= 0 || input.StartIdentity.value == "" {
		return false
	}
	if input.Mode.present && input.Mode.value != "auto" && input.Mode.value != "terminal" {
		return false
	}
	switch operation {
	case "read":
		return !input.Text.present && input.Keys == nil && (input.MaxBytes == nil || *input.MaxBytes > 0 && *input.MaxBytes <= 32768)
	case "send":
		return input.Text.present && input.Keys == nil && input.MaxBytes == nil
	case "keys":
		return input.Keys != nil && !input.Text.present && !input.Mode.present && input.MaxBytes == nil
	case "interrupt", "stop":
		return input.Keys == nil && !input.Text.present && !input.Mode.present && input.MaxBytes == nil
	default:
		return false
	}
}

func agentOperationPath(path string) (string, string, bool) {
	rest, found := strings.CutPrefix(path, "/v1/sessions/")
	if !found {
		return "", "", false
	}
	id, operation, found := strings.Cut(rest, "/agent/")
	return id, operation, found && id != "" && !strings.Contains(id, "/") && !strings.Contains(operation, "/")
}

func (gateway *Gateway) agentOperation(writer http.ResponseWriter, request *http.Request) {
	id, operation, valid := agentOperationPath(request.URL.Path)
	if !valid {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[agentRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.valid(operation) {
		writeError(writer, errorAgentInputInvalid)
		return
	}
	if gateway.agents == nil {
		writeError(writer, errorAgentUnavailable)
		return
	}
	target := sessions.AgentTarget{TmuxID: id, IdentityToken: input.IdentityToken.value, PaneID: input.PaneID.value, PID: processinfo.PID(*input.PID), StartIdentity: processinfo.StartIdentity(input.StartIdentity.value)}
	ctx := request.Context()
	var result any
	var err error
	switch operation {
	case "read":
		maxBytes := 0
		if input.MaxBytes != nil {
			maxBytes = *input.MaxBytes
		}
		var read agentcontrol.ReadResult
		read, err = gateway.agents.Read(ctx, target, input.Mode.value, maxBytes)
		if err == nil {
			writeAgentRead(writer, read)
			return
		}
	case "send":
		result, err = gateway.agents.Send(ctx, target, input.Text.value, input.Mode.value)
	case "keys":
		result, err = gateway.agents.Keys(ctx, target, *input.Keys)
	case "interrupt":
		result, err = gateway.agents.Interrupt(ctx, target)
	case "stop":
		result, err = gateway.agents.Stop(ctx, target, gateway.closeAgentTerminal)
	}
	if err != nil {
		writeAgentError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (gateway *Gateway) closeAgentTerminal(ctx context.Context, target sessions.AgentTarget) error {
	gateway.terminalLifecycle.Lock()
	defer gateway.terminalLifecycle.Unlock()
	if _, err := gateway.sessions.AgentTerminalKillInput(ctx, target); err != nil {
		return err
	}
	if err := gateway.closeLiveTerminals(ctx, target.TmuxID); err != nil {
		return err
	}
	return gateway.sessions.KillAgentTerminal(ctx, target)
}

func writeAgentRead(writer http.ResponseWriter, result agentcontrol.ReadResult) {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			writeError(writer, errorInternal)
			return
		}
		if len(encoded)+1 <= int(MaximumBodyBytes) {
			writeJSON(writer, http.StatusOK, result)
			return
		}
		result.Text = result.Text[len(result.Text)/2:]
		for len(result.Text) > 0 && !utf8.RuneStart(result.Text[0]) {
			result.Text = result.Text[1:]
		}
		result.Truncated = true
	}
}

func writeAgentError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessions.ErrAgentTargetStale):
		writeError(writer, errorAgentTargetStale)
	case errors.Is(err, agentcontrol.ErrBlocked):
		writeError(writer, errorAgentBlocked)
	case errors.Is(err, agentcontrol.ErrInvalidInput):
		writeError(writer, errorAgentInputInvalid)
	default:
		writeSessionError(writer, err)
	}
}
