# skíðblaðnir: product and architecture

tmux owns terminal sessions and pane processes. providers own execution and history.
each gateway controls one host; clients compose gateways directly. there is no
application database or coordinator.

2026-09-25 restoration scope: this original tmux product coexists independently
with herdr-mobile. [the deployment handoff](dev-server-handoff.md) owns the
namespace handback, skid-only private provider homes, shell setup, helper pins, and
qualification status. source preparation does not authorize publication,
installation, or modification of the other product's runtime.
herdr and ordinary shells retain their existing providers, account homes,
histories and native integrations. the private herdr-home plan is withdrawn;
hook coexistence is qualified at the actual integration boundary.

this document owns shared mechanisms, invariants, and scope. the accepted
[agent controls](agent-control.md), [client and attachment](agent-control-ux.md),
[spaces](spaces.md), [terminal creation](shells.md), and
[desktop browser](desktop-browser.md) specifications own their detailed contracts.
[design language](design-language.md) owns visual values; [codebase rules](rules/index.md)
own implementation conventions. a platform fact that contradicts a premise
reopens the responsible contract.

[the roadmap](roadmap.md) indexes delivery and open acceptance.
[the codebase map](codebase-map.md) locates implementation owners.
[testing policy](rules/testing.md) supersedes retired test recipes in older
feature plans; those recipes do not recreate removed gates.

## 1. Philosophy

- **tmux is the database and the process supervisor.** Session list, pane
  facts, and user options are the durable session state. Gateway restart means
  "list tmux again", never a recovery protocol.
- **providers own execution and history.** skid observes the exact foreground
  process, samples its status, and offers bounded reads and explicit controls.
  codex is terminal-only; claude may use native status/history/stop through a
  short-lived helper. no history is copied into a skid store, and no helper
  owns the provider's lifetime. identity hooks never publish status or content.
- **Android and each laptop are tmux clients.** Every gateway is an independent
  capability over one local tmux server. Android composes paired gateways; an
  attachment still means one process, one screen, and one draft shared with
  that machine's laptop.

from the android phone, the user can see every tmux session on the paired
devbox, macbook, and arch host in one collection, with machine identity,
optional exact foreground-agent identity and sampled status;
create on an explicit machine and directory using terminal or that host's
allowlisted agent profiles; create an independent terminal from a session's
current host/cwd/space; attach the same stock TUI that host's laptop sees; type, paste, and
dictate through Gboard; select rendered terminal text and explicitly copy it to
that phone's Android clipboard; detach without stopping anything; and kill an
exact machine-bound confirmed session. phone interrupt/stop and desktop
read/text/key controls follow [agent control](agent-control.md). one unavailable
machine does not block or authorize action against another.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; ASCII namespace `skidbladnir`; public source/release, one user on one tailnet, three hosts |
| Phone | Android 16/API 36; historical device evidence uses Galaxy S22+ `SM-S906W` |
| Hosts | Devbox and Arch: Linux/systemd user service. MacBook: Darwin/LaunchAgent. Exact tmux and command paths come from deployment-owned strict host config |
| Topology | Android talks directly to three independent loopback gateways; there is no coordinator or gateway-to-gateway link |
| Network | One pinned Tailscale Serve TLS `:8443` origin per machine; Funnel/public ingress forbidden |
| Machine identity | One random immutable `mh-` + 32-lowercase-hex installation handle per gateway; label, origin, bearer, and platform are not identity |
| Auth | One independently minted bearer per gateway, shared by the trusted clients; a five-minute one-use pairing token discloses it once. Ordinary `/v1` requests require the bearer and pinned machine handle |
| Profiles | Host config permits an empty array or the complete ordered `personal \| work \| work2 \| claude-work` table, with required `Codex \| Claude` provider and one provider-home discriminator for each row. Terminal is a launch choice, not a profile/provider. Callers never supply commands, account homes, or permission flags |
| agent control | ordinary tmux sessions; exact foreground identity, sampled native/terminal status, bounded reads and explicit controls under [agent control](agent-control.md); identity-only hooks, no lifecycle database or execution supervisor |
| State | Each host's tmux sessions/panes/user options are runtime truth; Android persists pairings, one phone-local terminal text-size preference, and one system-managed, task-scoped, content-free Dashboard return capsule; inventory snapshots stay in memory |
| spaces | one optional canonical label per tmux session in session-local `@skid_space_b64`; clients group equal labels across hosts and intersect independent machine/space filters; no space registry or lifecycle |
| terminal creation | standalone or from an exact source session; host-sampled cwd/space, independent tmux session, configured login shell, existing attachment; detailed contract in [shells.md](shells.md) |
| Handoff | direct tmux clients; laptop and phone share session, window/pane navigation, and latest-client sizing |
| Client | normal cli and small terminal ui; Kotlin/Compose phone dashboard with source-pinned xterm.js terminal |
| Host app | Go, tmux/PTY, platform-native process and pressure observation; standard library HTTP |
| Cutover | One GitHub release carries the signed APK and exact host bundles; gateway and APK contracts move in lockstep with no negotiation, range, legacy envelope, reader, migration, compatibility fallback, or smaller-fleet branch |
| Trust | Each agent is trusted as its host user; no hostile same-UID containment claim |

Nonempty profile mapping is one ordered, closed, host-local gateway-config table:

| Profile / label | Provider | Hosts | Command | Environment | Arguments | Foreground signatures |
| --- | --- | --- | --- | --- | --- | --- |
| `personal` / `Codex · Personal` | `Codex` | all | absolute native codex | `CODEX_HOME=<home>/.local/share/skidbladnir/providers/codex-personal` | `--yolo` | native executable basename `codex` |
| `work` / `Codex · Work` | `Codex` | all | same native codex | `CODEX_HOME=<home>/.local/share/skidbladnir/providers/codex-work` | `--yolo` | same |
| `work2` / `Codex · Work 2` | `Codex` | all | same native codex | `CODEX_HOME=<home>/.local/share/skidbladnir/providers/codex-work2` | `--yolo` | same |
| `claude-work` / `Claude · Work` | `Claude` | all | absolute native claude | `CLAUDE_CONFIG_DIR=<home>/.local/share/skidbladnir/providers/claude-work` | `--dangerously-skip-permissions --plugin-dir <home>/.local/share/skidbladnir/claude-agent-identity` | exact configured Claude argv[0] |

2026-09-17 accepted launch policy: new agent sessions use the explicit provider
permission bypasses above on all three hosts. deployment owns these arguments;
callers supply no permission setting. acceptance requires every declared profile
to retain its bypass flag through validation and launch, with claude's identity
plugin and managed name preserved. existing sessions retain their launch policy;
remaining provider trust/setup dialogs and project instructions still apply.

the app renders the closed profile table declared by each gateway. changing
that table changes the contract; callers cannot invent a profile. the gateway
launches its one-shot `agent-exec` boundary in the new pane, clears inherited
`HERDR_*` and provider homes, then execs the selected row's native command with
its exact home/flags in the requested cwd. existing tmux server/session
environments remain untouched. the
gateway does not gate launch on binary or configuration inspection;
the agent retains its ordinary provider configuration and terminal. deployment
owns the exact Codex hook files and one local Claude hook plugin, while absent/unloaded
hooks omit registered identity without blocking launch. new skid shell terminals
use private personal homes; deployment-owned bash/zsh startup functions select
those homes for bare commands and exact private homes for account commands,
only in marked skid terminals. ordinary shells and herdr panes retain their
existing commands and account state; no global provider rerouting is installed.
manual claude-personal has its own home but no forge row. these functions call
native providers through one closed product launcher, load the identity plugin
for claude, and never infer from cwd or read hook payloads. shared account
wrappers are not used. direct raw-provider
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
`agent` is reserved for the foreground provider program, while
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

provider conversation search/registry, unread-result attention, arbitrary
transcript-derived semantic state, chat ui, copied provider history, a
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
  pane. cwd, command, foreground process, and runtime registration come
  from that anchor.
  attached clients are the selected session's `session_attached` count.

- **Card facts:** machine label, exact local tmux id, tmux name, an opaque
  server-lifetime identity token, required dwarf icon portrait, launch profile
  (`@skid_profile` when present), optional exact foreground agent provider/PID,
  pane id/start identity, sampled status and methods; for a proven claude
  registration, runtime profile and provider session id; independently observed
  explicit claude name; objective (optional; URL-safe base64 in
  `@skid_objective_b64`, decoded by the gateway), optional space label
  (`@skid_space_b64`), pane cwd and active command when tmux exposes them,
  attached-client count. `agent` is optional; when present its provider, pid,
  pane/start identity, status and methods are required. no flat activity remains.
  Missing or invalid character metadata is assigned from Dvergatal and
  persisted during inventory; other invalid or unknown `@skid_*` metadata is
  absent, never guessed.
- **Card presentation:** the operator-owned tmux name is the primary work
  identity. The dwarf display name remains a smaller Big Shoulders signature.
  a fixed status facet is redundant decoration; the adjacent named status
  bay remains the semantic and accessible source. inferred terminal observations
  are labelled as such; a pane without an agent is `TERMINAL`. the machine label
  is quiet footer context in
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
- **agent status is sampled, never authority.** states are
  `working | blocked | idle | done | failed | stopped | unknown`, with source
  `native | terminal | unavailable`. claude uses a valid native observation when
  available; otherwise explicit provider chrome may supply an inferred terminal
  observation. codex uses terminal chrome only. unfamiliar chrome is unknown.
  status is distinct from inventory freshness, unread attention and liveness;
  every command revalidates its full target.
- a vanished session reconciles out. failed required tmux
  snapshot collection fails that machine's request; optional agent-observation
  failure omits identity or reports unavailable status, never fabricated facts.
- Laptop-created sessions use the same current-pane observation and hook
  registration path. Their character is normalized as above; absent hooks,
  unnamed provider sessions, raw launches, and unproven profiles are successful
  omission, never guessed.
- status carries no transition time or age. only top-level inventory freshness
  carries a clock; android does not locally decay the sampled status.
- The agent registration is exactly
  `v1:<pid>:<kernel-start-id>:<Codex|Claude>:<profile-key|->:<session-id-b64url>`
  in pane option `@skid_agent_runtime`. Inventory accepts its registered fields
  only when provider, PID, start id, pane, and foreground origin match the same
  observation used for optional identity. Stale, malformed, nested, ambiguous, or
  wrong-provider registrations are ignored and never repaired. Provider ids and
  names are bounded facts, not unique keys, addresses, or authority. only claude
  registration supplies projected profile/provider-session id; codex's old
  hook registration is not used for native binding or projected identity.
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
`observedAt`, before optional process enrichment. agent-control enrichment
runs after releasing the session lock
and does not replace that timestamp. the result is a sampled observation, not
an atomic provider snapshot. host clocks are never compared to each other.
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

### agent observation and identity hooks

[agent control](agent-control.md) owns status and bounded reads/controls.
`agentcontrol` enriches the collected tmux inventory outside the session lock,
using terminal capture and one claude native inspection per selected profile.
enrichment is bounded to two seconds and reuses the foreground five-second
inventory schedule. pressure has its own coalesced polling lane.

[identity registration](agent-identity-projection.md) is content-free and bound
to the exact foreground process lifetime. deployment-owned codex SessionStart
hooks and the claude-work plugin call the closed `agent-hook` command. its
bounded decoder reads only the documented session id and writes
`@skid_agent_runtime` after PID/start/tty/profile validation. the current
projection consumes claude registration only. missing, stale, malformed,
nested or ambiguous registration does not prevent terminal observation/control.
hooks never publish status, activity, lifecycle, attention, prompts or results.

`skid-notify` may emit BEL to its inherited exact pane as terminal-local
presentation. it stores no state, calls no gateway and has no privileged
product meaning.

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
  gateway entrypoint drops inherited `TMUX`, `TMUX_PANE`, `TMUX_TMPDIR`,
  `HERDR_*`, and `SKIDBLADNIR_SHELL`. tmux/helper child environments also drop
  `HERDR_*`; pane exec boundaries repeat that removal because existing tmux
  servers have their own inherited environment.
- `internal/platform` is only the closed `Linux | Darwin` native adapter.
  Deployment supplies one strict JSON host config containing expected platform,
  an exact tmux path, an advisory `testedVersion`, an absolute
  `nativeControlPath`, and either no profiles or the four closed profile rows. Every row has exactly one `Codex | Claude` provider and
  exactly one absolute provider-home environment value: `CODEX_HOME` for Codex
  or `CLAUDE_CONFIG_DIR` for Claude. Provider-home values are unique within a
  provider. Identical foreground-signature rows cannot be shared across
  providers; a concrete process matching more than one provider is
  unclassified.
  Unknown/null members, relative paths, duplicate keys,
  runtime platform mismatch, or a missing/broken/noncanonical tmux executable
  fail startup. `skidbladnir validate-host-config --host-config=ABSOLUTE_PATH`
  checks strict config/platform admission without invoking tmux or providers;
  executable availability and live behavior remain separate checks.
  a canonical installed version that differs from `testedVersion`
  remains runnable; `scripts/fleet verify` reports functional fleet health
  without turning advisory tmux-version drift into failure.
  `internal/process` owns native foreground and ancestry observation for
  runtime identity, command revalidation and the content-free SessionStart hook
  adapter. it establishes process identity, not work state.
  `internal/agentruntime` owns provider/profile validation,
  foreground classification, registration encoding/acceptance, and
  provider-specific argv rules. Linux process and
  pressure collection stays behind Linux build constraints; Darwin uses
  `KERN_PROC`, `KERN_PROCARGS2`, `proc_pidinfo`, `proc_pidpath`, processor
  ticks, native memory pressure, `vm.swapusage`, and `statfs`, never parsed
  `ps` output or a Linux fallback.
  `internal/agentcontrol` depends on sessions and the configured native helper;
  sessions never imports agentcontrol. the gateway composes both. short-lived
  claude helpers inspect/read/stop using the selected profile environment;
  neither helper nor client owns the provider runtime.
- Public `dev-server` is the sole machine-local install owner. It pins one immutable
  GitHub release, source SHA, and two host-bundle digests, while this repository
  owns the complete five-asset release pin. It renders the exact Devbox/MacBook/Arch host
  configs, SessionStart identity hook files, the local Claude identity plugin,
  and the BEL-only Codex notify asset; the
  product-local Claude shell commands load that plugin without editing user settings.
  hooks are installed only in skid's private provider homes. it
  owns user systemd services with lingering on Linux and one RunAtLoad
  LaunchAgent on macOS, and applies only its dedicated
  Tailscale Serve `:8443/v1` mapping. It removes only the retired owned root
  handler and never resets unrelated Serve state. Reinstall preserves credentials and tmux
  lifetimes after the first restoration handback, which requires fresh skid
  identities and excludes old herdr-backed rollback generations. sleep, logout,
  Tailscale loss, or service absence is ordinary
  machine-local unreachability; Skíðblaðnir does not wake a host.
  Codex and Claude are installed from exact reviewable npm locks; tmux follows
  each platform's native stable package channel. new skid agent sessions use
  the deployment-owned permission bypasses in §2.
- accepted 2026-09-17 operator scope: `scripts/fleet` owns only `verify`, `invite`,
  and `provision-clients`. apply acceptance, lifetime digests, reboot checkpoints,
  and outage/recovery commands are retired. `dev-server` owns installation,
  idempotence, credential/session preservation, service lifecycle, and autostart;
  behavioral verification follows the current [testing policy](rules/testing.md).
  historical results remain attributed to their original source.
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
| `POST /v1/sessions/{tmuxId}/agent/{operation}` | `read`, `send`, `keys`, `interrupt`, or `stop`; strict full foreground target plus operation inputs, [agent-control contract](agent-control.md#capability-and-api-contract) |
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
  providerless profile rows, flat `activity`/`status`, `runtime`, `interaction`, `attention`,
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
- release artifacts use one dedicated Skidbladnir signing key held outside git;
  release builds never use the ambient android debug keystore. the repository
  pins the public certificate digest. ordinary debug builds use android's debug
  identity and are a compile/development lane; there is no signed debug variant
  or device acceptance gate. `scripts/install-android` accepts only the pinned
  release identity. the private identity and password file are
  an operator backup obligation; losing them requires reinstall rather than a
  trust bypass. A release tag also carries Linux-amd64 and Darwin-arm64 host
  bundles, `SHA256SUMS`, and the public signing-certificate digest; signing
  remains local and release publication remains a reviewed draft action.
- `MachineStore` persists the exact three-machine collection in app-private
  preferences; handles, case-insensitive labels, origins, and bearer bytes are
  each unique, enforced at the store read boundary. an incomplete, invalid,
  or colliding collection quarantines the whole fleet. every bearer is
  AES-256-GCM encrypted by Android
  Keystore with a fresh nonce and AAD bound to handle and origin. Origins are
  pinned HTTPS `:8443` endpoints with hostname and no user-info, path, query,
  or fragment. Labels, origins, and handles are immutable in the app; bearer
  repair re-authenticates the same handle. quarantine admits no credentials
  and opens the fleet-reset screen before any dashboard or machine requests.
  that screen distinguishes an unreadable collection index from unreadable
  pairings. neither trusts partial plaintext metadata, permits bearer repair,
  or offers in-app destructive recovery. reset app data outside the app, then
  connect again. there is no old store reader or migration.
- An empty valid store opens `Connect your fleet`. `Connect` uses Google Code
  Scanner without camera permission and strictly parses one exact
  `skidbladnir.fleet-invite.v1` QR containing ordered Arch, Devbox, and MacBook
  labels, canonical HTTPS origins, immutable handles, and unique invitation
  tokens. Tailscale installation/login stays an explicit external action; the
  app neither embeds nor claims to control the VPN.
- The app redeems all three one-use tokens concurrently, awaits every result,
  and writes only after every returned handle and platform match and all
  bearer values are distinct. It seals all bearers
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
  repaired in-app. an invite naming different machines leaves the installed
  fleet unchanged; the reset screen explains that back keeps it, while replacing
  it requires an external app-data reset. ordinary upgrades preserve the
  collection; app-data loss returns to Connect. there is no old store reader,
  ADB provisioning path, or smaller-fleet branch.
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
- terminal renderer: Vendored pinned xterm.js in a locked WebView
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
  owns the implemented palette, typography, shape, ornament, motion and
  terminal theme. The key deck has stable row-major
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
  host's live streams on its trusted clients. Tailnet admission belongs to loopback binding plus
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

## 8. Upgrade ladder

agent control, spaces, terminal creation and the organized desktop browser are
accepted and implemented. their detailed specifications own their limits.
[the composition plan](spaces-and-shells.md) leaves terminal embedding as a
separate feasibility experiment; shipping it requires an accepted production
contract and its own evidence.

push, unread-result attention, provenance, copied provider history, durable
receipts and replay remain excluded. any new capability requires an explicit
scope and acceptance-criterion change; removing old code does not authorize it.

## 9. Verification

[testing policy](rules/testing.md) owns the temporary-test cleanup workflow and
retained engineering commands. `scripts/check verify` runs static checks and
builds, with no behavioral tests or live tmux/provider/device boundary.
release integrity is separate from behavioral acceptance.

accepted feature specifications retain their product acceptance criteria.
removed suites, unavailable boundaries and unexecuted checks are never passes.
[the roadmap](roadmap.md) points to open issues and source-attributed historical
evidence. deleting old test recipes does not erase failures or satisfy gaps.
