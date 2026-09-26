package agentcontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	SessionID string `json:"sessionId,omitempty"`
	PID       int    `json:"pid,omitempty"`
}

type nativeRequest struct {
	Operation  string                  `json:"operation"`
	Provider   agentruntime.Provider   `json:"provider"`
	ProfileKey agentruntime.ProfileKey `json:"profileKey"`
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

func (service *Service) native(ctx context.Context, profile agentruntime.Profile, operation string, targets []nativeTarget, input any, result any) bool {
	encoded, err := json.Marshal(nativeRequest{Operation: operation, Provider: profile.Provider, ProfileKey: profile.Key, Targets: targets, Input: input})
	if err != nil {
		return false
	}
	command := exec.CommandContext(ctx, service.nativePath)
	command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
	command.WaitDelay = 250 * time.Millisecond
	command.Stdin = bytes.NewReader(encoded)
	var output outputBuffer
	command.Stdout = &output
	// Provider stderr can contain prompts/account paths; it is never surfaced.
	command.Env = service.nativeEnvironment(profile)
	if command.Run() != nil {
		return false
	}
	var envelope nativeEnvelope
	return strictjson.Decode(output.data.Bytes(), &envelope) == nil && envelope.OK &&
		envelope.Error == nil && len(envelope.Result) > 0 && strictjson.Decode(envelope.Result, result) == nil
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
