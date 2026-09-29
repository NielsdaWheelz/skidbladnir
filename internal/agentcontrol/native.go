package agentcontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/runtimeenv"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type nativeTarget struct {
	SessionID     string             `json:"sessionId,omitempty"`
	PID           int                `json:"pid,omitempty"`
	StartIdentity string             `json:"startIdentity,omitempty"`
	View          *agentruntime.View `json:"view,omitempty"`
	TurnID        string             `json:"turnId,omitempty"`
}

type nativeRequest struct {
	Operation  string                  `json:"operation"`
	Provider   agentruntime.Provider   `json:"provider"`
	ProfileKey agentruntime.ProfileKey `json:"profileKey"`
	Endpoint   string                  `json:"endpoint,omitempty"`
	Targets    []nativeTarget          `json:"targets"`
	Input      any                     `json:"input,omitempty"`
}

type nativeFailure struct {
	Code     string `json:"code"`
	Dispatch string `json:"dispatch"`
}

type nativeEnvelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *nativeFailure  `json:"error,omitempty"`
}

type nativeInspection struct {
	Status            agentruntime.Status  `json:"status"`
	Methods           agentruntime.Methods `json:"methods"`
	SessionID         string               `json:"sessionId,omitempty"`
	View              *agentruntime.View   `json:"view,omitempty"`
	Turn              *agentruntime.Turn   `json:"turn,omitempty"`
	TerminalOwnsAgent bool                 `json:"terminalOwnsAgent,omitempty"`
}

// Keep the buffer named so io.Copy cannot bypass Write through bytes.Buffer.ReadFrom.
type outputBuffer struct{ data bytes.Buffer }

func (buffer *outputBuffer) Write(contents []byte) (int, error) {
	if len(contents) > 65536-buffer.data.Len() {
		return 0, errors.New("native output limit")
	}
	return buffer.data.Write(contents)
}

func (service *Service) native(ctx context.Context, profile agentruntime.Profile, operation string, targets []nativeTarget, input any, result any) error {
	mutation := operation == "send" || operation == "stop" || operation == "interrupt"
	dispatch := "not_sent"
	if mutation {
		dispatch = "unknown"
	}
	unavailable := &UnavailableError{Dispatch: dispatch}
	endpoint := ""
	if profile.Endpoint != "" {
		endpoint = "unix://" + profile.Endpoint
	}
	encoded, err := json.Marshal(nativeRequest{Operation: operation, Provider: profile.Provider, ProfileKey: profile.Key, Endpoint: endpoint, Targets: targets, Input: input})
	if err != nil || len(encoded) > 65536 {
		return &UnavailableError{Dispatch: "not_sent"}
	}
	command := exec.CommandContext(ctx, service.nativePath)
	command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
	command.WaitDelay = 250 * time.Millisecond
	command.Stdin = bytes.NewReader(encoded)
	var output outputBuffer
	command.Stdout = &output
	// Provider stderr can contain prompts/account paths; it is never surfaced.
	command.Env = service.nativeEnvironment(profile)
	runErr := command.Run()
	if runErr != nil {
		var exited *exec.ExitError
		if ctx.Err() != nil || !errors.As(runErr, &exited) {
			return unavailable
		}
	}
	// The normalized helper protocol uses omission for every absent fact. Go's
	// JSON decoder otherwise accepts null as zero for strings and booleans.
	decoder := json.NewDecoder(bytes.NewReader(output.data.Bytes()))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || token == nil {
			return unavailable
		}
	}
	var envelope nativeEnvelope
	if strictjson.Decode(output.data.Bytes(), &envelope) != nil {
		return unavailable
	}
	return decodeNativeEnvelope(envelope, result, unavailable)
}

func decodeNativeEnvelope(envelope nativeEnvelope, result any, unavailable *UnavailableError) error {
	if envelope.OK && envelope.Error == nil && len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		if strictjson.Decode(envelope.Result, result) == nil {
			return nil
		}
		return unavailable
	}
	if envelope.OK || envelope.Error == nil || len(envelope.Result) > 0 || envelope.Error.Dispatch != "not_sent" && envelope.Error.Dispatch != "unknown" {
		return unavailable
	}
	switch envelope.Error.Code {
	case "stale":
		return &nativeStaleError{Dispatch: envelope.Error.Dispatch}
	case "history_changed":
		return ErrHistoryChanged
	case "unavailable", "invalid", "unsupported", "rejected", "unknown":
		return &UnavailableError{Dispatch: envelope.Error.Dispatch}
	default:
		return unavailable
	}
}

func (service *Service) nativeEnvironment(profile agentruntime.Profile) []string {
	values := make(map[string]string)
	for _, value := range runtimeenv.WithoutHerdr(os.Environ()) {
		name, contents, ok := strings.Cut(value, "=")
		if ok {
			values[name] = contents
		}
	}
	delete(values, "CODEX_HOME")
	delete(values, "CLAUDE_CONFIG_DIR")
	for _, entry := range profile.Environment {
		values[entry.Name] = entry.Value
	}
	delete(values, "SKIDBLADNIR_SHELL")
	delete(values, "SKIDBLADNIR_AGENT")
	values["SKIDBLADNIR_CLAUDE_COMMAND"] = profile.Command
	result := make([]string, 0, len(values))
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}
