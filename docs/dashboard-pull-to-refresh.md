# v0 dashboard pull-to-refresh delta

2026-10-02 accepted design: [session views](session-views.md) removes phone
machine filtering, so manual verification targets every live fleet inventory
poller. source implements this cutover; qualification belongs to session views.
request/completion fencing, collection geometry and inventory-only refresh
remain governed here. earlier evidence retains its recorded source boundary.

the [groups](groups.md) presentation contract does not narrow refresh targets.
manual verification snapshots every live fleet inventory poller, including hosts
with no previously observed membership in the selected group. view selection
changes neither request/completion ordering nor pressure polling. its strip
stays outside the pull owner. the
evidence below predates groups. its source is implemented; [groups/shells hands-on acceptance](issues/groups-shells-hands-on.md)
remains open.

Status: source integrated over D5/D6/D8, 2026-08-27. Red was observed and
routine verification is green on the merged tree. The feature-tree signed
36-test S22+ platform gate was green before integration; the current complete
54-test release-bound platform gate is green. The hands-on native-feel check
remains `NOT_RUN`.
The 2026-08-28 refresh-boundary correction source is implemented. Its focused
signed same-version S22+ owner red and two-test green are recorded in
[`dashboard-refresh-boundary.md`](dashboard-refresh-boundary.md); routine
verification and the complete 54-test release-bound platform gate are green,
while hands-on acceptance remains `NOT_RUN`.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`roadmap.md`](roadmap.md) owns delivery order; this document owns this delta's
implementation boundary. The requested testing standard is
[`rules/testing.md`](rules/testing.md); no duplicate `testing-standards.md` is
introduced.

## Outcome and final state

The dashboard hard-cuts its visible `Refresh` button to standard pull-to-refresh
over the dwarf collection. Automatic reconciliation remains primary. Pull is
the sole manual shortcut and verifies inventory across every live fleet poller.

## Scope

Own only the dashboard control hard cut, the always-lazy collection viewport,
inventory-only manual intent, awaited-read completion correctness, gesture
copy, and their Android proofs. All external boundaries and unrelated screens
are consumed unchanged.

## Closed decisions

- No overflow item, icon, contextual Retry, custom accessibility action, or
  other tap equivalent. This is an explicit one-user v0 exception, not a
  general accessibility precedent.
- every view verifies the same live fleet inventory pollers. targets are
  captured when the pull is released.
- Completion means the required post-request `GET /v1/sessions` reads landed.
  Pressure remains independently automatic and is not part of pull completion.
- header, views, notices, forge recovery, card order, terminal behavior and
  pressure placement stay outside pull behavior. [session views](session-views.md)
  owns those collection changes; only the dwarf collection viewport pulls.
- No open product or architecture question remains for implementation.

## Goals and rules

- Reclaim header space while preserving user agency and literal freshness.
- Use Material 3 `PullToRefreshBox`; do not hand-roll pointer, threshold,
  overscroll, nested-scroll, or fling behavior.
- reuse the existing per-machine inventory lane, fleet routing, stale
  reduction, mutation fences, and `Dashboard.refreshing` derived state.
- Keep the last snapshot, viewport, focus, dialog/draft state, and stable card
  keys during verification. Never blank, reload, or jump the collection.
- Five-second foreground polls stay quiet. Foreground initial reads,
  post-mutation confirmation, and pull-requested reads may show the indicator.
- Pull never retries or replays a mutation or terminal byte and never changes
  admission rules.
- Recoverable stale/unreachable copy says `Pull down to check again`.
  Authentication keeps `Reconnect fleet`; identity change requires fleet
  reset outside the app. Neither claims pull can repair it.
- Forge outcome-unknown recovery names only an available path: pull when its
  ready target is available, reconnect the fleet for authentication, and reset the
  fleet outside the app for an identity-changed or missing target. Review-ready
  remains safe past-tense copy.

## Capability contract

1. A downward drag released below the platform threshold does nothing.
2. A downward drag released after the collection has reached its top and
   accumulated the platform threshold overscroll requests one verification of
   the captured live fleet targets. Drag-start position is not a second gesture rule.
3. The Compose pull owner dispatches no second gesture intent while the derived
   indicator is active. Programmatic callers still require their own
   post-intent read and coalesce through the same lane.
4. A pull racing an ordinary inventory poll requires exactly one later read;
   the leading pre-request result cannot finish the indicator.
5. Independent machines may finish or fail independently. The indicator ends
   after every targeted inventory ticket is satisfied or its polling stops.
6. Success replaces only that machine's snapshot. Failure retains the prior
   snapshot as `STALE`, or remains `Unreachable` when none exists; existing
   action fencing is unchanged.
7. Empty, short, reading, stale, and populated collections all accept the same
   pull gesture. horizontal view-strip scrolling cannot trigger it because
   the strip remains outside the pull owner.
8. Programmatic and pulled verification use one progress surface. It is
   visible and exposes indeterminate progress semantics, but no click action.
9. Success is quiet: no toast, snackbar, timestamp, haptic, or card-reorder
   animation is added.
10. if the accepted fleet has no live polling target, no indicator starts and
    the existing machine-access or fleet-reset notice remains the outcome.

## Structure and composition

```text
fixed DashboardTopBar (Dwarves + machine summary)
fixed view strip / machines disclosure / notices / recovery
`- live fleet target? PullToRefreshBox : inert collection container
   `- one always-present LazyVerticalGrid
      |- full-span empty/reading state, or
      `- stable-keyed dwarf cards

pull -> verifyVisibleInventory()
     -> all live fleet inventory handles
     -> existing per-machine coalescing inventory lane
     -> authenticated GET /v1/sessions
     -> existing Fresh | Stale | Unreachable reduction and admission
     -> awaited read tickets empty -> indicator hides
```

the same lazy grid renders empty and populated collections. fixed chrome does
not move. a `MachineAccess.Ready` fleet target is the foreground ui invariant for
a live poller; any such target mounts the platform pull owner, regardless of
the selected view's rows. the same lazy child remains with no live targets, so
a no-op release cannot strand material state at its threshold and no second
scroll body exists.

The 2026-08-28 correction hard-cuts only indicator placement and collection
spacing per [`dashboard-refresh-boundary.md`](dashboard-refresh-boundary.md).
Material continues to own gesture, threshold, resistance, nested-scroll, and
fling behavior; no custom gesture physics or second refresh surface exists.

## API and state design

### External boundaries

No HTTP, WSS, tmux, gateway, DTO, persistence, credential, or terminal schema
changes. No dependency is added; Material 3 is already in the Compose BOM.

### Android intent

the single manual inventory intent is:

```kotlin
fun verifyVisibleInventory()
```

it snapshots every live fleet polling runtime independently of the selected view and
requests awaited inventory reads even when an older requirement is active;
the lane coalesces them. It does not call `requestPressure`.
dashboard pull and `detachToSessions` use this same intent; the old `refresh` method does
not remain as an alias.

### Awaited-read ownership

`Dashboard.refreshing` remains a Boolean derived at publication. Replace the
machine set's ambiguous completion rule with one lane-local monotonic read
sequence and one main-thread map:

```text
awaitedInventorySequenceByMachine: MachineHandle -> required read sequence
inventory result: MachineHandle + completed read sequence
```

- A routine tick creates no awaited entry.
- An awaited request while idle targets the admitted run's sequence.
- An awaited request during a run targets the one coalesced trailing sequence.
- A result clears a machine only when `completed >= required`; an older posted
  result cannot clear a newer request.
- Start/re-entry, bearer repair, mutation confirmation, access loss, and stop
  use this one owner; delete direct ad-hoc writes to the old set.
- State is process-memory only. Add no timestamp, durable receipt, queue,
  cancellation, timeout, or retry policy.

The lane may encode the sequence internally, but its tested contract is the
ordering above; do not export a general operation/ticket framework.

## historical original hard cut and cleanup

the following cleanup, ownership and red/green instructions record the original
pull-to-refresh delivery. they are not a new assignment or retained test suite;
[testing policy](rules/testing.md) governs current work.

- Delete the header `TextButton`, `onRefresh`, and `refreshing` top-bar
  parameters.
- Delete `refresh()` after all production callers move to
  `verifyVisibleInventory()`; no alias, feature flag, dual UI, or compatibility
  branch remains.
- Delete manual pressure refresh from this path. Pressure's scheduled owner is
  unchanged.
- Replace Android instructions that imply a missing button (`Refresh before...`,
  `cancel and refresh`) with gesture-literal pull copy. Past-tense factual use
  of “refreshed” and the unchanged gateway wire-error literal need not change.
- Consolidate empty and populated collection rendering under the one lazy grid;
  delete the retired parallel static-body branch and orphaned parameters/imports.
- Do not extract a generic refresh surface, operation framework, or shared
  gesture library: there is one consumer.

## historical original work split

Work is sequential at the named API seam; paths do not overlap.
Before builder work, the root integrator replaces architecture's header-button
contract and adds this delta immediately before v0.5 in the roadmap; this plan
does not silently supersede either authority.

| Order | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| 1 | Root integrator | `docs/architecture.md`, `docs/roadmap.md`, this document (status only) | Scope/header/acceptance hard cut; no runtime proof |
| 2 | Android inventory builder | `android/app/src/main/java/dev/niels/skidbladnir/SkidbladnirController.kt`, `android/app/src/main/java/dev/niels/skidbladnir/ProductModel.kt`, `android/app/src/test/java/dev/niels/skidbladnir/MultiMachineContractTest.kt` | Pure target-selection and awaited-sequence proof |
| 3 | Compose dashboard builder | `android/app/src/main/java/dev/niels/skidbladnir/DashboardScreen.kt`, `android/app/src/androidTest/java/dev/niels/skidbladnir/MultiMachineUiInstrumentedTest.kt` | Real Compose gesture/visible-state proof |
| 4 | Read-only verifier | none | Diff, rule, and gate review only |

No owner changes `GatewayClient.kt`, gateway/tmux code, build files,
`scripts/test`, catalog, terminal files, or another owner's tests. A builder
owns and observes its red before production edits. The verifier writes no test
or production file.

## historical original red / green / refactor

### Red

1. Inventory unit proof: begin an ordinary read, request verification, publish
   its leading result, and prove the request remains pending; exactly one
   trailing result satisfies it. Also prove `All`, selected, and no-live-target
   routing without network or mocks.
2. Compose component proof, fixture-driven across populated and empty/short
   collections: the old source still renders `Refresh` and a threshold pull
   cannot produce the visible checking indicator. A drag released while the
   grid remains away from top cannot verify; after it reaches top, only
   threshold overscroll does. A below-threshold drag does nothing. A no-live
   scope remains inert with no stranded indicator.
3. Update the existing compact-header proof: no `Refresh` click surface exists.

The Compose red requires the real Android runtime. Obtain explicit current-turn
platform/ADB approval before running it; without approval it is `NOT_RUN`, the
UI builder has not observed red, and the slice cannot complete.

### Green

Implement only the contracts above. Run routine `scripts/test verify`, then the
separately approved platform suite containing the new component proof. Tests
assert the indicator, retained content, stale/action state, and header outcome;
they do not merely assert callback invocation or Material internals.

### Refactor

Remove the old control, intent, direct pending-set writes, and dual empty-body
path. Keep ticket logic in the existing inventory lane/controller owner. Add no
abstraction without a second production consumer.

## Acceptance and 80/20 gates

- Pull is the only manual verification affordance and works in every collection
  state at the top of the viewport.
- The indicator cannot finish on a read that began before the pull.
- fleet targeting, per-machine partial failure, stale mutation fencing,
  card identity, fixed chrome, and five-second recovery remain intact.
- Manual pull performs no pressure request and no mutation/input operation.
- Recoverable failure copy teaches the gesture; auth/identity copy remains
  truthful and distinct.
- current engineering checks and session-view qualification belong to
  [session views](session-views.md#acceptance); historical passes above do not
  qualify the cutover. a hands-on pull checks native resistance/threshold,
  no scroll jump and no obscured first-row text or control. without its real
  device boundary it remains `NOT_RUN`, never pass.

## Non-goals

Tap or accessibility-action equivalents; contextual Retry; whole-dashboard
scrolling; pull on Forge or Terminal; pressure-on-pull; configurable cadence;
network callbacks; haptics; last-checked telemetry; analytics; push/SSE/WSS or
tmux control mode; durable/offline inventory; card-order stabilization; new
motion/ornament; automatic terminal reconnect; buffered input; API or gateway
work; generalized hooks, provenance, thread tracking, payload parsing, or any
other retired machinery.
