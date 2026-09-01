# Design delta D7: the launcher mark

Status: implemented 2026-08-27. Proofs 1, 2 and 3 were each observed red,
then green; `scripts/test verify` — static, build and unit — is green, and the
ornament drift gate inside it is green over five generated files. The
separately approved S22+ platform gate is green — `OK (34 tests)`, proof 4
among them, on the physical device. That run was on the D7 branch before it
merged D6 and D8, whose own instrumented tests were not in it. Verification is
green again on the merged tree, and the current complete 54-test release-bound
S22+ platform gate is green. The hands-on glance is `NOT_RUN`: it is a
human-eye check and no gate substitutes for it.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`design-language.md`](design-language.md) owns visual identity and wins on
every value here; [`roadmap.md`](roadmap.md) owns delivery order;
[`ornament-pipeline.md`](ornament-pipeline.md) owns the build-time pipeline
this delta extends. This document owns the launcher identity. Testing standard
is [`rules/testing.md`](rules/testing.md).

Numbered D7 because [`forge-seal.md`](forge-seal.md) claims D5 and
[`destructive-chrome.md`](destructive-chrome.md) claims D6. Both landed
first.

**Sequencing.** D7 was written against a tree where the Hlíðskjálf-mark work
was still in flight, and D8 landed first; D7 merges onto D8's generator rather
than the reverse. The two share `scripts/gen-ornament` and nothing else in it:
D8 owns `_VALKNUT_GAP`, `_valknut` and `Ornament.kt`, D7 owns the icon
resources, and the merge keeps both untouched — the valknut constant here is
D8's `0.36`. D5 owns `Theme.kt`, `AngularIndication.kt` and the Forge
seal. No path overlaps.

## Outcome and final state

The launcher stops being a sloop. `scripts/gen-ornament` emits the whole
adaptive-icon resource set — background, foreground, monochrome, and the
`<adaptive-icon>` descriptor — from authored polygon constants, drift-gated as
one unit.

The generator gains, for the icon, one contract: **geometry that cannot be
seen at the size it renders is not emitted.** The generator raises rather than
writing it.

## Scope

The launcher identity — mark geometry, the three layers, the descriptor, and
the manifest wiring — plus the VectorDrawable emitter and legibility check the
four new files force. Nothing at runtime reads the mark, so no Kotlin
production source changes.

## The defects this closes

Measured against the checked-in tree, not asserted:

1. **The icon is a Bermuda sloop** — two triangular fore-and-aft sails on one
   mast. No Norse vessel is rigged this way.
2. **It has no provenance row.** §1.6 requires one and says a motif without
   provenance does not ship. The §2 ledger has no ship.
3. **Bone is its largest mass**, spent on the hull. §1.1 rations the palette;
   the 17:1 primary-*text* token is not a fill.
4. **It is a resampled curve.** `_SOURCE_PROW_PATHS` holds the retired
   clip-art's Béziers; `_parse_path` refacets each into three segments. §1.3 is
   satisfied in letter only — the hull's lower edge is a three-facet arc and
   reads as a wobble, not a cut.
5. **The themed layer collapses.** `ic_launcher.xml` aliases `monochrome` to
   the foreground, whose internal structure is background-coloured negative
   space 3.55 units wide on a 108-unit canvas. Under one flat tint it is a
   blob with slivers.

## Closed decisions

1. **The mark is Skíðblaðnir, not a generic longship.** *Gylfaginning* 43:
   built by the sons of Ívaldi, takes a fair wind the moment its sail is
   raised, folds into a pouch. That is the product. A dragon-prowed drakkar is
   the same tourist-shop register §8 already bans, one rung up.
2. **The stems are cut, not curved.** The defining longship feature is the
   curve §1.3 forbids. Cutting it is what makes the mark a dwarf-made object
   rather than a picture of a boat.
3. **Both stems are integral to the hull polygon**, not strokes added to it.
   Two separate hooks above a hull read as a face; killed empirically.
4. **The monochrome layer is its own drawable**, and its structure is carried
   by the geometry itself — never by background-coloured gaps. That is defect
   5's actual fix. It is carried by real absence: the gores are three disjoint
   solids with the slit between them — the same three in both layers — and the
   shields are dips cut into the hull's own sheer. Nothing in the mark is a
   hole, because a hole only cuts where there is already mass, and the shields
   straddle the sheer.
5. **`mipmap-anydpi-v26/ic_launcher.xml` becomes generated.** D5 declines to
   generate its mark because a drift gate would guard nothing. Here the gate
   guards the exact hand-edit that shipped defect 5, so the alias cannot
   return.
6. **No new token and no new shape.** Gold + Bronze are one §5 family; Garnet
   is already a §5 fill-only token; the lozenge is already §6's badge atom;
   the sail's foot uses §6's 45° cut. The §5 hue budget is untouched.
7. **The mark gets no Kotlin constant.** Nothing on device reads it. The
   geometry stays generator-internal, as the prow's did.
8. **`android:roundIcon` is deleted.** At `minSdk = 36` it is a duplicate
   reference to the same adaptive resource. No legacy density mipmaps exist or
   are added.
9. **The icon's floor is absolute, not a ratio.** A stroke-ratio floor is
   right for the valknut, which is drawn at arbitrary sizes against a stroke
   that scales with it. The icon has no stroke and a known size range —
   launchers pick from it — so its floor is stated in dp at the smallest size
   it must survive.
10. **The slit is derived and the notch is plain.** `_SLIT_W` is computed
    from the seam, the sail's left edge and the foot cut rather than stated:
    it is the one width at which each outer gore closes as a quadrilateral on
    the sail's own 45° cut. A narrower slit leaves a needle of gore running
    past the cut — 0.12 units at the 3.2 first shipped — and a wider one folds
    the gore into a bowtie; deriving the width removes both failures without a
    guard. The sheer notch takes no mouth constant either. It is the triangle
    `(cx−3.5, 70) (cx, 75) (cx+3.5, 70)`, widest where it meets the sheer,
    because a notch that widens below the sheer undercuts the deck and leaves
    an overhanging lip at each of six shoulders.

## Goals and rules

- **Honest by construction.** The generator raises rather than emitting
  geometry below the legibility floor or outside the mask envelope. Enforced,
  not documented.
- **Measure a feature against what it is drawn on.** The icon's reference is
  the canvas, not a stroke, because the icon has no stroke.
- **Authored, not traced.** Straight segments only, written as constants. No
  curve interpreter survives.
- Ornament stays silent and subordinate; the icon carries no semantics.
- Reuse before adding: one VectorDrawable emitter, no new dependency, no new
  gate.

## Capability contract

### Legibility invariant

A feature is a gap between disjoint masses, the width of a separation, or the
width of a mass. Every feature must hold one 2dp stroke of daylight at the
smallest launcher size the mark must survive.

Two checked features are mass widths — the outer gore, and the web of deck
between two sheer notches; every other one is a gap. Acute apexes are exempt:
the two hull stems and the vane taper to points by design, and a tapering
point antialiases to a rounded end rather than clotting, which is the failure
the floor exists to prevent.

```text
STROKE_FLOOR_DP = 2.0     # one 2dp stroke / one 2dp gap
LAUNCHER_MIN_DP = 48.0    # smallest launcher size the mark must survive
ICON_ENVELOPE   = 34.0    # units from centre
```

For an icon shown at `S` dp the 108-unit canvas maps to `1.5 · S` dp, so at
48dp **one unit is 0.667dp** and the floor is **3.0 units**.

The **monochrome layer is the binding case**: it is one flat tint, so every
separation there must be geometric. Both layers carry the same features but
one — the shield row, which colour separates in the foreground and the sheer
notches carry in the monochrome.

The foreground's inter-shield gap is `(54 − 43.5) − 2 · 4.4 = 1.70` units =
`1.13dp`, knowingly below the geometric floor and unchecked. It has no
monochrome counterpart, because the shields exist only in the foreground, and
its separation is carried by colour rather than geometry: at `cy = 70.5`,
where the lozenges are widest and the gap is narrowest, that gap is the hull's
own Gold `#D6A85F` under Garnet `#830E0D` — a 4.73:1 pair.

### Geometry (108-unit canvas, centre `(54,54)`)

| Element | Constants |
| --- | --- |
| Yard | `y = 31.5`, `x = 33.5…74.5`, stroke `5` |
| Mast | `x = 54`, `y = 23…67.5`, stroke `5` — the cap above `y = 23` is the masthead the vane roots on; the cap below lands on the sheer, so yard, sail and hull hang off one spine instead of stacking as three unattached masses |
| Vane | `(54,23) (68,24.5) (54,26)` — rooted *on* the mast, one mass with it, and clear of the yard |
| Sail | `x = 36…72`, `y = 37…62`, foot corners cut `8` at 45° |
| Gores | seams at `t = 0.27, 0.73`; the same three solids in both layers, filled `Bronze / Gold / Bronze` in the foreground and one `Gold` in the monochrome |
| Gore slit | `3.44`, derived as `2 · (seam − sail x0 − foot cut)`: the one width at which each outer gore's seam edge lands exactly on the sail's own 45° foot cut |
| Hull | `TIP (21,53)` `ITIP (33,65)` `SHEER (37,70)` `KEEL (34,79)`, mirrored about `x = 54` |
| Shields | `cx = 43.5, 54, 64.5`, `cy = 70.5`, `ry = 4.5`, `rx = 4.4`, fill `Garnet` — foreground only |
| Sheer notches | the monochrome's shield row: one triangle `(cx−3.5, 70) (cx, 75) (cx+3.5, 70)` cut into the hull's own sheer, widest at its mouth, because widening it inwards would undercut the deck |
| Background | `Ink`, plus one `DeepSurface` facet plane on the `(0,0)–(108,0)–(0,108)` diagonal |

**Envelope.** Every emitted point sits at `r ≤ 34.0` of centre. Android's
circular mask clips at `r = 36`; the never-clipped-by-any-mask guarantee is
`r = 33`. Actual maximum is `33.9706`, at `(31,29)` and `(77,29)` — the
yard's outer top corners.

**Clearances**, all computed from the constants above — the checked values,
each measured where the quantity it names is minimised:

| Clearance | Units | dp at 48dp |
| --- | ---: | ---: |
| Yard bottom → sail head | 3.00 | 2.00 |
| Vane → yard | 3.27 | 2.18 |
| Gore-slit width | 3.44 | 2.29 |
| Outer gore width | 8.00 | 5.33 |
| Sail foot → hull sheer | 8.00 | 5.33 |
| Sail cut → stem | 9.90 | 6.60 |
| Shield-notch web | 3.50 | 2.33 |
| Notch → sheer end | 3.00 | 2.00 |

Two of those are not the obvious reach. The vane is measured where it leaves
the mast's flank at `x = 56.5`, the narrowest point of the wedge between vane
and yard. The sail's foot cut and the stem's inner edge are both 45°, hence
parallel, so their separation is the constant `14/√2 = 9.899` — not a
horizontal reach at one height. Reaching horizontally from the luff at the
sail's foot height, as an earlier `6.00` entry did, described no gap in the
mark at all: at that height the sail's left boundary is the cut corner at
`x = 44`, not the luff at `x = 36`.

### Emitted files

`build_outputs()` gains four paths. `scripts/check-ornament` already iterates
that dict, so it gates them with no change, and `scripts/test static` already
runs it.

```text
android/app/src/main/res/drawable/ic_launcher_background.xml    (now generated)
android/app/src/main/res/drawable/ic_launcher_foreground.xml    (regenerated)
android/app/src/main/res/drawable/ic_launcher_monochrome.xml    (new)
android/app/src/main/res/mipmap-anydpi-v26/ic_launcher.xml      (now generated)
```

`Ornament.kt` is untouched by D7.

## API design

```python
# scripts/gen-ornament

STROKE_FLOOR_DP = 2.0
LAUNCHER_MIN_DP = 48.0
ICON_ENVELOPE   = 34.0

# Derived, never stated: the slit that lands each outer gore's seam edge on
# the sail's own foot cut. The sheer notch has no mouth constant to match —
# it is a plain triangle _SHIELD_RX_MONO wide on each side of cx.
_SLIT_W = 2 * (_SEAM_X[0] - _SAIL_X0 - _SAIL_FOOT_CUT)

def _require_legible(features: dict[str, float]) -> None:
    """Raise ValueError naming the first clearance below the floor."""

def _require_within_envelope(polygons) -> None:
    """Raise ValueError if any point exceeds ICON_ENVELOPE from centre."""

def _mark_layers() -> tuple[list, list, list]:
    """(background, foreground, monochrome) as (polygons, colour) lists."""

def _sail_gores() -> list[list[tuple[float, float]]]:
    """The three gore solids, one geometry serving both layers."""

def _sheer_notch(cx: float) -> list[tuple[float, float]]:
    """One monochrome shield, as a triangle cut into the sheer."""

def _vector(paths: list[tuple[str, str]], comment: str) -> bytes:
    """One VectorDrawable emitter, replacing the inline XML string in
    build_outputs(). Three callers instead of one; the descriptor is a plain
    string beside them."""

def _path_data(polygons) -> str:
    """M/L/Z path data, one closed ring per polygon."""

def build_outputs() -> dict[str, bytes]:
    """Unchanged signature. Checks both invariants before returning."""
```

`_fret_cell`, `_valknut`, `_split_with_gaps` and `_kotlin_segments` are reused
unchanged. Stdlib only, as its siblings are.

## Structure and composition

```text
scripts/gen-ornament
`- build_outputs()         -> Ornament.kt + the four icon resources
   |- _fret_cell()         -> Ornament.kt                    (unchanged)
   |- _valknut()           -> Ornament.kt                    (unchanged; the Hlíðskjálf work owns it)
   |- _mark_layers()       -> (background, foreground, monochrome)
   |- _require_within_envelope(...)   both invariants are checked by
   |- _require_legible({...})          build_outputs, not by _mark_layers
   |- _vector(_path_data(...))  x3
   `- the <adaptive-icon> descriptor  (a plain string, not a VectorDrawable)

scripts/check-ornament     (unchanged — iterates build_outputs())
scripts/test static        (unchanged — already runs ornament-static)
```

No HTTP, WSS, tmux, gateway, DTO, persistence, credential, controller-intent,
Compose, or terminal change.

## Content design (designer-owned)

The mark designer owns the geometry table above and delivers this rubric with
it. **Good** is:

1. At 48dp among neighbours it reads as a ship, not a container. The container
   gestalt is the known failure mode; the cut sail foot is what breaks it.
2. The garnet row is what locates the icon in a full grid at a glance.
3. The vane gives the mark a heading; nothing else in it is asymmetric.
4. Under one flat tint, gores and shields stay separable — the mark degrades,
   it does not collapse.
5. Nothing reads as a face, a crown, or a horned silhouette. All three were
   reached and rejected in exploration; §15 forbids the last.

4 is machine-checked twice below: by the legibility invariant in the
generator, and by proof 4 on the built artifact. 1–3 and 5 belong to the
hands-on pass.

## Hard cut and cleanup

- `_SOURCE_PROW_PATHS`, `_parse_path`, `_prow_polygons`, `_BEZIER_FACETS` and
  `_ICON_SAFE_RADIUS` are deleted. The Bézier interpreter and the refaceting
  step exist only to digest the retired clip-art; the generator gets smaller.
- The inline foreground-XML string in `build_outputs()` is replaced by
  `_vector`; the hand-written `ic_launcher_background.xml` and
  `ic_launcher.xml` become generated output at the same paths.
- `monochrome` stops aliasing the foreground; the comment in `ic_launcher.xml`
  explaining the alias goes with it.
- `android:roundIcon` is removed from `AndroidManifest.xml`.
- No flag, no dual icon, no legacy drawable, no density fallback, no
  backward-compatible path.

## Reuse and consolidation

- **One emitter, not four strings.** Three VectorDrawables force `_vector` and
  `_path_data` out of the inline string that served one; the `<adaptive-icon>`
  descriptor is a plain string beside them.
- **Extirpate.** Five generator symbols (above) delete outright.
- **Do not touch** `drawOrnamentBand` (D5's), `Ornament.kt`, `Theme.kt`,
  `DashboardScreen.kt`, or anything the Hlíðskjálf-mark work owns.

## Work split

Sequential at named seams; paths do not overlap. A builder observes its red
before production edits. The verifier writes no file.

| Order | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| 1 | Mark designer | `docs/launcher-mark.md` (geometry table + rubric) | Frozen values and the "good" rubric; no runtime proof |
| 2 | Root integrator | `docs/design-language.md`, `docs/ornament-pipeline.md`, `docs/roadmap.md` | Contract amendments; no runtime proof |
| 3 | Generator builder | `scripts/gen-ornament` + the four generated paths | Proofs 1–3 |
| 4 | Android shell builder | `AndroidManifest.xml`, `LauncherIconInstrumentedTest.kt` (new) | Proof 4 |
| 5 | Read-only verifier | none | Diff, rule, and gate review only |

Slice 2 amends: design language §2 (three ledger rows — Gotland picture-stone
ship, Gokstad shield rack, Viking masthead vane), §8 (the launcher mark), §17
(the D7 row); ornament-pipeline (the launcher-mark item in the pipeline list
and "App icon"); roadmap (D7 entry and status row).

## Red / green / refactor

One proof per invariant, split at the ownership boundary. Drift and
determinism already exist in `check-ornament` and are re-run unchanged, not
re-authored.

**Generator (static gate, `ornament-static`)**

1. **Legibility.** Move the seams to `_SEAM_T = (0.25, 0.75)`, which narrows
   the derived slit to 2.0 units — `_SLIT_W` is not settable directly.
   `scripts/check-ornament` fails with `ornament check failed: gore-slit width
   is 2.00 units = 1.33dp at 48dp, below the 2dp floor`. Restoring
   `(0.27, 0.73)` returns the slit to 3.44 and it passes.
2. **Envelope.** Raise the yard to `_YARD_Y = 29.5`. The check fails with
   `ornament check failed: (31,27) sits at r=35.468, outside the 34-unit mask
   envelope`.
3. **The vane sliver.** Restore the vane as first frozen,
   `[(54,23), (68,25.75), (54,28.5)]`. The check fails with `ornament check
   failed: vane -> yard is 0.99 units = 0.66dp at 48dp, below the 2dp floor`.
   This is the proof that carries the most weight: geometry that shipped
   before now fails the gate, so the invariant constrains the mark rather than
   restating whatever was drawn.

**Android shell (platform gate, `LauncherIconInstrumentedTest.kt`)**

4. The installed application icon is an `AdaptiveIconDrawable` and its
   `monochrome` layer is non-null. Both layers render to 108px bitmaps — one
   pixel per canvas unit, half alpha and up counting as covered — and are
   compared by **coverage, not by pixel**: of the foreground's covered pixels
   the monochrome must cut some away (`cut > 0`) and keep most of them
   (`cut * 4 < kept`). Two layers that differ only in fill colour are never
   pixel-identical, so a pixel comparison would pass vacuously; coverage is
   what separates a mark that degrades under one tint from one that collapses.
   This is defect 5 asserted on the built artifact, which the drift gate
   cannot reach.

**Deliberately unproven (80/20).** No test asserts the mark reads as a ship —
that is the hands-on pass. No per-path pixel test and no test of the
background facet; both would assert styling, which rules/cleanliness forbids.
No unit test of `_vector`: every run of the generator that clears both
invariants exercises it, and the drift gate byte-compares its output.

Proof 4 requires the real Android runtime. Obtain explicit current-turn
platform/ADB approval before running it; without approval it is `NOT_RUN`, the
builder has not observed red, and the slice cannot complete.

**Green:** implement only enough to satisfy the proofs and the frozen table.
Routine `scripts/test verify` stays green and device-free.

**Refactor:** collapse the four XML emissions onto one `_vector`; delete the
five dead generator symbols and any import they orphan; add no abstraction
without a second caller.

## Acceptance and 80/20 gates

- Routine verification green, including `ornament-static` over the enlarged
  generated set.
- One separately approved S22+ platform pass green, including proof 4 and the
  existing suites re-run unchanged. Done: `OK (34 tests)`, 33 existing plus
  proof 4, on the D7 branch before it merged D6 and D8. The gate refuses to run
  against an install that does not carry the pinned signing identity, so
  replacing a foreign-signed build first is part of the run and costs the app's
  pairing state.
- One hands-on glance: the icon reads as a ship at arm's length on the home
  screen and in the app switcher, and the themed-icon variant is still a ship.
  Without the device this is `NOT_RUN`, never pass.
- Integration, live, product, provision, tmux, and host gates are not
  required — those boundaries are byte-identical. Do not invoke them.

## Non-goals

The valknut, `_VALKNUT_GAP`, `ValknutStrokeRatio`, `HlidskjalfMark`, or
`OrnamentTest` — all owned by the Hlíðskjálf-mark work, none of them in this
branch. Any in-app use of the launcher mark: wordmark, splash, About view,
empty state. The Dwarves grid glyph. The Forge seal, `drawOrnamentBand`,
`AngularIndication`, or anything else D5 owns. A `material-icons` dependency — the app ships zero icon
artifacts and adds none. Legacy density mipmaps or a non-adaptive fallback
(`minSdk = 36`). Animating the icon. Any Kotlin production source change.
