# Skíðblaðnir v0 roadmap

Status: scope reset approved 2026-08-25; S1 pending. Supersedes the P0–P7
roadmap (git history through `6f2d697`); the `codex/p1-managed-agent` branch
and its worktree implement the superseded architecture and are abandoned, not
merged.

This document owns delivery order. The [architecture](architecture.md) owns
behavior and acceptance. Three slices, each an ordinary PR with one observable
outcome, a red proof observed before implementation, and real-boundary tests.
No proof ledger, evidence digests, or acceptance matrix — `NOT_RUN` honesty
and the closed logger survive; the ceremony does not.

```text
S1 tmux control plane
 -> S2 shared terminal
 -> S3 Android dashboard
 -> v0.5 (optional): trust-by-default status hooks; push
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
  card facts incl. user-option metadata, status chips with age, client count.
- `POST /v1/sessions`: cwd validation, profile-table allowlist, `ga-<dwarf>`
  naming from free catalogue keys, user options set at create, YOLO exec.
- `DELETE /v1/sessions/{id}`: id+name re-verified immediately before
  `kill-session`.
- `GET /v1/pressure` with the proven thresholds.
- `skid-notify` script + idempotent per-profile `notify` config line;
  `@skid_attention` set/cleared as specified.

Red: a kill with a stale id/name pair destroys the wrong session; a non-
allowlisted command launches; a bad cwd mutates state; an unauthenticated or
off-Tailnet request reaches `/v1`; a laptop-created session is invisible or
guessed at.

Gate: unit + real-tmux integration on an isolated `-L` socket covering
create/list/metadata/kill-exactness/notify; no Android required.

## S2 — shared terminal

Outcome: a phone-shaped client attaches to any listed session while the
laptop stays attached, then detaches leaving everything intact.

- WSS endpoint per architecture §5: one PTY + gateway-owned phone client +
  grouped shadow per connection; `active-pane`/`ignore-size`;
  session/window-level targeting with key-sequence pane selection through the
  phone PTY; last-link guard; OWNER/CONSTRAINED presence; typed
  `Reconnect required` on loss.
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

- Bearer entry, grid with portraits/status/attention/pressure strip, Forge,
  terminal screen on the existing harness, detach-vs-kill UI per architecture
  §4/§6.
- Stale/reconnect states shown honestly; process recreation returns to the
  grid and reattaches fresh.

Red: kill and detach are confusable; a card claims exact semantic state; a
recreated process replays input; the bearer reaches the WebView.

Gate: physical-device pass per architecture §9 acceptance, including
concurrent laptop/phone attach with unchanged laptop view, IME/dictation,
rotation, reconnect.

## v0.5 — optional, after v0 is in daily use

- Trust-by-default `Stop`/`UserPromptSubmit` hooks writing `@skid_status` via
  `$TMUX_PANE` (~50 lines, per-pane, no provenance).
- Push: FCM or ntfy delivery of the attention signal — redacted, deep-linked,
  deduped via a user option, with a late-send poller sweep.

Tier-3 machinery (authenticated provenance, exactly-once attention, durable
receipts, thread identity) returns only for push-to-act or unattended
orchestration, via a new architecture decision.

## Status

| Slice | Status |
| --- | --- |
| S1 tmux control plane | Pending |
| S2 shared terminal | Pending |
| S3 Android dashboard | Pending |
| v0.5 hooks + push | Not scheduled |
