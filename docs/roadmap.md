# Skíðblaðnir v0 roadmap

Status: scope reset approved 2026-08-25; S1–S3 and the corrective delta are
implemented. The 2026-08-26 multi-machine hard cut is also implemented and
accepted: routine verification, isolated Linux and Darwin tmux/API, live
Devbox and MacBook publication, 15-test S22+ platform instrumentation, and the
physical two-host outage/recovery journey are green. The earlier corrective
delta's named Codex hook-digest and hands-on terminal/Gboard checks remain
`NOT_RUN`; federation acceptance does not substitute for them.
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
 -> v0 multi-machine hard cut: Devbox + MacBook federation
 -> v0 fixed-topology dashboard cut: external provisioning + dense dashboard
 -> v0.5 (optional): push
```

## S1 — tmux control plane

Initial Devbox foundation, later generalized by the multi-machine hard cut
without retaining a one-host branch.

Outcome: an authenticated Tailnet client lists, creates, and kills one host's
tmux sessions and reads its pressure.

- Go gateway: loopback bind, Tailscale Serve mapping, host-minted bearer
  (constant-time check; re-mint revokes).
- `GET /v1/sessions`: poller over `list-sessions`/`list-panes` + `/proc`;
  card facts incl. user-option metadata, independent attention, status chips
  with age, client count. Exact Codex `WORKING|IDLE` comes only from the narrow
  process-lifetime-bound hook adapter; otherwise a live agent is `RUNNING`.
- `POST /v1/sessions`: cwd/name/objective validation, profile-table allowlist,
  unbounded `ga-<dwarf>[-N]` naming with catalogue reuse, user options set at
  create, YOLO exec.
- `DELETE /v1/sessions/{id}`: inventory `identityToken` binds a random
  tmux-server epoch + built-in PID/start time + id; all lifetime facts, the
  displayed name, ungrouped-or-last-link predicate, and `kill-session` share
  one tmux client queue; owned stale phone shadows reconcile first, while any
  remaining ordinary group fails closed.
- `GET /v1/pressure` with the proven thresholds.
- `skid-notify` script + idempotent per-profile `notify` config line;
  `@skid_attention` is surfaced; opening-and-clearing belongs to S2.
- Exact per-profile Codex lifecycle hook file; native digest review remains
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
  rotation moves/scales the outer viewport.
- Green: pure status and process-lifetime proofs; exact tmux RGB command-shape
  proof; compiled Android regression for 80-column geometry, rendered colored
  pixels, and native/page/IME containment.
- Acceptance: isolated tmux integration, exact devbox install/verify and Codex
  digest review, platform instrumentation, then hands-on S22+ prompt/Stop,
  right-edge Gboard/dictation, color, and portrait/rotation checks.

## v0 multi-machine hard cut — delivered

Outcome: one Android collection directly federates independent Devbox and
MacBook gateways without a coordinator, shared credential, durable inventory,
or cross-machine operation.

- Added immutable installation handles and authenticated machine binding to
  the hard-cut API envelope; sessions remain local and are addressed by full
  machine target.
- Added the Darwin process/pressure capability, exact tmux/platform pins,
  LaunchAgent installer, isolated bearer, and separate lifecycle assets while
  preserving the Linux gateway behavior.
- Replaced singular Android credential/origin state with encrypted explicit
  pairings, per-machine polling/failure/admission, visible machine identity,
  machine-first Forge, one exact machine-bound terminal, and opaque per-entry
  quarantine that never trusts a failed entry's stored destination.
- Removed the old envelope/store/UI path with no parser, migration, fallback,
  or mixed-version window.

Acceptance: routine verification; the same isolated tmux/API suite on Linux
and Darwin; live Devbox and MacBook install/reinstall; S22+ encrypted pairing
and lifecycle instrumentation; then one physical two-host read-only tmux
journey proving Activity recreation, machine-local outage fencing, healthy-host
progress, recovery, and unchanged pairings/lifetimes. All passed on 2026-08-26.

## v0 fixed-topology dashboard cut — delivered

Outcome: the provisioned Devbox and MacBook remain first-class runtime targets,
while machine administration leaves the app and the dense v0 dashboard returns.

- Removed app add, rename, healthy remove, and quarantine-clear capabilities;
  bearer repair remains bound to an existing immutable machine.
- Collapsed the dashboard chrome to one 64 dp row with `New agent` trailing and
  removed the duplicate create row.
- Restored each machine's current pressure metrics, 15-minute severity history,
  missing inputs, platform-unsupported metrics, and pressure reasons.

Acceptance: red pressure component proof, routine static/unit verification, the
exact 16-test S22+ platform gate, exact live gateway publication, and the
physical healthy/outage/recovery journey are green.

## v0 review corrective layer

Outcome: an adversarial review of the multi-machine and fixed-topology cuts
against the reviewed contract and the repository rules, with every confirmed
finding fixed at the source.

- Darwin ancestry observation now ends cleanly at the privilege/lifetime
  frontier instead of failing at root-owned `launchd`, so the Mac status hook
  can publish `@skid_lifecycle` at all; the faked hook proof in the
  integration suite was replaced by one that executes the real hook binary.
- Gateway, installer, dashboard, store, and test-suite findings (error-cause
  fidelity, ingress uniqueness quarantine, IPv6 origins, machine-bound bearer
  repair, exhaustive sealed matching, single-owner dedup, honest gate
  detection of skipped journeys, both-host read-only guards) fixed per
  `docs/rules`.

Acceptance: routine static and unit gates; the isolated integration suite on
Linux and Darwin; the Linux live grouped-session proof; exact Devbox and
MacBook install/reinstall publication; the exact 16-test S22+ platform gate;
and the physical healthy/outage/recovery journey are green. These external
runs re-prove both acceptance rows above against the corrected tree.

## v0.5 — optional, after corrected v0 is in daily use

- Push: FCM or ntfy delivery of the attention signal — redacted, deep-linked,
  deduped via a user option, with a late-send poller sweep.

Tier-3 machinery (authenticated provenance, exactly-once attention, durable
receipts, thread identity) returns only for push-to-act or unattended
orchestration, via a new architecture decision.

## Status

| Slice | Status |
| --- | --- |
| S1 tmux control plane | Implemented; corrective lifecycle source, isolated integration/live proof, and devbox install/verify green; Codex hook-digest review and hands-on status proof `NOT_RUN` |
| S2 shared terminal | Implemented; corrective RGB command shape and isolated integration/live proof green; renewed concurrent physical handoff `NOT_RUN` |
| S3 Android dashboard | Implemented; 9-test S22+ platform gate green, including viewport/geometry/rendered color; renewed hands-on Gboard/dictation proof `NOT_RUN` |
| v0 corrective delta | Source implemented; routine verification and automated external gates green; named hands-on checks above remain `NOT_RUN` |
| v0 multi-machine hard cut | Implemented, published to Devbox and MacBook, paired on S22+, and green across routine, both host, platform, and physical product gates |
| v0 fixed-topology dashboard cut | Implemented and published; routine, exact 16-test S22+ platform, and physical healthy/outage/recovery product gates green on the corrected tree |
| v0 review corrective layer | Implemented and published; routine, both-host integration, Linux live, exact 16-test S22+ platform, and physical product gates green |
| v0.5 push | Not scheduled |
