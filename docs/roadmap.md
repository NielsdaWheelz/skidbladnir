# Skíðblaðnir v0 roadmap

Status: scope reset approved 2026-08-25; S1–S3 and the corrective delta are
implemented. The 2026-08-26 multi-machine hard cut is also implemented and
accepted: routine verification, isolated Linux and Darwin tmux/API, live
Devbox and MacBook publication, create-only fixed-collection provisioning,
27-test S22+ platform instrumentation, and the physical two-host
outage/recovery journey are green. The earlier corrective
delta's named Codex hook-digest and hands-on terminal/Gboard checks remain
`NOT_RUN`; federation acceptance does not substitute for them.
The 2026-08-27 public-fleet hard cut is implemented and public. Exact-SHA
hosted CI, immutable `v0.2.24` publication and pinning, three-host convergence,
fleet doctor, and the complete 54-test release-bound S22+ platform gate are
green. Product and the named second-phone gate remain `NOT_RUN`.
The 2026-08-28 agent-identity projection hard-cut source is implemented in
Skíðblaðnir and `dev-server`, both routine verification suites are green, and
the approved exact-head isolated Darwin tmux integration is green. Exact
`v0.2.24` release/pin/deployment and the current Android platform gate are
green; Linux isolated, current Android hands-on, and the provider-live matrix
remain `NOT_RUN`.
The 2026-08-28 dashboard refresh-boundary correction source is implemented;
its owner red and focused signed same-version S22+ two-test green are recorded,
including the zero-duration branch. Routine verification and the complete
54-test release-bound platform gate are green. Hands-on acceptance remains
`NOT_RUN`.
The 2026-08-28 tmux-session rename delta is implemented, released, and deployed
under its accepted contract. Its owner reds and exact-tree gates are recorded;
unavailable boundaries remain `NOT_RUN`.
The 2026-08-28 agent-interaction-state candidate and its routine, exact-tree
Darwin, and signed same-version S22+ evidence were reviewed and rejected by the
accepted 2026-08-31 tmux terminal-activity scope correction. That evidence does
not prove the replacement. The replacement is merged in both repositories;
owner behavioral reds, focused greens, both routine suites, release-tree
Darwin isolated tmux, focused S22+ component, exact `v0.2.24`
release/pin/deployment, three-host doctor, and the complete 54-test
release-bound S22+ platform gate are green. Linux isolated tmux, S22+
hands-on, provider-live, product, and second-phone acceptance remain `NOT_RUN`.
The 2026-08-31 dashboard-return-continuity delta source is implemented. Its
boundary-owner and review-corrective reds are recorded; routine verification
is green. A production-signed same-version 56-test S22+ candidate suite was
green at `69.542` seconds before the terminal-activity rebase. The rebased
54-test release-bound platform gate is now green with encrypted pairing
preserved and the exact public release restored. Hands-on acceptance remains
`NOT_RUN`.
Supersedes the P0–P7 roadmap (git history through `6f2d697`); the
`codex/p1-managed-agent` branch and its worktree implement the superseded
architecture and are abandoned, not merged.

This document owns delivery order. The [architecture](architecture.md) owns
behavior and acceptance. Each slice has one observable outcome, a red proof
observed before implementation, and real-boundary tests.
Completed slice text records its historical delivery point; a later hard cut
supersedes conflicting behavior without rewriting that evidence as if it had
always existed.
Retired agent lifecycle/status/attention terms and enum names inside
completed-slice or red/green delivery records are historical only. The
terminal-activity section and architecture own current behavior.
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
 -> v0 public-fleet distribution and Connect hard cut
 -> v0 design delta D9: detach chrome
 -> v0 dashboard card hierarchy delta
 -> v0 machine-pressure rail delta
 -> v0 agent-identity projection hard cut
 -> v0 dashboard refresh-boundary correction
 -> v0 tmux-session rename delta
 -> v0 tmux terminal-activity hard cut
 -> v0 dashboard return-continuity delta
```

## S1 — tmux control plane

Initial Devbox foundation, later generalized by the multi-machine hard cut
without retaining a one-host branch.

Outcome: an authenticated Tailnet client lists, creates, and kills one host's
tmux sessions and reads its pressure.

- Go gateway: loopback bind, Tailscale Serve mapping, host-minted bearer
  (constant-time check; re-mint revokes).
- `GET /v1/sessions`: read-only poll over tmux plus one optional foreground
  process observation; each card has exact identity, client count, required
  current-window `Active | Quiet`, and optional process-lifetime agent
  metadata.
- `POST /v1/sessions`: cwd/tmux-name/objective validation, host profile-table
  allowlist, unbounded `skidbladnir-<profile>-<N>` generated names, independent
  balanced Dvergatal assignment, user options set at create, YOLO exec.
- `DELETE /v1/sessions/{tmuxId}`: inventory `identityToken` binds a random
  tmux-server epoch + built-in PID/start time + id; all lifetime facts, the
  displayed name, ungrouped-or-last-link predicate, and `kill-session` share
  one tmux client queue; owned stale phone shadows reconcile first, while any
  remaining ordinary group fails closed.
- `GET /v1/pressure` with the proven thresholds.
- `skid-notify` is a BEL-only terminal convenience with no product-state
  authority.
- Exact provider `SessionStart` identity installation; native digest review
  remains manual and fail-visible, and the helper accepts only an exact pane
  foreground provider origin.

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
  frontier instead of failing at root-owned `launchd`; the retained optional
  identity adapter therefore observes the same exact foreground origin as
  Linux.
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

- Fixed aligned rows: `Esc / - Home ↑ End PgUp` and
  `Tab Ctrl Alt ← ↓ → PgDn`; top detach and Android Back remain lifecycle
  actions.
- Page-owned one-shot Ctrl/Alt state is atomic, visible, spoken, independently
  toggleable, jointly consumable, and reset on input or any lifecycle boundary.
- Proven keys use exact xterm-compatible Ctrl/Alt encoding. IME composition,
  dictation-shaped, Unicode, multi-character, and paste input remain literal
  while consuming modifiers.
- Equal `48dp` cells use `2dp` gaps in one `356dp x 106dp` normal-font grid;
  both rows share overflow below that width or when large text needs it.
- IME containment, resize, viewport fitting, queue bounds, WSS, and tmux
  contracts remain unchanged.

Acceptance: focused real-Compose and locked-WebView proofs, routine
verification, the exact signed S22+ platform gate, and separately reported
hands-on Gboard/accessibility/viewport checks.

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
- Recovery copy names the `session`; `agent` is reserved for the opaque
  foreground terminal program. The later D9 hard-cuts the top action to the
  literal `Detach`.
- HTTP `/sessions`, tmux identity, internal target types, and test selectors do
  not change.

Historical red: a `SHELL` card is still called an agent even though no agent
process is present, and Detach promises that a transient agent rather than the
tmux session keeps running.

Historical acceptance: focused Android unit proof for detach lifetime copy,
compiled instrumentation assertions for dashboard language, routine
verification, and the next separately approved platform pass for
rendered-device confirmation.

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
  names the required filter, whole-fleet reconnect, or fleet reset outside the
  app first.

Red: pure target-selection and awaited-read ordering proof, then real Compose
gesture/header behavior on the approved S22+ runtime.

Acceptance: routine verification, one separately approved S22+ platform pass,
and one hands-on pull confirming native threshold/resistance, stable viewport,
and no essential first-row text or control obscured by the indicator. Gateway,
tmux, and transport boundaries are
unchanged and require no integration or live gate.

## v0 public-fleet distribution and Connect hard cut — active

Outcome: a trusted API-36 user installs one signed public GitHub APK, signs
into Tailscale once, taps `Connect`, scans one five-minute QR, and receives the
exact Devbox/MacBook/Arch fleet atomically. Public `dev-server` pins the same
release, converges each machine-local gateway as an auto-started service, and
generates a fresh QR per phone without copying durable bearers off-host.

- One release publishes the APK, Linux-amd64 and Darwin-arm64 host bundles,
  checksums, and Android signer fingerprint from one tag/SHA.
- Deployment-owned strict host configs replace platform-derived paths and
  expose the five existing `dev-server` Codex/Claude shortcuts on every host.
- Each gateway owns one in-memory, five-minute, one-use pairing invitation;
  restart, replacement, or bearer rotation invalidates it.
- Android accepts only the ordered exact-three QR, awaits all three redeems,
  and performs one encrypted create-only install. `Reconnect fleet` rotates
  bearers only for the exact installed identities.
- Delete ADB provisioning, manual bearer entry, two-machine assumptions,
  repo-owned host installers/services, headerless inventory pairing, and every
  fallback or legacy reader.

Red: host config cannot represent Arch/all profiles; two concurrent redeems
both win; malformed or partial fleets reach storage; partial storage becomes
readable; a release can stage with the wrong signer/version/assets; or
convergence changes identity/tmux lifetime or needs a manually started daemon.

Gate: routine pure/service/component proofs; separately approved release,
isolated tmux, live convergence on each host, real-scanner S22+ product, and
named second-phone gates. Every unavailable or unapproved external boundary is
`NOT_RUN`, never substituted by a lower proof. The detailed contract and
non-overlapping ownership are [public-fleet-distribution.md](public-fleet-distribution.md).

## v0 design delta D9 — detach chrome

Outcome: the terminal header hard-cuts the stock long detach `TextButton` to a
purpose-built, symmetric Niðavellir control whose entire visible and spoken
content is `Detach`. Kill remains Ember, `Cleft`, literal, and confirmed;
Android Back and all phone-only detach behavior remain unchanged.
This supersedes the product-language delta's visible detach lifetime copy and
its pure string proof; recovery language remains unchanged.

Scope, component contract, cleanup, work split, and red proof per
[detach-chrome.md](detach-chrome.md). Gate: routine verification plus one
separately approved S22+ platform pass and hands-on header glance. No
integration or live gate for the unchanged lifecycle boundary.

## v0 dashboard card hierarchy delta

Outcome: the dense session card becomes work-first without changing its
Niðavellir shell or any runtime contract. The tmux name leads; the dwarf name
remains a quieter Big Shoulders signature; a fixed colour-only status facet is
redundant to the named status bay; machine becomes quiet footer context in
`All` and disappears visually under an explicit machine filter. Objective,
availability, cwd, profile, and Kill occupy one stable compact grammar.

Red: pure directory abbreviation behavior, then a focused Compose card proof
for ordering, status/attention independence, conditional machine visibility,
spoken full context, target size, and the common-card height bound. The
existing multi-machine journey keeps only its product-boundary assertions.

Acceptance: routine verification plus one separately approved S22+ component
pass and hands-on typical/long-label/attention/unavailable/filtered/large-font
glance. No gateway, tmux, controller, sorting, polling, terminal, or public API
boundary changes. Scope per
[`dashboard-card-refactor.md`](dashboard-card-refactor.md).

## v0 machine-pressure rail delta

Outcome: each machine's pressure evidence occupies one compact disclosure rail
instead of a tall fixed report, without changing sampling, thresholds, history
rendering, or action admission.

- Consume the strict host-owned `GET /v1/pressure` per-signal states, aggregate
  recovery phase, and compact level-only history unchanged. CPU/swap remain
  informational; Android owns no thresholds, raw-value colour inference, or
  fallback decoder.
- Render one split-colour header, one stable non-wrapping flat typographic
  metric row, and the unchanged 16dp history band. Neutral labels and quiet
  informational/normal values yield only to Gold/Ember host-evaluated
  warm/hot exceptions. Remove the gem model and its fills, borders, shapes,
  selectors, and names.
- One rail tap opens a machine-bound local details sheet with all supported
  current signals, canonical reasons/freshness, and explicit `NO DATA`.
  Disclosure performs no request or mutation.
- The 2026-08-28 density follow-up omits pressure rails from `All` and renders
  exactly one rail under an explicit machine filter. Compact exceptional
  machine notices remain visible in `All`; polling, snapshots, disclosure, and
  admission do not change.

Red: deterministic Android presentation grammar/precision/accent roles, then
real Compose rail behavior at compact and large-text sizes. The density
follow-up adds one pure filter-visibility red and extends that existing Compose
journey with `All` absence and exact selected-machine presence. Existing host,
gateway, decoder, polling, history, and disclosure proofs remain their owners
and are not duplicated for these presentation-only cuts.

Acceptance: routine verification, one separately approved signed S22+ focused
component pass for `All`/selected placement,
geometry/semantics/history pixels/sheet interaction, and one approved hands-on
normal/mixed/missing/Darwin/large-text/greyscale glance.
Integration, live, and second-phone gates do not re-prove unchanged boundaries.
The product journey consumes the new placement but does not substitute for the
focused component proof. Scope per
[machine-pressure-rail.md](machine-pressure-rail.md).

## v0 runtime-release policy hard cut — active

Outcome: Codex, Claude, and tmux follow the latest stable release available
through each platform's managed channel. Skíðblaðnir release artifacts remain
immutable and digest-pinned; tool versions do not.

- Red: a changed tmux version blocks gateway or lifecycle startup; macOS pins
  the Homebrew formula; Arch excludes tmux from upgrades; Devbox rejects the
  repository's current stable package; Codex convergence installs one exact npm
  version and doctor rejects any other binary or payload digest.
- Green: the strict host config hard-cuts `tmux.version` to advisory
  `tmux.testedVersion`; runtime boundaries require a canonical executable but
  do not compare versions; doctor emits a nonblocking warning on drift; native
  package channels converge tmux and Claude to stable; npm selects
  `@openai/codex@latest`; all Codex version and payload-digest machinery is
  removed.
- Acceptance: routine verification in both repositories. Integration, live
  host, release publication, and device gates remain explicit `NOT_RUN` until
  separately approved.

## v0 agent-identity projection hard cut — active

Outcome: every visible tmux session exposes its exact tmux address and an
optional exact current Codex/Claude runtime projection without making the agent
less opaque or adding communication.

- Hard-cut session `id/profile` to `tmuxId/launchProfile`; profile choices gain
  required provider; Android hard-cuts agent-named session types and selectors
  to session terminology.
- One `internal/agentruntime` owner validates provider/profile configuration,
  classifies the already-observed foreground process, accepts one bounded
  process-lifetime registration, and applies Claude name/launch argv rules.
- Replace `status-hook` with the closed Codex/Claude `agent-hook`. SessionStart
  registers provider session id and runtime profile. At this slice's delivery,
  Codex also retained the then-current coarse lifecycle semantics; the later
  terminal-activity hard cut removes them without replacing them with provider
  state.
- Deployment hard-cuts the Codex hook files and loads one owned Claude
  SessionStart plugin through the existing router; it does not overwrite
  Claude user settings, and raw-binary launches remain unregistered.
- Inventory observes foreground once for optional agent identity. Missing or stale
  hooks omit registered facts; `@skid_profile` remains only the launch fact and
  never fills an unknown runtime profile.
- No provider API, transcript/config-store parsing, second registry, background
  worker, message endpoint, tool, scheduler, wake, or non-current-pane scan.

Red: pure runtime/config/registration/argv behavior, then one owner proof at
the hook, session, authenticated HTTP, and Android card boundaries. A rename
compile failure is not the red.

Acceptance: routine verification plus the separately approved isolated tmux,
focused S22+ component, deployment, and four-case live sample defined in
[agent-identity-projection.md](agent-identity-projection.md). Unapproved
external boundaries are `NOT_RUN`.

## v0 dashboard refresh-boundary correction — implemented

Outcome: the dwarf collection removes its permanent pull-threshold gutter and
returns to a `12dp` resting inset without changing pull mechanics or inventory
behavior. One active-only `2dp` Gold boundary line is determinate while pulling
and indeterminate while checking; cards remain stationary and unobscured.

Red: extend the existing real-Compose collection proof so the current `92dp`
inset and `24dp` circular indicator fail the approved resting and active
geometry. No pure, controller, pressure-internal, card-internal, gateway, or
tmux proof is added.

Acceptance: the focused red then green, routine verification, and—only when
separately approved—one S22+ platform pass and hands-on native-pull,
reduced-motion, spacing, and non-overlap glance. Every unapproved device gate is
`NOT_RUN`; integration, live, and tmux do not re-prove this presentation-only
boundary. Scope per
[`dashboard-refresh-boundary.md`](dashboard-refresh-boundary.md).

Implementation status: the legacy `92dp`/circular presentation failed the
focused signed same-version S22+ owner proof, and the final two-test candidate
passed on the same device at system animator scale `0.0`. The recovery boundary
restored the exact original release and preserved encrypted pairing. Routine
verification and the complete 54-test release-bound platform gate are green;
the hands-on gate remains `NOT_RUN`.

## v0 tmux-session rename delta — implemented, released, and deployed

Outcome: from an active Terminal, rename one exact ordinary tmux session in
place without changing its lifetime, attachment, work, character, or provider
facts.

- Add strict `PATCH /v1/sessions/{tmuxId}` with expected name, desired name,
  and lifetime token; success is bodyless `204` followed by mandatory inventory
  confirmation.
- One tmux queue compare-and-swaps server lifetime/id/current name before
  `rename-session`; native uniqueness owns collision. No retry, alias, provider
  synchronization, persistence, event observer, or compatibility route exists.
- Add one literal, accessible Terminal identity control and focused editor;
  keep the current ASCII name grammar and active phone attachment.
- Hard-cut the public identity-mismatch copy to action-neutral wording and
  extract only the now-shared name validator, identity predicate, session-path
  parser, and bodyless client response owner.

Red: one current-server authenticated PATCH journey and one real Compose
Terminal proof. Green adds one focused proof at tmux/session, HTTP, Android
state/client, and Compose boundaries. Acceptance is routine verification, the
same isolated tmux/API proof on separately approved Linux and Darwin sockets,
and one separately approved S22+ focused component/hands-on pass. Every other
external gate remains `NOT_RUN`. Scope and ownership are closed in
[session-renaming.md](session-renaming.md).

## v0 tmux terminal-activity hard cut — implemented, released, and deployed

Outcome: every fresh card exposes one provider-neutral answer to whether its
current tmux window had recent activity, with no agent-state or unread-result
claim.

- Add one required flat `activity: Active | Quiet` session field. Derive it on
  the host only from the existing card anchor's built-in `window_activity`, the
  inclusive ten-second window, and the existing five-second inventory cadence.
- Treat current-window creation/selection and output from any sibling pane as
  tmux activity. Ignore other windows, `session_activity`, alert flags, process
  facts, hooks, provider state, terminal content, CPU, and Android time.
- Retain optional exact `agent` metadata without a runtime-state wrapper or any
  influence on activity. Retain only the content-free `SessionStart` identity
  hook; make the Codex notifier BEL-only.
- Hard-delete every superseded state union and writer, result acknowledgement,
  semantic-adapter/lease plan, legacy decoder/presenter/priority/color path,
  and the rejected second semantic release epoch.
- Android renders `ACTIVE` and `QUIET` with literal/spoken meaning, Quiet-first
  fresh ordering, a restrained reduced-motion-safe Active facet, and existing
  stale-machine qualification. It owns no timer or threshold.
- Gateway and Android release atomically under the existing exact release
  tag/SHA/digest boundary. No alias, fallback, schema version, dual reader, or
  compatibility window exists.

Red: one behavior proof at strict timestamp derivation, authenticated HTTP DTO,
Android decode/order, real Compose presentation, and installed hook/notifier
topology. Green implements only those reds; refactor reuses the card-anchor,
projection-clock, poller, strict-ingress, sorting, and presentation owners before
requiring zero active legacy residue.

Acceptance: routine verification plus separately approved generic-program
isolated tmux and focused S22+ component/hands-on gates defined in
[terminal-activity.md](terminal-activity.md). Provider-live remains an optional
agent-identity proof and does not prove activity. Every unapproved external
boundary remains `NOT_RUN`.

Push, semantic next-move state, and unread-result attention are unscheduled.
Any one of them requires a new architecture decision; this slice adds no
scaffold.

## v0 dashboard return-continuity delta — implemented

Outcome: Dashboard is one retained route entry. Terminal drill-in followed by
top `Detach` or Android Back restores its typed filter before current-scope
verification, settles its semantic grid before card interaction, and reveals
the selected chip before Dashboard is considered settled. Saved-task recreation
returns to Dashboard without restoring the attachment.

Hard cut: replace nullable Dashboard-only selection with `DashboardScope`, move
grid ownership above the destination switch, centralize the existing
machine/id/lifetime card key, and delete reconstruction/local-grid paths. No
Terminal return payload, inventory snapshot/payload persistence, compatibility
path, navigation framework, backend, tmux, or dependency change.

Red: one device-free state/target proof plus separately approved real-Compose
return and real-`SavedStateRegistry` adapter proofs, all owned and observed by
the one Android builder. Acceptance is routine verification, the approved
platform proof, and one hands-on real-`MainActivity` MacBook
bottom-to-Terminal Detach/Back/recreation sample. Unapproved device work is
`NOT_RUN`; no integration, live, host, provider, or tmux gate re-proves this
Android-only boundary. Scope and split are closed in
[`dashboard-return-continuity.md`](dashboard-return-continuity.md).

Evidence: the three boundary-owner reds and four review-corrective reds failed
behaviorally against their intended contracts, and routine verification is
green. Before the terminal-activity rebase, the focused signed S22+ Detach/Back
journey passed at `OK (1 test)` in `7.925` seconds and the production-signed
same-version candidate ran the complete checked-in instrumentation suite at
`OK (56 tests)` in `69.542` seconds inside the 90-second boundary. The encrypted
pairing digest was unchanged, the test package was removed, and the exact public
v0.2.21 APK was restored. That evidence does not prove the rebased tree. The
rebased complete 54-test release-bound platform gate is now green with pairing
preserved and the exact public release restored; the hands-on real-product
sample remains `NOT_RUN`.

## Status

| Slice | Status |
| --- | --- |
| S1 tmux control plane | Implemented; the terminal-activity hard cut now owns current session activity and its gate status is recorded below |
| S2 shared terminal | Implemented; corrective RGB command shape and isolated integration/live proof green; renewed concurrent physical handoff `NOT_RUN` |
| S3 Android dashboard | Implemented; 9-test S22+ platform gate green, including viewport/geometry/rendered color; renewed hands-on Gboard/dictation proof `NOT_RUN` |
| v0 corrective delta | Source implemented; routine verification and automated external gates green; named hands-on checks above remain `NOT_RUN` |
| v0 multi-machine hard cut | Implemented, published to Devbox and MacBook, paired on S22+, and green across routine, both host, platform, and physical product gates |
| v0 fixed-topology dashboard cut | Implemented and published; create-only provisioning, routine, exact 27-test S22+ platform, and physical healthy/outage/recovery product gates green on the corrected tree |
| v0 review corrective layer | Implemented and published; routine, both-host integration, Linux live, exact 27-test S22+ platform, and physical product gates green |
| v0 profile delta — Devbox Claude Work | Integrated; routine and exact-tree external acceptance are owned by the merged candidate gates |
| v0 identity delta — automatic dwarf identity | Integrated hard cut; routine and isolated Linux/Darwin acceptance are owned by the merged candidate gates |
| v0 terminal key-deck delta | Two-row Ctrl/Alt hard cut implemented; both owner reds recorded, routine verification and the current complete 54-test release-bound S22+ platform green with pairing preserved and the exact public release restored; hands-on Gboard, TalkBack, Switch Access, reach, haptic, and `80 x 5` viewport checks `NOT_RUN` |
| v0 design delta D1 — terminal theme | Source implemented; routine verification and the 33-test instrumented S22+ suite green on the federated tree (devbox debug-signed run; the signed deviceDebug platform gate is MacBook-owned); hands-on OLED dim-text/256-color check `NOT_RUN` |
| v0 design delta D2 — chrome tokens | Source implemented and re-woven over the federation; adversarial review applied; routine verification and the 33-test instrumented S22+ suite green; hands-on pass (incl. the Forge warm-in glance) `NOT_RUN` |
| v0 design delta D3 — dwarf seals | Source implemented; golden/distinctness gates and the 33-test instrumented S22+ suite green; hands-on 48dp gallery pass `NOT_RUN` |
| v0 design delta D4 — ornament | Source implemented (interlace removed with the pairing screen); drift gate and the 33-test instrumented S22+ suite green; hands-on ornament/icon glance `NOT_RUN` |
| v0 design delta D6 — destructive and notice chrome | Implemented and verified; adversarial review applied (Role.Button, two contrast floors, an EmptyState severity contradiction); routine verification green (45 JVM tests) and the 35-test instrumented suite green on the physical S22+ (devbox debug-signed run); the cleft proof is mutation-checked; the hands-on cleft/stale glance stays `NOT_RUN` |
| v0 product-language delta — dwarves | Source implemented; routine verification green; rendered-device confirmation `NOT_RUN` |
| v0 design delta D7 — the launcher mark | Implemented; all three generator reds observed, then green; routine verification (static, build, unit) green, the ornament drift gate green over five generated files; its branch-local S22+ platform gate was green at `OK (34 tests)`, with the instrumented monochrome proof among them, and the current complete 54-test release-bound platform gate is green; the hands-on launcher glance `NOT_RUN` |
| v0 design delta D8 — the Hlíðskjálf mark | Source implemented; the legibility proofs observed red on the shipped geometry then green, drift gate and routine verification (27 gates) green; instrumented suite green on the physical S22+ (39 tests; sole failure is the MacBook-owned provisioning fixture, plus two provisioned-machine skips); hands-on 18dp glance `NOT_RUN` |
| v0 design delta D5 — the Forge seal | Implemented over D6/D8; routine verification and the instrumented S22+ suite green; the journey's placement assertions ride the MacBook-owned product gate and stay `NOT_RUN` from the Linux devbox, as does the hands-on mark/lit-cold glance |
| v0 dashboard pull-to-refresh delta | Integrated over D5/D6/D8; red observed and merged-tree routine verification green; feature-tree signed 36-test S22+ platform gate green on 2026-08-27 and current complete 54-test release-bound platform green; hands-on native threshold/resistance/viewport checks `NOT_RUN` |
| v0 public-fleet distribution and Connect hard cut | Implemented and public; exact-SHA hosted CI, immutable `v0.2.24` release/pin, three-host convergence, fleet doctor, and complete 54-test release-bound S22+ platform green; product and second-phone gates `NOT_RUN` |
| v0 design delta D9 — detach chrome | Implemented; focused S22+ red observed; routine verification, the exact 47-test S22+ platform gate, and the hands-on header glance green |
| v0 dashboard card hierarchy delta | Implemented and verified; routine verification, the 47-test physical S22+ platform gate, and hands-on synthetic-fixture visual/accessibility acceptance green on 2026-08-27 |
| v0 machine-pressure rail delta | Flat typographic-row hard cut implemented with its prior focused/device acceptance green; All-filter density follow-up implemented with its pure red/green, routine verification, signed same-version S22+ component placement, and complete 54-test release-bound platform green. Product and hands-on acceptance remain `NOT_RUN`. |
| v0 runtime-release policy hard cut | Implemented; red proofs and both routine suites green; exact `v0.2.24` release/pin, three-host convergence, and release-bound platform green; remaining integration and device gates reported separately |
| v0 agent-identity projection hard cut | Implemented; owner reds, focused greens, routine verification, approved exact-head Darwin isolated tmux, exact `v0.2.24` release/pin/deployment, and current Android platform green; Linux isolated, current Android hands-on, and provider-live acceptance `NOT_RUN` |
| v0 dashboard refresh-boundary correction | Source implemented; focused signed same-version S22+ owner red observed and final two-test green at system animator scale `0.0`; exact original release and encrypted pairing restored. Routine verification and the complete 54-test release-bound platform gate are green; hands-on acceptance `NOT_RUN` |
| v0 tmux-session rename delta | Implemented, released, and deployed under the accepted hard-cut contract; owner reds and exact-SHA gates recorded; every remaining boundary reported separately |
| v0 agent interaction-state candidate | Rejected and superseded by the 2026-08-31 terminal-activity correction; its source and evidence are historical and prove no active target |
| v0 tmux terminal-activity hard cut | Implemented and merged; owner reds, focused greens, both routine suites, release-tree Darwin isolated tmux, focused S22+ component, exact `v0.2.24` release/pin/deployment, three-host doctor, and complete 54-test release-bound S22+ platform green; Linux isolated, S22+ hands-on, provider-live, product, and second-phone acceptance `NOT_RUN` |
| v0 dashboard return-continuity delta | Source implemented; boundary-owner and review-corrective reds recorded; routine verification green; focused and 56-test production-signed S22+ candidates green before the terminal-activity rebase; rebased complete 54-test release-bound platform green with pairing preserved and exact release restored; hands-on acceptance `NOT_RUN` |
| Push or semantic agent state | Not scheduled; requires a new architecture decision |
