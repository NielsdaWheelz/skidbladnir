// Package codex owns the fixed P0 Codex profile contracts.
package codex

import (
	"fmt"

	"github.com/NielsdaWheelz/skidbladnir/internal/runtime/appserver"
)

const (
	pinnedCodexPath = "/home/niels/.local/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex"
	hookCommand     = "/home/niels/.local/bin/skidbladnir-hook --pinned-codex " + pinnedCodexPath + " --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /home/niels/.local/state/skidbladnir/hook-gaps"
)

type reviewedHook struct {
	event       appserver.HookEvent
	command     string
	key         string
	sourcePath  string
	currentHash string
	timeoutSec  uint64
}

// VerifyReviewedHooks accepts only the exact effective reviewed hook closure
// for one fixed profile. Codex owns discovery; this policy never reads config
// files or reproduces its layer precedence.
func VerifyReviewedHooks(profile string, snapshot appserver.HookSnapshot) error {
	reviewed, ok := reviewedHooksForProfile(profile)
	if !ok || len(snapshot.Hooks) < len(reviewed) {
		return fmt.Errorf("reviewed Codex hooks mismatch")
	}
	found := make(map[appserver.HookEvent]bool, len(reviewed))
	for _, hook := range snapshot.Hooks {
		if expected, reviewedEvent := reviewed[hook.Event]; reviewedEvent {
			if hook.Key == expected.key {
				if found[hook.Event] || !matchesReviewedHook(hook, expected) {
					return fmt.Errorf("reviewed Codex hook mismatch")
				}
				found[hook.Event] = true
				continue
			}
		}
		if lifecycleEvent(hook.Event) && (hook.Managed || hook.Enabled && hook.Trust == appserver.HookTrustTrusted) {
			return fmt.Errorf("foreign runnable lifecycle hook")
		}
	}
	if len(found) != len(reviewed) {
		return fmt.Errorf("missing reviewed Codex hook")
	}
	return nil
}

func reviewedHooksForProfile(profile string) (map[appserver.HookEvent]reviewedHook, bool) {
	var hooksPath string
	switch profile {
	case "personal":
		hooksPath = "/home/niels/.codex-personal/hooks.json"
	case "work":
		hooksPath = "/home/niels/.codex-work/hooks.json"
	case "work2":
		hooksPath = "/home/niels/.codex-work2/hooks.json"
	default:
		return nil, false
	}
	return map[appserver.HookEvent]reviewedHook{
		appserver.HookEventSessionStart:     profileHook(appserver.HookEventSessionStart, hooksPath, 5, "sha256:3b0f379949cf7a4fc934e108351d3a6ac7cc3cca55859f1001bcf3170308ec0a"),
		appserver.HookEventUserPromptSubmit: profileHook(appserver.HookEventUserPromptSubmit, hooksPath, 5, "sha256:ac2aaff534d83744941500927ed45277506e74bf3fdebea0459a7a445a1f54e2"),
		appserver.HookEventPostToolUse:      profileHook(appserver.HookEventPostToolUse, hooksPath, 5, "sha256:34df58c6aa6ff5db26dc6bb788c6de2de980a7ca177c1f8099b9db87e84aebc1"),
		appserver.HookEventSubagentStart:    profileHook(appserver.HookEventSubagentStart, hooksPath, 5, "sha256:cb1f5da901d975d405346b5affb9c76324328f2a36a93855490cc3baa2878ad9"),
		appserver.HookEventSubagentStop:     profileHook(appserver.HookEventSubagentStop, hooksPath, 5, "sha256:79ad6e8e0e9f5d0c1cdcffb3001d26d8fa315fff0e5e1789d822342f4773c391"),
		appserver.HookEventStop:             profileHook(appserver.HookEventStop, hooksPath, 5, "sha256:98227f57d2ee854732044e21a34e399f522551e10686b8ac88a1c1368a552dba"),
		appserver.HookEventSessionEnd:       profileHook(appserver.HookEventSessionEnd, hooksPath, 1, "sha256:6a15c5edd98abc2b5dbba6a2566dfc31d8bce3e501f6e1d306cc39c49c9cf821"),
	}, true
}

func profileHook(event appserver.HookEvent, sourcePath string, timeoutSec uint64, currentHash string) reviewedHook {
	return reviewedHook{event: event, command: hookCommand, key: sourcePath + ":" + eventKey(event) + ":0:0", sourcePath: sourcePath, currentHash: currentHash, timeoutSec: timeoutSec}
}

func matchesReviewedHook(hook appserver.Hook, expected reviewedHook) bool {
	return hook.Handler == appserver.HookHandlerCommand && hook.Command == expected.command && !hook.Async && hook.Key == expected.key && hook.SourcePath == expected.sourcePath && hook.Source == appserver.HookSourceUser && hook.Enabled && !hook.Managed && hook.CurrentHash == expected.currentHash && hook.Trust == appserver.HookTrustTrusted && hook.Matcher == nil && hook.TimeoutSec == expected.timeoutSec
}

func lifecycleEvent(event appserver.HookEvent) bool {
	switch event {
	case appserver.HookEventSessionStart, appserver.HookEventUserPromptSubmit, appserver.HookEventStop, appserver.HookEventSessionEnd:
		return true
	default:
		return false
	}
}

func eventKey(event appserver.HookEvent) string {
	switch event {
	case appserver.HookEventSessionStart:
		return "session_start"
	case appserver.HookEventUserPromptSubmit:
		return "user_prompt_submit"
	case appserver.HookEventPostToolUse:
		return "post_tool_use"
	case appserver.HookEventSubagentStart:
		return "subagent_start"
	case appserver.HookEventSubagentStop:
		return "subagent_stop"
	case appserver.HookEventStop:
		return "stop"
	case appserver.HookEventSessionEnd:
		return "session_end"
	default:
		return ""
	}
}
