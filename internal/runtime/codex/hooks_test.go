package codex

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/runtime/appserver"
)

func TestVerifyReviewedHooksAcceptsOnlyClosedReviewedSet(t *testing.T) {
	snapshot := reviewedSnapshot(t, "personal")
	if err := VerifyReviewedHooks("personal", snapshot); err != nil {
		t.Fatalf("verify reviewed hooks: %v", err)
	}
}

func TestVerifyReviewedHooksAcceptsAnyExactDecodedCWD(t *testing.T) {
	snapshot := reviewedSnapshot(t, "personal")
	snapshot.CWD = "/srv/another-validated-cwd"
	if err := VerifyReviewedHooks("personal", snapshot); err != nil {
		t.Fatalf("verify arbitrary decoded cwd: %v", err)
	}
}

func TestVerifyReviewedHooksRejectsRunnableForeignLifecycleHooks(t *testing.T) {
	for name, mutate := range map[string]func(*appserver.HookSnapshot){
		"missing reviewed":   func(snapshot *appserver.HookSnapshot) { snapshot.Hooks = snapshot.Hooks[:6] },
		"duplicate reviewed": func(snapshot *appserver.HookSnapshot) { snapshot.Hooks = append(snapshot.Hooks, snapshot.Hooks[0]) },
		"modified reviewed":  func(snapshot *appserver.HookSnapshot) { snapshot.Hooks[0].CurrentHash = "sha256:modified" },
		"untrusted reviewed": func(snapshot *appserver.HookSnapshot) { snapshot.Hooks[0].Trust = appserver.HookTrustUntrusted },
		"async reviewed":     func(snapshot *appserver.HookSnapshot) { snapshot.Hooks[0].Async = true },
		"managed foreign": func(snapshot *appserver.HookSnapshot) {
			snapshot.Hooks = append(snapshot.Hooks, appserver.Hook{Event: appserver.HookEventStop, Handler: appserver.HookHandlerCommand, Command: "/usr/bin/managed", Key: "managed", SourcePath: "/etc/codex/requirements.toml", Source: appserver.HookSourceSystem, Enabled: true, Managed: true, Trust: appserver.HookTrustManaged, TimeoutSec: 5, CurrentHash: "sha256:managed"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := reviewedSnapshot(t, "personal")
			mutate(&snapshot)
			if err := VerifyReviewedHooks("personal", snapshot); err == nil {
				t.Fatal("accepted runnable lifecycle hook drift")
			}
		})
	}
	for _, event := range []appserver.HookEvent{appserver.HookEventSessionStart, appserver.HookEventUserPromptSubmit, appserver.HookEventStop, appserver.HookEventSessionEnd} {
		event := event
		t.Run("trusted foreign "+string(event), func(t *testing.T) {
			snapshot := reviewedSnapshot(t, "personal")
			snapshot.Hooks = append(snapshot.Hooks, foreignHook(event, true))
			if err := VerifyReviewedHooks("personal", snapshot); err == nil {
				t.Fatal("accepted trusted foreign lifecycle hook")
			}
		})
	}
}

func TestVerifyReviewedHooksIgnoresDormantOrNonLifecycleForeignHooks(t *testing.T) {
	snapshot := reviewedSnapshot(t, "work")
	snapshot.Hooks = append(snapshot.Hooks,
		appserver.Hook{Event: appserver.HookEventStop, Handler: appserver.HookHandlerCommand, Command: "/usr/bin/disabled", Key: "disabled", SourcePath: "/tmp/disabled.json", Source: appserver.HookSourceProject, Enabled: false, Trust: appserver.HookTrustTrusted, TimeoutSec: 5, CurrentHash: "sha256:disabled"},
		appserver.Hook{Event: appserver.HookEventStop, Handler: appserver.HookHandlerCommand, Command: "/usr/bin/untrusted", Key: "untrusted", SourcePath: "/tmp/untrusted.json", Source: appserver.HookSourceProject, Enabled: true, Trust: appserver.HookTrustUntrusted, TimeoutSec: 5, CurrentHash: "sha256:untrusted"},
		appserver.Hook{Event: appserver.HookEventPreToolUse, Handler: appserver.HookHandlerCommand, Command: "/usr/bin/nonlifecycle", Key: "nonlifecycle", SourcePath: "/tmp/nonlifecycle.json", Source: appserver.HookSourceProject, Enabled: true, Trust: appserver.HookTrustTrusted, TimeoutSec: 5, CurrentHash: "sha256:nonlifecycle"},
	)
	if err := VerifyReviewedHooks("work", snapshot); err != nil {
		t.Fatalf("rejected ignored foreign hooks: %v", err)
	}
}

func TestVerifyReviewedHooksIgnoresTrustedForeignActivityHooks(t *testing.T) {
	for _, event := range []appserver.HookEvent{appserver.HookEventPostToolUse, appserver.HookEventSubagentStart, appserver.HookEventSubagentStop} {
		event := event
		t.Run(string(event), func(t *testing.T) {
			snapshot := reviewedSnapshot(t, "work2")
			snapshot.Hooks = append(snapshot.Hooks, foreignHook(event, true))
			if err := VerifyReviewedHooks("work2", snapshot); err != nil {
				t.Fatalf("rejected trusted foreign activity hook: %v", err)
			}
		})
	}
}

func foreignHook(event appserver.HookEvent, enabled bool) appserver.Hook {
	return appserver.Hook{Event: event, Handler: appserver.HookHandlerCommand, Command: "/usr/bin/foreign", Key: "foreign-" + string(event), SourcePath: "/tmp/foreign.json", Source: appserver.HookSourceProject, Enabled: enabled, Trust: appserver.HookTrustTrusted, TimeoutSec: 5, CurrentHash: "sha256:foreign"}
}

const reviewedHookCommandFixture = "/home/niels/.local/bin/skidbladnir-hook --pinned-codex /home/niels/.local/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /home/niels/.local/state/skidbladnir/hook-gaps"

func reviewedSnapshot(t *testing.T, profile string) appserver.HookSnapshot {
	t.Helper()
	var hooks []appserver.Hook
	switch profile {
	case "personal":
		hooks = []appserver.Hook{
			reviewedFixtureHook(appserver.HookEventSessionStart, "/home/niels/.codex-personal/hooks.json:session_start:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:3b0f379949cf7a4fc934e108351d3a6ac7cc3cca55859f1001bcf3170308ec0a", 5),
			reviewedFixtureHook(appserver.HookEventUserPromptSubmit, "/home/niels/.codex-personal/hooks.json:user_prompt_submit:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:ac2aaff534d83744941500927ed45277506e74bf3fdebea0459a7a445a1f54e2", 5),
			reviewedFixtureHook(appserver.HookEventPostToolUse, "/home/niels/.codex-personal/hooks.json:post_tool_use:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:34df58c6aa6ff5db26dc6bb788c6de2de980a7ca177c1f8099b9db87e84aebc1", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStart, "/home/niels/.codex-personal/hooks.json:subagent_start:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:cb1f5da901d975d405346b5affb9c76324328f2a36a93855490cc3baa2878ad9", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStop, "/home/niels/.codex-personal/hooks.json:subagent_stop:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:79ad6e8e0e9f5d0c1cdcffb3001d26d8fa315fff0e5e1789d822342f4773c391", 5),
			reviewedFixtureHook(appserver.HookEventStop, "/home/niels/.codex-personal/hooks.json:stop:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:98227f57d2ee854732044e21a34e399f522551e10686b8ac88a1c1368a552dba", 5),
			reviewedFixtureHook(appserver.HookEventSessionEnd, "/home/niels/.codex-personal/hooks.json:session_end:0:0", "/home/niels/.codex-personal/hooks.json", "sha256:6a15c5edd98abc2b5dbba6a2566dfc31d8bce3e501f6e1d306cc39c49c9cf821", 1),
		}
	case "work":
		hooks = []appserver.Hook{
			reviewedFixtureHook(appserver.HookEventSessionStart, "/home/niels/.codex-work/hooks.json:session_start:0:0", "/home/niels/.codex-work/hooks.json", "sha256:3b0f379949cf7a4fc934e108351d3a6ac7cc3cca55859f1001bcf3170308ec0a", 5),
			reviewedFixtureHook(appserver.HookEventUserPromptSubmit, "/home/niels/.codex-work/hooks.json:user_prompt_submit:0:0", "/home/niels/.codex-work/hooks.json", "sha256:ac2aaff534d83744941500927ed45277506e74bf3fdebea0459a7a445a1f54e2", 5),
			reviewedFixtureHook(appserver.HookEventPostToolUse, "/home/niels/.codex-work/hooks.json:post_tool_use:0:0", "/home/niels/.codex-work/hooks.json", "sha256:34df58c6aa6ff5db26dc6bb788c6de2de980a7ca177c1f8099b9db87e84aebc1", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStart, "/home/niels/.codex-work/hooks.json:subagent_start:0:0", "/home/niels/.codex-work/hooks.json", "sha256:cb1f5da901d975d405346b5affb9c76324328f2a36a93855490cc3baa2878ad9", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStop, "/home/niels/.codex-work/hooks.json:subagent_stop:0:0", "/home/niels/.codex-work/hooks.json", "sha256:79ad6e8e0e9f5d0c1cdcffb3001d26d8fa315fff0e5e1789d822342f4773c391", 5),
			reviewedFixtureHook(appserver.HookEventStop, "/home/niels/.codex-work/hooks.json:stop:0:0", "/home/niels/.codex-work/hooks.json", "sha256:98227f57d2ee854732044e21a34e399f522551e10686b8ac88a1c1368a552dba", 5),
			reviewedFixtureHook(appserver.HookEventSessionEnd, "/home/niels/.codex-work/hooks.json:session_end:0:0", "/home/niels/.codex-work/hooks.json", "sha256:6a15c5edd98abc2b5dbba6a2566dfc31d8bce3e501f6e1d306cc39c49c9cf821", 1),
		}
	case "work2":
		hooks = []appserver.Hook{
			reviewedFixtureHook(appserver.HookEventSessionStart, "/home/niels/.codex-work2/hooks.json:session_start:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:3b0f379949cf7a4fc934e108351d3a6ac7cc3cca55859f1001bcf3170308ec0a", 5),
			reviewedFixtureHook(appserver.HookEventUserPromptSubmit, "/home/niels/.codex-work2/hooks.json:user_prompt_submit:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:ac2aaff534d83744941500927ed45277506e74bf3fdebea0459a7a445a1f54e2", 5),
			reviewedFixtureHook(appserver.HookEventPostToolUse, "/home/niels/.codex-work2/hooks.json:post_tool_use:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:34df58c6aa6ff5db26dc6bb788c6de2de980a7ca177c1f8099b9db87e84aebc1", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStart, "/home/niels/.codex-work2/hooks.json:subagent_start:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:cb1f5da901d975d405346b5affb9c76324328f2a36a93855490cc3baa2878ad9", 5),
			reviewedFixtureHook(appserver.HookEventSubagentStop, "/home/niels/.codex-work2/hooks.json:subagent_stop:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:79ad6e8e0e9f5d0c1cdcffb3001d26d8fa315fff0e5e1789d822342f4773c391", 5),
			reviewedFixtureHook(appserver.HookEventStop, "/home/niels/.codex-work2/hooks.json:stop:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:98227f57d2ee854732044e21a34e399f522551e10686b8ac88a1c1368a552dba", 5),
			reviewedFixtureHook(appserver.HookEventSessionEnd, "/home/niels/.codex-work2/hooks.json:session_end:0:0", "/home/niels/.codex-work2/hooks.json", "sha256:6a15c5edd98abc2b5dbba6a2566dfc31d8bce3e501f6e1d306cc39c49c9cf821", 1),
		}
	default:
		t.Fatalf("unknown profile %q", profile)
	}
	return appserver.HookSnapshot{CWD: "/home/niels/src", Hooks: hooks}
}

func reviewedFixtureHook(event appserver.HookEvent, key, sourcePath, currentHash string, timeoutSec uint64) appserver.Hook {
	return appserver.Hook{Event: event, Handler: appserver.HookHandlerCommand, Command: reviewedHookCommandFixture, Key: key, SourcePath: sourcePath, Source: appserver.HookSourceUser, Enabled: true, Trust: appserver.HookTrustTrusted, TimeoutSec: timeoutSec, CurrentHash: currentHash}
}
