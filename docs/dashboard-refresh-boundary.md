# v0 dashboard refresh-boundary correction

2026-10-02: [session views](session-views.md) removes phone machine selection and
moves pressure to the machines disclosure. source implements that cutover;
qualification belongs to session views. the collection geometry and refresh
presentation below remain current. original delivery evidence follows and
retains its recorded source boundary.

Status: source implemented 2026-08-28. The focused signed same-version S22+
owner red was observed against the retired source, and the final two-test green
passed at system animator scale `0.0`; the exact original release and encrypted
pairing were restored. Routine verification and the complete 54-test
release-bound platform gate are green. Hands-on acceptance remains `NOT_RUN`.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`roadmap.md`](roadmap.md) owns delivery order;
[`dashboard-pull-to-refresh.md`](dashboard-pull-to-refresh.md) owns the existing
inventory intent. This document owns only collection spacing and refresh
presentation. Testing follows [`rules/testing.md`](rules/testing.md); no
duplicate `testing-standards.md` exists or is added.

## Outcome

Hard-cut the permanent pull-threshold gutter. The dwarf collection rests at the
normal dashboard rhythm while native pull mechanics and stationary content
remain unchanged.

goals: restore normal collection spacing, make transient feedback spend only
transient space, and preserve framework-owned interaction and truthful progress.

## Capability contract

- The grid's resting top inset is exactly `12dp`. The Material `80dp` pull
  threshold never contributes to layout.
- Pulling shows one determinate Gold boundary line from clamped
  `distanceFraction`; accepted verification shows one indeterminate line in the
  same band.
- the line is absent, including pixels and semantics, at rest and when the
  accepted fleet has no live poller.
- Cards, empty content, fixed chrome, pressure rails, notices, focus, stable
  keys, and scroll position never move.
- `verifyVisibleInventory()` targets every live fleet inventory poller in every
  view. awaited-read completion, pressure independence, and material
  threshold/resistance/nested-scroll/fling behavior retain their contracts.

## Final composition

```text
fixed dashboard chrome
`- live target? PullToRefreshBox : inert collection
   |- active-only collection-edge progress
   `- one LazyVerticalGrid(top = 12dp)
```

The progress band is `2dp` high, inset `12dp` from both horizontal edges, at
collection `y = 0..2dp`. It is full-opacity `Gold`, with a transparent track,
straight butt ends, and no container, shape, elevation, glow, stop mark, visible
label, or reserved height. The first grid item begins at `y = 12dp`, leaving
`10dp` clear.
Start/end padding remains `12dp`. Since 2026-10-02 the `84dp` bottom
clearance is trailing content padding, not a viewport exclusion
([forge seal](forge-seal.md#placement-and-semantics)), and card gaps are `8dp`
([session card](session-card.md#grid)).
pressure rails live in the machines disclosure under
[session views](session-views.md#phone-presentation-and-machine-scope-removal),
so no selected-machine rail-to-card gap remains. a dashboard notice is still a
real intervening sibling and owns its existing height.

While pulling, exactly one semantics node exposes determinate progress and no
custom description or action. While refreshing, exactly one node exposes
indeterminate progress, content description `Checking tmux sessions`, and no
action or role. At zero animator duration, checking renders the complete static
band at `x = 12dp..width - 12dp`, `y = 0..2dp`, with the same indeterminate
semantics; it never freezes on an empty frame.

## Architecture and API design

`DashboardScreen.kt` remains the sole presentation owner. Keep
`PullToRefreshBox`, `PullToRefreshState`, and the private indicator composable;
reuse Material's linear progress primitive and the existing `Gold` token.
`DashboardDwarfGrid` owns its fixed `12dp` inset locally.

The API-36 client observes the public Android animator-duration-scale boundary
while the collection indicator is composed. It reads the initial value,
registers one lifecycle-bound change listener, then re-reads to close the
registration race. Scale zero selects a complete determinate visual whose
outer semantics remain indeterminate; no nullable coroutine-context fallback,
settings permission, or test seam exists.

No public API, callback, DTO, model, schema, state, dependency, controller,
polling lane, gateway, tmux, persistence, pressure, card, Forge, or terminal
change exists. Add no generic progress surface, spacing system, motion token,
or test-only production seam for this single consumer.

## historical original hard cut and cleanup

the following cleanup, ownership and proof recipe record the 2026-08-28 change.
they are not a new assignment or retained suite; [testing policy](rules/testing.md)
governs current verification. the selected-machine pressure proof below is
historical; session views supersedes that placement.

Delete in the same change:

- `PullToRefreshDefaults.IndicatorBox` and both circular refresh branches;
- threshold-derived layout padding and the `topPadding: Dp` parameter;
- orphaned imports and comments tied to the retired placement.

Keep `CircularProgressIndicator` because Forge still consumes it. Retain no
circular renderer, feature flag, compatibility path, fallback, responsive
variant, negative margin, or custom gesture engine.

## historical original files and ownership

| Owner | Paths | Proof |
| --- | --- | --- |
| Root integrator | `docs/architecture.md`, `docs/roadmap.md`, `docs/design-language.md`, `docs/dashboard-pull-to-refresh.md`, this document | Authority alignment; no runtime claim |
| Compose builder | `android/app/src/main/java/dev/niels/skidbladnir/DashboardScreen.kt`, `android/app/src/androidTest/java/dev/niels/skidbladnir/MultiMachineUiInstrumentedTest.kt` | Existing collection proof plus existing parent pressure-placement proof |
| Read-only verifier | none | Diff, residue, and gate review only |

The runtime slice cannot be split further without violating builder-owned red
or duplicating the only changed behavior boundary. No owner touches card,
pressure, controller/model, gateway, build, catalog, or `scripts/test` paths.

## historical original red / green / refactor

**Red:** extend
`dwarfCollectionPullKeepsContentAndExposesOnlyActiveCheckingProgress`. On the
existing short live fixture, prove the first card starts `12dp +/- 1dp` below
the collection viewport. Hold one real partial drag before release and prove
one determinate progress node. After release, prove one indeterminate node. In
both phases assert, within `1dp`, `left = grid.left + 12dp`,
`right = grid.right - 12dp`, `top = grid.top`, and `height = 2dp`. The current
`92dp` inset and `24dp` circle must fail before production edits. Include
expected/actual grid, card, and indicator bounds in failures.

**Green:** make that same proof pass while retaining its existing threshold,
single-dispatch, active-only semantics, stationary-content, no-overlap,
empty/short/stale/reading/inert, and away-from-top behavior. Extend
`machinePressureRailIsCompactAccessibleAndDisclosesOnlyItsMachine` with one
selected ready/fresh/one-card/no-notice fixture proving the `16dp +/- 1dp`
visible rail-to-card interval at its existing `360dp / 1.0x` and
`320dp / 2.0x` sizes. Do not duplicate rail or card internals.

**Refactor:** remove the retired indicator path and padding propagation. Let
compile, static checks, and diff review prove residue removal; add no source-text
or framework-internal test. Add no screenshot golden or exact Material-threshold
constant assertion. Because a valid first indeterminate frame may contain no
Gold segment, wait for the first non-empty Gold frame under a short condition
bound; never sleep or pin an animation timestamp.

## Acceptance and 80/20 verification

- every view with a live fleet poller: `12dp` rest inset and stationary first
  card. pressure belongs to the machines disclosure; notices may intervene.
- Live empty collection: the same grid inset and existing pull/retention
  behavior, with no second layout path.
- Scrolled collection: existing top/threshold admission remains unchanged.
- no live fleet poller: inert, `12dp`, no progress pixels or semantics.
- Active line has the exact contracted bounds and never intersects chrome,
  empty text, cards, or Kill controls at the focused `360dp / 1.0x` fixture;
  the parent seam additionally holds at `320dp / 2.0x`.
- Completion is quiet: no toast, haptic, success mark, or card animation.

current engineering checks and session-view qualification belong to
[session views](session-views.md#acceptance). a hands-on pull/reduced-motion
glance requires the real device and current-turn approval; absent that boundary
it remains `NOT_RUN`. the original runtime evidence above does not qualify the
new view or pressure placement.

## Non-goals and tradeoff

this spacing/indicator contract adds no pressure polling, inventory sequencing
or cadence change; tap/accessibility refresh action; whole-dashboard scrolling;
forge, card or large-screen redesign; density setting; screenshot golden;
analytics; or inferred agent state. session views owns the separate navigation
and pressure-placement change.

Tradeoff: the boundary line is less familiar and less conspicuous than the
Material circle. The accepted gain is an unambiguous dense resting hierarchy,
with native gesture physics, stationary work, active progress semantics, and no
permanent cost for transient feedback. Material's accessibility bounds remain
larger than the visual `2dp` line; pixel geometry and semantic geometry are
therefore intentionally proved separately. One process-global Android duration-
scale listener lives only while the live collection indicator is composed; its
lifecycle cost buys correct first-frame and runtime reduced-motion behavior.
