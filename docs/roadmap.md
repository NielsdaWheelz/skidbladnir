# Skíðblaðnir Core implementation roadmap

Status: planned. No PR in this roadmap is implemented.

This document owns delivery order and PR boundaries. The
[architecture](architecture.md) owns behavior, system design, scope, and final
acceptance. [`docs/rules`](rules/index.md) owns implementation standards. A
roadmap item never overrides either authority.

## 1. Delivery contract

- Deliver Core as eight sequentially merged PRs. Do not build it as one branch.
- A PR owns one observable outcome, its red proof, implementation, and
  boundary evidence. Internal layering is not a delivery outcome.
- Architecture Section 8 lanes are exclusive path ownership inside a PR, not
  separate products or long-lived branches.
- The root integrator alone changes shared contracts, generated API, pins,
  test command composition, architecture, or this roadmap. Lane 0 paths are
  worked only under an explicit root assignment and are integrated by the root.
- Base each PR on the merged predecessor. Read-only investigation for the next
  PR may run early; implementation may not target an unmerged contract.
- Git worktrees are disposable development plumbing. They are never stored as
  Skíðblaðnir agents, runtime state, or API concepts.
- Intermediate `main` must build and pass every proof for behavior it contains.
  It is not Core-ready until every final gate passes; `NOT_RUN` is never a pass.
- If a platform proof contradicts the architecture, stop that PR, record the
  evidence, amend and review the architecture, then continue. Do not add a
  fallback.

Dependency order:

```text
P0 contracts/proofs
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

1. Rebase on the merged predecessor and run its required gates.
2. Name the one observable outcome and exact owned paths.
3. Add the behavior-level proof and observe it fail for the intended reason.
   A PR's Red list states acceptance criteria observed failing at the merged
   predecessor's base; sub-cycle reds inside the PR follow the same rule
   against the state at their own start and are named in the PR description,
   not in this roadmap.
4. Record external version/device prerequisites; absence is `NOT_RUN`.

Before merge:

1. The new proof passes through a surface this system declares — the `/v1`
   API, the local launcher/admin control socket, the launcher CLI, the
   `$XDG_STATE_HOME/skidbladnir/todo.txt` operator projection, or a repo-owned
   service's public surface — or through a declared external boundary (tmux,
   Codex/App Server, Android, Tailscale, kernel).
2. All predecessor proofs still pass; relevant static, unit, integration,
   component, system, platform, or live gates pass. Every new log site in this
   PR goes through the closed logger.
3. Refactor away duplicate paths, temporary adapters, test-only production
   seams, permissive parsing, unused configuration, and speculative options.
4. Update generated artifacts and evidence in the same PR as their source.
5. `git diff --check` passes; the PR contains no unrelated change.
6. The PR description states outcome, red observation, proof commands, live
   evidence, remaining `NOT_RUN` gates, and recovery impact.

No PR may claim App Server, tmux, Tailscale, or Android behavior from a fixture.
Fixtures prove deterministic adapter behavior only. Store live records under
`evidence/live/` with exact versions, command/procedure, observed result, and
relevant artifact digests. Live and device records are credential-free and
content-free: no prompt text, assistant text, tool input/output, patches,
stdout/stderr, reasoning, terminal bytes, `transcript_path` values, or account
identifiers — accounts appear only as the digest-only fingerprint the API
uses, and every device screenshot uses synthetic objectives and prompts
authored for the proof. Thread/session/tmux ids, PIDs, TTYs, cwds, versions,
digests, and timings are permitted. Lane 0's `static` gate fails an
`evidence/` file matching the forbidden classes.

## 2. P0 — contracts and platform proofs

### Outcome

Freeze the inputs every later lane implements against and prove that the chosen
Codex, tmux, and Android mechanisms exist on the exact target versions. This PR
ships no agent-management capability.

### Owns

- Lane 0 paths: `api/`, `catalog/`, `generated/`, `schemas/`, `codex.lock`,
  repository scripts, workflow, and `evidence/`.
- Minimal Go and Android build skeletons needed to generate contracts and run
  platform probes.
- Final closed DTOs, enums, bounds, error vocabulary, hook projections, and
  test/proof identifiers from the architecture.
- The reviewed per-profile hook configs, the `skidbladnir-hook` helper, the
  per-profile persisted hook-trust records, and their committed
  expected-digest set.
- One structured logger with a closed, typed field vocabulary — handle, state,
  timing, count, correlation handle, closed error code — in which objectives,
  prompts, hook payloads, terminal bytes, bearers, pairing secrets, and account
  metadata are unrepresentable; `static` fails any logging call outside it.
- The canonical Dvergatal catalogue with at least 100 final entries and stable
  `characterKey` values, plus `evidence/sources/dvergatal.md`: the per-set
  portrait production method and rights basis and the curation dedupe and
  exclusion decisions. Portrait bytes land in P5 and remain outside the
  protocol digest.

### Red

- Missing or mutated Codex binary, schema, hook config/helper, generated DTO,
  API schema, catalogue, or command inventory fails deterministically.
- A catalogue with a duplicate key or display name, a padded or disputed
  figure, or an entry missing its curation record passes `static`.
- An ad-hoc log call outside the closed logger initially passes `static`.
- An unknown App Server or hook field is rejected by the closed decoder.
- A nested Codex CLI inherits `SKIDBLADNIR_BINDING_ID` and its hooks initially
  mutate the tracked card; same-process native-subagent traffic can initially
  close or confirm the parent turn instead of remaining activity-only.
- A foreign lifecycle hook in any effective managed-requirements source
  initially survives readiness or the launcher's pre-exec gate.
- The live proof fails until exact start/unsubscribe/resume, hook identity,
  continuation-safe Stop behavior, summary fields, and grouped-tmux behavior
  are observed.
- The pinned terminal asset fails Unicode, ANSI, IME composition, editable
  dictation, clipboard, or resize on the target phone. The physical `SM-S906W`
  is a prerequisite; without it the platform gate is `NOT_RUN`, which blocks
  P0's merge gate.

### Green

- Pin exact Codex CLI/App Server artifacts and commit generated stable schemas.
- Generate Go/Kotlin DTOs from `api/skidbladnir.v1.json`; generated files are
  reproducible and never hand-edited.
- Establish the final test command dispatcher and machine-readable proof
  ledger. A declared but unavailable boundary reports `NOT_RUN`.
- Prove all three profile homes independently without copying credentials.
- Prove empty `thread/start -> thread/unsubscribe -> remote resume`, exact
  thread/session/hook identity, `SKIDBLADNIR_BINDING_ID` inheritance plus the
  helper's nearest exact pinned-Codex ancestor PID/start time, and the negative
  nested-CLI case in which inherited traffic is counted and discarded. Record
  the pin's native-subagent process/event model; prove child-process traffic is
  discarded and every accepted same-process subagent observation is
  activity-only, never lifecycle/turn/attention authority. Also prove the
  pinned bounded `thread/list` page with cwd, creation time, status,
  `forkedFromId`, and `parentThreadId`; warning-only exclusion of untrusted
  non-managed hooks under `resume --remote` with the reviewed persisted-trust
  set running; the recorded hook discovery layout and exact effective
  managed-requirements source enumeration; readiness and the launcher's
  pre-exec gate reject a foreign lifecycle hook from any such source; and
  Stop-versus-continuation ordering.
- Prove the recorded tmux version's grouped sessions, shadow clients,
  `active-pane`, `ignore-size`, exact pane identity, and last-link behavior.
- Use a minimal platform harness to prove the pinned terminal asset, Unicode,
  ANSI, IME composition, editable dictation, clipboard, and resize on the
  target phone. Retain the harness only if it becomes a real platform test.

### Merge gate

- `./scripts/test static` passes.
- Every architecture Section 3 Lane 0 proof has a credential-free evidence
  record and PASS result. A failed premise results in a reviewed architecture
  correction, not an exception in code.
- `evidence/sources/dvergatal.md` covers every catalogue entry; no key is
  frozen without a producible, reviewed portrait source — a key without one is
  removed before the freeze, not after.
- Shared contracts are frozen for P1; later changes return to Lane 0.

### Not in P0

Registry behavior, SQLite lifecycle state, a long-running gateway, product UI,
deployment, or a fake end-to-end path.

## 3. P1 — one managed laptop agent

### Outcome

Running `codex-personal`, `codex-work`, or `codex-work2` interactively inside
tmux starts exactly one registered Codex thread/TUI in the invoking cwd. Other
commands and outside-tmux invocations retain existing `ai-router` behavior.

### Owns

- Minimum Lane 1 agent, character, Codex-session, runtime-binding, lifecycle,
  Fact, migration, and recovery-projection storage needed by Start, plus the
  `0600` hook socket receiver and `SessionStart` projection.
- Minimum Lane 2 profile supervisor, App Server broker, launcher, exact argv,
  cwd validation, and invoking-pane runtime registration.
- The minimum repo-owned per-profile hook config installation needed for the
  real laptop boundary; P7 owns idempotent re-installation, readiness, and
  recovery hardening.
- A companion PR in `/home/niels/src/personal/dev-server` for the
  dev-server-owned `ai-router` delegation seam, which enters fail-visible mode
  only when the Skíðblaðnir enablement marker is present. That PR pins the
  `@openai/codex` npm version to `codex.lock` — or installs the router without
  re-running global npm installs — so a converge cannot silently move the
  frozen pin. Skíðblaðnir's installer only verifies that versioned seam and
  never edits router source or symlinks.

### Red

- Each profile command initially fails the managed-start proof.
- Wrong home/account/pin, hook drift, unsupported or conflicting launch
  arguments, invalid cwd, missing tmux, or a missing router seam on a
  marker-enabled host fails before TUI exec.
- Exact replay of one `clientCommandId` does not return the original receipt
  and does not hold allocation to exactly one thread and one card.

### Green

- Implement `PrepareNew`, `StartAgent`, exact pre-write dispatch recording,
  terminal `RecoveryRequired` on a lost response, durable refs, runtime
  registration, and `todo.txt` projection.
- `thread/start` carries no prompt. A preserved laptop prompt argument reaches
  only the exact-id TUI after refs and runtime identity are durable.
- Register the invoking tmux pane's immutable `session_id`/`window_id`/`pane_id`
  plus TTY and PID with kernel start time as the runtime binding, then exec the
  exact-id stock remote TUI in that pane under strict config, YOLO, and the
  pre-exec hook digest gate. The launcher creates no tmux session and renames
  nothing.
- Preserve supported interactive model/image/prompt/`-C` arguments; every other
  flag, option, or positional fails visibly before exec.
- Preserve every noninteractive and outside-tmux `ai-router` path byte-for-byte
  at the exec boundary.

### Merge gate

- Live proof for all three profiles: correct account/home/pin, arbitrary cwd
  including spaces, exact thread, one TUI in the invoking pane with the user's
  session and window selection unchanged, stable `ga-*` handle/character, no
  prompt, and no fallback.
- Exact replay returns the original receipt; simulated lost response never
  duplicates a provider write.
- The companion `dev-server` PR is merged and deployed: the installed devbox
  router reports the seam version this PR requires and the router
  classification proof passes against that installed file — a
  merged-but-undeployed seam is `NOT_RUN`, not PASS. The P1 evidence record
  names the dev-server commit sha, the installed router digest, and
  `codex --version` observed immediately before and after the converge that
  deployed the seam, still matching `codex.lock`.

### Not in P1

Resume/adoption, full state derivation, phone API, terminal bridging, Android,
or formal laptop-side Close.

## 4. P2 — lifecycle and continuity

### Outcome

Managed conversations have truthful state and one exact runtime across laptop
resume, dormant adoption, close/reopen, process loss, and host-service restart.

### Owns

- Remaining Lane 1 command reducer, observation reducer, hook reducers,
  attention, retention, idempotency, and reconciliation behavior.
- Remaining non-network Lane 2 broker, liveness, resume/adoption, close/reopen,
  exact process identity, and tmux lifecycle behavior.

### Red

- A Stop hook racing a continuation falsely publishes Idle or attention.
- Restart while a completion candidate is open drops the pending attention or
  raises it twice; a session end racing an open candidate derives Exception
  for a cleanly completed turn; a second prompt during confirmation strands
  the superseded turn open; a lost Stop leaves its completion unconfirmed and
  unraised.
- Two concurrent resumes create two TUIs; two adoptions create two cards.
- Stale or contradictory hook/summary/liveness evidence guesses a state.
- Restart leaves accepted lifecycle commands undriven or replays terminal input.

### Green

- Implement the total state derivation, post-Stop summary confirmation,
  attention acknowledgement/re-arm, gap repair, and stale/contradictory Unknown.
- Implement bounded `ListResumableSessions` and `PrepareResume`.
- A tracked live resume resolves exactly one non-Skíðblaðnir client and the
  gateway switches only it to the registered session, window, and pane;
  ambiguity prints the exact target instead of switching. A verified-dead
  tracked runtime reopens its exact thread.
- Explicitly adopt only untracked, unforked, non-subagent `notLoaded` exact
  threads; reject loaded (`idle`), `active`, `systemError`, missing, tracked,
  forked, subagent, or unverifiable threads with their typed codes. Never
  bulk-import.
- Implement idle-only Close, exact-thread Reopen, recovery state, lifecycle
  resumption, retention, and restart reconciliation without input replay.
  `CloseAgent` and attention acknowledgement are proved at the command-service
  boundary in this PR; their first user-facing caller is P3.

### Merge gate

- Real SQLite/service proofs cover every lifecycle command outcome and race.
- Real tmux/live-Codex proof covers live switch, dormant reopen, explicit
  adoption, idle close, process loss, and restart with one thread/TUI identity.
- Stop alone never yields Idle or attention; continuation and exhaustion derive
  the specified Working/Unknown states. Restart re-arms open confirmation
  schedules and raises each pending completion's attention exactly once; a
  session end after Stop resolves the candidate and derives Recoverable; a
  superseding prompt closes the prior turn without Idle or attention; a
  gap-marked lost Stop is summary-repaired to confirmed Idle with attention;
  reopened and adopted threads derive Idle from their incarnation's
  `SessionStart`.

### Not in P2

Public network API, Android, WSS terminal transport, generic jobs, transcript
storage, or a new laptop `skidbladnir close` surface.

## 5. P3 — phone control plane

### Outcome

An authenticated Tailnet client can pair, bootstrap, observe, start, close, and
reopen agents and inspect host pressure through the closed `/v1` protocol.

### Owns

- Lane 3 gateway, strict HTTP DTOs, pairing/installations, auth, durable SSE,
  bootstrap/paging, command routes, and `/proc` metrics.
- Lane 2 per-incarnation `ga-*` tmux session creation, whose first callers are
  this PR's phone Start and Reopen routes — a stated cross-lane ownership.
- System and integration proofs for SQLite Fact publication, authentication,
  replay, bounds, staleness, and pressure aggregation.

### Red

- Pair races mint multiple installations or permit replay.
- SSE bootstrap/catch-up/live races lose, duplicate, or reorder Facts.
- A revoked/wrong bearer, contract mismatch, invalid cwd, raw target, or
  oversized body crosses ingress.
- Missing `/proc` data renders healthy pressure instead of Unknown.

### Green

- Implement one-time replacement pairing and verifier-only bearer storage.
- Implement every non-terminal `/v1` route exactly as specified, with closed
  errors, limits, idempotent receipts, cursor retention, and immediate
  revocation.
- Implement atomic snapshot/Facts, gap-free SSE, bounded queues, heartbeats,
  overflow bootstrap, and honest stale projection.
- Sample and aggregate CPU/load/memory/swap/disk/PSI without admission or
  scheduling policy.
- Bind loopback and add the minimum repo-owned Tailscale Serve setup/verification
  needed for the real phone boundary. P7 owns idempotent installation,
  readiness, and recovery hardening.

### Merge gate

- Real HTTP + SQLite integration proof covers pairing, rotation/revocation,
  bootstrap, pagination, Start/Close/Reopen, asynchronous failures, attention,
  SSE ordering/reconnect/overflow, restart, and metrics.
- A real Tailnet client reaches `/v1`; loopback bypass, Funnel/public ingress,
  and an unverified Serve mapping do not satisfy the live proof.
- Injection, unknown fields/enums, incompatible contract digest, and off-scope
  App Server/terminal inputs fail with the exact typed result.

### Not in P3

Terminal WSS, Android UI, Funnel/public ingress, notifications, or provider
abstraction.

## 6. P4 — shared terminal transport

### Outcome

An authenticated terminal client can attach and detach from the one registered
tmux TUI while a laptop remains attached, without spawning another Codex
process or changing the laptop's session, pane, focus, draft, or geometry.

### Owns

- Lane 2 PTY/tmux shadow-client implementation and reconciliation.
- Lane 3 authenticated terminal WSS admission, frames, backpressure, liveness,
  and exact-identity checks.
- A protocol-faithful non-Android client used only to prove this boundary.

### Red

- Attaching starts a second TUI, steals the laptop client, resizes its pane,
  exposes a returned shell, or replays bytes/input after reconnect.
- Slow clients grow unbounded queues; stale pane/PID identity still accepts
  input; disconnect leaks shadow sessions.

### Green

- Implement one ephemeral grouped shadow session and gateway-owned PTY per WSS.
- Preserve laptop ownership with `active-pane`/`ignore-size`; report constrained
  geometry and attached-client count honestly.
- Forward bounded opaque terminal bytes only after exact identity checks.
- Create, resize, detach, disconnect, last-link promote, and reconcile shadows
  under the architecture's serialization rules.
- Package no transcript, terminal parser, scrollback replay, or provider token.

### Merge gate

- Real tmux/system proof shows two clients on one pane/PID/thread/draft, isolated
  focus/geometry, independent detach, bounded slow-client failure, no replay,
  shell cutoff, shadow cleanup, and exact-identity rejection.
- Auth/revocation and contract mismatch close live WSS within the stated bound.

### Not in P4

Android rendering, custom terminal semantics, screen-derived state, transcript
history, or multi-writer coordination.

## 7. P5 — Android dashboard

### Outcome

The physical phone pairs with the devbox and presents Hlíðskjálf: real agent
cards, The Forge, lifecycle actions, attention, stale state, and host pressure.

### Owns

- Lane 4 Android application shell, secure bearer storage, REST/SSE client,
  cache, Compose grid/detail/Forge/pressure surfaces, and accessibility.
- Final bundled Dvergatal portraits and static catalogue-to-asset validation.
- Component and physical-device proofs for all non-terminal surfaces.

### Red

- Process recreation loses authority or renders cached truth as fresh.
- SSE replay duplicates cards/attention or misses command failure.
- The Forge accepts invalid cwd or caller-owned character/runtime fields.
- Missing/orphan portrait assets, inaccessible controls, or 200% layout overflow
  pass the build.
- A minified, resource-shrunk release-configuration build renders a blank
  portrait for any card.

### Green

- Implement pairing/repairing, bootstrap, SSE convergence, disposable cache,
  stale/offline presentation, and revocation handling.
- Render the dense Dvergatal card grid, objective/activity/state/attention,
  pressure strip/history, typed failure reasons, and default-open filtering.
- Implement The Forge with profile, objective, editable cwd, and server-owned
  recents; preserve invalid input and never send a prompt.
- Implement Close/Reopen/acknowledge actions with command-settlement feedback.
- Bundle one validated portrait per catalogue key. No asset download or
  phone-side assignment exists. An entry whose portrait cannot be produced is a
  declared Lane 0 return that removes the key before it ships, never a
  placeholder.
- The agent sheet renders the full closed action set; `Open` and attachable
  card tap route to the sheet and render disabled with the stated reason that
  terminal viewing arrives in P6. Record `card-open` and
  `reopen-opens-terminal` as declared deferrals in the proof ledger with status
  `NOT_RUN`, not as absent behavior.

### Merge gate

- Compose/component tests use real client runtime and semantic interactions.
- Physical S22+ proof covers pairing, 15+ cards, two working agents, Start in a
  path containing spaces, state/attention/failure/stale/pressure behavior,
  process recreation, rotation, both navigation modes, TalkBack, Switch Access,
  and 200% scale, run against a minified release-configuration build.
- No terminal, prompt, provider, project, worktree, or raw-target concept leaks
  into the dashboard contract.

### Not in P5

Terminal rendering/input, microphone permission, custom speech recognition,
FCM, self-update, or visual product expansion beyond the architecture.

## 8. P6 — Android terminal and IME

### Outcome

Opening a card on the S22+ shows and controls the exact stock Codex TUI shared
with laptop tmux. Gboard typing, clipboard, and dictation behave like ordinary
editable terminal input and never auto-submit.

### Owns

- Lane 4 vendored/pinned xterm.js assets, locked-down WebView, Kotlin WSS/PTY
  bridge, terminal chrome, accessory keys, lifecycle, and device tests.
- The PTY/WSS-to-Android and laptop-to-phone-to-laptop ownership proofs.

### Red

- Phone attach changes laptop geometry/focus/process or starts another TUI.
- IME composition, Unicode, multiline editing, bracketed paste, or automatic
  terminal replies corrupt/send user text.
- WebView can access network/files/bearer or exposes a general JavaScript bridge.
- Reconnect replays terminal input/output or leaves a shadow client attached.

### Green

- Render the pinned stock TUI through local xterm.js assets and bounded
  `WebMessagePort`; Kotlin alone owns auth/network.
- Implement initial geometry, constrained-mode chrome, accessory keys, safe
  paste, IME composition, editable Gboard dictation, detach, and reconnect.
- Enforce CSP, asset-only loading, disabled file/content/network access, no
  `addJavascriptInterface`, no saved terminal bytes, and release debug posture.
- Add card-to-terminal navigation only with the functional real WSS path.

### Merge gate

- Physical S22+ proof covers ANSI/UTF-8, scrolling/history decision, resize,
  navigation, lifecycle recreation, Gboard typing/clipboard/dictation, mid-line
  correction, multiline prompt before submit, and accessory keys.
- Live handoff proof records one exact pane/PID/thread/draft with simultaneous
  laptop and phone attachment, independent detach, and unchanged laptop state.
- The declared TalkBack terminal limitation is recorded honestly; other Android
  surfaces retain their P5 accessibility pass.
- P5's deferred `card-open` and `reopen-opens-terminal` ledger rows flip to
  PASS in this PR with the terminal route enabled, re-running the card-tap
  routing, closed action-set, and TalkBack proofs.

### Not in P6

Native speech APIs, TTS, transcript reconstruction, terminal-output copy-out,
images, or a second runtime.

## 9. P7 — operations, recovery, and Core acceptance

### Outcome

Install and operate the complete private system on the devbox and S22+ with
fail-visible readiness, restart recovery, and one machine-readable Core verdict.

### Owns

- Remaining Lane 5 composition, process supervision, systemd units, Tailscale
  Serve installation/readiness, installer, and E2E journeys.
- The upgrade and rollback mechanism: ingress close, ordered stop with
  no-process-holds-`CODEX_HOME` verification, `VACUUM INTO` plus
  permission-preserving archives of each `CODEX_HOME`, the installed Codex
  binary, `schemas/codex/<pin>/`, and `codex.lock` under a digest manifest,
  install/generate/probe, reconcile, restore-on-any-failure, and the `0700`
  snapshot deleted after acceptance.
- Final cross-lane recovery, retention, security, resource-lifecycle, and
  acceptance proof closure.

### Red

- Clean install requires an undocumented manual mutation or overwrites
  `ai-router` ownership.
- Gateway/tmux/App Server/phone/Tailnet loss guesses state, leaks authority,
  replays input, or silently substitutes a profile.
- A forced probe failure after snapshot leaves a manifest artifact unrestored,
  or reopens ingress before re-probe.
- `accept core` can pass with an absent device/live proof or stale artifact.

### Green

- Install repository-owned binaries/config/hooks/services idempotently and
  verify, but never own, the dev-server router seam.
- Supervise the gateway and three isolated profile proxies; implement exact
  health/readiness degradation and no fallback.
- Verify loopback binding, Tailscale Serve TLS, Funnel absence, permissions,
  SQLite migrations/pragmas, hooks, tmux configuration, and account/model pins.
- Complete kill/restart recovery, Fact/observation/sample retention, pairing
  replacement, todo projection, operator diagnostics, and log-retention
  operation.
- Make all target test commands and proof-ledger aggregation authoritative.

### Merge gate

- Fresh-devbox install and restart/upgrade procedures are repeatable from repo
  state plus existing account authentication only.
- One recorded upgrade rehearsal — snapshot, forced probe failure, restore,
  `./scripts/test accept upgrade` PASS — with a content-free evidence record
  naming both pins and every restored artifact digest.
- Every architecture Section 9 ownership-boundary proof is PASS on the exact
  devbox and physical phone.
- `./scripts/test full` and `./scripts/test accept core` are PASS with no
  `NOT_RUN`; the worktree is clean and installed artifacts match the commit.

### Not in P7

FCM, A/B/self-update, off-host backup, permanent purge, native voice, images,
generic providers, orchestration, or any other v1.1 item.

## 10. Subagent execution

The root agent owns scope, shared contracts, merge order, final integration, and
the acceptance claim. Use at most three concurrent subagents, each with one
bounded assignment and non-overlapping paths:

| Role | Typical ownership | Must not change |
| --- | --- | --- |
| Domain | Lane 1 registry/storage/hooks and its tests | API schema, runtime, Android |
| Runtime/control | Lane 2 or Lane 3 for the current PR and its tests | Shared contracts, Android |
| Android | Lane 4 and its component/device tests | Host implementation, API schema |
| Independent verifier | Re-runs the PR's gates from a clean worktree, reproduces the red observation, audits evidence records, proposes missing proofs to the owning builder | Every file except its review/evidence record under `evidence/` |

Only two builder roles plus one independent verifier run concurrently with the
root. A builder owns its red test as well as its implementation; the verifier
does not backfill tests after code. Each subagent reads `AGENTS.md`, the relevant
architecture sections, this PR spec, and routed rules before acting.

Use one root-owned integration worktree per PR. Optional subagent worktrees start
from that PR's exact base and return bounded commits for root review/cherry-pick.
Never let two agents edit a shared or generated file. Delete only verified-clean
temporary worktrees after their commits are integrated.

## 11. Scope and progress control

- Track each PR as `Pending | InProgress | Blocked | Merged`; only the root
  changes status in this file. The status table is the only part of this file
  changed during a PR; any change to a PR's outcome, Owns, Red, Green, or merge
  gate is a roadmap amendment made by the root before that PR starts, recorded
  in the PR that makes it.
- A new user capability requires an architecture acceptance criterion before it
  enters a PR. A discovered implementation task stays inside the owning PR only
  when it is necessary for that PR's existing outcome.
- Do not pull v1.1 work forward because a nearby abstraction could support it.
- Do not claim schedule progress from scaffolding, generated code, fixture-only
  tests, or an unrun physical boundary.
- P7 closes Core. Anything remaining after P7 is either a failed acceptance
  gate or an explicitly re-scoped future capability.

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
