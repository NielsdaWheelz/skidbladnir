# tmux exec budget and serve priority

2026-10-21: accepted by the owner, including a3's amendments to architecture's
agent-observation paragraph and session latency's no-cache scope; not yet
implemented. the [cleanup campaign](cleanup.md) schedules it (see sequencing).
cross-repo decisions come from dev-server `docs/macbook-capacity-plan.md`
(macbook-capacity d6, d7, d10); this spec cites them and does not vary them.
the [exec overhead](issues/tmux-exec-overhead.md) and
[priority clamp](issues/launchd-standard-priority.md) issues track the problems.

## problem

macbook, 2026-10-21, installed v0.20.0 (`acf4e177`; origin/main `35de077`
carries identical go sources): over 30 s serve used 13.6% of one core, its
reaped tmux children 17.5% and the tmux server 5.0%. serve woke 770 times per
second. 4 hz sampling caught at least 7.1 tmux execs/s, a lower bound. the host
had 18-20 sessions, all agents, at most one attached terminal and no phone or
browser in the foreground.

every process under serve's tmux server, including agents and their tool
calls, runs at scheduling priority 20, because the launchd job declares no
`ProcessType`. ordinary login shells run at 31.

## current exec accounting (origin/main)

one `GET /v1/sessions` (`internal/gateway/gateway.go:390`, `observeSessions`):

| step | code | execs |
| --- | --- | --- |
| server inspection | `internal/sessions/manager.go:171` | 1 |
| epoch admission | `manager.go:180` → `internal/sessions/recovery.go:575` | 1 |
| builtin listing batch | `internal/tmux/inventory.go:66` | 1 |
| metadata batch: per row an `#{S:}` existence guard (O(N) each, O(N²) total), five `show-options`, two markers | `inventory.go:90-98` | 1 |
| identity after normalization | `manager.go:203` | 1 |
| identity after foreground enrichment | `manager.go:233` | 1 |
| per agent pane: screen read, then one `capture-pane` per selected row (≤192 top, ≤64 bottom) | `internal/agentcontrol/terminal.go:30-57`, `internal/tmux/observation.go:78,116-125` | 2 each |

a list costs 6+2K execs for K agent panes: about 8N metadata commands, plus h
capture commands per agent pane for h selected rows.

| caller | cadence | when |
| --- | --- | --- |
| hub observer lane, per machine | 5 s (`internal/notifier/observer.go:445-469` → `internal/fleetclient/client.go:214-222`) | always on the hub, for closed-client attention (`justify-polling` at `observer.go:446`); reaches each host's `/v1/sessions` |
| foreground phone | 5 s per machine | phone open |
| desktop browser | 5 s (`internal/sessionui/session.go:131`) | browser open |

other loops:
- the recovery checkpoint runs every 30 s, about 7 execs
  (`internal/sessions/recovery.go:286-311`, `583-643`).
- presence runs every 2 s per attached web terminal: one `list-clients`
  (`internal/gateway/terminal.go:20`, `internal/tmux/attachment.go:148`).
- attach readiness polls every 25 ms, for at most 5 s, once per attach
  (`attachment.go:22`).
- websocket ping, pressure sampling and process inspection spawn nothing.

predicted at N = K = 18 with one attached terminal and only the hub lane:
42/5 + 7/30 + 0.5 ≈ 9.1 execs/s. that is consistent with the lower bound
observed.

## invariants

these hold today and must keep holding:
- inventory is a sample, not an atomic snapshot ([polling](rules.md#timing-and-polling)).
  every row comes from one admitted server lifetime. a changed lifetime fails
  the list with `WorkspaceSourceChanged`, as today.
- metadata are session-local values. inherited lookup never substitutes for an
  absent local value ([session latency](session-latency.md#native-inventory-and-admission)).
- an observation describes the exact target of the row it enriches: session,
  pane, pane pid and foreground process. a reused observation is valid only
  while that pane's screen provably has not changed since its capture.
- one pane's refusal or failure never fails another pane's observation or the
  inventory.
- budgets and cadence are unchanged: 5 s inventory, a shared 2 s enrichment
  deadline, 5 s callers. no new daemon, connection, route or polling loop.

## option a: two execs per list (recommended)

### a1. inventory in one exec

one `source-file -` script on stdin, using the existing `inventoryBatch`
framing and 8 mib ceiling:

1. `B` bracket: epoch, pid, start time (existing).
2. `show-options -gq` for the five session options `@skid_character`,
   `@skid_auto_name_b64`, `@skid_space_b64`, `@skid_profile`,
   `@skid_objective_b64`.
3. one `list-sessions -F` record per session: today's builtin fields, plus the
   five options as `#{q:@skid_…}` fields, plus the selected pane's
   `pane_width`, `pane_height`, `alternate_on` and `window_activity` (inputs
   to a2 and a3).
4. `show-options -gq` for the same five options again.
5. `E` bracket.

admission: `B` supplies the epoch. if the epoch option is absent, or the
identity differs from the last admitted identity, run today's admission (server
inspection plus `EnsureServerIdentity`) and then the script. an absent server
keeps today's classification (`internal/tmux/recovery.go:110`, `missingServer`)
and today's empty-inventory result.

fallback: format lookup inherits from global options, so the format-read values
are admissible only when both global reads return no `@skid_` option. otherwise
discard them and run today's per-row metadata batch in a second exec. skid
never writes global `@skid_` options, so this path exists only for globals an
operator set by hand.

the two identity re-checks fold into brackets. the inventory script's `E`
bracket replaces the check at `manager.go:203`. the capture script's bracket
(a2) replaces `manager.go:233`, and must still run after foreground enrichment.
with no agent pane, a bracket-only script remains.

consistency: afaict tmux runs one client's queued non-waiting commands
consecutively in one `cmdq_next` pass, without interleaving other clients'
commands or pane input. correctness does not depend on it:
- every row fact, including metadata, comes from the single `list-sessions`
  command, which tmux evaluates against one server state.
- the brackets prove that one server lifetime produced the transcript.
- the only cross-command dependency is the global-option check. it fails only
  if an operator sets and clears a global `@skid_` option between steps 2 and
  4, and requiring both reads to be empty bounds that window to the script's
  own duration.

### a2. all captures in one exec

after foreground enrichment, one `source-file -` script on stdin:
1. a `B` bracket.
2. per agent pane, in inventory order, one block guarded by its target
   (session, pane, pane pid) and by the screen fields a1 read (`width height
   alternate`). inside: a start marker, one `capture-pane -p -e` per selected
   row (existing row selection and region limits), and an end marker. a failed
   guard emits a refusal marker instead.
3. an `E` bracket.

stdin removes the 16 kib client-message limit noted at `observation.go:22-24`.
the 192-row top cap stays, so presentation is unchanged.

- failure isolation: no block may resolve an unguarded `-t` target. a vanished
  pane refuses only its own block. use a format-only existence guard like
  `inventorySessionExists`, or prove per-line command-group isolation on the
  installed tmux.
- fallback: a refused block falls back to today's two-phase `ObservePane` for
  that pane alone, within the shared 2 s deadline.
- limits: per-pane region byte limits are unchanged. the batch uses the 8 mib
  transcript ceiling. overflow falls back to per-pane observation, never to
  silent truncation.

### a3. activity-gated reuse

`agentcontrol`, which owns `Enrich`, keeps one cached detection per live pane.
the key is: server identity, session id, pane id, pane pid, foreground process
identity (pid and start time), width, height, alternate screen and
`window_activity`.

- a pane whose key is unchanged reuses its cached detection with no capture,
  provided the cached capture's wall-clock time is at least 1 s after its
  `window_activity`. activity has one-second resolution, so output in the
  capture's own second could postdate the capture.
- any other pane captures.
- the cache holds live panes only and lives only in serve's memory. it drops
  panes absent from the current inventory, and clears on a server identity
  change or a detected wall-clock step (monotonic and wall deltas disagree).

why this is sound:
- the provider classifier is pure
  ([architecture](architecture.md#agent-observation)), so an unchanged screen
  yields the same detection.
- a reused detection is reported under the current inventory's `observedAt`,
  because the key proves the screen unchanged through this tick.
- `window_activity` is per window, so a multi-pane window recaptures on any
  pane's output. it can over-capture, never under-capture.
- assumption to prove: every change to a pane's visible grid comes either from
  pane output, which bumps `window_activity`, or from a size, alternate-screen
  or foreground change, which is in the key.

net: two execs per list. capture work happens only for panes that printed
output since their last capture.

## option b: host single-flight (not recommended now)

after option a, a list costs two execs and milliseconds of server work. lists
from the hub lane, a phone and a browser seldom overlap in time, and joining
in-flight lists saves at most the overlap. joining would also need semantics for
callers with different deadlines and cancellation. [session latency](session-latency.md)
excluded request single-flight from its scope. revisit only if measurement with
several clients shows overlapping lists dominate.

## option c: control mode (rejected)

- `refresh-client -B` subscriptions cover only the control client's attached
  session. 18 sessions need 18 clients, or a hidden session linking every window.
- attaching a control client raises `session_attached`, which distorts
  inventory's attached-client count and terminal presence.
- it needs a supervisor, `%begin`/`%end` framing, reconnects and new-lifetime
  handling.
- [polling](rules.md#timing-and-polling) forbids push infrastructure added to satisfy a
  generic preference, and [pull to refresh](dashboard-pull-to-refresh.md)
  lists control mode as a non-goal.
- tmux 3.8 `set-hook -B` monitors run tmux commands. they reach skid only
  through `run-shell` processes, which reintroduces spawns.

a command-only control channel would save the remaining ~0.4 execs/s. that
does not pay for the machinery.

## retained loops

- presence: 0.5 execs/s per attached web terminal. it bounds handoff and
  revocation lag.
- attach readiness: bounded and once per attach.
- recovery checkpoint: unchanged here. W2.8 owns its redundant lifetime checks.

## priority (macbook-capacity d7)

- change: `internal/install/services.go` adds
  `<key>ProcessType</key><string>Interactive</string>` to the
  `dev.niels.skidbladnir` plist. the notifications job, which starts its app
  through `open`, is unchanged.
- cause: with no `ProcessType`, launchd applies standard-job limits
  (launchd.plist(5)). `launchctl print` reports `spawn type = daemon (3)`, and
  serve and every descendant run at priority 20.
- evidence: a 2026-10-21 probe of throwaway agents measured child priority 20
  for an absent `ProcessType` and for `Standard`, and 31 for `Interactive`.
- install check: after bootstrap, the installer reads the new serve pid and
  requires its scheduling priority (the value `ps -o pri` reports) to be at
  least 31. otherwise install fails with an action. the undocumented
  `launchctl print` text is not parsed.
- linux is unchanged. `KillMode=process` keeps tmux and the agents in
  `skidbladnir.service`'s cgroup, so a weight or nice value there would
  throttle the agents too.
- running processes keep the clamp. the current tmux server and its panes stay
  at 20 until the server restarts in the maintenance window.

## targets and measurement (macbook-capacity d10)

acceptance runs at N ≥ 18 agent sessions, with no phone or browser in the
foreground and the hub lane active:
- at most 1.5 tmux execs/s from serve.
- serve, its reaped children and the tmux server together under 5% of one core.
- inventory rows, metadata and status equal the current implementation's on
  the same panes.

measurement:
- cpu: `proc_pid_rusage(RUSAGE_INFO_V4)` deltas over 30 s, converted with
  `mach_timebase_info`. use `ri_user_time + ri_system_time` for serve and the
  tmux server, and `ri_child_user_time + ri_child_system_time` for serve's
  reaped children.
- execs: a temporary isolated-socket test points the client's tmux path at a
  counting shim. on the real host, sample serve's children (a lower bound) or
  run `eslogger exec` with operator approval.
- wakeups are reported, not gated.

predicted after option a: hub lane 2/5 s plus recovery 7/30 s ≈ 0.63 execs/s,
plus 0.5/s per attached terminal.

## proving tests

these are temporary tests under [testing](rules.md#testing). run each against
the current and the changed implementation, on an isolated `tmux -L` socket,
with current-turn approval as `AGENTS.md` requires, and delete them before
commit.

1. inventory equivalence: rows and metadata match, with and without local
   `@skid_` values. with a global `@skid_` option set, the fallback runs and
   the results match.
2. lifetime: a server replaced mid-list yields `WorkspaceSourceChanged`. an
   absent server yields today's empty result. a fresh server without an epoch
   takes the admission path.
3. capture equivalence and isolation: batched captures equal per-pane
   captures. a vanished pane, a resized pane and an alternate-screen switch
   each refuse only their own block and fall back.
4. activity:
   - pane output bumps `window_activity`, including output that only rewrites
     existing cells, such as a spinner.
   - a quiet pane is reused.
   - output in the same second as a capture forces a recapture next tick.
   - a resize or a foreground change invalidates the cache entry.
5. budget: the exec count and rusage deltas at N = 18 meet the targets.
6. priority: the installed serve, a newly started tmux server and its pane
   shell all report priority 31.

## sequencing

the [cleanup campaign](cleanup.md) owns scheduling; this spec does not edit it.
overlaps:
- W2.7 sessions-read-path rewrites the same inventory code
  (`one-session-read-path`, `redundant-lifetime-prechecks`,
  `reclassify-and-recapture-merge`). a1 should be its target shape and land in
  or right after W2.7.
- W3.4 observation-cuts and W4.8 observation-refactors touch `Enrich` and the
  capture path. a2 and a3 land after W3.4 removes the diagnostics threading.
- W2.8 recovery-simplify owns the checkpoint's lifetime checks.
- W2.2 install-fixed-paths rewrites `internal/install`. the `ProcessType` line
  is independent and can land before or inside W2.2.
- W1.1 consolidates `docs/rules`, including `polling.md` and the `justify-*`
  convention. the citations here are as of `35de077`.
- the deferred `cross:fleet-read-model` stays deferred. option a meets the cost
  target without a read model.

when a3 lands, the root integrator amends architecture's agent-observation
paragraph from "one focused observation per session" to one per changed
session. a3 also amends session latency's exclusion of caches, so it needs
explicit owner acceptance.

## maintenance window (macbook-capacity d6)

skid needs these steps once, inside dev-server's window:
1. a released skid version with the `ProcessType` change, pinned by dev-server.
   release and re-pin are separate requests under the campaign's process.
2. install it on the macbook through dev-server's pinned path,
   `./workstation gateway apply skidbladnir`, which runs the reviewed
   bootstrap. launchd boots serve out and bootstraps it again; `AbandonProcessGroup` keeps tmux and the agents running.
3. verify that serve runs at priority 31.
4. the operator stops agent work, then runs `tmux kill-server`. serve's
   recovery reconstructs the layout as fresh shells, under a new server it
   starts itself ([session recovery](session-recovery.md): same-boot server
   loss). conversations resume by hand.
5. verify that the new tmux server and a pane shell run at priority 31, and
   record their pids.

options a1-a3 ship with any later release and need no window.

## excluded

- the fenced hub transfer and lagging fleet revisions: the owner handles them
  separately, and W2.1 fixes the hub.
- release pruning and receipt size: W2.2, `host-fleet:installer-receipt`.
- the mac serve log sink: [its issue](issues/macos-serve-log-discarded.md).
