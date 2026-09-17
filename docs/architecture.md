# Skíðblaðnir v0: product and architecture

current public release: v0.6.0, [organized desktop browser](desktop-browser.md),
source `2d6184c63d62396f69342200e4229cc902ca140c`, including the narrow-card action
repair. all three hosts and the s22+ run this release; host acceptance, fleet
verification and the physical phone product journey pass. release-bound platform
verification passes all 80 tests with a corrected background-selection assertion
and unchanged release runtime.
[delivery and acceptance](roadmap.md#v060-publication-and-deployment) distinguish
published artifacts from installed versions. historical v0.5.0 platform failures
and missing hands-on acceptance remain attributed to that release; publication
does not establish the new release's device acceptance.

accepted 2026-09-15 target: [spaces](spaces.md), pr 1 of
[spaces, shells, and client composition](spaces-and-shells.md). its contracts
are incorporated below; source is implemented, with verification and open
runtime acceptance recorded in the [roadmap](roadmap.md#spaces--source-implemented-runtime-acceptance-open).
this is an optional session-label and client-collection upgrade, with no new
runtime owner. shell creation and additional terminal composition are separate
prs. this document's release evidence remains attributed to its original
source; it does not prove spaces.

accepted 2026-09-15 pr 2 target: [new terminal here](shells.md). standalone
terminal creation and source-session create/attach extend the existing owners;
source is implemented. [delivery and evidence](roadmap.md#new-terminal-here--source-implemented-runtime-acceptance-open)
are tracked separately from pr 1; current runtime evidence and gaps are recorded
there.

accepted 2026-09-17 pr 3 target: [organized desktop browser](desktop-browser.md).
immediate local selection, spaces/agents/tabs, one current browser state and
fullscreen direct attachment; no per-space history or special source return.
source is implemented; [verification](roadmap.md#desktop-browser--source-implemented-runtime-acceptance-open)
tracks native acceptance. [the delivery split](spaces-and-shells.md) leaves
embedded-terminal feasibility to pr 4, with no accepted production terminal contract.

the accepted 2026-09-12 [agent-control target](agent-control.md) specifies the
scoped upgrade shipped in v0.3.1. its explicit v1 deltas supersede conflicting v0 restrictions
for that target; runtime acceptance is recorded in the [roadmap](roadmap.md).
unrelated terminal/platform rules and historical evidence retain their meaning.
the 2026-09-13 amendment keeps codex terminal-only and claude-work as the sole
claude launch profile; native codex binding is deferred.

The 2026-09-08 [readable terminal sizing](terminal-readable-sizing.md) target
replaces the 80-column/protected-desktop sizing contract with chosen phone text
size and tmux latest-client sizing. Its implementation landed 2026-09-08 and is
published in immutable `v0.2.30`; its runtime acceptance is `NOT_RUN`, and
historical evidence below does not prove this target.

Status: accepted implementation target after the 2026-08-25 scope reset, the
2026-08-26 multi-machine hard cut, the 2026-08-27 public-fleet hard cut, the
2026-08-28 agent-identity projection hard cut, the 2026-08-28 dashboard
refresh-boundary correction, the 2026-08-28 tmux-session rename delta, and the
accepted 2026-08-31 tmux terminal-activity hard cut, and the 2026-08-31
dashboard-return-continuity target, and the 2026-08-31 working-directory
chooser target, plus the accepted 2026-09-01 terminal touch-scroll target, the
accepted 2026-09-04 host-installer/operator hard cut, the accepted 2026-09-04
phone-local terminal selection-copy target, the accepted 2026-09-06 terminal
input-intent arbitration correction, and the accepted 2026-09-07 terminal
tap-to-IME correction. The terminal-activity and
touch-scroll changes are merged; exact `v0.2.27` publication, historical
three-host deployment/doctor evidence, and the complete 60-test release-bound
S22+ platform gate are green. The touch-scroll final targeted mutation rerun,
final-candidate hands-on journey, and live tmux/Claude Code journey were
explicitly waived for shipment on 2026-09-04, not passed. The
host-installer/operator cut makes `dev-server` the machine-local installer and
this repository's `scripts/fleet` the sole fixed-fleet workflow owner; the
complete upstream pin names exact `v0.2.30`. Its owner red and hermetic green
are recorded separately. Historical convergence/doctor evidence does not prove
the new ownership boundary. Cross-repository host-pin agreement, all three host
applies, and live one-host outage/recovery are green with exact tmux lifetime
sets preserved. Reboot, second-phone, Linux isolated tmux, broader S22+
hands-on, and provider-live acceptance remain `NOT_RUN` for this cut. The
physical product gate failed and remains unclaimed because installed exact
`v0.2.29` cannot re-enter its byte-distinct-newer update boundary. The
selection-copy source and its input-intent correction were published as
immutable `v0.2.28`, pinned across both repositories, verified on all three
gateways, and installed on the S22+. Hands-on use then exposed the unfocused
tap-to-IME regression. Its correction has an executed unchanged-runtime S22+
red, a focused `5/5` signed same-version green, routine verification, preserved
pairing, exact `v0.2.28` restoration, a complete `75/75` signed candidate, and
hands-on IME acceptance. Immutable `v0.2.29` is published and upstream-pinned.
Cross-repository deployment and the release-bound S22+ platform gate are green
at `OK (75 tests)`, with the exact public APK restored.
The rejected 2026-08-28 agent-interaction-state candidate and its evidence
prove no active target.

This document supersedes the audited-orchestration architecture (git history
through `6f2d697`). That design was internally consistent and is preserved in
history, but it built an audited Codex runtime system where the product is a
specialized Android remote for tmux. Its platform evidence — tmux 3.4 grouped
sessions, the pane-steal hazard, the Android terminal harness, profile
isolation, TUI key behavior — remains valid and is carried forward. Its
contract, provenance, and durable lifecycle machinery is retired. The
2026-08-28 identity cut retains one narrow pane-local SessionStart registration:
it projects only exact current runtime identity facts and never parses content,
tracks history, or creates authority. The terminal-activity cut removes
lifecycle, interaction, and attention projection entirely. It reads only
tmux's built-in current-window activity timestamp and makes no claim about who
owns the next move.

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
- **The agent is an opaque terminal program.** Codex and Claude own their
  conversations, approvals, git, configuration, and in-TUI commands.
  Skíðblaðnir never reads prompts, transcripts, results, or provider stores. A
  closed pane-local SessionStart adapter registers a bounded provider session id
  and runtime profile for the exact foreground process lifetime. Process
  observation proves optional identity only. A required `Active | Quiet` card
  fact reports recent tmux activity in the session's current window; it is
  independent of provider identity and never claims agent semantics. Missing
  registration never blocks or changes activity.
- **Android and each laptop are tmux clients.** Every gateway is an independent
  capability over one local tmux server. Android composes paired gateways; an
  attachment still means one process, one screen, and one draft shared with
  that machine's laptop.

From either trusted Android 16 phone, Niels can see every tmux session on the
paired Devbox, MacBook, and Arch host in one collection, with an honest
machine, optional exact agent identity, and recent terminal activity;
create on an explicit machine and directory using terminal or that host's
allowlisted agent profiles; create an independent terminal from a session's
current host/cwd/space; attach the same stock TUI that host's laptop sees; type, paste, and
dictate through Gboard; select rendered terminal text and explicitly copy it to
that phone's Android clipboard; detach without stopping anything; and kill an
exact machine-bound confirmed session. One unavailable machine does not block
or authorize action against the other.

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
| Profiles | Host config permits an empty array or the complete ordered `personal \| work \| work2 \| claude-work` table, with required `Codex \| Claude` provider and one provider-home discriminator for each row. Terminal is a launch choice, not a profile/provider. Callers never supply commands, account homes, or permission flags |
| Runtime and activity | Opaque terminal programs in ordinary tmux sessions; optional process-lifetime-bound pane identity registration plus one required `Active \| Quiet` fact derived only from the current tmux window's built-in activity timestamp; no provider state lookup, lifecycle/attention projection, provenance, history, payload parsing, or pin enforcement |
| State | Each host's tmux sessions/panes/user options are runtime truth; Android persists pairings, one phone-local terminal text-size preference, and one system-managed, task-scoped, content-free Dashboard return capsule; inventory snapshots stay in memory |
| spaces | one optional canonical label per tmux session in session-local `@skid_space_b64`; clients group equal labels across hosts and intersect independent machine/space filters; no space registry or lifecycle |
| terminal creation | standalone or from an exact source session; host-sampled cwd/space, independent tmux session, configured login shell, existing attachment; detailed contract in [shells.md](shells.md) |
| Handoff | direct tmux clients; laptop and phone share session, window/pane navigation, and latest-client sizing |
| Client | normal cli and small terminal ui; Kotlin/Compose phone dashboard with source-pinned xterm.js terminal |
| Host app | Go, tmux/PTY, platform-native process and pressure observation; standard library HTTP |
| Cutover | One GitHub release carries the signed APK and exact host bundles; gateway and APK contracts move in lockstep with no negotiation, range, legacy envelope, reader, migration, fallback, or smaller-fleet branch |
| Trust | Each agent is trusted as its host user; no hostile same-UID containment claim |

Nonempty profile mapping is one ordered, closed, host-local gateway-config table:

| Profile / label | Provider | Hosts | Command | Environment | Arguments | Foreground signatures |
| --- | --- | --- | --- | --- | --- | --- |
| `personal` / `Codex · Personal` | `Codex` | all | `<home>/.local/bin/codex` | `CODEX_HOME=<home>/.codex` | `--yolo` | native executable basename `codex`; or `node` with exact configured argv[1] |
| `work` / `Codex · Work` | `Codex` | all | `<home>/bin/codex-work` | `CODEX_HOME=<home>/.codex-work` | `--yolo` | same |
| `work2` / `Codex · Work 2` | `Codex` | all | `<home>/bin/codex-work2` | `CODEX_HOME=<home>/.codex-work2` | `--yolo` | same |
| `claude-work` / `Claude · Work` | `Claude` | all | `<home>/bin/claude-work` | `CLAUDE_CONFIG_DIR=<home>/.claude-work` | `--dangerously-skip-permissions --plugin-dir <home>/.local/share/skidbladnir/claude-agent-identity` | exact configured Claude argv[0] |

2026-09-17 accepted launch policy: new agent sessions use the explicit provider
permission bypasses above on all three hosts. deployment owns these arguments;
callers supply no permission setting. acceptance requires every declared profile
to retain its bypass flag through validation and launch, with claude's identity
plugin and managed name preserved. existing sessions retain their launch policy;
remaining provider trust/setup dialogs and project instructions still apply.

Adding a launch profile is adding one host-local row — a config change, not a
design event; the app renders exactly the rows each gateway declares. The
gateway execs the row's command with its flags in the requested
cwd. The gateway does not gate launch on binary or configuration inspection;
the agent retains its ordinary provider configuration and terminal. deployment
owns the exact Codex hook files and one local Claude hook plugin, while absent/unloaded
hooks omit registered identity without blocking launch. Plain personal commands
remain upstream commands. Explicit work wrappers select only their fixed
provider home and, for Claude, add the deployment-owned plugin directory; they
never infer from cwd or read or forward hook payloads. Direct raw-provider
launches bypass that plugin and remain honestly unregistered. A row also owns exact
foreground-process signatures for honest
presence detection; the shared observer resolves the pane tty's foreground
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
- Validate cwd; launch only the configured terminal shell or the target host's
  declared closed agent profile set.
- Codex credentials stay on their host; the app holds one encrypted bearer per
  paired gateway and never shares it across machines.
- App or gateway restart re-lists tmux; it never replays input or guesses.

### Non-goals

Provider conversation lookup, uniqueness, addressing, history, agent work or
next-move state, unread-result attention, transcript-derived semantic state, a
generalized hook runtime or trust-store editor, git/project-root
resolution, router-owned provider payload interception, SQLite lifecycle facts,
durable command receipts and replay,
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
app. the cli uses explicitly configured peers, each reached directly.

## 3. Platform evidence carried forward

Recorded on Linux tmux 3.4, Darwin tmux 3.7b, Codex CLI 0.149.1, and a
physical SM-S906W. Those versions identify the evidence run; the behavioral
findings below remain binding, but the tool versions do not. Hosts install the
latest stable release exposed by their managed package channel:

- stock terminal programs are shareable by concurrent tmux clients. direct
  attachment shares current window and active pane. navigation through either
  client is visible to the other; the gateway targets a session, never moves
  a different client's selection as a handoff operation.
- grouped tmux sessions share windows/processes. deleting one group member can
  leave those processes alive through another. skid creates no groups; detach
  closes only its owned client and never issues a session deletion.
- Stock TUI input: raw Ctrl-J (`0x0a`) is newline-without-submit; raw CR
  (`0x0d`) submits; these bytes survive the tmux 3.4 client path.
- S22+ WebView/xterm.js/Gboard: ANSI, Unicode, IME composition, editable
  dictation, clipboard, automatic DA/DSR/CPR replies, resize, rotation. Field
  acceptance additionally requires rendered color, a stable zero-horizontal-
  scroll viewport, chosen readable text size, and fully visible fitted cells.
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

independent `GET /v1/sessions` inventories drive one dense grouped card grid.
machine selection (`All` or one paired machine) intersects a separate space
selection (all spaces, unassigned, or one named space). equal canonical space
labels group across hosts; local session ids, names, identity tokens, profile
keys, and dwarf keys remain machine-scoped. unavailable-machine notices are
outside space filtering. [spaces](spaces.md) owns the exact grouping, editing,
creation, and content-free restoration contracts:

- One card anchors to the session's current window and that window's active
  pane. Cwd, command, runtime registration, and the built-in
  `window_activity` timestamp all come from that anchor.
  attached clients are the selected session's `session_attached` count.

- **Card facts:** machine label, exact local tmux id, tmux name, an opaque
  server-lifetime identity token, required dwarf icon portrait, launch profile
  (`@skid_profile` when present), optional exact foreground agent provider/PID,
  registered runtime profile and provider session id, observable explicit
  Claude name, objective (optional; URL-safe base64 in
  `@skid_objective_b64`, decoded by the gateway), optional space label
  (`@skid_space_b64`), pane cwd and active command when tmux exposes them,
  attached-client count, and required flat `Active | Quiet` activity.
  Missing or invalid character metadata is assigned from Dvergatal and
  persisted during inventory; other invalid or unknown `@skid_*` metadata is
  absent, never guessed.
- **Card presentation:** the operator-owned tmux name is the primary work
  identity. The dwarf display name remains a smaller Big Shoulders signature.
  A colour/motion-only activity facet occupies one fixed top-right position but
  is redundant decoration: an adjacent named activity bay remains the semantic and
  accessible source. The machine label is quiet footer context in
  `All`; a selected-machine filter supplies that visible context once, so its
  cards omit the repeated visual machine label while retaining machine identity
  in accessibility and every routed or destructive action. The quiet footer
  shows the configured runtime profile label for a proven runtime profile,
  `<provider> · profile unknown` for an agent without one, or the launch
  profile/unknown for a pane without an agent. It never substitutes launch
  profile for missing runtime profile. Cwd abbreviation never changes its
  complete spoken value. Provider session id/name and PID stay off the card.
- Character normalization runs under the gateway's one mutation lock. Valid assignments are retained. Missing or
  invalid assignments use least-live-use selection with a stable
  server-epoch/session-id tie-break and one identity-guarded conditional tmux
  write. A concurrent valid writer is accepted after reread; a changed or
  vanished session is never overwritten, and non-convergence fails the
  inventory instead of fabricating a card.
- **Activity is exactly `Active | Quiet`.** The gateway derives it from the
  current window's positive canonical `window_activity` and the host projection
  clock: `Active` through the inclusive ten-second boundary and `Quiet`
  afterward. Tmux output from any pane in that window, window creation, and
  making the window current count exactly because tmux updates that timestamp.
  Other windows, client-input `session_activity`, alert flags, hooks, process
  facts, CPU, terminal parsing, provider state, and Android time do not.
- The visible labels are `ACTIVE` and `QUIET`, spoken as recent or no recent
  tmux activity at the last check. Neither label means agent work, readiness,
  next-move ownership, completion, unread result, liveness, or safety.
- A missing, malformed, zero, overflowing, or future required activity
  timestamp is never mapped to either state. A vanished session reconciles out;
  a still-present session with an invalid required fact fails that machine's
  inventory. Failure of the inventory-wide `list-sessions` command is likewise
  an `InternalError`, not a fabricated cached card.
- Laptop-created sessions use the same current-pane observation and hook
  registration path. Their character is normalized as above; absent hooks,
  unnamed provider sessions, raw launches, and unproven profiles are successful
  omission, never guessed.
- Activity carries no timestamp or age on the wire. The current poll is never
  presented as state age; only top-level inventory freshness carries a clock.
  Android never counts down or locally decays a host-projected value.
- The agent registration is exactly
  `v1:<pid>:<kernel-start-id>:<Codex|Claude>:<profile-key|->:<session-id-b64url>`
  in pane option `@skid_agent_runtime`. Inventory accepts its registered fields
  only when provider, PID, start id, pane, and foreground origin match the same
  observation used for optional identity. Stale, malformed, nested, ambiguous, or
  wrong-provider registrations are ignored and never repaired. Provider ids and
  names are bounded facts, not unique keys, addresses, or authority.
- grid order: named space headings in the shared ascii-folded/exact utf-8 label
  order, then unassigned. within each group use the current agent-control order:
  case-folded/exact machine label, machine handle, case-folded/exact tmux name,
  then local tmux id. no urgency sorting. retained stale rows remain explicitly
  unavailable and non-actionable. each gateway retains local name/id order;
  grouping belongs to clients, never the host inventory envelope.

The Dashboard is one retained Android navigation entry. Opening Terminal does
not replace that entry: top `Detach` and Android Back return to its same typed
machine and space filters. those filters restore before inventory verification;
the semantic first-visible session or heading and offset settle before dashboard
interaction. an unchanged list returns to the same item and pixel offset; live
insertion/reorder preserves its key; a removed item clamps its former rendered
index. an empty or unavailable selected machine or space remains
selected. Restoration is immediate, non-animated, and one-shot before cards
become interactive; selecting a different filter cancels pending restoration,
while selecting the active filter is a no-op. terminal access-loss recovery
selects the affected machine, retains space selection, resets viewport to top,
and shows its notice. confirmed creation outside the selected space changes that
space filter before post-create navigation; ordinary membership edits do not.
the schema-2 task capsule stores a space-label fingerprint and typed heading or
session anchor, never raw labels. a missing restored label stays selected as
`previously selected space`; creation then requires an explicit named/unassigned
choice. schema-1 navigation is discarded, with no compatibility reader.
Lifecycle stop never consumes pending restoration; only a modeled non-live
machine outcome may resolve it without an inventory snapshot.
The filter strip need not retain its exact horizontal offset, but it reveals the
selected machine chip before the restored Dashboard is settled.
Filter changes use the one live grid's stable-key clamping; no per-filter
viewport history exists.

Pressure remains machine-local per
[`machine-pressure-rail.md`](machine-pressure-rail.md). `All` omits pressure
rails; an explicit machine filter renders exactly that machine's rail and local
details disclosure. Compact exceptional machine notices remain visible in
`All`, so removing the repeated diagnostic rails does not hide stale,
unreachable, unauthenticated, identity-changed, or failed-pressure state. Each
machine retains independent five-second poll work. A failed inventory poll preserves only
that machine's last in-memory snapshot as literal `STALE`; stale, unreachable,
unauthenticated, or identity-changed machines cannot create, attach, send
terminal input, or kill. Pressure failure never disables action against a
fresh inventory. Polls may overlap across machines but coalesce per
machine/resource; mutations and terminal input are never retried or replayed.
The session service captures one projection clock after collecting and
validating the tmux snapshot. Inventory and create expose that exact value as
`observedAt`; the same host clock and the accepted `window_activity` second
derive every card's activity. The gateway never substitutes a handler clock,
Android never rederives activity, and host clocks are never compared to each
other.
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
its access/connect outcome remains visible. The collection always rests at a
`12dp` top inset; the pull threshold reserves no layout space. Pulling and
checking render one active-only `2dp` Gold progress line inside that gutter,
with determinate progress semantics while pulling and indeterminate checking
semantics after release. The line never moves or obscures collection content
and is wholly absent at rest and in inert scopes. Forge outcome-unknown
recovery copy is target-aware: a visible ready target teaches the pull, a
ready target hidden by another filter first names the filter change,
authentication names whole-fleet reconnect, and a changed or missing identity
names app-data reset and a fresh connect. Review-ready copy remains a past-tense fact,
not another verification command. Under an explicit machine filter, the one
pressure rail is a compact disclosure control: a
machine/aggregate/cause/freshness header, one stable non-wrapping
flat typographic metric row, then the unchanged 16dp categorical history band
with no title. Metric labels are neutral; informational and normal values are
quiet, while only host-evaluated warm/hot values and marks spend Gold/Ember.
CPU and swap are visibly informational, missing supported evidence remains
muted as `NO DATA ?`, and unsupported inventory is never product copy. Android
never derives a pressure state or colour from a raw value. A tap opens one
machine-bound details sheet containing every supported current metric,
full states, reasons, freshness, and `NO DATA`; it reads the accepted pressure
snapshot and performs no request or mutation. Pressure freshness is independent
of inventory freshness. Stale pressure preserves and labels its last snapshot;
missing and unsupported remain distinct in the protocol.

### Activity and optional identity

[`terminal-activity.md`](terminal-activity.md) owns the exact two-state
derivation. Inventory reads the current window's built-in
`#{window_activity}` without enabling a monitor, setting an option, installing
a tmux hook, or clearing anything. The host projects `Active` through the
inclusive ten-second boundary and `Quiet` afterward. The existing five-second
inventory schedule is the only poller. A generic terminal program is the full
integration fixture; no provider-live behavior is needed to establish activity.

Each Codex profile has one deployment-owned `SessionStart` identity hook, while
all Claude profiles use the one deployment-owned local `SessionStart` plugin;
both call the closed `agent-hook` command. A bounded decoder reads only the
documented provider session id and writes `@skid_agent_runtime` after the exact
foreground PID/start/tty/profile validation defined by the agent-identity cut.
Missing, untrusted, unloaded, stale, malformed, nested, or ambiguous hooks omit
optional identity and never change activity. No prompt, stop, lifecycle,
interaction, attention, or result event is accepted.

Codex may retain `skid-notify` only as a terminal-local desktop convenience: it
resolves the inherited exact pane and writes BEL. It stores no option, calls no
gateway, carries no content, and has no privileged product meaning. If tmux
observes that byte it is ordinary window activity, identical to output from any
other program. Claude and raw launches remain fully activity-capable without a
notifier.

### Start (The Forge)

The Forge first requires a machine and then offers terminal and that machine's
declared agent profiles. Terminal remains available with zero profiles.
An explicit machine filter may preselect it; otherwise no
machine is inferred. A fresh machine replaces the primary cwd editor with one
full-height, machine-bound chooser: Home, distinct current tmux cwd values,
one-level-at-a-time Home browsing with local folder filtering, and a secondary
exact-path page. Folder entry and explicit `Use` remain distinct; selection
only fills the Forge draft. Listing is bounded, read-only, on demand, and
non-persistent. It never invokes tmux, a shell, an agent, a crawler, a watcher,
or another gateway. [`working-directory-chooser.md`](working-directory-chooser.md)
owns the exact state, content, symlink, bound, and red/green contracts.

Changing machine closes the chooser, invalidates its requests, clears
cwd/agent-profile choice, retains a terminal choice, and preserves tmux
name/objective and the space draft. submission
names the target and sends
`POST /v1/sessions` with required `kind:"agent"` and `profile`, or
`kind:"terminal"` and no profile; both carry
`{cwd, optionalTmuxName?, objective?, space?}` to only that machine:

1. Cwd: input and normalized absolute path are each 1–4,096 UTF-8 bytes; C0/C1,
   U+2028/U+2029, and bidi controls are rejected. Exact `~`/`~/` expands against
   the service UID home; all other input must be absolute. The normalized path
   must be an existing searchable directory. Failure is typed and mutates
   nothing.
2. Agent launch requires one of the target gateway's declared profiles; terminal
   forbids a profile and uses the host shell policy in [shells.md](shells.md).
3. Optional tmux name is 1–64 ASCII letters, digits, underscores, or hyphens;
   optional objective is 1–240 NFC Unicode scalars without terminal controls.
   optional space is 1–64 unicode-15 nfc scalars with the exact whitespace and
   display-safety rules in [spaces](spaces.md#3-labels-equality-and-ordering).
   invalid input mutates nothing. interactive named-space creation prefills a
   visible editable label; a space supplies no other launch context.
4. One tmux client command queue creates the session (named
   `optionalTmuxName` or the smallest free `skidbladnir-<profile>-<N>` for an agent,
   `skidbladnir-terminal-<N>` for a terminal),
   initializes the random
   server-scoped `@skid_server_epoch` if absent, sets agent-only `@skid_profile` and
   `@skid_character`, sets encoded `@skid_objective_b64` and `@skid_space_b64`
   only when supplied,
   and starts the chosen launch. agent commands retain their exact arguments and
   environment. terminal uses the one-shot current-binary entrypoint specified
   in [shells.md](shells.md#3-host-composition-and-launch): literal directory
   entry followed by exec of the configured login shell, without fallback.
   Managed Claude inserts `--name <tmuxName>` before those arguments; configured
   Claude arguments containing `-n` or `--name` are invalid host config. A later queue failure
   leaves the newly visible session for inventory/recovery; it never performs
   an unproven cleanup kill. No prompt is sent; the opaque agent's own
   remaining permission, trust, and setup flows appear in the terminal under
   the explicit launch policy above.

### new terminal here

[shells.md](shells.md) owns pr 2's exact request, launch, client-completion,
ownership, and h/d/p proof contracts. `POST /v1/sessions/{tmuxId}/shell` accepts
only `{identityToken}`. the host samples current pane cwd and local space,
guards creation by session lifetime, then returns a new independent session.
tui `T` (shift+t) and the android attach-header action create once and attach the returned
reference; source name or agent replacement does not retarget the operation.
one-shot launch failure may follow session creation; no shell-readiness promise,
automatic retry, or persistent creation receipt exists. desktop detach leaves the
created shell selected in the browser; return to the source is ordinary navigation.

### attach and handoff

- opening a card or `skid enter` uses the exact machine/session lifetime. the
  gateway launches one owned tmux client directly attached to that session;
  no shadow, group, active-pane isolation, or second agent is created.
- the identity predicate and `attach-session -E -t <id>` share one tmux queue.
  initial effective options must be `window-size latest`, `destroy-unattached off`,
  and `detach-on-destroy on`. unsupported options emit terminal-only
  `TerminalConfigurationUnsupported` before `Hello`; no source option is changed.
- the first measured resize precedes pty creation. both clients participate in
  latest-client sizing and share window/pane navigation, screen, draft, and turn.
  [readable sizing](terminal-readable-sizing.md) owns phone viewport behavior.
- detach, loss, or gateway shutdown closes only the owned pty/client. the existing
  presence monitor binds that client to the same session lifetime and ignores
  rename. an explicit session switch closes on detection, not atomically; bytes
  may pass before the next observation. no extra watchdog or replay exists.

### spaces

`PUT /v1/sessions/{tmuxId}/space` accepts exactly `{identityToken,space}`.
the required string is a canonical label or empty to clear; omission/null are
invalid. success is bodyless `204`. the gateway's mutation lock and one tmux
queue bind assignment to server epoch/pid/start and session id, independently of
name, pane, or foreground process. rename and agent replacement preserve the
target; replacing the session rejects it. repeated same-value assignment and
clearing absence are valid. concurrent writers use last applied write semantics;
lost completion stays unknown and is never replayed automatically.

membership is only session-local canonical unpadded base64url metadata. invalid
or absent local metadata projects as unassigned without repair; global options
are not inherited. creation publishes membership in its existing queue. no space
id, registry, launch context, lifecycle, tmux group, or new hook exists.

cli/tui/phone expose assignment and clearing. human collections use headings;
json remains peer-oriented with optional `space` per row. machine/space filters
intersect, selection/actions retain session lifetimes, and an emptied selected
space stays selected. filtering never removes source inventory or narrows refresh
by previously observed space membership. android uses one retained dashboard
entry and the existing metadata mutation/read ordering; a digest-only unresolved
space can regain its display label from later inventory. exact contracts, file
ownership, costs, and acceptance are in [spaces.md](spaces.md).

### Rename

The active Terminal's middle identity control opens one literal tmux-name
editor. It sends
`PATCH /v1/sessions/{tmuxId} {tmuxName,newTmuxName,identityToken}` to the pinned
machine. The desired name uses the Forge's existing 1–64-character ASCII grammar;
unchanged input is disabled and is `SessionNameConflict` if submitted directly.

Under the gateway's mutation lock, one tmux command queue verifies server
epoch/PID/start-time, id, and expected current name before `rename-session`
targets only the id. Tmux owns destination uniqueness.
The request body has exactly three case-sensitive, non-duplicate string keys.
destination-failure classification revalidates source identity after observing
the destination. Stale identity and collision mutate nothing.
Success is bodyless `204`; Android never retries, supersedes that machine's
inventory, and requires one later inventory read before replacing the terminal
target. A content-free transient mutation fence survives terminal Detach until
that read; no name or history does. The same id/token keeps the active phone
attachment, process, panes, geometry, character, options, and provider facts;
only the authoritative tmux name and header change. Provider session names are
never synchronized.

### Detach

detach closes only the selected machine's owned pty/client;
the source session and its process are never destroyed by Detach. Phone loss,
app backgrounding, and process recreation destroy only the attachment; the
next open attaches fresh with no byte replay.

### Kill

`DELETE /v1/sessions/{tmuxId}` routes to the selected machine and requires its
pinned machine header, local tmux session id, displayed tmux name, and inventory
`identityToken`. The request field is the hard-cut `tmuxName`; no legacy
`name` reader exists. The token binds the session id to that server's random epoch
plus built-in PID and start time. One
tmux client command queues the epoch/PID/start-time/id/name predicate and
`kill-session`; stale tokens, including after server restart and id/name reuse,
cannot reach deletion. the gateway validates, closes its owned terminal
connections, then revalidates and deletes. group membership is no restriction:
only the selected session is removed; shared windows/processes may survive.
the app confirms `Kill <tmuxName> on <machine>?` and never offers kill and detach in the
same gesture. There is no working/idle
gate — the human is looking at the terminal facts; the guarantee is exactness
of target, not semantic safety.

### desktop and agent controls

the implemented [desktop browser](desktop-browser.md) replaces the preceding
grouped table with spaces, global agents, session tabs and a browser-content area.
it owns pr 3's exact selection, keys, geometry and return rules; no new public api
or runtime owner. fullscreen direct attachment remains; persistent chrome during
attachment belongs to pr 4's investigation.
ordinary commands expose list, info, enter,
read, send, keys, interrupt, stop, kill, start, shell, and space. `list --space LABEL` /
`--unassigned`, `start --space LABEL`, and `space TARGET --set LABEL | --clear`
use the [spaces contract](spaces.md#6-cli-and-shared-fleet-presentation).
default private peer configuration
is `~/.config/skidbladnir/client.json`. exact names select across complete live
inventory; `--machine` resolves collisions/outages and `--ref` preserves exact
identity. cli, tui, and jarvis consume one fleetclient projection. jarvis's nine
noninteractive tools invoke `skid --json`, with prompt text through stdin.
[agent-control ux](agent-control-ux.md) owns schemas, selection, and exit contracts.
interrupt retains the session; stop attempts provider halt then closes it; kill
closes the session alone. halt and closure remain separately observed outcomes.

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
  an exact tmux path, an advisory `testedVersion`, and the four
  closed profile rows. Every row has exactly one `Codex | Claude` provider and
  exactly one absolute provider-home environment value: `CODEX_HOME` for Codex
  or `CLAUDE_CONFIG_DIR` for Claude. Provider-home values are unique within a
  provider. Identical foreground-signature rows cannot be shared across
  providers; a concrete process matching more than one provider is
  unclassified.
  Unknown/null members, relative paths, duplicate keys,
  runtime platform mismatch, or a missing/broken/noncanonical tmux executable
  fail startup. A canonical installed version that differs from `testedVersion`
  remains runnable; `scripts/fleet verify` reports functional fleet health
  without turning advisory tmux-version drift into failure.
  `internal/process` is the single native observer consumed once per pane by
  optional agent identity projection and by the content-free SessionStart hook
  adapter. It never derives activity. `internal/agentruntime` owns provider/profile validation,
  foreground classification, registration encoding/acceptance, and
  provider-specific argv rules. Linux process and
  pressure collection stays behind Linux build constraints; Darwin uses
  `KERN_PROC`, `KERN_PROCARGS2`, `proc_pidinfo`, `proc_pidpath`, processor
  ticks, native memory pressure, `vm.swapusage`, and `statfs`, never parsed
  `ps` output or a Linux fallback.
- Public `dev-server` is the sole machine-local install owner. It pins one immutable
  GitHub release, source SHA, and two host-bundle digests, while this repository
  owns the complete five-asset release pin. It renders the exact Devbox/MacBook/Arch host
  configs, SessionStart identity hook files, the local Claude identity plugin,
  and the BEL-only Codex notify asset; the
  explicit Claude work wrapper loads that plugin without editing user settings. It
  owns user systemd services with lingering on Linux and one RunAtLoad
  LaunchAgent on macOS, and applies only its dedicated
  Tailscale Serve `:8443/v1` mapping. It removes only the retired owned root
  handler and never resets unrelated Serve state. Reinstall preserves credentials and tmux
  lifetimes. Sleep, logout, Tailscale loss, or service absence is ordinary
  machine-local unreachability; Skíðblaðnir does not wake a host.
  Codex and Claude are installed from exact reviewable npm locks; tmux follows
  each platform's native stable package channel. new skid agent sessions use
  the deployment-owned permission bypasses in §2.
- accepted 2026-09-17 operator scope: `scripts/fleet` owns only `verify`, `invite`,
  and `provision-clients`. apply acceptance, lifetime digests, reboot checkpoints,
  and outage/recovery commands are retired. `dev-server` owns installation,
  idempotence, credential/session preservation, service lifecycle, and autostart;
  their verification belongs to the replacement test system. historical results
  remain attributed to their original source.
  `scripts/fleet invite` is independent of deployment and verification: on linux or
  macos it reads the existing private `~/.config/skidbladnir/client.json`, selects
  arch/devbox/macbook, requests fresh invitations directly over their authenticated
  https endpoints, and prints one qr only after all three succeed. no ssh, local
  gateway binary, dev-server checkout, release comparison, pressure check, or
  operator lock is required. a new invite replaces the previous one; rerun after
  failure. pairing does not alter installed phone data until the user scans it.
  invitation stream-bounds every response before aggregation.
  `provision-clients` collects each host's existing handle, private Serve origin,
  and bearer, validates the complete unique fleet before distribution, and
  installs mode-0600 client files on macbook, devbox, arch, and jarvis. each user
  file is replaced atomically; distribution is sequential and may partially
  complete. repair the transport and rerun. it requires no release pin, checkout,
  runtime-generation or pressure check. verification and provisioning run from
  the macbook and reach devbox through the installer-owned
  `~/.ssh/config.d/dev-server` with explicit `ssh -F`.
  only `verify` requires an absolute `SKIDBLADNIR_DEV_SERVER_CHECKOUT`, whose
  release pin must be tracked and byte-exact at `HEAD`. it retains release,
  runtime, service, private Serve, and authenticated pressure checks.
- Host apply atomically initializes and then preserves
  `~/.config/skidbladnir/machine-handle` as a mode-`0600` regular file. The
  handle is 128 random bits encoded as `mh-` plus 32 lowercase hexadecimal
  digits. Gateway startup fails closed on missing, insecure, or malformed
  content and never mints, repairs, or substitutes identity. Intentional
  deletion creates a new machine and requires explicit fleet reset on Android.
- No SQLite. Session metadata lives in tmux user options (`@skid_profile`,
  `@skid_objective_b64`, `@skid_space_b64`, `@skid_character`, `@skid_agent_runtime`, the
  server-scoped `@skid_server_epoch`). Poller state is in-memory and rebuilt on start.
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
| `GET /v1/sessions` | `{machine:{handle,platform},observedAt,profiles,sessions}`; every profile has `key,label,provider`; session fields include `tmuxId`, `tmuxName`, `character`, opaque `identityToken`, local facts, optional `space`, optional `launchProfile`, and optional exact `agent` with current agent-control status/methods |
| `POST /v1/directory-listings` | Strict `{directory}` with a canonical Home token; returns the bound machine, current token, optional parent, ordered immediate directory children, and omission bit; no files, metadata, partial result, cache, or fallback |
| `POST /v1/sessions` | required `kind:"agent"` with `profile`, or `kind:"terminal"` without profile; common `{cwd, optionalTmuxName?, objective?, space?}`. success `201 {observedAt,session}` uses the existing strict session DTO; exact creation errors include `code,message,dispatch` |
| `POST /v1/sessions/{tmuxId}/shell` | exact `{identityToken}`; same creation response/error shape; host-sampled cwd/space and session-lifetime gate; no agent predicate |
| `PUT /v1/sessions/{tmuxId}/space` | exact `{identityToken,space}`; nonempty canonical label assigns, empty clears; session-lifetime predicate without name/agent; bodyless `204` |
| `PATCH /v1/sessions/{tmuxId}` | `{tmuxName,newTmuxName,identityToken}`; one-queue expected-name/lifetime rename, bodyless `204`, then client inventory confirmation |
| `GET /v1/sessions/{tmuxId}/terminal` | WSS upgrade requires the inventory `identityToken` in `Skidbladnir-Session-Identity`; one queue validates the full server lifetime, id, and name before direct pty/client attachment |
| `DELETE /v1/sessions/{tmuxId}` | `{tmuxName,identityToken}`; one-queue exact lifetime/name session deletion |
| `GET /v1/pressure` | `{unsupported,current,history}` with the complete platform capability partition from §4 |

errors use `{code,message}` and the existing optional `dispatch` for operations
that distinguish `not_sent` from `unknown`. the spaces route requires that
distinction; its mapping is in [spaces](spaces.md#5-host-api-and-projection).
agent-control errors retain their own spec. session and v0 mappings:

| Code | HTTP | Literal message |
| --- | ---: | --- |
| `Unauthenticated` | 401 | `Authentication required.` |
| `InvalidRequest` | 400 | `The request is not valid.` |
| `RequestTooLarge` | 413 | `The request is too large.` |
| `WorkingDirectoryInvalid` | 422 | `Choose a valid working directory.` |
| `WorkingDirectoryUnavailable` | 422 | `That directory does not exist or cannot be opened.` |
| `DirectoryListingUnavailable` | 422 | `This directory cannot be browsed. Enter the path instead.` |
| `DirectoryListingTooLarge` | 422 | `This directory has too many folders to show. Enter the path instead.` |
| `ProfileUnknown` | 422 | `Choose an available profile.` |
| `SessionNameInvalid` | 422 | `Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.` |
| `ObjectiveInvalid` | 422 | `Use 1–240 characters without terminal controls.` |
| `SpaceInvalid` | 422 | `use 1–64 nfc characters; only interior ordinary spaces, without display controls.` |
| `SessionNameConflict` | 409 | `A session with that name already exists.` |
| `SessionNotFound` | 404 | `That session no longer exists.` |
| `SessionIdentityMismatch` | 409 | `The session changed. Refresh and try again.` |
| `PairingInviteRejected` | 401 | `This fleet invite is invalid, expired, or already used.` |
| `MachineIdentityMismatch` | 409 | `The machine identity changed. Fleet reset is required.` |
| `InternalError` | 500 | `Skíðblaðnir could not complete the request.` |

Malformed, oversized, auth, and machine-binding failures are distinguished;
expected Start and Kill failures retain their domain codes. A missing required
supported pressure input is modeled as `UNKNOWN`; only unmodeled defects become
the content-free `InternalError`. DTOs are a strict hard cut: unknown keys or
enum values are defects, with no protocol branch or compatibility state.

- WSS: text frames `Hello | Presence | Resize | Detach | Error`; Hello/Presence
  contain only kind and attached-client count. The first client Resize gates
  attachment creation. Binary
  frames are pty bytes both ways. one websocket owns one pty/client and closes
  both on connection loss. terminal-only `TerminalConfigurationUnsupported` names
  the three required tmux options; it is not an http error. No byte
  replay or gateway scrollback; slow clients disconnect and reattach fresh.
  Any WSS loss freezes terminal input behind a typed `Reconnect required`.
- Bounds: HTTP body 64 KiB; cwd 4,096 bytes; one directory listing scans at
  most 4,096 entries, returns at most 256 folders and 32 KiB of path text, and
  encodes to at most 64 KiB; chooser filter 256 Unicode scalars and history 32
  views; objective 240 scalars; space 64 scalars / 256 utf-8 bytes; terminal frame 64 KiB; queue 1 MiB; geometry
  20–1024 × 5–512. Named, not schema-frozen.
- Hand-written DTOs; no generated clients, contract digests, or lock files.
  Optional JSON fields are omitted, never `null`; old `id`, flat `profile`,
  providerless profile rows, `status`, `runtime`, `interaction`, `attention`,
  and compatibility decoders do not exist.

## 6. Android surface

- Compile/target/min SDK 36; one manually installed package distributed as the
  public GitHub Release asset `skidbladnir-android.apk`.
- accepted 2026-09-17 installation contract: `scripts/install-android <apk>`
  validates the package and pinned public signer, requires exactly one connected
  authorized device, runs `adb install -r`, and exits with the installation result.
  same-version replacement is supported. it preserves app data, never uninstalls
  or clears storage, and has no app launch, reconnect, visual confirmation, host
  verification, or acceptance journey. package-manager failure is a command
  failure, never an automatic uninstall or downgrade. sdk build-tools 36.1.0 and
  platform-tools are required; optional `ANDROID_HOME` selects the sdk, defaulting
  to `~/Library/Android/sdk` on macos or `~/Android/Sdk` on linux. adb may also be
  on `PATH`. usb debugging authorization is a one-time device setup prerequisite.
  installation, optional qr pairing, and behavioral verification are separate
  operations. success establishes installation only.
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
- Grid, selected-machine pressure rail/details sheet, filters, Forge, and
  terminal follow §4. One Dashboard entry lives above the Dashboard/Terminal
  destination switch and exclusively owns machine/space selection, the live lazy-grid
  state, and pending saved restoration. Android saved-instance state may retain
  one exact-version schema-2 capsule containing only both filter discriminants,
  the machine handle when selected, a comparison-only space-label fingerprint
  when named, a typed session/heading anchor, rendered-item index, and pixel
  offset. [spaces](spaces.md#10-android-navigation-and-content-free-restoration)
  owns its exact schema and unresolved-label behavior. it is validated
  against the newly accepted fleet, never interpreted as a terminal target, and
  never written to preferences, files, tmux, or a gateway. session fingerprints
  retain the domain-separated sha-256 over machine handle, tmux id, and
  high-entropy inventory token. named filter/heading fingerprints use the separate
  label domain specified in spaces; neither raw token nor raw label enters saved
  state. machine scope
  validation uses store-accepted paired handles, never current reachability or
  inventory freshness. A fresh task or explicit app-data/fleet reset starts
  `All` at top; no compatibility reader or per-filter history exists. The Forge
  preserves invalid drafts. Exact cwd entry
  exists only on the chooser's focused URI-keyboard page with autocorrect and
  smart punctuation disabled; IME Done uses the path and never creates a
  session. Picker state, inventory snapshots, and drafts are process-memory
  only.
- Terminal: the proven harness. Vendored pinned xterm.js in a locked WebView
  (`WebViewAssetLoader`, CSP `default-src 'none'` + bundle, no JS bridge, no
  network/file access; Kotlin owns WSS/auth; bearer never enters WebView).
  The CSP admits xterm's generated style elements and attributes, which are
  required for ANSI rendering, while scripts remain bundle-only and every
  exfiltration-capable resource class remains denied.
  The tmux phone client explicitly advertises its RGB capability; xterm owns a
  deterministic ANSI palette. The renderer fits whole cells at the phone's
  saved nominal text size (`16sp` default, `12..24sp`, step `1sp`), applying
  Android text scaling once. Keyboard/rotation refit the grid without rewriting
  that preference. An insufficient viewport blocks user input with explicit
  recovery controls while preserving output and automatic terminal replies.
  [Readable terminal sizing](terminal-readable-sizing.md) owns the preference,
  exact version-4 packaged-page protocol, initial geometry, content, and proofs.
  The [terminal key deck](terminal-key-deck.md) is one stable aligned `2 x 7`
  input surface: `Esc / - Home ↑ End PgUp` over
  `Tab Ctrl Alt ← ↓ → PgDn`. Top `Detach` always owns phone detach. Android Back
  dismisses active native terminal selection first and otherwise owns phone
  detach.
  Ctrl and Alt are independent visible one-shot modifiers; the page publishes
  their state atomically, consumes both on the next input, and resets both at
  lifecycle boundaries. Proven keys use xterm-compatible Ctrl/Alt encoding;
  IME, dictation-shaped, Unicode, multi-character, and pasted text remain
  literal while consuming modifiers. Deck, typed, composed, and pasted input
  share one page-owned ordered ingress. Equal cells are at least `48dp` square,
  with `4dp` outer padding and `2dp` row/column gaps; both rows share one
  horizontal overflow state below the `356dp` normal-font fit or when large
  text requires it. Gboard Enter sends `0x0d`. Paste strips ESC
  and C0 except newline/tab before bracketed paste. Gboard owns typing,
  clipboard reads/paste UI, and dictation; dictation stays editable and never
  auto-sends. The app owns only the explicit terminal-selection clipboard
  write. IME composition and non-composition Gboard input stay inside the
  terminal edge; both the page and native WebView enforce zero horizontal
  viewport movement.
  One prevented, trusted DOM TouchEvent stream owns terminal tap, scroll, and
  selection on the API-36 client. A primary one-finger vertical drag over the
  xterm screen becomes a normalized xterm line-wheel input; xterm alone routes
  it to exact local scrollback, cursor fallback, or negotiated mouse reporting.
  A completed sub-threshold tap becomes a semantic xterm primary tap; xterm's
  active mouse protocol alone decides whether press/release input exists. Two
  truthful Android accessibility wheel actions enter the same route. Physical DOM wheels and
  semantic line-wheel input share one xterm-owned router. There is no
  transcript, mode detector, application-side router, synthetic wheel-event
  injection, or tmux scroll command. The closed implementation and proof
  boundary is [`terminal-touch-scroll.md`](terminal-touch-scroll.md).
  If IME composition is active when an otherwise eligible touch begins and no
  released selection already owns the interaction, the page latches that
  complete touch stream as composition-owned. It consumes the stream without
  tap, scroll, mouse, cursor, selection, modifier, or viewport effects and does
  not reclassify it when composition ends. Android/WebView alone decides the
  composition outcome through xterm's existing literal input path; the next
  fresh gesture resumes ordinary terminal routing.
  A single-contact long press instead claims phone-local selection even under
  negotiated mouse reporting; hold-drag extends through xterm's own selection
  owner. Release snapshots at most `256 KiB` of well-formed UTF-8 text into one
  transient native floating action mode. Its explicit `Copy` writes that exact
  snapshot once to Android's primary plain-text clipboard and clears it. The
  path emits no terminal input and has no WSS, tmux, provider, transcript,
  browser-clipboard, persistence, or clipboard-read capability. The closed
  implementation and proof boundary is
  [`terminal-selection-copy.md`](terminal-selection-copy.md).
- The terminal header always names machine and session; its middle identity
  block is the literal Rename control and retains separate presence state. At
  most one active phone terminal exists, and its connection owns one exact
  `SessionTarget`;
  reconnect re-reads that machine before opening WSS.
  Identity change closes the active terminal and disables that pairing until
  explicit fleet reset.
  Rotation, IME resize, Activity/process recreation, and app backgrounding
  cleanly recreate or release the attachment; nothing replays. Recreation from
  Terminal returns to the saved Dashboard entry and never restores or
  automatically opens an attachment.
- Near-black tonal surfaces, deterministic procedural dwarf icons as landmarks,
  and semantic labels on all controls. [`design-language.md`](design-language.md)
  is the reviewed future visual target for palette, typography, shape,
  ornament, motion, and terminal theme; roadmap D1–D4 remain unimplemented and
  do not describe the current source. The key deck has stable row-major
  traversal and spoken Ctrl/Alt state; terminal scroll exposes reviewed custom
  accessibility actions. Copy is accessible after a trusted touch selection;
  end-to-end screen-reader selection construction is not claimed.
  Accessibility beyond those reviewed surfaces remains best-effort.

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
  server lifetime, id, and name before direct pty/client attachment, and the stream
  closes on mismatch. Android never supplies raw tmux targets, commands, or
  homes.
- Logs carry names, timings, and typed errors — never terminal bytes, cwd,
  objectives, prompts, provider session ids/names, provider homes, argv,
  transcript paths, origins, bearers, account data, or other credentials.
  The machine handle may appear in protocol diagnostics; it is opaque and
  non-secret.
- Terminal selection and clipboard text, previews, and hashes remain transient
  and never enter logs, analytics, saved state, crash context, or evidence.
  Production never reads the Android clipboard. Explicit manual copies use the
  Android system preview and remote-device rendering hint; those are not
  secrecy or device-locality guarantees.
- YOLO agents share their host UID; containment requires a separate UID/VM and
  is explicitly out of scope.

## 8. Upgrade ladder (deliberately not in v0)

the accepted [spaces target](spaces.md) adds optional session labels and grouped
client views only. [the delivery plan](spaces-and-shells.md) separates shell
creation and additional terminal navigation/composition; neither is part of pr 1.
spaces add no execution ownership or lifecycle resource. source is implemented;
the roadmap records its verification and remaining runtime boundaries.

the accepted [new terminal here target](shells.md) closes pr 2's launch and
create/attach contracts. it adds no persistent runtime owner, provider, or
companion relationship. the accepted [pr 3 desktop browser](desktop-browser.md)
adds keyboard regions and status ordering in the global agent list. its one
current model resumes after fullscreen attachment, without history or special
source return. it changes no host or phone execution owner.
pr 4 investigates terminal embedding separately; shipping that capability requires
an explicit terminal contract and acceptance, not evidence from pr 3.

the [agent-control target](agent-control.md) is the accepted upgrade shipped in v0.3.1
for semantic status, provider reads, and cross-agent interaction. it owns its
scope and acceptance criteria; opaque-agent and activity-only rules describe v0,
not that target. push, unread-result attention, provenance, copied provider
history, durable receipts, and replay remain outside the target.

## 9. Verification

2026-09-17 test retirement supersedes the test plans and required red/green
procedures below and in feature specifications. all repository-owned behavioral
suites and their runners are removed. product contracts and behavioral acceptance
criteria remain; deleting their checks does not establish acceptance or erase
earlier failures. historical results retain their original source attribution.

current verification is `scripts/check verify`: formatting, syntax, lint,
dependency integrity, catalogue/generated-asset checks, and go/android builds.
`scripts/release TAG` builds and verifies signed artifacts once before creating a
draft. `scripts/check-release` verifies existing artifacts with the public signing
certificate; only signing needs private key configuration.
`scripts/check published-release TAG SOURCE_SHA` downloads and verifies the public
release without approval flags or environment tokens. the current checker uses
`--source` to validate the clean exact published source, catalogue, and certificate
asset; it does not run the historical release checker. source/version/platform,
signing, archive, checksum, pin, and hosted-verification checks remain.
release-note formatting is not release identity. host binary reproduction is
an explicit `scripts/check-release --reproduce` audit, not routine verification.
without that audit, release checks do not independently prove source-to-binary
equivalence. dependency pins live in their lockfile; catalogue validation owns
data, not a second implementation of icon rendering. these commands establish
only their named engineering properties. routine verification runs no behavioral
tests and touches no tmux, provider, or device boundary. the removed unit, integration, provider-live,
live, platform, product, second-phone, and full gates have no replacement in this pr.

the next pr defines the test system from present requirements. the
[testing status](rules/testing.md) owns the interim commands and the
[coverage issue](issues/test-system-reset.md) records the gap.

### retired verification plan and retained behavioral criteria

Verification follows an 80/20 boundary shape:

- desktop browser adds [a1–a5 acceptance](desktop-browser.md#7-acceptance-and-delivery):
  focused transition/layout cases, the existing real browser/pty/gateway/isolated-
  tmux journey on linux/darwin, manual fixture inspection, and routine verification.
  no new gate or phone/provider-live matrix. unavailable/unapproved runtime
  boundaries remain `NOT_RUN`; old attachment evidence does not prove pr 3;

- shells adds [h/d/p acceptance](shells.md#6-acceptance-and-redgreenrefactor):
  one real host creation proof on linux/darwin, one desktop create/attach journey,
  and one phone create/attach journey with real platform completion/restoration
  cases. small pure contract tables supplement these boundaries; no mocked
  internal api or historical result establishes them. the roadmap records each
  executed boundary; unavailable or unapproved boundaries remain `NOT_RUN`;

- spaces adds its [a1–a9 acceptance](spaces.md#12-acceptance-and-bounded-proof-plan):
  compact go/android label/transport/navigation proofs, approved linux/darwin
  isolated-tmux membership/lifetime acceptance, and approved real-compose/registry
  plus phone-to-isolated-host editing/filter/return acceptance. runtime boundaries
  require current-turn approval; missing boundaries are `NOT_RUN`. existing
  routine gates are reused and historical release proofs do not satisfy these
  new criteria;

- pure table tests own handle/origin/strict DTO and host-config validation,
  pressure signal and recovery classification, pressure capability partitions,
  Android pressure presentation, fleet-QR parsing, federation
  reduction/routing/sort, admission decisions, provider/profile validation,
  foreground classification, registration acceptance, strict terminal-activity
  parsing/derivation, Android activity presentation/order, and provider argv;
- a gateway service test owns invitation replacement/expiry/bearer-rotation
  invalidation and proves exactly one winner under concurrent redemption,
  without invoking tmux;
- one real-temp-tree workdir proof owns cwd grammar, Home containment,
  one-level ordering, omissions, symlinks, cancellation, and bounds; one normal
  authenticated Gateway `httptest` owns the strict listing transport without
  tmux; one Android JVM fixture matrix owns protocol and picker state; with
  separately approved device capability, one production-owned Compose semantic
  journey owns picker interaction and accessibility without posting Create;
- the same approved isolated-socket integration runs on Linux and Darwin and
  owns real gateway + tmux list/create/activity/agent identity/attach/
  detach/exact kill plus authentication and machine-binding rejection before
  mutation;
- approved live publication owns Devbox and Arch systemd plus Mac LaunchAgent
  install, restart, exact Serve and host-config state, a functional configured
  tmux runtime, local re-list, isolated
  bearers, and identity-preserving reinstall;
- a pre-publication release gate owns public-repository state, exact clean-main
  SHA, exact-SHA hosted verification, unused monotonic tag, signer, APK, two
  host bundles and checksums;
  missing signing or GitHub evidence is `NOT_RUN`;
- a separate post-publication read-only gate downloads the release and owns the
  final non-draft immutable tag target, exact five assets, their contents, and
  this repository's byte-exact five-asset pin; product verification separately
  binds the `dev-server` tag, source, and two host-bundle digests;
- approved S22+ instrumentation owns exact-three encrypted collection
  install/reconnect, atomic failure/quarantine, activity presentation/order, the
  absence of pressure rails in `All`, the selected machine's compact pressure
  rail and local details disclosure, terminal behavior, and visible
  stale-action admission;
- a separately approved API-36 terminal selection-copy component matrix owns
  trusted touch selection with mouse reporting off/on, native contextual Back
  ordering, exact bounded Unicode snapshot transfer, the real Android primary
  clip, composition-first touch arbitration and fresh-gesture recovery,
  lifecycle clearing, and zero terminal/network/tmux/provider traffic;
- one approved physical S22+ product journey owns the real scanner,
  three-host federation/routing, per-machine pressure disclosure,
  process recreation, machine-local outage/recovery, and preserved pairings
  and production tmux lifetimes. Its explicit capability permits only the
  gateway's bounded inventory reconciliation of gateway-owned character
  metadata;
  it proves the machine-local session lifetime set is unchanged. Host
  lifecycle mutation coverage stays in isolated gates.
- one separately approved named second-phone gate installs the same public APK
  and connects with a fresh QR; until the device is named it is `NOT_RUN`.

The terminal/identity proofs additionally cover latest-client shared sizing and
shared window/pane navigation, client-only detach, bounded backpressure, exact
foreground process lifetime, inherited nested-Codex rejection,
Gboard/IME/dictation, stable text size with fully fitted rotation geometry,
true color, the reviewed key-deck inputs and atomic one-shot Ctrl/Alt lifecycle,
and reconnect without replay. The
retired proof-ledger/acceptance matrix does not return. Existing
`evidence/live/` records remain historical platform evidence.

Agent-identity acceptance additionally owns one separately approved
`provider-live` installed-hook sample per provider and launch origin across
Linux and Darwin: Linux managed Codex plus laptop Claude, and Darwin managed
Claude plus laptop Codex. It requires a clean exact released checkout, the
fixed installed binary/config/catalogue with exact ownership and modes, and
the installed version matching the declared tag and source SHA before tmux or
a provider is launched. That sample proves provider/PID from the one foreground
observation and accepts registered profile/id only for the exact current
process lifetime; managed Claude name equals its initial tmux name and Codex
names remain absent. The sample never dispatches provider input: Codex receives
generated opaque stdin only through ephemeral `exec` with
`--dangerously-bypass-hook-trust` while a test-owned, project-local synchronous
`SessionStart` hold blocks the agent loop until the exact private tmux server
dies; Claude runs `-p --no-session-persistence </dev/null>` with a separate
test-owned hold-only plugin while the installed router plugin performs identity
registration. Independently of hook loading, Codex selects a test-only,
unauthenticated provider whose model traffic terminates at a content-free
loopback sentinel. The sentinel never reads bytes and counts accepted
connections; the gate requires zero. CLI-owned provider selection disables
retries, WebSockets, and telemetry, and the launcher scrubs ambient proxies. A
missing Codex hold therefore makes the gate red without exposing input to an
external provider. Claude receives no input and cannot persist a session.
Holds, input, and provider output remain content-free in evidence; input
reaches neither an external provider API nor a provider session store. The
separately approved isolated session integration gate owns process replacement,
tmux-server restart, and tmux rename behavior. No provider API, transcript
parser, background worker, or communication action exists. The sample proves
optional identity only; it proves no terminal-activity behavior.

Routine `scripts/test verify` is static analysis, compile/build, and pure unit
tests only; it never invokes tmux, a provider, or ADB. `integration`,
`provider-live`, `live`, host publication, `release`, `published-release`,
`platform`, and `product` remain `NOT_RUN` without
explicit user approval in the current turn and their exact
command/environment capabilities.
Tmux tests refuse inherited `TMUX`, `TMUX_PANE`, and `TMUX_TMPDIR`, own one
private explicit `-L` or `-S` socket, and clean up only identities they created.
A skipped external boundary is never a pass.

Acceptance additionally requires: Devbox, MacBook, and Arch sessions remain distinct
and route only by machine target; `All` cards, exceptional machine notices,
Forge, terminal, and kill confirmation visibly name their machine; `All`
renders no pressure rail, while a selected machine filter renders exactly that
machine's rail and replaces only the card's repeated visual machine label; the
card remains machine-named to accessibility; one host outage leaves the
other fresh and actionable while only the failed snapshot becomes stale and
non-mutating; origin/handle or bearer failure cannot cross machines; each
Forge uses only local profiles/paths; laptop and phone share one pane/PID/draft
with tmux latest-client geometry and shared navigation; detach leaves work
alive; kill removes only the exact machine-local session lifetime; stale
identities mutate nothing and grouped deletion preserves sibling links; every fresh card exposes exactly one required
`Active | Quiet` value from the current window's built-in activity timestamp;
`ACTIVE` and `QUIET` are distinguishable without color and are spoken only as
recent or no recent tmux activity at the last check; the inclusive ten-second
host threshold, existing five-second poll, current-window/sibling-pane/window-
selection semantics, stale qualification, Quiet-first ordering, reduced-motion
fallback, and required-observation failure behavior match
[`terminal-activity.md`](terminal-activity.md); no provider, hook, process fact,
terminal parser, alert flag, phone clock, or user option derives it; every row
exposes `tmuxId` and `tmuxName`, exact foreground Codex or
Claude exposes provider/PID, valid hooks add only bounded runtime profile and
provider session id, explicit Claude name flags map exactly, and launch profile
never substitutes for an unknown runtime profile; first inventory persists one valid dwarf for every visible ordinary
session and preserves concurrent valid assignment; the terminal key deck exposes only its reviewed terminal inputs;
Ctrl/Alt never alter literal IME, dictation-shaped, Unicode, multi-character,
or paste input or survive a lifecycle boundary; and leaving through the top detach action or Back detaches
only the phone; both pressure capability sets are honest, host statuses drive
every metric mark and exception accent, missing evidence stays visible,
recovery is explicit, and pressure
disclosure adds no network or mutation; app, gateway, and LaunchAgent restart
converge to each local
`tmux list-sessions` truth.

Rename acceptance additionally requires: one fresh exact request changes only
the tmux name; the active terminal stays attached and adopts the reread
same-id/token target; invalid, unchanged, conflicting, stale, missing,
restarted-server, and concurrent-loser requests mutate nothing; an unknown
transport outcome is never replayed and converges through inventory; and no
provider rename, alias, history, second name owner, or content-bearing log is
introduced. [`session-renaming.md`](session-renaming.md) owns the detailed
delivery boundary and red/green proof shape.

Working-directory chooser acceptance additionally requires: Home or a current
machine-local cwd is selectable without a keyboard; a six-level Home path is
reachable by touch with truthful machine, location, Parent, Back, and explicit
Use semantics; exact entry still reaches every valid cwd; listing reveals only
bounded immediate folders and never mutates; create revalidates; stale or late
responses cannot cross machine or chooser lifetime; and no path, folder name,
filter, payload, size, or selection enters logs or evidence. The hard cut leaves
no primary raw cwd editor, duplicate validator, compatibility route, fallback,
durable chooser state, or filesystem machinery. The full contract and owner
proofs are [`working-directory-chooser.md`](working-directory-chooser.md).

Terminal touch-scroll acceptance additionally requires: a trusted one-finger
vertical drag and truthful custom accessibility wheel actions traverse xterm's
single normalized wheel owner; a scrollback-capable buffer attempts exact
phone-local line movement even at its bounds, a buffer with no scrollback
capability emits only xterm's cursor sequence, and negotiated mouse tracking
emits only xterm's mouse report. Gesture arbitration, direction, bounded
amplification, cancellation, selection/focus/IME coexistence,
composition-first whole-stream ownership, and outer viewport containment match
[`terminal-touch-scroll.md`](terminal-touch-scroll.md).
The hard cut leaves no transcript, application-side `scrollLines`, mode branch,
escape encoder, synthetic wheel event, tmux command, fallback, or compatibility
path. The sole dependency delta is one source-pinned, digest-locked,
reproducibly generated xterm patch/API whose physical and semantic callers
share one internal router. Its declared Darwin arm64 audit build uses pinned
Node/npm and an integrity-checked platform esbuild package with install scripts
disabled; it pins the upstream source/lock and post-patch manifest/lock
separately, including the reviewed build-only remediation for the tag's stale
lock metadata and executable development dependency advisories. It audits the
complete executable build lock with no severity or development-dependency
omission. Another build platform requires its own explicit pin and
identical-output proof.

Terminal selection-copy acceptance additionally requires: trusted long-press
and hold-drag select the intended xterm cells with mouse reporting off or on;
the native floating `Copy` action is discoverable and invocable after touch
selection; the phone's real primary plain-text clip then equals the immutable
release snapshot once and selection clears. Back first clears selection and
then retains detach through a selected-only view-resolved overlay-priority
callback; no key or Activity/Compose-specific interception exists. Empty,
oversize, malformed, cancelled, dismissed,
backgrounded, disabled, rotated, unavailable, and disposed paths never write;
selection and copy emit no terminal input, WSS, network, tmux, or provider
traffic. An unfocused below-slop tap emits one content-free intent after xterm's
semantic tap succeeds; native lifecycle/selection authorization then acquires
WebView focus and requests the Android IME through `WindowInsets`. An unfocused
drag emits no such intent. The hard cut leaves one generic source-pinned xterm
patch/artifact, one prevented TouchEvent owner, one native
selection/clipboard/IME-presentation boundary, exact generation-correlated
version-3 packaged messages, no DOM/browser clipboard writer, and no
compatibility or fallback path. The full contract and owner proofs are
[`terminal-selection-copy.md`](terminal-selection-copy.md).

Distribution acceptance additionally requires: the public release has the
five owned immutable assets and one signer; `dev-server` pins and applies the
same version on all three hosts, and `scripts/fleet verify` accepts them without
changing existing credentials or tmux lifetimes; a fresh phone needs only APK install, one-time Tailscale login,
`Connect`, and one fresh five-minute QR; concurrent double redemption has one
winner; Android commits all three encrypted credentials or none; reconnect
changes bearers only for exact installed identities; and no coordinator,
public ingress, credential in source/release/logs/argv, legacy provisioning,
host defaults, compatibility fallback, or partial retry remains.

The Android platform gate uses two explicit trust roots. The post-publication
checkout running `scripts/test` owns policy and a `release-pin.json` that is
tracked and byte-exact at `HEAD`. The required absolute, non-symlink
`SKIDBLADNIR_RELEASE_SOURCE_CHECKOUT` is clean at the release's exact source SHA
and owns metadata and signing behavior. It also owns build inputs/outputs and
test enumeration by default. An explicit `--test-source-checkout` may select a
clean descendant with the same tracked pin and changes confined to documentation,
`scripts/test`, Go integration tests, and Android instrumentation. Runtime,
dependency, build, and signing inputs must remain byte-identical to the release;
full-suite enforcement, pairing comparison, and exact public APK restoration
still apply. Record the corrected test SHA separately from the unchanged runtime
SHA and preserve the original failed result. The later pin commit is never
treated as release source. [public-fleet-distribution.md](public-fleet-distribution.md)
owns this test-correction procedure.

Dashboard acceptance additionally requires: a threshold pull at the top of an
empty, short, stale, reading, or populated dwarf collection verifies only the
current filter's live machine targets; a below-threshold release, a release
while the collection remains away from the top, or a pull while verification
is already active adds no work; a pull
racing an ordinary inventory read requires exactly one later coalesced read;
the shared indicator retains content and ends only after every targeted read
lands or its poller stops; and manual verification performs no pressure,
mutation, or terminal-input operation. The collection's first content begins
`12dp` below its viewport in live and inert scopes; the active progress line is
confined to `y = 0..2dp`, horizontally inset `12dp`, and does not change card,
empty-content, focus, or scroll bounds.

dashboard-return acceptance additionally requires: from any machine/space
intersection, leaving a scrolled collection for terminal and returning through
detach or android back restores both filters and the surviving first-visible
session/heading at the same offset without an intermediate all/top frame;
post-detach verification uses machine scope, never observed space membership;
reorder, deletion, empty, unavailable, background, and same-task saved-state cases follow the
rules in [`dashboard-return-continuity.md`](dashboard-return-continuity.md);
fresh task/reset starts all machines/all spaces at top; process restoration never
retains a terminal target, attachment, bytes, or input; and no inventory snapshot/payload,
raw identity token, raw space label, or terminal content enters saved state.
