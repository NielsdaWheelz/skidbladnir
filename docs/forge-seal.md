# Design delta D5: the Forge seal

Status: implemented on `codex/forge-seal`, 2026-08-27. Mergeable in either
order with [dashboard-pull-to-refresh.md](dashboard-pull-to-refresh.md) (P2R);
see "Sequencing" below.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`design-language.md`](design-language.md) owns visual identity and wins on
every value here; [`roadmap.md`](roadmap.md) owns delivery order. This document
owns the create affordance's implementation boundary. Testing standard is
[`rules/testing.md`](rules/testing.md).

## Outcome and final state

The dashboard's create affordance leaves the header and becomes **the Forge
seal**: a 56dp octagonal control anchored bottom-end, carrying **the unstruck
seal** — the dwarf seal with every trait at zero. It is lit in `ForgeGlow` when
a machine can forge and cold stone when none can.

The header then carries no create affordance. It keeps its title, its machine
summary, and — until P2R lands — `Refresh`, which is P2R's to delete.

## Sequencing

P2R deletes the header `Refresh` button and consolidates the collection into one
`PullToRefreshBox` + one `LazyVerticalGrid`. D5 deletes the header's create
button and overlays the seal on the grid. Both were drafted asserting the other
delta's half of the header, which would break whichever merged second. The
boundary is re-cut by concern instead, and each delta's proofs assert only its
own contract:

- **P2R owns the refresh gesture and the inventory intent.** It proves no
  `Refresh` click surface remains, and says nothing about the create action.
- **D5 owns the dashboard's action chrome.** It proves no create affordance
  remains in the header and that the seal is the only one, and says nothing
  about `Refresh`.

Two P2R sentences must be amended to that effect, **by P2R, in its own branch**.
D5 does not touch that file: it is untracked in both worktrees, so editing it
here would not amend it — it would land a whole stale copy over P2R's live one
and silently revert decisions P2R has already taken. The amendment is:

- Hard cut: drop `` `New dwarf` remains trailing in the same 64 dp row `` — say
  nothing about the create action's position.
- Red proof 3: drop the same clause; assert only that no `Refresh` click
  surface exists in the header.
- Composition sketch: name the header by content, not by button.

Code merges in either order. **Docs do not**: D5 and P2R rewrite the same
`architecture.md` paragraph, and P2R's version re-asserts the trailing `New
dwarf` header action. That conflict is unavoidable and resolves by hand to
P2R's pull-to-refresh prose plus D5's Forge-seal sentence, with the header
described as title and summary only.

Neither delta deletes `DashboardTopBar`. It stays a real composable while either
button lives in it: after D5 alone it is the title block plus `Refresh`. Once
both have landed it is a title block, and inlining it is a one-line cleanup
owned by whichever merges second.

## Scope

The create affordance's mark, control, placement, states, and semantics, plus
the two primitives it forces open (`octagonVertices`, `AngularIndication`'s
shape) and the dead generality it exposes in `drawOrnamentBand`. Everything
else is consumed unchanged.

## Closed decisions

1. **No hammer, anvil, crossed hammers, or tongs.** Design language §8 and §15
   ban the register. Provenance is not the objection (Skíðblaðnir and Mjölnir
   come from one wager, *Skáldskaparmál* 35) — Skíðblaðnir loses that wager,
   Mjölnir marks the wielder not the smith, and the silhouette dies at 24dp
   monoline. §15 records the reconsideration so it stays decided once.
2. **The mark is not generated.** `scripts/gen-ornament` exists for computed
   topology (weave parity, baked over/under gaps). The unstruck seal is an
   octagon plus two straight lines; generating it would add a drift gate with
   nothing to guard. It is drawn in Compose from frozen fractions.
3. **The crossbar is centred**, at 50% of the stave. It reads as `+`.
4. **The lit/cold flip does not animate.** §12 budgets one ambient animation
   (the Forge sheet warm-in). `canForge` is a discrete reported fact; it renders
   instantly. No new motion token.
5. **Icon-only.** No label, tooltip, extended form, or long-press menu.
6. **The seal renders in every dashboard state**, including zero machines, cold.
   Absence is displayed, not hidden.
7. `testTag("new-agent")` moves with the control, unchanged. Test selectors do
   not change with product language (roadmap, dwarves delta).

## Goals and rules

- **Structural, not propped.** The mark is the seal's own geometry at zero, not
  a tool drawn onto a button (§1.2, §8).
- **Honest by construction.** The catalogue key is assigned server-side after
  creation ([automatic-dwarf-identity.md](automatic-dwarf-identity.md)); the
  client cannot know the identity it is about to create. The blank is the only
  truthful picture (§1.4).
- **Provably not text.** Not one of the 30 segments in `RuneSegments` is
  horizontal — runic branches are cut across the wood grain, never along it.
  (29 are diagonal; úr has one vertical segment, which is why the claim is
  "no horizontal", not "all diagonal".) A perpendicular bar can therefore
  never be a rune, so §8's "runes are ornament, never text" holds by
  construction and is enforced as a unit proof, not a promise.
- No new provenance row: the octagon (§2, faceted planar surfaces) and the
  shared stave (§2, bind-rune monograms) are already in the ledger.
- Corners cut, no curve, no shadow, no glow implying unreported activity.
- Reuse before adding: no new dimension-token system, no shape library, no
  second drawing path.

## Capability contract

### Geometry (unit box 0..1 on the control's square)

| Constant | Value | Meaning |
| --- | ---: | --- |
| octagon cut | `0.29` | identical to `NidavellirShapes.Octagon`; the frame and the clip are one geometry |
| `StaveTop` | `0.23` | stave start |
| `StaveBottom` | `0.77` | stave end, at `x = 0.5` |
| `BarY` | `0.50` | crossbar height |
| `BarHalfWidth` | `0.19` | crossbar spans `0.31 … 0.69` |

Invariant (machine-checked): every mark endpoint sits ≥ `0.10` of the side from
every octagon edge. Verified minimum is `0.23` at the stave tips and `0.31` at
the bar ends; the two half-strokes consume `0.040`, leaving a painted gap of
`0.190` of the side — 10.6dp at 56dp.

Strokes are optical, not fractional: frame `1.5dp`, mark `3dp`. The frame is
heavier than the seal's `side × 0.012` hairline because there is no mineral
ground separating it from the field. The half-strokes consume `0.040` of the
side as a *sum* — mark `0.027`, frame `0.013` — not each.

**The frame is drawn unclipped.** `DwarfPortrait` clips its box to the octagon
and then strokes the same vertices, so exactly half its hairline is clipped
away; there it does not matter, because a mineral field backs the frame.
`ForgeSeal` has no such ground, and copying that idiom would render the
specified `1.5dp` frame as a `0.75dp` inner line that goes weak against Ink —
which is the reason the frame was thickened in the first place. So: fill the
octagon path, then stroke its eight edges with no clip. Same vertices, so
"the frame and the clip are one geometry" is unaffected.

**56dp is a floor as well as a size.** Strokes are dp-fixed, so shrinking the
control thickens the mark relative to it: at 24dp the mark is 12.5% of the side
against 5.4% at 56dp, and the bar arms project only 3.06dp past a 3dp stave — an
arm-to-stroke ratio of 1.02 against 3.05 at 56dp. It survives (unlike the
valknut, which closes below ~64dp, because this mark has no crossings to lose),
but `ForgeSeal` must not be reused below ~40dp. The app ships exactly one
instance, at 56dp.

### States

One input, `canForge: Boolean`; pressed comes from the interaction source.

| State | Field | Frame + mark |
| --- | --- | --- |
| Lit | `ForgeGlow` | `Gold` |
| Cold | `DeepSurface` | `Muted` at `DisabledAlpha.Content` |
| Pressed | unchanged | `AngularIndication(Octagon)` over it |

Cold changes hue *and* field, not opacity alone — §12's required non-opacity
cue. The perceptual work is done by the metal, not the field; see the rubric.
Lit is the only appearance of `ForgeGlow` outside the Forge sheet, and it
reports a fact the app holds (`MachineState.canForge`), never activity.

### Placement and semantics

- 56dp, anchored `BottomEnd`, 16dp margin, inside `systemBarsPadding`. The
  margin belongs to a wrapper `Box`, not to `ForgeSeal`: `Modifier.padding` is a
  layout modifier on the same node, so threading it through the control would
  report semantics bounds 32dp larger than the octagon a user can see — and the
  clearance below is measured against exactly those bounds.
- The grid's bottom `contentPadding` becomes `16 + 56 + 12 = 84dp`; the last row
  never sits under the seal.
- `contentDescription = "New dwarf"`; `Role.Button`; `enabled = canForge` so
  disabled state is spoken; 56dp ≥ the 48dp floor, so no
  `minimumInteractiveComponentSize()`.

## Structure and composition

```text
Box(fillMaxSize, Ink, systemBarsPadding)
|- Column(fillMaxSize)
|  |- title + summary            <- inlined; no click surface, no fixed height
|  |- MachineFilters / MachineStrip* / notices / recovery
|  `- PullToRefreshBox           <- P2R
|     `- LazyVerticalGrid        <- contentPadding bottom 84dp
`- ForgeSeal(align = BottomEnd, padding 16dp)
      canForge -> field + metal
      click    -> controller.openForge()
```

## API design

```kotlin
// Theme.kt — one owner for the 29% cut expansion; DwarfPortrait and
// ForgeSeal both consume it instead of open-coding vertices.
internal fun octagonVertices(size: Size): List<Offset>

// Theme.kt — collapsed; see "Reuse and consolidation".
internal fun DrawScope.drawFretBand(color: Color)

// AngularIndication.kt — already shape-parameterised on main: the destructive
// chrome delta (D6) needed a Cleft-shaped press for the kill control and made
// the same object -> data class change D5 had specified. D5 consumes it as-is
// and adds the third shape.
internal data class AngularIndication(val shape: Shape) : IndicationNodeFactory

// ForgeSeal.kt (new) — the mark in the control's unit box, exactly two
// segments: the stave and the crossbar. One owner for the frozen fractions.
internal val UnstruckMark: List<Pair<Offset, Offset>>

// ForgeSeal.kt (new) — one consumer, so no size, label, or colour parameter,
// and no `modifier` either: the control is exactly its own 56dp square, so its
// semantics bounds and its touch target are one rectangle (the frame's outer
// half-stroke, 0.75dp, overhangs it). Placement is the caller's, in a padded
// wrapper. (`DwarfPortrait`, `EmptyState`, and
// `AgentCard` take no modifier either; this codebase does not follow the
// always-accept-a-modifier convention, and rules/simplicity forbids a
// parameter with no call site.)
@Composable internal fun ForgeSeal(canForge: Boolean, onClick: () -> Unit)
```

`UnstruckMark` lives in `ForgeSeal.kt`, where it belongs, which puts that file's
class initialiser inside a pure-JVM proof. Every other top-level declaration
there must therefore stay JVM-safe — the one `private val` holding
`AngularIndication(NidavellirShapes.Octagon)` is; a top-level `Path`, `Paint`,
or `Stroke` would not be, and would fail `ForgeSealTest` for a reason that has
nothing to do with the mark.

Each consumer constructs its own indication inline, as
`AngularIndication(NidavellirShapes.Card)` and
`AngularIndication(NidavellirShapes.Cleft)` already do: the data class's
structural equality is what stops `Modifier.clickable` churning across
recompositions, so hoisting the instance buys nothing.

No HTTP, WSS, tmux, gateway, DTO, persistence, credential, controller-intent, or
terminal change. No dependency is added.

## Content design (designer-owned)

The mark designer owns the geometry table and the state palette above, and
froze this rubric with them. **Good** is:

*Machine-checked:*

1. `octagonVertices` expands the cut that `NidavellirShapes.Octagon` itself
   reports, read off all four of its corners — the frame and the clip are
   provably one geometry, not two that agree today.
2. Every mark endpoint clears every octagon edge by ≥ `0.10` of the side by
   point-to-segment distance, and the whole frozen table is pinned as a golden
   vector. Pinning only the minimum is not enough: a wider bar would leave the
   minimum unchanged.
3. The crossbar is horizontal, and none of the 30 `RuneSegments` entries is.
4. Lit's field pixel is `ForgeGlow` and cold's is `DeepSurface`, sampled at the
   quarter-point, clear of every stroke.

*Hands-on:*

5. At 56dp on Ink at arm's length, the shape is nameable as one cross without
   leaning in: the bar is attached at the stave's centre, and the crossing is
   not a blob.
6. Beside any `DwarfPortrait` at its shipped size — the gallery renders the
   seals at 56dp and the portraits at dwarf-seals.md's 48dp acceptance size —
   the unstruck seal is called apart on sight by three absences — no mineral hue, no beard mass, no branch
   leaving the stave. (The missing Bone initial is not a rubric line: `ForgeSeal`
   has no `Text` child, so its absence is structural, not a judgement.)
7. In peripheral vision alone, lit and cold are called correctly. **The cue is
   the metal, not the field.** Gold against Muted-at-38% is 3.87:1 apart with a
   full hue swing; `ForgeGlow` against `DeepSurface` is 1.151:1 and carries
   nothing perceptually. Proof 4 still pins the field — it is what catches an
   opacity-only regression — but nothing may promise a user reads it.

Recorded so a later reviewer does not "fix" it: the cold mark over its own field
is 2.13:1, below the 3:1 non-text floor. That is intended. It is §12's
`DisabledAlpha.Content`, WCAG 1.4.11 exempts inactive components, and the cold
seal is located by its frame against Ink rather than read.

Also recorded, because it is the subtle half of the §8 argument: a bare stave is
*not* non-runic. It is exactly how íss renders, since `RuneSegments[8]` is empty
and `DwarfPortrait` draws the shared stave itself. The crossbar is the whole
argument — proof 3 must stay anchored on it.

## Hard cut and cleanup

- The header's `Button` goes, and `canForge` and `onNewAgent` leave
  `DashboardTopBar`'s parameter list entirely. `canForge` is computed in
  `DashboardScreen` and reaches only `ForgeSeal`. `DashboardTopBar` itself,
  the `dashboard-top-bar` tag, the 64dp height, and `dashboard-title` all stay
  — see "Sequencing".
- `AngularIndication`'s singleton `object`, its `other === this` equals, and its
  `System.identityHashCode` are deleted.
- The row-geometry assertions in `MultiMachineUiInstrumentedTest` ("New dwarf
  trails the title inside the bar") are deleted, not adapted — they assert the
  layout of a control that no longer exists there. That test's
  machine-administration assertions are orthogonal and survive intact.
- No flag, no dual affordance, no header fallback, no legacy path.

## Reuse and consolidation

- **Extract, don't duplicate.** The octagon's eight vertices are currently
  open-coded inside `DwarfPortrait`. D5 would be the second writer, so the
  expansion moves to `Theme.kt` beside `NidavellirShapes.Octagon` and both draw
  from it. The frame a user sees and the clip that cuts it can no longer drift.
- **Generalize the one primitive that says so.** `AngularIndication`'s own
  comment names this reopening: *"Card-only by design — a second consumer with
  another shape reopens docs/chrome-tokens.md, not this file."* D5 is that
  consumer.
- **Extirpate dead generality.** `drawOrnamentBand(unitAspect, layers)` has one
  caller passing `unitAspect = 1f` and a one-element list. Its second layer died
  with the pairing screen's interlace band (ornament-pipeline.md). It collapses
  to `drawFretBand(color)`. Taken here because `Theme.kt` is already in D5's
  path and rules/simplicity bans speculative surface; it is two deleted
  parameters and one call site, and nothing else in the ornament system moves.

## Work split

Sequential at named seams; paths do not overlap. A builder observes its red
before production edits. The verifier writes no file.

| Order | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| 1 | Mark designer | `docs/forge-seal.md` (geometry table + rubric) | Frozen values and the "good" rubric; no runtime proof |
| 2 | Root integrator | `docs/architecture.md`, `docs/design-language.md`, `docs/chrome-tokens.md`, `docs/roadmap.md` | Contract amendments; no runtime proof |
| 3 | Compose builder | `Theme.kt`, `AngularIndication.kt`, `ForgeSeal.kt` (new), `DashboardScreen.kt`, `SealGalleryActivity.kt`, `ForgeSealTest.kt` (new, JVM), `DashboardChromeInstrumentedTest.kt`, `MultiMachineUiInstrumentedTest.kt` | Every runtime proof below |
| 4 | Read-only verifier | none | Diff, rule, and gate review only |

Slice 2 amends: architecture §"The dashboard header is one compact row…" →
title, summary, and the bottom-anchored create action; design language §5
(ForgeGlow is no longer the Forge sheet alone), §6 (29% is not exactly regular),
§8 (the unstruck seal beside the Hlíðskjálf mark), §13 (the Forge seal), §15
(hammer reconsidered and re-rejected on register), §17 (the D5 row);
chrome-tokens §"Angular indication" (no longer a Card-only singleton); roadmap
(D5 entry and status row).

## Red / green / refactor

One proof per ownership boundary. Each red fails first — proofs 1–2 by
referencing symbols that do not exist.

**Geometry (JVM unit, `ForgeSealTest.kt`)**

1. The mark's crossbar is horizontal and no `RuneSegments` entry contains a
   horizontal segment, across all 16 rune indices (index 8, íss, is legitimately
   empty) and all 30 segments. This is §8 enforced.
2. `octagonVertices` returns 8 vertices whose cut is the one
   `NidavellirShapes.Octagon` itself reports — read off the shape, not restated
   — and every mark endpoint clears every octagon *edge* by ≥ 0.10 of the side.
   Point-to-segment distance: a vertex-only check would pass a mark that pokes
   through the middle of an edge. Do not assert the eight edges are equal; at
   the shipped 29% cut they are not (design-language.md §6).

**Control (component, `DashboardChromeInstrumentedTest.kt`)**

3. Lit: the node tagged `new-agent` speaks `New dwarf`, is ≥ 48dp square, is
   enabled, and one tap requests the Forge exactly once.
4. Cold: the same node is not clickable, and its field pixel at
   `(0.25 · side, 0.25 · side)` — inside the octagon, off both strokes — is
   `DeepSurface`, not `ForgeGlow`. The hue cue is real, not opacity alone.

**Dashboard composition (journey, `MultiMachineUiInstrumentedTest.kt`)**

5. The header carries no create affordance; the create affordance is a single
   node in the bottom-end quadrant; tapping it opens `forge-sheet`; and after
   scrolling to the last card, that card's bounds do not intersect the seal's.

**Deliberately unproven (80/20).** `AngularIndication`'s shape parameter gets no
test of its own: proof 3 and the existing card-click proof are both consumers'
gates, and asserting the flash's pixel geometry would assert styling, which
rules/cleanliness forbids.

The **scrolled** last-row clearance is hands-on, not automated. The journey runs
against whatever sessions two real machines happen to hold — as few as two, which
never fill the viewport, so a scroll-and-measure assertion there would pass
without exercising the 84dp inset at all. Proof 5 asserts the honest,
deterministic part (the seal overlaps no rendered card, on a card count that must
be non-zero); the scrolled case is the acceptance glance below.

Closed decision 6's **zero-machine** case is also unproven. `DashboardScreen`
takes a `SkidbladnirController`, whose constructor spawns three executors, a
`MachineStore` over real storage, and a `GatewayClient`, and starts polling — so
no component test can render it, and the device journeys all require provisioned
pairings. The nearest gate is `genuinelyUnavailablePairingDisablesMachineMutations`,
which does render a cold seal over an empty grid once the failed machine is
filtered. The `state.machines.isEmpty()` branch itself is unreachable on a
provisioned device and is not gated at all.

Proofs 3–5 require the real Android runtime. Obtain explicit current-turn
platform/ADB approval before running them; without approval they are `NOT_RUN`,
the builder has not observed red, and the slice cannot complete.

**Green:** implement only enough to satisfy the proofs and the visual contract.
Routine `scripts/test verify` stays green and device-free.

**Refactor:** collapse `drawOrnamentBand` to `drawFretBand`; delete the header's
create `Button` with the parameters and imports it orphans; add no abstraction
without a second production consumer. `DashboardTopBar` itself stays — see
"Sequencing".

## Acceptance and 80/20 gates

Status as run on 2026-08-27, from the Linux devbox with the S22+ attached:

- **Routine verification — GREEN.** `scripts/test verify` (static, build, unit),
  including proofs 1 and 2. Red was observed first on both source sets, each
  failing only on the intended unresolved symbols.
- **S22+ instrumented platform pass — GREEN**, 35 tests, up from 33: proofs 3
  and 4 are the two new ones, and every existing suite re-ran unchanged. Proof
  5's header half (no create affordance, title read through its tag, machine
  administration absent) is in that count.
- **Proof 5's placement half — `NOT_RUN`.** Its bottom-quadrant and
  non-occlusion assertions live in the two-host journey, and `scripts/test
  product` hard-refuses on anything but Darwin ("the physical two-host outage
  journey must run on the MacBook") and refuses inherited tmux state. It is
  MacBook-owned; this is structural, not a failure, and forcing it from here
  would be a lab-only hack.
- **Hands-on glance — `NOT_RUN`, and never pass without it:** the mark reads as
  `+` at arm's length; lit and cold are separable peripherally; the seal is
  instantly distinguishable from a struck seal; the last grid row is never
  occluded when the grid is full; the seal is reachable one-handed. The debug
  `SealGalleryActivity` — already the named instrument for pairwise seal
  distinguishability (dwarf-seals.md) — now leads with the lit and cold seals,
  so the struck/unstruck and lit/cold comparisons are real side-by-sides rather
  than memory.
- Integration, live, tmux, and host gates are not required — those boundaries
  are byte-identical. Do not invoke them.

## Non-goals

Wire, gateway, tmux, controller-intent, or terminal work. The Forge sheet's
contents, copy, fret band, or submit button. The app icon (the launcher stays
the prow — the app is the ship). A labelled or extended control, tooltip, or
long-press menu. Animating the lit/cold flip, or any second ambient animation.
Generating the mark in `scripts/gen-ornament`. A dimension-token system, a
generic shape-indication library, or a third `AngularIndication` consumer.
Ornament on the control. Re-opening P2R's gesture, inventory intent, or grid
consolidation.
