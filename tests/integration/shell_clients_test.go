//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessionui"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
)

// The proxy only delays or cuts actual gateway replies. Every request reaches
// the production gateway process, and every terminal is owned by isolated tmux.
func TestShellDesktopRealTTYCreateAttachDetachAndLostReply(t *testing.T) {
	fixture := newShellGateway(t, "set -g default-shell /bin/sh\n", false)
	target, err := url.Parse(fixture.backend)
	if err != nil {
		t.Fatal("parse owned gateway address")
	}
	var holdReply, cutReply atomic.Bool
	accepted := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseReply := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseReply()
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = http.DefaultTransport.(*http.Transport).Clone()
	proxy.ErrorLog = nil
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.Request.Method != http.MethodPost || response.StatusCode != http.StatusCreated {
			return nil
		}
		if holdReply.Swap(false) {
			accepted <- struct{}{}
			<-release
		}
		if cutReply.Swap(false) {
			response.Body.Close()
			return errors.New("owned connection cut after creation")
		}
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, _ error) {
		connection, _, err := writer.(http.Hijacker).Hijack()
		if err != nil {
			t.Error("could not close owned downstream connection")
			return
		}
		connection.Close()
	}
	server := httptest.NewTLSServer(proxy)
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "example.com:8443" {
			return nil, errors.New("unregistered shell client peer")
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	config := filepath.Join(t.TempDir(), "peers.json")
	encoded, err := json.Marshal(map[string]any{"peers": []map[string]string{{"label": "arch", "origin": "https://example.com:8443", "machine": integrationMachineText, "bearer": fixture.bearer}}})
	if err != nil || os.WriteFile(config, encoded, 0o600) != nil {
		t.Fatal("write private shell client configuration")
	}
	client, err := fleetclient.Open(config)
	if err != nil {
		t.Fatal("open real shell fleet client")
	}
	browser := startShellBrowser(t, client)
	browser.waitFor(t, "empty collection", "no sessions in this view")
	browser.write(t, "n")
	browser.waitFor(t, "terminal launch form", "launch: terminal")
	browser.write(t, "\t\tsource\r\rproject\r")
	browser.waitFor(t, "created source selected", "created source")
	inventory := shellClientInventory(t, client)
	if len(inventory) != 1 || inventory[0].Name != "source" || inventory[0].AttachedClients != 0 || inventory[0].LaunchProfile != "" || inventory[0].Space.String() != "project" {
		t.Fatal("standalone terminal did not select one ordinary, unattached session")
	}
	source := inventory[0]
	browser.write(t, "gjj\r")
	browser.waitFor(t, "project filter", "space: project")
	browser.write(t, "mj\r")
	browser.waitFor(t, "arch filter", "machine: arch")
	browser.waitFor(t, "fresh scoped shell row", "—")
	// The first 201 exists at the actual gateway before duplicate input arrives.
	holdReply.Store(true)
	browser.write(t, "t")
	select {
	case <-accepted:
	case <-time.After(terminalIntegrationTimeout):
		t.Fatal("new terminal here did not reach the production gateway")
	}
	browser.write(t, "t")
	browser.waitFor(t, "duplicate suppressed", "operation in flight")
	releaseReply()
	var created fleetclient.Session
	waitForTerminalCondition(t, "new exact terminal attached", func() bool {
		inventory = shellClientInventory(t, client)
		if len(inventory) != 2 {
			return false
		}
		for _, row := range inventory {
			if row.Ref != source.Ref {
				created = row
			}
		}
		return created.AttachedClients == 1
	})
	if created.Space != source.Space || created.CWD != source.CWD {
		t.Fatal("shortcut did not use the host's current source location")
	}
	browser.write(t, "\x1dd")
	browser.waitFor(t, "detached collection", "detached; work continues")
	browser.waitFor(t, "arch filter", "machine: arch")
	browser.waitFor(t, "project filter", "space: project")
	browser.write(t, " ")
	browser.waitFor(t, "confirmed terminal details", "session: "+created.Name)
	browser.waitFor(t, "confirmed terminal reference", created.Ref)
	browser.write(t, "q")
	browser.waitFor(t, "collection controls", "enter attach")

	cutReply.Store(true)
	browser.write(t, "t")
	browser.waitFor(t, "uncertain creation", "unknown")
	browser.waitFor(t, "no automatic retry", "not repeated")
	inventory = shellClientInventory(t, client)
	if len(inventory) != 3 {
		t.Fatalf("lost creation reply session count=%d want=3", len(inventory))
	}
	browser.write(t, "\x12 ")
	browser.waitFor(t, "confirmed terminal details", "session: "+created.Name)
	browser.waitFor(t, "confirmed terminal reference", created.Ref)
	inventory = shellClientInventory(t, client)
	foundSource := false
	for _, row := range inventory {
		foundSource = foundSource || row.Ref == source.Ref
		if row.AttachedClients != 0 {
			t.Fatal("uncertain creation stole terminal attachment")
		}
	}
	if len(inventory) != 3 || !foundSource {
		t.Fatal("lost reply was retried or destroyed the source")
	}
	browser.write(t, "qq")
	select {
	case err := <-browser.done:
		if err != nil {
			t.Fatal("composed terminal browser failed to quit")
		}
	case <-time.After(terminalIntegrationTimeout):
		t.Fatal("detached browser did not regain keyboard ownership")
	}
}

func shellClientInventory(t *testing.T, client *fleetclient.Client) []fleetclient.Session {
	t.Helper()
	result := client.Execute(context.Background(), fleetclient.Request{Operation: "list"})
	var value fleetclient.Inventory
	if !result.OK || json.Unmarshal(result.Value, &value) != nil || value.Partial || len(value.Peers) != 1 || value.Peers[0].Profiles == nil || len(value.Peers[0].Profiles) != 0 {
		t.Fatal("real client did not observe available zero-profile host")
	}
	return value.Peers[0].Sessions
}

type shellBrowser struct {
	pty    *os.File
	done   chan error
	exited chan struct{}
	mutex  sync.Mutex
	output strings.Builder
}

func startShellBrowser(t *testing.T, client *fleetclient.Client) *shellBrowser {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal("open real browser tty")
	}
	if pty.Setsize(master, &pty.Winsize{Cols: 110, Rows: 32}) != nil {
		t.Fatal("size real browser tty")
	}
	reader, err := cancelreader.NewReader(master)
	if err != nil {
		t.Fatal("open bounded tty observer")
	}
	browser := &shellBrowser{pty: master, done: make(chan error, 1), exited: make(chan struct{})}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if err != nil {
				return
			}
			browser.mutex.Lock()
			if browser.output.Len() < 1024*1024 {
				browser.output.Write(buffer[:n])
			}
			browser.mutex.Unlock()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(browser.exited)
		browser.done <- sessionui.Run(ctx, client, slave, slave)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-browser.exited:
		case <-time.After(terminalIntegrationTimeout):
			t.Error("browser did not stop after cancellation")
		}
		reader.Cancel()
		<-readerDone
		reader.Close()
		master.Close()
		slave.Close()
	})
	return browser
}

func (browser *shellBrowser) write(t *testing.T, text string) {
	t.Helper()
	browser.mutex.Lock()
	browser.output.Reset()
	browser.mutex.Unlock()
	if _, err := io.WriteString(browser.pty, text); err != nil {
		t.Fatal("write browser input")
	}
}

func (browser *shellBrowser) waitFor(t *testing.T, description, text string) {
	t.Helper()
	defer func() {
		if !t.Failed() {
			return
		}
		browser.mutex.Lock()
		defer browser.mutex.Unlock()
		output := browser.output.String()
		t.Logf("browser observer: bytes=%d title=%t checking=%t unavailable=%t protocol_error=%t", len(output), strings.Contains(output, "shared sessions"), strings.Contains(output, "checking inventory"), strings.Contains(output, "unavailable"), strings.Contains(output, "protocol_error"))
		select {
		case err := <-browser.done:
			t.Logf("browser already exited: error_present=%t", err != nil)
		default:
		}
	}()
	waitForTerminalCondition(t, "browser presentation: "+description, func() bool {
		browser.mutex.Lock()
		defer browser.mutex.Unlock()
		return strings.Contains(strings.Join(strings.Fields(ansi.Strip(browser.output.String())), ""), strings.Join(strings.Fields(text), ""))
	})
}
