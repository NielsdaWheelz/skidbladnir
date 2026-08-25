package logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	skidbladnirv1 "github.com/NielsdaWheelz/skidbladnir/generated/api/go"
)

var (
	handlePattern      = regexp.MustCompile(`^ga-[a-z0-9]{10}$`)
	correlationPattern = regexp.MustCompile(`^cor_[a-zA-Z0-9_-]{16,96}$`)
)

type Handle struct{ value skidbladnirv1.AgentHandle }

func ParseHandle(value string) (Handle, error) {
	if !handlePattern.MatchString(value) {
		return Handle{}, errors.New("invalid agent handle")
	}
	return Handle{value: skidbladnirv1.AgentHandle(value)}, nil
}

func (value Handle) valid() bool { return handlePattern.MatchString(string(value.value)) }

type Correlation struct {
	value skidbladnirv1.CorrelationHandle
}

func ParseCorrelation(value string) (Correlation, error) {
	if !correlationPattern.MatchString(value) {
		return Correlation{}, errors.New("invalid correlation handle")
	}
	return Correlation{value: skidbladnirv1.CorrelationHandle(value)}, nil
}

func (value Correlation) valid() bool { return correlationPattern.MatchString(string(value.value)) }

type State struct{ value skidbladnirv1.AgentState }

var (
	StateStarting    = State{value: skidbladnirv1.AgentStateStarting}
	StateIdle        = State{value: skidbladnirv1.AgentStateIdle}
	StateWorking     = State{value: skidbladnirv1.AgentStateWorking}
	StateClosing     = State{value: skidbladnirv1.AgentStateClosing}
	StateRecoverable = State{value: skidbladnirv1.AgentStateRecoverable}
	StateException   = State{value: skidbladnirv1.AgentStateException}
	StateUnknown     = State{value: skidbladnirv1.AgentStateUnknown}
	StateClosed      = State{value: skidbladnirv1.AgentStateClosed}
)

func (value State) valid() bool {
	switch value.value {
	case skidbladnirv1.AgentStateStarting,
		skidbladnirv1.AgentStateIdle,
		skidbladnirv1.AgentStateWorking,
		skidbladnirv1.AgentStateClosing,
		skidbladnirv1.AgentStateRecoverable,
		skidbladnirv1.AgentStateException,
		skidbladnirv1.AgentStateUnknown,
		skidbladnirv1.AgentStateClosed:
		return true
	default:
		return false
	}
}

type ErrorCode struct{ value skidbladnirv1.ErrorCode }

var (
	ErrorUnauthenticated             = ErrorCode{value: skidbladnirv1.ErrorCodeUnauthenticated}
	ErrorPairingInvalid              = ErrorCode{value: skidbladnirv1.ErrorCodePairingInvalid}
	ErrorProtocolMismatch            = ErrorCode{value: skidbladnirv1.ErrorCodeProtocolMismatch}
	ErrorInvalidRequest              = ErrorCode{value: skidbladnirv1.ErrorCodeInvalidRequest}
	ErrorAgentNotFound               = ErrorCode{value: skidbladnirv1.ErrorCodeAgentNotFound}
	ErrorAgentClosed                 = ErrorCode{value: skidbladnirv1.ErrorCodeAgentClosed}
	ErrorAgentWorking                = ErrorCode{value: skidbladnirv1.ErrorCodeAgentWorking}
	ErrorAgentNotAttachable          = ErrorCode{value: skidbladnirv1.ErrorCodeAgentNotAttachable}
	ErrorAgentUntracked              = ErrorCode{value: skidbladnirv1.ErrorCodeAgentUntracked}
	ErrorWorkingDirectoryInvalid     = ErrorCode{value: skidbladnirv1.ErrorCodeWorkingDirectoryInvalid}
	ErrorWorkingDirectoryUnavailable = ErrorCode{value: skidbladnirv1.ErrorCodeWorkingDirectoryUnavailable}
	ErrorProfileUnavailable          = ErrorCode{value: skidbladnirv1.ErrorCodeProfileUnavailable}
	ErrorRuntimeLaunchFailed         = ErrorCode{value: skidbladnirv1.ErrorCodeRuntimeLaunchFailed}
	ErrorCommandConflict             = ErrorCode{value: skidbladnirv1.ErrorCodeCommandConflict}
	ErrorCursorInvalid               = ErrorCode{value: skidbladnirv1.ErrorCodeCursorInvalid}
	ErrorLivenessUnverifiable        = ErrorCode{value: skidbladnirv1.ErrorCodeLivenessUnverifiable}
)

func (value ErrorCode) valid() bool {
	switch value.value {
	case skidbladnirv1.ErrorCodeUnauthenticated,
		skidbladnirv1.ErrorCodePairingInvalid,
		skidbladnirv1.ErrorCodeProtocolMismatch,
		skidbladnirv1.ErrorCodeInvalidRequest,
		skidbladnirv1.ErrorCodeAgentNotFound,
		skidbladnirv1.ErrorCodeAgentClosed,
		skidbladnirv1.ErrorCodeAgentWorking,
		skidbladnirv1.ErrorCodeAgentNotAttachable,
		skidbladnirv1.ErrorCodeAgentUntracked,
		skidbladnirv1.ErrorCodeWorkingDirectoryInvalid,
		skidbladnirv1.ErrorCodeWorkingDirectoryUnavailable,
		skidbladnirv1.ErrorCodeProfileUnavailable,
		skidbladnirv1.ErrorCodeRuntimeLaunchFailed,
		skidbladnirv1.ErrorCodeCommandConflict,
		skidbladnirv1.ErrorCodeCursorInvalid,
		skidbladnirv1.ErrorCodeLivenessUnverifiable:
		return true
	default:
		return false
	}
}

type Event struct {
	handle      Handle
	state       State
	timing      time.Duration
	count       uint64
	correlation Correlation
	errorCode   ErrorCode
}

func NewEvent(handle Handle, state State, timing time.Duration, count uint64, correlation Correlation, errorCode ErrorCode) (Event, error) {
	if !handle.valid() || !state.valid() || timing < 0 || !correlation.valid() || !errorCode.valid() {
		return Event{}, errors.New("invalid log event")
	}
	return Event{handle: handle, state: state, timing: timing, count: count, correlation: correlation, errorCode: errorCode}, nil
}

func (event Event) valid() bool {
	return event.handle.valid() && event.state.valid() && event.timing >= 0 && event.correlation.valid() && event.errorCode.valid()
}

type Logger struct{ output io.Writer }

func New(output io.Writer) Logger { return Logger{output: output} }

func (logger Logger) Write(event Event) error {
	if logger.output == nil || !event.valid() {
		return errors.New("invalid logger write")
	}
	if err := json.NewEncoder(logger.output).Encode(struct {
		Handle      string `json:"skidbladnir.agent.handle"`
		State       string `json:"skidbladnir.agent.state"`
		TimingMS    int64  `json:"skidbladnir.timing.ms"`
		Count       uint64 `json:"skidbladnir.count"`
		Correlation string `json:"skidbladnir.correlation.handle"`
		ErrorCode   string `json:"skidbladnir.error.code"`
	}{
		Handle:      string(event.handle.value),
		State:       string(event.state.value),
		TimingMS:    event.timing.Milliseconds(),
		Count:       event.count,
		Correlation: string(event.correlation.value),
		ErrorCode:   string(event.errorCode.value),
	}); err != nil {
		return fmt.Errorf("write structured log: %w", err)
	}
	return nil
}
