# Design delta D4: ornament

Status: implemented 2026-08-26; routine verification (including the
ornament drift gate) green; the 33-test instrumented suite green on the
physical S22+ (devbox debug-signed run); the hands-on ornament/icon glance
stays `NOT_RUN`. The interlace band was removed with the pairing screen in
the multi-machine cut, and the prow icon this delta shipped was replaced by
the launcher mark ([launcher-mark.md](launcher-mark.md), D7), which owns the
launcher identity from here on.
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
  deterministically emits every checked-in piece of ornament geometry —
  `Ornament.kt` path-data constants for the two drawn marks, and the launcher
  icon's own resources:
  1. **Fret cell**: one chevron/key meander unit for the Forge title band
     (monoline, 45°/90° turns only, band height ≤ 16dp, sized so any target
     width tiles a whole number of units — bands are straight strips in v0,
     so no corner mitering path exists yet; a future framed border re-opens
     the corner rule in the design language, not here).
  2. **Valknut**: the tricursal form — three interlocked triangles,
     Borromean topology, straight lines only. The break width cut at each
     crossing is `_VALKNUT_GAP`, which carries its own derivation;
     [hlidskjalf-mark.md](hlidskjalf-mark.md) owns why that value and not
     another.
  3. **Launcher mark**: Skíðblaðnir under sail — cut stems, a square sail in
     three gores, a shield row at the sheer, a masthead vane — authored as
     straight-edged polygon constants and emitted as the whole adaptive-icon
     set: three VectorDrawables (background, foreground, monochrome) and the
     `<adaptive-icon>` descriptor that names them. The generator raises
     rather than emitting a feature below the legibility floor or a point
     outside the mask envelope; the geometry stays generator-internal with no
     Kotlin constant, because nothing at runtime reads it (no dead code
     ships). [`launcher-mark.md`](launcher-mark.md) owns its constants and
     proofs.
- The generated files are checked in. `scripts/check-ornament` (Python 3,
  stdlib only, like its sibling checks) imports `build_outputs()`, calls it
  twice to prove determinism, and byte-compares the result against every
  checked-in path — drift between generator and checked-in source fails the
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
- **The valknut**: [hlidskjalf-mark.md](hlidskjalf-mark.md) owns every surface
  the mark renders on and its legibility contract. This document owns only its
  generation. It stays decorative and unlabeled wherever it appears — the
  literal text beside it carries the semantics — and it never renders beside a
  degraded or repair state.
- **App icon**: the launcher identity is the launcher mark — the app is named
  for the vessel; the valknut marks Hlíðskjálf inside the app, never the
  launcher. The adaptive icon is four generated resources: an Ink background
  cut once on the canvas diagonal by a DeepSurface facet plane, the colored
  foreground, a monochrome drawable of its own cut for one flat tint, and the
  `<adaptive-icon>` descriptor the manifest's icon reference resolves to.
  Launcher label stays `Skíðblaðnir`.

## Hard cut

- The flat non-adaptive `res/drawable/ic_launcher.xml` is deleted with the
  manifest repoint; no orphaned or dual icon resource remains.
- The generator owns `Ornament.kt` and every icon resource entirely; hand
  edits are forbidden and caught by the drift gate. Nothing else is removed.

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
