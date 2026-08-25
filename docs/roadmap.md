# Skíðblaðnir Core implementation roadmap

Status: P0 in progress; P1–P7 pending.

This document owns delivery order and PR boundaries. The
[architecture](architecture.md) owns behavior and acceptance;
[`docs/rules`](rules/index.md) owns implementation standards. A roadmap item
never overrides either.

## 1. Delivery contract

- Deliver eight sequential PRs. Each PR has one observable outcome, its red
  proof, implementation, and boundary evidence.
- Base each PR on its merged predecessor. Investigation may run early;
  implementation may not target an unmerged contract.
- Architecture lanes are exclusive path ownership inside the current PR, not
  product entities or long-lived branches. Worktrees are disposable tooling.
- Only the root integrator changes `api/`, `catalog/`, `generated/`, schemas,
  pins, command composition, architecture, roadmap, or shared evidence ledger.
- A builder owns and observes its red before implementation. A verifier writes
  no production code or replacement test. No agent crosses its assigned paths.
- Intermediate `main` builds and proves all behavior it contains. Core is not
  accepted until every final gate is PASS; `NOT_RUN` is never pass.
- A contradictory platform proof stops the PR and amends the architecture. No
  compatibility path, fallback, or weakened test is permitted.

```text
P0 contracts and exact platform proofs
 -> P1 one managed laptop agent
 -> P2 lifecycle and continuity
 -> P3 phone control plane
 -> P4 shared terminal transport
 -> P5 Android dashboard
 -> P6 Android terminal and IME
 -> P7 operations and Core acceptance
```

### Per-PR red/green/refactor

Before implementation:

1. Rebase on the merged predecessor and run its gates.
2. Name one observable outcome and exact owned paths.
3. Add the behavior proof and observe the intended failure.
4. Record version/device prerequisites; absence is `NOT_RUN`.

Before merge:

1. The proof passes through a declared public surface or external boundary.
2. Predecessor proofs and relevant tiers pass; every log uses the closed logger.
3. Remove duplicate paths, test-only production seams, permissive parsing,
   unused options, and speculative abstractions.
4. Regenerate source-owned artifacts/evidence in the same PR.
5. `git diff --check` passes and the PR contains no unrelated change.
6. The PR description records outcome, red, commands, live evidence, remaining
   `NOT_RUN`, and recovery impact.

Fixtures prove adapters only. Live/device records under `evidence/live/` include
exact versions, procedure, outcome, and artifact digests, but never credentials,
account identity, prompts, assistant/tool/terminal content, paths to transcripts,
patches, reasoning, or raw terminal bytes. Synthetic proof input may execute but
is not copied into evidence.

## 2. P0 — contracts and platform proofs

### Outcome

Freeze every later input and prove the ordinary pinned Codex TUI, per-runtime
hook origin, tmux handoff, and Android terminal mechanisms on exact targets. P0
ships no agent-management capability.

### Owns

- Lane 0: API, Dvergatal, generated DTOs, `codex.lock`, hook-only pinned schemas,
  scripts/workflow/proof ledger/evidence.
- Minimal Go/Android build skeleton and platform probes.
- Final bounds, enums, errors, facts, hook projections, test/proof ids.
- Reviewed profile hook configs, `skidbladnir-hook`, exact trust entries, and
  reproducible digest set.
- One closed typed logger in which sensitive/content fields are unrepresentable;
  static rejects every other log call.
- >=100 final Dvergatal entries plus source/rights/curation evidence. Portrait
  bytes land in P5.

### Red

- Missing/mutated binary, hook schema/config/helper, generated DTO, API,
  catalogue, terminal asset, or command inventory initially passes.
- Duplicate/padded/disputed/uncited catalogue content or ad-hoc logging passes.
- A nested Codex inherits the runtime marker and its raw hook origin is accepted;
  code assumes hooks distinguish public `/fork` from `/new`, or grants native
  subagents root lifecycle authority.
- `/new`, `/clear`, `/fork`, Stop, a subsequent prompt, SessionEnd, pane death,
  or PID death lacks a reproducible raw event/process ordering record.
- A direct pinned TUI, tmux client, or target-phone terminal behavior remains
  unproved. Missing physical S22+ makes platform `NOT_RUN` and blocks merge.

### Green

- Pin exact binary/source/tag/digest and only the seven hook input schemas; no
  repo-owned App Server schema/daemon/session transport/adapter remains; the
  isolated one-shot read-only hook-catalog probe is the sole test/runtime seam.
- Generate Go/Kotlin DTOs and contract digest reproducibly. Establish final test
  dispatcher and one machine-readable row for every P0 and final ownership
  proof.
- Prove all profile homes independently without copying/logging credentials.
- Prove direct ordinary start/resume, exact runtime marker + nearest pinned TUI
  PID/start/TTY, canonical basename thread id, session corroboration, fresh
  public-fork/revert identities, the absence of a hook-level fork/new
  discriminator, frozen production trust hashes, the blocking review behavior
  of one extra untrusted handler, nested-CLI discard, and native-subagent
  activity-only behavior.
- Record `/new`, `/clear`, and `/fork` raw root-hook sequences in one unchanged
  TUI PID, plus active-Escape interruption, same-root continuation, unordered
  loaded-root SessionEnd, `/quit`, idle Ctrl-C, SIGKILL, pane, and PID behavior.
- Record exact resume grammar and the unmarked-TUI tmux/`/proc` inventory facts
  consumed later; do not project cards, settle turns, or exercise recovery.
- Prove tmux grouped shadows, `active-pane`, `ignore-size`, last-link safety,
  repaint, target-specific input, and stock TUI edit/newline/history/scroll/
  normal-buffer behavior.
- Retain the Android harness only as real platform coverage: pinned xterm,
  ANSI/Unicode, automatic replies, Gboard composition/dictation/clipboard,
  editable multiline input, resize/rotation, and locked WebView.

### Merge gate

- `./scripts/test accept p0` passes exactly `static`, `unit`, `platform`, and
  `p0-codex-pin|p0-profile-direct-tui|p0-hook-origin|p0-hook-identity|p0-tui-lifecycle|p0-tmux-handoff|p0-tui-keys|p0-android-terminal`.
  Every record is current, credential/content-free, not future-dated, and names
  exact current artifact digests.
- Dvergatal evidence covers every frozen key and a producible portrait method.
- App Server protocol schemas, session adapters/transports, daemons, and proof
  rows are absent. Exact negative-process assertions and the isolated one-shot
  read-only hook-catalog preflight are permitted and prove no runtime ownership.
- Shared contracts are frozen. Later change returns to Lane 0.

### Not in P0

Registry/SQLite lifecycle, atomic card rollover, Stop settlement/attention,
gap/restart repair, resume arbitration/switch, external-card materialization,
gateway, product UI, deployment, or fake E2E. P0 proves the production hook
bytes and three-profile trust lock in isolated homes; P1 installs them only
after the real receiver exists and enforces the effective-cwd preflight.

## 3. P1 — one managed laptop agent

### Outcome

Inside tmux, `codex-personal|work|work2` starts one registered ordinary Codex
TUI in the invoking cwd/card. Outside tmux and other subcommands remain exact
router behavior.

### Owns

- Minimum Lane 1 agent/character/runtime/pending+confirmed-binding/command/Fact
  storage, migrations, hook receiver, and `todo.txt` projection.
- Minimum Lane 2 launcher, cwd/argv validation, invoking-pane registration,
  direct exact-binary exec, liveness, and external-TUI inventory.
- Minimum hook installation used at the real laptop boundary.
- Companion dev-server PR for the versioned marker-gated router seam and frozen
  Codex install; this repo verifies, never edits, that seam.

### Red

- Each profile fails managed start; wrong home/pin/hook/cwd/tmux/seam or
  conflicting argv reaches TUI exec. An extra disabled, untrusted, modified,
  managed, plugin, project, session, or foreign hook reaches registration or an
  interactive review screen.
- Duplicate `clientCommandId` creates multiple cards/runtimes.
- A pre-existing laptop TUI is guessed as managed or invisible to Android
  inventory.

### Green

- `PrepareNew` atomically creates card/character/command, registers exact
  invoking tmux/PID/start/TTY, and returns one direct exec plan.
- Launcher injects runtime id + YOLO, preserves reviewed interactive arguments,
  performs the bounded exact-cwd one-shot hook-catalog preflight, and execs the
  absolute pinned binary in place. No tmux creation, prompt, App Server session
  transport/daemon, PATH lookup, or returned phone shell.
- First root hooks fill binding/objective and Working state. External exact
  pinned panes become attach-only Unknown cards without screen/transcript reads.
- Exact replay returns its immutable receipt and never creates another runtime.

### Merge gate

- Live all profiles: isolated home, correct pin, arbitrary cwd including spaces,
  one invoking-pane TUI/PID, stable `ga-*`/character, no prompt/fallback.
- Router proof covers root/resume delegation and byte-equivalent remaining
  paths. The companion PR is merged and deployed; evidence names its SHA,
  installed router digest, seam version, and Codex version before/after converge.

### Not in P1

Resume picker/switch, full reducer, phone API, WSS, Android, or formal Close.

## 4. P2 — lifecycle and continuity

### Outcome

Managed cards retain truthful state and one exact runtime across root rollover,
laptop resume, Close/Reopen, process loss, gap repair, and gateway restart.

### Owns

- Remaining Lane 1 reducer, attention, retention, idempotency, reconciliation,
  binding rollover, and all lifecycle commands.
- Remaining non-network Lane 2 resume picker/switch, close/reopen, exact
  liveness, tmux lifecycle, and external-card behavior.

### Red

- Stop/continuation races false-idle; restart drops/doubles attention; gaps,
  authoritative SessionEnd, or second prompts strand/incorrectly close turns;
  historical/unconfirmed-root SessionEnd mutates the card.
- `/new|clear|fork` duplicates cards or loses runtime continuity.
- Two resume callers create two managed TUIs; ambiguous tmux client switching
  moves the wrong client.
- Stale/contradictory evidence guesses state; restart leaves commands undriven
  or replays terminal input.

### Green

- Implement total state derivation, one-second Stop settlement, exact-once
  attention/ack/re-arm, gap import, rollover, and Unknown.
- Bare `resume` lists bounded local tracked cards only. Exact resume switches
  one resolved live client, reopens a verified-dead card, or creates a pending
  binding for an explicitly supplied untracked UUID.
- Implement managed idle Close, latest-confirmed-ref Reopen, external attach-only
  restrictions, retention, command recovery, and restart reconciliation.

### Merge gate

- Real SQLite/service proofs cover every command outcome and reducer race.
- Real tmux/Codex proofs cover live switch, dormant reopen, explicit pending
  resume, rollover, idle close, external inventory, process loss, and restart
  while preserving one card/runtime identity.

### Not in P2

Public API, Android, WSS, generic jobs, transcript storage, or new standalone
close CLI.

## 5. P3 — phone control plane

### Outcome

An authenticated Tailnet client pairs, bootstraps, observes, starts, closes,
reopens, acknowledges, and inspects host pressure through the closed `/v1` API.

### Owns

- Lane 3 HTTP/auth/pairing/installations/SSE/bootstrap/paging/commands/metrics.
- Lane 2 gateway-origin deterministic tmux creation for phone Start/Reopen.
- Real SQLite/system proofs for Facts, auth, replay, bounds, and pressure.

### Red

- Pair race/replay mints authority; SSE races lose/duplicate/order Facts.
- Wrong/revoked bearer, contract mismatch, invalid cwd, raw target, or oversized
  input crosses ingress.
- Missing `/proc` renders healthy; Start command replay creates another runtime.

### Green

- Implement replacement pairing with verifier-only persistence and immediate
  connection revocation.
- Implement every non-terminal `/v1` route, closed error, bound, idempotent
  receipt, cursor, and asynchronous settlement exactly once.
- Implement snapshot+Facts/SSE continuity, bounded queues/heartbeats/resync, and
  honest stale state.
- Sample/aggregate CPU/load/memory/swap/disk/PSI; never schedule or gate Start.
- Bind loopback and install/verify minimal Tailscale Serve mapping.

### Merge gate

- Real HTTP+SQLite covers pairing, rotation/revocation, paging, all commands,
  attention, SSE race/reconnect/overflow/restart, external Unknown cards, and
  metrics.
- Real Tailnet client reaches `/v1`; public/Funnel/unverified mapping fails.
- Unknown fields/enums, injection, incompatible digest, and raw runtime inputs
  receive exact typed results.

### Not in P3

Terminal WSS, Android UI, notifications, Funnel, or provider abstraction.

## 6. P4 — shared terminal transport

### Outcome

An authenticated client attaches to the one registered tmux TUI while laptop
remains attached, then detaches without another TUI or laptop state change.

### Owns

- Lane 2 grouped-shadow PTY implementation/reconciliation.
- Lane 3 terminal WSS auth/frames/backpressure/liveness/identity.
- One protocol-faithful non-Android boundary client.

### Red

- Attach spawns TUI, steals/resizes laptop, exposes returned shell, or replays
  bytes/input. Slow client grows unbounded; stale identity accepts input;
  disconnect leaks/kills the wrong session.

### Green

- One shadow session + PTY per WSS, permanent `active-pane|ignore-size`, exact
  client-context targeting, truthful presence/geometry.
- Opaque bounded bytes only after exact identity; serialized attach/resize/
  detach/last-link promotion/GC; no parser/history/provider token.

### Merge gate

- Real tmux/system proof: two clients, one pane/PID/draft, isolated focus/size,
  independent detach, bounded slow failure, no replay/shell, exact cleanup.
- Revocation/digest mismatch closes live WSS within bound.

### Not in P4

Android rendering, custom terminal semantics, screen state, transcript history,
or writer leases.

## 7. P5 — Android dashboard

### Outcome

The physical phone pairs and presents real cards, Forge, lifecycle actions,
attention, stale state, and pressure.

### Owns

- Lane 4 secure bearer, REST/SSE/cache, Compose grid/detail/Forge/pressure, and
  accessibility.
- Final original Dvergatal portraits and total asset validation.

### Red

- Recreation loses authority or presents cache as fresh; SSE replay duplicates;
  Forge leaks invalid/raw runtime fields; portrait or accessibility/layout
  failure passes; minified shrink strips an asset.

### Green

- Pair/repair/bootstrap/SSE convergence/disposable cache/stale/revoked paths.
- Dense cards, external Unknown language, objective/activity/state/attention,
  pressure/history, exhaustive failure reasons, default-open filter.
- Forge exact profile/objective/cwd and server recents; preserve invalid drafts;
  no prompt/character/runtime field.
- Close/Reopen/Acknowledge settlement UI and full disabled action reasons.
- One statically referenced portrait per catalogue key.
- Until P6, card-open/reopen-terminal proof rows stay explicit `NOT_RUN`.

### Merge gate

- Component tests use real Compose/client runtime and semantic interactions.
- Physical release-configuration S22+ proof: pair, >=15 cards, two working,
  path with spaces, failures/stale/attention/pressure, recreation/rotation,
  navigation modes, TalkBack, Switch Access, 200%, all portraits.
- No terminal/prompt/provider/project/worktree/raw-target leak.

### Not in P5

Terminal input/rendering, microphone/native speech, FCM, self-update, or visual
scope beyond architecture.

## 8. P6 — Android terminal and IME

### Outcome

Opening a card on S22+ controls the exact stock TUI shared with laptop. Gboard
typing, clipboard, and dictation remain editable and never auto-submit.

### Owns

- Lane 4 pinned xterm assets, locked WebView, Kotlin WSS/PTY bridge, terminal
  chrome/accessory/lifecycle, and device tests.
- PTY-to-Android and laptop-phone-laptop proofs.

### Red

- Phone changes laptop focus/geometry/process or starts TUI; IME/Unicode/
  multiline/paste/replies corrupt or submit; WebView reaches network/files/
  bearer; reconnect replays or leaks shadow; rotation loses editable draft.

### Green

- Local xterm through bounded WebMessagePort; Kotlin-only auth/network.
- Initial geometry, constrained chrome, exact accessory keys, safe paste,
  composition/editable dictation, rotation preservation, detach/reconnect.
- Asset-only CSP, disabled network/file/content, no Java bridge or saved bytes.
- Enable card-to-terminal only with real WSS.

### Merge gate

- Physical S22+ proves ANSI/UTF-8/history/resize/navigation/recreation/rotation,
  Gboard type/clipboard/dictation, mid-line correction, multiline before submit,
  every accessory key, and no auto-send/replay.
- Live handoff records one pane/PID/thread/draft with concurrent laptop/phone,
  independent detach, unchanged laptop. Terminal TalkBack limitation is honest.
- P5 deferred terminal proof rows flip from `NOT_RUN` to PASS.

### Not in P6

Native speech/TTS, transcript reconstruction, output copy, images, or second
runtime.

## 9. P7 — operations, recovery, and Core acceptance

### Outcome

Install and operate the complete private system on devbox/S22+ with fail-visible
readiness, restart/upgrade recovery, and one machine-readable Core verdict.

### Owns

- Lane 5 entrypoint, systemd, Tailscale installer/readiness, E2E.
- Upgrade/rollback snapshot, manifest, forced-failure rehearsal, restore, and
  cleanup mechanism.
- Final recovery/retention/security/resource-lifecycle/acceptance closure.

### Red

- Clean install needs undocumented mutation/overwrites router; service/TUI/
  phone/Tailnet loss guesses or replays; forced upgrade failure leaves an
  artifact changed/reopens ingress; acceptance passes stale/missing proof.

### Green

- Idempotently install repo binaries/config/hooks/service and verify the
  separately owned router seam.
- Supervise gateway only; preserve registered independent TUI processes across
  gateway restart and reconcile exact facts. No App Server service exists.
- Verify loopback/Serve/Funnel absence, permissions, SQLite, hook/pin/router,
  tmux, profile homes, and recovery.
- Complete retention, pairing replacement, todo projection, diagnostics, log
  operation, upgrade/restore, and authoritative test aggregation.

### Merge gate

- Fresh devbox install/restart/upgrade is repeatable from repo plus existing
  profile authentication.
- Recorded snapshot -> forced probe failure -> byte/digest/permission restore
  -> `accept upgrade` PASS.
- Every architecture ownership proof is current PASS on exact devbox/phone.
- `./scripts/test full` and `accept core` PASS with no `NOT_RUN`; worktree clean;
  installed artifacts match commit.

### Not in P7

FCM, A/B update, off-host backup, purge, native voice, images, generic runtime,
or orchestration.

## 10. Subagent execution

Root owns scope/contracts/merge order/final claim. Use at most three concurrent
subagents, two builders plus one verifier, with non-overlapping paths:

| Role | Typical ownership | Must not change |
| --- | --- | --- |
| Domain | Lane 1 plus its tests | API/runtime/Android |
| Runtime/control | Current Lane 2 or 3 plus tests | Shared contract/Android |
| Android | Lane 4 plus tests | Host/API |
| Verifier | Re-run/audit evidence; may write only assigned evidence review | Production/tests/contracts |

Each reads `AGENTS.md`, relevant architecture/roadmap sections, and routed
rules. Optional worktrees begin at exact PR base and return bounded commits.
Never let two agents edit a shared/generated file. Delete only verified-clean
temporary worktrees.

## 11. Scope and progress

- Only root changes this status table. Outcome/Owns/Red/Green/gate changes are a
  reviewed roadmap amendment before the affected PR continues.
- A new capability requires an architecture acceptance criterion. Necessary
  implementation detail for an existing outcome stays in its owner.
- No v1.1 abstraction is pulled forward. Scaffolding, fixtures, or an unrun
  physical boundary are not progress claims.
- P7 closes Core; anything remaining is a failed gate or explicitly re-scoped
  future work.

| PR | Status |
| --- | --- |
| P0 Contracts and platform proofs | InProgress |
| P1 One managed laptop agent | Pending |
| P2 Lifecycle and continuity | Pending |
| P3 Phone control plane | Pending |
| P4 Shared terminal transport | Pending |
| P5 Android dashboard | Pending |
| P6 Android terminal and IME | Pending |
| P7 Operations and Core acceptance | Pending |
