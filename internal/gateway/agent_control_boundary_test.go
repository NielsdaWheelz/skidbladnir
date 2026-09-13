package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
	"github.com/creack/pty"
)

// These fixtures replace only the external tmux executable and native-provider
// command. Gateway, dispatch, sessions, kernel identity and PTY are production.
// They never invoke tmux or a provider; live terminal semantics remain NOT_RUN.
type controlFixtureState struct {
	PID           int
	PaneID        string
	Registration  string
	Exists        bool
	NativeMode    string
	NativeStops   int
	NativeSends   int
	Pastes        int
	LastKey       string
	LiteralInput  bool
	BufferName    string
	DeletedBuffer string
}

type controlHarness struct {
	directoryGatewayHarness
	target    sessions.AgentTarget
	statePath string
}

func newControlHarness(t *testing.T, nativeMode string) controlHarness {
	t.Helper()
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestAgentControlFixtureProcess$")
	child.Env = append(os.Environ(), "SKID_CONTROL_FIXTURE=agent", "GORACE=atexit_sleep_ms=0")
	terminal, err := pty.Start(child)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait(); _ = terminal.Close() })
	ready := make([]byte, 1)
	if _, err := terminal.Read(ready); err != nil {
		t.Fatal("fixture process did not start")
	}
	observed, err := processinfo.ObserveForeground(processinfo.PID(child.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	provider := agentruntime.ProviderCodex
	homeVariable := "CODEX_HOME"
	endpoint := "unix:///fixture/socket"
	if strings.HasPrefix(nativeMode, "claude-") {
		provider = agentruntime.ProviderClaude
		homeVariable = "CLAUDE_CONFIG_DIR"
		endpoint = ""
	}
	registration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{Provider: provider, PID: observed.PID, StartIdentity: observed.StartIdentity}, "unit", "fixture-thread")
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state.json")
	state := controlFixtureState{PID: child.Process.Pid, PaneID: "%1", Registration: registration, Exists: true, NativeMode: nativeMode}
	writeControlState(t, statePath, state)
	paths := make(map[string]string)
	for _, mode := range []string{"tmux", "native"} {
		path := filepath.Join(root, mode+"-fixture")
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
		script := "#!/bin/sh\nGORACE=atexit_sleep_ms=0 SKID_CONTROL_FIXTURE=" + quote(mode) + " SKID_CONTROL_STATE=" + quote(statePath) + " exec " + quote(executable) + " -test.run=^TestAgentControlFixtureProcess$ -- \"$@\"\n"
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		paths[mode] = path
	}
	cataloguePath := filepath.Join(root, "characters.json")
	if err := os.WriteFile(cataloguePath, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	directories, err := workdir.New(root)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessions.New(sessions.Config{TmuxPath: paths["tmux"], CataloguePath: cataloguePath, Workdir: directories, Profiles: []agentruntime.Profile{{Key: "unit", Label: "Unit", Provider: provider, Command: executable, NativeEndpoint: endpoint, Environment: []agentruntime.EnvironmentVariable{{Name: homeVariable, Value: root}}, ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: filepath.Base(executable)}}}}})
	if err != nil {
		t.Fatal(err)
	}
	control, err := agentcontrol.New(manager, paths["native"])
	if err != nil {
		t.Fatal(err)
	}
	harness := newDirectoryGatewayHarness(t, root)
	harness.gateway.sessions = manager
	harness.gateway.agents = control
	inventory, err := manager.List(context.Background())
	if err != nil || len(inventory.Sessions) != 1 || inventory.Sessions[0].Agent == nil {
		t.Fatalf("fixture inventory did not identify its own process: %v", err)
	}
	return controlHarness{directoryGatewayHarness: harness, target: sessions.TargetOf(inventory.Sessions[0]), statePath: statePath}
}

func writeControlState(t *testing.T, path string, state controlFixtureState) {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (harness controlHarness) state(t *testing.T) controlFixtureState {
	t.Helper()
	encoded, err := os.ReadFile(harness.statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state controlFixtureState
	if err := json.Unmarshal(encoded, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func (harness controlHarness) call(t *testing.T, operation string, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	fields["identityToken"] = harness.target.IdentityToken
	fields["paneId"] = harness.target.PaneID
	fields["pid"] = harness.target.PID
	fields["startIdentity"] = harness.target.StartIdentity
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	harness.gateway.ServeHTTP(response, harness.request(http.MethodPost, "/v1/sessions/$1/agent/"+operation, string(encoded)))
	return response
}

func TestAgentControlNativeAdmissionAndUnknownNeverPaste(t *testing.T) {
	for _, mode := range []string{"blocked", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			harness := newControlHarness(t, mode)
			response := harness.call(t, "send", map[string]any{"text": "fixture"})
			if response.Code != http.StatusOK {
				t.Fatalf("native admission status=%d, want 200", response.Code)
			}
			var result agentcontrol.WriteResult
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			want := "accepted"
			if mode == "unknown" {
				want = "unknown"
			}
			if result.Method != "native" || result.Outcome != want {
				t.Fatal("native outcome was not preserved")
			}
			state := harness.state(t)
			if state.NativeSends != 1 || state.Pastes != 0 {
				t.Fatal("native submission was suppressed or replayed through terminal")
			}
		})
	}
}

func TestAgentControlExplicitDialogTextAndStaleTarget(t *testing.T) {
	harness := newControlHarness(t, "unavailable")
	response := harness.call(t, "send", map[string]any{"mode": "terminal", "text": "first\nsecond; $(literal)"})
	if response.Code != http.StatusOK {
		t.Fatalf("deliberate terminal send status=%d", response.Code)
	}
	state := harness.state(t)
	if !state.LiteralInput || state.Pastes != 1 {
		t.Fatal("multiline terminal input changed or was not submitted once")
	}
	state.PaneID = "%2"
	writeControlState(t, harness.statePath, state)
	response = harness.call(t, "send", map[string]any{"mode": "terminal", "text": "fixture"})
	if response.Code != http.StatusConflict || harness.state(t).Pastes != 1 {
		t.Fatal("changed active pane was controlled instead of rejected")
	}
	if strings.Contains(harness.logs.String(), "literal") {
		t.Fatal("agent payload reached logs")
	}
}

func TestAgentControlCodexTerminalInterruptUsesEscape(t *testing.T) {
	harness := newControlHarness(t, "unavailable")
	response := harness.call(t, "interrupt", map[string]any{})
	if response.Code != http.StatusOK || harness.state(t).LastKey != "Escape" {
		t.Fatal("codex interruption did not use its native terminal interrupt key")
	}
}

func TestAgentControlUnavailableStatusAndHistoryRetainTerminalFallback(t *testing.T) {
	t.Run("inspection timeout", func(t *testing.T) {
		harness := newControlHarness(t, "inspect-stall")
		response := httptest.NewRecorder()
		harness.gateway.ServeHTTP(response, harness.request(http.MethodGet, "/v1/sessions", ""))
		if response.Code != http.StatusOK {
			t.Fatalf("list status=%d", response.Code)
		}
		var inventory sessionsResponseDTO
		if err := json.Unmarshal(response.Body.Bytes(), &inventory); err != nil {
			t.Fatal(err)
		}
		if len(inventory.Sessions) != 1 || inventory.Sessions[0].Agent.Status.Source != "terminal" || inventory.Sessions[0].Agent.Status.State != "blocked" {
			t.Fatal("slow native inspection suppressed available terminal state")
		}
	})
	t.Run("read timeout", func(t *testing.T) {
		harness := newControlHarness(t, "read-stall")
		response := harness.call(t, "read", map[string]any{})
		if response.Code != http.StatusOK {
			t.Fatalf("read status=%d", response.Code)
		}
		var result agentcontrol.ReadResult
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Source != "terminal" || result.Scope != "visible" || result.Text == "" {
			t.Fatal("native timeout consumed the terminal fallback budget")
		}
	})
	t.Run("native older output", func(t *testing.T) {
		harness := newControlHarness(t, "ready")
		response := harness.call(t, "read", map[string]any{})
		var result agentcontrol.ReadResult
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Source != "native" || result.Scope != "recent_messages" {
			t.Fatal("native history was not used beyond terminal capture")
		}
	})
}

func TestAgentControlStopPreservesHaltAndClosureOutcomes(t *testing.T) {
	for _, mode := range []string{"unavailable", "stop-replaced"} {
		t.Run(mode, func(t *testing.T) {
			harness := newControlHarness(t, mode)
			response := harness.call(t, "stop", map[string]any{})
			var result agentcontrol.StopResult
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
				t.Fatalf("stop status=%d", response.Code)
			}
			state := harness.state(t)
			if mode == "unavailable" {
				if result.Agent != "unconfirmed" || result.Terminal != "closed" || state.Exists || state.LastKey != "Escape" {
					t.Fatal("fallback stop lost its attempt or overstated cancellation")
				}
			} else if result.Agent != "interrupted" || result.Terminal != "unconfirmed" || result.Reason != "stale" || !state.Exists {
				t.Fatal("partial stop discarded native outcome or closed replacement pane")
			}
		})
	}
}

func TestAgentControlHistorySurvivesUnloadedNativeSession(t *testing.T) {
	harness := newControlHarness(t, "not-loaded")
	response := httptest.NewRecorder()
	harness.gateway.ServeHTTP(response, harness.request(http.MethodGet, "/v1/sessions", ""))
	var inventory sessionsResponseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &inventory); err != nil {
		t.Fatal(err)
	}
	agent := inventory.Sessions[0].Agent
	if agent.Methods.Read != "native" || agent.Methods.Send != "terminal" || agent.Status.Source != "terminal" {
		t.Fatal("unloaded native session lost independent history or terminal status")
	}
	response = harness.call(t, "read", map[string]any{})
	var result agentcontrol.ReadResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Source != "native" {
		t.Fatal("readable unloaded history fell back to terminal")
	}
}

func TestAgentControlStopRevalidatesAfterPhoneDetach(t *testing.T) {
	harness := newControlHarness(t, "idle")
	ctx, cancel := context.WithCancel(context.Background())
	key, _ := harness.gateway.registerLiveTerminal("$1", cancel)
	go func() {
		<-ctx.Done()
		state := harness.state(t)
		state.PaneID = "%2"
		writeControlState(t, harness.statePath, state)
		harness.gateway.unregisterLiveTerminal(key, nil)
	}()
	response := harness.call(t, "stop", map[string]any{})
	var result agentcontrol.StopResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Agent != "interrupted" || result.Terminal != "unconfirmed" || result.Reason != "stale" || !harness.state(t).Exists {
		t.Fatal("stop killed a replacement pane appearing during phone detach")
	}
}

func TestAgentControlFailedLoadCleansOnlyItsBuffer(t *testing.T) {
	harness := newControlHarness(t, "load-lost")
	response := harness.call(t, "send", map[string]any{"mode": "terminal", "text": "fixture"})
	state := harness.state(t)
	if response.Code != http.StatusOK || state.Pastes != 0 || state.BufferName == "" || state.DeletedBuffer != state.BufferName {
		t.Fatal("lost load acknowledgement left its unique input buffer")
	}
}

func TestAgentControlClaudeExitRequiresObservedTerminalOwnership(t *testing.T) {
	for _, mode := range []string{"claude-interactive", "claude-attachment"} {
		t.Run(mode, func(t *testing.T) {
			harness := newControlHarness(t, mode)
			response := harness.call(t, "stop", map[string]any{})
			var result agentcontrol.StopResult
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			want := "unconfirmed"
			if mode == "claude-interactive" {
				want = "stopped"
			}
			if result.Agent != want || result.Terminal != "closed" {
				t.Fatalf("exit outcome=%s/%s, want %s/closed", result.Agent, result.Terminal, want)
			}
		})
	}
}

func TestAgentControlUnloadedCodexStopsThroughTerminal(t *testing.T) {
	harness := newControlHarness(t, "not-loaded")
	response := harness.call(t, "stop", map[string]any{})
	var result agentcontrol.StopResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	state := harness.state(t)
	if result.Agent != "unconfirmed" || result.Terminal != "closed" || state.NativeStops != 0 || state.LastKey != "Escape" {
		t.Fatal("unloaded Codex stop claimed or dispatched a native halt")
	}
}

func TestAgentControlFixtureProcess(t *testing.T) {
	mode := os.Getenv("SKID_CONTROL_FIXTURE")
	if mode == "" {
		return
	}
	if mode == "agent" {
		_, _ = os.Stdout.Write([]byte("r"))
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	path := os.Getenv("SKID_CONTROL_STATE")
	encoded, err := os.ReadFile(path)
	if err != nil {
		os.Exit(60)
	}
	var state controlFixtureState
	if json.Unmarshal(encoded, &state) != nil {
		os.Exit(61)
	}
	write := func() {
		encoded, _ := json.Marshal(state)
		if os.WriteFile(path, encoded, 0o600) != nil {
			os.Exit(62)
		}
	}
	if mode == "native" {
		var request struct {
			Operation string `json:"operation"`
		}
		if json.NewDecoder(os.Stdin).Decode(&request) != nil {
			os.Exit(63)
		}
		if state.NativeMode == "unavailable" {
			fmt.Print(`{"ok":false,"error":{"code":"unavailable","dispatch":"not_sent"}}`)
			os.Exit(1)
		}
		switch request.Operation {
		case "inspect":
			if strings.HasPrefix(state.NativeMode, "claude-") {
				fmt.Printf(`{"ok":true,"result":[{"ok":true,"result":{"status":{"state":"idle","source":"native"},"methods":{"read":"native","send":"terminal","interrupt":"terminal"},"sessionId":"fixture-thread","terminalOwnsAgent":%t}}]}`, state.NativeMode == "claude-interactive")
				os.Exit(0)
			}
			if state.NativeMode == "not-loaded" {
				fmt.Print(`{"ok":true,"result":[{"ok":true,"result":{"status":{"state":"unknown","source":"unavailable","reason":"provider_unavailable"},"methods":{"read":"native","send":"terminal","interrupt":"terminal"},"sessionId":"fixture-thread"}}]}`)
				os.Exit(0)
			}
			if state.NativeMode == "inspect-stall" {
				time.Sleep(12 * time.Second)
			}
			status := "idle"
			if state.NativeMode == "blocked" {
				status = "blocked"
			}
			fmt.Printf(`{"ok":true,"result":[{"ok":true,"result":{"status":{"state":%q,"source":"native"},"methods":{"read":"native","send":"native","interrupt":"native"},"sessionId":"fixture-thread","turnId":"fixture-turn"}}]}`, status)
		case "send":
			state.NativeSends++
			write()
			if state.NativeMode == "unknown" {
				fmt.Print("lost acknowledgement")
				os.Exit(1)
			}
			fmt.Print(`{"ok":true,"result":{"method":"native","outcome":"accepted","turnId":"new-turn"}}`)
		case "read":
			if state.NativeMode == "read-stall" {
				time.Sleep(12 * time.Second)
			}
			fmt.Print(`{"ok":true,"result":{"text":"older than viewport","source":"native","scope":"recent_messages","truncated":false}}`)
		case "stop":
			state.NativeStops++
			write()
			if strings.HasPrefix(state.NativeMode, "claude-") {
				child, _ := os.FindProcess(state.PID)
				_ = child.Kill()
				state.Exists = false
				write()
				fmt.Print(`{"ok":true,"result":{"agent":"unconfirmed"}}`)
				os.Exit(0)
			}
			if state.NativeMode == "stop-replaced" {
				state.PaneID = "%2"
				write()
			}
			fmt.Print(`{"ok":true,"result":{"agent":"interrupted"}}`)
		default:
			fmt.Print(`{"ok":false,"error":{"code":"unsupported","dispatch":"not_sent"}}`)
			os.Exit(1)
		}
		os.Exit(0)
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(64)
	}
	args = args[1:]
	command := args[0]
	last := args[len(args)-1]
	const epoch = "v1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	switch command {
	case "list-sessions":
		if state.Exists {
			if last == "#{session_id}" {
				fmt.Println("$1")
			} else {
				fmt.Println("$1|fixture||0|1|" + epoch + "|1234|1720000000")
			}
		}
	case "show-options":
		switch last {
		case "@skid_server_epoch":
			fmt.Println(epoch)
		case "@skid_character":
			fmt.Println("norse.durinn")
		case "@skid_agent_runtime":
			fmt.Println(state.Registration)
		case "@skid_profile":
			fmt.Println("unit")
		case "@skid_internal", "@skid_objective_b64":
		default:
			os.Exit(65)
		}
	case "display-message":
		switch last {
		case "#{@skid_server_epoch}|#{pid}|#{start_time}":
			fmt.Println(epoch + "|1234|1720000000")
		case "#{session_id}|#{session_name}":
			if state.Exists {
				fmt.Println("$1|fixture")
			} else {
				os.Exit(1)
			}
		case "#{session_id}|#{pane_id}|#{pane_pid}|#{session_attached}|#{session_group_attached}":
			fmt.Println("$1|" + state.PaneID + "|" + strconv.Itoa(state.PID) + "|0|0")
		case "#{pane_current_path}":
			fmt.Println(filepath.Dir(path))
		case "#{pane_current_command}":
			fmt.Println("fixture")
		case "#{alternate_on}":
			fmt.Println("1")
		default:
			os.Exit(66)
		}
	case "capture-pane":
		fmt.Println("❯ 1. Yes\n  2. No\nEnter to select · Esc to cancel")
	case "load-buffer":
		input, _ := io.ReadAll(os.Stdin)
		state.BufferName = args[2]
		state.LiteralInput = string(input) == "first\nsecond; $(literal)"
		write()
		if state.NativeMode == "load-lost" {
			os.Exit(1)
		}
	case "paste-buffer":
		state.Pastes++
		write()
	case "send-keys":
		state.LastKey = last
		write()
	case "delete-buffer":
		state.DeletedBuffer = last
		write()
	case "if-shell":
		if strings.Contains(strings.Join(args, " "), "kill-session -t '$1'") {
			state.Exists = false
			write()
		} else {
			os.Exit(68)
		}
	default:
		os.Exit(67)
	}
	os.Exit(0)
}
