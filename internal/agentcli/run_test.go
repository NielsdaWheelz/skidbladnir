package agentcli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"strings"
	"testing"
)

func TestOrdinaryCommandParsingAndLiteralStdin(t *testing.T) {
	parsed, err := parse([]string{"--config", "/private/client.json", "send", "reviewer", "--stdin", "--json"})
	if err != nil || parsed.request.Name != "reviewer" || !parsed.stdin || !parsed.json || parsed.config != "/private/client.json" {
		t.Fatal("ordinary command flags were not admitted")
	}
	parsed, err = parse([]string{"read", "reviewer", "--machine", "arch", "--max-bytes", "32768", "--terminal"})
	if err != nil || parsed.request.Machine != "arch" || parsed.request.Mode != "terminal" || parsed.request.MaxBytes != 32768 {
		t.Fatal("read flags changed")
	}
	parsed, err = parse([]string{"send", "reviewer", "--", "--literal"})
	if err != nil || parsed.request.Text != "--literal" {
		t.Fatal("literal operand changed")
	}
	for _, args := range [][]string{{"send", "reviewer", "text", "--stdin"}, {"list", "--ref", "x"}, {"unknown-command"}, {"list", "--unknown-option"}, {"enter", "reviewer", "--json"}, {"read", "reviewer", "--max-bytes", "0"}} {
		if _, err := parse(args); err == nil {
			t.Error("invalid or retired grammar admitted")
		}
	}
}

func TestFailureIsOneJSONEnvelopeAndUsageIsDistinct(t *testing.T) {
	for _, args := range [][]string{{"list", "--json", "--config", "/missing/client.json"}, {"send", "reviewer", "--stdin", "--json", "--config", "/missing/client.json"}} {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, strings.NewReader("a\nb\n"), &stdout, &stderr)
		var result struct {
			OK bool `json:"ok"`
		}
		if code != 1 || json.Unmarshal(stdout.Bytes(), &result) != nil || result.OK || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 || stderr.Len() != 0 {
			t.Fatal("failure was not one structured envelope")
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"read", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatal("invalid syntax did not exit 2")
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatal("json usage failure was decorated")
	}
}

func TestHumanMetadataIsReadableAndPartialResultsStayStructured(t *testing.T) {
	row := fleetclient.ObservedSession{Label: "arch", Machine: "mh-11111111111111111111111111111111", ObservedAt: "2026-09-13T00:00:00Z", Session: fleetclient.Session{Name: "reviewer", Ref: "opaque", CWD: "/tmp/project", Agent: &fleetclient.Agent{Provider: "Claude", Status: fleetclient.Status{State: "idle", Source: "native"}}}}
	encoded, _ := json.Marshal(row)
	var stdout, stderr bytes.Buffer
	code := render(command{request: fleetclient.Request{Operation: "info"}}, fleetclient.Result{OK: true, Value: encoded}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "session: reviewer") || !strings.Contains(stdout.String(), "state: idle (native)") || json.Valid(stdout.Bytes()) {
		t.Fatal("human metadata is not readable labeled text")
	}
	stdout.Reset()
	stderr.Reset()
	stop := fleetclient.Result{OK: true, Value: json.RawMessage(`{"agent":"unconfirmed","terminal":"closed"}`)}
	if render(command{request: fleetclient.Request{Operation: "stop"}, json: true}, stop, &stdout, &stderr) != 1 || !json.Valid(stdout.Bytes()) || stderr.Len() != 0 {
		t.Fatal("uncertain stop lost its envelope")
	}
}

func TestBareNonTTYPrintsUsageAndExplanation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: skid") || !strings.Contains(stderr.String(), "ttys") {
		t.Fatal("bare non-tty invocation omitted usage or tty explanation")
	}
}

func TestHelpDescribesDesktopBrowserNavigation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("help failed: exit=%d stderr=%q", code, stderr.String())
	}
	help := stdout.String()
	for _, want := range []string{
		"browser (80x24 minimum)", "g spaces; a agents; t tabs", "tab/shift-tab",
		"arrows select locally", "enter attaches fullscreen", "ctrl-] d returns to the browser",
		"T (shift+t) creates and attaches a terminal here", "e edits membership",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("browser help omits %q", want)
		}
	}
	for _, retired := range []string{"t creates and attaches", "g chooses space"} {
		if strings.Contains(help, retired) {
			t.Errorf("browser help retains retired binding %q", retired)
		}
	}
}

func TestSpaceCommandGrammar(t *testing.T) {
	ref := fleetclient.Reference{Machine: "mh-11111111111111111111111111111111", TmuxID: "$1", IdentityToken: "fixture"}.Encode()
	for index, args := range [][]string{
		{"list", "--space", "alpha"}, {"list", "--unassigned"},
		{"start", "worker", "--machine", "arch", "--profile", "work", "--space=alpha"},
		{"space", "worker", "--set", "alpha"}, {"space", "--ref", ref, "--clear"},
		{"space", "--set", "alpha", "--", "--literal"},
	} {
		if _, err := parse(args); err != nil {
			t.Errorf("space grammar rejected: case=%d", index)
		}
	}
	for index, args := range [][]string{
		{"--space", "alpha"}, {"list", "--space", "alpha", "--unassigned"},
		{"start", "worker", "--machine", "arch", "--profile", "work", "--unassigned"},
		{"info", "worker", "--space", "alpha"}, {"space", "worker"},
		{"space", "worker", "--set", "alpha", "--clear"}, {"space", "worker", "--set", ""},
		{"space", "worker", "--clear", "--space", "alpha"}, {"space", "worker", "--clear=true"},
		{"space", "worker", "--set", " alpha"}, {"space", "worker", "--set", "a\tb"},
	} {
		if _, err := parse(args); err == nil {
			t.Errorf("invalid space grammar admitted: case=%d", index)
		}
	}
}

func TestSpaceHumanGroupingAndAcknowledgement(t *testing.T) {
	var value fleetclient.Inventory
	if json.Unmarshal([]byte(`{"partial":true,"peers":[{"label":"arch","machine":"mh-11111111111111111111111111111111","ok":true,"profiles":[],"sessions":[{"name":"later","ref":"fixture","attachedClients":0,"space":"zulu"},{"name":"first","ref":"fixture","attachedClients":0,"space":"alpha"},{"name":"shell","ref":"fixture","attachedClients":0}]},{"label":"offline","machine":"mh-22222222222222222222222222222222","ok":false,"error":{"code":"unavailable","dispatch":"not_sent"}}]}`), &value) != nil {
		t.Fatal("decode synthetic inventory")
	}
	encoded, _ := json.Marshal(value)
	var stdout, stderr bytes.Buffer
	if render(command{request: fleetclient.Request{Operation: "list"}}, fleetclient.Result{OK: true, Value: encoded}, &stdout, &stderr) != 1 {
		t.Fatal("partial grouped list lost failure status")
	}
	text := stdout.String()
	alpha, zulu, unassigned := strings.Index(text, "space: alpha"), strings.Index(text, "space: zulu"), strings.Index(text, "unassigned")
	if alpha < 0 || zulu <= alpha || unassigned <= zulu || strings.Count(text, "offline") != 1 {
		t.Fatal("human grouping lost ordering, headings, or peer outage")
	}
	stdout.Reset()
	if render(command{request: fleetclient.Request{Operation: "space"}}, fleetclient.Result{OK: true, Value: json.RawMessage(`{"space":""}`)}, &stdout, &stderr) != 0 || stdout.String() != "space cleared\n" {
		t.Fatal("clear acknowledgement fabricated other output")
	}
	parsed, err := parse([]string{"space", "worker", "--set", "e\u0301"})
	if err != nil || parsed.request.Space.String() != "é" {
		t.Fatal("human membership input did not normalize nfc")
	}
}
