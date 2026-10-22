package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var (
	errorTerminalTargetChanged = apiError{Code: "TerminalTargetChanged", Message: "the terminal changed. refresh before trying again.", Status: http.StatusConflict, Dispatch: "not_sent"}
	errorTerminalUnavailable   = apiError{Code: "TerminalUnavailable", Message: "the terminal is unavailable. open it to inspect before trying again.", Status: http.StatusConflict, Dispatch: "not_sent"}
	errorTerminalInputBlocked  = apiError{Code: "TerminalInputBlocked", Message: "send unavailable for this screen. open the terminal or use text/keys.", Status: http.StatusConflict, Dispatch: "not_sent"}
)

type terminalRequest struct {
	IdentityToken  string   `json:"identityToken"`
	PaneID         string   `json:"paneId"`
	Text           *string  `json:"text"`
	InitialProfile *string  `json:"initialProfile"`
	Keys           []string `json:"keys"`
	MaxBytes       *int     `json:"maxBytes"`
	Explain        *bool    `json:"explain"`
}

// shaped reports whether input names a target and only the members operation admits.
// agentcontrol owns the members' value bounds.
func (input terminalRequest) shaped(operation string) bool {
	if input.IdentityToken == "" || !digitsAfter(input.PaneID, '%') ||
		input.InitialProfile != nil && (operation != "send" || *input.InitialProfile == "") {
		return false
	}
	switch operation {
	case "inspect":
		return input.Text == nil && input.Keys == nil && input.MaxBytes == nil
	case "stop", "close":
		return input.Text == nil && input.Keys == nil && input.MaxBytes == nil && input.Explain == nil
	case "read":
		return input.Text == nil && input.Keys == nil && input.Explain == nil
	case "send", "text":
		return input.Keys == nil && input.MaxBytes == nil && input.Explain == nil
	case "keys":
		return input.Text == nil && input.MaxBytes == nil && input.Explain == nil
	}
	return false
}

func (gateway *Gateway) terminalOperation(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	id, operation := request.PathValue("tmuxId"), request.PathValue("operation")
	if !digitsAfter(id, '$') {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, ok := decodeJSON[terminalRequest](writer, request)
	if !ok {
		return
	}
	if !input.shaped(operation) {
		writeError(writer, errorInvalidRequest)
		return
	}
	if ctx.Err() != nil {
		writeError(writer, terminalFailure(ctx.Err()))
		return
	}
	target := sessions.TerminalTarget{TmuxID: id, IdentityToken: input.IdentityToken, PaneID: input.PaneID}
	var result any
	var err error
	switch operation {
	case "inspect":
		// TerminalInspect resolves the target itself, so a failure's duration includes resolution.
		startedAt := time.Now()
		var status sessions.TerminalStatus
		var diagnostics agentcontrol.Diagnostics
		status, diagnostics, err = gateway.agents.TerminalInspect(ctx, target)
		if err != nil {
			break
		}
		if status.Source == sessions.SourceUnavailable {
			gateway.logObservationFailure(id, status.Reason, time.Since(startedAt))
		}
		inspected := struct {
			TerminalStatus sessions.TerminalStatus   `json:"terminalStatus"`
			Diagnostics    *agentcontrol.Diagnostics `json:"diagnostics,omitempty"`
		}{TerminalStatus: status}
		if input.Explain != nil && *input.Explain {
			// A nil slice is zero rules, but encoding/json writes it as null.
			if diagnostics.Rules == nil {
				diagnostics.Rules = []string{}
			}
			inspected.Diagnostics = &diagnostics
		}
		result = inspected
	case "read":
		var read agentcontrol.ReadResult
		read, err = gateway.agents.TerminalRead(ctx, target, input.MaxBytes)
		if err != nil {
			break
		}
		// Keep the newest text that fits one response: halve from the start on a rune boundary.
		for {
			encoded, _ := json.Marshal(read) // strings and a bool always encode
			if len(encoded)+1 <= int(MaximumBodyBytes) {
				break
			}
			read.Text = read.Text[len(read.Text)/2:]
			for len(read.Text) > 0 && !utf8.RuneStart(read.Text[0]) {
				read.Text = read.Text[1:]
			}
			read.Truncated = true
		}
		result = read
	case "send":
		result, err = gateway.agents.TerminalSend(ctx, target, valueOf(input.Text), agentruntime.ProfileKey(valueOf(input.InitialProfile)))
	case "text":
		result, err = gateway.agents.Text(ctx, target, valueOf(input.Text))
	case "keys":
		result, err = gateway.agents.Keys(ctx, target, input.Keys)
	case "stop":
		result, err = gateway.agents.TerminalStop(ctx, target)
	case "close":
		// Admission is the session lifetime, not the current pane or foreground.
		deadline, _ := ctx.Deadline()
		validationContext, cancelValidation := context.WithDeadline(ctx, deadline.Add(-2*time.Second))
		validationErr := gateway.sessions.ResolveSession(validationContext, id, target.IdentityToken)
		if validationErr == nil {
			validationErr = validationContext.Err()
		}
		cancelValidation()
		if validationErr != nil {
			writeError(writer, terminalFailure(validationErr))
			return
		}
		closed := terminalCloseResult{Interrupt: "not_sent", Terminal: "not_closed"}
		stopDeadline := time.Now().Add(2 * time.Second)
		if cap := deadline.Add(-8 * time.Second); cap.Before(stopDeadline) {
			stopDeadline = cap
		}
		stopContext, cancelStop := context.WithDeadline(ctx, stopDeadline)
		stopped, stopErr := gateway.agents.TerminalStop(stopContext, target)
		cancelStop()
		if stopErr == nil {
			closed.Interrupt = stopped.Outcome
		} else if terminalFailure(stopErr).Dispatch == "unknown" {
			closed.Interrupt = "unknown"
		}
		closeErr := gateway.closeSession(ctx, id, target.IdentityToken)
		if closeErr == nil {
			closed.Terminal = "closed"
		} else if terminalFailure(closeErr).Dispatch == "unknown" {
			closed.Terminal = "unknown"
		}
		result = closed
	}
	if err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	writeJSON(writer, http.StatusOK, result, nil)
}

type terminalCloseResult struct {
	Interrupt string `json:"interrupt"`
	Terminal  string `json:"terminal"`
}

func terminalFailure(err error) apiError {
	var failure apiError
	switch {
	case errors.Is(err, sessions.ErrTerminalTargetChanged):
		failure = errorTerminalTargetChanged
	case errors.Is(err, agentcontrol.ErrTerminalInputBlocked):
		failure = errorTerminalInputBlocked
		var blocked *agentcontrol.TerminalInputBlockedError
		if errors.As(err, &blocked) {
			switch blocked.Reason {
			case "draft":
				failure.Message = "terminal contains a draft. open it before sending."
			case "dialog":
				failure.Message = "respond to the dialog in the terminal."
			case "initial_not_ready":
				failure.Message = "the requested agent is not ready for its initial prompt. inspect the terminal before sending."
			}
		}
	case errors.Is(err, sessions.ErrTerminalUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		failure = errorTerminalUnavailable
	case errors.Is(err, sessions.ErrTerminalWriteUnknown):
		failure = errorTerminalUnavailable
		failure.Message = "could not confirm terminal input. inspect the terminal before trying again."
		failure.Dispatch = "unknown"
		return failure
	case errors.Is(err, sessions.ErrSessionDeleteUnknown):
		failure = errorTerminalUnavailable
		failure.Message = "could not confirm terminal closure. refresh before trying again."
		failure.Dispatch = "unknown"
		return failure
	case errors.Is(err, agentcontrol.ErrInvalidInput):
		failure = errorInvalidRequest
	default:
		failure = sessionFailure(err)
	}
	failure.Dispatch = "not_sent"
	return failure
}
