# Design delta D4: ornament

Status: implemented 2026-08-26; routine verification (including the
ornament drift gate) and the release icon build green; the S22+ platform gate
green; the hands-on ornament/icon glance stays `NOT_RUN`.
Depends on D2 ([chrome-tokens.md](chrome-tokens.md)) for tokens and shapes.

[`design-language.md`](design-language.md) §7 (ornament families and
construction), §8 (iconography), and §13 (component register) own the
design; this document owns the build pipeline and implementation boundary.

## Outcome

The app gains its three pieces of authored ornament — the Forge fret band,
the valknut empty-state mark, and the adaptive app icon — all generated or
authored at build time, checked in as source, drift-gated, and
semantics-invisible. No ornament is computed on the
phone at runtime beyond drawing pre-built paths.

## Goals and rules

- **Build-time only.** Knot/fret topology is never computed on-device
  (design language §7). The phone draws checked-in path constants; repeating
  bands tile one cached cell.
- **Two families, split by scale, never blended**: the angular fret family
  for chrome bands; woven interlace only at large scale — in v0 that is the
  valknut empty-state mark alone (the multi-machine cutover deleted the
  pairing screen, the interlace band's only chrome surface). No zoomorphic
  elements anywhere.
- **Ornament is silent and subordinate**: decoration carries no semantics,
  never intercepts input, sits at ≤ 40% opacity in Muted or Gold, and every
  screen remains complete with ornament deleted (geometry-first rule).
- **No ornament near errors or destructive surfaces** — the kill dialog and
  every error text stay bare.
- Deterministic generation: the generator takes no clock, no randomness
  beyond its fixed seed constants; same inputs, same bytes, forever.

## Capability contract

### Pipeline

- `scripts/gen-ornament` (Python 3, stdlib only, no third-party imports)
  deterministically emits `Ornament.kt` — path-data constants for:
  1. **Fret cell**: one chevron/key meander unit for the Forge title band
     (monoline, 45°/90° turns only, band height ≤ 16dp, sized so any target
     width tiles a whole number of units — bands are straight strips in v0,
     so no corner mitering path exists yet; a future framed border re-opens
     the corner rule in the design language, not here).
  2. **Valknut**: the tricursal form — three interlocked triangles,
     Borromean topology, straight lines only.
  3. **Ship prow**: the launcher mark — the existing gold prow glyph
     refaceted into the Niðavellir grammar (straight segments only) and
     emitted solely as the adaptive-icon foreground/monochrome drawable; the
     geometry stays generator-internal with no Kotlin constant, because
     nothing at runtime reads it (no dead code ships).
- The generated file is checked in. `scripts/check-ornament` (Python 3,
  stdlib only, like its sibling checks) regenerates to a temporary path and
  byte-compares — drift between generator and checked-in source fails the
  static gate. `scripts/test` gains exactly one line in its existing
  `static` case (`run ornament-static ./scripts/check-ornament`, matching the
  existing explicit-name convention); nothing else in `scripts/test`
  changes, and Python checks stay outside the `bash-syntax`/`shellcheck`
  lists by existing precedent.
- Algorithms are reimplemented from the published construction methods
  (Bain grid-and-dot / parity weave); no third-party knot code is vendored
  (the GPL and unlicensed generators named in design language §7 stay out).

### Surfaces

- **The Forge**: one fret band under the sheet title, Gold at ≤ 40% opacity,
  even unit count at every rendered width (unit-quantized drawing), tiled
  from a single cached cell (`drawWithCache` + repeating shader per design
  language §7). Present only on the Forge — chips, cards, and the key deck
  gain no ornament in this delta.
- **Empty grid state**: when the inventory is genuinely empty, the valknut
  renders centered in Muted with the existing literal empty-state text
  unchanged; the mark is decorative and unlabeled (the text carries the
  semantics).
- **App icon**: the launcher identity stays the ship prow — the app is named
  for the vessel; the valknut marks Hlíðskjálf inside the app, never the
  launcher. The current flat `res/drawable/ic_launcher.xml` (a gold prow on
  Ink, not adaptive) is replaced by an adaptive icon: foreground the redrawn
  angular prow, background Ink, monochrome layer the same prow;
  `android:icon`/`android:roundIcon` repoint to it. Launcher label stays
  `Skíðblaðnir`.

## Hard cut

- The flat non-adaptive `res/drawable/ic_launcher.xml` is deleted with the
  manifest repoint; no orphaned or dual icon resource remains.
- The generator owns `Ornament.kt` entirely; hand edits are forbidden and
  caught by the drift gate. Nothing else is removed.

## Work split

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Generator + gate | Root integrator | `scripts/gen-ornament` (new), `scripts/check-ornament` (new), `scripts/test` (static composition only) | Drift red below |
| Compose surfaces | Compose UI builder | `Ornament.kt` (generated), `DashboardScreen.kt` (empty state and Forge) | Compose proofs below |
| Icon | Compose UI builder (same slice) | `res/mipmap*/` (new), `res/drawable/ic_launcher.xml` (replaced), `AndroidManifest.xml` (icon refs only) | Build proof |
| Verification | Read-only verifier | none | Review only |

## Red / green / refactor

Red (each observed failing first):

1. Drift gate: mutate one byte of the checked-in `Ornament.kt`;
   `scripts/check-ornament` fails; regeneration restores green.
2. Determinism: two consecutive generator runs produce identical bytes.
3. Compose test: ornament nodes expose no semantics and are not clickable;
   the existing accessibility traversal proofs for Forge and grid still pass
   unchanged.
4. Compose test: the empty-state text remains the literal copy; the valknut
   adds no content description.
5. Build: the adaptive icon resolves in all densities (release build
   compiles with the new manifest refs).

Green: implement only enough to pass; routine `scripts/test verify`
(static now including `check-ornament`, build, unit) stays green.

Refactor: ensure every Forge width uses one tile-drawing helper rather than
parallel renderers; nothing else.

## Acceptance and gates

Routine verification; ornament rendering and the launcher icon are folded
into the next separately-approved platform pass and a hands-on glance
(bands read as carved texture at arm's length, not noise; icon reads on both
light and dark launcher backgrounds). No integration or live gate.

## Non-goals

Ornament on cards, chips, or the key deck; framed (four-sided) fret borders
and their corner mitering; zoomorphic or Borre ring-chain art; a Dvergatal
catalogue/About view and its Junicode epigraphs (future work, reopens
design-language §9 shipping note); animated ornament; runtime generation;
any third-party knot-generation dependency.
