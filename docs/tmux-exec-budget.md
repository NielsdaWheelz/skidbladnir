# tmux exec budget and serve priority

Every tmux command skid runs is a process spawn, and the hub lists every machine
every five seconds whether or not anyone is looking, so inventory's exec count is
a standing cost. This spec bounds it to two execs per steady list and
sets serve's launchd priority. dev-server's `docs/macbook-capacity-plan.md`
(decisions d6, d7, d10) supplies the targets; the open measurement is
[exec overhead](issues/tmux-exec-overhead.md).

## Invariants

- Inventory is a sample, not an atomic snapshot ([polling](rules.md#timing-and-polling)).
  Every row comes from one admitted server lifetime; a changed lifetime fails the
  list with `WorkspaceSourceChanged`.
- Metadata are session-local values; inherited lookup never substitutes for an
  absent local value. The residual: a window- or pane-local `@skid_` definition on
  the selected pane shadows the format value undetected, and a non-catalogue
  window or pane `@skid_character` makes every list rewrite that session's local
  key. skid writes none, and such definitions are unsupported.
- An observation describes the exact target of the row it enriches: session,
  pane, pane PID and foreground process. A reused observation is valid only while
  that pane's screen provably has not changed since its capture.
- One pane's refusal or failure never fails another pane's observation or the
  inventory.
- Budgets and cadence: 5 s inventory, one shared 2 s enrichment deadline, 5 s
  callers. No new daemon, connection, route or polling loop.

## Two execs per list

### a1. Inventory in one exec

One `source-file -` script on stdin, framed by `B`/`E` identity records within the
8 MiB transcript ceiling:

1. `B`: epoch, PID, start time.
2. `show-options -q` at `-s`, `-g` and `-g -w` for the five session options
   `@skid_character`, `@skid_auto_name_b64`, `@skid_space_b64`, `@skid_profile`,
   `@skid_objective_b64`. Format lookup checks server options, then pane, window
   and global-window options, then session and global-session options, so `-s` and
   `-g -w` shadow even a present local value and `-g` fills its absence.
3. One `list-sessions -F` record per session: the builtin fields, the five options
   as `#{q:@skid_…}` fields, and the selected pane's `pane_width`, `pane_height`,
   `alternate_on` and `window_activity` (inputs to a2 and a3).
4. The same fifteen `show-options -q` reads again.
5. `E`.

Admission: `B` supplies the epoch. When the epoch option is absent, or the identity
differs from the last admitted one, the manager inspects the server, ensures its
epoch, and runs the script again. An absent server is an empty inventory.

Fallback: format values inherit from server, global-window and global-session
options, so they are admissible only when both probe reads at all three scopes
return no `@skid_` option; otherwise they are discarded and a second exec reads
each session's local options. skid never writes global `@skid_` options, so this
path exists only for globals an operator set by hand.

Consistency: every row fact, metadata included, comes from the single
`list-sessions` command, which tmux evaluates against one server state; the
brackets prove that one server lifetime produced the transcript; and the only
cross-command dependency, the global-option check, fails only if an operator sets
and clears a global `@skid_` option within the script's own duration, which
requiring both reads to be empty bounds. AFAICT tmux also runs one client's queued
commands consecutively, but correctness does not depend on it.

### a2. All captures in one exec

After foreground enrichment, one `source-file -` script on stdin
(`agentcontrol.Inventory` → `sessions.Manager.CapturePanes` →
`tmux.Client.CapturePanes`):

1. `B`.
2. Per agent pane, in inventory order, one block guarded by the row's exact target:
   lifetime, session ID, selected `pane_id`, `pane_pid` and the `width height
   alternate` that a1 read. Inside, a start marker, one `capture-pane -p -e` per
   selected row (the ordinary row selection and region limits) and an end marker. A
   failed guard prints one refusal marker instead.
3. `E`.

- Every `-t` sits inside its block's guard: an unguarded failing `-t` skips the rest
  of its command group and exits the client 1, losing the whole transcript, while an
  `if-shell -F -t` naming a vanished session evaluates without a target, so its
  guard is false (probed on tmux 3.8).
- Markers frame each block and the rows inside are counted, so a row that looks
  like a marker is data. A non-UTF-8 row fails only its pane; any other surprise
  fails the batch.
- A refused block falls back to per-pane observation for that pane alone, within
  the shared 2 s deadline; the fallback tells a changed target from a changed
  screen. Each captured pane's foreground is then re-sampled against the session's
  own sample. A row with no rows or columns gets no block and reads
  `capture_failed`.
- stdin lifts tmux's 16 KiB client-message limit, but the 192-row top cap stays so
  that batch and per-pane observations agree. Region byte limits are unchanged;
  overflow falls back per pane, never to silent truncation.
- A batch with no bracketed transcript (deadline, client failure, overflow) falls
  back per pane for every pane, then one `InspectServer` under the list's own 5 s
  deadline proves the lifetime, so a slow capture never fails the list. A bracket
  that proves another lifetime, or none, fails it with `WorkspaceSourceChanged`.
- A batch that overruns the 2 s deadline leaves its fallbacks no time: every
  pending pane reads `observation_timeout` for that list. One exec instead of 2K is
  the price.
- An inventory with rows always sends the batch, a bracket alone when nothing needs
  capture; an inventory with no rows sends none, since nothing in it names a
  lifetime.
- Creation keeps per-pane observation under its own 2 s deadline: its pane rarely
  runs the provider yet, and a target change reads `capture_failed`, never failing
  the create response.

### a3. Activity-gated reuse

`agentcontrol` (`enrich.go`) keeps one cached detection per live pane, keyed by
`sessions.Session.ScreenKey`: server identity, session ID, pane ID, pane PID, the
whole foreground sample (provider, PID and start time; exec keeps the last two),
width, height, alternate screen and `window_activity`.

- A pane whose key is unchanged reuses its cached detection without a capture; any
  other pane captures.
- An entry is stored only from a batch capture whose start, taken before the batch
  exec, is at least `window_activity` + 1.1 s: 1 s for activity's one-second
  resolution and 100 ms for a clock step too small to detect. A fallback capture is
  never stored: its fresh screen need not match the key, and a resize back without
  output would reuse the other size's detection.
- Only an inventory reads or writes the cache, keeping exactly its own settled
  detections, reused or new. An inventory with no rows changes nothing; stale
  entries cannot match, because pane IDs are not reused within a lifetime and the
  lifetime is in the key.
- A wall/monotonic disagreement over 100 ms since the previous enrichment, or
  during this one, clears every entry; a server change needs no clear, since no key
  matches and the next inventory drops them.
- One mutex guards every access; overlapping lists only cost captures. Creation,
  inspect and send never touch the cache: a created pane printed within the second,
  so no capture of it could settle.

Why it is sound: the classifier is pure, so an unchanged screen yields the same
detection, reported under the current inventory's `observedAt` because the key
proves the screen unchanged through this tick. `window_activity` is per window, so
a multi-pane window recaptures on any pane's output: it can over-capture, never
under-capture. A spinner rewriting one cell bumps it; resize and copy mode do not,
but size is keyed and capture reads the base screen.

Residuals: `send-keys -R` clears the grid and homes the cursor without bumping
activity, so with no later output the cached detection survives; a resize away and
back while the program does not redraw lets tmux reflow the grid under an equal
key; two opposite clock steps between consecutive enrichments cancel; a quiet pane
becomes reusable only once a capture starts at least 1.1 s after its last output.
Adding `cursor_x,cursor_y` to a1's row and the key would close the first; it is not
owner-approved.

## Rejected

- Host single-flight: a list costs two execs, lists from the hub
  lane, a phone and a browser seldom overlap, and joining them would need semantics
  for callers with different deadlines and cancellation. Revisit only if
  measurement with several clients shows overlap dominating.
- Control mode: `refresh-client -B` subscriptions cover only the control client's
  attached session, so 18 sessions need 18 clients or a hidden session linking every
  window; attaching a control client raises `session_attached`, distorting the
  attached-client count and presence; it needs a supervisor, `%begin`/`%end`
  framing, reconnects and new-lifetime handling; `set-hook -B` monitors reach skid
  only through `run-shell` spawns. Saving the remaining ~0.4 execs/s does not pay
  for that machinery.

## Retained loops

- Presence: one `list-clients` every 2 s per attached web terminal (0.5 execs/s); it
  bounds handoff and revocation lag.
- Attach readiness: every 25 ms for at most 5 s, once per attach.
- Recovery checkpoint: every 30 s, three execs (`InspectServer`, the inventory
  batch, the capture batch); a mutation checkpoint is also three.
- WebSocket pings, pressure sampling and process inspection spawn nothing.

## Priority (macbook-capacity d7)

- The `dev.niels.skidbladnir` launchd plist declares
  `<key>ProcessType</key><string>Interactive</string>`. Without it launchd applies
  standard-job limits (`launchctl print` reports `spawn type = daemon (3)`), and
  serve, the tmux server it starts and every pane shell and agent run at scheduling
  priority 20 instead of 31; a probe measured 20 for an absent `ProcessType` and for
  `Standard`, 31 for `Interactive`. The notifications job, which starts its app
  through `open`, is unchanged.
- After bootstrap the installer reads the new serve PID and requires its priority
  (`ps -o pri`) to be at least 31 within 5 s, else install fails with an action; the
  undocumented `launchctl print` text is not parsed.
- Linux is unchanged: `KillMode=process` keeps tmux and the agents in
  `skidbladnir.service`'s cgroup, so a weight or nice value there would throttle the
  agents too.
- Running processes keep their priority: after an install that changes serve's,
  the tmux server and its panes keep the old one until the server restarts. Stop
  serve, `kill-server`, start serve; [recovery](session-recovery.md) rebuilds the
  workspace under a server serve starts.

## Targets and measurement (macbook-capacity d10)

Acceptance runs at N ≥ 18 agent sessions, with no phone or browser in the
foreground and the hub lane active:

- At most 1.5 tmux execs/s from serve.
- serve, its reaped children and the tmux server together under 5% of one core.
- Inventory rows, metadata and status equal to the per-pane implementation's on the
  same panes.

CPU comes from `proc_pid_rusage(RUSAGE_INFO_V4)` deltas over 30 s, converted with
`mach_timebase_info`: `ri_user_time + ri_system_time` for serve and the tmux
server, `ri_child_user_time + ri_child_system_time` for serve's reaped children.
Execs come from a temporary isolated-socket test whose tmux path is a counting
shim, or on the real host from sampling serve's children (a lower bound) or
`eslogger exec` with operator approval. Wakeups are reported, not gated. Predicted:
the hub lane's 2 execs per 5 s plus recovery's 3 per 30 s, about 0.5 execs/s, plus
0.5/s per attached terminal.
