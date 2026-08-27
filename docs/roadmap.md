# Skíðblaðnir v0 roadmap

Status: scope reset approved 2026-08-25; S1–S3 and the corrective delta are
implemented. The 2026-08-26 multi-machine hard cut is also implemented and
accepted: routine verification, isolated Linux and Darwin tmux/API, live
Devbox and MacBook publication, create-only fixed-collection provisioning,
27-test S22+ platform instrumentation, and the physical two-host
outage/recovery journey are green. The earlier corrective
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
 -> v0 review corrective layer
 -> v0 profile delta: Devbox Claude Work
 -> v0 identity delta: automatic dwarf identity
 -> v0 terminal key-deck delta
 -> v0 design delta D1: terminal theme
 -> v0 design delta D2: chrome tokens
 -> v0 design delta D3: dwarf seals
 -> v0 design delta D4: ornament
 -> v0 product-language delta: dwarves
 -> v0 design delta D8: the Hlíðskjálf mark
 -> v0 design delta D6: destructive and notice chrome
 -> v0 design delta D5: the Forge seal
 -> v0 dashboard pull-to-refresh delta
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
- `POST /v1/sessions`: cwd/tmux-name/objective validation, host profile-table
  allowlist, unbounded `skidbladnir-<profile>-<N>` generated names, independent
  balanced Dvergatal assignment, user options set at create, YOLO exec.
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
- Collapsed the dashboard chrome to one 64 dp row with the primary create
  action trailing and removed the duplicate create row.
- Restored each machine's current pressure metrics, 15-minute severity history,
  missing inputs, platform-unsupported metrics, and pressure reasons.

Acceptance: red pressure component proof, routine static/unit verification, the
exact 27-test S22+ platform gate, exact live gateway publication, and the
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
MacBook install/reinstall publication; the exact 27-test S22+ platform gate;
and the physical healthy/outage/recovery journey are green. These external
runs re-prove both acceptance rows above against the corrected tree.

## v0 profile delta — Devbox Claude Work — delivered

Outcome: the Devbox Forge advertises and launches the work-account Claude CLI
as a fourth closed profile without widening the MacBook profile set or any raw
launch surface.

- Ordered Devbox choices are `personal`, `work`, `work2`, `claude-work`; the
  MacBook exposes only its three Codex capsules. Provider-qualified labels are
  supplied by each gateway.
- Claude launches only `/home/niels/bin/claude-work` with
  `CLAUDE_CONFIG_DIR=/home/niels/.claude-work` and
  `--permission-mode auto`; callers cannot override the capsule.
- Exact argv[0] `/home/niels/.local/bin/claude` identifies the foreground
  process as `RUNNING/Process`. Claude has no lifecycle or attention adapter.

Acceptance: pure host/profile composition and exact-selector proofs, the
isolated real-tmux/API journey on the Devbox, exact Devbox republication, and
the S22+ declared-label/selection surface.

## v0 identity delta — automatic dwarf identity — delivered

Outcome: every visible ordinary tmux session has one persistent Dvergatal
character independent of its operator-owned tmux name, with no management UI.

- Inventory normalizes missing/invalid character metadata through one
  race-safe tmux compare-and-set boundary and never assigns phone shadows.
- `tmuxName`, `optionalTmuxName`, and required `character` are one hard-cut Go
  and Android contract. No legacy `name`, optional character, parser, or
  fallback remains.
- Generated names use `skidbladnir-<profile>-<N>` and character selection is
  independently balanced and stable for the session lifetime.

Acceptance: unit ownership proofs plus the same approved isolated real-tmux
journey on Linux and Darwin.

## v0 terminal key-deck delta — delivered

Outcome: the terminal exposes one ordered, accessible input-only deck without
mixing navigation or detach actions into terminal bytes.

- Fixed order: `Esc`, one-shot `Ctrl`, `Tab`, `Line break`, arrows, `Home`,
  `End`; top detach and Android Back remain lifecycle actions.
- Ctrl state is owned by the page/native terminal boundary, is visible and
  spoken, and resets on the next input, focus loss, rotation, kill dialog,
  detach, reconnect, or page disposal.
- IME composition, dictation, paste, resize, queue bounds, and WSS contracts
  remain unchanged.

Acceptance: pure/native terminal proofs, routine verification, and the exact
S22+ platform gate.

## v0 design deltas — Niðavellir adoption

[`design-language.md`](design-language.md) owns the visual identity these
deltas adopt; each has its own implementation-boundary spec, red proofs, and
gates, and lands as its own change. Landing order is D1 → D2 → D3 → D4 → D8 →
D6 → D5: D1 is independent; D2 creates the token system every later one
consumes; D5 lands last because it reshapes the dashboard chrome the others
established. D7, the launcher mark, is specified on its own branch and has not
landed — the numbers are delta numbers, not positions. No gateway, tmux, public
API, or input-semantics work appears in any of them.

### D1 — terminal theme

Outcome: the terminal renders the derived Niðavellir ANSI/ITheme table in
vendored JetBrains Mono behind the unchanged bundle-only CSP posture.
Scope and ownership per [terminal-theme.md](terminal-theme.md). Gate: routine
verification plus one separately approved platform pass; no integration gate
for this unchanged transport boundary.

### D2 — chrome tokens

Outcome: one owned token file (color, shape, type, motion, interaction
states) and every Compose surface — pairing, grid, Forge, terminal chrome,
key deck styling — consuming it; `SHELL` becomes visually distinct from
`RUNNING`. Scope and ownership per [chrome-tokens.md](chrome-tokens.md).
Gate: routine verification plus one separately approved platform pass.

### D3 — dwarf seals

Outcome: the procedural portrait becomes the deterministic Niðavellir seal
(octagon, mineral field, Younger-Futhark bind-rune, angular figure), a pure
function of `character.key`. Scope per [dwarf-seals.md](dwarf-seals.md).
Gate: routine verification; the named 48dp distinguishability check is a
hands-on device pass and stays `NOT_RUN` until approved.

### D4 — ornament

Outcome: the build-time-generated Forge fret band, the valknut empty-state
mark, and the adaptive app icon.
Scope per [ornament-pipeline.md](ornament-pipeline.md). Gate: routine
verification plus icon/ornament checks folded into the next approved platform
pass.

### D6 — destructive and notice chrome

Outcome: Ember stops meaning five things. One severity type
(`NoticeTone`/`noticeToneColor`) owns every failure, degradation, and armed
recovery; staleness moves from Ember to Muted while trust events stay loud;
the kill control gains `Cleft`, the only asymmetric shape in the product, so
architecture's "detach and kill are visibly different actions" survives
greyscale without an icon. Three hand-rolled banner constructions collapse to
one `NoticePanel` and two hand-rolled kill buttons to one `KillButton`, whose
`contentDescription` finally distinguishes the kill controls in a grid that
previously all spoke a bare "Kill". Zero strings change.
Scope per [destructive-chrome.md](destructive-chrome.md). Numbered 6 because
5 is claimed by a Forge-seal delta specified on its own branch. Gate: routine
verification, plus the three rendered proofs (cleft asymmetry, disabled cue
and spoken target, notice-panel consumers) folded into the next approved
platform pass.

### D5 — the Forge seal

Outcome: the create action leaves the dashboard header and becomes a
bottom-trailing 56dp octagonal control carrying the unstruck seal — the D3
seal with every trait at zero — lit when a machine can create and cold when
none can. Scope per [forge-seal.md](forge-seal.md). It composes with the
dashboard pull-to-refresh delta by concern: pull-to-refresh owns the
collection gesture and inventory intent, D5 owns the dashboard's action chrome,
and each proves only its own half. Gate: routine verification plus one
separately approved platform pass; the mark's legibility and the lit/cold
glance are hands-on and stay `NOT_RUN` until approved.

## v0 product-language delta — dwarves

Outcome: the UI names each persistent session persona a dwarf while preserving
literal tmux-session language wherever process lifetime or destructive safety
matters.

- Dashboard, Forge, navigation, and pairing copy use `Dwarf` / `Dwarves` for
  the Dvergatal-backed persona.
- Detach and recovery copy name the `session`; `agent` is reserved for the
  opaque foreground terminal program.
- HTTP `/sessions`, tmux identity, internal target types, and test selectors do
  not change.

Red: a `SHELL` card is still called an agent even though no agent process is
present, and Detach promises that a transient agent rather than the tmux
session keeps running.

Acceptance: focused Android unit proof for detach lifetime copy, compiled
instrumentation assertions for dashboard language, routine verification, and
the next separately approved platform pass for rendered-device confirmation.

## v0 design delta D7 — the launcher mark

Outcome: the launcher icon is Skíðblaðnir under sail — cut stems, a square
sail in three gores, a Garnet shield row, a masthead vane — and the whole
adaptive-icon resource set is generated from authored polygon constants and
drift-gated as one unit. Scope per [launcher-mark.md](launcher-mark.md).

- `scripts/gen-ornament` emits the background, foreground, monochrome, and
  `<adaptive-icon>` descriptor; the Bézier interpreter and the traced clip-art
  prow it digested are deleted.
- The generator raises rather than emitting a feature below the 2dp-at-48dp
  legibility floor or a point outside the 34-unit mask envelope.
- The monochrome layer is its own drawable, separated by geometry. Both
  layers carry the same sail, whose gores stand apart across a derived slit;
  the shield row is the one feature they cut differently — colour in the
  foreground, notches in the sheer in the monochrome — so one flat tint cannot
  collapse the mark into a blob.
- `android:roundIcon` is gone: at `minSdk = 36` it duplicated the same
  adaptive resource, and no legacy density mipmap exists or is added.

Red: seams moved to `t = 0.25, 0.75` narrow the derived gore slit to 2.0
units and fail the check at 1.33dp; the yard raised to `y = 29.5` fails it at
`r = 35.468`, outside the envelope; the vane as first frozen fails it at
0.66dp against the yard.

Acceptance: routine verification including the ornament drift gate over the
five generated files; one separately approved S22+ platform pass carrying the
instrumented proof that the installed icon's monochrome layer cuts daylight
where the foreground carries structure in colour and still keeps most of the
mark; one hands-on glance that the mark reads as a ship at arm's length.

## v0 design delta D8 — the Hlíðskjálf mark

Outcome: the valknut marks the Dwarves surface wherever that surface is named,
and its weave is legible at every size it renders — which it was at none of
them before.

- The mark leads the `Dwarves` title at 24dp in Gold, leads both `Back to
  Dwarves` affordances at 18dp in their button's own content colour, and keeps
  the empty grid at 48dp in Muted at 40%. One composable renders all four.
- The generator's crossing break becomes `_VALKNUT_GAP = 0.36`, the first width
  that leaves no surviving strand shorter than the stroke is wide, and
  `drawValknut` scales its stroke with the mark instead of fixing it at 2dp.
- The detach control gets no mark: it names what happens to the session, not
  where the button goes.

Numbered D8 behind three deltas specified on their own branches, none of them
on main when this was written — D5 the forge seal, D6 destructive chrome, D7
the launcher mark. D8 landed first, and D7 sequences behind it because both
edit `scripts/gen-ornament`.

Red: the mark for the dwarves renders only when there are none, and at the one
size it did render its six crossings closed to about a physical pixel — the
shortest strand was `1.37dp` long under a `2dp` stroke.

Acceptance: two JVM geometry proofs holding the legibility invariant against
the stroke ratio rather than against a size, an instrumented proof that the
top-bar mark leads the literal title and stays semantics-silent, the existing
ornament drift gate, routine verification, and the next separately approved
platform pass plus one hands-on 18dp glance for rendered-device confirmation.
Scope per [hlidskjalf-mark.md](hlidskjalf-mark.md).

## v0 dashboard pull-to-refresh delta

Outcome: the dashboard removes its header `Refresh` action and makes standard
pull-to-refresh over the dwarf collection the sole manual inventory shortcut.

- `All` targets every live machine poller; a machine filter targets only that
  machine. Pressure, mutations, and terminal input remain outside the intent.
- Empty and populated states share one lazy collection. Fixed chrome stays in
  place and the last snapshot remains visible while checking.
- An awaited monotonic inventory-read sequence prevents a pre-pull poll result
  from finishing a newer pull; the existing per-machine coalescing lane owns
  the one required trailing read.
- No tap, overflow, contextual Retry, fallback, old controller alias, or second
  empty-state body remains.
- Forge recovery names pull only for a ready target in scope; otherwise it
  names the required filter, bearer, or external provisioning repair first.

Red: pure target-selection and awaited-read ordering proof, then real Compose
gesture/header behavior on the approved S22+ runtime.

Acceptance: routine verification, one separately approved S22+ platform pass,
and one hands-on pull confirming native threshold/resistance, stable viewport,
and no essential first-row text or control obscured by the indicator. Gateway,
tmux, and transport boundaries are
unchanged and require no integration or live gate.

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
| v0 fixed-topology dashboard cut | Implemented and published; create-only provisioning, routine, exact 27-test S22+ platform, and physical healthy/outage/recovery product gates green on the corrected tree |
| v0 review corrective layer | Implemented and published; routine, both-host integration, Linux live, exact 27-test S22+ platform, and physical product gates green |
| v0 profile delta — Devbox Claude Work | Integrated; routine and exact-tree external acceptance are owned by the merged candidate gates |
| v0 identity delta — automatic dwarf identity | Integrated hard cut; routine and isolated Linux/Darwin acceptance are owned by the merged candidate gates |
| v0 terminal key-deck delta | Integrated; routine and exact S22+ platform acceptance are owned by the merged candidate gates |
| v0 design delta D1 — terminal theme | Source implemented; routine verification and the 33-test instrumented S22+ suite green on the federated tree (devbox debug-signed run; the signed deviceDebug platform gate is MacBook-owned); hands-on OLED dim-text/256-color check `NOT_RUN` |
| v0 design delta D2 — chrome tokens | Source implemented and re-woven over the federation; adversarial review applied; routine verification and the 33-test instrumented S22+ suite green; hands-on pass (incl. the Forge warm-in glance) `NOT_RUN` |
| v0 design delta D3 — dwarf seals | Source implemented; golden/distinctness gates and the 33-test instrumented S22+ suite green; hands-on 48dp gallery pass `NOT_RUN` |
| v0 design delta D4 — ornament | Source implemented (interlace removed with the pairing screen); drift gate and the 33-test instrumented S22+ suite green; hands-on ornament/icon glance `NOT_RUN` |
| v0 design delta D6 — destructive and notice chrome | Implemented and verified; adversarial review applied (Role.Button, two contrast floors, an EmptyState severity contradiction); routine verification green (45 JVM tests) and the 35-test instrumented suite green on the physical S22+ (devbox debug-signed run); the cleft proof is mutation-checked; the hands-on cleft/stale glance stays `NOT_RUN` |
| v0 product-language delta — dwarves | Source implemented; routine verification green; rendered-device confirmation `NOT_RUN` |
| v0 design delta D7 — the launcher mark | Implemented; all three generator reds observed, then green; routine verification (static, build, unit) green, the ornament drift gate green over five generated files; the approved S22+ platform gate green at `OK (34 tests)`, the instrumented monochrome proof among them, run on the D7 branch before it merged D6 and D8 and not re-run on the merged tree; the hands-on launcher glance `NOT_RUN` |
| v0 design delta D8 — the Hlíðskjálf mark | Source implemented; the legibility proofs observed red on the shipped geometry then green, drift gate and routine verification (27 gates) green; instrumented suite green on the physical S22+ (39 tests; sole failure is the MacBook-owned provisioning fixture, plus two provisioned-machine skips); hands-on 18dp glance `NOT_RUN` |
| v0 design delta D5 — the Forge seal | Implemented over D6/D8; routine verification and the instrumented S22+ suite green; the journey's placement assertions ride the MacBook-owned product gate and stay `NOT_RUN` from the Linux devbox, as does the hands-on mark/lit-cold glance |
| v0 dashboard pull-to-refresh delta | Integrated over D5/D6/D8; red observed and merged-tree routine verification green; feature-tree signed 36-test S22+ platform gate green on 2026-08-27; merged 45-test platform and hands-on native threshold/resistance/viewport checks `NOT_RUN` |
| v0.5 push | Not scheduled |
