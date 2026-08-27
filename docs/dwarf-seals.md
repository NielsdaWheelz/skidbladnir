# Design delta D3: dwarf seals

Status: implemented 2026-08-26; golden-vector, catalogue-distinctness, and
S22+ platform gates green; the hands-on 48dp gallery pass stays `NOT_RUN`.
Depends on D2 ([chrome-tokens.md](chrome-tokens.md)) for color and type
tokens. Builds on the landed automatic dwarf identity contract
([automatic-dwarf-identity.md](automatic-dwarf-identity.md)): `character` is
required on every card, the API carries no image or color fields, and Android
derives all presentation from `character.key`.

[`design-language.md`](design-language.md) §11 owns the seal's design; this
document freezes the generator algorithm and owns the implementation
boundary. The live reference implementation is the Niðavellir artifact page's
seal forge; the Kotlin port must be trait-for-trait equivalent to the frozen
formulas below (the formulas, not the JS, are normative).

## Outcome

Every session card renders a deterministic Niðavellir seal — octagonal
frame, mineral field, Younger-Futhark bind-rune maker's mark, angular beard
silhouette, Bone initial — as a pure function of `character.key`, replacing
the current circle-and-arc portrait. Same key, same seal, forever, across
process recreation, reinstall, and gateway restart.

## Goals and rules

- **Pure function, no state.** Trait derivation reads only the key string.
  No caching files, no persisted render, no randomness, no clock.
- **Determinism is contract.** The byte-slice formulas below are frozen; a
  change to any formula is a design event that re-rolls every user-visible
  identity, and requires reopening this document — never a silent tweak.
- **Angular only**: monoline rune strokes with butt caps and miter joins;
  straight segments and 45° facets; no curve anywhere in the seal.
- Runes are ornament, never text (design language §8): the bind-rune carries
  no `contentDescription` of its own; the semantics stay
  "Portrait of <displayName>" exactly as today.
- The catalogue stays frozen; this delta reads it, never edits it.

## Capability contract

### Trait derivation (owned pure code, new `SealSpec.kt`)

```text
internal enum class SealMetal { Gold, Bronze }

internal data class SealSpec(
  mineral: Int,          // 0..7
  runes: List<Int>,      // 1..3 distinct values of 0..15, normally 2..3
  beardTeeth: Int,       // 3..6
  beardDepthStep: Int,   // 0..3
  facetMask: Int,        // 0..255, bit e = octagon edge e carries Gold
  metal: SealMetal,
)

internal fun sealSpec(characterKey: String): SealSpec
```

With `h = SHA-256(UTF-8(characterKey))` (bytes as unsigned):

```text
mineral        = h[0] mod 8
runeCount      = 2 + (h[4] and 1)
runes          = first distinct values of h[5+i] mod 16 for i in 0..7,
                 stopping at runeCount; if the eight draws yield fewer
                 distinct values, the shorter list stands (defined, not error)
beardTeeth     = 3 + (h[8] mod 4)
beardDepthStep = h[9] mod 4
facetMask      = h[12]
metal          = Gold if (h[14] and 1) == 1 else Bronze
```

Golden vectors (computed 2026-08-26; the red proofs pin them exactly):

| key | mineral | runes | teeth | depth | facetMask | metal |
| --- | ---: | --- | ---: | ---: | ---: | --- |
| `norse.modsognir` | 6 | [12, 9] (bjarkan+ár) | 6 | 1 | 229 | Bronze |
| `norse.durinn` | 4 | [9, 10] (ár+sól) | 3 | 0 | 24 | Bronze |
| `tolkien.gimli` | 4 | [5, 10] (kaun+sól) | 3 | 0 | 239 | Gold |

Verified against the frozen 101-entry catalogue: all 101 `SealSpec` tuples
are pairwise distinct, and the coarse `(mineral, runes)` projection alone is
already collision-free — every dwarf differs in field color or rune mark
before beard and facets are considered.

### Mineral fields (fill-only tones; the full table is owned by
`SealSpec.kt` in this delta — gem hexes match design language §5)

```text
0 Garnet #830E0D   1 Hematite #6D2114   2 BlueGlass #26619C   3 Basalt #2A2D33
4 MossStone #333B34  5 BronzeStone #43392C  6 Porphyry #3A2C33  7 Slate #2E3A3D
```

### Rune geometry

The 16 Younger-Futhark (long-branch) glyphs are a frozen table of straight
line segments in stave-relative coordinates (x in stave-widths from the
shared vertical stave, y in 0..1 of stave height), drawn monoline in the
seal's metal; `íss` contributes the bare stave. The reference segment table
is the artifact seal forge's `RUNES` array, carried into `SealSpec.kt` as
constants; it is decorative reference interpretation, explicitly not a text
encoding.

### Rendering (replaces `DwarfPortrait` drawing in `DashboardScreen.kt`)

Draw order at any size, on a square canvas clipped by the `Octagon` shape
(`CutCornerShape(29)` percent, added to `NidavellirShapes` in this delta as
its first consumer): mineral fill → two flat facet planes (white 4.5% upper-left triangle,
black 16% lower-right triangle) → beard silhouette (black 34%, `beardTeeth`
zigzag teeth, depth from `beardDepthStep`) → bind-rune (shared stave y
0.15–0.58 centered, branches within ±0.30 width, metal stroke ~4.5% of side)
→ octagon frame (base edge stroke in a neutral dark, Gold on the edges set
in `facetMask`) → Bone initial (first character of `displayName`, Display
face, bottom-right). The composable's size and card layout are unchanged
from today; the existing `contentDescription` stays.

## Hard cut

Delete the current portrait internals: the six-color hash palette, the
circle/arc/beard-path drawing, and the ad-hoc `hash = fold(17, *31)` seed.
No fallback portrait, no old/new switch.

## Work split

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Trait derivation | Android builder | `SealSpec.kt` (new), `SealSpecTest.kt` (new) | Golden + distinctness proofs |
| Rendering | Android builder (same slice) | `DashboardScreen.kt` | Compose proofs |
| Catalogue gate | Root integrator | `scripts/check-catalogue` | Static distinctness gate |
| Verification | Read-only verifier | none | Review only |

No gateway, wire, catalogue-content, tmux, or controller change.

## Red / green / refactor

Red (each observed failing first):

1. Pure JVM: the three golden vectors above decode exactly from their keys.
2. Pure JVM: `sealSpec` is stable across repeated calls and across string
   re-decoding (UTF-8 byte hashing, not platform default charset).
3. Static gate: `scripts/check-catalogue` gains a seal-projection check —
   all catalogue keys yield pairwise-distinct `SealSpec` tuples — run in
   routine verification with the same formulas transcribed; a formula drift
   between script and Kotlin is caught by the golden vectors.
4. Compose test: every card renders a seal for its required character and
   keeps `contentDescription = "Portrait of <displayName>"`; no code path
   renders the retired circle portrait.

Green: implement only enough to pass; routine `scripts/test verify` stays
green and device-free.

Refactor: keep derivation (`SealSpec.kt`) pure and rendering effectful;
delete the retired drawing helpers; no seal caching layer.

## Acceptance and gates

Routine verification plus the Compose/instrumented card suite re-run under
the existing separately-approved platform gate. The named hands-on check —
all 101 seals pairwise distinguishable at 48dp on the S22+ panel, spot-read
by scrolling a synthetic full-catalogue grid in the debug build — stays
`NOT_RUN` until approved; a distinguishability failure re-opens the mineral
table or facet weights in this document, not ad-hoc in code.

## Non-goals

User-selected or editable seals, seal persistence beyond the derivation,
animation, wire/image fields, catalogue edits, Elder Futhark, zoomorphic
elements, portrait export, and any use of the seal outside the session card.
