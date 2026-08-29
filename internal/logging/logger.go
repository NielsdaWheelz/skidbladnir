package logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

var tmuxIDPattern = regexp.MustCompile(`^\$[0-9]+$`)

type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodDelete Method = "DELETE"
	MethodOther  Method = "OTHER"
)

func (method Method) valid() bool {
	switch method {
	case MethodGet, MethodPost, MethodDelete, MethodOther:
		return true
	default:
		return false
	}
}

type Route string

const (
	RouteHealth         Route = "/healthz"
	RouteSessions       Route = "/v1/sessions"
	RouteSession        Route = "/v1/sessions/{tmuxId}"
	RouteTerminal       Route = "/v1/sessions/{tmuxId}/terminal"
	RoutePressure       Route = "/v1/pressure"
	RoutePairingInvites Route = "/v1/pairing-invites"
	RoutePairings       Route = "/v1/pairings"
	RouteUnmatched      Route = "unmatched"
)

func (route Route) valid() bool {
	switch route {
	case RouteHealth, RouteSessions, RouteSession, RouteTerminal, RoutePressure, RoutePairingInvites, RoutePairings, RouteUnmatched:
		return true
	default:
		return false
	}
}

type ErrorCode string

const (
	ErrorNone                        ErrorCode = ""
	ErrorUnauthenticated             ErrorCode = "Unauthenticated"
	ErrorInvalidRequest              ErrorCode = "InvalidRequest"
	ErrorRequestTooLarge             ErrorCode = "RequestTooLarge"
	ErrorWorkingDirectoryInvalid     ErrorCode = "WorkingDirectoryInvalid"
	ErrorWorkingDirectoryUnavailable ErrorCode = "WorkingDirectoryUnavailable"
	ErrorProfileUnknown              ErrorCode = "ProfileUnknown"
	ErrorSessionNameInvalid          ErrorCode = "SessionNameInvalid"
	ErrorObjectiveInvalid            ErrorCode = "ObjectiveInvalid"
	ErrorSessionNameConflict         ErrorCode = "SessionNameConflict"
	ErrorSessionNotFound             ErrorCode = "SessionNotFound"
	ErrorSessionIdentityMismatch     ErrorCode = "SessionIdentityMismatch"
	ErrorPairingInviteRejected       ErrorCode = "PairingInviteRejected"
	ErrorMachineIdentityMismatch     ErrorCode = "MachineIdentityMismatch"
	ErrorSessionGroupedConflict      ErrorCode = "SessionGroupedConflict"
	ErrorInternal                    ErrorCode = "InternalError"
)

func (code ErrorCode) valid() bool {
	switch code {
	case ErrorUnauthenticated,
		ErrorInvalidRequest,
		ErrorRequestTooLarge,
		ErrorWorkingDirectoryInvalid,
		ErrorWorkingDirectoryUnavailable,
		ErrorProfileUnknown,
		ErrorSessionNameInvalid,
		ErrorObjectiveInvalid,
		ErrorSessionNameConflict,
		ErrorSessionNotFound,
		ErrorSessionIdentityMismatch,
		ErrorPairingInviteRejected,
		ErrorMachineIdentityMismatch,
		ErrorSessionGroupedConflict,
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
	eventGatewayStarted         eventKind = "Gateway.Started"
	eventRequestCompleted       eventKind = "Request.Completed"
	eventSessionsListed         eventKind = "Sessions.Listed"
	eventSessionCreated         eventKind = "Session.Created"
	eventSessionKilled          eventKind = "Session.Killed"
	eventPressureSampled        eventKind = "Pressure.Sampled"
	eventAuthenticationRejected eventKind = "Authentication.Rejected"
)

type Event struct {
	kind          eventKind
	method        Method
	route         Route
	status        int
	duration      time.Duration
	errorCode     ErrorCode
	count         uint64
	tmuxID        string
	tmuxName      string
	launchProfile agentruntime.ProfileKey
	level         PressureLevel
	reasons       []PressureReason
}

func NewGatewayStarted() Event { return Event{kind: eventGatewayStarted} }

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

func NewSessionCreated(tmuxID, tmuxName string, launchProfile agentruntime.ProfileKey, duration time.Duration) (Event, error) {
	event := Event{kind: eventSessionCreated, tmuxID: tmuxID, tmuxName: tmuxName, launchProfile: launchProfile, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid session-created log event")
	}
	return event, nil
}

func NewSessionKilled(tmuxID, tmuxName string, duration time.Duration) (Event, error) {
	event := Event{kind: eventSessionKilled, tmuxID: tmuxID, tmuxName: tmuxName, duration: duration}
	if !event.valid() {
		return Event{}, errors.New("invalid session-killed log event")
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
	case eventGatewayStarted:
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
		return validTmuxID(event.tmuxID) && validTmuxName(event.tmuxName) && profileErr == nil && event.duration >= 0
	case eventSessionKilled:
		return validTmuxID(event.tmuxID) && validTmuxName(event.tmuxName) && event.duration >= 0
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

func validTmuxName(value string) bool {
	return value != ""
}

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
	case eventGatewayStarted:
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
		fields["skidbladnir.session.tmux_name"] = event.tmuxName
		fields["skidbladnir.session.launch_profile"] = event.launchProfile
		fields["skidbladnir.duration.ms"] = event.duration.Milliseconds()
	case eventSessionKilled:
		fields["skidbladnir.session.tmux_id"] = event.tmuxID
		fields["skidbladnir.session.tmux_name"] = event.tmuxName
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
