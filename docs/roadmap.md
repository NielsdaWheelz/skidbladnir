# Skíðblaðnir v0 roadmap

Status: scope reset approved 2026-08-25; S1–S3 source implemented. Daily use on
2026-08-26 reopened three acceptance defects: horizontal Gboard drift, tmux-
activity-derived status, and a colorless/narrow terminal. Their corrective
source delta is implemented and routine verification is green. Isolated tmux,
current devbox deployment, and automated S22+ platform acceptance passed on
2026-08-26 under their explicit gates. Daily use then exposed a lifecycle-hook
boundary defect: the helper walked beyond its terminal session into PID 1, and
deployment preflight rejected Codex's native hook-state table. Its source repair
and routine verification are green. Transactional deployment and repeatable
verification, the owned hook-file digest, isolated tmux/live gates, and an
installed lifecycle-producer smoke are also green. The automated 9-test S22+
platform gate is green; real Codex event delivery and renewed hands-on S22+
acceptance remain `NOT_RUN`. The Claude Work profile source delta, routine
verification, isolated tmux integration, exact devbox install/reverify,
automated S22+ platform gate, and user-reported focused S22+ pass are green.
The automatic dwarf-identity hard cut is source implemented and routine
verification is green; isolated real-tmux acceptance remains `NOT_RUN`.
Supersedes the P0–P7 roadmap (git history through `6f2d697`); the
`codex/p1-managed-agent` branch and its worktree implement the superseded
architecture and are abandoned, not merged.

This document owns delivery order. The [architecture](architecture.md) owns
behavior and acceptance. Each slice has one observable outcome, a red proof
observed before implementation, and real-boundary tests.
No proof ledger, evidence digests, or acceptance matrix — `NOT_RUN` honesty
and the closed logger survive; the ceremony does not.

Routine verification contains no tmux or device mutation. Each real-tmux or
physical-device gate is invoked separately only after explicit user approval in
that same turn; otherwise it remains `NOT_RUN`.

```text
S1 tmux control plane
 -> S2 shared terminal
 -> S3 Android dashboard
 -> v0 corrective delta: lifecycle truth + terminal viewport/color
 -> v0 profile delta: Claude Work
 -> v0 identity delta: automatic dwarf identity
 -> v0.5 (optional): push
```

## Repo transition (start of S1)

- Retain: `catalog/` (Dvergatal), `android/` harness and its device evidence,
  `evidence/live/` records (historical platform evidence, no longer gate
  inputs), `internal/logging`, tmux findings/tests, `scripts/test` skeleton.
- Prune: hook schemas and `skidbladnir-hook`, hook-trust artifacts under
  `deploy/codex/`, `cmd/skidbladnir-contract`, `api/*.lock` digests,
  `generated/`, router-seam code, `codex.lock` enforcement, proof-ledger
  wiring in `scripts/test`, and the superseded P1 packages.
- `api/skidbladnir.v1.json` shrinks to the five routes with hand-written DTOs
  or is dropped entirely; no codegen either way.

## S1 — tmux control plane

Outcome: an authenticated Tailnet client lists, creates, and kills tmux
sessions and reads pressure.

- Go gateway: loopback bind, Tailscale Serve mapping, single bearer
  (devbox-minted CLI, constant-time check, re-mint revokes).
- `GET /v1/sessions`: poller over `list-sessions`/`list-panes` + `/proc`;
  card facts incl. required character metadata, independent attention, status
  chips with age, client count. Exact Codex `WORKING|IDLE` comes only from the
  narrow process-lifetime-bound hook adapter; otherwise a live agent is
  `RUNNING`. Inventory assigns a persistent Dvergatal key to every visible
  ordinary session with missing or invalid character metadata.
- `POST /v1/sessions`: cwd/tmux-name/objective validation, profile-table
  allowlist, independent character allocation, default
  `skidbladnir-<profile>-<N>` naming, user options set at create, exact row
  command/environment/argument exec.
- `DELETE /v1/sessions/{id}`: inventory `identityToken` binds a random
  tmux-server epoch + built-in PID/start time + id; all lifetime facts, the
  displayed name, ungrouped-or-last-link predicate, and `kill-session` share
  one tmux client queue; owned stale phone shadows reconcile first, while any
  remaining ordinary group fails closed.
- `GET /v1/pressure` with the proven thresholds.
- `skid-notify` script + idempotent per-Codex-profile `notify` config line;
  `@skid_attention` is surfaced; opening-and-clearing belongs to S2.
- Exact per-Codex-profile lifecycle hook file; native digest review remains
  manual and fail-visible, and the helper accepts only the pane foreground
  Codex origin.

Red: a kill with a stale lifetime token (including server restart plus id/name
reuse) destroys the wrong session; a grouped ordinary sibling lets Kill claim
that surviving work ended; a non-allowlisted command launches; a bad
cwd mutates state; an unauthenticated or
off-Tailnet remote request reaches `/v1`; a laptop-created session is
invisible or guessed at.

Gate: unit + real-tmux integration on an isolated `-L` socket covering
create/list/metadata/kill-exactness, stale-shadow recovery, ordinary-group
refusal without mutation, a command-boundary name collision, and notify; no
Android required.

## S2 — shared terminal

Outcome: a phone-shaped client attaches to any listed session while the
laptop stays attached, then detaches leaving everything intact.

- WSS endpoint per architecture §5: one PTY + gateway-owned phone client +
  grouped shadow per connection; `active-pane`/`ignore-size`;
  explicit tmux RGB client capability;
  session/window-level targeting with later navigation through the phone PTY;
  last-link guard; OWNER/CONSTRAINED presence; typed
  `Reconnect required` on loss.
- The inventory identity token travels in the non-query terminal header; one
  tmux queue gates shadow creation, attachment, and attention clearing on the
  exact server lifetime/id/name before any mutation.
- One protocol-faithful non-Android test client.

Red: attach steals or resizes the laptop's view or moves the shared active
pane; detach or WSS teardown kills the source session; a slow client grows
unbounded; bytes replay after reconnect.

Gate: real-tmux proof with two live clients — source window active pane and
laptop geometry byte-identical across phone attach/detach; last-link guard
exercised; bounded backpressure.

## S3 — Android dashboard

Outcome: the physical S22+ pairs, shows the grid, forges sessions, opens the
shared terminal, and kills with confirmation.

- Bearer entry, grid with procedural dwarf icons/status/attention/pressure
  strip, Forge, terminal screen on the existing harness, detach-vs-kill UI per
  architecture §4/§6.
- Stale/reconnect states shown honestly; process recreation returns to the
  grid and reattaches fresh.
- Terminal maintains at least 80 visible columns, renders ANSI/true color, and
  prevents horizontal movement at both the xterm/IME and native WebView layers.

Red: kill and detach are confusable; a card claims exact semantic state; a
recreated process replays input; the bearer reaches the WebView; a long IME
composition pans the terminal viewport horizontally.

Gate: physical-device pass per architecture §9 acceptance, including
concurrent laptop/phone attach with unchanged laptop view, IME/dictation,
rotation, reconnect.

## v0 corrective delta — field acceptance

- Red: a phone attach/input redraw claims semantic work; a fresh open/close
  turns an idle agent active; stale attention remains during a new prompt; a
  prior process lifetime supplies status to a replacement process; ANSI output
  is monochrome; portrait reports fewer than 80 columns; long Gboard input or a
  rotation moves/scales the outer viewport; a lifecycle hook walks beyond its
  terminal session or can publish to a pane on another terminal; Codex's native
  hook state blocks an otherwise valid deploy, or a deprecated disable alias
  passes preflight.
- Green: pure status, process-lifetime, target-terminal, terminal-session
  ancestry, and strict hook-config policy proofs; exact tmux RGB command-shape
  proof; compiled Android regression for 80-column geometry, rendered colored
  pixels, and native/page/IME containment.
- Acceptance: isolated tmux integration, exact devbox install/verify and Codex
  digest review, platform instrumentation, then hands-on S22+ prompt/Stop,
  right-edge Gboard/dictation, color, and portrait/rotation checks.

## v0 profile delta — Claude Work

Outcome: The Forge launches the work-account Claude CLI as the fourth closed
profile while retaining the existing tmux, API, terminal, and kill contracts.

- Ordered profiles are `personal`, `work`, `work2`, `claude-work`, with
  provider-qualified labels supplied by the gateway.
- Claude launches only `/home/niels/bin/claude-work` with
  `CLAUDE_CONFIG_DIR=/home/niels/.claude-work` and
  `--permission-mode auto`; callers cannot override the capsule.
- Exact argv[0] `/home/niels/.local/bin/claude` identifies the foreground
  process as `RUNNING/Process`. Claude has no lifecycle hook or semantic-state
  inference.
- Forge, card, and destructive copy are provider-neutral; unknown profile keys
  stay explicitly unknown.

Red: ordered production-capsule proof, exact argv[0] near misses, Android
declared-label/selection behavior, and the existing isolated API/tmux journey
extended through Claude advertise/create/list while retaining detach and exact
kill proofs.

Gate: routine verification; one isolated real-tmux integration journey; one
focused S22+ pass of the stock Claude permission UI, shared terminal,
concurrent attach, reconnect, detach, and exact kill. External gates remain
`NOT_RUN` without their explicit current-turn approval.

## v0 identity delta — automatic dwarf identity

Outcome: every visible ordinary tmux session has one persistent Dvergatal
character independent of its operator-owned tmux name, with no management UI.

- Inventory repairs missing or invalid `@skid_character` values after phone-
  shadow reconciliation and before returning cards; valid values never change.
- Assignment counts valid characters on visible ordinary sessions, chooses a
  least-used catalogue entry, and uses a stable SHA-256 score to break ties.
- Each repair is one exact conditional tmux queue bound to the server lifetime,
  session id, observed option value, and absence of the shadow marker.
- Create chooses tmux name and character independently. Generated names are
  `skidbladnir-<profile>-<N>`; the API hard-cuts to `tmuxName` and
  `optionalTmuxName`; every successful card has a required character.
- Android keeps the existing procedural portrait and adds no action or state.

Red: pure allocation/name behavior, required gateway and Android contracts,
and one isolated real-tmux inventory journey covering persistence, invalid
repair, concurrent assignment, shadow exclusion, and non-character immutability.

Gate: routine verification plus one isolated real-tmux integration journey.
No platform or live gate is added.

## v0.5 — optional, after corrected v0 is in daily use

- Push: FCM or ntfy delivery of the attention signal — redacted, deep-linked,
  deduped via a user option, with a late-send poller sweep.

Tier-3 machinery (authenticated provenance, exactly-once attention, durable
receipts, thread identity) returns only for push-to-act or unattended
orchestration, via a new architecture decision.

## Status

| Slice | Status |
| --- | --- |
| S1 tmux control plane | Implemented; terminal-session/native-hook-state repair, routine proof, deploy/reverify, isolated integration/live, hook-file digest, and installed producer smoke green; real Codex delivery and hands-on status proof `NOT_RUN` |
| S2 shared terminal | Implemented; corrective RGB command shape and isolated integration/live proof green; renewed concurrent physical handoff `NOT_RUN` |
| S3 Android dashboard | Implemented; 9-test S22+ platform gate green, including viewport/geometry/rendered color; renewed hands-on Gboard/dictation proof `NOT_RUN` |
| v0 corrective delta | Source implemented; routine, host external, and automated physical-device proof green; named real-Codex/hands-on checks remain `NOT_RUN` |
| v0 profile delta — Claude Work | Source, routine verification, isolated tmux integration, exact devbox install/reverify, 9-test S22+ platform gate, and user-reported focused S22+ acceptance green; the integration red was not observed before implementation |
| v0 identity delta — automatic dwarf identity | Source implemented; routine verification and isolated real-tmux acceptance green |
| v0.5 push | Not scheduled |
