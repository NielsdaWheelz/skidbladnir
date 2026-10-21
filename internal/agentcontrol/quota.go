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
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// Keep the buffer named so io.Copy cannot bypass Write through bytes.Buffer.ReadFrom.
type outputBuffer struct{ data bytes.Buffer }

func (buffer *outputBuffer) Write(contents []byte) (int, error) {
	if len(contents) > 65536-buffer.data.Len() {
		return 0, errors.New("quota output limit")
	}
	return buffer.data.Write(contents)
}

// quota reads only the selected account's normalized report. An unavailable
// helper, malformed report or changed configuration leaves that report absent.
func (service *Service) quota(ctx context.Context, profile agentruntime.Profile) *UsageReport {
	revision := service.sessions.SettingsRevision()
	current, found := service.sessions.Profile(profile.Key)
	if !found || current.Provider != profile.Provider || current.Home != profile.Home {
		return nil
	}
	encoded, err := json.Marshal(struct {
		ProfileKey agentruntime.ProfileKey `json:"profileKey"`
		Endpoint   string                  `json:"endpoint"`
	}{profile.Key, "unix://" + agentruntime.CodexEndpoint(profile)})
	if err != nil || len(encoded) > 65536 {
		return nil
	}
	command := exec.CommandContext(ctx, service.quotaReadPath)
	command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
	command.WaitDelay = 250 * time.Millisecond
	command.Stdin = bytes.NewReader(encoded)
	var output outputBuffer
	command.Stdout = &output
	// Provider stderr may contain account data and never enters skid output.
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		switch name {
		case "CODEX_HOME", "CLAUDE_CONFIG_DIR", "SKIDBLADNIR_CLAUDE_COMMAND", "SKIDBLADNIR_SHELL", "SKIDBLADNIR_AGENT":
		default:
			environment = append(environment, value)
		}
	}
	command.Env = append(environment, "CODEX_HOME="+profile.Home)
	runErr := command.Run()
	if ctx.Err() != nil || service.sessions.SettingsRevision() != revision {
		return nil
	}
	// The quota protocol uses omission for absent facts; reject null everywhere.
	decoder := json.NewDecoder(bytes.NewReader(output.data.Bytes()))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || token == nil {
			return nil
		}
	}
	var envelope struct {
		OK     *bool           `json:"ok"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  *struct {
			Code string `json:"code"`
		} `json:"error,omitempty"`
	}
	if strictjson.Decode(output.data.Bytes(), &envelope) != nil || envelope.OK == nil {
		return nil
	}
	if !*envelope.OK || runErr != nil || envelope.Error != nil || len(envelope.Result) == 0 {
		return nil
	}
	var report UsageReport
	if strictjson.Decode(envelope.Result, &report) != nil {
		return nil
	}
	return &report
}
