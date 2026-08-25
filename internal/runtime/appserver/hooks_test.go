package appserver

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestListHooksInitializesAndSendsOneExactCWDRequest(t *testing.T) {
	connection := scriptedResponses(
		initializeResponse(),
		fmt.Sprintf(`{"id":2,"result":%s}`, hooksListFixture("/srv/skidbladnir", "user", false, true, "trusted", "command", false)),
	)
	snapshot, err := ListHooks(context.Background(), connection, testCodexHome, "/srv/skidbladnir")
	if err != nil {
		t.Fatalf("list hooks: %v", err)
	}
	if snapshot.CWD != "/srv/skidbladnir" || len(snapshot.Hooks) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	want := strings.Join([]string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"skidbladnir","version":"0.149.1"}}}`,
		`{"method":"initialized"}`,
		`{"id":2,"method":"hooks/list","params":{"cwds":["/srv/skidbladnir"]}}`,
	}, "\n") + "\n"
	if got := connection.requests.String(); got != want {
		t.Fatalf("request transcript = %q, want %q", got, want)
	}
}

func TestListHooksRejectsNonCanonicalCWDBeforeAnyWrite(t *testing.T) {
	for _, cwd := range []string{"relative", "/srv/../srv"} {
		t.Run(cwd, func(t *testing.T) {
			connection := scriptedResponses()
			if _, err := ListHooks(context.Background(), connection, testCodexHome, cwd); err == nil {
				t.Fatalf("accepted non-canonical cwd %q", cwd)
			}
			if got := connection.requests.String(); got != "" {
				t.Fatalf("sent request for invalid cwd: %q", got)
			}
		})
	}
}

func TestListHooksRejectsNonCanonicalCodexHomeBeforeAnyWrite(t *testing.T) {
	connection := scriptedResponses()
	if _, err := ListHooks(context.Background(), connection, "/home/niels/../niels/.codex", "/srv/skidbladnir"); err == nil {
		t.Fatal("accepted non-canonical codex home")
	}
	if got := connection.requests.String(); got != "" {
		t.Fatalf("sent request for invalid codex home: %q", got)
	}
}

func TestDecodeHooksListProjectsOneExactCWD(t *testing.T) {
	snapshot, err := DecodeHooksList([]byte(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false)), "/home/niels/src")
	if err != nil {
		t.Fatalf("decode hooks/list: %v", err)
	}
	if snapshot.CWD != "/home/niels/src" || len(snapshot.Hooks) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	hook := snapshot.Hooks[0]
	if hook.Event != HookEventSessionStart || hook.Handler != HookHandlerCommand || hook.Command != "/home/niels/.local/bin/skidbladnir-hook --pinned-codex /opt/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /run/user/1000/skidbladnir/gaps" || hook.Async || hook.Source != HookSourceUser || hook.Trust != HookTrustTrusted || hook.Managed || !hook.Enabled {
		t.Fatalf("hook projection = %#v", hook)
	}
}

func TestDecodeHooksListRejectsProtocolDrift(t *testing.T) {
	for name, result := range map[string]string{
		"multiple cwd entries":      strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `]}`, `,{}]}`, 1),
		"foreign cwd":               hooksListFixture("/tmp", "user", false, true, "trusted", "command", false),
		"unknown field":             strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `"trustStatus":"trusted"`, `"trustStatus":"trusted","unexpected":true`, 1),
		"duplicate field":           strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `"key":"/home/niels/.codex-personal/hooks.json:session_start:0:0"`, `"key":"other","key":"/home/niels/.codex-personal/hooks.json:session_start:0:0"`, 1),
		"unknown source":            hooksListFixture("/home/niels/src", "remote", false, true, "trusted", "command", false),
		"unknown trust":             hooksListFixture("/home/niels/src", "user", false, true, "reviewed", "command", false),
		"unknown handler":           hooksListFixture("/home/niels/src", "user", false, true, "trusted", "shell", false),
		"errors":                    strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `"errors":[]`, `"errors":[{"path":"/tmp","message":"drift"}]`, 1),
		"async non-boolean":         strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `"async":false`, `"async":"false"`, 1),
		"non-canonical source path": strings.Replace(hooksListFixture("/home/niels/src", "user", false, true, "trusted", "command", false), `"sourcePath":"/home/niels/.codex-personal/hooks.json"`, `"sourcePath":"/home/niels/.codex-personal/../.codex-personal/hooks.json"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeHooksList([]byte(result), "/home/niels/src"); err == nil {
				t.Fatal("accepted protocol drift")
			}
		})
	}
}

func TestDecodeHooksListRejectsNonCanonicalExpectedCWD(t *testing.T) {
	result := hooksListFixture("/srv/../srv", "user", false, true, "trusted", "command", false)
	if _, err := DecodeHooksList([]byte(result), "/srv/../srv"); err == nil {
		t.Fatal("accepted non-canonical expected cwd")
	}
}

func TestDecodeHooksListAcceptsExactSchemaIntegerBounds(t *testing.T) {
	result := hooksListFixture("/srv/skidbladnir", "user", false, true, "trusted", "command", false)
	result = strings.Replace(result, `"timeoutSec":5`, `"timeoutSec":18446744073709551615`, 1)
	result = strings.Replace(result, `"additionalContextLimit":null`, `"additionalContextLimit":18446744073709551615`, 1)
	result = strings.Replace(result, `"displayOrder":0`, `"displayOrder":-9223372036854775808`, 1)
	snapshot, err := DecodeHooksList([]byte(result), "/srv/skidbladnir")
	if err != nil {
		t.Fatalf("decode schema integer bounds: %v", err)
	}
	if got := fmt.Sprint(snapshot.Hooks[0].TimeoutSec); got != "18446744073709551615" {
		t.Fatalf("timeoutSec = %s, want max uint64", got)
	}
}

func TestDecodeHooksListRejectsNumbersOutsideExactSchemaRanges(t *testing.T) {
	base := hooksListFixture("/srv/skidbladnir", "user", false, true, "trusted", "command", false)
	for name, result := range map[string]string{
		"negative timeout":                  strings.Replace(base, `"timeoutSec":5`, `"timeoutSec":-1`, 1),
		"fractional timeout":                strings.Replace(base, `"timeoutSec":5`, `"timeoutSec":1.5`, 1),
		"negative additional context limit": strings.Replace(base, `"additionalContextLimit":null`, `"additionalContextLimit":-1`, 1),
		"overflowing additional context":    strings.Replace(base, `"additionalContextLimit":null`, `"additionalContextLimit":18446744073709551616`, 1),
		"overflowing display order":         strings.Replace(base, `"displayOrder":0`, `"displayOrder":9223372036854775808`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeHooksList([]byte(result), "/srv/skidbladnir"); err == nil {
				t.Fatal("accepted invalid schema numeric")
			}
		})
	}
}

func TestDecodeHooksListAcceptsClosedNonCommandVariantsAndOptionalEmpties(t *testing.T) {
	base := hooksListFixture("/home/niels/src", "project", false, true, "untrusted", "command", false)
	for name, hook := range map[string]string{
		"mcp tool": strings.Replace(base, `"handlerType":"command","command":"/home/niels/.local/bin/skidbladnir-hook --pinned-codex /opt/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /run/user/1000/skidbladnir/gaps","async":false`, `"handlerType":"mcpTool","server":"server","tool":"tool"`, 1),
		"prompt":   strings.Replace(base, `"handlerType":"command","command":"/home/niels/.local/bin/skidbladnir-hook --pinned-codex /opt/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /run/user/1000/skidbladnir/gaps","async":false`, `"handlerType":"prompt"`, 1),
		"agent":    strings.Replace(base, `"handlerType":"command","command":"/home/niels/.local/bin/skidbladnir-hook --pinned-codex /opt/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /run/user/1000/skidbladnir/gaps","async":false`, `"handlerType":"agent"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeHooksList([]byte(hook), "/home/niels/src"); err != nil {
				t.Fatalf("decode known handler variant: %v", err)
			}
		})
	}
	withOptionalEmpties := strings.Replace(base, `"matcher":null,"timeoutSec":5,"statusMessage":null`, `"matcher":"","timeoutSec":601,"statusMessage":""`, 1)
	if _, err := DecodeHooksList([]byte(withOptionalEmpties), "/home/niels/src"); err != nil {
		t.Fatalf("decode optional empty fields: %v", err)
	}
}

func TestDecodeHooksListAcceptsSchemaOptionalMetadataOmission(t *testing.T) {
	result := hooksListFixture("/srv/any-cwd", "user", false, true, "trusted", "command", false)
	for _, field := range []string{`"async":false,`, `"matcher":null,`, `"statusMessage":null,`, `"additionalContextLimit":null,`, `"pluginId":null,`} {
		result = strings.Replace(result, field, "", 1)
	}
	snapshot, err := DecodeHooksList([]byte(result), "/srv/any-cwd")
	if err != nil {
		t.Fatalf("decode schema-optional metadata omission: %v", err)
	}
	if len(snapshot.Hooks) != 1 || snapshot.Hooks[0].Async || snapshot.Hooks[0].Matcher != nil {
		t.Fatalf("optional metadata projection = %#v", snapshot)
	}
}

func hooksListFixture(cwd, source string, managed, enabled bool, trust, handler string, asynchronous bool) string {
	return fmt.Sprintf(`{"data":[{"cwd":%q,"hooks":[{"key":"/home/niels/.codex-personal/hooks.json:session_start:0:0","eventName":"sessionStart","handlerType":%q,"command":"/home/niels/.local/bin/skidbladnir-hook --pinned-codex /opt/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /run/user/1000/skidbladnir/gaps","async":%t,"matcher":null,"timeoutSec":5,"statusMessage":null,"additionalContextLimit":null,"sourcePath":"/home/niels/.codex-personal/hooks.json","source":%q,"pluginId":null,"displayOrder":0,"enabled":%t,"isManaged":%t,"currentHash":"sha256:1111111111111111111111111111111111111111111111111111111111111111","trustStatus":%q}],"warnings":["ignored"],"errors":[]}]}`, cwd, handler, asynchronous, source, enabled, managed, trust)
}
