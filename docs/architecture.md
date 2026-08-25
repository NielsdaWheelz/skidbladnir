# Skíðblaðnir Core: product and architecture

Status: reviewed implementation target. Evidence last checked: 2026-08-25.

Specification precedence:

1. This document owns product behavior, architecture, scope, trust, and
   acceptance.
2. [`docs/rules`](rules/index.md) owns implementation standards. Section 5's
   explicit Go composition is the narrower translation where a rule names a
   TypeScript/Effect mechanism; the underlying correctness obligation still
   binds.
3. [`roadmap.md`](roadmap.md) owns implementation order and PR boundaries. It
   may not waive this contract.

Resolve a conflict before implementation. A platform proof that contradicts a
premise reopens this document; it never authorizes a fallback.

## 1. Outcome

Skíðblaðnir is Niels's private Android command deck for Codex subscription
agents on one remote devbox. From a Galaxy S22+, Niels can:

- see a dense grid of live `ga-*` agents and current/history host pressure;
- start an agent under `personal`, `work`, or `work2` in any validated directory;
- open the exact same stock Codex TUI from Android and laptop tmux;
- type, paste, or dictate through Gboard;
- detach either device without stopping the TUI or its work;
- close an idle tracked TUI and reopen its latest confirmed conversation.

The phone is the cockpit, Codex owns conversation and execution, and tmux owns
the persistent terminal. Skíðblaðnir owns cards, tracked runtime identity,
attachment, lifecycle facts, attention, and host telemetry.

Core uses one ordinary pinned Codex TUI process per tracked runtime. The TUI
owns Codex's in-process session machinery; Skíðblaðnir uses no shared/external
App Server daemon, session transport, or broker. One bounded, read-only,
pre-registration `hooks/list` subprocess is the sole exception and owns no
runtime state. This is load-bearing: exact-pin evidence showed
that stock `/new`, `/clear`, and `/fork` replace the active thread inside one
TUI, while shared App Server broadcasts expose no initiating client identity.
A shared daemon therefore cannot assign the new thread to the correct card when
several TUIs use one profile. A per-runtime marker plus exact process ancestry
can.

The product stays terminal-first:

- preserve the stock Codex TUI rather than rebuilding chat;
- preserve mosh/tmux rather than creating a competing workflow;
- use authenticated hooks and exact tmux/process facts, never screen or
  transcript parsing, for agent state;
- track agents/runtimes; cwd is a launch input and fact, never a project;
- show `UNKNOWN` whenever evidence is insufficient;
- finish Core before wake notifications or self-update.

Wake (FCM) and Release (Kache-style A/B packages) are v1.1.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; ASCII namespace `skidbladnir`; private, one user, one phone, one devbox |
| Phone | Galaxy S22+ `SM-S906W`, Android 16/API 36; wireless debugging is a build affordance |
| Host | One Hetzner VPS; Linux, mosh, systemd user service, exact proven tmux 3.4 |
| Network | Tailscale Serve TLS to a loopback gateway; Funnel/public ingress forbidden |
| Profiles | Closed `personal | work | work2`; callers never supply `CODEX_HOME` |
| Runtime | One ordinary pinned Codex TUI per tracked runtime; no shared/external App Server daemon, session transport, or broker |
| Interaction | The stock TUI in tmux; no transcript, composer, or screen parsing |
| Android start | Explicit validated cwd; profile defaults; YOLO; no initial prompt |
| Laptop start | Current/effective cwd; reviewed interactive flags; YOLO |
| Workspace | Shared-live only; worktrees remain prompt-level instructions |
| Handoff | One TUI; independent tmux clients may attach concurrently |
| Capacity | No agent cap, scheduler, pressure gate, or one-writer-per-repo rule |
| Client | Kotlin/Compose dashboard; vendored pinned xterm.js terminal |
| Host app | Go, SQLite, tmux/PTY, hooks, `/proc` |
| Transport | Strict HTTPS JSON, durable SSE facts, authenticated WSS terminal |
| Trust | Codex is trusted as the devbox user; no hostile same-UID containment claim |

Profile mapping is closed:

| Profile | Existing command | Required `CODEX_HOME` |
| --- | --- | --- |
| `personal` | `codex-personal` | `/home/niels/.codex-personal` |
| `work` | `codex-work` | `/home/niels/.codex-work` |
| `work2` | `codex-work2` | `/home/niels/.codex-work2` |

The dev-server-owned `ai-router` selects this profile before Skíðblaðnir. Inside
tmux, root interactive starts and `resume` delegate to one fixed
Skíðblaðnir-launcher path when an owner-only enablement marker names the exact
seam version. Outside tmux and for every other Codex subcommand, the router
execs the exact profile-selected binary unchanged. With the marker present,
missing/incompatible delegation fails visibly; without it, dev-server remains
standalone. The seam is versioned and introspectable. Skíðblaðnir verifies it
but never edits router source or symlinks and never imports `llm-calling`.

### Product language

| Role | Product language | Contract |
| --- | --- | --- |
| Android app | **Skíðblaðnir** | Launcher label and accessibility name; code/paths/package use `skidbladnir` |
| Agent/host overview | **Hlíðskjálf** | Documentation motif; literal screen label is `Agents` |
| New-agent surface | **The Forge** | Sheet title `New agent`; `+` label `Create new agent`; action `Start agent` |
| Dwarf catalogue | **Dvergatal** | Append-only bundled names and portraits; documentation motif |
| Human source of intent | **Gloriana** | Concept only, not a type or screen |
| Future orchestrator | **Sūtradhāra** | Reserved; Core has no placeholder or abstraction |

Prospero, Solomon/the Seal, shabti, and Gwydion/*Cad Goddeu* remain reserved
until a concrete feature owns them. Domain, error, trust, and destructive-action
language stays literal.

### Capability contract

Core exposes one path per capability:

1. **Start** validates cwd, creates one card/runtime, and foreground-execs the
   pinned ordinary TUI. It never submits a prompt.
2. **Discover** routes supported future laptop invocations through the same
   launcher and inventories pre-existing pinned TUIs as attach-only `UNKNOWN`
   external cards.
3. **Observe** accepts authenticated lifecycle hooks plus exact process/tmux
   liveness. Terminal content is never evidence.
4. **Attach** bridges Android to the registered pane without replacing its TUI
   or changing another client's selection or geometry.
5. **Detach** closes only Android's client/shadow.
6. **Close** stops only a verified idle managed TUI and locally closes its card.
7. **Reopen** foreground-execs `codex resume <latest-confirmed-thread-id>`.
8. **Recover** reconciles durable facts and exact liveness; it never replays
   terminal input or guesses conversation identity.

### Goals

- Match public Kache behavior: dense `ga-*` cards, dwarf portraits,
  `WORKING`/`IDLE`, objective/activity, many agents, voice-driven mobile TUI,
  tmux continuity, and recovery.
- Surpass that known surface with explicit profile isolation, honest unknowns,
  typed failures, host-pressure history, and one proof per ownership boundary.
- Let phone and laptop slide into one live process, screen, draft, and turn.

### Non-goals

- Project enrollment, repo/worktree identity, filesystem browsing, workspace
  modes, quotas, scheduling, cgroups, shared memory, or multi-writer leases.
- Native transcript/chat, prompt/steer/interrupt APIs, token streaming, terminal
  parsing, transcript/SQLite state-file parsing, or App Server use beyond the
  exact one-shot read-only hook-catalog preflight.
- Generic runtimes/providers, fallback, SDK/embedded-Codex paths, or
  `llm-calling` integration.
- Raw shell, tmux target, executable, `CODEX_HOME`, or thread input from Android.
- Character personas, catalogue CRUD/fetch/picker/reroll, or character sources
  outside Old Norse/Eddic, Tolkien, and Germanic/operatic works.
- Core: FCM, A/B update, terminal image transfer, native speech/TTS, permanent
  purge, or off-host disaster recovery.

## 3. Evidence and decisions

Claims are **observed**, **platform fact**, or **Skíðblaðnir decision**.
Similarity is not attribution.

| Evidence | Supports | Does not prove |
| --- | --- | --- |
| Kache [agent grid](https://x.com/yacineMTB/status/2090580615049408711) | Dense two-column `ga-*` cards, character art, objective/activity, `WORKING`/`IDLE`, attention | Private backend or durability |
| Kache [mobile TUI](https://pbs.twimg.com/media/HQVGPaeXQAAARj3?format=jpg&name=medium) | Stock-looking Codex TUI on Android | Its transport implementation |
| Kache [A/B post](https://x.com/yacineMTB/status/2090436678519312650) | Two installed variants intended to preserve update access | Signing/rollback details |
| Kache [recovery screenshot](https://x.com/yacineMTB/status/2090443641697288552) | Proposed `ga-*`, tmux, saved-session, `todo.txt` inventory | That recovery ran |
| [Codex hooks](https://learn.chatgpt.com/docs/hooks) | Session, prompt, tool, stop, subagent, and end events | Stable transcript contents; those are explicitly not promised |

The image's bottom row is Gboard: toolbar, emoji/stickers/GIF, Writing Tools,
clipboard, settings, and microphone. Core uses the installed IME rather than
copying that row. The observed `whitebox` label has no public semantic contract;
Skíðblaðnir shows `CODEX · profile` and never copies Kache branding.

Exact 0.149.1 source, binary, and live probes established:

- a stock TUI uses the normal terminal buffer and can be shared by tmux clients;
- `SessionStart` occurs immediately before the first `UserPromptSubmit` for a
  selected root, not merely on TUI attach;
- canonical rollout basenames encode the root thread UUID without reading the
  transcript; `session_id` corroborates the hook session but is not a resume
  key;
- public forks get a fresh thread/session, but their hook payload is
  observationally identical to `/new` (`source=startup`, no parent field);
  native subagents run in the root TUI process, use a child thread id plus the
  root session id, and carry explicit subagent discrimination;
- `/new`, `/clear`, and `/fork` can replace the selected root inside one stock
  TUI process; old roots remain loaded until graceful TUI shutdown;
- active Escape interrupts a turn without `Stop`, after which the same root
  accepts an immediate next prompt; the interrupted loopback HTTP stream is not
  transport-cancelled at this pin even after the TUI returns to Ready;
- graceful shutdown emits one `SessionEnd` for every loaded root, unordered
  across roots, including an empty replacement root that never emitted
  `SessionStart`; SIGKILL emits neither `Stop` nor `SessionEnd`;
- an enabled untrusted hook is excluded from the runnable hook engine but stops
  ordinary TUI startup at an interactive review screen; a tracked launch must
  reject that universe before registration rather than wait for user review;
- shared App Server notifications expose no initiating TUI/client identity;
- tmux 3.4 grouped sessions share panes/processes while clients retain
  independent current-window context; `active-pane` and `ignore-size` are
  required for phone attachment.

Therefore:

| Question | Decision | Why |
| --- | --- | --- |
| TUI or structured chat? | Stock ordinary TUI | It already owns tools, approvals, rendering, and slash commands |
| Runtime owner? | One TUI process per card/runtime | The runtime marker authenticates conversation rollover to exactly one card |
| App Server integration? | Hook-catalog preflight only | A bounded one-shot `hooks/list` call prevents an interactive trust stop; no App Server process owns a card, session, transport, or lifecycle fact |
| Handoff? | Grouped tmux shadow client | It preserves one process, screen, and unsent draft |
| Status truth? | Hooks + exact liveness | Semantic and content-free; unknown stays explicit |
| Android terminal? | Pinned local xterm.js | Mature VT/IME behavior while Kotlin retains auth/network |
| Voice/clipboard? | Gboard/system IME | Ordinary editable terminal input, no second composer |
| Close? | Idle exact-process stop + local close | History remains in Codex; no archive/delete |
| Second runtime? | Deferred | A concrete second CLI must first prove the capability contract |

### Lane 0 exact-pin proofs

P0 proves only raw platform mechanisms on the committed versions. Registry,
card projection, lifecycle reduction, command recovery, and SQLite do not exist
yet; their behavior belongs to P1/P2 and cannot be claimed by a P0 probe.

| Proof id | Gate | Exact P0 claim |
| --- | --- | --- |
| `p0-codex-pin` | `static` | Binary/source/tag/digest, root/resume grammar, reviewed hook schemas/config/helper, and generated contracts are immutable and reproducible |
| `p0-profile-direct-tui` | `live` | Each profile starts ordinary `codex` and resumes a canonical UUID with exact home/cwd/config/YOLO in one foreground process; no shared/external daemon, remote transport, or broker is started |
| `p0-hook-origin` | `live` | Exact production handler hashes match the frozen three-profile trust lock; a foreign untrusted handler is catalogued and blocks for review; `SKIDBLADNIR_RUNTIME_ID` reaches every reviewed hook; nearest exact pinned-Codex ancestor PID/start/TTY matches the invoking TUI; inherited nested-CLI traffic is rejected |
| `p0-hook-identity` | `live` | Raw hook payloads establish basename thread id, session corroboration, fresh revert/public-fork identities, native-subagent discrimination, and the event/PID sequence for `/new`, `/clear`, and `/fork`, without claiming hooks distinguish `/fork` from `/new` |
| `p0-tui-lifecycle` | `live` | Raw `Stop`, active-Escape interruption and same-root continuation, unordered loaded-root `SessionEnd`, pane/PID death, ordinary `/quit`, idle Ctrl-C, and killed-work behavior are recorded |
| `p0-tmux-handoff` | `live` | Grouped shadow attach/detach, client-context targeting, last-link guard, repaint, focus, input routing, `active-pane`, and `ignore-size` match Section 4 |
| `p0-tui-keys` | `live` | Stock TUI intra-line edit, exact raw Ctrl-J (`0x0a`) newline-without-submit, raw CR (`0x0d`) submit, history, scroll, and normal-buffer behavior are recorded |
| `p0-android-terminal` | `platform` | Exact S22+ WebView/xterm.js/Gboard proves ANSI, Unicode, composition, editable dictation, clipboard, automatic DA/DSR/CPR replies, resize, and rotation |

The P0 hook-identity record also identifies whether native subagents execute in
the registered process or a child. The later reducer must accept only root
lifecycle authority and project authenticated native-subagent observations as
activity-only. P0 inventory evidence records the exact tmux/`/proc` predicate
for an unmarked TUI; P1 materializes external cards. P0 does not prove atomic
card rollover, Stop settlement/attention, restart/gap repair, resume arbitration,
or external-card behavior.

Failure changes this design or remains `NOT_RUN`; it never enables a fallback.

## 4. Target behavior

### Android Start

1. `+` opens `New agent` with profile, short objective, and editable cwd. Cwd
   defaults to the first server recent for that profile, initially
   `/home/niels/src`; at most eight server recents are suggestions.
2. `StartAgent` accepts no prompt, model, runtime, thread, character, or target.
3. Cwd parses once: at most 4,096 valid UTF-8 bytes; no NUL/C0/C1; optional
   leading `~`/`~/` resolves against the service UID home; cleaned; absolute;
   existing directory enterable by the service UID; input must already equal
   the resulting canonical string. Symlinks follow kernel resolution. Failure
   before mutation is `WorkingDirectoryInvalid`; a later enter failure settles
   `WorkingDirectoryUnavailable`.
4. One durable Start creates a `STARTING` card and deterministic gateway-owned
   tmux runtime, sets a fresh runtime id, registers exact tmux/PID/start/TTY,
   and foreground-execs the pinned ordinary Codex TUI with profile home, cwd,
   reviewed hooks, strict config, and YOLO. It sends no prompt.
5. Exact live process registration settles Start and publishes `IDLE`; the card
   is attachable with a null Codex binding until its first root turn.
6. Niels types/pastes/dictates in the stock TUI. `SessionStart` immediately
   followed by `UserPromptSubmit` establishes the first exact Codex binding and
   `WORKING` state.

Cwd is execution context, not identity or a sandbox. YOLO grants the same
filesystem authority as the devbox user. Start has no provider write before the
TUI itself creates its conversation; deterministic tmux/runtime identity makes
crash replay local and discoverable.

### Laptop Start and Resume

The natural path remains:

```text
mosh devbox
tmux attach ...
cd any-directory
codex-personal | codex-work | codex-work2
```

For a root invocation, the router selects the profile and delegates to the
launcher. The launcher completes all read-only validation, including the exact
effective-hook preflight, before `PrepareNew`. The gateway then creates the
card/character and registers the invoking tmux session/window/pane, TTY,
launcher PID, and kernel start time. The launcher execs the exact pinned binary
in place; exec preserves PID. It creates no tmux session, renames nothing,
sends no prompt, and never returns a shell to the phone.

The launcher accepts the exact pin's reviewed root/resume grammar and forwards
only flags that do not override profile, executable, transport, hook config,
approval, sandbox, or runtime identity. It validates `-C/--cd`; injects YOLO;
and preserves supported prompt/image/model and other non-conflicting
interactive flags. A conflicting or unknown-at-the-pin option fails before
registration or exec. `--remote`, caller-supplied `CODEX_HOME`, arbitrary
`-c/--config`, approval, sandbox, and hook-trust bypasses are forbidden.
Noninteractive and outside-tmux commands retain ordinary router behavior.

Resume has one local flow:

- `resume <exact UUID>` accepts only a canonical UUID: parse and re-render the
  lowercase `8-4-4-4-12` form before any lookup or exec. An arbitrary Codex
  `SESSION_ID`/session name is rejected, never forwarded. The UUID first
  resolves a current Skíðblaðnir binding. Live
  means switch exactly the invoking non-gateway tmux client to the registered
  session/window/pane; dead means reopen the same card in the invoking pane;
  no match creates a new card with that pending thread ref and execs ordinary
  `codex resume <UUID>`. No prefix/fuzzy/session-id resolution exists.
- bare `resume` shows only the bounded local Skíðblaðnir card picker. Selecting
  a live card switches it; selecting a dormant card reopens it. Importing an
  older untracked conversation requires its exact UUID; Core never lists or
  parses Codex history.
- zero or multiple candidate laptop clients never trigger an implicit switch;
  print the exact tmux target and handle. Client-context-less `switch-client`
  is forbidden.

A supplied pending ref is stored separately from a confirmed binding. Every
root `SessionStart` is gated by it until confirmation: the parsed UUID must
equal the pending ref. Mismatch derives `EXCEPTION` and stops accepting input;
the generic rollover rule cannot bypass this gate. A direct fresh start has no
pending ref.

### Conversation rollover

The card follows the tracked TUI, not a forever-fixed conversation. Within one
authenticated TUI process:

- when no pending resume remains, the first root `SessionStart` creates the
  active binding;
- a later root `SessionStart` with a new thread id (including `/new`, `/clear`,
  or `/fork`) atomically replaces that card's active binding and session id;
- the replacement closes any prior open turn without Idle/attention, because
  the new root prompt is continuation evidence;
- the previous thread remains ordinary Codex history but is no longer owned or
  reopened by that card;
- the card keeps handle, character, objective, runtime, and tmux attachment.

Because this pin emits `SessionStart` immediately before the new root's first
prompt, closing after selecting a new empty root but before typing reopens the
last confirmed conversation. No submitted work exists in the unconfirmed root.
Native subagents never cause rollover.

### Pre-existing laptop TUIs

At startup, inventory tmux plus `/proc` for an exact pinned Codex process under a
recognized profile home that has no registered runtime id. A candidate must
have the pane TTY and be either its exact pane-root process or the unique Codex
process group that owns that terminal's foreground process group; nested/child
Codex descendants and panes with ambiguous candidates are excluded. Persist one
attach-only external card per proven pane/PID/start tuple. It has null Codex
binding, state `UNKNOWN`, reason `external-runtime-untracked`, and an assigned
handle/character. Android may attach to its exact terminal, but Close/Reopen and
semantic state are unavailable. Ending it and starting/resuming through the
launcher creates a normal managed card. Skíðblaðnir never reads its screen,
transcript, SQLite, title, or inferred current thread.

### Phone/laptop handoff

- Opening a card starts no second TUI and asks no laptop client to detach.
- The gateway creates an ephemeral tmux session grouped with the registered
  source, then attaches one gateway-owned phone PTY to that shadow.
- The phone client keeps `active-pane` and `ignore-size`. Initial targeting is
  one client-context invocation; later targeting uses the gateway-owned phone
  client context. Never mutate another client's selection.
- Effective global `window-size latest` is readiness-checked. With an unflagged
  laptop client present, phone resize changes only its viewport and reports
  `CONSTRAINED`; when the phone is the only client it owns sizing and reports
  `OWNER`.
- Both devices see/type into the same process, screen, draft, turn, and active
  binding. Inputs may interleave; one human is expected to type at a time.
- Detach first closes the phone client, then destroys its shadow through one
  in-server last-link guard. If it is the final group link, retain/promote it so
  the shared pane is never accidentally killed. Reconcile exact pane/TTY/PID
  after cleanup.

### Close, Reopen, and recovery

- While `WORKING`, Close is disabled. Ctrl-C is ordinary TUI input.
- Accepting Close atomically enters `CLOSING`, rejects new attachments, pins
  the latest managed runtime identity, closes Android shadows, re-verifies exact
  liveness and settled-idle facts, then terminates only that TUI. Any uncertainty
  is `LivenessUnverifiable`; a newer turn is `AgentWorking`.
- A launcher-origin close kills only the exact Codex PID; the user's pane returns
  to its shell after every phone bridge has already closed. A gateway-origin
  TUI is the pane root, so its pane ends. External cards cannot Close.
- Local closure preserves the latest confirmed Codex ref. Reopen is offered only
  when that ref exists and every prior binding is verified dead. It creates a
  fresh runtime and execs ordinary `codex resume <exact-thread-id>`.
- Phone/WSS/mosh loss affects only that client. Gateway restart reconstructs
  owned runtime facts from SQLite, tmux, `/proc`, and hook gap files; no input is
  replayed.
- Runtime death after a settled turn is `RECOVERABLE`; death with an unclosed
  turn is `EXCEPTION`; death before any confirmed ref is `EXCEPTION` and cannot
  Reopen; absent/stale/contradictory evidence is `UNKNOWN`.
- `$XDG_STATE_HOME/skidbladnir/todo.txt` is an awaited atomic `0600` projection
  of handle, profile, latest confirmed ref, tmux target, objective, and state.
  SQLite is authority; code never reads `todo.txt`.

## 5. Architecture and ownership

```text
                         laptop / mosh
                               |
                               v
                       registered tmux pane
                               ^
                    grouped phone shadow client
                               ^
Galaxy S22+                   PTY/WSS
  Gboard -> local xterm.js      |
             |                  |
  Compose dashboard             |
             | HTTPS/SSE/WSS    |
             v                  |
        Go gateway -------------+
          |   |   |
          |   |   `-- exact tmux + /proc liveness
          |   `------ SQLite + hook/control sockets
          `---------- pinned ordinary Codex TUI per runtime
```

Android never reaches Codex directly. Terminal bytes travel between the exact
tmux PTY and Android without gateway interpretation. Hooks carry the only
conversation semantics retained by Skíðblaðnir.

### Composition

- One systemd user service owns gateway, SQLite, hook/control sockets,
  lifecycle reconciliation, tmux shadows/PTYs, SSE, and metrics. It supervises
  every background task; required-owner failure closes ingress and restarts the
  composition.
- Tracked Codex TUIs are independent child/runtime processes, not gateway
  children that must die with the service. Their durable registration and tmux
  ownership survive gateway restart.
- Bind loopback. Tailscale Serve exposes only `/v1`; Funnel is forbidden.
  `/healthz` and `/readyz` remain loopback-only.
- `/healthz` proves process life. `/readyz` proves configuration, migrations,
  SQLite pragmas, hook/helper/pin/router digests, sockets, Serve, and the exact
  recorded tmux behavior/version. Profile health independently proves expected
  `CODEX_HOME`, readable auth/config state, installed hook config, and pinned
  executable. It never reads or logs account identity.
- Liveness sampling is every two seconds for live bindings and fresh at
  terminal/close/reopen admission. Host pressure samples every five seconds.
  The one-second post-Stop settle schedule is event-triggered and self-ending.

### Launcher and hook origin

Installation places one reviewed user-level hook file in each closed profile
home, persists Codex trust for those exact handlers, installs one reproducible
`skidbladnir-hook`, and records their digests. Immediately before exec, the
launcher verifies its absolute pinned binary, router seam, profile home, hook
file, trust entries for the reviewed handlers, and helper. Before registration,
it runs the exact binary's one-shot stdio hook-catalog command with strict
config and the requested cwd. The bounded response must contain exactly the
seven reviewed, enabled, trusted, non-managed user handlers with exact source
path and frozen hashes, and no warning, error, plugin, managed, modified,
untrusted, disabled, or foreign handler. The subprocess must exit cleanly
within its deadline. Any difference fails visibly before a card/runtime exists;
no review choice or trust mutation is automated. The launcher uses direct argv,
never shell interpolation or PATH lookup. A same-UID mutation between preflight
and exec remains the declared residual trust risk.

The launcher mints `SKIDBLADNIR_RUNTIME_ID`, registers it with:

```text
profile + runtime id + Codex PID + kernel start time + pane TTY
+ tmux server/session/window/pane ids + origin
```

and then execs the exact pinned binary. For laptop origin, launcher PID becomes
the TUI PID. For gateway origin, the TUI is the pane root. An external origin
has no runtime id and is never hook-authoritative.

The reviewed synchronous hooks are:

| Hook | Retained projection |
| --- | --- |
| `SessionStart` | Root thread/session, effective cwd/model/source; binding create/replace |
| `UserPromptSubmit` | Turn id/time, `WORKING`, optional first objective preview |
| `PostToolUse` | Tool name/time only |
| `SubagentStart` / `SubagentStop` | Safe activity only |
| `Stop` | Completion candidate with turn id/time |
| `SessionEnd` | Semantic process-session end; exact liveness still controls attachability |

The helper validates exactly one vendored hook schema, reads the runtime id,
parses only the canonical rollout basename's UUID without opening the
transcript, and discards the path. It walks `/proc` from itself to the nearest
ancestor whose executable is the exact pinned Codex binary and reports that
PID, kernel start time, and TTY. It sends one bounded projection to an
owner-only Unix socket, prints no model-visible output, and awaits an ACK.
Delivery failure writes an atomic owner-only gap containing only the validated
identity/projection tuple and optional bounded objective preview; the gap never
contains the full prompt, transcript path, tool input/output, assistant text,
reasoning, or terminal bytes.

The gateway accepts root authority only when:

1. runtime id resolves to one live/historical managed binding;
2. nearest exact Codex PID/start/TTY equals that binding;
3. hook profile equals the binding profile;
4. the hook is root traffic, not explicitly discriminated native-subagent
   traffic; and
5. a pending resume ref, when present, equals the parsed root thread id.

The runtime tuple authenticates origin; thread/session identify the selected
root inside that already-authenticated TUI. A later root `SessionStart` is
binding rollover, not foreign traffic. Only `SessionStart` may replace the
confirmed root. Every other root-lifecycle fact is authoritative only when its
thread matches the current confirmed root; shutdown hooks from historical or
never-confirmed loaded roots are counted and discarded. A nested Codex process
inherits the env marker but fails nearest-ancestor identity. Native-subagent
events resolve the same runtime and project activity only. Hooks cannot
classify a public `/fork` versus `/new`; controlled command injection proves
the test sequence, not a production discriminator. Unknown hook shapes are
defects; well-formed traffic that fails authority is counted and discarded.

### Turn and state reducer

Hook facts serialize per runtime. `UserPromptSubmit` opens one newest root turn.
A later root prompt or binding rollover closes any prior open turn without Idle
or attention. `Stop` creates a completion candidate but does not immediately
publish Idle. After one second with no newer root prompt/session:

- the turn closes;
- a live binding publishes `IDLE`;
- result-ready attention is raised once in the same SQLite transaction.

A newer prompt during the interval cancels the candidate. Gateway restart
reconstructs and re-arms the remaining interval from durable timestamps. A gap
marker applies the same reducer using its original observation time. Duplicate
hook/gap delivery is idempotent. An authoritative current-root `SessionEnd`
after a qualifying Stop resolves completion before runtime-loss derivation;
authoritative `SessionEnd` or process death with an unclosed turn and no
qualifying Stop derives death-while-working. Historical/unconfirmed-root
`SessionEnd` never changes card state.

No timer asserts conversation truth beyond this coalescing rule: Stop is the
semantic completion fact; the delay only orders an immediately concurrent
continuation. Missing lifecycle evidence is `UNKNOWN`, never guessed Idle.

### Ownership and trust

| Owner | Owns | Must not own |
| --- | --- | --- |
| Android | Dashboard, portraits, requested cwd, terminal rendering/input, disposable cache | Agent truth, assignment, Codex refs, tmux commands |
| Dev-server router | Profile selection and scoped launcher delegation | Agent lifecycle or launcher implementation |
| Gateway | Auth, DTOs, paging, SSE/WSS admission | Transcript, provider policy, terminal meaning |
| Registry/commands | Agent identity, assignment, lifecycle facts, attention, idempotency | Repos, worktrees, generic jobs |
| Launcher/hooks | Closed profile, exact exec, authenticated semantic observation | Guessed correlation or content |
| Tmux/terminal | Exact binding, PTY bytes, attach/detach/backpressure | Conversation meaning or state parsing |
| Metrics | `/proc` sampling, aggregation, pressure reasons | Scheduling or agent attribution |
| SQLite | Skíðblaðnir facts/receipts | Codex auth/history or terminal bytes |

The TUI, tmux, gateway, and Core agents share one devbox UID. YOLO code can
modify same-UID files/processes. Tailnet and bearer auth protect remote access,
not the control plane from the agent itself. Hostile-agent containment requires
a separate UID or VM and is outside Core.

The terminal endpoint is shell-equivalent authority. It accepts only a server
resolved handle and closes on exact PID/start/TTY/pane mismatch. Android never
supplies raw target, executable, home, or command.

## 6. Domain, storage, and protocols

### Agent, catalogue, and assignment

An **agent** has opaque UUIDv7 id, stable `ga-*` handle, immutable
`characterKey`, closed origin `Managed | External`, profile, optional objective,
working directory, created time, and optional local closure time. It is the
product card/runtime continuity; it is not a repo, worktree, cwd, Codex thread,
or character persona. Origin is projected explicitly; null Codex binding cannot
distinguish a new managed conversation from an external TUI.

**Dvergatal** is append-only build data at `catalog/characters.json`. Core ships
at least 100 distinct named dwarfs from exactly `OldNorse | Tolkien |
GermanicOperatic`. Each entry is exactly:

```text
{key, displayName, tradition, source: {work, locus}}
```

Keys match `^[a-z0-9]+([.-][a-z0-9]+)*$`; display names are unique NFC text of
at most 64 scalars with objective control/separator exclusions; citations are
non-empty and structured. Variant spellings are one figure, disputed dwarfs
are excluded, and no `FairyTale` or `ModernMedia` family exists.
`evidence/sources/dvergatal.md` records curation decisions and one set-level
portrait method/rights basis. Portraits are original square WebP assets named by
the total transform (`norse.durinn` -> `dwarf_norse_durinn.webp`); no scraped or
rights-holder artwork ships. A shipped key is never removed, renamed, or reused;
metadata corrections and appends are allowed.

Assignment occurs in the agent-insert transaction. Hash the stored agent-id text
with SHA-256, interpret the first eight bytes unsigned big-endian, and begin at
that value modulo catalogue length. First scan for a key held by no non-purged
agent; then for one held by no open agent; otherwise use the starting key.
Closed agents keep their key. Exhaustion repeats portraits but never caps agents
or repeats handles. A missing persisted key/portrait is a startup defect, never
reassignment or placeholder.

### Bindings and state

A **Codex binding** is the latest root selected by a managed TUI: exact
`thread.id`, corroborating `session_id`, and confirmed effective cwd/model.
`thread.id` alone resumes. A pending explicit resume ref is
separate command/runtime support state and never projected as confirmed. One
active `(profile, thread.id)` belongs to at most one managed agent; rollover
releases the previous ref. Native subagents never become bindings or cards.

A **runtime binding** is one TUI incarnation: opaque runtime id; closed origin
`gateway | launcher | external`; immutable tmux server/session/window/pane ids;
pane TTY; exact PID/kernel start time; start/end facts. Tmux names and indexes
are never identity. At most one managed live runtime exists per agent. External
bindings are attach-only and have no runtime id or hook authority.

Card state derives by first match over this total order:
`CLOSED > CLOSING > STARTING > EXCEPTION > WORKING > IDLE > RECOVERABLE > UNKNOWN`.
Two simultaneous predicates are a defect.

| State | Evidence |
| --- | --- |
| `STARTING` | Accepted managed launch; exact live runtime not registered and no settled failure |
| `IDLE` | Live managed runtime, bound or not yet bound; no open turn/candidate; null binding means a new conversation awaiting its first prompt, not a resumable conversation |
| `WORKING` | Live managed runtime with open root turn or unsettled Stop candidate |
| `CLOSING` | Accepted Close; exact runtime stop not committed |
| `RECOVERABLE` | No live managed runtime; latest confirmed thread exists; death followed a settled turn |
| `EXCEPTION` | Managed launch failure, runtime death while working, runtime end before any ref, or pending-ref contradiction |
| `UNKNOWN` | External runtime, or required evidence absent/stale/contradictory |
| `CLOSED` | Local closure committed; no live binding |

`derivationReason` is present exactly for non-nominal
`CLOSING|RECOVERABLE|EXCEPTION|UNKNOWN`. `failureCode` and
`failureObservedAt` appear together only when a lifecycle command settled with
that failure; observation-derived reasons do not invent a command error.
Owned-state contradictions are defects, not `UNKNOWN`.

Derivation reasons are the closed set:

```text
close-accepted | runtime-ended-idle | runtime-ended-working |
runtime-ended-unbound | runtime-launch-failed | system-error |
pending-resume-contradiction | external-runtime-untracked |
evidence-absent | evidence-stale | evidence-contradictory |
start-binding-deadline
```

Attachability means the latest runtime is freshly verified by exact
PID/start/TTY/pane and state is not `CLOSING` or `CLOSED`. External `UNKNOWN`
cards are attachable. Admission always rechecks; an ordinary read returns the
last sample.

### SQLite and lifecycle commands

SQLite uses WAL, `foreign_keys=ON`, one serialized writer, and a named
`busy_timeout`, asserted on every connection/readiness. Migrations are
transactional. Facts use a never-reused monotonic sequence; every other table
has its own UUIDv7 id.

| Table | Fact |
| --- | --- |
| `agents` | Handle, character, origin, profile, optional objective, cwd, created/closed |
| `codex_bindings` | Current confirmed root ref/session/effective cwd/model per agent |
| `pending_resumes` | Exact user-selected ref until first root hook or terminal failure |
| `runtime_bindings` | Runtime id/origin and exact tmux/TTY/PID incarnation/end |
| `agent_observations` | Closed hook/liveness facts and safe activity |
| `lifecycle_commands` | Installation/key/kind/digest, step, immutable outcome |
| `facts` | Ordered public projection changes |
| `attention` | Completion/exception attention and acknowledgement |
| `host_samples` | CPU/load/memory/swap/disk/PSI |
| `installations` | Bearer verifier, pairing/revocation/last seen/contract digest |
| `schema_migrations` | Applied migration and time |

Lifecycle kinds are exactly `StartAgent | CloseAgent | ReopenAgent`. A unique
`(installation_id, client_command_id)` plus normalized semantic-payload digest
linearizes replay. Terminal outcomes are immutable. Exact terminal replay
returns the original receipt; in-flight replay returns current receipt and never
duplicates a stage.

Start stages are `accepted -> runtime_created -> runtime_registered -> outcome`.
Phone-origin workers resume deterministic tmux creation/registration after
restart. Launcher-origin commands never outlive the invoking launcher: the
reconciler may settle an observed registration/outcome but never creates a user
pane or execs on its behalf. A named binding deadline settles failure. Close and
Reopen pin exact binding/ref at acceptance and serialize per agent.

Gateway runtime session names are deterministic per runtime id. Replay adopts a
same-named session only after exact invocation/PID/pane verification; otherwise
it removes only that gateway-owned session and recreates it. Launcher and
external origins never authorize mutation of the user's tmux session/window/
pane. Close may signal the exact managed launcher TUI PID only.

Agent projection and SSE Fact commit atomically. Fact kinds are exactly:

```text
AgentStarted | AgentBound | AgentStateChanged | ActivityObserved |
AttentionRaised | AttentionAcknowledged | CommandSettled | AgentClosed |
HostPressureChanged
```

Facts/host samples retain seven days; safe observations 30; ended runtimes and
acknowledged attention 90; command receipts for installation lifetime;
agents/bindings until explicit future purge. Pruning retains the newest
state-bearing observation per kind, latest binding/end, and unacknowledged
attention. A state changed by pruning alone is a defect. Terminal bytes and
Codex transcripts are never stored.

### HTTP and local control

Strict bounded `/v1` JSON runs over Tailnet TLS. Pair is the only unauthenticated
route and requires a local ten-minute challenge. Every response is `no-store`;
there are no cookies, redirects, CORS, cleartext, or HTML errors.

Pairing uses at least 128 CSPRNG bits, a constant-time verifier comparison, one
active challenge, single-use consumption, and five-attempt destruction.
Successful pairing atomically registers a fresh 256-bit bearer verifier and
revokes every prior phone installation, closing their SSE/WSS within one
liveness period. At most one phone is active. Raw bearer/secret material never
enters SQLite, repo, logs, or evidence.

Every request/response carries `Skidbladnir-Contract`, a SHA-256 over the
domain-separated committed API bytes plus sorted catalogue key set. The digest
is generated once into Go/Kotlin; static recomputes it. Catalogue name/source
or portrait-byte corrections do not change the protocol; key changes do.
Mismatch returns `ProtocolMismatch` on Pair, HTTP, SSE, and WSS before mutation.

| Method/path | Contract |
| --- | --- |
| `POST /v1/pair` | Consume challenge; replace phone bearer |
| `GET /v1/bootstrap` | Profile health, host pressure, counts, snapshot Fact cursor, up to eight confirmed recent cwds/profile |
| `GET /v1/agents?cursor=&limit=&state=` | Keyset `(created desc,id desc)`, `limit<=100`, `open|closed|all`, default open |
| `GET /v1/agents/{handle}` | Exact card, binding, attention, attachability, reasons/failure |
| `POST /v1/agents` | `StartAgent(clientCommandId,profile,objective,cwd)`; `202` |
| `POST /v1/agents/{handle}/close` | Managed idle-only Close; `202` |
| `POST /v1/agents/{handle}/reopen` | Managed confirmed-ref Reopen; `202` |
| `POST /v1/agents/{handle}/attention/acknowledge` | Idempotent acknowledgement |
| `GET /v1/events` | Durable SSE after `Last-Event-ID` |
| `GET /v1/host/samples?window=&resolution=` | Current plus <=300 truthful aggregate buckets |
| `GET /v1/terminal/{handle}` | Authenticated WSS upgrade after fresh attachability check |

Every `202` returns `{handle,clientCommandId,acceptedAt}`. Each accepted command
settles exactly once as `Succeeded` or one closed failure in a durable
`CommandSettled` Fact. No asynchronous failure is silent.

Local owner-only socket commands are exactly:

```text
PrepareNew | ListTrackedAgents | PrepareResume |
CreatePairingChallenge | ListInstallations | RevokeInstallation
```

They accept typed profile/cwd/argv/pane/TTY or exact selected thread id as
applicable; never raw shell, executable, home, socket, SQL, or generic
maintenance. Android can supply only profile, objective, and cwd for Start.

Closed errors are:

```text
Unauthenticated | PairingInvalid | ProtocolMismatch | InvalidRequest |
AgentNotFound | AgentClosed | AgentWorking | AgentNotAttachable |
AgentUntracked | WorkingDirectoryInvalid | WorkingDirectoryUnavailable |
ProfileUnavailable | RuntimeLaunchFailed | CommandConflict |
CursorInvalid | ResyncRequired | LivenessUnverifiable
```

Each has one schema-owned HTTP mapping and one named trigger. Unexpected
protocol/storage/process states are defects with a correlation handle.

### SSE and terminal WSS

Bootstrap and `snapshotFactCursor` come from one SQLite read transaction. SSE
subscriber registration is serialized with post-commit Fact fan-out; stored and
queued facts merge into strictly contiguous ascending sequence. Expired cursor
or bounded-queue overflow returns `ResyncRequired`; malformed/filter-invalid
cursor is `CursorInvalid`. SSE carries no terminal or transcript data.

WSS text frames are the closed `Hello | Presence | Resize | Detach | Error`;
binary client frames are input and binary server frames are PTY output. One WSS
owns one PTY/client/shadow and tears all three down on any close. `Hello` and
`Presence` expose only whole-group attached-client count and
`Owner|Constrained` geometry. Initial phone geometry is set before attach;
tmux's repaint is the only initial output. There is no byte replay or gateway
scrollback. Slow clients disconnect and reattach fresh. Exact identity is
rechecked before each input and output flush.

Any WSS loss immediately freezes terminal input and shows a typed
`Reconnect required` state. The client buffers and automatically resends no
bytes. A draft already delivered to the PTY remains owned by the TUI; delivery
that was not acknowledged is unknowable and is never guessed. The user returns
to Agents and explicitly opens a fresh attachment.

Hard bounds: HTTP body 64 KiB; objective 240 Unicode scalars; cwd 4,096 UTF-8
bytes; page 100; history 300; SSE 256 Facts/1 MiB; terminal frame 64 KiB and
queue 1 MiB; geometry 20–240 columns and 5–120 rows; raw hook input 4 MiB;
projected hook message or gap 16 KiB; unary handler 30 s. These are transport
bounds, not agent quotas.

Objective is NFC, non-empty for Android Start, at most 240 scalars, with no
C0/C1, line/paragraph separators, or bidi overrides. Laptop/external cards may
have semantic absence until the first accepted root prompt supplies the same
bounded preview. The helper NFC-normalizes that prompt, collapses whitespace,
controls, line/paragraph separators, and bidi controls to single ASCII spaces,
then takes at most 240 scalars and trims the result; it never retains the full
prompt. Wire enums use PascalCase except the externally fixed lowercase
profile/list/window values declared by the schema.

## 7. Product and operations

### Android surface

- Compile/target/min SDK 36. One manually installed Core package.
- The Forge has profile, objective, and editable single-line cwd. Cwd disables
  autocapitalization/autocorrect/smart punctuation and preserves pasted bytes;
  invalid input/form state remains intact. Character assignment is automatic.
- Hlíðskjálf is a virtualized adaptive two-column grid, one column at large
  font. Cards show portrait/name, handle, `CODEX · profile`, state, optional
  objective, safe activity, age, attention, and typed non-nominal reason.
  External cards visibly state `External TUI · status unavailable`.
- An Idle card with null Codex binding states
  `New conversation · not yet resumable`; it never implies saved history.
- Grid order is attention first, then `WORKING`, transitional, problem,
  `IDLE`, then last observation descending and handle. `CLOSED` is behind an
  explicit filter.
- Card tap opens an attachable terminal. Long-press/overflow opens a sheet with
  exact confirmed refs, cwd, state/reason/correlation, attention/activity, and
  the exhaustive `Open | Close | Reopen | Acknowledge` actions. Invalid actions
  are disabled with their literal reason. Close warns that an unsent TUI draft
  is not lifecycle evidence and will be discarded.
- One exhaustive `agentErrorMessage` maps every closed failure; UI never
  invents a failure from `derivationReason`.
- The <=48 dp pressure strip shows `OK|WARM|HOT|UNKNOWN`, every active reason,
  attention count, and `LIVE|RECONNECTING|STALE`. Expanded history shows CPU,
  memory, swap, normalized load, distinct-filesystem disk, and CPU/memory/I/O
  PSI with <=300 one-hour points.
- SSE loss marks every card/pressure view stale with last-Fact age until
  bootstrap/cursor recovery succeeds.
- Terminal detail is full screen. Its >=48 dp horizontally scrollable accessory
  row is `Agents | Esc | Ctrl-C | Tab | Left | Right | Up | Down | Home | End |
  Newline | Detach`. `Newline` sends raw Ctrl-J (`0x0a`), the pin's first
  non-submit editor binding; ordinary Enter sends raw CR (`0x0d`). These exact
  bytes survive the pinned tmux 3.4 client path, whose build does not expose a
  CSI-u extended-key format. Esc
  long-press sends Esc Esc. Agents, Detach, back, and backgrounding destroy only
  the phone attachment. Android process recreation returns to the Agents grid
  with no terminal attachment; opening the card creates a fresh attachment and
  never restores or replays terminal bytes.
- `CONSTRAINED` chrome explains the cursor-following viewport and that rotation
  or terminal zoom reveals more cells without reflowing the laptop pane.
  History uses the pinned TUI scroll keys/touch mapping and never tmux copy mode.
- Gboard owns typing, clipboard, emoji, settings, and voice. There is no
  microphone permission or `SpeechRecognizer`; dictation stays editable and
  never auto-sends.
- xterm.js runs in screen-reader mode as one labeled terminal region. All
  Compose surfaces are 48 dp, TalkBack-labeled, Switch-Access reachable, and
  usable at 200%/both navigation modes. Character-level terminal TalkBack is a
  declared limitation.
- Near-black tonal surfaces, warm high-contrast type, restrained luminous-thread
  accent, and avatars as landmarks rather than state. No decorative motion.

Kotlin owns WSS/auth. WebView loads only packaged assets through
`WebViewAssetLoader`; CSP is `default-src 'none'` plus the bundle; file/content/
network access and universal/file JavaScript access are off; no
`addJavascriptInterface`; xterm never receives the bearer. Kotlin exchanges
bounded bytes over `WebMessagePort`.

Paste strips ESC and C0 except newline/tab (CR normalized) before bracketed
paste, so content cannot terminate the frame or auto-submit. The pinned
emulator's enumerated DA1/DA2/DSR/CPR replies are the only automatic input.
There is no clipboard addon/OSC 52 or native title reporting.

Cache only disposable card/metric projections. Terminal scrollback is memory
only. Objective is the only prompt-derived persisted field and is redacted from
logs. `android:allowBackup=false`; release WebView debugging is disabled.

### Host pressure

Every five seconds, record CPU delta, normalized one-minute load, memory, swap,
distinct-filesystem disk, and CPU/memory/I/O PSI. Missing required input yields
`UNKNOWN`. WARM/HOT thresholds are: memory available `15%/8%`; disk `15%/5%`;
normalized load `1.0/2.0`; CPU PSI `some avg60 20%/50%`; memory/I/O PSI
`full avg60 1%/5%`. Escalate immediately; de-escalate after 60 seconds below
the prior threshold. Show every reason; pressure never blocks Start.

### Security, install, and upgrade

- Logs use one closed logger and contain only handle, state, timing, count,
  correlation, and closed error. Never objective, prompt, transcript, tool,
  terminal, token, bearer, secret, or account data. Evidence is likewise
  credential/content-free; synthetic proof prompts are allowed but never
  recorded verbatim.
- UI always shows `YOLO` and profile and never implies containment.
- Upgrades are idle maintenance: close ingress; require every tracked agent
  idle; detach/stop tracked TUIs; stop gateway; verify no tracked process remains;
  `VACUUM INTO`; archive each profile home, pinned binary, hook schemas/config,
  and `codex.lock` with permissions plus digest manifest; install/reprobe; then
  `./scripts/test accept upgrade`. Any failure restores every manifest artifact
  and reprobes before ingress. Each pin change rehearses forced failure/restore
  once. Snapshot is mode 0700 under
  `$XDG_STATE_HOME/skidbladnir/snapshots/<pin>` and deleted after acceptance.
- Tmux upgrades use the same maintenance path and must reprove grouped-client
  semantics. There is no fallback binary or mixed runtime.

### Future runtime seam

Do not create a Core runtime interface. A second runtime must first prove:

```text
launch interactive client; recover exact conversation identity;
observe work/idle/end without screen scraping; attach terminal bytes;
stop/detach honestly; isolate account credentials
```

Then hard-cut one explicit discriminator/adapter. Evaluate `llm-calling` only
against that concrete runtime. Missing capability stays explicit.

## 8. Target files and work split

```text
api/skidbladnir.v1.json
catalog/characters.json
generated/api/{go,kotlin}/
schemas/codex/<pinned-version>/hooks/
codex.lock
cmd/{skidbladnir,skidbladnir-hook}/
internal/{registry,storage,commands,events,hooks}/
internal/runtime/{codex,tmux,terminal}/
internal/gateway/{api,auth,sync}/
internal/metrics/
migrations/
android/app/src/main/
android/app/src/main/assets/terminal/
android/app/src/main/res/drawable-nodpi/dwarf_*.webp
tests/{fixtures,integration,system,e2e,live}/
evidence/{sources,live}/
scripts/{build,test,install-devbox}
deploy/{systemd,tailscale,codex}/
.github/workflows/verify.yml
```

Go is primary. Gradle owns Android only. Use standard Go HTTP, explicit
composition, and one SQLite driver. No web framework, ORM, DI container,
queue/broker framework, Makefile, or parallel build surface.

Lane 0 owns generated contracts/pins/catalogue/scripts/evidence and the required
justification-token grammar. No suppression is valid without a non-empty
repository token.

| Lane | Owns | First red proof |
| --- | --- | --- |
| 0 Contract/build/evidence | API, Dvergatal, DTOs, pin/hook schemas, scripts/workflow/evidence | Drift, mutable pin, wrong inventory, or unreviewed hook fails |
| 1 Registry/storage/hooks | Agents, assignment, commands, facts, reducer, migrations | Assignment/replay/rollover/Stop races are atomic |
| 2 Runtime/terminal | Launcher/router seam, Codex/tmux/PTY | Cwd, exact exec/origin, external inventory, switch, shadow, close/reopen |
| 3 Gateway/metrics | HTTP/auth/SSE/WSS/metrics | Real HTTP+SQLite/Facts/bounds/pressure |
| 4 Android | `android/**` | Real-runtime dashboard, terminal, IME, accessibility |
| 5 Composition | Entrypoint/deploy/system/e2e | Real devbox/phone/laptop handoff and recovery |

Lane 0 is serial; lanes 1–4 may follow only its frozen contract; lane 5
composes. No lane edits another's paths.

## 9. Verification and acceptance

Use red/green/refactor for each observable behavior. Tests assert public
surfaces or declared platform boundaries. Use real SQLite and repo-owned
services; fixtures may model Codex/tmux/Android/Tailscale only for deterministic
adapter tests and never claim a live boundary. `NOT_RUN` is not PASS.

Canonical commands:

```text
./scripts/test static
./scripts/test unit
./scripts/test integration
./scripts/test component
./scripts/test system
./scripts/test platform
./scripts/test live
./scripts/test upgrade-live
./scripts/test e2e
./scripts/test verify
./scripts/test full
./scripts/test accept p0
./scripts/test accept core
./scripts/test accept upgrade
```

Required Core gates are static, unit, integration, component, system, platform,
live, and e2e. `verify/full` are compositions. `accept core` emits one
machine-readable result for each required gate and proof row and fails on
`FAIL|NOT_RUN`. `accept p0` composes only static, unit, platform, and the eight
named P0 proof rows; it never imports later `NOT_RUN` rows. `upgrade-live` is a
dedicated all-profile ordinary-TUI synthetic start/hook/Stop/exit round trip,
not an alias for all live tests. At P7, `accept upgrade` composes the then-current
deterministic gates, `upgrade-live`, and rows explicitly marked upgrade-required;
phone Core acceptance remains separate.

### One proof per ownership boundary

| Proof id | Boundary | Authoritative proof |
| --- | --- | --- |
| `command-profile` | Existing command -> profile/launcher | Live all profiles: managed root/resume classification; exact home/pin/cwd/argv; seam fail-visible; other/outside-tmux unchanged; no fallback |
| `launcher-codex-runtime` | Launcher -> Codex runtime | Live direct TUI: registered runtime id/PID/start/TTY/pane before exec; no external App Server; pending exact resume confirmed or fails visibly |
| `hooks-registry` | Hooks -> registry | Real hook JSON + SQLite/SSE: exact process origin, objective/work/activity/settled Idle/end/gap, nested CLI discard, and exactly-once attention across restart |
| `core-root-rollover` | Root/subagent hooks -> card | Real hook/TUI: `/new|clear|fork` replaces one card binding; native subagents are activity-only |
| `lifecycle-tmux-process` | Lifecycle -> tmux/process | Real tmux/TUI: idempotent Start, external attach-only inventory, live switch, dormant Reopen, idle Close, shell cutoff |
| `pty-wss-android` | PTY/WSS -> Android | System/device: ANSI/UTF-8/IME/clipboard, geometry, backpressure, reconnect/no replay |
| `laptop-phone-laptop` | Laptop -> phone -> laptop | Live same pane/PID/thread/draft; laptop focus/geometry/process unchanged; independent Detach |
| `sqlite-fact-card` | SQLite Fact -> card | Real HTTP/SSE atomic snapshot/catch-up/live/dedupe/overflow/retention |
| `dvergatal-agent-card` | Dvergatal -> card | Static catalogue/source/portrait + SQLite/API/Compose assignment/stability/exhaustion |
| `kernel-pressure` | Kernel -> pressure | Real `/proc` persistence/aggregation/render plus synthetic thresholds/missing/hysteresis |
| `bearer-terminal-authority` | Bearer -> terminal | Pair race, replacement/revocation, wrong bearer, raw-target injection, off-Tailnet denial |
| `restart-recovery` | Restart -> recovery | Kill phone/gateway/PTY/tmux/TUI: honest live/recoverable/exception/unknown, never guess/replay |
| `core-scale-concurrency` | Capacity -> cards/runtimes | Device/E2E: >=15 cards and >=2 real concurrent working agents without a cap |
| `core-android-start-profiles` | Android Start -> profiles | Device/E2E: all three homes, cwd variants, empty TUI, YOLO, and fail-before-mutation invalid forms |
| `core-android-accessibility` | Compose -> accessibility services | Physical device: TalkBack/Switch Access, 200%, navigation modes, IME, rotation, process recreation |
| `core-runtime-drift-refusal` | Installed artifacts -> launch | Live: profile/pin/config/hook/router/protocol drift fails visibly before exec, with no fallback |

This is the 80/20 verification shape: deterministic gates per change, one proof
per crossing, one expensive real devbox/phone flow—not every helper matrix.

### Core acceptance

- Physical `SM-S906W` preflight records API/build, Tailscale, WebView, Gboard
  typing/dictation/clipboard, navigation modes, 200% scale, accessibility,
  rotation, IME resize, and process recreation.
- UI language is exact: app `Skíðblaðnir`; `Create new agent`; `New agent`;
  `Start agent`; terminal exit `Agents`; no reserved mythic term leaks into
  route/schema/type/UI. Static enforces the denylist.
- Dvergatal has >=100 validated entries from only three allowed traditions,
  unique key/name, structured citation, one original bundled portrait each, and
  complete curation/provenance. No missing/orphan asset.
- Phone shows >=15 cards with distinct assignment while available, no cap, and
  >=2 real concurrent working agents. Completion attention reaches its card in
  one tap; closed cards leave the default grid; catalogue exhaustion still
  permits Start with unique handles.
- Android Start works for all profiles with exact isolated homes, existing cwd
  including spaces and `~/`, ordinary empty TUI, YOLO, and no substitution.
  Invalid/missing/relative/non-directory cwd fails before runtime creation and
  preserves the form.
- New laptop starts appear automatically. Pre-existing TUIs appear honestly as
  attach-only external `UNKNOWN`. Phone/laptop share exact managed TUI without
  changing laptop session/window/pane/geometry/PID/draft/turn. Live resume
  switches only the invoking client; dormant resume/reopen keeps the same card;
  explicit untracked UUID becomes pending then hook-confirmed. No second managed
  TUI or external App Server daemon is started.
- In-TUI `/new`, `/clear`, and `/fork` retain the card/TUI and replace only its
  confirmed active Codex binding on the next root prompt. Native subagents stay
  activity-only.
- Gboard text/clipboard/reviewed dictation and every accessory key reach the
  TUI; dictation is corrected mid-line and extended with a second line before
  submission; nothing auto-sends or replays except recorded emulator replies.
  Output above the fold is readable without changing laptop pane mode/view.
- Every card state comes only from declared hook/liveness facts. Stop never
  publishes Idle/attention before its settle interval; concurrent continuation
  cancels both; gap/restart repairs exactly once; terminal content has no state
  authority. Every non-nominal cause/failure renders distinctly.
- Working Close rejects. Idle Close stops only exact managed TUI and exposes no
  shell to phone; Reopen reaches latest confirmed conversation. External cards
  cannot Close/Reopen. Attention acknowledgement is idempotent and re-arms.
- Tailnet/SSE loss marks cards and pressure stale with age. Constrained terminal
  explains its cause. Phone/WSS/gateway/tmux/TUI loss never replays or
  substitutes input/profile/agent.
- Current/history CPU, memory, swap, load, disk, PSI and all reasons render;
  missing is Unknown; Hot never blocks Start.
- TalkBack/Switch Access pass on all Compose surfaces at 200%, both navigation
  modes, edge-to-edge, IME, rotation/process recreation; terminal limitation is
  recorded rather than falsely passed.
- API/schema/UI contain no project/repo/worktree/workspace/quota/generic-runtime/
  transcript/composer/direct-turn/raw-target/App-Server surface.
- Profile/pin/config/hook/router drift and unknown protocol fail visibly before
  an untracked managed TUI starts. No fallback exists.
- `./scripts/test accept core` is PASS with no required `NOT_RUN`.
- Every accepted evidence record is at or before the verifier host's current UTC
  time and names exact current artifact digests; future-dated or digest-stale
  evidence fails acceptance rather than being silently reused.

## 10. Deferred v1.1+

- Wake: opaque FCM attention, redacted notification, doze/deep-link proof.
- Release: A/B packages, signed immutable candidate, health/activation/rollback.
- Native voice only if Gboard fails; images/rich clipboard/output copy; purge;
  off-host backup/full restore.
- Second concrete runtime, then one explicit adapter and `llm-calling` review.
- Any future structured non-terminal interaction requires a new post-Core
  architecture contract and replacement proofs; it cannot coexist as fallback.
- Separate UID/VM hostile-agent containment.

Anything else requires a new acceptance criterion and scope decision.
