# dashboard return continuity

2026-09-15 target amendment: [spaces](spaces.md) extends this retained-entry
contract with independent space selection, heading-aware viewport keys, and the
exact schema-2 task capsule. source is implemented; [the roadmap](roadmap.md#spaces--source-implemented-runtime-acceptance-open)
records verification and open device acceptance. the delivery evidence
immediately below is historical.

Status: source implemented, 2026-08-31. The JVM, real-Compose, and real-registry
boundary-owner reds, plus four review-corrective grid-reset, scope-membership,
poller-ownership, and semantic-capture reds, were observed failing for their
intended behavior. Routine verification
is green. Before the terminal-activity rebase, the production-signed
same-version 56-test S22+ candidate suite was green at `69.542` seconds;
encrypted pairing was preserved and the exact public release was restored.
That candidate does not prove the rebased tree. The rebased complete 54-test
release-bound platform gate is now green with encrypted pairing preserved and
the exact public release restored. The hands-on sample remains `NOT_RUN`.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`roadmap.md`](roadmap.md) owns delivery order; this document owns the Android
state model, delivery boundary, and red/green/refactor shape. All implementation
follows [`rules/index.md`](rules/index.md), with tests governed by
[`rules/testing.md`](rules/testing.md); no parallel `testing-standards.md` exists
or is added.

## Outcome

Attaching is a drill-in from one retained Dashboard entry. Top `Detach` and
Android Back return to that same entry: same machine and space filters, same
semantic viewport (session or group heading). live tmux inventory revalidates in
place. a return never constructs all/all at the top.

Principle: drill-in never resets the workspace. Tmux owns sessions; Android
owns this bounded navigation/spatial context.

## Goals and rules

- Preserve intent (filter), orientation (viewport), and freshness
  (current inventory) independently.
- Restore scope before post-detach verification so verification targets that
  scope; settle the grid before card interaction and reveal the selected chip
  before the Dashboard is considered settled.
- Keep one state owner and one exit path. Terminal knows nothing about its
  predecessor.
- Use the existing lazy grid, stable lifetime identity, filtering, polling,
  notices, empty states, and phone-only detach lifecycle.
- Restore the grid without animation, `All`/top flash, toast, snackbar, restore
  message, or live-region announcement.
- Apply saved restoration before cards become interactive. Selecting a
  different filter cancels it; selecting the active filter is a no-op.

## Capability contract

given a machine/space intersection, first-visible session or heading `K`, and
offset `O`:

1. Attach to any visible card, then top `Detach` or Android Back: the first
   dashboard frame has both original filters and `K` at `O` when `K` survives.
2. Inserts or reorder move `K` in the collection but not in the viewport.
3. If `K` disappeared, restore the saved index clamped into the new filtered
   collection, with `O` clamped to valid geometry.
4. If the collection is empty or the paired machine is unavailable, retain the
   filter and render the existing truthful state.
5. New cards below a former bottom do not create sticky-bottom behavior.
6. Exact horizontal filter-strip offset is not restored, but the selected chip
   is brought fully into view when it fits, or to its leading edge when it does
   not, before the restored Dashboard is considered settled.
7. Background/foreground, Activity recreation, and OS restoration of the same
   task retain the Dashboard entry. If Terminal was visible, recreation lands
   on Dashboard and never restores an attachment, terminal target, or bytes.
8. A fresh task, app-data reset, or restored machine absent from the accepted
   fleet starts all machines/all spaces at top. temporary outage never clears
   either filter. an unresolved named space retains its fingerprint selection.
9. Selecting a different filter cancels pending restoration and uses normal
   stable-key clamping; selecting the active filter is a no-op. No per-filter
   viewport history is created.
10. Terminal access loss remains the explicit exception: atomically select the
   affected machine, retain the space filter, cancel saved restoration, reset
   its viewport to top, and
   show the existing notice. Dashboard-side pending Forge/kill access loss keeps
   its existing affected-machine selection and live-grid clamping behavior.

During saved-task inventory loading, the existing neutral Booting/Reading
surface may precede the restored grid. An `All`/top or false-empty grid frame
may not.

“exact” means the same session/heading and pixel offset for unchanged geometry,
and the same semantic item with platform-clamped geometry after layout change. Fresh
tmux truth wins over frozen ordering.

## Ownership and final composition

```text
MainActivity (task/saved-state owner)
|- DashboardEntryState                 retained above destination switch
|  |- scope: DashboardScope            machine dimension
|  |- space selection                   named display label is memory-only
|  |- gridState: LazyGridState
|  `- pending saved restoration
|- SkidbladnirController               inventory/terminal/action owner
`- current destination
   |- Dashboard -> consumes entry + fresh machine state
   `- Terminal  -> exact attachment only; no return payload
```

`DashboardEntryState` is created/restored before the controller, passed to the
controller and app shell, and retained while Dashboard leaves composition. It
is the sole mutable owner of machine/space selection and viewport. controller
commands read or change it only through semantic methods; dashboard supplies ordered safe
keys for one-shot restoration, while snapshots read the owned grid directly.

After `super.onCreate`, `MainActivity` calls the entry's one production
`install(SavedStateRegistry)` adapter before constructing the controller. The
adapter consumes once and registers one snapshot provider. Do not add a second
`onSaveInstanceState` owner or persist on every scroll event.

`SkidbladnirUiState.Dashboard.selectedMachine` is deleted. The controller no
longer reconstructs navigation state in `publishDashboard`. Inventory remains
in controller process memory; no inventory snapshot or payload is copied into
the entry or saved capsule.

## state schema

the exact current capsule is schema 2 in
[spaces §10](spaces.md#10-android-navigation-and-content-free-restoration).
that section owns its primitive keys, closed variants, fingerprint framing,
unresolved-name rules, and one-time schema cut. do not implement the historical
schema-1 capsule or keep a compatibility reader.

- retain `DashboardScope.All | Machine(handle)` as the machine dimension.
- the live entry owns the independent space selection and may retain its known
  label in memory. the saved selection contains only all/unassigned/named and a
  fingerprint for named; no raw label is accepted into the saved representation.
- `DashboardCardKey` retains its current machine/id/lifetime fingerprint and
  unchanged compose key. the shared rendered-item sequence wraps it in
  `DashboardItemKey` alongside named and unassigned heading keys.
  the session fingerprint remains sha-256 over utf-8
  `skidbladnir.dashboard-card.v1`, then machine handle, tmux id, and identity
  token, each framed by its four-byte big-endian utf-8 byte length. rename does
  not change it; replacement does. no raw token enters navigation state.
- `DashboardViewport` anchors to that typed item key. its nonnegative former
  index now counts rendered items, including headings; its offset keeps the
  existing nonnegative platform meaning. no anchor requires index/offset zero.
- `DashboardEntrySnapshot` contains exact schema version 2, machine scope,
  comparison-only space selection, and viewport. it contains no inventory,
  raw label, name, objective, cwd, raw lifetime token, editor, destination,
  connection, attachment, input, or credential.
- use only the existing activity-owned saved-state registry key and adapter.
  no preferences, database, file, gateway field, salt, key store, alternate
  schema, or new dependency. no capsule/unsupported version starts fresh;
  malformed current-version state is a trusted-state defect.

## Restoration algorithm

1. Hold a restored capsule pending until the controller atomically calls
   `acceptFleet` with every handle accepted from `MachineStore`, before its
   first Dashboard publication. Reachability, access, polling, and snapshot
   freshness do not affect membership. A missing pairing resets scope, viewport,
   and pending anchor together to all machines/all spaces/top; temporary outage
   does not.
2. Keep the live `LazyGridState` object for ordinary Terminal round trips; the
   existing stable lazy key owns in-process insert/reorder anchoring.
3. for saved-task restoration, wait until every machine in restored machine scope
   either has a current/retained snapshot or has reached a non-live outcome.
   `Reading` without a snapshot remains pending; `Unreachable`,
   `AuthRequired`, or `IdentityChanged` resolves without one. A lifecycle stop
   remains pending; an unexplained foreground poller stop is a defect.
4. dashboard projects ordered `DashboardItemKey` values from the same grouped
   item sequence it renders, including retained stale rows and headings. the
   entry receives keys, not session snapshots. resolve the saved selected-space
   name from observations without changing its comparison key; an absent label
   remains selected with generic copy and does not block settled rendering. if
   the saved key survives, use its new index and saved offset; otherwise clamp
   the saved index. empty stays empty.
5. Call `requestScrollToItem(resolvedIndex, savedOffset)` once while cards are
   withheld, then consume restoration and expose the grid for its next measure;
   platform layout clamps geometry. Never animate from top. Existing neutral
   Booting/Reading may remain pending.
6. While pending, `snapshot()` re-emits that accepted capsule unchanged, so
   repeated process death cannot erase place. Otherwise it synchronously matches
   `gridState.firstVisibleItemIndex` to the last layout's visible session/heading key
   and records that index/offset; no scroll observer or inventory shadow exists.
   no measured session/heading key means no anchor/top.
7. Later polls and filter changes use normal keyed lazy-grid behavior. Selecting
   a different filter cancels the pending capsule; selecting the active one is a
   no-op. No delayed corrective scroll runs after user input.

Do not save prior ordering merely to choose a historical neighbor. The accepted
fallback is the new collection's clamped former index; this avoids persisting an
inventory projection.

## Internal API and composition

The state holder exposes only the behavior the two current consumers need:

```kotlin
val scope: DashboardScope
val space: DashboardSpaceSelection
val gridState: LazyGridState
val restorationPending: Boolean
fun acceptFleet(handles: Set<MachineHandle>)
fun selectScope(scope: DashboardScope)
fun selectSpace(space: DashboardSpaceSelection)
fun selectTerminalAccessLoss(handle: MachineHandle)
fun resetAll()
fun restoreOnce(keys: List<DashboardItemKey>)
fun snapshot(): DashboardEntrySnapshot
```

Mutation stays main-thread confined. `acceptFleet` is the only restored-scope
normalization boundary; after it, invalid scope selection is a caller defect.

- `DashboardEntryState.kt` owns the types, exact saved-state adapter,
  fleet-validation rule, anchor resolution, and live grid state.
- `SkidbladnirController` receives one required `DashboardEntryState`; no
  default, singleton, mirrored field, or Terminal return field exists.
- `Dashboard` and `Terminal` implement one closed `SkidbladnirUiState.Workspace`
  subtype; Booting and FleetConnect remain outside the workspace host.
- `DashboardEntryState.selectScope(DashboardScope)` replaces
  `selectMachine(MachineHandle?)`; the owner rejects an unpaired machine,
  cancels pending restoration, and lets the live keyed grid clamp. The current
  scope is a no-op.
- `selectTerminalAccessLoss` always selects that machine, cancels restoration,
  and schedules top while preserving space selection. `resetAll` clears both
  filters, pending state, and viewport to all/all/top. these are semantic
  operations, not a boolean viewport policy.
- session grouping consumes both filters. inventory targets, pressure visibility,
  and refresh routing read machine scope only. forge adds the explicit space
  prefill/unresolved-choice rules from the spaces spec.
- The controller is the sole restoration-readiness oracle: background lifecycle
  stop leaves pending state untouched; a foreground Ready/Reading machine with
  no owned poller is a defect; modeled non-live outcomes may resolve empty.
- Dashboard supplies only its ordered safe keys from one `LaunchedEffect` keyed
  to pending state, scoped machine outcomes, and those keys. That effect calls
  one controller command; the controller checks readiness and invokes
  `restoreOnce` only when ready. Readiness is never mirrored into UI state.
- `DashboardScreen` and `DashboardDwarfCollection` receive the entry explicitly.
  The collection-local `rememberLazyGridState()` is removed.
- While `restorationPending`, the collection renders the existing content-free
  centered progress treatment as a distinct branch; it never sends an empty
  projection through the ordinary empty-state renderer. Once machine outcomes
  resolve, `restoreOnce` uses `LazyGridState.requestScrollToItem` before the
  next grid measure, then exposes cards.
- `MachineFilters` gives only the selected chip a `BringIntoViewRequester` and
  issues one request when a selected scope enters composition or explicitly
  changes. It persists no strip offset, moves no focus, emits no announcement,
  and never repeats for a poll or recomposition. Platform-minimal horizontal
  relocation is allowed; direct horizontal input wins.
- Machine-filter taps call `entry.selectScope` directly. The controller reads
  or forces scope only for controller-owned operations; it exposes no gated
  pass-through selection command.
- lazy item identity and restoration use the same `DashboardItemKey` projection.
  session keys remain the existing `DashboardCardKey` strings; headings have
  the comparison-only keys defined in spaces. test tags remain selectors, not
  another lifetime identity.
- Extract one production `DashboardTerminalHost` in `MainActivity.kt`. It owns
  the closed destination switch and Terminal Back, renders the actual screens,
  and receives required `onOpenTerminal` and `onDetach` events. Production binds
  both to the controller; the Compose proof binds only those route events to its
  local destination fixture. There is no slot, screen factory, or navigator.
- `DashboardScreen` receives `entry` and required `onOpenTerminal` and threads
  the latter to cards. `TerminalScreen` receives required `onDetach`; top
  Detach and reconnect-panel Dwarves use it. Neither event has a default.
- The controller detach operation tears down the phone attachment, publishes
  Dashboard, then verifies `entry.scope`.

this navigation owner adds no public api, terminal protocol, credential store,
or dependency. the spaces feature owns its separate host membership operation;
its presence does not make terminal navigation a host responsibility.

```kotlin
@Composable
internal fun DashboardTerminalHost(
    state: SkidbladnirUiState.Workspace,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
    onOpenTerminal: (SessionTarget) -> Unit,
    onDetach: () -> Unit,
)
```

`DashboardEntryState.install(savedStateRegistry)` is the sole construction path
for an Activity owner and may be called exactly once per owner.

## Content and interaction design

The product/content designer owns this closed schema. No new runtime content
type is created.

| Situation | Good content |
| --- | --- |
| Successful return | No message; continuity is the feedback. |
| Filtered collection now empty | Existing empty state with the selected chip still visible. |
| Selected machine unavailable | Existing machine notice and disabled-action treatment. |
| Anchor disappeared | Silent deterministic fallback. |

Add no restore message, preference, bookmark, `Clear` action, sticky-bottom
control, highlight animation, haptic, telemetry, or explanatory copy.

## historical original hard cut and cleanup

the following delivery instructions describe the original return-continuity
implementation. they are retained as context for its evidence, not a new
assignment. spaces uses its own [file and removal contract](spaces.md#11-reuse-removals-and-files),
which supersedes the old path exclusions and card-only capsule design.

Delete in the same implementation:

- nullable `selectedMachine` from Dashboard and every nullable-scope helper;
- the Dashboard-only cast in `publishDashboard`;
- collection-local `rememberLazyGridState()` and its import;
- inline raw-token card-key construction;
- any obsolete tests, helpers, comments, or parameters tied to those owners.

Retain no old/new overload, `returnFilter`/`returnScroll` Terminal field,
shadow index cache, compatibility saver, hidden composed Dashboard, feature
flag, preference, fallback navigator, or test-only production seam. Do not add
Navigation 3, a ViewModel migration, or a generic state/navigation framework
without a second production requirement.

## historical original files and non-overlapping ownership

| Slice | Exclusive paths | Proof |
| --- | --- | --- |
| Root integrator / product-content designer | `docs/dashboard-return-continuity.md`, `docs/architecture.md`, `docs/roadmap.md` | Reviewed authority; no runtime claim |
| Android builder | new `android/app/src/main/java/dev/niels/skidbladnir/DashboardEntryState.kt`; `ProductModel.kt`; `SkidbladnirController.kt`; `MainActivity.kt`; `DashboardScreen.kt`; `TerminalScreen.kt`; new JVM `android/app/src/test/java/dev/niels/skidbladnir/DashboardEntryStateTest.kt`; `MultiMachineContractTest.kt`; `MultiMachineUiInstrumentedTest.kt` (Compose and registry cases); mechanical `ProductContractTest.kt` and `TerminalChromeInstrumentedTest.kt` updates | One JVM, one Compose, and one real-registry red; owns and observes all three |
| Read-only verifier | none | Diff, residue, behavior, and gate review only |

The Android edge stays with one builder because route state, controller
transition, and destination disposal form one compile/runtime seam; splitting
them would create overlapping ownership. Do not touch `SessionCard.kt`,
`SessionCardInstrumentedTest.kt`, `TerminalConnection.kt`, `GatewayClient.kt`,
build files, manifests, gateway/tmux code, `catalog/`, or `scripts/test`.

Each changed ownership boundary has one proof shape:

| Boundary | Sole proof |
| --- | --- |
| Scope, saved snapshot, fingerprint, anchor resolution, refresh targeting | One JVM fixture matrix |
| Retained entry, destination disposal, Detach/Back, grid/chip geometry | One real-Compose journey |
| Production `SavedStateRegistry` adapter | One real-registry owner round trip |

No external protocol or tmux ownership boundary changes, so none receives a
ceremonial duplicate proof.

## historical original red / green / refactor

**Baseline:** run `./scripts/test verify`. Do not run tmux or a device gate.

**State red:** add one fixture-matrix JVM proof against the production state
owner: typed MacBook scope survives snapshot/restore, targets only MacBook,
resolves the same lifetime after reorder, clamps a deleted lifetime, validates
an absent pairing atomically to `All`/top, retains a present-but-unavailable
pairing, consumes its modeled non-live empty outcome, preserves pending state
across a lifecycle stop/save-again, and defects on an unexplained foreground
stop. The same matrix covers `All`, active-scope no-op versus different-scope
cancellation, Terminal access-loss top override, and one literal canonical
fingerprint vector; it does not derive the expected digest with production
code. Update—not duplicate—the existing access-loss proof for its new state
owner. No context, I/O, network, storage, reflection, or mocks. If a new target
type is required to compile, add only a non-working identity skeleton; red must
fail an assertion, not compilation.

**Compose red:** one real `LazyVerticalGrid` journey uses production host, entry,
filter, card, and collection code with the existing overflowing filter fixture
and enough deterministic MacBook cards to overflow the grid. Select MacBook
through UI, scroll, record stable-card bounds, open a card through the host, and
return once through the actual top Detach control and once through Android Back.
Use an actual non-started controller only for unused screen actions and close it
in `finally`. The Terminal fixture is exactly `Verifying`, `kill = null`, and
`rename = null`, so no WebView is constructed. Route events drive the fixture,
not a fake navigator. After settling—and before test scroll/click—assert
selected semantics and the same card top within `1dp`. A fitting selected chip
must be wholly within the filter viewport; an oversized one aligns its leading
edge within `1dp` and may overflow the trailing edge. Do not assert internal
index, saver fields, scroll distance, or callback counts; do not mock internal
components or duplicate the production host. In this same journey, hold
restoration in `Reading`, advance frames manually, and assert each observable
Dashboard frame keeps MacBook selected and exposes neither `All` selection nor
the false-empty state before the restored cards appear. Because this component
fixture's controller is intentionally non-started, the JVM matrix owns its
readiness decision; after the fixture advances to Fresh, the test drives the
real entry's `restoreOnce` with that projection's production-derived keys. It
adds no controller injection or alternate restoration path.

**Registry red:** one Android component case drives the production adapter with
real `SavedStateRegistryController` owners: a pending nonzero capsule proves
scope, fingerprint, index, offset, and exact save-again behavior; a settled
fresh entry proves the provider's `All`/top state. A component with no Compose
layout cannot honestly manufacture a measured settled anchor, so it does not
try; that adapter-plus-layout composition belongs to the UI journey and the
hands-on recreation sample. The same registry case covers absent state, an
unknown version starting fresh, and malformed current-version state defecting.
It lives in `MultiMachineUiInstrumentedTest.kt` and uses no Activity, Compose
state-restoration surrogate, store, network, mock, or new test seam.

Both instrumented reds require explicit current-turn platform approval. Without
it, they are `NOT_RUN`; the Android builder must not implement working production
behavior until all three owned reds have been observed failing. A compile-only,
non-working identity skeleton is the sole permitted precursor.

**Green:** implement only the contracts above. Run `./scripts/test verify`, then
the separately approved complete platform gate.

**Refactor:** remove retired owners and nullable paths, centralize the existing
lifetime key, and update existing fixtures rather than duplicating them. Add no
abstraction without its named production consumer.

## original acceptance and 80/20 verification

- MacBook/bottom returns through Detach and Back with filter, anchor, and offset
  intact; no `All`/top frame appears.
- `All` obeys the same contract.
- Reorder, insertion, anchor deletion, empty, and unavailable states follow the
  capability contract without stale action admission.
- Post-detach verification targets only the restored visible live scope.
- Background and saved-task recreation restore Dashboard context; Terminal is
  detached and no content/input/target is restored or replayed.
- Fresh task/reset starts `All`/top.
- Pull-to-refresh stationary viewport, pressure scope, Forge recovery, and the
  explicit access-loss override remain unchanged.
- Routine verification is green.
- One separately approved S22+ platform pass is green, followed by one hands-on
  real-`MainActivity` MacBook-bottom sample covering attach -> Detach, attach ->
  Back, and same-task Activity/process recreation. It checks Dashboard-not-
  Terminal, spatial continuity, and selected-chip visibility with no captured
  content. Without approval, device, live overflow fixture, or exact lifecycle
  control, the affected case is `NOT_RUN`, never pass.
- Integration, provider-live, repository-live, host, tmux, and the full
  three-host product gates do not re-prove this Android boundary and must not
  run; only the explicitly scoped hands-on sample touches the already-running
  product.

## retained non-goals and tradeoffs

No independent viewport per filter; cold-launch/device-reboot continuity;
exact horizontal filter-strip offset restoration; pressure/Forge sheet
restoration; keyboard/D-pad input-focus restoration; TalkBack/Switch Access
focus restoration; manual sorting; sticky bottom; persisted inventory; terminal
scrollback or target persistence; automatic reattach; deep link; adaptive split
pane; predictive Back redesign; new decorative/highlight motion or content;
analytics; backend work; or cross-device state.

Tradeoffs are explicit:

- One active Dashboard context solves the reported journey; filter-to-filter
  history remains absent. Filter changes use the same keyed grid and therefore
  clamp rather than recreate a remembered position for each filter.
- task saved state carries comparison-only filter and item fingerprints; the gain is honest
  semantic restoration after OS recreation, and the cost is a narrowly
  documented exception to “Android persists only pairings.” The stored form is
  only a deterministic fingerprint: it leaks equality but cannot be replayed as
  a terminal/mutation header. label hashes are dictionary-guessable and are not
  a secrecy mechanism. hash work is one bounded pass per visible
  collection projection.
- System task state, rather than preferences, preserves continuity without
  inventing a durable workspace record. The accepted cost is that a true cold
  launch or OS-discarded task starts `All`/top.
- Fresh inventory may change the surrounding order; continuity anchors the
  user's place rather than presenting stale pixels.
- Deleted-anchor fallback uses a clamped index, not a saved neighbor list; this
  keeps inventory out of persistence.
- A non-live machine with no retained snapshot consumes restoration as empty;
  later recovery begins at top. This gives up delayed place recovery to prevent
  a corrective scroll after the user has resumed interaction.
- Saved restoration of `All` waits for each machine's first retained/current or
  non-live outcome. This trades fastest partial rendering for one deterministic
  anchor resolution with no later jump.
- Exact input and accessibility focus restoration is deferred. Compose input
  focus is not accessibility focus, so claiming one proof for both would be
  false confidence; this slice preserves filter and place only. A later focus
  capability needs its own interaction design and assistive-technology proof.
- The filter strip keeps no offset. One platform-minimal bring-into-view motion
  may reveal the selected chip after loading; avoiding even that motion would
  require another retained viewport and is outside the 80/20 boundary.
- The Activity-owned holder and closed two-destination shell remain;
  ViewModel/SavedStateHandle or Navigation 3 becomes justified only with another
  lifecycle owner, a third destination, deep links, multiple stacks, or
  adaptive panes.
- Automated saved-state coverage proves the production registry adapter with a
  deterministic real owner, not network-coupled `MainActivity`. The full app
  composition stays one hands-on acceptance sample; this avoids fake
  controller/storage seams, but the component green never substitutes for that
  separately reported sample.
