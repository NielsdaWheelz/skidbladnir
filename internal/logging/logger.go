package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

var (
	handlePattern      = regexp.MustCompile(`^ga-[a-z2-7]{6,64}$`)
	correlationPattern = regexp.MustCompile(`^corr-[a-z0-9]{4,64}$`)
)

type Handle struct{ value string }

func ParseHandle(value string) (Handle, error) {
	if !handlePattern.MatchString(value) {
		return Handle{}, fmt.Errorf("invalid handle")
	}
	return Handle{value: value}, nil
}

type Correlation struct{ value string }

func ParseCorrelation(value string) (Correlation, error) {
	if !correlationPattern.MatchString(value) {
		return Correlation{}, fmt.Errorf("invalid correlation")
	}
	return Correlation{value: value}, nil
}

type State struct{ value string }

var (
	StateIdle    = State{value: "Idle"}
	StateWorking = State{value: "Working"}
	StateUnknown = State{value: "Unknown"}
)

type ErrorCode struct{ value string }

var (
	ErrorSocketDelivery  = ErrorCode{value: "SocketDelivery"}
	ErrorHookShape       = ErrorCode{value: "HookShape"}
	ErrorProcessIdentity = ErrorCode{value: "ProcessIdentity"}
)

type Event struct {
	Handle      Handle
	State       State
	Timing      time.Duration
	Count       uint64
	Correlation Correlation
	ErrorCode   ErrorCode
}

type Logger struct{ output io.Writer }

func New(output io.Writer) Logger {
	return Logger{output: output}
}

func (logger Logger) Write(event Event) {
	_ = json.NewEncoder(logger.output).Encode(struct {
		Handle      string `json:"handle"`
		State       string `json:"state"`
		TimingMS    int64  `json:"timing_ms"`
		Count       uint64 `json:"count"`
		Correlation string `json:"correlation"`
		ErrorCode   string `json:"error_code"`
	}{
		Handle:      event.Handle.value,
		State:       event.State.value,
		TimingMS:    event.Timing.Milliseconds(),
		Count:       event.Count,
		Correlation: event.Correlation.value,
		ErrorCode:   event.ErrorCode.value,
	})
}
