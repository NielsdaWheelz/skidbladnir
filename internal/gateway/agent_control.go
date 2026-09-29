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
	Turn          *agentruntime.Turn         `json:"turn"`
	Conversation  *agentruntime.Conversation `json:"conversation"`
	Mode          stringField                `json:"mode"`
	Method        stringField                `json:"method"`
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
	if operation == "close" && input.Method.value == "native" {
		return input.Conversation != nil && input.Conversation.Valid() &&
			(input.Turn == nil || input.Turn.Valid() && input.Turn.State == "inProgress" && input.Conversation.Provider == agentruntime.ProviderCodex) && input.PID == nil && !input.PaneID.present && !input.StartIdentity.present && !input.Mode.present && !input.Text.present && input.Keys == nil && input.MaxBytes == nil
	}
	if input.PaneID.value == "" || input.PID == nil || *input.PID <= 0 || input.StartIdentity.value == "" || input.Conversation != nil || input.Turn != nil {
		return false
	}
	switch operation {
	case "read":
		return input.Mode.value == "terminal" && !input.Method.present && !input.Text.present && input.Keys == nil && (input.MaxBytes == nil || *input.MaxBytes > 0 && *input.MaxBytes <= 32768)
	case "text":
		return input.Text.present && !input.Mode.present && !input.Method.present && input.Keys == nil && input.MaxBytes == nil
	case "keys":
		return input.Keys != nil && !input.Text.present && !input.Mode.present && !input.Method.present && input.MaxBytes == nil
	case "stop", "close":
		return input.Method.value == "terminal" && input.Keys == nil && !input.Text.present && !input.Mode.present && input.MaxBytes == nil
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
	if operation == "close" && input.Method.value == "native" {
		if err := gateway.sessions.ResolveSession(ctx, id, input.IdentityToken.value); err != nil {
			writeAgentError(writer, err)
			return
		}
		result, err = gateway.agents.Close(ctx, *input.Conversation, input.Turn, func(ctx context.Context) error {
			gateway.terminalLifecycle.Lock()
			defer gateway.terminalLifecycle.Unlock()
			return gateway.closeSessionTerminal(ctx, id, input.IdentityToken.value)
		})
	} else {
		target := sessions.AgentTarget{TmuxID: id, IdentityToken: input.IdentityToken.value, PaneID: input.PaneID.value, PID: processinfo.PID(*input.PID), StartIdentity: processinfo.StartIdentity(input.StartIdentity.value)}
		switch operation {
		case "read":
			maxBytes := 0
			if input.MaxBytes != nil {
				maxBytes = *input.MaxBytes
			}
			var read agentcontrol.ReadResult
			read, err = gateway.agents.TerminalRead(ctx, target, maxBytes)
			if err == nil {
				writeAgentRead(writer, read)
				return
			}
		case "text":
			result, err = gateway.agents.Text(ctx, target, input.Text.value)
		case "keys":
			result, err = gateway.agents.Keys(ctx, target, *input.Keys)
		case "stop":
			result, err = gateway.agents.TerminalStop(ctx, target)
		case "close":
			result, err = gateway.agents.TerminalClose(ctx, target, gateway.closeAgentTerminal)
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
	writeError(writer, agentFailure(err))
}

func agentFailure(err error) apiError {
	var unavailable *agentcontrol.UnavailableError
	switch {
	case errors.As(err, &unavailable):
		failure := errorAgentUnavailable
		failure.Dispatch = unavailable.Dispatch
		return failure
	case errors.Is(err, sessions.ErrAgentTargetStale):
		failure := errorAgentTargetStale
		var dispatched interface{ DispatchState() string }
		if errors.As(err, &dispatched) {
			failure.Dispatch = dispatched.DispatchState()
		}
		return failure
	case errors.Is(err, agentcontrol.ErrHistoryChanged):
		return errorHistoryChanged
	case errors.Is(err, agentcontrol.ErrInvalidInput):
		return errorAgentInputInvalid
	default:
		return sessionFailure(err)
	}
}
