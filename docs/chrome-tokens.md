# Design delta D2: chrome tokens

Status: implemented 2026-08-26 with adversarial-review fixes applied;
re-woven over the multi-machine federation the same day. Routine
verification green; the 33-test instrumented suite green on the physical
S22+ (devbox debug-signed run — the signed deviceDebug platform gate is
MacBook-owned); the hands-on pass (incl. the Forge warm-in) stays
`NOT_RUN`.

[`architecture.md`](architecture.md) owns product behavior and acceptance —
including literal labels, 48dp targets, and the distinct-status-color
contract; [`design-language.md`](design-language.md) owns the visual values
(§5 color, §6 shape, §9 type, §12 motion, §13 components); this document owns
the implementation boundary and delivery plan.

## Outcome

One owned token system — color, shape, typography, motion, interaction
states — and every Compose surface consuming it: bearer repair, Hlíðskjálf grid,
The Forge, terminal chrome, and key-deck styling. `SHELL` renders Bronze and
becomes visually distinct from `RUNNING` (today both are Frost). No wire,
input-semantics, or behavior change of any kind.

## Goals and rules

- **Tokens are the single source.** Every color, shape, type role, state
  alpha, and motion constant used by Compose code lives in the token file;
  screens reference tokens, never literals. `MainActivity.kt` keeps no color
  definitions. Sole exception: `DwarfPortrait`'s interior literals belong to
  D3's replacement (see hard cut) and are not migrated twice.
- **Behavior is untouched.** Every label string, semantics property, touch
  target, traversal order, spoken state, dialog flow, and controller call is
  byte-identical in intent; this delta may change only visual presentation.
  The key deck's reviewed content and interaction contract
  ([terminal-key-deck.md](terminal-key-deck.md)) is restyled, not reopened.
- **Corners are cut, never rounded** (design language §6): every
  `RoundedCornerShape` in app chrome is replaced by the token shapes. The
  octagon is `CutCornerShape(29%)` on a square; the lozenge is a rotated
  square — no new geometry dependency.
- **Nothing bounces; nothing glows.** Effects motion (color/opacity) is a
  100ms standard tween; the sole ambient animation in the app is the Forge
  sheet warm-in; the attention pulse renders static when the system disables
  animator scale. All motion respects design language §12 exactly.
- Fonts bundle as Android font resources with their OFL license texts in the
  APK; dynamic and user/agent-originated text never routes through Display or
  scholarly faces (design language §9).

## Capability contract

### Owned tokens (new file `Theme.kt`)

```text
// Colors ship as flat top-level internal vals in Theme.kt (not an object):
// the nine existing vals moved verbatim, and the three screen files already
// reference them as bare same-package symbols — wrapping would force edits
// the hard cut does not require.
NidavellirColors (flat vals)
  Ink #0C0D0F, DeepSurface #15171A, RaisedSurface #202329, ForgeGlow #28231A,
  Bone #F3F0E8, Muted #AAA69D,
  Gold #D6A85F, Ember #E46C55, Moss #76B082, Frost #78A9C6,
  Bronze #CD7F32, Orpiment #E8B923
  // Overlook and the gem fills (design language §5) are documented values
  // with no consumer in this delta and are deliberately NOT shipped here —
  // no speculative tokens (rules/simplicity). Overlook waits for a topmost
  // sheet; the gems land with D3's mineral table.

internal object NidavellirShapes
  Card    = CutCornerShape(10.dp)
  Chip    = CutCornerShape(4.dp)
  Key     = CutCornerShape(4.dp)
  Sheet   = CutCornerShape(topStart = 12.dp, topEnd = 12.dp)
  // Octagon (CutCornerShape(29) percent) lands with D3's seal — its first
  // consumer; not shipped speculatively here.

internal object NidavellirType
  Display    // Big Shoulders variable, caps usage, wght ~650
  Data       // JetBrains Mono variable, wght ~500, ≥11sp

internal object NidavellirMotion
  EffectsTween   = tween(100ms, standard easing)         // never a spring
  ForgeWarmIn    = tween(400ms)                          // the one ambient
  StateLayer     = pressed .10f
  DisabledAlpha  = content .38f, container .12f
  // SpatialSpring and the hover/focus/dragged state-layer constants
  // (design language §12) join with their first consumer — nothing in this
  // delta animates layout or has hover/drag surfaces; no speculative tokens.

internal fun statusColor(kind: SessionStatusKind): Color   // exhaustive, injective
internal fun attentionPulseEnabled(animatorDurationScale: Float): Boolean
```

`statusColor` maps `Working→Moss, Running→Frost, Idle→Gold, Shell→Bronze,
Unknown→Muted` and moves from `private` to `internal` so its injectivity is a
pure JVM proof. Pressure history and details keep `Normal→Moss, Warm→Gold,
Hot→Ember, Unknown→Muted`; the collapsed rail independently quiets
informational/normal values to Bone with Muted labels/marks and spends
Gold/Ember only on host-evaluated Warm/Hot exceptions.

### Fonts

- `res/font/big_shoulders.ttf` (variable) and `res/font/jetbrains_mono.ttf`
  (variable), upstream release bytes; Compose consumes them via
  `FontVariation.Settings` weight axes.
- OFL license texts bundled at
  `android/app/src/main/assets/licenses/BigShoulders-OFL.txt` and
  `JetBrainsMono-OFL.txt` (the OFL requires the license accompany the fonts).
- Roles per design language §9: Display carries the wordmark, screen titles,
  and dwarf display names; Data carries every machine fact (status bays,
  ages, cwd, tmux names, ids, pressure numerals, key-deck labels) at ≥ 11sp;
  body text stays the system face. Junicode does not ship in this delta.

### Surfaces restyled (visual only)

- **MaterialTheme**: color scheme sourced from tokens (adds
  `tertiary`-slot-free Bronze/Orpiment as plain token uses, not scheme
  slots); typography wires Display/Data roles onto the styles that carry
  them; default component shapes become the token shapes.
- **Bearer-repair screen**: wordmark `SKÍÐBLAÐNIR` in Display caps; layout,
  copy, and flow unchanged.
- **Grid cards**: DeepSurface, `Card` shape, a single top-edge Gold hairline
  at 25% alpha, work-first stack — tmux name is primary in Data, the dwarf
  display name remains a smaller quiet Display-face signature, and directory
  plus conditional machine/profile form the runtime-facts lines in Data.
- **Status bay and facet**: the hand-rolled status `Surface` (not an M3 chip
  component) takes the `Chip` shape, fill = status color at 18% over surface,
  1dp hairline and label in the status color, label + named signal/age in Data
  ≥ 11sp. A separate 12dp solid `Chip`-shaped facet repeats only the status
  color in the fixed top-right slot and is semantics-silent. Literal strings
  and the bay's existing accessibility label are unchanged.
- **Attention badge**: net-new construction — today attention is a bare
  Ember `!` text glyph. It becomes an Orpiment lozenge (rotated square)
  carrying the same semantics (`contentDescription = "Needs attention"`,
  unchanged). Pulse: opacity 1.0→0.55, ~1.6s, no-bounce, only when
  `attentionPulseEnabled`; otherwise a static full-opacity lozenge — a
  designed state, not a frozen frame. Opening the card still clears
  attention (existing gateway behavior on attach; it is the WCAG 2.2.2 stop
  mechanism and is documented as such here).
- **The Forge**: ForgeGlow surface, `Sheet` shape, title in Display; the
  ForgeWarmIn color transition on open is the app's only ambient animation
  and is skipped when animations are disabled. Error text stays plain Ember
  body — no ornament on error or destructive surfaces, ever.
- **Kill dialog**: DeepSurface, `Card` shape, copy untouched, zero
  decoration.
- **Terminal chrome and key deck**: top bar and deck bezel on tokens; keys
  take the `Key` shape and Data labels; the armed-Ctrl visual stays
  Gold-fill/Gold-content as today. Geometry (48dp/8dp), order, semantics,
  spoken state, and enablement per the key-deck spec — unchanged.
- **Interaction states**: pressed/focus/hover/dragged use the StateLayer
  alphas as Bone over the component surface. Pressed components adopt the
  angular indication — an inset copy of the pressed component's own cut
  outline at pressed alpha, via one `IndicationNodeFactory` implementation
  (`AngularIndication.kt`) that takes the shape it is cut to, so the flash
  and the component can never be two outlines. This delta ships the one
  Card-shaped consumer; the Forge seal added the Octagon-shaped second
  ([forge-seal.md](forge-seal.md)). This requires a structural change this delta
  owns: M3's `Card(onClick)` hardcodes its internal `ripple()` and never
  reads `LocalIndication`, so `AgentCard` is rebuilt as a plain `Surface`
  with an explicit `Modifier.clickable(interactionSource, indication =
  AngularIndication(NidavellirShapes.Card), …)` carrying the card's existing
  semantics and click behavior unchanged. Every other Material component that owns its ripple
  internally (buttons, text fields, the Forge's profile `FilterChip`) keeps
  the platform ripple in this delta; unifying them is a listed non-goal
  until reviewed on-device.
  **Superseded by D6** ([destructive-chrome.md](destructive-chrome.md)):
  `AngularIndication` shipped here as a singleton hardcoding the `Card` cut,
  with a note that a second consumer of another shape reopens this document.
  D6 reopened it — the kill control's press flash has no legal alternative,
  since the platform ripple is circular and §15 forbids circular ripples. It
  is now `AngularIndication(shape)`, a data class whose structural equality
  keeps `Modifier.clickable` from churning; the card passes `Card` and the
  kill control passes `Cleft`. This is what design-language §12 asked for in
  the first place — "an inset copy of the component's own cut-corner outline
  … one implementation, used everywhere" — so the singleton, not the
  parameter, was the deviation.
- **Disabled**: 38% content / 12% container plus one non-opacity cue (chip
  hairline dropped).

## Hard cut

- Color vals leave `MainActivity.kt`; no duplicate definitions remain.
- Every `RoundedCornerShape` use in app chrome is deleted, not aliased —
  the banners, card, status bay, and dialog call sites — with one declared
  exception: `DwarfPortrait`'s internals (its rounded clip and its eight raw
  color literals) are D3's wholesale replacement and are deliberately left
  untouched here rather than half-migrated twice.
- No parallel theme, style flag, or old/new toggle exists anywhere.

## Work split

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Tokens + theme | Compose UI builder | `Theme.kt` (new), `AngularIndication.kt` (new), `MainActivity.kt`, `res/font/*`, `assets/licenses/*`, `android/app/build.gradle.kts` (font resources only), `ThemeTest.kt` (new, JVM) | Pure token proofs below |
| Screens | Compose UI builder (same slice) | `DashboardScreen.kt`, `TerminalScreen.kt`, `TerminalKeyDeck.kt`, `DashboardChromeInstrumentedTest.kt` (new) | Compose behavior proofs below |
| Verification | Read-only verifier | none | Review only |

No gateway, tmux, protocol, controller-logic, or `scripts/test` change. The
key-deck and identity contracts are consumed, never edited.

## Red / green / refactor

Red (each observed failing first):

1. Pure JVM: `statusColor` is injective across all five kinds — fails today
   because `Shell` and `Running` both return Frost.
2. Pure JVM: `attentionPulseEnabled(0f)` is false; positive scales are true.
3. Compose test (new `DashboardChromeInstrumentedTest.kt` — there is no
   dashboard Compose suite today; these proofs create it): the status bay
   for each kind still renders its literal label with named signal and age
   (guards the restyle against copy drift).
4. Compose test (same new suite): card, chip, and key targets remain ≥ 48dp
   with ≥ 8dp spacing, and the rebuilt clickable card keeps its click action
   and semantics (guards the `Card`→`Surface`+`clickable` restructure and
   `CutCornerShape` padding changes).
5. Instrumented: the Display face resolves `Ð` and `Í` (wordmark and dwarf
   names render in Big Shoulders, not a fallback).
6. Instrumented: with animations disabled (the existing
   `animationsDisabled = true` test config), the attention badge renders
   static at full opacity (pixel-asserted), and the Forge sheet opens and
   settles with all animations resolved. `ModalBottomSheet` ANRs the S22+
   under a `createComposeRule`-owned host activity in every tested variant
   (paused clock, running clock, bare `assertIsDisplayed`); it composes fine
   when the suite drives a real activity via `ActivityScenario` +
   `createEmptyComposeRule` (the multi-machine journey opens the Forge that
   way). The warm-in *look* (opens lit in ForgeGlow, no strand) still
   belongs to the hands-on pass.

Green: implement only enough to satisfy those proofs plus the visual
contract; routine `scripts/test verify` stays green and device-free.

Refactor: collapse any remaining color/shape literal into the token objects;
delete dead style helpers; add no abstraction with only a speculative second
consumer.

## Acceptance and gates

Routine verification; one separately approved platform pass (the existing
instrumented suites — terminal, key deck, bearer store — re-run green, plus
the new dashboard chrome suite and proofs 5–6);
one hands-on S22+ look: SHELL vs RUNNING distinguishable at a glance,
chip text comfortable at 11sp, state-layer visibility on Ink at night
brightness, Forge warm-in reads as material rather than effect. No
integration or live gate.

## Non-goals

Seal generator (D3), ornament assets and app icon (D4), Junicode bundling,
light theme, dynamic color, M3-internal button/text-field ripple unification,
`androidx.graphics-shapes` dependency, key-deck content or interaction
changes, and any wire or gateway work.
