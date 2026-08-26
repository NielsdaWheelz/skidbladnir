# Skíðblaðnir v0: product and architecture

Status: reviewed implementation target after the 2026-08-25 scope reset.

This document supersedes the audited-orchestration architecture (git history
through `6f2d697`). That design was internally consistent and is preserved in
history, but it built an audited Codex runtime system where the product is a
specialized Android remote for tmux. Its platform evidence — tmux 3.4 grouped
sessions, the pane-steal hazard, the Android terminal harness, profile
isolation, TUI key behavior — remains valid and is carried forward. Its
contract, provenance, and durable lifecycle machinery is retired. Field use on
2026-08-26 added one narrow exception: content-free Codex hooks may project
coarse lifecycle into the current tmux pane; they do not identify or inspect a
conversation.

Specification precedence: this document owns product behavior, architecture,
scope, and acceptance; [`roadmap.md`](roadmap.md) owns delivery order;
[`docs/rules`](rules/index.md) applies where it does not conflict with the
v0 scope. A platform fact that contradicts a premise reopens this document.

## 1. Philosophy

- **tmux is the database and the process supervisor.** Session list, pane
  facts, and user options are the only durable state. Gateway restart means
  "list tmux again", never a recovery protocol.
- **The agent is an opaque terminal program.** Each agent handles its own
  commands, approvals, git, configuration, resume behavior, and subagents.
  Skíðblaðnir never parses, tracks, or reconstructs conversations. A
  three-event Codex adapter projects only `working | idle` into the pane; an
  agent without such an adapter remains honestly `RUNNING`. Claude Work has no
  lifecycle adapter in v0. Any future CLI is another fixed launch command.
- **Android and the laptop are two tmux clients.** One process, one screen,
  one draft; either device attaches and detaches freely.

From a Galaxy S22+, Niels can: see every tmux session on the devbox with an
honest status chip and attention badge; create a session in an explicit
directory running one allowlisted profile command; attach the same stock TUI
the laptop sees; type, paste, and dictate through Gboard; detach without
stopping anything; and kill an exact confirmed session.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; ASCII namespace `skidbladnir`; private, one user, one phone, one devbox |
| Phone | Galaxy S22+ `SM-S906W`, Android 16/API 36 |
| Host | One Hetzner VPS; Linux, mosh, systemd user service, proven tmux 3.4 |
| Network | Tailscale Serve TLS on `:8443` to a loopback gateway; Funnel/public ingress forbidden |
| Auth | One devbox-minted bearer, entered once in the app; every `/v1` request requires it |
| Profiles | Closed `personal \| work \| work2 \| claude-work` command allowlist; callers never supply commands, account homes, or permission flags |
| Runtime | Opaque terminal programs in ordinary tmux sessions; optional provider-owned coarse pane lifecycle, no provenance, thread tracking, payload parsing, or pin enforcement |
| State | tmux sessions/panes/user options only; no SQLite, facts ledger, or command receipts |
| Handoff | Grouped shadow tmux clients; laptop and phone attach concurrently |
| Client | Kotlin/Compose dashboard; vendored pinned xterm.js terminal |
| Host app | Go, tmux/PTY, `/proc` sampling; standard library HTTP |
| Trust | The agent is trusted as the devbox user; no hostile same-UID containment claim |

Profile mapping is one ordered, closed gateway-config table:

| Profile / label | Command | Environment | Arguments | Foreground signatures |
| --- | --- | --- | --- | --- |
| `personal` / `Codex · Personal` | `/home/niels/bin/codex-personal` | `CODEX_HOME=/home/niels/.codex-personal` | `--dangerously-bypass-approvals-and-sandbox` | native executable basename `codex`; or `node` with exact argv[1] `/home/niels/.local/bin/codex` |
| `work` / `Codex · Work` | `/home/niels/bin/codex-work` | `CODEX_HOME=/home/niels/.codex-work` | same | same |
| `work2` / `Codex · Work 2` | `/home/niels/bin/codex-work2` | `CODEX_HOME=/home/niels/.codex-work2` | same | same |
| `claude-work` / `Claude · Work` | `/home/niels/bin/claude-work` | `CLAUDE_CONFIG_DIR=/home/niels/.claude-work` | `--permission-mode auto` | exact argv[0] `/home/niels/.local/bin/claude` |

Adding another launch profile is adding one row — a config change, not a
design event; the app renders whatever rows the gateway declares. The gateway
execs the row's command with its flags in the requested cwd. The gateway does
not gate launch on binary or configuration inspection; the agent sees exactly
what a laptop launch would see. Deployment owns the exact Codex lifecycle-hook
file, while absent/unloaded hooks degrade status to `RUNNING` rather than
blocking launch. There is no router interception: laptop launches are plain
shell commands, and their sessions simply appear in the inventory. A row also
owns exact foreground-process signatures for honest status detection; the
gateway resolves the pane tty's foreground process group through `/proc` and
never treats every `node` process as an agent.
Every populated foreground selector is conjunctive. A signature has either an
exact executable basename or an exact absolute argv[0]; optional argv[1] is
also exact. There are no prefix, version-basename, broad-Node, or fuzzy
fallbacks.

### Product language

Skíðblaðnir (app), Hlíðskjálf (grid motif; literal label `Agents`), The Forge
(new-session sheet; literal `New agent`), and Dvergatal (the append-only dwarf
catalogue at `catalog/characters.json`, ≥100 entries, original deterministic
procedural icon portraits) are retained. Gateway-created sessions are named
`ga-<dwarf>` from a free catalogue key; after the base names are occupied, the
gateway reuses catalogue characters with the smallest free `-2`, `-3`, … name
suffix. The key is stored in a tmux user option and deterministically seeds the
card landmark; v0 owns no raster portrait pack or portrait manifest. The
catalogue therefore does not cap the number of agents. Domain, error, and
destructive-action language stays literal.

### Guarantees (cheap, disaster-preventing)

- Never kill a tmux session other than the exact confirmed target.
- Detach and kill are visibly different actions; kill always confirms.
- Validate cwd; allowlist the four profile commands; nothing else launches.
- Credentials stay on the devbox; the app holds only its bearer.
- App or gateway restart re-lists tmux; it never replays input or guesses.

### Non-goals

Thread/conversation identity, transcript-derived semantic state, a generalized
hook runtime or trust-store editor, git/project-root resolution, router
interception, SQLite lifecycle facts, durable command receipts and replay,
adoption, pin-parity launch refusal, upgrade rehearsals, proof-ledger
acceptance matrices, App Server integration, project enrollment, quotas,
scheduling, orchestration, and multi-user anything. See §8 for what would
ever bring the retired machinery back.

## 3. Platform evidence carried forward

Recorded on exact versions (tmux 3.4, codex-cli 0.149.1, physical SM-S906W)
and still binding on v0:

- A stock TUI uses the normal terminal buffer and is shareable by tmux
  clients. Grouped sessions share panes/processes while clients keep
  independent current-window context; `active-pane` and `ignore-size` are
  required for phone attachment.
- **Pane-steal hazard:** every pane-level targeting form (`switch-client` with
  a pane target, pane-targeted attach, `select-pane`) mutates the window's
  shared active pane and drags an unflagged laptop client with it. Targeting
  must be session/window-level; a client's own active pane is selected only by
  a key sequence written into that client's PTY.
- Killing the last grouped session kills the shared panes: Detach must run a
  last-link guard and never destroys the source session.
- Stock TUI input: raw Ctrl-J (`0x0a`) is newline-without-submit; raw CR
  (`0x0d`) submits; these bytes survive the tmux 3.4 client path.
- S22+ WebView/xterm.js/Gboard: ANSI, Unicode, IME composition, editable
  dictation, clipboard, automatic DA/DSR/CPR replies, resize, rotation. Field
  acceptance additionally requires rendered color, a stable zero-horizontal-
  scroll viewport, and at least 80 displayed columns.
- SIGKILL of a TUI emits nothing; only tmux/process facts are reliable
  liveness evidence. (This is why v0 trusts tmux, not agent self-reporting,
  for aliveness.)
- The server epoch is unshadowable at the pin: `set-option -s` stores a user
  option in `global_options`, and format lookup checks that table before
  pane/window/session options ([tmux 3.4 `options.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/options.c),
  [tmux 3.4 `format.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/format.c)).
  A lifetime identity also carries the built-in server `pid` and `start_time`,
  so restoring an old user-option value into a new server is insufficient.
- The exact-kill gate is one synchronous client queue: `if-shell -F` inserts
  the chosen branch immediately after itself and `cmdq_next` drains that queue
  before returning to other event work ([tmux 3.4 `cmd-if-shell.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/cmd-if-shell.c),
  [tmux 3.4 `cmd-queue.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/cmd-queue.c)).

The retired thread/identity findings (rollout basenames, fork identity, durable
hook ordering) remain true and recorded in git history; v0 does not consume
them.

## 4. Product behavior

### Dashboard

`GET /v1/sessions` inventory, polled from `tmux list-sessions`/`list-panes` plus
`/proc`, drives a dense card grid:

- One card anchors to the session's current window and that window's active
  pane. Cwd, command, lifecycle, bell, and attention all come from that anchor.
  Attached clients are `session_group_attached` for a grouped session and
  `session_attached` otherwise. Gateway-owned phone shadows carry
  `@skid_internal=phone-shadow` and never appear as cards.

- **Card facts:** session name, an opaque server-lifetime identity token, dwarf
  icon portrait when `@skid_character` is set, the declared label for a known
  `@skid_profile` key or `profile unknown`,
  objective (optional; URL-safe base64 in `@skid_objective_b64`, decoded by the
  gateway), pane cwd and active command when tmux exposes them,
  attached-client count, status chip with its named signal and age, attention
  badge. Invalid or unknown `@skid_*` metadata is absent, never guessed.
- **Status and attention are orthogonal.** Attention is a badge, never a status
  replacement. Every status chip names its signal and age:
  - `WORKING` — an allowlisted foreground agent process has a matching
    process-lifetime-bound `@skid_lifecycle` `working` fact;
  - `RUNNING` — an allowlisted agent is alive but no matching lifecycle fact is
    available; this is deliberately not guessed as working or idle;
  - `IDLE` — the same exact foreground process lifetime has an `idle` fact;
  - `SHELL` — no agent process in the pane (exited to shell);
  - `UNKNOWN` — the session was enumerated but its required anchor or process
    observation failed. Failure of the inventory-wide `list-sessions` command
    is an `InternalError`, not a fabricated cached card.
- Laptop-created sessions appear with whatever facts tmux exposes; absence of
  Skíðblaðnir metadata is displayed, not guessed.
- The age source is named honestly (`lifecycle`, `process`, or `poll failure`).
  tmux input/output activity is never lifecycle evidence. The lifecycle value
  is exactly `v1:<foreground-pid>:<kernel-start-time>:<working|idle>:<epoch>`;
  PID and kernel start time prevent a later Codex process in the same pane from
  inheriting stale state.
- Grid order: attention first, then `WORKING`, `RUNNING`, `IDLE`, `SHELL`, `UNKNOWN`,
  name, tmux id.

### Attention

The `notify` line in each Codex profile names `skid-notify`. Codex invokes it
on turn completion; it reads inherited `$TMUX_PANE`, stores the current Unix
epoch in `@skid_attention`, and rings the bell. Opening the card clears the
flag (bell clears natively on view; the gateway unsets the option on attach).
Claude Work makes no provider-specific lifecycle or attention promise.

Each Codex profile also has one repository-owned `hooks.json` containing only
three synchronous command hooks: `SessionStart(startup|resume|clear)` writes
`IDLE`, `UserPromptSubmit` writes `WORKING` and clears stale attention, and
`Stop` writes `IDLE`. The helper drains but never parses hook input. It resolves
the exact target pane's tty, requires the hook process to share that terminal,
then walks its own `/proc` ancestry through the terminal's session leader. It
accepts exactly one logical Codex runtime: either a single Codex process, or
the pin's direct native-`codex` child plus exact Node launcher. Ancestors beyond
that terminal-session boundary are never inspected. That runtime's outer
process must be the pane tty's foreground process-group leader; a second/nested
runtime is ignored even when it takes foreground control. The option is bound
to the outer process's PID and kernel start time. Codex retains its native
exact-digest hook review: install verifies the file bytes, preserves only
Codex's opaque native `[hooks.state]` data, and rejects conflicting user-level
hook sources or the deprecated `features.codex_hooks` alias; then the user
approves a new digest once with `/hooks`.
Skíðblaðnir does not edit an opaque trust store or bypass Codex review. Missing,
untrusted, or unloaded hooks leave the honest `RUNNING` state.

### Start (The Forge)

`POST /v1/sessions {cwd, profile, optionalName?, objective?}`:

1. Cwd: ≤4,096 UTF-8 bytes, no NUL/C0/C1; optional leading `~`/`~/` expands
   against the service UID home; must be an existing directory. Failure is
   typed and mutates nothing.
2. Profile must be one of the four allowlisted commands.
3. Optional name is 1–64 ASCII letters, digits, underscores, or hyphens,
   beginning with a letter or digit;
   optional objective is 1–240 NFC Unicode scalars without terminal controls.
   Invalid input mutates nothing.
4. One tmux client command queue creates the session (named `optionalName` or
   the smallest free catalogue-derived name), initializes the random
   server-scoped `@skid_server_epoch` if absent, sets `@skid_profile` and
   `@skid_character`, sets encoded `@skid_objective_b64` only when supplied,
   and runs the profile command with its exact row arguments in the cwd. A
   later queue failure
   leaves the newly visible session for inventory/recovery; it never performs
   an unproven cleanup kill. No prompt or objective is sent; the agent's own
   onboarding, permission, and trust flows appear like any laptop launch.

### Attach and handoff

- Opening a card starts no second process and never detaches the laptop.
- The gateway creates an ephemeral session grouped with the target, attaches
  one gateway-owned phone PTY as a client with `active-pane` and
  `ignore-size`, and targets **session/window-level only**. The grouped session
  opens on the target's current window and active pane; later phone navigation
  is ordinary key input through the phone PTY, never a pane-targeted tmux
  command. Never mutate another client's selection.
- With an unflagged laptop client present, phone resize changes only its own
  viewport (`CONSTRAINED`); alone, the phone owns sizing (`OWNER`).
- Both devices share process, screen, draft, and turn.
- A session is an internal phone shadow only when both its reserved random
  `skid-phone-<32 lowercase hex>` name and `@skid_internal=phone-shadow` marker
  match. The gateway protects every shadow owned by a live connection. On
  inventory and failed attachment startup it reconciles only an unprotected,
  unattached shadow whose server lifetime, id, name, marker, and group topology
  still match: a duplicate grouped link is removed; a last link is made an
  ordinary visible session by clearing the internal marker and
  `destroy-unattached`. Attached, protected, ambiguous, or changed sessions are
  never mutated.

### Detach

Detach closes only the phone client and its shadow through a last-link guard;
the source session and its process are never destroyed by Detach. Phone loss,
app backgrounding, and process recreation destroy only the attachment; the
next open attaches fresh with no byte replay.

### Kill

`DELETE /v1/sessions/{id}` requires the client to send the tmux session id, the
displayed name, and the inventory's opaque `identityToken`. The token binds the
session id to the server's random epoch plus built-in PID and start time. One
tmux client command queues the epoch/PID/start-time/id/name predicate, an
ungrouped-or-last-link predicate, and `kill-session`; stale tokens, including
after a server restart and id/name reuse or restoration of an old epoch,
cannot reach the kill branch. Before that queue, the gateway removes only
identity-proven, unattached phone shadows, then revalidates the target. Any
remaining grouped sibling is ambiguous and fails closed without mutating the
selected or sibling session; the user resolves that group in tmux. The app
confirms destructively ("Kill session ga-durinn? This tmux session and its
processes end now.")
and never offers kill and detach in the same gesture. There is no working/idle
gate — the human is looking at the terminal facts; the guarantee is exactness
of target, not semantic safety.

### Pressure

`GET /v1/pressure`: five-second `/proc` sampling of CPU, normalized load, memory,
swap, disk, PSI; WARM/HOT thresholds as previously proven (memory 15%/8%,
disk 15%/5%, load 1.0/2.0, CPU PSI some avg60 20%/50%, memory/I-O PSI full
avg60 1%/5%); escalate immediately and de-escalate by one level after each
continuous 60 seconds below the held level. Any missing required
threshold input makes the overall sample `UNKNOWN` and is named; CPU and swap
remain display-only because no thresholds are specified. Pressure never
blocks Start. The wire returns `current` plus at most 180 chronological
five-second samples from the last 15 minutes; the final history item is
`current`. Known metrics are present, missing metrics are named and never
encoded as null, and held hysteresis retains the reasons for the held level.

## 5. Host architecture

```text
Galaxy S22+                       laptop / mosh
  Compose + xterm.js                    |
     | HTTPS/WSS (Tailscale Serve)      v
     v                            ordinary tmux client
  Go gateway ---- tmux commands ----> tmux server (the database)
     |                                  |
     `-- /proc sampling                 `-- panes running agents / anything
```

- One systemd user service runs the gateway. Bind loopback; a path-scoped
  Tailscale Serve mapping on `:8443` exposes `/v1` only; `/healthz` stays
  loopback. Port `443` remains owned by the devbox's existing Caddy service. Its
  unit uses `KillMode=process` and the operator's normal `0022` umask; it must
  not impose sandbox properties inherited by gateway-started tmux/agent
  descendants. Gateway restart never kills the tmux database or changes the
  stock agent runtime. Installation verifies both the loaded systemd directive
  and the running gateway environment before acceptance. The unit supplies the
  fixed `HOME=/home/niels` and stable interactive base `PATH` (`~/bin`,
  `~/.local/bin`, mise shims, then system paths), never an ephemeral
  Codex-injected path, and drops inherited `TMUX`, `TMUX_PANE`, and
  `TMUX_TMPDIR` so gateway commands cannot follow an invoking client's socket.
  Acceptance also pins `/usr/bin/tmux -V` to `tmux 3.4`, rejects every retired
  hook registration or live hook socket, owns the exact narrow lifecycle hook
  file for each Codex profile, and
  proves Caddy is active on `:443` with no effective reverse-proxy upstream to
  gateway port `7341`. During the one-time cutover, the old helper binary and
  gap directory may remain inert at their original paths only so Codex
  processes loaded before the cut are not disrupted; no source or new profile
  launch can select them, and they are archived after those processes exit.
- A manually launched gateway is an operator/debug mode: it still drops
  `TMUX` and `TMUX_PANE`, but intentionally honors that process's
  `TMUX_TMPDIR` as the chosen default-server root. Production never inherits
  it.
- No SQLite. Session metadata lives in tmux user options (`@skid_profile`,
  `@skid_objective_b64`, `@skid_character`, `@skid_attention`,
  `@skid_lifecycle`, the
  server-scoped `@skid_server_epoch`, and the reserved `@skid_internal` shadow
  marker). Poller state is in-memory and rebuilt on start.
- The API is:

| Method/path | Contract |
| --- | --- |
| `GET /v1/sessions` | Observed-at inventory with card facts and per-card opaque `identityToken` above, plus the ordered profile choices for the Forge |
| `POST /v1/sessions` | `{cwd, profile, optionalName?, objective?}`; typed failures |
| `GET /v1/sessions/{id}/terminal` | WSS upgrade requires the inventory `identityToken` in `Skidbladnir-Session-Identity`; one queue validates the full server lifetime, id, and name before creating any shadow/PTY |
| `DELETE /v1/sessions/{id}` | `{name,identityToken}`; owned stale-shadow reconciliation, then one-queue exact lifetime/last-link kill |
| `GET /v1/pressure` | Current sample plus recent window |

Errors use only `{code,message}` with this exhaustive S1 mapping:

| Code | HTTP | Literal message |
| --- | ---: | --- |
| `Unauthenticated` | 401 | `Authentication required.` |
| `InvalidRequest` | 400 | `The request is not valid.` |
| `RequestTooLarge` | 413 | `The request is too large.` |
| `WorkingDirectoryInvalid` | 422 | `Choose a valid working directory.` |
| `WorkingDirectoryUnavailable` | 422 | `That directory does not exist or cannot be opened.` |
| `ProfileUnknown` | 422 | `Choose an available profile.` |
| `SessionNameInvalid` | 422 | `Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.` |
| `ObjectiveInvalid` | 422 | `Use 1–240 characters without terminal controls.` |
| `SessionNameConflict` | 409 | `A session with that name already exists.` |
| `SessionNotFound` | 404 | `That session no longer exists.` |
| `SessionIdentityMismatch` | 409 | `The session changed. Refresh before killing it.` |
| `SessionGroupedConflict` | 409 | `This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.` |
| `InternalError` | 500 | `Skíðblaðnir could not complete the request.` |

Malformed, oversized, and auth failures are distinguished; expected Start and
Kill failures retain their domain codes. An unavailable `/proc` pressure input
is modeled as `UNKNOWN`; only unmodeled defects become the content-free
`InternalError`.

- WSS: text frames `Hello | Presence | Resize | Detach | Error`; binary
  frames are PTY bytes both ways. One WSS owns one PTY/client/shadow and
  tears all three down on any close, subject to the last-link guard. No byte
  replay or gateway scrollback; slow clients disconnect and reattach fresh.
  Any WSS loss freezes terminal input behind a typed `Reconnect required`.
- Bounds: HTTP body 64 KiB; cwd 4,096 bytes; objective 240 scalars; terminal
  frame 64 KiB; queue 1 MiB; geometry 20–240 × 5–120. Named, not schema-frozen.
- Hand-written DTOs; no generated clients, contract digests, or lock files.

## 6. Android surface

- Compile/target/min SDK 36; one manually installed package.
- Grid, Forge, and terminal per §4. Forge preserves invalid drafts; cwd field
  disables autocorrect/smart punctuation.
- Terminal: the proven harness. Vendored pinned xterm.js in a locked WebView
  (`WebViewAssetLoader`, CSP `default-src 'none'` + bundle, no JS bridge, no
  network/file access; Kotlin owns WSS/auth; bearer never enters WebView).
  The CSP admits xterm's generated style elements and attributes, which are
  required for ANSI rendering, while scripts remain bundle-only and every
  exfiltration-capable resource class remains denied.
  The tmux phone client explicitly advertises its RGB capability; xterm owns a
  deterministic ANSI palette. The renderer preserves at least 80 columns in
  portrait by adapting glyph scale before publishing PTY geometry, rather than
  collapsing the TUI into a narrow responsive layout. Accessory row `Agents |
  Esc | Ctrl-C | Tab | arrows | Home | End | Newline |
  Detach`; `Newline` sends raw `0x0a`, Enter sends `0x0d`. Paste strips ESC
  and C0 except newline/tab before bracketed paste. Gboard owns typing,
  clipboard, and dictation; dictation stays editable and never auto-sends. IME
  composition and non-composition Gboard input stay inside the terminal edge;
  both the page and native WebView enforce zero horizontal viewport movement.
- Rotation, IME resize, and process recreation preserve or cleanly recreate
  the attachment; nothing replays.
- Near-black tonal surfaces, deterministic procedural dwarf icons as landmarks,
  semantic labels on all controls; accessibility beyond labels is best-effort.

## 7. Security

- Tailnet-only ingress; TLS via Tailscale Serve; no Funnel, cookies,
  redirects, or CORS.
- One 256-bit bearer minted by a devbox CLI command and entered once in the
  app; every `/v1` request supplies exactly one Authorization header,
  constant-time comparison; re-minting revokes the old token and closes live
  streams. Tailnet admission belongs to loopback binding plus Tailscale Serve,
  not a caller-supplied identity header.
- The terminal endpoint is shell-equivalent authority: Kotlin supplies the
  inventory token outside the WebView in the non-query
  `Skidbladnir-Session-Identity` header; one tmux queue validates the full
  server lifetime, id, and name before any PTY/shadow mutation, and the stream
  closes on mismatch. Android never supplies raw tmux targets, commands, or
  homes.
- Logs carry names, timings, and typed errors — never terminal bytes,
  objectives, prompts, tokens, or credentials.
- Launched agents share the devbox UID; containment requires a separate UID/VM
  and is explicitly out of scope.

## 8. Upgrade ladder (deliberately not in v0)

- **v0.5 — push:** FCM (or ntfy/UnifiedPush) delivery of the Tier-1 attention
  signal: redacted data-only message, deep link, dedupe bit in a tmux user
  option, and a poller sweep that sends late rather than never. Push-to-glance
  needs nothing more.
- **Retired until push-to-act:** authenticated provenance, exactly-once
  attention, durable receipts, thread identity, and replay return only if
  notifications grow buttons that act unattended (resurrection, approval,
  chaining), or unattended orchestration arrives. That decision reopens this
  document; nothing here scaffolds for it.

## 9. Verification

Right-sized: deterministic unit tests; one real-tmux integration suite that
refuses an invoking tmux client and uses only pinned binaries plus explicit,
test-owned sockets with identity-gated destructive cleanup
(create/list/attach/geometry/last-link/kill-exactness, stale-shadow recovery
without a preceding inventory read, and ordinary-group refusal without any
target/sibling/PID mutation on an isolated `-L` socket); one physical S22+
  pass (pair, grid, Forge with spaces-and-`~` cwd,
  concurrent laptop/phone attach with unchanged laptop view, IME/dictation,
  long right-edge input without horizontal drift, stable 80-column geometry
  across rotation, rendered ANSI/true color, reconnect, detach-vs-kill); one
  status-adapter proof binds the process terminal to the exact target pane,
  bounds observation at that terminal's session leader, binds lifecycle to the
  exact foreground process lifetime, and rejects inherited nested-Codex traffic;
  `scripts/test` keeps `static`, `unit`,
  `integration`, `platform`, `live` with that reduced meaning. The retired
proof-ledger/acceptance matrix does not return. Existing `evidence/live/`
records remain as historical platform evidence.

Routine `scripts/test verify` is static analysis, compile/build, and pure unit
tests only; it never invokes tmux or ADB. The `integration` and `live` gates are
separate and remain `NOT_RUN` without explicit user approval in the current
turn, their exact command-line capability plus environment capability, an
absent invoking `TMUX`/`TMUX_PANE`, and a private test-owned socket. The
`platform` gate likewise requires explicit current-turn approval and its exact
device-mutation capability. A skipped external boundary is never a pass.

Acceptance is one sentence per guarantee in §2 plus: laptop and phone share
one pane/PID/draft with laptop geometry and focus unchanged; a kill ends only
the unambiguously last session named by the freshly confirmed lifetime token,
stale tokens kill nothing, and ambiguous ordinary groups fail closed without
mutation; a detached phone leaves its agent running; attention is independent from
status; an instrumented Codex moves `IDLE -> WORKING -> IDLE`, while an
uninstrumented live agent remains `RUNNING`; every status chip states its signal
and age; restart of app or gateway converges to
`tmux list-sessions` truth.
