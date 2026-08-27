# Design delta D8: the Hlíðskjálf mark

Status: implemented 2026-08-27; drift gate, the two `OrnamentTest` proofs,
and routine verification (27 gates) green; the instrumented suite ran on the
physical S22+ — 39 tests, the only failure the provisioning fixture that
requires MacBook-owned private staging input, and two `assumeTrue` skips that
need provisioned machines. The hands-on on-panel glance stays `NOT_RUN`: no
automated proof can judge whether the 18 dp weave reads comfortably to an eye.

[`architecture.md`](architecture.md) owns product behavior and acceptance —
including the product-language line that names Hlíðskjálf the grid motif
behind the literal label `Dwarves`; [`design-language.md`](design-language.md)
§8 owns the mark itself and wins on every value here;
[`ornament-pipeline.md`](ornament-pipeline.md) owns the build-time pipeline
this delta corrects; [`roadmap.md`](roadmap.md) owns delivery order. This
document owns the mark's legibility contract and its placement. Testing
standard is [`rules/testing.md`](rules/testing.md).

Numbered D8 because the forge seal claims D5, destructive chrome claims D6,
and the launcher mark claims D7; those three are specified on their own
branches and each brings its own document when it lands, so this one does not
link to them. **Sequencing: this lands first.** D7 states the dependency from
its side — both edit `scripts/gen-ornament`, and this delta owns the valknut
entirely while D7 owns the icon resources and adopts this delta's proof form.
D5 owns `Theme.kt`'s Forge control and the `drawOrnamentBand` collapse; this
delta's only `Theme.kt` additions are `ValknutStrokeRatio`, the `drawValknut`
signature, and `HlidskjalfMark`.

## Outcome and final state

The Hlíðskjálf mark appears on the surface it names. It leads the `Dwarves`
title on the dashboard, leads every affordance that returns to the dashboard,
and still marks the genuinely empty hall — and at all three sizes its weave is
visible, which it was not at any size the app previously drew it.

## The defects this closes

1. **The mark for the dwarves was visible only in their absence.** `drawValknut`
   had exactly one call site, `EmptyState(ornament = true)`, which renders only
   when the inventory is genuinely empty. The screen whose motif is Hlíðskjálf
   carried no mark whenever it had anything on it.

2. **The weave did not resolve at the size it shipped.** The generator cut each
   crossing at `gap = 0.10` of the crossed edge, and `drawValknut` drew the
   result with a fixed `2.dp` stroke. Measured on the checked-in geometry, the
   narrowest break was `0.0555` of the unit box — `2.66 dp` at the empty
   state's 48 dp against a `2 dp` stroke, so `0.34 dp` of clear ground per
   crossing, roughly one physical pixel on the S22+. The shortest surviving
   strand was `0.0286` of the unit box: `1.37 dp` long under a `2 dp` stroke,
   wider than it was long, so it rendered as a speck. The six crossings closed
   and the mark read as a solid triangular clot. The hands-on ornament glance
   that would have caught this is still `NOT_RUN` (D4).

3. **`design-language.md` §8 stated a floor that was wrong.** "Render at 24dp+
   with 2dp stroke" is not achievable by that geometry at that stroke; the real
   floor was nearer 64 dp. §8 now states the invariant instead of a size.

## Closed decisions

- **The stroke scales with the mark; it is never a fixed dp.** A fixed stroke
  is 4% of a 48 dp mark and 11% of an 18 dp one, so shrinking the mark fattens
  the strand until it swallows the baked breaks. Legibility must be a property
  of the geometry against its own stroke, so that it holds at every rendered
  size or at none. `ValknutStrokeRatio = 0.055`.
- **`_VALKNUT_GAP = 0.36`, and the value is derived, not chosen.** Below 0.36 a
  crossing whose two cuts nearly meet leaves a stub of surviving edge between
  them shorter than the stroke is wide. 0.36 is the first width that consumes
  every such stub: the segment count drops from 15 to 12, the shortest strand
  rises from `0.005` to `0.0836` of the unit box, and the narrowest break rises
  to `0.1997`. Widening further only eats the triangles.
- **One mark, one drawing path, three sizes.** 24 dp Gold on the dashboard title
  lockup, 18 dp inheriting content colour on the two return affordances, 48 dp
  Muted-at-40% on the empty state. `HlidskjalfMark` is the single composable;
  no site draws the valknut directly.
- **The detach control gets no mark.** `Detach · session keeps running` names
  what the action does to the session, not where it goes. A mark there would
  assert the button means "go to Dwarves" when it means "detach", which is the
  visibly-distinct-actions guarantee architecture owns.
- **The mark stays in-app.** The launcher identity is D7's; §8's "the app
  icon's core" phrasing is not this delta's to spend.

## Capability contract

### Generator (`scripts/gen-ornament`)

`_VALKNUT_GAP = 0.36` becomes a named module constant carrying the derivation
above; `_valknut(gap: float = _VALKNUT_GAP)`. Nothing else in the pipeline
changes, and `scripts/check-ornament` gates the regenerated `Ornament.kt` for
drift and determinism exactly as before.

### Drawing (`Theme.kt`)

```text
internal const val ValknutStrokeRatio = 0.055f
internal fun DrawScope.drawValknut(color: Color)                    // was (color, strokeWidth: Dp = 2.dp)
internal fun HlidskjalfMark(color: Color, markSize: Dp, tag: String, modifier: Modifier = Modifier)
```

`drawValknut` reads `size.minDimension * ValknutStrokeRatio`; the `Dp` stroke
parameter is hard-cut, and no call site passed it. `HlidskjalfMark` is the only
renderer: it sizes the canvas, clears its own subtree semantics, and carries
`tag` solely so the proofs can assert its silence.

### Placement

| Surface | Size | Colour | Tag |
| --- | ---: | --- | --- |
| `DashboardTopBar`, leading the `Dwarves` title | 24 dp | Gold | `dashboard-mark` |
| `TerminalScreen` `ReconnectPanel`, leading `Back to Dwarves` | 18 dp | `LocalContentColor` | `terminal-dwarves-mark` |
| `MainActivity` bearer repair, leading `Back to Dwarves` | 18 dp | `LocalContentColor` | `bearer-repair-dwarves-mark` |
| `EmptyState(ornament = true)` | 48 dp | Muted 40% | `EmptyStateOrnament` |

Both button marks take `LocalContentColor.current` so they dim with their own
label when the button is disabled, exactly as a leading icon would; the
bearer-repair button is disabled while a repair is pending. Both are separated
from their label by `ButtonDefaults.IconSpacing`.

## Goals and rules

- **Ornament stays silent and subordinate.** The mark carries no content
  description and no text, adds no click action, and every screen stays
  complete with it deleted. The literal labels beside it carry all meaning.
- **No new dependency.** The app still ships no icon artifact and makes no
  `Icon()` call; the mark is drawn geometry from the checked-in constants.
- **No ornament near errors or destructive surfaces.** The kill dialog, the
  notice surface, and every error string stay bare.
- **Topology stays build-time.** The phone draws frozen segments; no weave
  logic runs on-device.

## Hard cut

`drawValknut`'s `strokeWidth: Dp` parameter is removed, not defaulted.
`EmptyState`'s inline `Canvas` is removed in favour of `HlidskjalfMark`, and
`DashboardScreen.kt` drops the two semantics imports that call site alone used.
No second mark, no compact variant, and no second drawing path is introduced.

## Work split

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Gap derivation + regeneration | Root integrator | `scripts/gen-ornament` (`_VALKNUT_GAP`), `Ornament.kt` (generated) | Drift gate; both `OrnamentTest` proofs red first |
| Stroke and the one renderer | Compose UI builder | `Theme.kt` (`ValknutStrokeRatio`, `drawValknut`, `HlidskjalfMark`, `BackToDwarvesContent`) | `OrnamentTest`; mark-silence instrumentation |
| Placement | Compose UI builder (same slice) | `DashboardScreen.kt`, `TerminalScreen.kt`, `MainActivity.kt` | Instrumented: silence, leading position, disabled dim |
| Docs | Root integrator | `design-language.md` §8 §13 §17, `ornament-pipeline.md`, `roadmap.md` | Review only |
| Verification | Read-only verifier | none | Review only |

D7, the launcher mark, sequences behind this delta: both
edit `scripts/gen-ornament`, and this one moves frozen geometry that the
drift gate byte-compares, so concurrent edits would collide in the generated
file rather than in the generator.

## Red / green / refactor

Red (each observed failing first, at the shipped `gap = 0.10`):

1. `OrnamentTest`: every strand is at least as long as its own stroke is wide.
   Observed failure: `shortest strand is 0.028552026 of the unit box against a
   stroke of 0.055`.
2. `OrnamentTest`: every baked break is wider than the strand crossing it.
   Observed failure: `narrowest break is 0.05546999 of the unit box against a
   stroke of 0.055`.
3. `DashboardChromeInstrumentedTest`: the top-bar mark leads the literal title
   on one row and offers neither content description nor text.
4. Drift: `scripts/check-ornament` fails against the stale `Ornament.kt` until
   it is regenerated.
5. `OrnamentTest`: the interior-break count is pinned. Of the six crossings,
   `gap = 0.36` bakes three interior breaks and cuts three out to a vertex.
   The count is *not* monotonic in the gap — widening consumes interior breaks
   — so without this the width bound in (2) alone would pass a future gap that
   left almost nothing woven. Observed failure at `gap = 0.18`, which weaves
   five.
6. `HlidskjalfMarkInstrumentedTest`: inside a button the mark stays silent, the
   button still announces exactly `Back to Dwarves`, and the mark dims with the
   button rather than staying at full strength (a pixel proof — the reason the
   mark takes `LocalContentColor`).
7. `DashboardChromeInstrumentedTest`: mark and title still share one 64 dp row
   in that reading order after the mark took 32 dp out of it.

Both `OrnamentTest` proofs are pure JVM geometry over the checked-in `Valknut`
constant and `ValknutStrokeRatio`, so they hold the invariant rather than a
rendered size, and they fail for any future gap or stroke change that closes
the weave again.

Green: the derived gap and the proportional stroke, nothing more.

Refactor: the four render sites share `HlidskjalfMark`, and the two that are a
mark leading `Back to Dwarves` inside a button share `BackToDwarvesContent`, so
the label and its mark cannot drift apart between screens. `drawValknut` keeps
its own drawing path rather than joining `drawOrnamentBand`, because the mark
does not tile (ornament-pipeline.md's refactor note).

## Acceptance and gates

Routine verification (`scripts/test verify` — static including the ornament
drift gate, build, unit) plus the instrumented suite, both green on the
physical S22+.

The row-crowding half of the old hands-on note is now automated rather than
deferred: `theTopBarKeepsTheMarkLeadingTheTitleOnOneRow` composes
`DashboardTopBar` directly and proves mark and title share one 64 dp row in
that reading order. It does not sit behind the `assumeTrue` for provisioned
machines that gates the equivalent assertions in
`MultiMachineUiInstrumentedTest`, so it runs on every device.

What stays hands-on and is not claimed here: whether the weave reads as three
interlocked triangles to an eye at 18 dp on the panel. That is a judgement, not
a measurement.

## Non-goals

A second or compact mark variant; the launcher icon (D7); any mark on the
detach control, on cards, chips, or the key deck; framed fret borders;
animated ornament; a Dvergatal catalogue view; any gateway, protocol, tmux, or
input-semantics change.
