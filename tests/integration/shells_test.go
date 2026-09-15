//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func TestShellCreationRejectsDirectoryLossBeforeStartingTmux(t *testing.T) {
	fixture := newShellGateway(t, "", false)
	client, err := tmuxclient.New(fixture.tmuxWrapper, "")
	if err != nil {
		t.Fatal("construct configured tmux client")
	}
	_, dispatched, err := client.CreateSession(context.Background(), filepath.Join(fixture.home, "removed"), "", tmuxclient.ServerIdentity{}, []string{"-d", "-s", "must-not-exist", "--", sleepPath, "300"})
	if err == nil || dispatched {
		t.Fatal("directory loss before process start did not prove not_sent")
	}
	if _, err := os.Lstat(fixture.socketPath); !os.IsNotExist(err) {
		t.Fatal("rejected directory started tmux")
	}
}

func TestShellCreationRetainsSessionReferenceAcrossObservedAgentReplacement(t *testing.T) {
	fixture := newShellGateway(t, "", true)
	body, err := json.Marshal(map[string]string{
		"kind": "agent", "profile": "personal", "cwd": fixture.home,
		"optionalTmuxName": "observed-agent", "space": "project", "objective": "do not copy",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusCreated)
	source := decodeCreateSessionResponse(t, response)
	sourceID, retainedToken := source["tmuxId"].(string), source["identityToken"].(string)
	observe := func() map[string]any {
		response := request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
		assertStatus(t, response, http.StatusOK)
		for _, value := range decodeObject(t, response)["sessions"].([]any) {
			row := value.(map[string]any)
			if row["tmuxId"] == sourceID {
				if row["identityToken"] != retainedToken {
					t.Fatal("source session lifetime changed during process replacement")
				}
				agent, _ := row["agent"].(map[string]any)
				return agent
			}
		}
		return nil
	}
	var originalAgent map[string]any
	waitForTerminalCondition(t, "actual source agent observed", func() bool {
		originalAgent = observe()
		return originalAgent != nil && originalAgent["provider"] == "Codex"
	})
	directory := filepath.Join(fixture.home, "changed")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal("create replacement agent directory")
	}
	agentHookTmux(t, fixture.socket, "rename-session", "-t", sourceID, "renamed-agent")
	agentHookTmux(t, fixture.socket, "respawn-pane", "-k", "-t", sourceID, "-c", directory, "--", sleepPath, "300")
	waitForTerminalCondition(t, "replacement agent lifetime observed", func() bool {
		current := observe()
		return current != nil && current["provider"] == "Codex" &&
			(current["pid"] != originalAgent["pid"] || current["startIdentity"] != originalAgent["startIdentity"])
	})
	body, err = json.Marshal(map[string]string{"identityToken": retainedToken})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions/"+sourceID+"/shell", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusCreated)
	created := decodeCreateSessionResponse(t, response)
	if created["tmuxId"] == sourceID || created["cwd"] != directory || created["space"] != "project" || created["launchProfile"] != nil || created["objective"] != nil || created["agent"] != nil {
		t.Fatal("retained session reference did not create an independent terminal after agent replacement")
	}
	if observe() == nil {
		t.Fatal("terminal creation replaced the source agent")
	}
}

func TestShellCreationEnforcesAdmissionAndStrictStartup(t *testing.T) {
	fixture := newShellGateway(t, "set-option -g remain-on-exit on\n", false)
	create, err := json.Marshal(map[string]string{"kind": "terminal", "cwd": fixture.home})
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(create))
	assertStatus(t, response, http.StatusCreated)
	source := decodeCreateSessionResponse(t, response)
	sourceID := source["tmuxId"].(string)
	shellBody, err := json.Marshal(map[string]string{"identityToken": source["identityToken"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, path, body, bearer, machine, code string
		status                                  int
	}{
		{"legacy launch", "/v1/sessions", `{"cwd":"~","profile":"personal"}`, fixture.bearer, integrationMachineText, "InvalidRequest", 400},
		{"terminal profile", "/v1/sessions", `{"kind":"terminal","profile":"personal","cwd":"~"}`, fixture.bearer, integrationMachineText, "InvalidRequest", 400},
		{"duplicate kind", "/v1/sessions", `{"kind":"terminal","kind":"terminal","cwd":"~"}`, fixture.bearer, integrationMachineText, "InvalidRequest", 400},
		{"cwd override", "/v1/sessions/" + sourceID + "/shell", `{"identityToken":"unused","cwd":"~"}`, fixture.bearer, integrationMachineText, "InvalidRequest", 400},
		{"null identity", "/v1/sessions/" + sourceID + "/shell", `{"identityToken":null}`, fixture.bearer, integrationMachineText, "InvalidRequest", 400},
		{"stale identity", "/v1/sessions/" + sourceID + "/shell", `{"identityToken":"stale"}`, fixture.bearer, integrationMachineText, "SessionIdentityMismatch", 409},
		{"create auth", "/v1/sessions", string(create), "invalid", integrationMachineText, "Unauthenticated", 401},
		{"source auth", "/v1/sessions/" + sourceID + "/shell", string(shellBody), "invalid", integrationMachineText, "Unauthenticated", 401},
		{"create machine", "/v1/sessions", string(create), fixture.bearer, "mh-22222222222222222222222222222222", "MachineIdentityMismatch", 409},
		{"source machine", "/v1/sessions/" + sourceID + "/shell", string(shellBody), fixture.bearer, "mh-22222222222222222222222222222222", "MachineIdentityMismatch", 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := requestForMachine(t, fixture.client, http.MethodPost, fixture.origin+test.path, test.bearer, test.machine, "", test.body)
			assertStatus(t, response, test.status)
			failure := decodeObject(t, response)
			if len(failure) != 3 || failure["code"] != test.code || failure["dispatch"] != "not_sent" {
				t.Fatal("creation rejection violated its exact dispatch-bearing error contract")
			}
		})
	}
	response = request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	if len(decodeObject(t, response)["sessions"].([]any)) != 1 {
		t.Fatal("definite creation rejection changed sessions")
	}

	// tmux would silently substitute /bin/sh for this setting. The helper must
	// instead retain an ordinary failed pane without executing a fallback.
	shellPath := filepath.Join(fixture.root, "removed-shell")
	if err := os.Symlink("/bin/sh", shellPath); err != nil {
		t.Fatal("create configured shell")
	}
	agentHookTmux(t, fixture.socket, "set-option", "-g", "default-shell", shellPath)
	if err := os.Remove(shellPath); err != nil {
		t.Fatal("remove configured shell after tmux admission")
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(create))
	assertStatus(t, response, http.StatusCreated)
	invalidShell := decodeCreateSessionResponse(t, response)["tmuxId"].(string)
	waitForTerminalCondition(t, "invalid shell exits without substitution", func() bool {
		return agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", invalidShell, "#{pane_dead}:#{pane_dead_status}") == "1:1"
	})

	// The real tmux subprocess has entered this directory before its configured
	// executable removes it. The helper must reject its own subsequent chdir.
	agentHookTmux(t, fixture.socket, "set-option", "-g", "default-shell", "/bin/sh")
	lostDirectory := filepath.Join(fixture.home, "lost-at-dispatch")
	if err := os.Mkdir(lostDirectory, 0o700); err != nil {
		t.Fatal("create disappearing directory")
	}
	wrapper, err := os.ReadFile(fixture.tmuxWrapper)
	if err != nil {
		t.Fatal("read configured tmux test executable")
	}
	interposed := strings.Replace(string(wrapper), "set -eu\n", "set -eu\nif [ \"${1-}\" = new-session ]; then rmdir "+shellQuote(lostDirectory)+"; fi\n", 1)
	if err := os.WriteFile(fixture.tmuxWrapper, []byte(interposed), 0o700); err != nil {
		t.Fatal("arm directory-loss boundary")
	}
	create, err = json.Marshal(map[string]string{"kind": "terminal", "cwd": lostDirectory})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(create))
	assertStatus(t, response, http.StatusCreated)
	lost := decodeCreateSessionResponse(t, response)["tmuxId"].(string)
	waitForTerminalCondition(t, "lost working directory exits without fallback", func() bool {
		return agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", lost, "#{pane_dead}:#{pane_dead_status}") == "1:1"
	})
}

func TestShellCreationReportsUnknownAfterPartialQueueExecution(t *testing.T) {
	fixture := newShellGateway(t, "", false)
	wrapper, err := os.ReadFile(fixture.tmuxWrapper)
	if err != nil {
		t.Fatal("read configured tmux test executable")
	}
	// The tmux queue creates the session, then its final target lookup fails.
	// This is a real external failure after the irreversible create boundary.
	interposed := strings.Replace(string(wrapper), "set -eu\n", "set -eu\nif [ \"${1-}\" = new-session ]; then exec "+shellQuote(tmuxPath)+" -S "+shellQuote(fixture.socketPath)+" -f "+shellQuote(fixture.tmuxConfig)+" \"$@\" ';' set-option -t '=missing-test-session:' @skid_test never; fi\n", 1)
	if err := os.WriteFile(fixture.tmuxWrapper, []byte(interposed), 0o700); err != nil {
		t.Fatal("arm partial queue failure")
	}
	body, err := json.Marshal(map[string]string{"kind": "terminal", "cwd": fixture.home})
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusInternalServerError)
	failure := decodeObject(t, response)
	if len(failure) != 3 || failure["code"] != "InternalError" || failure["dispatch"] != "unknown" {
		t.Fatal("partial creation lost unknown dispatch evidence")
	}
	response = request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	rows := decodeObject(t, response)["sessions"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["tmuxName"] != "skidbladnir-terminal-1" {
		t.Fatal("unknown creation was replayed, hidden, or compensated")
	}

	// A source metadata observation failure precedes the create queue.
	if err := os.WriteFile(fixture.tmuxWrapper, []byte(strings.Replace(string(wrapper), "set -eu\n", "set -eu\nif [ \"${1-}\" = show-options ] && [ \"${5-}\" = @skid_space_b64 ]; then exit 75; fi\n", 1)), 0o700); err != nil {
		t.Fatal("arm source metadata read failure")
	}
	source := rows[0].(map[string]any)
	body, err = json.Marshal(map[string]string{"identityToken": source["identityToken"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions/"+source["tmuxId"].(string)+"/shell", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusInternalServerError)
	if decodeObject(t, response)["dispatch"] != "not_sent" {
		t.Fatal("failed source observation was not a definite rejection")
	}
	response = request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	if len(decodeObject(t, response)["sessions"].([]any)) != 1 {
		t.Fatal("source observation failure created a session")
	}
}

func TestShellCreationGatesTheSourceLifetimeAtDispatch(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(strconv.FormatBool(restart), func(t *testing.T) {
			fixture := newShellGateway(t, "", false)
			body, err := json.Marshal(map[string]string{"kind": "terminal", "cwd": fixture.home})
			if err != nil {
				t.Fatal(err)
			}
			response := request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(body))
			assertStatus(t, response, http.StatusCreated)
			source := decodeCreateSessionResponse(t, response)
			original := captureTestTmuxServer(t, tmuxPath, fixture.socketPath)
			ready, release := filepath.Join(fixture.root, "dispatch-ready"), filepath.Join(fixture.root, "dispatch-release")
			wrapper, err := os.ReadFile(fixture.tmuxWrapper)
			if err != nil {
				t.Fatal("read configured tmux test executable")
			}
			gate := "set -eu\nif [ \"${1-}\" = -N ]; then : > " + shellQuote(ready) + "; n=0; while [ ! -f " + shellQuote(release) + " ]; do n=$((n+1)); [ \"$n\" -lt 500 ] || exit 76; sleep 0.01; done; fi\n"
			if err := os.WriteFile(fixture.tmuxWrapper, []byte(strings.Replace(string(wrapper), "set -eu\n", gate, 1)), 0o700); err != nil {
				t.Fatal("arm source dispatch gate")
			}
			body, err = json.Marshal(map[string]string{"identityToken": source["identityToken"].(string)})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, fixture.origin+"/v1/sessions/"+source["tmuxId"].(string)+"/shell", bytes.NewReader(body))
			if err != nil {
				t.Fatal("construct gated source request")
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+fixture.bearer)
			req.Header.Set("Skidbladnir-Machine", integrationMachineText)
			result := make(chan struct {
				response *http.Response
				err      error
			}, 1)
			go func() {
				response, err := fixture.client.Do(req)
				result <- struct {
					response *http.Response
					err      error
				}{response, err}
			}()
			waitForTerminalCondition(t, "source reaches dispatch boundary", func() bool { _, err := os.Stat(ready); return err == nil })
			agentHookTmux(t, fixture.socket, "kill-session", "-t", source["tmuxId"].(string))
			waitForTerminalCondition(t, "source server exits", func() bool { return processStartIdentity(original.pid) != original.kernelStartTime })
			if restart {
				if _, err := isolatedTmuxCommand(tmuxPath, "-S", fixture.socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "replacement", "--", sleepPath, "300").Output(); err != nil {
					t.Fatal("start replacement test server")
				}
			}
			if err := os.WriteFile(release, nil, 0o600); err != nil {
				t.Fatal("release source dispatch")
			}
			completed := <-result
			if completed.err != nil {
				t.Fatal("receive gated source rejection")
			}
			wantStatus, wantDispatch := http.StatusInternalServerError, "unknown"
			if restart {
				wantStatus, wantDispatch = http.StatusConflict, "not_sent"
			}
			assertStatus(t, completed.response, wantStatus)
			if decodeObject(t, completed.response)["dispatch"] != wantDispatch {
				t.Fatal("source lifetime gate misreported dispatch")
			}
			response = request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
			assertStatus(t, response, http.StatusOK)
			rows := decodeObject(t, response)["sessions"].([]any)
			if restart {
				if len(rows) != 1 || rows[0].(map[string]any)["tmuxName"] != "replacement" {
					t.Fatal("source lifetime gate changed replacement server sessions")
				}
			} else if len(rows) != 0 {
				t.Fatal("source lifetime gate started a replacement server")
			}
		})
	}
}

func TestShellCreationOwnsAnIndependentSession(t *testing.T) {
	fixture := newShellGateway(t, "set-option -g default-shell /bin/sh\nset-option -g default-command 'exit 71'\n", false)
	response := request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	profiles, ok := inventory["profiles"].([]any)
	if !ok || len(profiles) != 0 {
		t.Fatal("zero-profile inventory omitted its empty launch table")
	}
	if _, err := os.Lstat(fixture.socketPath); !os.IsNotExist(err) {
		t.Fatal("inventory started the idle server")
	}

	directory := filepath.Join(fixture.home, "literal ' \\ #{session_name};")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal("create literal working directory")
	}
	body, err := json.Marshal(map[string]string{"kind": "terminal", "cwd": directory, "space": "project"})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusCreated)
	source := decodeCreateSessionResponse(t, response)
	sourceID := source["tmuxId"].(string)
	if source["tmuxName"] != "skidbladnir-terminal-1" || source["space"] != "project" || source["launchProfile"] != nil {
		t.Fatal("terminal did not receive ordinary identity and optional metadata")
	}
	waitForTerminalCondition(t, "literal shell working directory", func() bool {
		return agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", sourceID, "#{pane_current_path}") == directory &&
			agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", sourceID, "#{pane_current_command}") == "sh"
	})
	if got := agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", sourceID, "#{session_path}"); got != directory {
		t.Fatal("new session lost its literal default directory")
	}
	pid, err := strconv.Atoi(agentHookTmux(t, fixture.socket, "display-message", "-p", "-t", sourceID, "#{pane_pid}"))
	if err != nil {
		t.Fatal("read terminal process identity")
	}
	process, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil || process.Argument(0) != "-sh" || len(process.Argv) != 1 {
		t.Fatal("terminal did not execute the configured login shell directly")
	}

	// A retained session lifetime survives name and foreground replacement.
	agentHookTmux(t, fixture.socket, "rename-session", "-t", sourceID, "renamed-source")
	agentHookTmux(t, fixture.socket, "respawn-pane", "-k", "-t", sourceID, "-c", fixture.home, "--", sleepPath, "300")
	body, err = json.Marshal(map[string]string{"identityToken": source["identityToken"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions/"+sourceID+"/shell", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusCreated)
	created := decodeCreateSessionResponse(t, response)
	createdID := created["tmuxId"].(string)
	if createdID == sourceID || created["tmuxName"] != "skidbladnir-terminal-1" || created["space"] != "project" || created["cwd"] != fixture.home || created["launchProfile"] != nil || created["objective"] != nil {
		t.Fatal("source creation did not copy only fresh cwd/space into an independent terminal")
	}
	agentHookTmux(t, fixture.socket, "set-option", "-u", "-t", sourceID, "@skid_space_b64")
	agentHookTmux(t, fixture.socket, "kill-session", "-t", sourceID)
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions/"+sourceID+"/shell", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusNotFound)
	failure := decodeObject(t, response)
	if failure["dispatch"] != "not_sent" || len(failure) != 3 {
		t.Fatal("stale source rejection lost definite dispatch evidence")
	}
	response = request(t, fixture.client, http.MethodGet, fixture.origin+"/v1/sessions", fixture.bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	rows := inventory["sessions"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["tmuxId"] != createdID || rows[0].(map[string]any)["space"] != "project" {
		t.Fatal("source mutation/closure changed the independent terminal")
	}
	// An inherited global option is not local membership.
	agentHookTmux(t, fixture.socket, "set-option", "-u", "-t", createdID, "@skid_space_b64")
	agentHookTmux(t, fixture.socket, "set-option", "-g", "@skid_space_b64", "Z2xvYmFs")
	body, err = json.Marshal(map[string]string{"identityToken": created["identityToken"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, fixture.client, http.MethodPost, fixture.origin+"/v1/sessions/"+createdID+"/shell", fixture.bearer, "", string(body))
	assertStatus(t, response, http.StatusCreated)
	unassigned := decodeCreateSessionResponse(t, response)
	if unassigned["space"] != nil || unassigned["tmuxId"] == createdID {
		t.Fatal("source creation inherited global membership or reused a session")
	}
}
