package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var (
	errorTerminalTargetChanged = apiError{Code: "TerminalTargetChanged", Message: "the terminal changed. refresh before trying again.", Status: http.StatusConflict, logCode: logging.ErrorTerminalTargetChanged, Dispatch: "not_sent"}
	errorTerminalUnavailable   = apiError{Code: "TerminalUnavailable", Message: "the terminal is unavailable. open it to inspect before trying again.", Status: http.StatusConflict, logCode: logging.ErrorTerminalUnavailable, Dispatch: "not_sent"}
	errorTerminalInputBlocked  = apiError{Code: "TerminalInputBlocked", Message: "send unavailable for this screen. open the terminal or use text/keys.", Status: http.StatusConflict, logCode: logging.ErrorTerminalInputBlocked, Dispatch: "not_sent"}
)

type terminalRequest struct {
	IdentityToken stringField `json:"identityToken"`
	PaneID        stringField `json:"paneId"`
	Text          stringField `json:"text"`
	Keys          *[]string   `json:"keys"`
	MaxBytes      *int        `json:"maxBytes"`
	Explain       *bool       `json:"explain"`
}

func (input *terminalRequest) UnmarshalJSON(encoded []byte) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	for _, value := range members {
		if bytes.Equal(value, []byte("null")) {
			return errors.New("terminal request fields cannot be null")
		}
	}
	type wire terminalRequest
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*input = terminalRequest(decoded)
	return nil
}

func (input terminalRequest) valid(operation string) bool {
	if input.IdentityToken.value == "" || len(input.PaneID.value) < 2 || input.PaneID.value[0] != '%' || strings.IndexFunc(input.PaneID.value[1:], func(r rune) bool { return r < '0' || r > '9' }) >= 0 ||
		input.Explain != nil && operation != "inspect" {
		return false
	}
	switch operation {
	case "inspect", "stop", "close":
		return !input.Text.present && input.Keys == nil && input.MaxBytes == nil
	case "read":
		return !input.Text.present && input.Keys == nil && (input.MaxBytes == nil || *input.MaxBytes > 0 && *input.MaxBytes <= 32768)
	case "send", "text":
		return input.Keys == nil && input.MaxBytes == nil && input.Text.value != "" && len(input.Text.value) <= 32768 && utf8.ValidString(input.Text.value) && !strings.ContainsRune(input.Text.value, 0)
	case "keys":
		return !input.Text.present && input.MaxBytes == nil && input.Keys != nil && len(*input.Keys) >= 1 && len(*input.Keys) <= 16
	default:
		return false
	}
}

func terminalOperationPath(path string) (string, string, bool) {
	rest, found := strings.CutPrefix(path, "/v1/sessions/")
	if !found {
		return "", "", false
	}
	id, operation, found := strings.Cut(rest, "/terminal/")
	return id, operation, found && len(id) >= 2 && id[0] == '$' && strings.IndexFunc(id[1:], func(r rune) bool { return r < '0' || r > '9' }) < 0 && !strings.ContainsRune(operation, '/')
}

func (gateway *Gateway) terminalOperation(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	id, operation, valid := terminalOperationPath(request.URL.Path)
	if !valid {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[terminalRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.valid(operation) {
		writeError(writer, errorInvalidRequest)
		return
	}
	if ctx.Err() != nil {
		writeError(writer, terminalFailure(ctx.Err()))
		return
	}
	target := sessions.TerminalTarget{TmuxID: id, IdentityToken: input.IdentityToken.value, PaneID: input.PaneID.value}
	var result any
	var err error
	switch operation {
	case "inspect":
		startedAt := time.Now()
		status, diagnostics, inspectErr := gateway.agents.TerminalInspect(ctx, target)
		if inspectErr != nil {
			writeError(writer, terminalFailure(inspectErr))
			return
		}
		inspected, mapErr := mapTerminalInspectResponse(status, diagnostics, input.Explain != nil && *input.Explain)
		if mapErr != nil {
			writeError(writer, errorInternal)
			return
		}
		if status.Source == sessions.SourceUnavailable {
			gateway.logObservationFailure(agentcontrol.ObservationFailure{TmuxID: id, Reason: status.Reason, Elapsed: time.Since(startedAt)})
		}
		result = inspected
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
	case "send":
		result, err = gateway.agents.TerminalSend(ctx, target, input.Text.value)
	case "text":
		result, err = gateway.agents.Text(ctx, target, input.Text.value)
	case "keys":
		result, err = gateway.agents.Keys(ctx, target, *input.Keys)
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
		closeErr := gateway.closeSessionTerminal(ctx, id, target.IdentityToken)
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
	writeJSON(writer, http.StatusOK, result)
}

type terminalCloseResult struct {
	Interrupt string `json:"interrupt"`
	Terminal  string `json:"terminal"`
}

var diagnosticRuleID = regexp.MustCompile(`^[a-z0-9_.-]{1,48}$`)

// Diagnostics are present iff the request asked for an explanation.
type terminalInspectResponse struct {
	TerminalStatus sessions.TerminalStatus   `json:"terminalStatus"`
	Diagnostics    *agentcontrol.Diagnostics `json:"diagnostics,omitempty"`
}

// mapTerminalInspectResponse sends only a valid status and a bounded,
// content-free explanation; anything else is a producer defect.
func mapTerminalInspectResponse(status sessions.TerminalStatus, diagnostics agentcontrol.Diagnostics, explain bool) (terminalInspectResponse, error) {
	if !status.Valid() {
		return terminalInspectResponse{}, errors.New("invalid terminal status")
	}
	if !explain {
		return terminalInspectResponse{TerminalStatus: status}, nil
	}
	if len(diagnostics.Rules) > 8 {
		return terminalInspectResponse{}, errors.New("too many diagnostic rules")
	}
	for _, rule := range diagnostics.Rules {
		switch rule.Region {
		case agentcontrol.DiagnosticTop, agentcontrol.DiagnosticBottom, agentcontrol.DiagnosticCompound:
		default:
			return terminalInspectResponse{}, errors.New("invalid diagnostic rule region")
		}
		if !diagnosticRuleID.MatchString(rule.ID) {
			return terminalInspectResponse{}, errors.New("invalid diagnostic rule id")
		}
	}
	if diagnostics.Rules == nil {
		diagnostics.Rules = []agentcontrol.DiagnosticRule{}
	}
	if diagnostics.Capture != nil && (diagnostics.Capture.Width < 1 || diagnostics.Capture.Height < 1) {
		return terminalInspectResponse{}, errors.New("invalid diagnostic capture dimensions")
	}
	for _, elapsed := range []*int64{diagnostics.ElapsedMs.Resolve, diagnostics.ElapsedMs.Capture, diagnostics.ElapsedMs.Classify} {
		if elapsed != nil && (*elapsed < 0 || *elapsed > math.MaxInt32) {
			return terminalInspectResponse{}, errors.New("invalid diagnostic stage duration")
		}
	}
	return terminalInspectResponse{TerminalStatus: status, Diagnostics: &diagnostics}, nil
}

// terminalLifecycle orders attachment admission and exact session deletion.
// Its one permit is cancellable; no worker escapes the requesting context.
func (gateway *Gateway) lockTerminalLifecycle(ctx context.Context) error {
	select {
	case gateway.terminalLifecycle <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-gateway.terminalLifecycle
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ctx carries the operation's absolute deadline; all waits consume that budget.
func (gateway *Gateway) closeSessionTerminal(ctx context.Context, id, identityToken string) error {
	startedAt := time.Now()
	deadline, _ := ctx.Deadline()
	admissionContext, cancelAdmission := context.WithDeadline(ctx, deadline.Add(-2*time.Second))
	defer cancelAdmission()
	err := gateway.lockTerminalLifecycle(admissionContext)
	if err != nil {
		return err
	}
	defer func() { <-gateway.terminalLifecycle }()
	if err := gateway.sessions.ResolveSession(admissionContext, id, identityToken); err != nil {
		return err
	}
	cancelAdmission()
	cleanupDeadline := time.Now().Add(6 * time.Second)
	if cap := deadline.Add(-2 * time.Second); cap.Before(cleanupDeadline) {
		cleanupDeadline = cap
	}
	cleanupContext, cancelCleanup := context.WithDeadline(ctx, cleanupDeadline)
	cleanupErr := gateway.closeLiveTerminals(cleanupContext, id, identityToken)
	cancelCleanup()
	if cleanupErr != nil {
		gateway.log(logging.NewTerminalCleanupFailed())
	}
	// Cleanup failure cannot veto exact deletion. Parent cancellation still owns
	// later phases; never pass the expired cleanup context into deletion.
	if err := ctx.Err(); err != nil {
		return err
	}
	deleteErr := gateway.sessions.Kill(ctx, sessions.KillInput{TmuxID: id, IdentityToken: identityToken})
	if deleteErr != nil {
		return deleteErr
	}
	// Closure reports exact deletion, independently of attachment cleanup.
	event, eventErr := logging.NewSessionKilled(id, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-killed log event")
	} // justify-defect: sessions deleted this canonical tmux id.
	gateway.log(event)
	return nil
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
	var dispatched interface{ DispatchState() string }
	if errors.As(err, &dispatched) {
		failure.Dispatch = dispatched.DispatchState()
	}
	return failure
}
