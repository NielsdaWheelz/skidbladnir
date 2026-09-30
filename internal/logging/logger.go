package logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var tmuxIDPattern = regexp.MustCompile(`^\$[0-9]+$`)

type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodPatch  Method = "PATCH"
	MethodDelete Method = "DELETE"
	MethodOther  Method = "OTHER"
)

func (method Method) valid() bool {
	switch method {
	case MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete, MethodOther:
		return true
	default:
		return false
	}
}

type Route string

const (
	RouteTerminalControl   Route = "/v1/sessions/{tmuxId}/terminal/{operation}"
	RouteConversations     Route = "/v1/conversations/{operation}"
	RouteHealth            Route = "/healthz"
	RouteSessions          Route = "/v1/sessions"
	RouteSession           Route = "/v1/sessions/{tmuxId}"
	RouteSessionGroup      Route = "/v1/sessions/{tmuxId}/group"
	RouteSessionShell      Route = "/v1/sessions/{tmuxId}/shell"
	RouteTerminal          Route = "/v1/sessions/{tmuxId}/terminal"
	RoutePressure          Route = "/v1/pressure"
	RoutePairingInvites    Route = "/v1/pairing-invites"
	RoutePairings          Route = "/v1/pairings"
	RouteDirectoryListings Route = "/v1/directory-listings"
	RouteDirectorySearches Route = "/v1/directory-searches"
	RouteTerminalContexts  Route = "/v1/terminal-contexts/{connectionId}"
	RouteUnmatched         Route = "unmatched"
)

func (route Route) valid() bool {
	switch route {
	case RouteTerminalControl, RouteConversations, RouteHealth, RouteSessions, RouteSession, RouteSessionGroup, RouteSessionShell, RouteTerminal, RoutePressure, RoutePairingInvites, RoutePairings, RouteDirectoryListings, RouteDirectorySearches, RouteTerminalContexts, RouteUnmatched:
		return true
	default:
		return false
	}
}

type ErrorCode string

const (
	ErrorTerminalTargetChanged       ErrorCode = "TerminalTargetChanged"
	ErrorTerminalUnavailable         ErrorCode = "TerminalUnavailable"
	ErrorTerminalInputBlocked        ErrorCode = "TerminalInputBlocked"
	ErrorAgentTargetStale            ErrorCode = "AgentTargetStale"
	ErrorAgentUnavailable            ErrorCode = "AgentUnavailable"
	ErrorHistoryChanged              ErrorCode = "HistoryChanged"
	ErrorAgentInputInvalid           ErrorCode = "AgentInputInvalid"
	ErrorNone                        ErrorCode = ""
	ErrorUnauthenticated             ErrorCode = "Unauthenticated"
	ErrorInvalidRequest              ErrorCode = "InvalidRequest"
	ErrorRequestTooLarge             ErrorCode = "RequestTooLarge"
	ErrorWorkingDirectoryInvalid     ErrorCode = "WorkingDirectoryInvalid"
	ErrorWorkingDirectoryUnavailable ErrorCode = "WorkingDirectoryUnavailable"
	ErrorDirectoryListingUnavailable ErrorCode = "DirectoryListingUnavailable"
	ErrorDirectoryListingTooLarge    ErrorCode = "DirectoryListingTooLarge"
	ErrorDirectorySearchUnavailable  ErrorCode = "DirectorySearchUnavailable"
	ErrorDirectorySearchTooLarge     ErrorCode = "DirectorySearchTooLarge"
	ErrorTerminalContextUnavailable  ErrorCode = "TerminalContextUnavailable"
	ErrorProfileUnknown              ErrorCode = "ProfileUnknown"
	ErrorSessionNameInvalid          ErrorCode = "SessionNameInvalid"
	ErrorObjectiveInvalid            ErrorCode = "ObjectiveInvalid"
	ErrorGroupInvalid                ErrorCode = "GroupInvalid"
	ErrorSessionNameChanged          ErrorCode = "SessionNameChanged"
	ErrorSessionNameConflict         ErrorCode = "SessionNameConflict"
	ErrorSessionNotFound             ErrorCode = "SessionNotFound"
	ErrorSessionIdentityMismatch     ErrorCode = "SessionIdentityMismatch"
	ErrorPairingInviteRejected       ErrorCode = "PairingInviteRejected"
	ErrorMachineIdentityMismatch     ErrorCode = "MachineIdentityMismatch"
	ErrorInternal                    ErrorCode = "InternalError"
)

func (code ErrorCode) valid() bool {
	switch code {
	case ErrorTerminalTargetChanged, ErrorTerminalUnavailable, ErrorTerminalInputBlocked, ErrorAgentTargetStale, ErrorAgentUnavailable, ErrorHistoryChanged, ErrorAgentInputInvalid, ErrorUnauthenticated,
		ErrorInvalidRequest,
		ErrorRequestTooLarge,
		ErrorWorkingDirectoryInvalid,
		ErrorWorkingDirectoryUnavailable,
		ErrorDirectoryListingUnavailable,
		ErrorDirectoryListingTooLarge,
		ErrorDirectorySearchUnavailable,
		ErrorDirectorySearchTooLarge,
		ErrorTerminalContextUnavailable,
		ErrorProfileUnknown,
		ErrorSessionNameInvalid,
		ErrorObjectiveInvalid,
		ErrorGroupInvalid,
		ErrorSessionNameChanged,
		ErrorSessionNameConflict,
		ErrorSessionNotFound,
		ErrorSessionIdentityMismatch,
		ErrorPairingInviteRejected,
		ErrorMachineIdentityMismatch,
		ErrorInternal:
		return true
	default:
		return false
	}
}

type PressureLevel string

const (
	PressureNormal  PressureLevel = "Normal"
	PressureWarm    PressureLevel = "Warm"
	PressureHot     PressureLevel = "Hot"
	PressureUnknown PressureLevel = "Unknown"
)

func (level PressureLevel) valid() bool {
	switch level {
	case PressureNormal, PressureWarm, PressureHot, PressureUnknown:
		return true
	default:
		return false
	}
}

type PressureReason string

const (
	ReasonMemory    PressureReason = "Memory"
	ReasonDisk      PressureReason = "Disk"
	ReasonLoad      PressureReason = "Load"
	ReasonCPUPSI    PressureReason = "CpuPsi"
	ReasonMemoryPSI PressureReason = "MemoryPsi"
	ReasonIOPSI     PressureReason = "IoPsi"
)

func (reason PressureReason) valid() bool {
	switch reason {
	case ReasonMemory, ReasonDisk, ReasonLoad, ReasonCPUPSI, ReasonMemoryPSI, ReasonIOPSI:
		return true
	default:
		return false
	}
}

type eventKind string

const (
	eventGatewayStarted            eventKind = "Gateway.Started"
	eventRequestCompleted          eventKind = "Request.Completed"
	eventSessionsListed            eventKind = "Sessions.Listed"
	eventSessionCreated            eventKind = "Session.Created"
	eventSessionKilled             eventKind = "Session.Killed"
	eventPressureSampled           eventKind = "Pressure.Sampled"
	eventAuthenticationRejected    eventKind = "Authentication.Rejected"
	eventTerminalCleanupFailed     eventKind = "Terminal.CleanupFailed"
	eventTerminalObservationFailed eventKind = "Terminal.ObservationFailed"
)

type Event struct {
	kind              eventKind
	method            Method
	route             Route
	status            int
	duration          time.Duration
	errorCode         ErrorCode
	count             uint64
	tmuxID            string
	launchProfile     agentruntime.ProfileKey
	observationReason sessions.StatusReason
	level             PressureLevel
	reasons           []PressureReason
}

func NewGatewayStarted() Event { return Event{kind: eventGatewayStarted} }

func NewTerminalCleanupFailed() Event { return Event{kind: eventTerminalCleanupFailed} }

func NewRequestCompleted(method Method, route Route, status int, duration time.Duration, errorCode ErrorCode) (Event, error) {
	event := Event{kind: eventRequestCompleted, method: method, route: route, status: status, duration: duration, errorCode: errorCode}
	if !event.valid() {
		return Event{}, errors.New("invalid request log event")
	}
	return event, nil
}

func NewSessionsListed(count uint64, duration time.Duration) (Event, error) {
	event := Event{kind: eventSessionsListed, count: count, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid sessions-listed log event")
	}
	return event, nil
}

func NewSessionCreated(tmuxID string, launchProfile agentruntime.ProfileKey, duration time.Duration) (Event, error) {
	event := Event{kind: eventSessionCreated, tmuxID: tmuxID, launchProfile: launchProfile, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid session-created log event")
	}
	return event, nil
}

func NewSessionKilled(tmuxID string, duration time.Duration) (Event, error) {
	event := Event{kind: eventSessionKilled, tmuxID: tmuxID, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid session-killed log event")
	}
	return event, nil
}

func NewTerminalObservationFailed(tmuxID string, reason sessions.StatusReason, duration time.Duration) (Event, error) {
	event := Event{kind: eventTerminalObservationFailed, tmuxID: tmuxID, observationReason: reason, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid terminal-observation-failed log event")
	}
	return event, nil
}

func NewPressureSampled(level PressureLevel, reasons []PressureReason, duration time.Duration) (Event, error) {
	event := Event{kind: eventPressureSampled, level: level, reasons: append([]PressureReason(nil), reasons...), duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid pressure-sampled log event")
	}
	return event, nil
}

func NewAuthenticationRejected(route Route) (Event, error) {
	event := Event{kind: eventAuthenticationRejected, route: route}
	if !event.valid() {
		return Event{}, errors.New("invalid authentication-rejected log event")
	}
	return event, nil
}

func (event Event) valid() bool {
	switch event.kind {
	case eventGatewayStarted, eventTerminalCleanupFailed:
		return true
	case eventRequestCompleted:
		if !event.method.valid() || !event.route.valid() || event.status < 100 || event.status > 599 || event.duration < 0 {
			return false
		}
		return (event.status < 400 && event.errorCode == ErrorNone) || (event.status >= 400 && event.errorCode.valid())
	case eventSessionsListed:
		return event.duration >= 0
	case eventSessionCreated:
		_, profileErr := agentruntime.ParseProfileKey(string(event.launchProfile))
		return validTmuxID(event.tmuxID) && (event.launchProfile == "" || profileErr == nil) && event.duration >= 0
	case eventSessionKilled:
		return validTmuxID(event.tmuxID) && event.duration >= 0
	case eventTerminalObservationFailed:
		switch event.observationReason {
		case sessions.ReasonObservationTimeout, sessions.ReasonCaptureFailed, sessions.ReasonProcessFailed:
			return validTmuxID(event.tmuxID) && event.duration >= 0
		default:
			return false
		}
	case eventPressureSampled:
		if !event.level.valid() || event.duration < 0 {
			return false
		}
		seen := make(map[PressureReason]struct{}, len(event.reasons))
		for _, reason := range event.reasons {
			if !reason.valid() {
				return false
			}
			if _, exists := seen[reason]; exists {
				return false
			}
			seen[reason] = struct{}{}
		}
		return true
	case eventAuthenticationRejected:
		return event.route.valid()
	default:
		return false
	}
}

func validTmuxID(value string) bool { return tmuxIDPattern.MatchString(value) }

type Logger struct{ output io.Writer }

type WriteError struct{ Err error }

func (err *WriteError) Error() string { return fmt.Sprintf("write structured log: %v", err.Err) }

func (err *WriteError) Unwrap() error { return err.Err }

func New(output io.Writer) Logger { return Logger{output: output} }

func (logger Logger) Write(event Event) error {
	if logger.output == nil || !event.valid() {
		return errors.New("invalid logger write")
	}
	fields := map[string]any{"event.name": event.kind}
	switch event.kind {
	case eventGatewayStarted, eventTerminalCleanupFailed:
	case eventRequestCompleted:
		fields["http.request.method"] = event.method
		fields["http.route"] = event.route
		fields["http.response.status_code"] = event.status
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
		if event.errorCode != ErrorNone {
			fields["skidbladnir.error.code"] = event.errorCode
		}
	case eventSessionsListed:
		fields["skidbladnir.count"] = event.count
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventSessionCreated:
		fields["skidbladnir.session.tmux_id"] = event.tmuxID
		if event.launchProfile != "" {
			fields["skidbladnir.session.launch_profile"] = event.launchProfile
		}
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventSessionKilled:
		fields["skidbladnir.session.tmux_id"] = event.tmuxID
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventTerminalObservationFailed:
		fields["skidbladnir.session.tmux_id"] = event.tmuxID
		fields["skidbladnir.observation.reason"] = event.observationReason
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventPressureSampled:
		fields["skidbladnir.pressure.level"] = event.level
		reasons := event.reasons
		if reasons == nil {
			reasons = []PressureReason{}
		}
		fields["skidbladnir.pressure.reasons"] = reasons
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventAuthenticationRejected:
		fields["http.route"] = event.route
	}
	if err := json.NewEncoder(logger.output).Encode(fields); err != nil {
		return &WriteError{Err: err}
	}
	return nil
}
