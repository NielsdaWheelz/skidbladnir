package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var (
	errorAgentTargetStale  = apiError{Code: "AgentTargetStale", Message: "the session changed. refresh and try again.", Status: http.StatusConflict, logCode: logging.ErrorAgentTargetStale, Dispatch: "not_sent"}
	errorAgentUnavailable  = apiError{Code: "AgentUnavailable", Message: "this action is unavailable for this session.", Status: http.StatusConflict, logCode: logging.ErrorAgentUnavailable, Dispatch: "not_sent"}
	errorAgentInputInvalid = apiError{Code: "AgentInputInvalid", Message: "the agent input is not valid.", Status: http.StatusBadRequest, logCode: logging.ErrorAgentInputInvalid, Dispatch: "not_sent"}
	errorHistoryChanged    = apiError{Code: "HistoryChanged", Message: "native history changed. restart the result scan.", Status: http.StatusConflict, logCode: logging.ErrorHistoryChanged, Dispatch: "not_sent"}
)

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
