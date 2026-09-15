//go:build integration

package integration_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
)

// The real binary owns terminal-exec. An in-process gateway would launch the
// Go test executable and could not prove the production startup boundary.
type shellGatewayFixture struct {
	root, home, project, binary, socket, socketPath string
	tmuxWrapper, tmuxConfig, hostConfig             string
	origin, backend, bearer                         string
	server                                          *httptest.Server
	client                                          *http.Client
}

func newShellGateway(t *testing.T, tmuxConfiguration string, withProfiles bool) *shellGatewayFixture {
	t.Helper()
	root := t.TempDir()
	fixture := &shellGatewayFixture{root: root, home: filepath.Join(root, "home")}
	fixture.project = filepath.Join(fixture.home, "project ' \\ #{literal};")
	if err := os.MkdirAll(fixture.project, 0o700); err != nil {
		t.Fatal("create shell fixture directory")
	}
	fixture.binary = filepath.Join(root, "skid ' \\ #{literal};")
	built := buildAgentHookCLI(t, repositoryRoot(t), root)
	if err := os.Rename(built, fixture.binary); err != nil {
		t.Fatal("place literal-path gateway executable")
	}
	fixture.socket = randomTmuxSocketName(t, "skid-shell")
	fixture.socketPath = namedTmuxSocketPath(fixture.socket)
	if err := os.MkdirAll(filepath.Dir(fixture.socketPath), 0o700); err != nil {
		t.Fatal("create private socket parent")
	}
	t.Cleanup(func() {
		if err := cleanupRegisteredTmuxSocket(tmuxPath, fixture.socketPath); err != nil {
			t.Errorf("clean shell fixture socket: %v", err)
		}
	})
	fixture.tmuxConfig = filepath.Join(root, "tmux.conf")
	configuration := "set -g default-shell /bin/sh\nset -g window-size latest\nset -g destroy-unattached off\nset -g detach-on-destroy on\n" + tmuxConfiguration
	if err := os.WriteFile(fixture.tmuxConfig, []byte(configuration), 0o600); err != nil {
		t.Fatal("write isolated tmux configuration")
	}
	// The deployment-owned executable is the existing isolation seam. The
	// gateway clears inherited socket environment, so this fixture restores
	// only the registered private root and binds every command to its socket.
	fixture.tmuxWrapper = filepath.Join(root, "tmux ' \\ #{literal};")
	wrapper := "#!/bin/sh\nset -eu\nexport TMUX_TMPDIR=" + shellQuote(testSocketRoot()) + "\n" +
		"case \"${1-}\" in\n" +
		"-V) exec " + shellQuote(tmuxPath) + " -V ;;\n" +
		"-N) shift; exec " + shellQuote(tmuxPath) + " -N -S " + shellQuote(fixture.socketPath) + " -f " + shellQuote(fixture.tmuxConfig) + " \"$@\" ;;\n" +
		"-T) [ \"${2-}\" = RGB ] || exit 64 ;;\n" +
		"-*) exit 64 ;;\n" +
		"esac\nexec " + shellQuote(tmuxPath) + " -S " + shellQuote(fixture.socketPath) + " -f " + shellQuote(fixture.tmuxConfig) + " \"$@\"\n"
	if err := os.WriteFile(fixture.tmuxWrapper, []byte(wrapper), 0o700); err != nil {
		t.Fatal("write isolated tmux executable")
	}
	fixture.hostConfig = filepath.Join(root, "host.json")
	profiles := []any{}
	if withProfiles {
		for _, key := range []string{"personal", "work", "work2", "claude-work"} {
			provider, command, variable := "Codex", "/bin/sleep", "CODEX_HOME"
			arguments := []string{"300"}
			signature := map[string]string{"executableBase": "sleep"}
			if key == "claude-work" {
				provider, command, variable = "Claude", "/bin/cat", "CLAUDE_CONFIG_DIR"
				arguments = []string{}
				signature = map[string]string{"argument0": command}
			}
			profileHome := filepath.Join(fixture.home, key)
			if err := os.Mkdir(profileHome, 0o700); err != nil {
				t.Fatal("create external provider fixture home")
			}
			profiles = append(profiles, map[string]any{
				"key": key, "label": key, "provider": provider, "command": command,
				"arguments":            arguments,
				"environment":          []map[string]string{{"name": variable, "value": profileHome}},
				"foregroundSignatures": []map[string]string{signature},
			})
		}
	}
	config, err := json.Marshal(map[string]any{
		"platform":          platform.Current().Kind,
		"nativeControlPath": filepath.Join(root, "not-installed"),
		"tmux":              map[string]string{"path": fixture.tmuxWrapper, "testedVersion": "tmux 3.4"},
		"profiles":          profiles,
	})
	if err != nil || os.WriteFile(fixture.hostConfig, config, 0o600) != nil {
		t.Fatal("write shell fixture host configuration")
	}
	bearerPath := filepath.Join(root, "bearer")
	fixture.bearer, err = auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint shell fixture bearer")
	}
	machinePath := filepath.Join(root, "machine")
	if err := os.WriteFile(machinePath, []byte(integrationMachineText+"\n"), 0o600); err != nil {
		t.Fatal("write shell fixture machine identity")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("reserve gateway loopback address")
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal("release gateway loopback address")
	}
	fixture.backend = "http://" + address
	command := exec.Command(fixture.binary, "gateway", "--listen="+address,
		"--host-config="+fixture.hostConfig, "--bearer-file="+bearerPath,
		"--machine-handle-file="+machinePath,
		"--catalogue-path="+filepath.Join(repositoryRoot(t), "catalog", "characters.json"))
	command.Env = append(withoutEnvironment(os.Environ(), "HOME", "TMUX", "TMUX_PANE", "TMUX_TMPDIR"), "HOME="+fixture.home)
	// Gateway diagnostics may carry fixture identity. Never copy them into
	// acceptance evidence; response status and process outcomes are sufficient.
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Start(); err != nil {
		t.Fatal("start production shell gateway")
	}
	done := make(chan struct{})
	var processErr error
	go func() {
		processErr = command.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
			return
		default:
		}
		_ = command.Process.Signal(syscall.SIGTERM) // justify-ignore-error: the owned process may have just exited.
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			_ = command.Process.Kill() // justify-ignore-error: only the exact test-owned gateway process is terminated.
			<-done
			t.Error("shell fixture gateway exceeded shutdown deadline")
		}
	})
	probe := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case <-done:
			t.Fatalf("production gateway rejected shell fixture startup: %v", processErr)
		default:
		}
		request, err := http.NewRequest(http.MethodGet, fixture.backend+"/v1/sessions", nil)
		if err != nil {
			t.Fatal("construct gateway readiness observation")
		}
		request.Header.Set("Authorization", "Bearer "+fixture.bearer)
		request.Header.Set("Skidbladnir-Machine", integrationMachineText)
		response, err := probe.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("production gateway did not expose shell fixture inventory")
		}
		// justify-polling: the real subprocess exposes readiness through HTTP.
		time.Sleep(tmuxConvergencePollInterval)
	}
	backend, err := url.Parse(fixture.backend)
	if err != nil {
		t.Fatal("parse gateway loopback origin")
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxy.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	fixture.server = httptest.NewTLSServer(proxy)
	t.Cleanup(fixture.server.Close)
	fixture.origin, fixture.client = fixture.server.URL, fixture.server.Client()
	fixture.client.Timeout = 15 * time.Second
	return fixture
}
