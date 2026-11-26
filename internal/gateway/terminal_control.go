package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var (
	errorTerminalTargetChanged   = apiError{api.Error{Code: "TerminalTargetChanged", Message: "the terminal changed. refresh before trying again."}, http.StatusConflict}
	errorTerminalUnavailable     = apiError{api.Error{Code: "TerminalUnavailable", Message: "the terminal is unavailable. open it to inspect before trying again."}, http.StatusConflict}
	errorReplyAddressUnavailable = apiError{api.Error{Code: "ReplyAddressUnavailable", Message: "the original terminal process is unavailable."}, http.StatusConflict}
	errorReplyAddressAmbiguous   = apiError{api.Error{Code: "ReplyAddressAmbiguous", Message: "the reply address matches more than one terminal."}, http.StatusConflict}
	errorTerminalInputBlocked    = apiError{api.Error{Code: "TerminalInputBlocked", Message: "send unavailable for this screen. open the terminal or use text/keys."}, http.StatusConflict}
)

type terminalRequest struct {
	IdentityToken  string              `json:"identityToken"`
	CloseID        string              `json:"closeId,omitempty"`
	PaneID         string              `json:"paneId"`
	Text           *string             `json:"text"`
	InitialProfile *string             `json:"initialProfile"`
	Keys           []string            `json:"keys"`
	MaxBytes       *int                `json:"maxBytes"`
	Explain        *bool               `json:"explain"`
	Foreground     *api.PeerForeground `json:"foreground"`
}

// shaped reports whether input names a target and only the members operation
// admits, each within its bounds. It is the only check of those bounds:
// agentcontrol, sessions and tmux take the values as given. Key names and
// initialProfile are admitted downstream, where their sets live.
func (input terminalRequest) shaped(operation string) bool {
	if input.CloseID != "" && (operation != "close" || !sessions.ValidBranchID(input.CloseID)) || input.IdentityToken == "" || !tmux.ValidID(input.PaneID, '%') ||
		input.InitialProfile != nil && (operation != "send" || *input.InitialProfile == "") ||
		input.Foreground != nil && (operation != "address" || !validPeerForeground(*input.Foreground)) {
		return false
	}
	switch operation {
	case "inspect":
		return input.Text == nil && input.Keys == nil && input.MaxBytes == nil
	case "stop", "close", "address":
		return input.Text == nil && input.Keys == nil && input.MaxBytes == nil && input.Explain == nil
	case "read":
		return input.Text == nil && input.Keys == nil && input.Explain == nil &&
			(input.MaxBytes == nil || *input.MaxBytes >= 1 && *input.MaxBytes <= 32768)
	case "send", "text":
		return input.Keys == nil && input.MaxBytes == nil && input.Explain == nil && input.Text != nil &&
			validTerminalText(*input.Text)
	case "keys":
		return input.Text == nil && input.MaxBytes == nil && input.Explain == nil && len(input.Keys) >= 1 && len(input.Keys) <= 16
	}
	return false
}

func (gateway *Gateway) terminalOperation(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	id, operation := request.PathValue("tmuxId"), request.PathValue("operation")
	if !tmux.ValidID(id, '$') {
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
		var status api.TerminalStatus
		var diagnostics api.Diagnostics
		status, diagnostics, err = gateway.agents.TerminalInspect(ctx, target)
		if err != nil {
			break
		}
		if status.Source == api.SourceUnavailable {
			gateway.logObservationFailure(id, status.Reason, time.Since(startedAt))
		}
		inspected := api.Inspect{TerminalStatus: status}
		if input.Explain != nil && *input.Explain {
			// A nil slice is zero rules, but encoding/json writes it as null.
			if diagnostics.Rules == nil {
				diagnostics.Rules = []string{}
			}
			inspected.Diagnostics = &diagnostics
		}
		result = inspected
	case "read":
		limit := 16384
		if input.MaxBytes != nil {
			limit = *input.MaxBytes
		}
		var read api.ReadResult
		read, err = gateway.agents.TerminalRead(ctx, target, limit)
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
	case "address":
		result, err = gateway.agents.TerminalAddress(ctx, gateway.machine.String(), target, input.Foreground)
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
		closeID, validationErr := gateway.sessions.BeginClose(validationContext, id, target.IdentityToken, target.PaneID, "interrupt", input.CloseID)

		cancelValidation()
		if validationErr != nil {
			writeError(writer, terminalFailure(validationErr))
			return
		}
		closed := api.CloseResult{Interrupt: "not_sent", Terminal: "not_closed"}
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
		if closed.Interrupt != "written" {
			// The captured pane was refused or its input outcome is unknown.
			// No deletion was dispatched; finish only this local close attempt.
			gateway.finishClose(ctx, target.IdentityToken, closeID, false)
			result = closed
			break
		}
		closeErr := gateway.closeSession(ctx, id, target.IdentityToken, closeID)
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

func terminalFailure(err error) apiError {
	var failure apiError
	var blocked *agentcontrol.TerminalInputBlockedError
	switch {
	case errors.Is(err, sessions.ErrBranchWriteUnknown):
		failure = branchFailure(err)
	case errors.Is(err, agentcontrol.ErrReplyUnavailable):
		failure = errorReplyAddressUnavailable
	case errors.Is(err, agentcontrol.ErrReplyAmbiguous):
		failure = errorReplyAddressAmbiguous
	case errors.Is(err, sessions.ErrTerminalTargetChanged):
		failure = errorTerminalTargetChanged
	case errors.As(err, &blocked):
		failure = errorTerminalInputBlocked
		switch blocked.Reason {
		case "draft":
			failure.Message = api.InputBlockedDraft
		case "dialog":
			failure.Message = api.InputBlockedDialog
		case "initial_not_ready":
			failure.Message = api.InputBlockedNotReady
		}
	case errors.Is(err, sessions.ErrTerminalUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		failure = errorTerminalUnavailable
	case errors.Is(err, sessions.ErrTerminalWriteUnknown):
		failure = errorTerminalUnavailable
		failure.Message = "could not confirm terminal input. inspect the terminal before trying again."
		failure.Dispatch = "unknown"
	case errors.Is(err, sessions.ErrSessionDeleteUnknown):
		failure = errorTerminalUnavailable
		failure.Message = "could not confirm terminal closure. refresh before trying again."
		failure.Dispatch = "unknown"
	case errors.Is(err, agentcontrol.ErrInvalidInput):
		failure = errorInvalidRequest
	default:
		failure = sessionFailure(err)
	}
	return failure
}

func validTerminalText(text string) bool {
	return text != "" && len(text) <= 32768 && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}

func validPeerForeground(foreground api.PeerForeground) bool {
	return (foreground.Provider == string(agentruntime.ProviderCodex) || foreground.Provider == string(agentruntime.ProviderClaude)) &&
		foreground.PID > 0 && foreground.StartIdentity != "" && len(foreground.StartIdentity) <= 20 &&
		strings.Trim(foreground.StartIdentity, "0123456789") == ""
}

func (gateway *Gateway) peerReply(writer http.ResponseWriter, request *http.Request) {
	digest := request.PathValue("address")
	if len(digest) != 16 {
		writeError(writer, errorInvalidRequest)
		return
	}
	if decoded, err := base64.RawURLEncoding.Strict().DecodeString(digest); err != nil || len(decoded) != 12 {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, ok := decodeJSON[struct {
		Text string `json:"text"`
	}](writer, request)
	if !ok {
		return
	}
	if !validTerminalText(input.Text) {
		writeError(writer, errorInvalidRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	result, err := gateway.agents.TerminalReply(ctx, gateway.machine.String(), digest, input.Text)
	if err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	writeJSON(writer, http.StatusOK, result, nil)
}
