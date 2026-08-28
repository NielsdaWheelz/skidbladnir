# Skíðblaðnir v0: product and architecture

Status: accepted implementation target after the 2026-08-25 scope reset, the
2026-08-26 multi-machine hard cut, and the 2026-08-27 public-fleet hard cut.

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
[`design-language.md`](design-language.md) owns visual identity — color,
typography, shape, ornament, motion, and the terminal theme — subordinate to
this document;
[`docs/rules`](rules/index.md) applies where it does not conflict with the
v0 scope. A platform fact that contradicts a premise reopens this document.

## 1. Philosophy

- **tmux is the database and the process supervisor.** Session list, pane
  facts, and user options are the only durable state. Gateway restart means
  "list tmux again", never a recovery protocol.
- **The agent is an opaque terminal program.** Codex handles `/quit`, `/new`,
  `resume`, approvals, git, and its own configuration exactly as it normally
  does. Skíðblaðnir never parses, tracks, or reconstructs its conversations. A
  three-event Codex adapter projects only `working | idle` into the pane; an
  agent without such an adapter remains honestly `RUNNING`. Any future CLI
  (Claude Code, OpenCode, Kimi) is just another fixed launch command and may add
  the same small provider-owned adapter later. Devbox Claude Work has no
  lifecycle or attention adapter in v0 and therefore remains honestly
  `RUNNING` while its exact foreground process is alive.
- **Android and each laptop are tmux clients.** Every gateway is an independent
  capability over one local tmux server. Android composes paired gateways; an
  attachment still means one process, one screen, and one draft shared with
  that machine's laptop.

From either trusted Android 16 phone, Niels can see every tmux session on the
paired Devbox, MacBook, and Arch host in one collection, with an honest
machine, status, and attention;
create on an explicit machine and directory using only that host's allowlisted
profiles; attach the same stock TUI that host's laptop sees; type, paste, and
dictate through Gboard; detach without stopping anything; and kill an exact
machine-bound confirmed session. One unavailable machine does not block or
authorize action against the other.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; ASCII namespace `skidbladnir`; public source/release, two trusted users/phones on one tailnet, three acceptance hosts |
| Phone | Galaxy S22+ `SM-S906W` plus one named second phone before its gate; Android 16/API 36 |
| Hosts | Devbox and Arch: Linux/systemd user service. MacBook: Darwin/LaunchAgent. Exact tmux and command paths come from deployment-owned strict host config |
| Topology | Android talks directly to three independent loopback gateways; there is no coordinator or gateway-to-gateway link |
| Network | One pinned Tailscale Serve TLS `:8443` origin per machine; Funnel/public ingress forbidden |
| Machine identity | One random immutable `mh-` + 32-lowercase-hex installation handle per gateway; label, origin, bearer, and platform are not identity |
| Auth | One independently minted bearer per gateway, shared by the two trusted phones; a five-minute one-use pairing token discloses it once. Ordinary `/v1` requests require the bearer and pinned machine handle |
| Profiles | Every host exposes closed `personal \| work \| work2 \| claude-personal \| claude-work` from deployment-owned config. Callers never supply commands, account homes, or permission flags |
| Runtime | Opaque terminal programs in ordinary tmux sessions; optional provider-owned coarse pane lifecycle, no provenance, thread tracking, payload parsing, or pin enforcement |
| State | Each host's tmux sessions/panes/user options are runtime truth; Android persists only pairings and keeps inventory snapshots in memory |
| Handoff | Grouped shadow tmux clients; laptop and phone attach concurrently |
| Client | Kotlin/Compose multi-machine dashboard; vendored pinned xterm.js terminal |
| Host app | Go, tmux/PTY, platform-native process and pressure observation; standard library HTTP |
| Cutover | One GitHub release carries the signed APK and exact host bundles; gateway and APK contracts move in lockstep with no legacy envelope, reader, migration, fallback, or smaller-fleet branch |
| Trust | Each agent is trusted as its host user; no hostile same-UID containment claim |

Profile mapping is one ordered, closed, host-local gateway-config table:

| Profile / label | Hosts | Command | Environment | Arguments | Foreground signatures |
| --- | --- | --- | --- | --- | --- |
| `personal` / `Codex · Personal` | all | `<home>/bin/codex-personal` | `CODEX_HOME=<home>/.codex-personal` | `--dangerously-bypass-approvals-and-sandbox` | native executable basename `codex`; or `node` with exact configured argv[1] |
| `work` / `Codex · Work` | all | `<home>/bin/codex-work` | `CODEX_HOME=<home>/.codex-work` | same | same |
| `work2` / `Codex · Work 2` | all | `<home>/bin/codex-work2` | `CODEX_HOME=<home>/.codex-work2` | same | same |
| `claude-personal` / `Claude · Personal` | all | `<home>/bin/claude-personal` | `CLAUDE_CONFIG_DIR=<home>/.claude-personal` | `--permission-mode auto` | exact configured Claude argv[0] |
| `claude-work` / `Claude · Work` | all | `<home>/bin/claude-work` | `CLAUDE_CONFIG_DIR=<home>/.claude-work` | `--permission-mode auto` | exact configured Claude argv[0] |

Adding a launch profile is adding one host-local row — a config change, not a
design event; the app renders exactly the rows each gateway declares. The
gateway execs the row's command with its flags in the requested
cwd. The gateway does not gate launch on binary or configuration inspection;
the agent sees exactly what a laptop launch would see. Deployment owns the
exact Codex lifecycle-hook file, while absent/unloaded hooks degrade status to
`RUNNING` rather than blocking launch. There is no router interception:
laptop launches are plain shell commands, and their sessions simply appear in
the inventory. A row also owns exact foreground-process signatures for honest
status detection; the shared observer resolves the pane tty's foreground
process group using Linux `/proc` or native Darwin process facts and never
treats every `node` process as an agent.

### Product language

Skíðblaðnir (app), Hlíðskjálf (grid motif; literal label `Dwarves`), The Forge
(new-session sheet; literal `New dwarf`), and Dvergatal (the append-only dwarf
catalogue at `catalog/characters.json`, ≥100 entries, original deterministic
procedural icon portraits) are retained. Every visible ordinary session owns a
valid Dvergatal key in `@skid_character`, whether created from the laptop or
the gateway. Inventory repairs a missing or invalid key before returning a
card; valid keys survive tmux rename and process replacement for the tmux
session lifetime. The key independently seeds the card landmark and never
defines the operator-owned tmux name. Generated names use the smallest free
`skidbladnir-<profile>-<N>`; Dvergatal does not cap the number of sessions. A
visible session persona is a dwarf in product and navigation language;
`agent` is reserved for the opaque foreground terminal program, while
lifecycle, recovery, error, and destructive-action language stays literal and
names the tmux session when relevant. v0 owns no raster portrait pack or
portrait manifest.

### Guarantees (cheap, disaster-preventing)

- Never kill a tmux session other than the exact confirmed target.
- Detach and kill are visibly different actions; kill always confirms.
- Validate cwd; allowlist only the target host's declared closed profile set;
  nothing else launches.
- Codex credentials stay on their host; the app holds one encrypted bearer per
  paired gateway and never shares it across machines.
- App or gateway restart re-lists tmux; it never replays input or guesses.

### Non-goals

Thread/conversation identity, transcript-derived semantic state, a generalized
hook runtime or trust-store editor, git/project-root resolution, router
interception, SQLite lifecycle facts, durable command receipts and replay,
adoption, pin-parity launch refusal, upgrade rehearsals, proof-ledger
acceptance matrices, App Server integration, project enrollment, quotas,
scheduling, orchestration, and multi-user anything. See §8 for what would
ever bring the retired machinery back.

Automatic discovery, a machine registry or coordinator, gateway proxying,
cross-machine move/broadcast/scheduling/failover/wake, durable inventory,
public ingress, fleet-wide pressure, arbitrary host setup, Tailscale policy
automation, and independent phone revocation are also out of scope. The app
has no add, rename, or remove machine capability. It installs or reconnects
only the exact three-machine fleet from one transient QR; quarantine and
machine-identity replacement still require explicit app-data reset outside the
app. Android federation is the whole runtime control plane.

## 3. Platform evidence carried forward

Recorded on Linux tmux 3.4, Darwin tmux 3.7b, Codex CLI 0.149.1, and a
physical SM-S906W. Those versions identify the evidence run; the behavioral
findings below remain binding, but the tool versions do not. Hosts install the
latest stable release exposed by their managed package channel:

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

Independent `GET /v1/sessions` inventories, polled from each local tmux server
plus the platform process observer, drive one dense card grid. `All` and one
chip per paired machine filter the same collection; local session IDs, names,
identity tokens, profile keys, and dwarf keys remain machine-scoped:

- One card anchors to the session's current window and that window's active
  pane. Cwd, command, lifecycle, bell, and attention all come from that anchor.
  Attached clients are `session_group_attached` for a grouped session and
  `session_attached` otherwise. Gateway-owned phone shadows carry
  `@skid_internal=phone-shadow` and never appear as cards.

- **Card facts:** machine label, tmux name, an opaque server-lifetime identity
  token, required dwarf icon portrait, profile (`@skid_profile` or unknown),
  objective (optional; URL-safe base64 in `@skid_objective_b64`, decoded by the
  gateway), pane cwd and active command when tmux exposes them,
  attached-client count, status with its named signal and age, attention
  badge. Missing or invalid character metadata is assigned from Dvergatal and
  persisted during inventory; other invalid or unknown `@skid_*` metadata is
  absent, never guessed.
- **Card presentation:** the operator-owned tmux name is the primary work
  identity. The dwarf display name remains a smaller Big Shoulders signature.
  A colour-only status facet occupies one fixed top-right position but is
  redundant decoration: an adjacent named status bay remains the semantic and
  accessible status source. The machine label is quiet footer context in
  `All`; a selected-machine filter supplies that visible context once, so its
  cards omit the repeated visual machine label while retaining machine identity
  in accessibility and every routed or destructive action. Profile and cwd are
  quiet context, and cwd abbreviation never changes its complete spoken value.
- Character normalization runs after phone-shadow reconciliation under the
  gateway's one mutation lock. Valid assignments are retained. Missing or
  invalid assignments use least-live-use selection with a stable
  server-epoch/session-id tie-break and one identity-guarded conditional tmux
  write. A concurrent valid writer is accepted after reread; a changed or
  vanished session is never overwritten, and non-convergence fails the
  inventory instead of fabricating a card. Phone shadows are never candidates.
- **Status and attention are orthogonal.** Attention is a badge, never a status
  replacement. Every named status bay names its signal and age:
  - `WORKING` — an allowlisted foreground agent process has a matching
    process-lifetime-bound `@skid_lifecycle` `working` fact;
  - `RUNNING` — an allowlisted agent is alive but no matching lifecycle fact is
    available; this is deliberately not guessed as working or idle;
  - `IDLE` — the same exact foreground process lifetime has an `idle` fact;
  - `SHELL` — no agent process in the pane (exited to shell);
  - `UNKNOWN` — the session was enumerated but its required anchor or process
    observation failed. Failure of the inventory-wide `list-sessions` command
    is an `InternalError`, not a fabricated cached card.
- Laptop-created sessions appear with whatever facts that machine's tmux
  exposes. Their character is normalized as above; absence of other
  Skíðblaðnir metadata is displayed, not guessed.
- The age source is named honestly (`lifecycle`, `process`, or `poll failure`).
  tmux input/output activity is never lifecycle evidence. The lifecycle value
  is exactly `v1:<foreground-pid>:<kernel-start-time>:<working|idle>:<epoch>`;
  PID and kernel start time prevent a later Codex process in the same pane from
  inheriting stale state.
- Grid order: attention first, then `WORKING`, `RUNNING`, `IDLE`, `SHELL`,
  `UNKNOWN`, machine label, tmux name, local tmux id. Android owns this full
  cross-host order; each gateway publishes only local facts.

Each machine has its own pressure rail per
[`machine-pressure-rail.md`](machine-pressure-rail.md), freshness, access state,
inline error, and independent five-second poll work. A failed inventory poll preserves only
that machine's last in-memory snapshot as literal `STALE`; stale, unreachable,
unauthenticated, or identity-changed machines cannot create, attach, send
terminal input, or kill. Pressure failure never disables action against a
fresh inventory. Polls may overlap across machines but coalesce per
machine/resource; mutations and terminal input are never retried or replayed.
Signal age begins at the host's own `observedAt - signalAt` and then advances
with Android monotonic time; host clocks are never compared to each other.
The dashboard header is one compact row carrying the title and machine
summary. The primary `New dwarf` action is the Forge seal, a
bottom-trailing octagonal control over the grid; it is lit when a machine
can create and cold when none can. Automatic five-second reconciliation
remains primary. Standard pull-to-refresh over the dwarf collection is the
sole manual verification shortcut: it snapshots the current machine filter,
requests inventory only, and remains visibly active until a post-request
inventory read has landed for every live target. A pre-request read cannot
satisfy that intent. Fixed chrome does not pull, existing collection content
remains in place, and there is no tap, overflow, contextual-retry, or
custom-accessibility equivalent. The pull owner is active only when the
visible scope has a live poller; otherwise the same collection is inert and
its access/connect outcome remains visible. Forge outcome-unknown
recovery copy is target-aware: a visible ready target teaches the pull, a
ready target hidden by another filter first names the filter change,
authentication names whole-fleet reconnect, and a changed or missing identity
names app-data reset and a fresh connect. Review-ready copy remains a past-tense fact,
not another verification command. Each pressure rail is one compact disclosure
control: machine/aggregate/cause/freshness header, one stable non-wrapping row
of status-coloured metric gems, then the unchanged 16dp categorical history
band with no title. CPU and swap are visibly informational, missing supported
evidence remains as `?`, and unsupported inventory is never product copy. A tap
opens one machine-bound details sheet containing every supported current metric,
full states, reasons, freshness, and `NO DATA`; it reads the accepted pressure
snapshot and performs no request or mutation. Pressure freshness is independent
of inventory freshness. Stale pressure preserves and labels its last snapshot;
missing and unsupported remain distinct in the protocol.

### Attention

The `notify` line in each Codex profile names `skid-notify`. Codex invokes it on turn
completion; it reads inherited `$TMUX_PANE`, stores the current Unix epoch in
`@skid_attention`, and rings the bell. Opening the card clears the flag (bell
clears natively on view; the gateway unsets the option on attach).
Claude Work makes no provider-specific lifecycle or attention promise.

Each Codex profile also has one repository-owned `hooks.json` containing only
three synchronous command hooks: `SessionStart(startup|resume|clear)` writes
`IDLE`, `UserPromptSubmit` writes `WORKING` and clears stale attention, and
`Stop` writes `IDLE`. The helper drains but never parses hook input. It uses
the shared platform process observer to walk its ancestry and accepts exactly
one logical Codex runtime: either a single Codex process, or the managed
launcher's direct native-`codex` child plus exact Node launcher. The hook
ancestry must remain inside the target pane tty's exact
kernel device and terminal session through its leader. That runtime's outer
process must be the pane tty's foreground process-group leader; a second/nested
runtime is ignored even when it takes foreground control. The option is bound
to the outer process's PID and kernel
start time. Codex retains its native hook-file digest review: install
verifies the file bytes and rejects conflicting user-level hook sources, then
the user approves a new digest once with `/hooks`; Skíðblaðnir does not edit an
opaque trust store or bypass Codex review. Missing, untrusted, or unloaded hooks
leave the honest `RUNNING` state.

### Start (The Forge)

The Forge first requires a machine and then renders only that machine's
declared profiles. An explicit machine filter may preselect it; otherwise no
machine is inferred. Changing machine clears cwd/profile and preserves
tmux name/objective. Submission names the target and sends
`POST /v1/sessions {cwd, profile, optionalTmuxName?, objective?}` to only that
machine:

1. Cwd: ≤4,096 UTF-8 bytes, no NUL/C0/C1; optional leading `~`/`~/` expands
   against the service UID home; must be an existing directory. Failure is
   typed and mutates nothing.
2. Profile must be one of the target gateway's declared allowlisted commands.
3. Optional tmux name is 1–64 ASCII letters, digits, underscores, or hyphens;
   optional objective is 1–240 NFC Unicode scalars without terminal controls.
   Invalid input mutates nothing.
4. One tmux client command queue creates the session (named
   `optionalTmuxName` or the smallest free `skidbladnir-<profile>-<N>`),
   initializes the random
   server-scoped `@skid_server_epoch` if absent, sets `@skid_profile` and
   `@skid_character`, sets encoded `@skid_objective_b64` only when supplied,
   and runs the profile command with that row's exact arguments in the cwd. A later queue failure
   leaves the newly visible session for inventory/recovery; it never performs
   an unproven cleanup kill. No prompt is sent; the opaque agent's own
   permission and trust flows appear in the terminal like any laptop launch.

### Attach and handoff

- Opening a card routes by its full `(machineHandle, session)` target, starts
  no second process, and never detaches that machine's laptop.
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

Detach closes only the active target machine's phone client and shadow through
a last-link guard;
the source session and its process are never destroyed by Detach. Phone loss,
app backgrounding, and process recreation destroy only the attachment; the
next open attaches fresh with no byte replay.

### Kill

`DELETE /v1/sessions/{id}` routes to the selected machine and requires its
pinned machine header, local tmux session id, displayed tmux name, and inventory
`identityToken`. The request field is the hard-cut `tmuxName`; no legacy
`name` reader exists. The token binds the session id to that server's random epoch
plus built-in PID and start time. One
tmux client command queues the epoch/PID/start-time/id/name predicate, an
ungrouped-or-last-link predicate, and `kill-session`; stale tokens, including
after a server restart and id/name reuse or restoration of an old epoch,
cannot reach the kill branch. Before that queue, the gateway removes only
identity-proven, unattached phone shadows, then revalidates the target. Any
remaining grouped sibling is ambiguous and fails closed without mutating the
selected or sibling session; the user resolves that group in tmux. The app
confirms `Kill <tmuxName> on <machine>?` and never offers kill and detach in the
same gesture. There is no working/idle
gate — the human is looking at the terminal facts; the guarantee is exactness
of target, not semantic safety.

### Pressure

`GET /v1/pressure` samples every five seconds. Both platforms report CPU,
normalized load, swap, and disk. Linux additionally reports memory available
and CPU/memory/I-O PSI; Darwin reports native current system memory pressure
(`Normal | Warning | Critical`) and declares Linux-only signals unsupported.
Linux's unsupported set is exactly `memoryPressure`; Darwin's is exactly
`memoryAvailablePercent`, `cpuPsiSomeAvg60Percent`,
`memoryPsiFullAvg60Percent`, and `ioPsiFullAvg60Percent`.
`unsupported` is sorted, unique, and constant for a running gateway; `missing`
contains only supported signals that failed. Absent metric keys equal the
union of those sets.

WARM/HOT thresholds remain memory available 15%/8%, disk 15%/5%, normalized
load 1.0/2.0, CPU PSI some avg60 20%/50%, and memory/I-O PSI full avg60 1%/5%
on Linux. Darwin memory warning maps to WARM and critical to HOT. Required
inputs are disk/load plus Linux memory/PSI or Darwin native memory pressure;
a missing required supported signal yields `UNKNOWN`, while unsupported never
does. CPU and swap remain display-only. Escalation is immediate and
de-escalation advances one level after each continuous 60 seconds below the
held level. Pressure never blocks Start. The wire returns `current` plus at
most 180 chronological five-second samples from the last 15 minutes; the final
history item is `current`.

## 5. Host architecture

```text
                    trusted Android phone
                      Compose + xterm.js
                   /          |          \
        HTTPS/WSS :8443       |       HTTPS/WSS :8443
                /             |             \
  Devbox Go gateway   MacBook Go gateway   Arch Go gateway
   systemd/Linux       launchd/Darwin       systemd/Linux
          |                  |                   |
      local tmux          local tmux           local tmux
```

- Gateways never know each other. Each binds numeric loopback, exposes only
  `/v1` through its exact Tailscale Serve `:8443` mapping, keeps `/healthz`
  loopback-only, and observes and mutates only the local default tmux server.
  Gateway restart never kills tmux or changes the stock agent runtime. Every
  gateway entrypoint drops inherited `TMUX`, `TMUX_PANE`, and `TMUX_TMPDIR`.
- `internal/platform` is only the closed `Linux | Darwin` native adapter.
  Deployment supplies one strict JSON host config containing expected platform,
  an exact tmux path, an advisory `testedVersion`, Codex entrypoint, and the five
  closed profile rows. Unknown/null members, relative paths, duplicate keys,
  runtime platform mismatch, or a missing/broken/noncanonical tmux executable
  fail startup. A canonical installed version that differs from `testedVersion`
  remains runnable and is reported as a nonblocking `dev-server doctor` warning.
  `internal/process` is the
  single observer consumed by
  session status and the content-free lifecycle adapter. Linux process and
  pressure collection stays behind Linux build constraints; Darwin uses
  `KERN_PROC`, `KERN_PROCARGS2`, `proc_pidinfo`, `proc_pidpath`, processor
  ticks, native memory pressure, `vm.swapusage`, and `statfs`, never parsed
  `ps` output or a Linux fallback.
- Public `dev-server` is the sole host-deployment owner. It pins one immutable
  GitHub release and asset digests, renders the exact Devbox/MacBook/Arch host
  configs and hook/notify assets, owns user systemd services with lingering on
  Linux and one RunAtLoad LaunchAgent on macOS, and converges only its dedicated
  Tailscale Serve `:8443/v1` mapping. It removes only the retired owned root
  handler and never resets unrelated Serve state. Reinstall preserves credentials and tmux
  lifetimes. Sleep, logout, Tailscale loss, or service absence is ordinary
  machine-local unreachability; Skíðblaðnir does not wake a host.
  Codex, Claude, and tmux are intentionally outside the immutable Skíðblaðnir
  release pin; convergence follows each platform's latest stable channel.
- Convergence atomically initializes and then preserves
  `~/.config/skidbladnir/machine-handle` as a mode-`0600` regular file. The
  handle is 128 random bits encoded as `mh-` plus 32 lowercase hexadecimal
  digits. Gateway startup fails closed on missing, insecure, or malformed
  content and never mints, repairs, or substitutes identity. Intentional
  deletion creates a new machine and requires explicit fleet reset on Android.
- No SQLite. Session metadata lives in tmux user options (`@skid_profile`,
  `@skid_objective_b64`, `@skid_character`, `@skid_attention`,
  `@skid_lifecycle`, the
  server-scoped `@skid_server_epoch`, and the reserved `@skid_internal` shadow
  marker). Poller state is in-memory and rebuilt on start.
- Authentication runs before identity disclosure. Ordinary requests send
  exactly one `Skidbladnir-Machine` header matching the gateway installation
  handle. A missing or wrong handle fails before mutation, WSS upgrade, or tmux
  invocation. There is no headerless inventory exception. Profile and session
  DTOs carry no machine fields: machine
  identity appears only in the top-level envelope, and Android composes each
  session with its machine target client-side, so a gateway cannot mislabel
  local facts as another machine's. The API is:

| Method/path | Contract |
| --- | --- |
| `POST /v1/pairing-invites` | Normal bearer + machine auth, empty body; replaces the in-memory slot and returns one five-minute `pairingInviteToken`, expiry, and machine |
| `POST /v1/pairings` | `Skidbladnir-Invite` token + expected machine, empty body; atomically consumes the slot and returns that machine's current bearer once |
| `GET /v1/sessions` | `{machine:{handle,platform},observedAt,profiles,sessions}`; `platform` is `Linux \| Darwin`, and every session contains required `tmuxName`, required `character`, local card facts, and opaque `identityToken` |
| `POST /v1/sessions` | `{cwd, profile, optionalTmuxName?, objective?}`; typed failures |
| `GET /v1/sessions/{id}/terminal` | WSS upgrade requires the inventory `identityToken` in `Skidbladnir-Session-Identity`; one queue validates the full server lifetime, id, and name before creating any shadow/PTY |
| `DELETE /v1/sessions/{id}` | `{tmuxName,identityToken}`; owned stale-shadow reconciliation, then one-queue exact lifetime/last-link kill |
| `GET /v1/pressure` | `{unsupported,current,history}` with the complete platform capability partition from §4 |

Errors use only `{code,message}` with this exhaustive v0 mapping:

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
| `PairingInviteRejected` | 401 | `This fleet invite is invalid, expired, or already used.` |
| `MachineIdentityMismatch` | 409 | `The machine identity changed. Fleet reset is required.` |
| `SessionGroupedConflict` | 409 | `This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.` |
| `InternalError` | 500 | `Skíðblaðnir could not complete the request.` |

Malformed, oversized, auth, and machine-binding failures are distinguished;
expected Start and Kill failures retain their domain codes. A missing required
supported pressure input is modeled as `UNKNOWN`; only unmodeled defects become
the content-free `InternalError`. DTOs are a strict hard cut: unknown keys or
enum values are defects, with no protocol branch or compatibility state.

- WSS: text frames `Hello | Presence | Resize | Detach | Error`; binary
  frames are PTY bytes both ways. One WSS owns one PTY/client/shadow and
  tears all three down on any close, subject to the last-link guard. No byte
  replay or gateway scrollback; slow clients disconnect and reattach fresh.
  Any WSS loss freezes terminal input behind a typed `Reconnect required`.
- Bounds: HTTP body 64 KiB; cwd 4,096 bytes; objective 240 scalars; terminal
  frame 64 KiB; queue 1 MiB; geometry 20–240 × 5–120. Named, not schema-frozen.
- Hand-written DTOs; no generated clients, contract digests, or lock files.

## 6. Android surface

- Compile/target/min SDK 36; one manually installed package distributed as the
  public GitHub Release asset `skidbladnir-android.apk`.
- Device and release artifacts use one dedicated Skidbladnir signing key held
  outside Git; builds never read either host's ambient Android debug keystore.
  The repository pins its public certificate digest. Device gates use an
  explicit debuggable build variant signed by that identity, validate
  mode-0600 key configuration, validate both candidate APKs and any installed
  package against the pin, and stop before ADB mutation on any mismatch.
  Routine debug builds remain an untrusted compile/test lane and are never
  installed by an acceptance gate. The private identity and password file are
  an operator backup obligation; losing them requires reinstall rather than a
  trust bypass. A release tag also carries Linux-amd64 and Darwin-arm64 host
  bundles, `SHA256SUMS`, and the public signing-certificate digest; signing
  remains local and release publication remains a reviewed draft action.
- `MachineStore` persists the exact three-machine collection in app-private
  preferences; handles, case-insensitive labels, origins, and bearer bytes are
  each unique, enforced at the store read boundary — every member of a
  colliding group is quarantined. Every bearer is
  AES-256-GCM encrypted by Android
  Keystore with a fresh nonce and AAD bound to handle and origin. Origins are
  pinned HTTPS `:8443` endpoints with hostname and no user-info, path, query,
  or fragment. Labels, origins, and handles are immutable in the app; bearer
  repair re-authenticates the same handle. An unreadable entry is an opaque
  quarantine slot:
  its plaintext metadata is never trusted or used as a request destination,
  exposes no in-app destructive recovery, and blocks bearer repair while the
  collection is incomplete. If the authoritative collection index itself is
  unreadable, a separately labeled collection quarantine exposes the same
  fail-closed state. There is no old store reader or migration.
- An empty valid store opens `Connect your fleet`. `Connect` uses Google Code
  Scanner without camera permission and strictly parses one exact
  `skidbladnir.fleet-invite.v1` QR containing ordered Arch, Devbox, and MacBook
  labels, canonical HTTPS origins, immutable handles, and unique invitation
  tokens. Tailscale installation/login stays an explicit external action; the
  app neither embeds nor claims to control the VPN.
- The app redeems all three one-use tokens concurrently, awaits every result,
  and writes only after every returned handle matches. It seals all bearers
  before one synchronous preference commit and exact readback. Failure,
  cancellation, process death, partial success, pre-existing data, or
  quarantine leaves no new readable collection and requires a new QR. There is
  no automatic retry. If a target commit is confirmed but its rollback cannot
  be confirmed, the app synchronously deletes and verifies absence of the
  fleet-only Keystore key before process quarantine, so restart cannot
  resurrect either encrypted snapshot.
- `Reconnect fleet` replaces manual bearer entry. It may rotate bearers in one
  commit only when labels, origins, and handles exactly equal the complete
  readable installed fleet. Quarantine or identity replacement cannot be
  repaired in-app. Ordinary upgrades preserve the collection; app-data loss
  returns to Connect. There is no old store reader, ADB provisioning path, or
  smaller-fleet branch.
- Grid, per-machine pressure rails/details sheets, filters, Forge, and terminal follow §4. The Forge
  preserves invalid drafts; its cwd field disables autocorrect/smart
  punctuation. Inventory snapshots and drafts are process-memory only.
- Terminal: the proven harness. Vendored pinned xterm.js in a locked WebView
  (`WebViewAssetLoader`, CSP `default-src 'none'` + bundle, no JS bridge, no
  network/file access; Kotlin owns WSS/auth; bearer never enters WebView).
  The CSP admits xterm's generated style elements and attributes, which are
  required for ANSI rendering, while scripts remain bundle-only and every
  exfiltration-capable resource class remains denied.
  The tmux phone client explicitly advertises its RGB capability; xterm owns a
  deterministic ANSI palette. The renderer preserves at least 80 columns in
  portrait by adapting glyph scale before publishing PTY geometry, rather than
  collapsing the TUI into a narrow responsive layout. The
  [terminal key deck](terminal-key-deck.md) is one stable scrolling row
  `Esc | Ctrl | Tab | Line break | Left | Up | Down | Right | Home | End` and
  contains terminal input only; top `Detach` and Android Back own phone
  detach. Ctrl is a visible one-shot modifier, clears on the next input or
  lifecycle boundary, and never rewrites IME, dictation, or paste text. Deck,
  typed, composed, and pasted input share one page-owned
  ordered ingress. Targets are at least `48dp` square with at least `8dp`
  separation. `Line break` sends raw `0x0a`; Enter sends `0x0d`. Paste strips ESC
  and C0 except newline/tab before bracketed paste. Gboard owns typing,
  clipboard, and dictation; dictation stays editable and never auto-sends. IME
  composition and non-composition Gboard input stay inside the terminal edge;
  both the page and native WebView enforce zero horizontal viewport movement.
- The terminal header always names machine and session. At most one active
  phone terminal exists, and its connection owns one exact `AgentTarget`;
  reconnect re-reads that machine before opening WSS.
  Identity change closes the active terminal and disables that pairing until
  explicit fleet reset.
  Rotation, IME resize, Activity/process recreation, and app backgrounding
  cleanly recreate or release the attachment; nothing replays.
- Near-black tonal surfaces, deterministic procedural dwarf icons as landmarks,
  and semantic labels on all controls. [`design-language.md`](design-language.md)
  is the reviewed future visual target for palette, typography, shape,
  ornament, motion, and terminal theme; roadmap D1–D4 remain unimplemented and
  do not describe the current source. The key deck has stable traversal and
  spoken Ctrl state; accessibility beyond the reviewed surface remains
  best-effort.

## 7. Security

- Tailnet-only ingress; TLS via Tailscale Serve; no Funnel, cookies,
  redirects, or CORS.
- Each gateway owns an independently minted 256-bit bearer. Every `/v1`
  request supplies exactly one Authorization header and uses constant-time
  comparison; re-minting one host revokes only that token and closes that
  host's live streams on both phones. Tailnet admission belongs to loopback binding plus
  Tailscale Serve, not a caller-supplied identity header.
- Each gateway has at most one in-memory five-minute pairing invitation.
  Creating another replaces it; restart or bearer rotation invalidates it;
  redemption consumes it atomically. Only a domain-separated SHA-256 verifier
  is retained. Invalid, expired, used, wrong-machine, or replaced redemption
  is one non-oracular `PairingInviteRejected`. The fleet QR is transient,
  generated once per phone, passed to `qrencode` over stdin, and never stored or
  logged.
- After authentication, `Skidbladnir-Machine` binds the pinned pairing to the
  reached installation. It is not a credential and never substitutes for the
  bearer. A mismatch discloses no actual handle and cannot reach tmux.
- The terminal endpoint is shell-equivalent authority: Kotlin supplies the
  inventory token outside the WebView in the non-query
  `Skidbladnir-Session-Identity` header; one tmux queue validates the full
  server lifetime, id, and name before any PTY/shadow mutation, and the stream
  closes on mismatch. Android never supplies raw tmux targets, commands, or
  homes.
- Logs carry names, timings, and typed errors — never terminal bytes, cwd,
  objectives, prompts, origins, bearers, account data, or other credentials.
  The machine handle may appear in protocol diagnostics; it is opaque and
  non-secret.
- YOLO agents share their host UID; containment requires a separate UID/VM and
  is explicitly out of scope.

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

Verification follows an 80/20 boundary shape:

- pure table tests own handle/origin/strict DTO and host-config validation,
  pressure signal and recovery classification, pressure capability partitions,
  Android pressure presentation, fleet-QR parsing, federation
  reduction/routing/sort, and admission decisions;
- a gateway service test owns invitation replacement/expiry/bearer-rotation
  invalidation and proves exactly one winner under concurrent redemption,
  without invoking tmux;
- the same approved isolated-socket integration runs on Linux and Darwin and
  owns real gateway + tmux list/create/status/attention/attach/detach/exact
  kill plus authentication and machine-binding rejection before mutation;
- approved live publication owns Devbox and Arch systemd plus Mac LaunchAgent
  install, restart, exact Serve and host-config state, a functional configured
  tmux runtime, local re-list, isolated
  bearers, and identity-preserving reinstall;
- a pre-publication release gate owns public-repository state, exact clean-main
  SHA, exact-SHA hosted verification, unused monotonic tag, signer, APK, two
  host bundles, and checksums; missing signing or GitHub evidence is `NOT_RUN`;
- a separate post-publication read-only gate downloads the release and owns the
  final non-draft immutable tag target, exact five assets, their contents, and
  the byte-exact `dev-server` pin of the tag, source, and all five digests;
- approved S22+ instrumentation owns exact-three encrypted collection
  install/reconnect, atomic failure/quarantine, lifecycle reconciliation, the
  compact pressure rail and local details disclosure, terminal behavior, and
  visible stale-action admission;
- one approved physical S22+ product journey owns the real scanner,
  three-host federation/routing, per-machine pressure disclosure,
  process recreation, machine-local outage/recovery, and preserved pairings
  and production tmux lifetimes. Its explicit capability permits only the
  gateway's bounded inventory reconciliation of gateway-owned character
  metadata and stale phone shadows;
  it proves the machine-local session lifetime set is unchanged. Host
  lifecycle mutation coverage stays in isolated gates.
- one separately approved named second-phone gate installs the same public APK
  and connects with a fresh QR; until the device is named it is `NOT_RUN`.

The terminal/status proofs additionally cover unchanged laptop geometry and
focus, last-link detach, bounded backpressure, exact foreground process
lifetime, inherited nested-Codex rejection, Gboard/IME/dictation, stable
80-column rotation geometry, true color, the reviewed key-deck inputs and
one-shot Ctrl lifecycle, and reconnect without replay. The
retired proof-ledger/acceptance matrix does not return. Existing
`evidence/live/` records remain historical platform evidence.

Routine `scripts/test verify` is static analysis, compile/build, and pure unit
tests only; it never invokes tmux or ADB. `integration`, `live`, host
publication, `release`, `published-release`, `platform`, and `product` remain `NOT_RUN` without
explicit user approval in the current turn and their exact
command/environment capabilities.
Tmux tests refuse inherited `TMUX`, `TMUX_PANE`, and `TMUX_TMPDIR`, own one
private explicit `-L` or `-S` socket, and clean up only identities they created.
A skipped external boundary is never a pass.

Acceptance additionally requires: Devbox, MacBook, and Arch sessions remain distinct
and route only by machine target; `All` cards, pressure/error state, Forge,
terminal, and kill confirmation visibly name their machine, while a selected
machine filter replaces only the card's repeated visual machine label and the
card remains machine-named to accessibility; one host outage leaves the
other fresh and actionable while only the failed snapshot becomes stale and
non-mutating; origin/handle or bearer failure cannot cross machines; each
Forge uses only local profiles/paths; laptop and phone share one pane/PID/draft
with laptop geometry and focus unchanged; detach leaves work alive; kill ends
only the exact unambiguously last machine-local lifetime; stale identities and
ordinary groups mutate nothing; attention is independent from status; exact
hooks move `IDLE -> WORKING -> IDLE` while an uninstrumented live agent remains
`RUNNING`; first inventory persists one valid dwarf for every visible ordinary
session, preserves concurrent valid assignment, and never assigns a phone
shadow; the terminal key deck exposes only its reviewed terminal inputs,
one-shot Ctrl never alters literal IME/dictation/paste input or survives a
lifecycle boundary, and leaving through the top detach action or Back detaches
only the phone; both pressure capability sets are honest, host statuses drive
every gem, missing evidence stays visible, recovery is explicit, and pressure
disclosure adds no network or mutation; app, gateway, and LaunchAgent restart
converge to each local
`tmux list-sessions` truth.

Distribution acceptance additionally requires: the public release has the
five owned immutable assets and one signer; `dev-server` pins and converges the
same version on all three hosts without changing existing credentials or tmux
lifetimes; a fresh phone needs only APK install, one-time Tailscale login,
`Connect`, and one fresh five-minute QR; concurrent double redemption has one
winner; Android commits all three encrypted credentials or none; reconnect
changes bearers only for exact installed identities; and no coordinator,
public ingress, credential in source/release/logs/argv, legacy provisioning,
host defaults, compatibility fallback, or partial retry remains.

Dashboard acceptance additionally requires: a threshold pull at the top of an
empty, short, stale, reading, or populated dwarf collection verifies only the
current filter's live machine targets; a below-threshold release, a release
while the collection remains away from the top, or a pull while verification
is already active adds no work; a pull
racing an ordinary inventory read requires exactly one later coalesced read;
the shared indicator retains content and ends only after every targeted read
lands or its poller stops; and manual verification performs no pressure,
mutation, or terminal-input operation.
