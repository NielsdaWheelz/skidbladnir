# Skíðblaðnir v1: product and architecture

Status: reviewed implementation target; no implementation is implied. Audience:
implementers and reviewers. Evidence last checked: 2026-08-23.

Specification precedence:

1. This document owns Skíðblaðnir behavior, architecture, scope, trust, and
   acceptance.
2. [`docs/rules`](rules/index.md) owns implementation standards unless this
   document names a narrower Skíðblaðnir rule. The rules bind by intent,
   translated to Go and Kotlin. Where a rule document's mechanism is specific to a
   TypeScript/Effect runtime — returned effect values, self-wired service
   layers, layer kinds, generic-parameter forms — Section 5 composition and
   Section 8 explicit Go composition are the named narrower rule; the
   underlying obligations (supervised termination, single wiring, closed
   variants) still bind. `frontend.md` binds the Compose client's state,
   variant, and error-message rules; its routing rules have no Skíðblaðnir
   surface.
3. Imported [`docs/rules/modules`](rules/modules/index.md) binds only when
   linked from this document; a cross-reference from one rule document into
   `modules/` does not bind Skíðblaðnir.
4. [`testing.md`](rules/testing.md) is the testing authority. Section 9 owns the
   concrete Skíðblaðnir proof set.

Resolve any conflict before implementation.

## 1. Outcome

Skíðblaðnir is Niels's private Android command deck for Codex subscription
agents on one remote devbox. From a Galaxy S22+, Niels can:

- see a dense grid of live `ga-*` agents and host pressure;
- start an agent under `personal`, `work`, or `work2` in a chosen directory;
- open the exact same Codex TUI from Android or laptop tmux;
- type, paste, or dictate through the system keyboard;
- detach without stopping work;
- close an idle TUI, then reopen the exact saved conversation.

The phone is the cockpit, Codex owns conversation/execution, and tmux owns the
persistent terminal. Skíðblaðnir owns identity, attachment, lifecycle facts,
attention, and host telemetry.

The design is terminal-first:

- preserve the stock Codex TUI instead of rebuilding chat;
- preserve mosh/tmux instead of creating a competing workflow;
- use hooks and summary-only App Server reads, never screen scraping, for state;
- track agents; cwd is a launch input/fact, never a project or directory entity;
- show `UNKNOWN` whenever evidence is insufficient;
- finish one Core path before notifications or self-update.

Core v1 is this document's implementation scope. Wake (FCM) and Release
(Kache-style A/B packages) are recorded for v1.1 only.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; repository/ASCII namespace `skidbladnir`; private, one user, one phone, one devbox |
| Phone | Samsung Galaxy S22+ `SM-S906W`, Android 16/API 36; wireless debugging allowed |
| Host | One Hetzner VPS; Linux, mosh, tmux 3.4 floor with recorded proven version, systemd user services |
| Network | Tailscale TLS; loopback gateway; Funnel/public ingress forbidden |
| Profiles | Closed enum `personal | work | work2`; never caller-supplied `CODEX_HOME` |
| Runtime | One managed Codex App Server daemon per profile; Unix socket only |
| Interaction | Stock remote Codex TUI in tmux; no Skíðblaðnir transcript/composer |
| Android start | Explicit validated devbox cwd; profile default model; YOLO |
| Laptop start | Current shell cwd; normal interactive model/image/prompt flags; YOLO |
| Workspace | Shared-live only; worktrees remain prompt-level Codex instructions |
| Handoff | One live TUI; tmux shadow clients may attach concurrently without takeover |
| Capacity | No agent cap, scheduler, pressure gate, or one-writer-per-repo rule |
| Client | Kotlin/Compose dashboard; vendored pinned xterm.js terminal asset |
| Host app | Go, SQLite, narrow App Server broker, tmux/PTY, hooks, `/proc` |
| Transport | Strict HTTPS JSON, durable SSE facts, authenticated WSS terminal |
| Trust | Codex is trusted as the devbox user; no hostile-agent containment claim |

Profile mapping:

| Profile | Existing command | Required `CODEX_HOME` |
| --- | --- | --- |
| `personal` | `codex-personal` | `/home/niels/.codex-personal` |
| `work` | `codex-work` | `/home/niels/.codex-work` |
| `work2` | `codex-work2` | `/home/niels/.codex-work2` |

All three commands currently resolve through `ai-router`. Skíðblaðnir remains its
own repository. Installation adds one narrow router/shim integration; it does
not import or depend on `llm-calling`.

### Product language

| Role | Product language | Contract |
| --- | --- | --- |
| Android app | **Skíðblaðnir** | Display name; repository, code, paths, and package identifiers use `skidbladnir`. |
| Agent and host overview | **Hlíðskjálf** | The grid and pressure surface. Literal navigation and accessibility label: `Agents`. |
| New-agent surface | **The Forge** | `+` is labeled `Create new agent`; submission remains `Start agent`. Aulë/Hephaestus supply motif only. |
| Dwarf character catalogue | **Dvergatal** | Versioned, bundled names and portraits; presentation only. |
| Human source of intent | **Gloriana** | Conceptual role, not a screen, type, or service. |
| Future orchestration engine | **Sūtradhāra** | Reserved name; Core has no placeholder, route, schema, or abstraction for it. |

Prospero, Solomon/the Seal, shabti, and Gwydion/*Cad Goddeu* are reserved
product vocabulary until a concrete capability owns each name. Beyond the
ASCII product namespace, mythic language stays in presentation; domain/API
concepts and lifecycle, error, trust, and destructive-action labels remain
literal.

### Capability contract

Core exposes one path per capability:

1. **Start**: validate an explicit cwd, allocate an exact empty Codex thread,
   then run its remote TUI in a deterministic tmux target. Never send a prompt
   through Start.
2. **Discover**: register supported laptop starts through the same launcher.
3. **Observe**: combine hooks, `thread/read(includeTurns=false)`, and exact
   process/tmux liveness.
4. **Attach**: bridge Android to the registered tmux pane without replacing the
   TUI or taking over another client's window, pane, or geometry.
5. **Detach**: close only Android's terminal client; leave TUI/turn/tmux alive.
6. **Close**: stop an idle exact TUI and locally close the card; preserve Codex
   history.
7. **Reopen**: run `resume --remote unix:// <exact-thread-id>`.
8. **Recover**: reconcile exact refs and liveness; never replay terminal input.

### Goals

- Match known Kache behavior: `ga-*` cards, named dwarf portraits,
  `WORKING`/`IDLE`, objective/activity, many agents, voice-driven mobile TUI,
  tmux continuity, and recovery.
- Surpass the known surface with explicit account separation, honest unknowns,
  host-pressure history, typed failures, and boundary proofs.
- Let phone and laptop slide into one live terminal with no transcript copy or
  second conversation client.

### Non-goals

- Project enrollment, filesystem browsing, repo/worktree identity, git/PR
  management, workspace modes, collaboration policy, quotas, scheduling,
  cgroups, or shared agent memory. The editable cwd and recent suggestions do
  not create directory records.
- Native transcript/chat, prompt/steer/interrupt HTTP APIs, token streaming,
  terminal parsing, or Android-to-App-Server access.
- App Server-driven turns. The host broker may initialize, verify account/model,
  create/list/read summary-only threads, and unsubscribe; nothing more.
- Provider/runtime registry, provider fallback, SDK/embedded Codex fallback, or
  `llm-calling` integration.
- Raw shell/tmux target/App Server endpoint/`CODEX_HOME` input from Android. Cwd
  is the sole caller-selected host path and is validated as specified below.
- Coordinated multi-writer leases. Tmux may have multiple viewers; one person
  types at a time.
- Character-driven prompts/personas, catalogue CRUD/network fetch, character
  picker/reroll, or any character tradition outside Old Norse/Eddic, Tolkien,
  and Germanic/operatic sources.
- Core: FCM, A/B updating, prompt/terminal image transfer, custom speech
  recognition, TTS, terminal screen reconstruction, permanent purge, or
  off-host disaster recovery.

## 3. Evidence and decisions

Claims are **observed**, **stated**, **platform fact**, or **Skíðblaðnir decision**.
Similarity is not attribution.

| Evidence | Supports | Does not prove |
| --- | --- | --- |
| Kache [agent grid](https://x.com/yacineMTB/status/2090580615049408711) | Observed dense two-column `ga-*` cards, character art, objective/activity, `WORKING`, `IDLE`, attention marks | Private backend, concurrency, durability |
| Kache [mobile TUI](https://pbs.twimg.com/media/HQVGPaeXQAAARj3?format=jpg&name=medium) | Observed stock-looking Codex TUI on Android | Embedded vs tmux vs `--remote` implementation |
| Kache [A/B post](https://x.com/yacineMTB/status/2090436678519312650) | Stated/observed two installed variants intended to preserve access during updates | Signing, activation, rollback safety |
| Kache [recovery screenshot](https://x.com/yacineMTB/status/2090443641697288552) | Observed proposal to inventory `ga-*`, tmux, saved Codex sessions, `todo.txt` | That recovery ran or matches Skíðblaðnir internals |
| [Codex App Server](https://learn.chatgpt.com/docs/app-server) | Platform fact: rich-client interface, explicit thread cwd, remote TUI, Unix sockets, generated versioned schemas | Production support or in-flight survival after process death |
| [Codex hooks](https://learn.chatgpt.com/docs/hooks) | Platform fact: session/prompt/tool/stop/end events and exact fields | Stable transcript format; the docs explicitly deny that |

The image's bottom row is Gboard, not Skíðblaðnir: toolbar menu, stickers/emoji,
GIF, Writing Tools, clipboard, settings, microphone. Core relies on the installed
IME and does not copy this row.

The observed `whitebox` card label has no public semantic contract. Skíðblaðnir does
not invent one; the equivalent badge is the truthful runtime/profile pair
`CODEX · personal|work|work2`. The observed product is branded `k-stack` in its
own UI; Skíðblaðnir matches surveyed behavior, never branding, naming, or
unobserved internals.

Local inspection on 2026-08-23 found `codex-cli 0.149.0`, tmux 3.4, the three
commands above, `--remote`, `resume --remote`, `agents`, `app-server proxy`, and
`app-server daemon bootstrap|start|version`. No App Server socket was live.
Tmux 3.4 exposes grouped sessions with independent current windows plus client
`active-pane` and `ignore-size` flags; an isolated probe confirmed independent
group window selection. These are dated observations, not the release pin.

| Question | Decision | Why |
| --- | --- | --- |
| TUI or structured chat? | Stock TUI | It already owns conversation, tools, approvals, and rendering. |
| App Server or embedded CLI? | App Server daemon only | Shared account-local state and exact remote resume. |
| App Server transport? | Default per-profile Unix socket | Host-local; no raw experimental network listener. |
| How bind tmux to thread? | Broker allocates/resolves id before TUI exec | Shared-daemon hooks do not identify their initiating tmux client. |
| How hand off devices? | Native tmux grouped shadow client | One TUI preserves live screen/draft; client flags isolate phone focus and sizing. |
| Status truth? | Hooks + summary read + liveness | Semantic, repairable, attachability-aware; no pixel inference. |
| Android terminal? | Vendored pinned [xterm.js](https://github.com/xtermjs/xterm.js) in local WebView | Mature VT/IME rendering; native app retains auth/network ownership. |
| Voice/clipboard? | System IME/Gboard | Ordinary terminal text; no parallel speech/composer lifecycle. |
| Close? | Idle TUI stop + local close | Conversation remains exact and resumable. |
| Second runtime? | Defer | One real second CLI must define the honest abstraction. |

App Server and its raw WebSocket transport are documented as experimental and
unsupported for production workloads. The per-profile Unix socket narrows
exposure only; it carries the same experimental App Server protocol, so the pin
and probes — not the transport — are the mitigation. This private prototype
accepts that risk through an immutable pin, Unix sockets, generated stable
schemas, live probes, and fail-closed upgrades. It must not claim OpenAI
production support.

Lane 0 must prove on the exact pin:

- empty `thread/start` -> `thread/unsubscribe` -> stock remote TUI resume;
- App Server thread/session ids map exactly to hook ids;
- whether pinned `thread/list` summaries carry `serviceName`, cwd, and creation
  time (Start stabilization depends on the answer);
- tmux 3.4 grouped sessions plus `active-pane`/`ignore-size` preserve one TUI and
  input buffer without changing the attached laptop's selection or geometry;
- phone pane targeting in a split window changes neither the window's active
  pane nor another client's input routing, and phone input reaches only the
  targeted pane;
- killing a non-last grouped session preserves pane id, TTY, and PID; killing
  the last one destroys them; a fresh client attach repaints the full current
  screen; a sole `ignore-size` client sizes the window and yields to any
  unflagged client, under effective global `window-size latest`;
- `SessionEnd` delivery and ACK complete before an exiting laptop TUI's parent
  shell regains the pane;
- stopping an idle TUI leaves its thread resumable, and a recorded probe shows
  what an in-flight remote turn does when its TUI stops;
- the pinned TUI's exact intra-line edit, newline-without-submit, history, and
  scroll key sequences, and whether its transcript lives in the normal buffer
  or the alternate screen;
- S22+ WebView/xterm.js/Gboard handles ANSI, Unicode, IME, and resize, and the
  pinned emulator's automatic reply set (DA1, DA2, DSR, CPR) is recorded.

Failure reopens this design; it never authorizes a fallback.

## 4. Target behavior

### Android start

1. Tap `+`; select profile, short card objective, and working directory. The
   editable cwd defaults to the first server-supplied recent for that profile,
   initially `/home/niels/src`; up to eight server-derived recents are one-tap
   suggestions. The server owns the default and the suggestions; the client
   persists neither.
2. Submit `StartAgent`. The API accepts no prompt.
3. Before allocation, cwd parses into one canonical value or fails
   `WorkingDirectoryInvalid`: at most 4,096 valid UTF-8 bytes, no NUL and no
   C0/C1 control bytes; one optional leading `~`/`~/` resolves against the
   service UID's home; cleaned; absolute; an existing directory the service UID
   can enter. Symlinks resolve as the kernel resolves them. There is no
   reject-list — any string that is not that canonical value is invalid. Argv
   is constructed directly, so shell syntax is ordinary path text and needs no
   special case. A cwd that parses but cannot be entered when the runtime
   starts settles `WorkingDirectoryUnavailable`.
4. The broker calls stable `thread/start` with that normalized cwd, profile
   default model, approval `never`, danger-full-access sandbox, no experimental
   capability, and `serviceName:"skidbladnir"`.
5. Persist returned `thread.id` and `thread.sessionId`; unsubscribe the broker's
   automatic thread subscription.
6. Create the deterministic per-incarnation `ga-*` tmux session. Register its
   exact TTY/PID/start time, then exec pinned
   `codex resume --remote unix:// <thread.id>`.
7. `SessionStart` confirms the mapping and effective cwd; show `IDLE`; open the
   terminal.
8. Niels types/pastes/dictates the real prompt in the stock TUI.

Cwd is execution context, not identity or an enrollment boundary. No root
allowlist applies: YOLO already grants the agent the devbox user's filesystem
authority. Validation prevents malformed launches; it is not a sandbox.

A lost post-write `thread/start` response stabilizes once before any product
outcome: the reconciler reads that profile's summary `thread/list`, matching
`serviceName:"skidbladnir"`, the normalized cwd, and the dispatch window. A unique
match completes the original command against that thread; absence or ambiguity
terminates it as `RecoveryRequired`, an immutable receipt that is never
redispatched. The user-confirmed retry is a new `StartAgent` under a fresh
`clientCommandId` and warns that an unused empty thread may remain. If Lane 0
proves the pin's summaries cannot carry the match fields, stabilization is
skipped and `RecoveryRequired` is the intentionally modeled terminal outcome.
No real work can be duplicated because Start never sends a prompt.

### Laptop start and resume

The natural flow remains:

```text
mosh devbox
tmux attach ...
cd any-directory
codex-personal | codex-work | codex-work2
```

The shim asks the local broker to create a new exact thread. `resume <id>`
verifies that id; bare `resume` uses a broker-backed picker and resolves an id
before exec. The shim registers current tmux/TTY/PID, then execs the exact-id
remote TUI. A tracked TUI is bound to one thread for its process lifetime; open
another through another card/invocation. `PrepareNew` and `PrepareResume`
record their `thread/start` dispatch in `lifecycle_commands` under the reserved
launcher installation identity, with the same kind/digest/outcome fields as
phone starts. A lost post-write response is `RecoveryRequired`: the shim prints
the unused-empty-thread warning and exits without exec; a re-run is a new agent
by intent, never a replay.

Interactive model/image/prompt/`-C` arguments are preserved; conflicting
transport, `CODEX_HOME`, approval, or sandbox inputs fail. Noninteractive Codex
subcommands remain ordinary `ai-router` behavior and are not Skíðblaðnir agents.
Starts outside tmux remain usable remote Codex but are not shown by Skíðblaðnir.

The first `UserPromptSubmit` supplies at most 240 normalized scalars as the
objective when a laptop-created card lacks one. Cwd is observed for resume and
diagnosis only; it never becomes project identity.

### Phone/laptop handoff

- Opening a card never asks the laptop to detach and never starts a second TUI.
  The laptop's mosh connection, tmux client, terminal, and Codex PID stay live.
- The gateway creates an ephemeral tmux session grouped with the registered
  source session, then attaches the phone PTY to that shadow. Group members
  share the exact windows/panes/processes but keep current-window state separate.
- The phone client keeps `active-pane` and `ignore-size` permanently. Never
  target a shared-window pane with `switch-client -c` or any select outside the
  phone client's own command context: on tmux 3.4 both set the window's shared
  active pane and redirect other clients' input. The gateway attaches with one
  invocation — `attach-session -t <shadow> -f active-pane,ignore-size \;
  select-window \; select-pane`; later targeting uses a reserved binding in a
  dedicated Skíðblaðnir key table, armed with `switch-client -c <phone-tty> -T
  <table>` and delivered by writing the bound key into the gateway-owned phone
  PTY. Client-context `select-pane` under `active-pane` moves only that client;
  Skíðblaðnir-created sessions have a single pane and need no selection. While a
  laptop client is attached, `ignore-size` keeps rotation and IME resizing from
  reflowing the laptop TUI; a smaller phone sees tmux's cursor-following
  viewport, and rotation/terminal zoom reveals more cells.
- `ignore-size` protects other clients, not geometry: tmux honors it only while
  an unflagged client is attached, so Android owns sizing exactly when it is
  the group's sole client, and each ownership transition may resize the shared
  window once. Never toggle the flag — clearing it while a laptop is attached
  reflows the laptop TUI immediately. `OWNER | CONSTRAINED` derives from the
  observed group client list and geometry, never from flag state. The source
  server must run effective global `window-size latest` (the 3.4 default),
  asserted by `/readyz`; on tmux 3.4, global `window-size manual` crashes the
  server on detached session creation.
- Both devices see and type into the same screen, draft/input buffer, process,
  turn, and thread. Inputs can interleave; Core assumes one human writer and has
  no edit lease.
- `Detach` destroys only the ephemeral phone client/shadow session. It never
  detaches a laptop client or stops the source pane/TUI. Cleanup first detaches
  the phone client, then destroys the shadow through one in-server guarded
  command — `if-shell -t =<shadow> -F "#{e|>:#{session_group_size},1}"
  "kill-session -t =<shadow>"` — never a separate verify-then-kill: killing the
  group's last session destroys the shared panes and TUI. Any surviving group
  member permits destruction; when the shadow is the last link, retain it,
  promote the exact shadow binding, and reconcile. After destruction, re-verify
  by stored pane id/TTY/PID that the target still lives under a linked session;
  a source session lost in that window reports through the recovery evidence
  path, never as a clean detach. Detach, promotion, and shadow GC for one agent
  serialize in the gateway.

A second `codex --remote` process is not handoff: it could resume the same
thread, but it would not share this TUI's unsent draft, local screen state, or
terminal process. Skíðblaðnir therefore shares one TUI through tmux.

### Close, reopen, recovery

- While `WORKING`, Close is disabled. Send terminal `Ctrl-C`; wait for semantic
  `Stop`/summary `idle`, then Close.
- Accepting Close atomically enters `CLOSING`, rejects new attaches, and fails
  concurrent `ReopenAgent`/`StartAgent` for that agent with `CommandConflict`.
  Close pins the runtime binding verified at admission. The worker verifies
  exact PID start time/TTY/pane, closes Android attachments, then re-verifies
  idleness from the latest hook facts — with a fresh summary read past the
  staleness bound — immediately before stopping; an unclosed turn or active
  summary fails the command `AgentWorking` and `CLOSING` reverts to the derived
  state. It then stops only that idle TUI and commits local closure. Absence of
  the pinned incarnation commits closure only while it is still the agent's
  latest binding and no live binding exists; otherwise the command fails typed.
  It never archives or deletes the App Server thread.
- Close closes Android attachments before stopping the TUI, so it never streams
  a returned shell. Android-started runtimes exec the TUI as the pane process,
  so its exit destroys the pane and exposes no shell. On spontaneous exit of a
  laptop shell-child TUI, the bridge closes on first end evidence —
  `SessionEnd` before hook ACK, then liveness or pane-identity mismatch — with
  bounded, not zero, latency; the brief glimpse is accepted within the terminal
  endpoint's shell-equivalent trust.
- Reopen is permitted only from `RECOVERABLE`, `CLOSED`, or `EXCEPTION` holding
  exact refs; acceptance atomically enters `STARTING` and rejects concurrent
  close/reopen/attach for that agent. Inside that gate the worker re-verifies
  that every stored runtime binding is verified dead — registered PID/start
  time/TTY exactly absent. Verified-dead is distinct from not-verified-alive;
  unverifiable liveness fails `LivenessUnverifiable` and never starts a TUI.
  Reopen then creates a new dedicated per-incarnation tmux runtime for the
  exact stored `thread.id`. Missing exact thread becomes `EXCEPTION`; no
  search-by-title/cwd/recency. `PrepareResume` applies the same gate and prints
  the live target instead. Two simultaneously live runtime bindings for one
  thread is a defect, never a race outcome.
- Phone death, WSS/mosh loss, or tmux client detach does not affect TUI/turn.
- Gateway restart reconciles SQLite with summary-only thread reads and exact
  tmux/PID/TTY facts, and resumes every non-terminal lifecycle command from its
  last committed step before ingress opens. Unresolved or contradictory
  evidence is `UNKNOWN`.
- Tmux/host loss with an exact thread is `RECOVERABLE` when the agent was idle
  and `EXCEPTION` when it died while working; reopen resumes history either way
  and never claims killed in-flight work survived.
- Profile daemon loss degrades that profile's health only: new Start/Reopen
  fail `ProfileUnavailable`, and existing cards keep deriving from hook and
  liveness evidence. No embedded or cross-profile fallback. Recover the same
  pin, then reopen exact threads.
- `$XDG_STATE_HOME/skidbladnir/todo.txt` is an atomic `0600` operator recovery
  projection of handle, profile, exact refs, last tmux target, objective,
  state — rewritten from committed state in one awaited step after the Fact
  commit, never inside a SQLite transaction. SQLite is authority; `todo.txt` is
  never input, never read by Skíðblaðnir code, and carries no field absent from
  SQLite.
- Terminal input is never replayed after disconnect. Reattach, inspect, decide.

## 5. Architecture and ownership

```text
                         mosh / laptop
                              |
                              v
                       source tmux pane
                              ^
                    grouped shadow client
                              ^
Galaxy S22+                                  App Server daemon per profile
  Gboard -> local xterm.js                   personal | work | work2
             |                                  ^ proxy       ^ unix://
  Compose dashboard                             |             |
             | HTTPS + SSE + WSS                |             |
             v                                  |             |
      Go gateway + narrow broker ---------------+      tmux -> Codex TUI
        |       |       |                             --remote unix://
        |       |       `-- PTY bridge
        |       `---------- /proc sampler
        `------------------ SQLite + hook/control sockets
```

Android never reaches App Server. The broker owns only identity/summary methods;
it never starts a turn or projects conversation content. Terminal bytes pass
between exact tmux PTY and Android without interpretation.

### Composition

- Bootstrap official daemon management once per profile. Require exact
  CLI/daemon version before that profile is available.
- One systemd user service owns gateway, SQLite, three bounded `app-server
  proxy` children, hook/control sockets, tmux/shadow reconciliation, PTYs, SSE,
  and metrics.
- Bind loopback; expose `/v1` only through Tailscale Serve; disable Funnel.
  `/healthz` and `/readyz` bind loopback only and are never exposed through
  Serve.
- `/healthz` proves process life. `/readyz` proves config, migrations, SQLite
  pragmas, sockets, Serve, and tmux: server liveness, version at or above the
  3.4 floor and equal to the recorded proven version, effective global
  `window-size latest`, and the grouped-session/`active-pane`/`ignore-size`
  behavior probe. Tmux version drift fails readiness until the handoff proof is
  re-run and the new version recorded. Profile health is separate and never
  substituted.
- Supervise every task. Required-owner failure closes ingress and restarts the
  composition; profile-only failure degrades only that profile.

### Narrow App Server broker

Each profile proxy has one serialized JSONL writer, bounded decoder, request
correlation, and drained/redacted stderr. Commit generated non-experimental
schemas for the exact pin.

Allow only `initialize`, `account/read`, `model/list`, and the minimum stable
`thread/start|read|list|unsubscribe` shapes. Reads set or imply
`includeTurns:false`; reject any response carrying turns. `thread/list` is
called only in the pinned bounded page shape; the bare-`resume` picker and
`ListResumableSessions` never request an unbounded list.

`thread/unsubscribe` is a required post-allocation step, not best effort: retry
it on one named bounded schedule while draining/discarding that thread's
notifications. Exhaustion is a defect; the broker then recycles that profile's
proxy connection, which drops every subscription. A connection with an
unreleased subscription is never reused. Notifications naming any thread id are
counted, drained, and discarded, never a failure.

Broker faults have two classes. Transport faults — malformed or oversized
frame, unmatched response, write error — fail the in-flight request, log a
defect with correlation handle, and recycle the proxy connection; failures past
a named consecutive threshold mark the profile `ProfileUnavailable` until the
supervised availability probe (`initialize` plus daemon/CLI version and
schema-digest comparison, on a named schedule and on `skidbladnir profile recheck`)
restores it with a durable Fact. Drift — unknown method/variant, pin/schema/
config mismatch — is Skíðblaðnir's own defect: that profile is unusable until
operator repair and reports distinctly from daemon loss. `ProfileUnavailable`
itself covers only observed daemon/socket outage of a correctly pinned profile.

Profile availability also requires `account/read` to match the account recorded
in profile config and `model/list` to contain the profile's default model;
mismatch is config drift. `/v1/bootstrap` profile health carries a digest-only
account fingerprint, so account separation is visible without identity.

Never send `turn/*`, archive/delete, item, transcript, goal, fork, MCP, tool,
shell, or approval methods. The broker is identity/status plumbing, not chat.

### Launcher and hooks

The launcher resolves profile config, asks the broker for exact refs, starts the
matching daemon, registers exact runtime identity over the local control socket,
and execs only strict-config YOLO `resume --remote unix://`. It constructs argv
directly; no user string becomes shell source. No embedded path exists.

Installation writes each profile's hook config and records its digest. Profile
readiness verifies those digests against the committed expected set; a mismatch
is hook drift and makes that profile unavailable on the same fail-visible path
as pin drift. "Reviewed" names that digest check and nothing more. Hook facts
carry devbox-user trust, not Codex trust: any same-UID process can write the
`0600` hook socket, and peer-UID verification distinguishes nothing under this
trust model.

The reviewed user-level hooks in each profile emit:

| Hook | Stored projection |
| --- | --- |
| `SessionStart` | Confirm profile/thread mapping; observe cwd/model/source |
| `UserPromptSubmit` | Turn id, `WORKING`, optional initial objective preview |
| `PostToolUse` | `tool_name` and time only |
| `Stop` | Terminal turn, `IDLE`, result-ready attention |
| `SessionEnd` | Semantic end; liveness still decides attachability |

The hook helper validates one documented input shape and sends one bounded
message to a `0600` Unix socket. It returns no model-visible output. Delivery is
ACKed; failure writes an atomic gap marker and never blocks Codex. A later hook
or summary read repairs the gap. Hook facts order by turn id; a `Stop` closes
only its own turn. Every observation records capture time; newer capture wins,
and a summary read captured before the latest hook never overrides that hook.
Never read `transcript_path`; discard full
prompt after objective projection, assistant text, tool input/output, patches,
stdout/stderr, and reasoning.

### Ownership and trust

| Owner | Owns | Must not own |
| --- | --- | --- |
| Android | Dashboard, bundled portraits, requested cwd, local terminal rendering/input, disposable cache | Agent truth, character assignment, Codex refs, effective cwd, tmux commands |
| Gateway | Auth, strict DTOs, paging, SSE/WSS admission | Transcript, turns, provider policy |
| Registry/commands | Agent identity, character assignment, lifecycle facts, idempotency, attention | Repos, worktrees, generic jobs, terminal bytes |
| Broker | Stable thread identity/summary allowlist and schema | Turn control, content, archive/delete |
| Launcher/hooks | Closed profile, exact-id exec, semantic observations | Guessed correlation, raw network exposure |
| Tmux/terminal | Exact binding, PTY bytes, attach/detach/backpressure | Conversation meaning, status parsing |
| Metrics | `/proc`, aggregation, pressure reasons | Admission, scheduling, agent attribution |
| SQLite | Skíðblaðnir facts and receipts | Codex auth/history, terminal bytes |

Core, tmux, App Server, and Codex share the devbox user. YOLO code can modify
Skíðblaðnir files/processes. Tailnet and bearer auth protect remote access, not the
control plane from same-UID code.

The terminal endpoint is high-trust shell-equivalent authority. It accepts only
a registered handle, resolves targets server-side, and closes when exact
PID/start-time/TTY/pane identity ends. The Start API's bounded cwd is the only
host path accepted remotely; no API accepts a raw terminal target, command,
profile home, or executable. Hostile-agent containment requires a separate
UID/VM and is out of Core scope.

## 6. Domain, storage, and protocols

### Identity and state

An **agent** has opaque id, stable `ga-*` handle, immutable `characterKey`,
profile, objective, created time, optional local closure time. The id/handle is
authority; the dwarf name and portrait are presentation. An agent is not a
process, thread, cwd, repo, or character-driven runtime persona.

**Dvergatal** is the immutable repository-owned character catalogue at
`catalog/characters.json`, embedded at build time rather than stored as CRUD
data. Core ships at least 100 curated named dwarfs from exactly
`OldNorse | Tolkien | GermanicOperatic`; no `FairyTale` or `ModernMedia`
catalogue family exists. Each entry has exactly `{key, ordinal, displayName,
tradition, source, iconKey}`. Keys are namespaced, such as `norse.durinn` and
`tolkien.durin-i`; ordinals, keys, and icon keys are unique and stable, while
display names may overlap across traditions. Every icon key resolves to one
bundled square WebP portrait. No runtime URL or download exists.

Initial phone and supported laptop registration assign `characterKey` in the
same transaction that inserts the agent. Compute SHA-256 over the UTF-8
canonical agent id, interpret the first eight digest bytes as an unsigned
big-endian integer, and take modulo catalogue length. From that ordinal, scan
cyclically for the first entry not used by a non-`CLOSED` agent and persist it
once. If every entry is active, use the derived starting entry; repeated
characters are allowed and the handle still disambiguates. Reopen never
reassigns. Catalogue size therefore never caps agents.

A **Codex binding** stores App Server `thread.id` (exact resume ref) and
`thread.sessionId`. Hooks/CLI often call the root ref a session id; Lane 0 proves
the pinned mapping. Requested/effective cwd and model are launch/diagnostic facts
only.

A **runtime binding** is one TUI incarnation: immutable server-generated tmux
`session_id`/`window_id`/`pane_id` — never session names or window/pane
indexes, which rename and renumber — plus pane TTY and PID with kernel start
time, and start/end facts. Tmux ids are meaningful only within one live server
and are guarded by the PID/start-time check. Reopen adds another binding. At
most one live runtime binding exists per agent; contrary evidence is a defect,
never a second card. Android shadows are attachments; only an exact last-link
promotion after source session loss may replace the binding, re-pointing
`session_id` while `window_id`/`pane_id`/TTY/PID stay fixed.

Card state derives by first-match over a total order — `CLOSED` > `CLOSING` >
`STARTING` > `EXCEPTION` > `WORKING` > `IDLE` > `RECOVERABLE` > `UNKNOWN` —
each predicate applying only when every earlier one fails; deriving two states
from one evidence set is a defect:

| State | Evidence |
| --- | --- |
| `STARTING` | Accepted launch; no binding/failure yet |
| `IDLE` | Live binding; newest evidence (`SessionStart`, `Stop`, or summary idle) indicates idle; no unclosed prompt |
| `WORKING` | Live binding; unclosed prompt or summary status `active` |
| `CLOSING` | Accepted close; exact TUI stop unconfirmed |
| `RECOVERABLE` | No live binding; exact stored thread previously existed |
| `EXCEPTION` | Failed/unknown start, `systemError`, missing exact thread, or death while working |
| `UNKNOWN` | Required evidence absent, stale, or contradictory |
| `CLOSED` | Local closure committed; no live binding |

Evidence rules: fresh hook/summary disagreement triggers one on-demand summary
re-read; disagreement that survives it derives `UNKNOWN`, never
`WORKING`/`IDLE`. An accepted-but-uncommitted close stays `CLOSING` even when
the TUI is already dead. Evidence older than three liveness periods is stale
for the `UNKNOWN` rule. `STARTING` is bounded by a named start-binding
deadline: expiry settles the owning command and the card — `RECOVERABLE` when
exact refs are persisted, `EXCEPTION` with the settled failure code otherwise —
evaluated continuously by the supervised reconciler, not only at restart.

The card projection carries the closed derivation reason plus
`failureCode`/`failureObservedAt`; `EXCEPTION` and `UNKNOWN` always carry one
closed code. Evidence Skíðblaðnir observes but does not own — hooks, summary reads,
tmux, `/proc`, PID/start-time/TTY facts — that is absent, stale, or
contradictory derives `UNKNOWN`, at read time, never persisted as a status
column. Skíðblaðnir-owned state contradicting itself — two live bindings, a binding
without its agent, a refs-less `codex_sessions` row, corrupt storage — is a
defect with correlation handle, never `UNKNOWN`.

### SQLite

Every table has its own opaque UUIDv7 id except monotonic Facts, whose sequence
is strictly monotonic and never reused across pruning or restart (persisted
high-water mark). Foreign keys are explicit; migrations are transactional and
declare required engine capabilities. Every connection sets `foreign_keys=ON`
and a named `busy_timeout`; the database runs `journal_mode=WAL` with one
serialized writer; startup and `/readyz` assert these pragmas, and a connection
missing them is a defect. Hook ACK latency is never blocked by read snapshots.

| Table | Fact |
| --- | --- |
| `agents` | Handle/character key/profile/objective/created/closed |
| `codex_sessions` | Exact agent/profile/thread+session refs, requested/effective cwd, model |
| `runtime_bindings` | Verified tmux/TTY/PID incarnation and end |
| `agent_observations` | Closed hook/liveness facts and safe activity |
| `lifecycle_commands` | Installation key, kind/digest, target, per-step state, dispatch/outcome |
| `facts` | Ordered public projection changes for SSE |
| `attention` | Unacknowledged completion/exception |
| `host_samples` | CPU/load/memory/swap/disk/PSI sample |
| `installations` | Bearer verifier, pairing/rotation/revocation/last-seen, last client contract digest |
| `schema_migrations` | Applied migration and time |

Lifecycle command kinds are exactly `StartAgent | CloseAgent | ReopenAgent`;
this is not a job framework. `(installation_id, client_command_id)` is unique
and serializes replays against the original, so one logical command never
executes twice concurrently. The digest covers the normalized semantic payload,
including Start cwd. Each command records its last committed step —
`dispatched`, `refs_persisted`, `runtime_registered`, terminal `outcome`. Exact
replay of a terminal command returns its immutable receipt; exact replay of an
in-flight command returns the original `202` body and current status without
redispatching any stage; different payload/kind is `CommandConflict`.

Record `thread/start` dispatch before proxy write. A lost post-write response
stabilizes once through summary `thread/list` (section 4); absence or ambiguity
is `RecoveryRequired`, never redispatched. The composition resumes every other
non-terminal command from its recorded step until it reaches a terminal
outcome: no dispatch record means the proxy write never happened and the
reconciler re-drives it; persisted refs with an unregistered runtime re-drive
the tmux start. Resumption, not user retry, completes accepted commands; an
undriven non-terminal command is a defect. This per-command resume loop is the
named narrower Skíðblaðnir rule replacing `operation-types.md`'s durable-operation
runners; section 8's exclusion of queue/broker infrastructure stands.

Tmux session names are deterministic per runtime incarnation — handle plus
runtime-binding id — never per agent, so reopen and prior incarnations cannot
collide. The name is a creation-idempotency convention only; replay and bridge
identity key on stored tmux ids plus TTY/PID/start time. Replay adopts an
existing same-named session only after exact verification that the pane root
matches the registered identity running the pinned invocation; an unregistered
or unverified session is killed and recreated, never adopted, never
attachable. Close targets exact PID/start time and is replay-safe after
absence. Projection plus SSE Fact commit atomically. Facts are a closed
PascalCase-discriminated union declared in `api/skidbladnir.v1.json` —
`AgentStarted | AgentBound | AgentStateChanged | ActivityObserved |
AttentionRaised | AttentionAcknowledged | CommandSettled | AgentClosed |
HostPressureChanged` — generated into both clients and matched exhaustively;
adding a kind is a Lane 0 contract change.

Retention: Facts/host samples seven days; safe activity and hook/liveness
observations 30; closed runtimes and acknowledged attention 90; lifecycle
receipts for installation lifetime; agents/bindings until future explicit
purge. Retention never deletes the latest state-bearing evidence: per
non-purged agent, the newest hook fact per kind, newest summary status, newest
liveness fact, latest binding with its end fact, and unacknowledged attention
are kept regardless of age; a derived card state changed by pruning alone is a
defect. Pruning persists the minimum retained Fact sequence. Old SSE cursors
require bootstrap. Terminal bytes and Codex transcripts are never stored.

### HTTP and local control

Strict bounded `/v1` JSON runs over Tailnet TLS. Within `/v1`, Pair is the only
unauthenticated route and requires a local ten-minute challenge. Every other
route requires a Keystore-held installation bearer; SQLite stores only its
verifier. No cookies, redirects, CORS, cleartext, or HTML errors. Responses use
`no-store`.

The pairing secret is at least 128 bits from a CSPRNG, compared in constant
time against its stored verifier, single-use, one active at a time, and
consumed in one SQLite transaction — first success or ten minutes, whichever
comes first, ends it. Five failed attempts destroy the live challenge; the
route then answers `PairingInvalid` until a new local
`CreatePairingChallenge`. The QR carries the Serve host and the secret.
Pairing replaces: a successful `POST /v1/pair` registers the new installation
bearer and revokes every prior phone installation in the same transaction, so
at most one phone installation is active and re-pair is the phone-loss
recovery path. Retired installations keep their `lifecycle_commands` receipts;
replay keys are never reused. Installation bearers are generated 256-bit
material stored only as domain-separated verifiers; bearers do not expire, and
revocation is their only invalidation.

Every `/v1` request carries `Skidbladnir-Contract: <contract digest>`, covering
`api/skidbladnir.v1.json`, canonical `catalog/characters.json`, and the
lexicographically ordered portrait paths plus raw bytes; every response echoes
it. Mismatch is
`ProtocolMismatch` on every route including `/v1/pair`, answered with the
server digest; `/v1/events` and `/v1/terminal/{handle}` reject the upgrade with
the same code.
`ProtocolMismatch` never describes App Server drift.

| Method/path | Contract |
| --- | --- |
| `POST /v1/pair` | Consume challenge; register phone bearer |
| `GET /v1/bootstrap` | Profile health with account fingerprint, host projection, counts, Fact cursor, per profile up to eight recent cwds most-recent-first (recent = confirmed effective by `SessionStart`) |
| `GET /v1/agents?cursor=&limit=&state=` | Keyset pages ordered `(created desc, id desc)`; `limit <= 100`; `state` enum `open \| closed \| all`, default `open` (excludes `CLOSED`); cursor encodes order key and filter |
| `GET /v1/agents/{handle}` | Exact card character/binding/attention/attachability/derivation reason/failure fields |
| `POST /v1/agents` | `StartAgent(clientCommandId, profile, objective, cwd)`; `202` |
| `POST /v1/agents/{handle}/close` | Idle-only idempotent close; `202` |
| `POST /v1/agents/{handle}/reopen` | Exact-thread idempotent reopen; `202` |
| `POST /v1/agents/{handle}/attention/acknowledge` | Idempotent acknowledgement |
| `GET /v1/events` | Durable SSE after `Last-Event-ID` |
| `GET /v1/host/samples?window=&resolution=` | Current sample + aggregated series `<= 300` points; `window` enum `15m \| 1h \| 6h \| 24h \| 7d`; `resolution` derived from `window`, explicit overflow is `InvalidRequest`; buckets aggregate pressure-truthfully (min for available memory/disk, max otherwise), carry worst state and reason union, `UNKNOWN` on any missing required signal, and bucket start times |
| `GET /v1/terminal/{handle}` | Authenticated WSS upgrade |

Every agent/card projection carries `characterKey`, `displayName`, and
`iconKey`. `POST /v1/agents` accepts none of them; assignment is server-owned.

Every `202` returns `{handle, clientCommandId, acceptedAt}`; Start mints the
card in `STARTING` before answering, and exact replay returns the same body.
Each accepted command reaches exactly one terminal outcome — `Succeeded` or one
closed failure code — persisted in `lifecycle_commands` and published as a
durable `CommandSettled` Fact carrying handle, `clientCommandId`, and outcome.
`RecoveryRequired`, `ExactThreadMissing`, and post-acceptance failures are
delivered only through that Fact and the card's failure fields; an accepting
response never returns them, and no command resolves silently.

The local launcher/admin socket is `0600` with peer-UID verification. Closed
commands: `PrepareNew`, `ListResumableSessions`, `PrepareResume`,
`RegisterRuntime`, `CreatePairingChallenge`, `ListInstallations`,
`RevokeInstallation`. It accepts no shell, executable, raw socket/terminal
target/home, SQL, or generic maintenance command. The launcher's
current/effective cwd is a typed `PrepareNew` input. Revocation takes effect
immediately: every authenticated route rejects a revoked verifier with
`Unauthenticated`, and its open SSE/WSS connections close within one liveness
period. Rotation is pair-then-revoke.

Android may supply only `profile`, bounded objective, and cwd as launch choices.
It cannot supply executable, home, tmux target, App Server endpoint, sandbox,
approval, provider, model, thread id, or initial prompt.

Closed errors:

```text
Unauthenticated | PairingInvalid | ProtocolMismatch | InvalidRequest |
AgentNotFound | AgentClosed | AgentWorking | AgentNotAttachable |
WorkingDirectoryInvalid | WorkingDirectoryUnavailable | ProfileUnavailable |
ExactThreadMissing | CommandConflict |
RecoveryRequired | CursorInvalid | ResyncRequired | LivenessUnverifiable
```

Every closed error has one named trigger. `LivenessUnverifiable` is Close or
Reopen admission failing to verify binding liveness. `CursorInvalid` is a
malformed, filter-mismatched, or beyond-high-water cursor — a client defect
signal, never silence. `ResyncRequired` is a well-formed SSE cursor expired or
overtaken by queue overflow and instructs bootstrap. Transient internal failure
is retried internally until success or defect; no generic transient code
exists. `api/skidbladnir.v1.json` carries the one authoritative code-to-HTTP-status
mapping; both generated clients derive from it.

Unknown internal state, corrupt storage, impossible process identity, or unknown
hook/protocol shape is a defect with correlation handle, never empty success.

### SSE and terminal WSS

Bootstrap projection and `snapshotFactCursor` come from one SQLite read
transaction. Fact fan-out to subscriber queues happens only after the owning
transaction commits, serialized in commit order and mutually ordered with
subscriber registration; enqueueing an uncommitted Fact is a defect. Register
SSE subscriber before catch-up query; merge queued/stored Facts by sequence.
Each subscriber stream emits strictly contiguous ascending sequence — a gapped
queue observation reads the missing committed range from SQLite before
emitting — so `Last-Event-ID` resume is gap-free within retention. Only durable
Facts have ids. Expired cursor or queue overflow requires bootstrap. SSE
carries no terminal or transcript data. `/v1/events` emits a comment heartbeat
and the terminal WSS uses ping/pong on one named interval; a peer missing a
named count of heartbeats is closed and reconnects.

Kotlin owns WSS/auth. Terminal assets load through `WebViewAssetLoader`;
`shouldInterceptRequest` denies every non-asset request; file access, content
access, and JavaScript file/universal access are off; `setBlockNetworkLoads`
is on; page CSP is `default-src 'none'` plus the packaged bundle. Packaged
xterm.js receives no bearer. Kotlin exchanges bounded terminal data with it
through `WebMessagePort`; no `addJavascriptInterface`, no general JavaScript
bridge.

Section 5's "without interpretation" covers gateway and PTY bridge; the client
interprets. The pinned emulator's posture is fixed: no clipboard addon, so no
OSC 52 write; `windowOptions` unset, so no window or title report;
agent-supplied titles never render in native chrome. Paste is bracket-safe —
strip `ESC` bytes and C0 controls except `\n`/`\t` (CR normalized) before
framing, so an embedded `ESC[201~` cannot close bracketed paste early and
auto-send. The pinned build's automatic replies (DA1, DA2, DSR, CPR) are
client-originated input, the only exemption to "nothing auto-sends", and are
enumerated in Lane 0's evidence.

WSS text frames are `Hello | Presence | Resize | Detach | Error`; binary client
frames are input; binary server frames are PTY output. `Hello`/`Presence`
expose only attached-client count and `Owner | Constrained` geometry mode; they
are not durable Facts. The count spans the registered source session's whole
group via `list-clients`, classifying gateway-owned shadow clients apart from
non-Skíðblaðnir clients; the non-Skíðblaðnir count drives geometry mode and presence —
single-session `session_attached` is never sufficient. Each WSS connection owns
one gateway PTY, tmux client, and ephemeral shadow session, created at upgrade
and torn down on any close — `Detach`, slow-client disconnect, or abnormal
loss — always under the last-link guard; reconciliation destroys orphaned
shadows. Phone geometry precedes attachment: the gateway sets the shadow PTY
winsize from the phone's initial geometry before attaching, and tmux's
attach-time repaint of the current screen is the phone's only initial
content — the gateway buffers and replays no bytes. Resize always updates the
phone viewport; `ignore-size` prevents it from resizing the shared pane while a
laptop client is present. No output history or input byte is replayed; a slow
client disconnects and reattaches fresh. The bridge verifies exact
PID/start-time/TTY/pane identity on the data path — before forwarding each
input frame and on each output flush — and watches exact-PID exit; identity
failure drops the frame and closes the bridge.

Hard bounds: HTTP body 64 KiB; objective 240 Unicode scalars; cwd 4,096 UTF-8
bytes; page 100; history 300; SSE queue 256 Facts/1 MiB; broker JSONL frame 1
MiB; terminal frame 64 KiB; terminal queue 1 MiB; geometry 20-240 columns and
5-120 rows; projected hook 16 KiB; unary handler 30 s (`/v1/events` and
`/v1/terminal/{handle}` are bounded by queues and liveness instead). These are
transport bounds, not agent quotas.

Objective is at most 240 Unicode scalars after NFC normalization, with no C0 or
C1 controls, no line or paragraph separators, and no bidi overrides; the same
parse serves the Android field and the `UserPromptSubmit` projection, and
line-oriented `todo.txt` depends on it. Wire enums are PascalCase — card states
`Starting | Idle | Working | Closing | Recoverable | Exception | Unknown |
Closed`, pressure `Ok | Warm | Hot | Unknown`, geometry `Owner | Constrained`,
Fact kinds, WSS frames — while profile values keep the external spellings
`personal | work | work2` naming real commands and homes. Uppercase forms in
this document are UI copy or quoted observation, not wire values.

## 7. Product and operations

### Android surface

- Compile/target/min SDK 36 after physical build preflight. One manually
  installed package for Core.
- `+`, labeled `Create new agent` for accessibility, opens **The Forge**. This
  Start sheet contains profile, objective, and editable cwd defaulted to the
  first server-supplied recent, with up to eight server-derived recents as
  one-tap suggestions; allow type/paste, not filesystem browsing. The cwd field
  is single-line URI-class input with autocapitalization, autocorrect,
  suggestion substitution, and smart punctuation disabled; paste preserves
  bytes exactly. Validation errors keep the sheet and input intact; its submit
  action is labeled `Start agent`. Character assignment is automatic; Core has
  no character field, picker, or reroll.
- **Hlíðskjálf** is the adaptive virtualized two-column agent and host overview;
  it uses one column at large font. Card:
  Dvergatal display name and bundled portrait, handle, profile, state,
  objective, safe activity, age, attention. Name/icon/color never carries state
  or identity alone. Grid order is a stable total order: unacknowledged
  attention first, then state rank
  (`WORKING`; `STARTING`/`CLOSING`; `EXCEPTION`/`UNKNOWN`/`RECOVERABLE`;
  `IDLE`), then last observation descending, then handle. `CLOSED` cards are
  excluded from the default grid behind an explicit `Closed` filter.
- Every non-nominal card carries a one-line typed reason under its state:
  `EXCEPTION`, `UNKNOWN`, `RECOVERABLE`, and `CLOSING` render the closed
  derivation reason. A `202` command outcome resolves visibly on its card as
  success or the typed failure. One exhaustive `agentErrorMessage` helper at
  the screen boundary maps every closed error variant to UI text; an unmapped
  variant fails loudly.
- Card tap opens the terminal when the card is attachable; otherwise it opens
  the agent sheet. Long-press and an explicit overflow control open the agent
  sheet from any card: dwarf name, handle, profile, exact refs, state with
  reason and correlation handle, attention, last activity, and the closed
  action set `Open | Close | Reopen | Acknowledge`. Close is offered only for
  `IDLE`; elsewhere it renders disabled with its reason. Reopen is offered for
  `RECOVERABLE`, `CLOSED`, and `EXCEPTION` holding exact refs and opens the
  terminal once a live runtime-binding Fact commits; a missing exact thread
  renders `ExactThreadMissing` and offers no Reopen. Acknowledge clears the
  mark on Fact commit. No action is bound to swipe.
- Top strip is at most 48 dp: overall `OK | WARM | HOT | UNKNOWN`, every active
  reason (for `UNKNOWN`, the missing signal names), the unacknowledged-attention
  count that jumps the grid to the first such card, and client link state
  `LIVE | RECONNECTING | STALE`. Expanding it shows CPU, memory, swap,
  normalized load, distinct-filesystem disk, and the three PSI series, each
  with a one-hour sparkline of at most 300 points from `GET /v1/host/samples`.
  No reason collapses into a color; pressure never blocks Start.
- On SSE loss the client reconnects by cursor and falls back to bootstrap;
  while not `LIVE`, every card and the pressure strip render stale-marked with
  the age of the last Fact that produced them, and no element asserts a state
  newer than its evidence. `CursorInvalid`/`ResyncRequired` force bootstrap and
  clear the mark on success.
- Detail is full-screen terminal, not transcript. Accessory row is one
  horizontally scrollable row of `>= 48 dp` keys:
  `Agents | Esc | Ctrl-C | Tab | Left | Right | Up | Down | Home | End |
  Newline | Detach`. `Newline` sends the pinned TUI's
  newline-without-submit sequence; `Esc` long-press sends `Esc Esc`. Keys are
  built from the Lane 0 key-sequence record, never guessed. `Agents`,
  `Detach`, and the back gesture are one exit: each returns to the grid and
  destroys the phone's terminal client and shadow session; backgrounding and
  process recreation destroy them too. No phone attachment outlives the
  terminal screen; reopening a card creates a new shadow, and teardown never
  changes another client's presence, geometry ownership, or pane.
- Terminal chrome shows other attached-device presence and names the geometry
  mode: `OWNER` is silent; `CONSTRAINED` shows one persistent line stating the
  view is a cursor-following viewport onto a pane whose geometry another client
  owns, and that rotation and pinch/font zoom reveal more cells without
  reflowing the shared pane. Terminal history is read by forwarding the pinned
  TUI's own scroll keys and vertical touch drag mapped to them; the phone never
  enters tmux copy-mode — pane modes are shared with every client — and never
  sends wheel events a `mouse on` server would turn into copy-mode. If Lane 0
  proves the TUI owns no scrollback, chrome offers a read-only `History` view
  from a server-side non-mutating pane capture that changes no pane mode and
  feeds no status derivation.
- System IME owns typing, clipboard, emoji/GIF, settings, voice. Core requests no
  microphone permission and has no `SpeechRecognizer`. Dictation remains
  editable and never auto-sends.
- Near-black tonal surfaces, warm high-contrast type, restrained luminous-thread
  accent. Avatars are landmarks, not status. No decorative motion.
- Minimum 48 dp targets. Grid, start sheet, agent sheet, top strip, terminal
  chrome, and accessory row are fully TalkBack-labeled and Switch-Access
  reachable; each card announces dwarf name, handle, profile, state, reason,
  attention, and age as one node. The terminal surface is one labeled region
  running xterm.js screen-reader mode announcing the active line;
  character-level TalkBack navigation inside terminal content is a recorded
  limitation, not a claimed capability. Physical tests cover edge-to-edge, IME
  resize, both navigation modes, 200% scale, TalkBack, Switch Access, rotation,
  and process recreation.
- Cache only disposable card/metric projections. Terminal scrollback stays in
  memory. The objective is prompt-derived with declared handling: persisted in
  SQLite, projected in Facts, cards, and `todo.txt`, cached on the phone,
  redacted from logs. No other prompt text, assistant text, tool content, or
  terminal byte enters saved state, logs, backups, or analytics.
  `android:allowBackup` is `false`, so cached projections never leave the
  device; release builds disable WebView contents debugging, and wireless
  debugging is a build/preflight affordance, not a runtime posture.

### Host pressure

One supervised `/proc` sample every five seconds records CPU delta, normalized
one-minute load, memory, swap, distinct-filesystem disk, and CPU/memory/I/O PSI.
Missing required signals yield `UNKNOWN`.

The same supervised scheduler owns liveness and summary cadence with named
constants (`justify-polling`): pane/PID/TTY liveness every two seconds per live
runtime binding and at attach/close/reopen admission; summary
`thread/read(includeTurns=false)` only on gap marker, hook/summary conflict,
card open, an unclosed turn past the staleness bound, or reconciliation — never
a per-agent steady timer. No `/v1` request path probes on demand; requests read
the last sampled facts.

WARM/HOT thresholds: memory available `15%/8%`; disk `15%/5%`; normalized load
`1.0/2.0`; CPU PSI `some avg60 20%/50%`; memory/I/O PSI `full avg60 1%/5%`.
Escalate immediately; de-escalate after 60 seconds below prior threshold. Show
all reasons. No Prometheus/Grafana, alerts, attribution, throttling, or gating.

### Security and lifecycle

- `skidbladnir pair` prints one short-lived one-time secret/QR; operator scrollback
  holds it only until it is used or expires. Successful pairing replaces:
  register the new bearer, revoke every prior phone installation in one
  transaction, close their SSE/WSS. `skidbladnir pair --revoke` revokes without
  replacement. Bearers, Codex auth, and signing material never enter repo/logs.
- Loopback plus Tailscale Serve plus bearer are all required. Android rejects
  cleartext/unapproved hosts; WebView loads packaged assets only.
- Logs contain handles, state, timings, counts, correlations; redact objectives,
  prompts, terminal/tool content, tokens, auth, account metadata.
- UI always shows `YOLO` and selected profile; it never implies sandboxing.
- Upgrades are idle maintenance with all agents idle: close ingress; stop the
  gateway unit, every tracked TUI, and all three App Server daemons, confirming
  no process holds any `CODEX_HOME`; snapshot SQLite via `VACUUM INTO` plus a
  permission-preserving archive of each `CODEX_HOME`, the installed Codex
  binary, `schemas/codex/<pinned-version>/`, and `codex.lock`, under a digest
  manifest; install/generate/probe the new pin; reconcile; run
  `./scripts/test accept upgrade`. Any failure restores every manifest artifact
  and re-probes before ingress reopens. Each pin change first rehearses the
  restore path once: snapshot, forced probe failure, restore, `accept upgrade`
  PASS. The snapshot lives under `$XDG_STATE_HOME/skidbladnir/snapshots/<pin>` mode
  `0700`, never leaves the host, and is deleted once acceptance passes. Host
  tmux upgrades take the same path — probe grouped
  sessions/`active-pane`/`ignore-size`, re-run the handoff gate, record the new
  proven version. No fallback.

### Future runtime seam

Do not create a v1 runtime interface. A selected second runtime must first prove:

```text
launch interactive client; recover exact conversation identity;
observe work/idle/end without screen scraping; attach terminal bytes;
stop/detach honestly; isolate account credentials
```

Then add one explicit discriminator and adapter by hard cutover. Missing
capabilities remain explicit; no runtime/account fallback. Evaluate
`llm-calling` only against that concrete need.

## 8. Target files and work split

```text
api/skidbladnir.v1.json
catalog/characters.json
generated/api/{go,kotlin}/
schemas/codex/<pinned-version>/
codex.lock
cmd/{skidbladnir,skidbladnir-hook}/
internal/{registry,storage,commands,events,hooks}/
internal/runtime/{appserver,codex,tmux,terminal}/
internal/gateway/{api,auth,sync}/
internal/metrics/
migrations/
android/app/src/main/
android/app/src/main/assets/terminal/
android/app/src/main/res/drawable-nodpi/dwarf_*.webp
tests/{fixtures,integration,system,e2e}/
evidence/{sources,live}/
scripts/{build,test,install-devbox}
deploy/{systemd,tailscale,codex}/
.github/workflows/verify.yml
go.{mod,sum}
android/{gradlew,gradle/,settings.gradle.kts,build.gradle.kts}
```

Go/Go modules are primary. Gradle owns only Android and is called by repository
scripts. Use standard Go HTTP, explicit composition, one SQLite driver. No web
framework, ORM, DI container, queue, broker infrastructure, Makefile, or parallel
build/test surface.

Lane 0 owns the justification-token grammar required by `errors.md`,
`retries.md`, `polling.md`, and `overrides.md` — `justify-defect`,
`justify-ignore-error`, `justify-service-invariant-check`, `justify-polling`,
`justify-retry-schedule`, and the override/dead-code/assertion tokens — as Go
and Kotlin comment markers; `static` fails on any suppression without its token
or a token with empty justification.

| Lane | Owns | First red proof |
| --- | --- | --- |
| 0 Contract/build/evidence | API, Dvergatal data/validation, generated DTO/schema, pin, scripts, workflow, evidence | Contract/catalog drift, mutable binary, or wrong command inventory fails |
| 1 Registry/storage/hooks | Registry, character assignment, commands, facts, hooks, migrations | Assignment is atomic/stable; event sequences derive one truthful state; replay keys are exact |
| 2 Runtime/terminal | App Server adapter, launcher, tmux, PTY | Cwd validation, exact allocation, shadow attach/detach, close/reopen |
| 3 Gateway/metrics | API/auth/sync/metrics | Real HTTP+SQLite+SSE/WSS, bounds, pressure |
| 4 Android | `android/**` | Bundled portraits; real-runtime grid, terminal, IME, keys, accessibility |
| 5 Composition | Entrypoint, deploy, tests/{system,e2e} | Real phone/devbox laptop-phone handoff |

Lane 0 is serial; 1-4 may follow; 5 composes. Shared contract changes return to
Lane 0. No lane edits another lane's paths.

## 9. Red/green/refactor and acceptance

For each behavior: write the behavior-level failure (**red**), implement only
enough (**green**), then remove duplication, speculative options, internal mocks,
and test seams without behavior change (**refactor**).

Use real SQLite and all repo-owned services. Tmux, Codex/App Server, Android,
Tailscale, and kernel are external/platform boundaries. Deterministic tiers may
use protocol-faithful boundary fixtures; only live/device gates claim the real
integration. `NOT_RUN` is not pass.

Target commands:

```text
./scripts/test static         # format/lint/type/dependency/generated/docs/workflow/justification-tokens
./scripts/test unit           # pure; no I/O
./scripts/test integration    # real SQLite/services
./scripts/test component      # real Compose/WebView runtime
./scripts/test system         # gateway + SQLite + real tmux + external fixtures
./scripts/test platform       # connected Android instrumentation
./scripts/test live           # pinned profiles/devbox/Tailscale
./scripts/test e2e            # real devbox gateway + pinned Codex/tmux + physical S22+ journeys
./scripts/test verify         # static + build + unit + integration + component + system
./scripts/test full           # verify + platform + live + e2e
./scripts/test accept core    # machine-readable; every required gate PASS
./scripts/test accept upgrade # deterministic tiers + live per-profile probe; no phone
```

Required gates are static, unit, integration, component, system, platform,
live, and e2e; `verify` and `full` are compositions, not separately required.
`accept core` aggregates exactly the required gates plus every proof-table row,
emitting one machine-readable result per gate; any `FAIL` or `NOT_RUN` keeps
Core unaccepted. `accept upgrade` runs the deterministic tiers plus a live
per-profile initialize/version/schema-digest check and one empty
`thread/start -> unsubscribe -> resume -> stop` round trip; full Core
acceptance with the physical device remains a release gate, not an upgrade
gate.

### One proof per ownership boundary

| Boundary | Authoritative proof |
| --- | --- |
| Existing command -> profile | Live `personal/work/work2`: exact home/pin, Unix remote TUI, no fallback |
| Broker -> App Server | Generated-schema fixtures + live start/unsubscribe/list/read; reject all content/turn methods |
| Hooks -> registry | Real hook JSON + SQLite/SSE: bind, objective, work/activity/idle/end, gap/unknown; `Stop`/exception raise attention exactly once, acknowledge is idempotent and re-arms on later events, unacknowledged attention survives restart |
| Lifecycle -> tmux/process | Real tmux + controllable TUI: cwd, idempotent start, exact identity, idle close, shell cutoff, reopen |
| PTY/WSS -> Android | System/device: ANSI/UTF-8/IME/clipboard, viewport/size ownership, backpressure/reconnect/no replay |
| Laptop -> phone -> laptop | Live: same pane/PID/thread/draft; laptop focus/geometry/process unchanged; independent detach |
| SQLite Fact -> card | Real HTTP/SSE: atomic snapshot, catch-up/live race, dedupe, overflow resync; retention prune keeps and removes exactly per the stated windows without changing a derived state; expired cursor forces bootstrap |
| Dvergatal -> agent/card | Static catalogue/source/icon validation + real SQLite/API/Compose: atomic assignment, stable reopen/restart, exact name/portrait/handle, deterministic exhaustion without a cap |
| Kernel -> pressure | Real `/proc` persist/aggregate/render + synthetic threshold/missing/hysteresis |
| Bearer -> terminal authority | Pair race, rotation/revocation, wrong bearer, raw-target injection, off-Tailnet denial |
| Restart -> recovery | Kill gateway/PTY/tmux/App Server: live/recoverable/exception/unknown, never guess/replay |

This is the 80/20 shape: deterministic gates per change, one proof per crossing,
one expensive real phone/devbox flow. Do not test every internal helper matrix.

### Core acceptance

- Physical `SM-S906W` preflight records API/build, Tailscale, WebView, Gboard
  typing/dictation/clipboard, navigation modes, scale, accessibility.
- The installed app identifies as **Skíðblaðnir**; the agent and host overview
  is **Hlíðskjálf**; `+` is announced as `Create new agent`, opens **The Forge**,
  and submits through `Start agent`. Reserved product vocabulary appears in no
  Core route, schema, navigation item, or domain type.
- Dvergatal contains at least 100 validated entries from only Old Norse/Eddic,
  Tolkien, and Germanic/operatic sources, each with one unique bundled portrait;
  no `FairyTale` or `ModernMedia` family, missing asset, or orphaned asset
  exists.
- Phone displays at least 15 idle/live cards with distinct dwarf names and
  portraits and no configured cap; at least two real agents work concurrently;
  pressure remains advisory. Close/reopen, restart, and laptop discovery retain
  the assigned character. With 15 cards present, a completion raises the
  attention count and reaches its card in one tap; closed agents are absent from
  the default grid. Exhausting the catalogue still permits Start and preserves
  unique `ga-*` handles.
- Android Start works independently for all three profiles with exact isolated
  home, a manually entered existing cwd (including spaces and `~/`), empty exact
  thread, remote TUI, YOLO, and no substitution. Invalid, missing, relative, or
  non-directory cwd fails before thread allocation and preserves the form.
- Laptop start in arbitrary cwd appears automatically. While laptop mosh/tmux
  remains attached, phone opens the exact TUI without changing laptop session,
  window, pane, geometry, PID, thread, draft, or turn; phone Detach changes none
  of them.
- Gboard text/clipboard/reviewed dictation and accessory keys reach TUI;
  nothing auto-sends or replays after reconnect beyond the pinned emulator's
  enumerated automatic replies. A dictated prompt is corrected mid-line and
  extended to a second line before submission. Output above the fold is read
  from the phone with the laptop's pane mode, view, and geometry unchanged.
- Every declared card state arises only from declared hook/summary/liveness
  evidence. Terminal content never affects status. Each declared
  `EXCEPTION`/`UNKNOWN` cause and each asynchronous command failure renders its
  distinct typed reason on the phone; no command resolves silently.
- Working Close rejects. Idle Close stops exact TUI, exposes no returned shell,
  retains binding; Reopen reaches exact conversation. From the phone alone: an
  idle card closes and the same card reopens to the exact conversation, a
  `RECOVERABLE` card reopens, and an attention mark is acknowledged
  idempotently, clears, and re-arms on a later completion.
- Tailnet or SSE loss marks the grid and pressure strip stale with last-Fact
  age; the phone never renders an unmarked stale state. While the laptop is
  attached, terminal chrome states `CONSTRAINED` and its cause.
- Phone/Tailnet/WSS/gateway/tmux/App Server losses preserve declared recovery and
  input semantics; no prompt, bytes, profile, or agent is replayed/substituted.
- Current/history CPU, memory, swap, load, disk, PSI and reasons render;
  missing is `UNKNOWN`; HOT never blocks Start.
- TalkBack and Switch Access pass on every Compose surface at 200% with both
  navigation modes, edge-to-edge, IME, and rotation/process recreation on the
  target phone; the terminal region records its declared TalkBack limitation
  rather than a pass.
- API/schema/UI have no project/repo/worktree identity, workspace mode, quota,
  generic provider, transcript/composer, direct turn mutation, raw target, or
  phone-visible App Server surface. Host allowlist has no turn/content method or
  retained transcript subscription.
- Profile absence, pin/schema/config/hook drift, unknown protocol, and missing
  exact thread fail visibly. No fallback exists.
- `./scripts/test accept core` is `PASS`; any missing live gate is `NOT_RUN` and
  Core remains unaccepted.

## 10. Deferred v1.1+

- Wake: opaque FCM attention, redacted notification, doze/deep-link proof.
- Release: `dev.niels.skidbladnir.a/.b`, manual provisioning, signed immutable
  candidate, health/activation/rollback; signing/ADB outside YOLO UID.
- Native voice review only if Gboard fails real tests.
- Images, rich clipboard, terminal text copy-out (touch selection of terminal
  output), purge, off-host backup, full devbox restore.
- Second concrete runtime; then explicit adapter and `llm-calling` evaluation.
- Structured App Server client only if terminal is materially insufficient; it
  replaces terminal interaction and is never a fallback.
- Separate UID/VM hostile-agent containment.

Anything else requires a new acceptance criterion and explicit scope decision.
