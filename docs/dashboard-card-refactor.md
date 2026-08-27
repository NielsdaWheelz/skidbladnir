# v0 dashboard card hierarchy

Status: implemented and verified; routine verification, the 47-test physical
S22+ platform gate, and hands-on synthetic-fixture visual/accessibility
acceptance are green on 2026-08-27.

This is the implementation contract for one hard-cut presentation refactor of
the dashboard session card. It is subordinate to
[`architecture.md`](architecture.md), [`roadmap.md`](roadmap.md),
[`design-language.md`](design-language.md), and
[`rules/testing.md`](rules/testing.md). The requested testing standard is that
last file; do not create a parallel `testing-standards.md`.

## Outcome

Make the card faster to identify and materially shorter without changing the
Niðavellir visual language:

- tmux name is the primary identity because it names the work;
- dwarf name remains a distinctive Big Shoulders signature, at lower weight;
- a fixed top-right, colour-only status facet accelerates scanning;
- the adjacent named status bay remains the semantic source of truth;
- attention remains a separate signal;
- machine is quiet text in `All` and is visually absent in a machine filter;
- objective, directory, profile, and actions occupy stable, compact strata.

This is not a new runtime capability. Tmux remains the database, the agent
remains opaque, and the phone remains a tmux client.

## Scope and final structure

One card, top to bottom:

```text
tmux-name                                      ◇ ◆
dwarf-name
[48dp dwarf seal] [WORKING                     ]
                   [lifecycle · 3m              ]
optional availability marker
optional objective, two lines
quiet directory, one line
machine · profile                         [Kill]
```

`◇` is the existing conditional attention lozenge. `◆` is the always-present
status facet. In a selected-machine view the footer is `profile [Kill]`.

Rules:

- Keep the existing card shell, angular press indication, gold top hairline,
  adaptive grid, grid keys, open behavior, and separate Kill target.
- Use one layout at every supported grid width; do not add compact/comfortable
  modes or breakpoints.
- Card padding is 10dp. Header, portrait/status row, present availability,
  present objective, present directory, and footer are major strata with 8dp
  only between present strata. Tmux/dwarf and status/evidence are internal line
  pairs. Portrait/bay and footer/Kill use at least 8dp horizontally; attention
  and facet use 4dp. Touch targets remain at least 48dp and visible text at
  least 11sp. Omitted content contributes no phantom gap.
- The tmux name is exact, high-contrast Data face, at most two lines.
- The dwarf display name is exact, one line, Big Shoulders Display face,
  smaller and quieter than the tmux name. Do not uppercase or alias it.
- The dwarf portrait is fixed at 48dp. Preserve its rendering and spoken label.
- The status bay and portrait share a row. The bay is at least 48dp high,
  flexible-width, and retains literal status, named signal, age, semantic tone,
  border, and accessible status description.
- The status facet is a 12dp `NidavellirShapes.Chip` using the existing
  `statusColor` tone. It has no text, animation, border, or accessibility node:
  it is redundant decoration, never the status source of truth.
- The attention lozenge remains independently conditional, Orpiment, animated,
  spoken, and reduced-motion safe. It sits immediately left of the fixed facet;
  its absence must not move the facet.
- Objective is absent when null and is capped at two lines.
- Preserve the existing conditional availability marker and copy. It belongs
  immediately below status, before objective/operator context; copy repair is a
  separate scope.
- Directory is absent when null and otherwise uses one quiet Data-face line.
  Preserve paths with at most two non-empty slash-separated segments; otherwise
  show `…/{penultimate}/{final}`. TalkBack receives the complete source value
  prefixed by `Directory`.
- Footer context is unbadged, muted Data text. In `All`, render the exact machine
  label then insert one structural ` · ` before the exact resolved profile
  label; do not rewrite punctuation inside either source value. In a machine
  filter, omit machine and the structural separator. Preserve `profile unknown`
  fallback.
- The footer owns one exact spoken context in both views:
  `Machine {machine}. Profile {resolved profile}.` It replaces, rather than
  duplicates, the visible footer text in semantics. Thus a machine-filtered
  card remains machine-named while `All` does not double-announce it.
- Truncation may shorten text; it must never resize the grid, overlap Kill, or
  synthesize facts.

At the default font scale and 170dp grid width, a common card with a one-line
tmux name, no objective, and a short directory must be no taller than 200dp.
Larger font scales may grow vertically and must reflow without clipping.

## Content design contract

The product/content designer owns the grammar below and approves fixture copy
before the red proof. Dynamic content remains source-owned; design never invents
agent summaries.

| Feature | Schema | Good content |
|---|---|---|
| Work identity | `session.tmuxName` | Exact operator-owned literal; no prefix, alias, or generated explanation. |
| Persona | `character.displayName` | Exact catalogue literal; typography supplies character, not rewritten copy. |
| Lifecycle | `session.status.kind`, `.signal`, `.signalAt` → `statusContent` | Exact uppercase kind, lowercase named signal, and derived age; Working/Idle require Lifecycle, Running/Shell require Process, Unknown requires PollFailure. |
| Objective | nullable `session.objective` | Exact existing value; omission is preferable to placeholder text. |
| Directory | nullable `session.cwd` | Tail-preserving visual abbreviation; complete value in accessibility semantics. |
| Context | conditional machine label, resolved profile label | No visible prefixes; insert one structural ` · ` without rewriting source punctuation. Resolve matching `ProfileChoice.label`, else raw non-null profile, else `profile unknown`. |
| Availability | existing derived marker | Existing exceptional copy and semantic tone; never restate healthy state. |
| Action | `Kill` | Existing title-case literal, confirmation, machine-aware speech, and destructive styling. |

The valid three-minute status fixture matrix is closed:

| Kind / signal | Visible bay | Spoken status | Facet |
|---|---|---|---|
| Working / Lifecycle | `WORKING` / `lifecycle · 3m` | `Observed working from lifecycle 3 minutes ago` | Moss |
| Running / Process | `RUNNING` / `process · 3m` | `Observed running from process 3 minutes ago` | Frost |
| Idle / Lifecycle | `IDLE` / `lifecycle · 3m` | `Observed idle from lifecycle 3 minutes ago` | Gold |
| Shell / Process | `SHELL` / `process · 3m` | `Observed shell from process 3 minutes ago` | Bronze |
| Unknown / PollFailure | `UNKNOWN` / `poll failure · 3m` | `Observed unknown from poll failure 3 minutes ago` | Muted |

## Capability and API contract

The only production API change is one required presentation argument:

```kotlin
@Composable
internal fun AgentCard(
    agent: VisibleAgent,
    machine: MachineState,
    showMachineLabel: Boolean,
    onOpen: () -> Unit,
    onKill: () -> Unit,
)
```

`DashboardDwarfGrid` alone derives
`showMachineLabel = state.selectedMachine == null`. No default is allowed, so a
new caller must choose its context explicitly. Machine identity remains present
in the selected filter, pressure strip, card accessibility/destructive flow,
terminal, confirmation, Forge, and error paths; only repeated visual footer
text disappears.

Keep `statusContent`, `statusColor`, `KillButton`, `DwarfPortrait`, existing
profile resolution, machine availability derivation, and the card lifetime key
as their current owners define them. Do not add a view model, DTO, gateway
field, database, cache, status enum, token, or public card system.

If the card is extracted, `SessionCard.kt` owns `AgentCard`, `DwarfPortrait`,
the private status facet/bay, attention lozenge, and the small pure directory
abbreviation helper. `DashboardScreen.kt` owns only composition and filter
context. Do not generalize a one-consumer metadata rail or badge primitive.

Planned file edge:

- `android/app/src/main/java/dev/niels/skidbladnir/DashboardScreen.kt`
- `android/app/src/main/java/dev/niels/skidbladnir/SessionCard.kt` (new)
- `android/app/src/test/java/dev/niels/skidbladnir/SessionCardTest.kt` (new)
- `android/app/src/androidTest/java/dev/niels/skidbladnir/SessionCardInstrumentedTest.kt` (new)
- `android/app/src/androidTest/java/dev/niels/skidbladnir/DashboardChromeInstrumentedTest.kt`
- `android/app/src/androidTest/java/dev/niels/skidbladnir/MultiMachineUiInstrumentedTest.kt`

`SealGalleryActivity.kt` is a compile consumer of the fixed portrait API; edit
it only if the parameter removal requires the mechanical call-site update.

## Hard cut and cleanup

- Delete the Frost machine pill, its `agent-machine-pill-*` selector, and all
  filtered-view pill assertions.
- Delete the old 58dp/default-size portrait path; require the one 48dp form at
  both card and seal-gallery call sites.
- Delete the old dwarf-first header, standalone status-chip row, three-line
  objective path, and two-line directory path.
- Move, do not copy, card-only functions during extraction; remove orphaned
  imports, comments, selectors, and tests from `DashboardScreen.kt` and
  `DashboardChromeInstrumentedTest.kt`.
- No legacy branch, feature flag, fallback layout, compatibility overload,
  deprecated symbol, animation experiment, or hidden density preference.
- `activeCommand`, `attachedClients`, sorting, grouping, status derivation,
  attention derivation, polling, navigation, Kill behavior, and gateway
  contracts are unchanged and out of scope.

## Red / green / refactor proof

Follow [`rules/testing.md`](rules/testing.md). The implementation builder owns
the red proof, observes it fail, makes the minimum production change, and then
refactors. Do not test private layout mechanics when user-visible behavior can
be queried.

One proof per ownership boundary:

1. **Pure presentation unit proof:** long, short, root, and relative directory
   fixtures establish the exact visible abbreviation. No I/O or mocks.
2. **Compose component proof:** one real `AgentCard` fixture family establishes
   tmux-before-dwarf order, fixed status-facet position,
   literal status/signal/age, independent attention semantics, complete spoken
   directory, visible `All` versus hidden-but-spoken filtered machine context,
   truncation, 48dp targets, and the 200dp common-height bound. Move existing
   card/status/attention proofs here; do not copy them. Exact font character and
   optical hierarchy belong to code review plus the designer glance, not a
   brittle implementation assertion.
3. **Existing multi-machine product journey:** retain `All` machine-label proof;
   selected filters prove session exclusion and scope orientation, not a second
   component-layout contract. Remove only the obsolete pill assertions.

Red must fail for target behavior, not an unresolved symbol. A new pure helper
may first exist only as a compile-complete identity stub so its red proof fails
by assertion. Visual tone/geometry checks may sample the facet, but literal
status and accessibility remain semantic assertions. No screenshot golden is
introduced.

Directory fixtures are exact: `/` and `/src/skidbladnir` remain unchanged;
`/srv/workspaces/skidbladnir/android` becomes `…/skidbladnir/android`;
`src/skidbladnir` remains unchanged; `workspace/src/skidbladnir` and
`~/src/skidbladnir` become `…/src/skidbladnir`. Each speaks one complete source
value as `Directory {source}`, never source plus abbreviation.

The common-height fixture is exact: width 170dp, font scale 1.0, machine
`Devbox`, tmux `ga-durinn`, dwarf `Durinn`, Working/Lifecycle three minutes
old, no attention or objective, ready/fresh inventory, cwd `/src/skidbladnir`,
profile key `work` resolved to `Codex · Work`, and `All` context. Its variants
toggle attention, selected-machine context, the five valid status pairs, and
`InventoryState.Stale(snapshot, GatewayFailure.Transport)`; stale preserves
`STALE · actions disabled`. `InventoryState.Unreachable` has no card.

The synthetic long-content fixture is exact: tmux
`skidbladnir-codex-work-12345678901234567890123456789012345678900` (64
characters), dwarf `Alberich of Nibelheim`, objective `Refactor the dashboard
card layout without changing runtime behavior.`, cwd
`/srv/workspaces/skidbladnir/android`, machine
`MacBook Pro Across The Far Tailnet Realm` (40 characters), and resolved profile
`Codex · Work`. Failure output must not echo objective or complete cwd content.

The 80/20 verification shape is:

- focused pure unit proof;
- focused Compose component proof;
- routine static/build/unit verification required by the repository;
- one separately approved S22+ platform pass executing the focused Compose
  component proof;
- one approved-device hands-on glance at typical, longest-label, attention,
  stale/non-mutating-machine, `All`, selected-machine, and large-font fixtures.

The hands-on result is `NOT_RUN` without explicit device approval in that turn.
Do not invoke tmux, integration, live, platform, or ADB as an implicit part of
this slice.

## Acceptance criteria

- A user identifies the work from tmux name before persona or metadata.
- The dwarf name remains visibly distinctive in Big Shoulders.
- Every card has one stable colour facet, while status remains fully legible and
  understandable in grayscale and to TalkBack.
- Attention and lifecycle status remain independently perceivable.
- `All` visibly disambiguates machine; a selected-machine view contains no
  repeated visual machine label.
- Objective, directory, availability, and footer obey the structure and
  truncation rules above at minimum width and large font.
- Open, Kill, disabled mutation, lifetime identity, filtering, ordering, and
  seal-gallery behavior are unchanged.
- The common-card height target passes and the UI contains no old pill/status
  layout, duplicate proof, dead selector, or compatibility path.
- Canonical architecture, roadmap, design language, and token prose agree with
  the final hard-cut behavior before the implementation edge is accepted.

## Non-overlapping work ownership

1. **Root integrator / content designer — docs only:** this file, then the
   necessary acceptance edits in `architecture.md`, `roadmap.md`,
   `design-language.md`, and `chrome-tokens.md`. No Android files.
2. **Card builder — Android card edge:** `DashboardScreen.kt`, new
   `SessionCard.kt`, new focused local/unit and instrumentation card tests,
   `DashboardChromeInstrumentedTest.kt`, `MultiMachineUiInstrumentedTest.kt`,
   and the mechanical `SealGalleryActivity.kt` call site. This single owner
   writes and observes red before behavioral production edits.
3. **Verifier — read-only:** inspect the exact diff, run only approved gates,
   check proof ownership/deletions, and report device work honestly as passed,
   failed, or `NOT_RUN`.

These slices are sequential. No second builder edits the card edge, shared
catalogue, test composition, architecture, or roadmap.

## Non-goals

No new status semantics, attention ranking, machine grouping, detail sheet,
density setting, active-command display, client count, stale-copy repair,
responsive variant, animation, data collection, API change, or terminal change.
Any of those requires a separate reviewed capability and red proof.
