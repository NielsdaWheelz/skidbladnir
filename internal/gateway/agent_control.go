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
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var (
	errorAgentTargetStale  = apiError{Code: "AgentTargetStale", Message: "the session changed. refresh and try again.", Status: http.StatusConflict, logCode: logging.ErrorAgentTargetStale, Dispatch: "not_sent"}
	errorAgentUnavailable  = apiError{Code: "AgentUnavailable", Message: "this action is unavailable for this session.", Status: http.StatusConflict, logCode: logging.ErrorAgentUnavailable, Dispatch: "not_sent"}
	errorAgentInputInvalid = apiError{Code: "AgentInputInvalid", Message: "the agent input is not valid.", Status: http.StatusBadRequest, logCode: logging.ErrorAgentInputInvalid, Dispatch: "not_sent"}
	errorHistoryChanged    = apiError{Code: "HistoryChanged", Message: "native history changed. restart the result scan.", Status: http.StatusConflict, logCode: logging.ErrorHistoryChanged, Dispatch: "not_sent"}
)

type agentRequest struct {
	IdentityToken stringField                `json:"identityToken"`
	PaneID        stringField                `json:"paneId"`
	PID           *int                       `json:"pid"`
	StartIdentity stringField                `json:"startIdentity"`
	Binding       *agentruntime.Binding      `json:"binding"`
	Turn          *agentruntime.Turn         `json:"turn"`
	Conversation  *agentruntime.Conversation `json:"conversation"`
	Cursor        stringField                `json:"cursor"`
	Mode          stringField                `json:"mode"`
	Scope         stringField                `json:"scope"`
	Method        stringField                `json:"method"`
	Input         stringField                `json:"input"`
	Delivery      stringField                `json:"delivery"`
	Text          stringField                `json:"text"`
	MaxBytes      *int                       `json:"maxBytes"`
	Keys          *[]string                  `json:"keys"`
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
	if input.IdentityToken.value == "" {
		return false
	}
	if operation == "results" {
		return input.Conversation != nil && input.PID == nil && !input.PaneID.present && !input.StartIdentity.present && input.Binding == nil && input.Turn == nil && !input.Mode.present && !input.Scope.present && !input.Method.present && !input.Input.present && !input.Delivery.present && !input.Text.present && input.MaxBytes == nil && input.Keys == nil && (!input.Cursor.present || input.Cursor.value != "")
	}
	if input.PaneID.value == "" || input.PID == nil || *input.PID <= 0 || input.StartIdentity.value == "" || input.Conversation != nil || input.Cursor.present {
		return false
	}
	switch operation {
	case "read":
		return input.Mode.present && (input.Mode.value == "terminal" && !input.Scope.present && input.Binding == nil && input.Turn == nil || input.Mode.value == "native" && input.Binding != nil && (input.Scope.value == "latest" || input.Scope.value == "history")) && !input.Method.present && !input.Input.present && !input.Delivery.present && !input.Text.present && input.Keys == nil && (input.MaxBytes == nil || *input.MaxBytes > 0 && *input.MaxBytes <= 32768)
	case "send":
		return input.Binding != nil && input.Text.present && (input.Input.value == "peer" || input.Input.value == "user") && (input.Delivery.value == "direct" || input.Delivery.value == "queue" && input.Input.value == "user") && !input.Mode.present && !input.Scope.present && !input.Method.present && input.Keys == nil && input.MaxBytes == nil
	case "text":
		return input.Text.present && !input.Mode.present && !input.Scope.present && !input.Method.present && !input.Input.present && !input.Delivery.present && input.Binding == nil && input.Turn == nil && input.Keys == nil && input.MaxBytes == nil
	case "keys":
		return input.Keys != nil && !input.Text.present && !input.Mode.present && !input.Scope.present && !input.Method.present && !input.Input.present && !input.Delivery.present && input.Binding == nil && input.Turn == nil && input.MaxBytes == nil
	case "stop", "close":
		return input.Method.present && (input.Method.value == "terminal" || input.Method.value == "native" && input.Binding != nil) && input.Keys == nil && !input.Text.present && !input.Mode.present && !input.Scope.present && !input.Input.present && !input.Delivery.present && input.MaxBytes == nil
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
	ctx := request.Context()
	var result any
	var err error
	if operation == "results" {
		result, err = gateway.agents.Results(ctx, id, input.IdentityToken.value, *input.Conversation, input.Cursor.value)
	} else {
		target := sessions.AgentTarget{TmuxID: id, IdentityToken: input.IdentityToken.value, PaneID: input.PaneID.value, PID: processinfo.PID(*input.PID), StartIdentity: processinfo.StartIdentity(input.StartIdentity.value), Binding: input.Binding, Turn: input.Turn}
		switch operation {
		case "read":
			maxBytes := 0
			if input.MaxBytes != nil {
				maxBytes = *input.MaxBytes
			}
			var read agentcontrol.ReadResult
			read, err = gateway.agents.Read(ctx, target, input.Mode.value, input.Scope.value, maxBytes)
			if err == nil {
				writeAgentRead(writer, read)
				return
			}
		case "send":
			result, err = gateway.agents.Send(ctx, target, input.Text.value, input.Input.value, input.Delivery.value)
		case "text":
			result, err = gateway.agents.Text(ctx, target, input.Text.value)
		case "keys":
			result, err = gateway.agents.Keys(ctx, target, *input.Keys)
		case "stop":
			result, err = gateway.agents.Stop(ctx, target, input.Method.value)
		case "close":
			result, err = gateway.agents.Close(ctx, target, input.Method.value, gateway.closeAgentTerminal)
		}
	}
	if err != nil {
		writeAgentError(writer, err)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > int(MaximumBodyBytes) {
		writeError(writer, errorInternal)
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
		if result.Text == "" {
			writeError(writer, errorInternal)
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
	var unavailable *agentcontrol.UnavailableError
	switch {
	case errors.As(err, &unavailable):
		failure := errorAgentUnavailable
		failure.Dispatch = unavailable.Dispatch
		writeError(writer, failure)
	case errors.Is(err, sessions.ErrAgentTargetStale):
		failure := errorAgentTargetStale
		if dispatched, ok := err.(interface{ DispatchState() string }); ok {
			failure.Dispatch = dispatched.DispatchState()
		}
		writeError(writer, failure)
	case errors.Is(err, agentcontrol.ErrHistoryChanged):
		writeError(writer, errorHistoryChanged)
	case errors.Is(err, agentcontrol.ErrInvalidInput):
		writeError(writer, errorAgentInputInvalid)
	default:
		writeSessionError(writer, err)
	}
}
