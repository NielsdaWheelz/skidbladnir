# Niðavellir: the Skíðblaðnir design language

Status: reviewed design reference, 2026-08-26. This document owns visual
identity: color, shape, ornament, typography, iconography, motion, and the
terminal theme. [`architecture.md`](architecture.md) owns product behavior and
acceptance and wins on any conflict; [`roadmap.md`](roadmap.md) owns delivery
order. This document changes no source. Adopting any section into the app is a
delta slice with its own red/green plan; §17 names the deltas.

The name: *Vǫluspá* 37 places a hall of gold of Sindri's line on Niðavellir,
the "dark fields" — the same poem whose stanzas 10–16 supply the Dvergatal
([Vǫluspá, literal text](https://www.voluspa.org/literal/voluspa.htm)). A hall
of gold on dark fields is this app's entire visual thesis: warm metal accents
on near-black stone. The name appears in docs only; UI text stays literal per
architecture §2.

## 1. Philosophy

1. **A hall of gold on dark fields.** Near-black tonal stone carries everything;
   gold and gem color is rationed the way a hoard is — small, bright, and
   deliberate. Decoration never claims more area than information.
2. **Geometry first, ornament after.** The film-dwarven design rule: strong
   geometric structure is the foundation of dwarven material culture, and
   embellishment is applied to it afterward, never instead of it
   ([Culture on Call, WETA design](https://www.cultureoncall.com/the-magic-of-middle-earth-bringing-the-imaginary-to-life/)).
   A component must be complete and legible with every ornament deleted.
3. **The Narvi rule.** The Doors of Durin split one artifact: the dwarf Narvi
   made the doors; the elf Celebrimbor drew the flowing signs
   ([Tolkien Gateway](https://tolkiengateway.net/wiki/Doors_of_Durin)).
   Dwarf-owned chrome — frames, corners, borders, dividers, chips — is
   straight-edged and faceted. A curve is a marked guest, allowed only in
   large-scale interlace art (§7) and in glyphs of text faces. Rounded corners
   do not exist in this language; corners are cut.
4. **Engineering over magic.** The angular Key to Erebor was designed to
   suggest a race relying on engineering and mechanical skill rather than magic
   (same source). Visual honesty is the app's existing law — chips name their
   signal and age, absence is displayed rather than guessed — and the
   aesthetic must reinforce it: no glows implying activity that tmux did not
   report, no ornament that reads as affordance, no decoration on error or
   destructive copy.
5. **Carved, not printed.** Surfaces are rock strata: elevation is a tonal
   step computed from one formula (§5), never a drop shadow. Ornament reads as
   cut into or out of the surface. Motion has mass: stone does not bounce (§12).
6. **One style per component, provenance for every motif.** Blending Viking art
   styles, faking runes onto Latin letters, or sourcing clip-art is exactly
   what separates kitsch from a considered style. Every motif in this language
   has a primary-source anchor in §2, matching the Dvergatal's curation bar.
   A motif without provenance does not ship.

## 2. Provenance ledger

| Motif | Source | Role here |
| --- | --- | --- |
| Gold-on-dark ground | *Vǫluspá* 37, Sindri's hall on Niðavellir | Palette thesis |
| Gold-and-garnet cloisonné | Sutton Hoo shoulder-clasps / Staffordshire Hoard ([British Museum](https://www.britishmuseum.org/collection/object/H_1939-1010-2-a-l)) | Gem fill colors; gold-line-on-dark card grammar |
| Angular key/fret (meander) | Pictish stones, Class II–III edges ([overview](https://en.wikipedia.org/wiki/Pictish_stone)); Greek-key corner methods ([Edkins](https://www.theedkins.co.uk/jo/greekkey/corners.htm)) | The default repeating border family (§7) |
| Chevron / V-motif banding | Film-dwarven Art Deco borrowing (Culture on Call, above) | Simplest member of the fret family; dividers |
| Faceted planar surfaces | Iron Hills armor, planar like a cut gemstone ([Nick Keller](https://www.nickkellerart.com/projects/1N6G82)) | Cut-corner shape grammar (§6) |
| Interlace with breaks | Insular plaitwork, Book of Durrow onward; Bain grid-and-dot construction ([method restatement](http://www.zuggsoft.com/sca/celtic/celtic.htm)); Mercat scaffold ([jamis/celtic_knot, public domain](https://github.com/jamis/celtic_knot)) | Large-scale art only (§7) |
| Valknut, tricursal | Tängelgårda / Stora Hammars I stones, beside Odin figures ([Valknut](https://en.wikipedia.org/wiki/Valknut)) | Hlíðskjálf mark: Odin's seat overlooking all sessions |
| Ship under one square sail | Gotland picture stones, same corpus as the valknut row: the Viking-age slabs carry a ship under one large sail (Tjängvide I, Alskog — [Gotlands Museum catalogue](https://www.gotlandicpicturestones.se/s/index/item/2052)); the sail itself is unbroken vertical lengths of cloth sewn together ([Viking Ship Museum, Roskilde](https://www.vikingeskibsmuseet.dk/en/professions/boatyard/experimental-archaeological-research/maritime-crafts/maritime-technology/wool-sailcloth)) | Launcher mark: hull, mast, yard, and the sail's three gores (§8) |
| Shield row along the gunwale | Gokstad ship burial, c. 890: 32 round shields to each side, painted alternately yellow and black ([Museum of the Viking Age](https://www.vikingtidsmuseet.no/english/the-collection/the-gokstad-ship/the-gokstad-ship.html)) | Launcher mark: the Garnet row at the sheer (§8). §6 overrides the round shape with its lozenge and §5 the alternating paint with one fill, so the row renders uniform |
| Ship vane | Söderala vane, gilt bronze, c. 1050: worn from use on a ship before it was reused as a church weathervane ([Söderala vane](https://en.wikipedia.org/wiki/S%C3%B6derala_vane)) | Launcher mark: the vane at the masthead, the mark's one asymmetry (§8) |
| Bind-runes on a shared stave | Runestone/coin monograms ([Bind rune](https://en.wikipedia.org/wiki/Bind_rune)) | Dwarf-seal maker's mark in portraits (§11) |
| Younger Futhark | Viking-age script, c. 800–1100, Unicode U+16A0–16FF ([Runic block](https://en.wikipedia.org/wiki/Runic_(Unicode_block))) | The only runes that appear, ornament-only (§8) |
| Forge-heat sequence | Black-red → red → orange → gold → yellow tempering ramp ([red heat](https://en.wikipedia.org/wiki/Red_heat)) | Ordinal intensity ramp (§5) |
| Diminuendo | Insular initial-to-text size decay, Kells/Lindisfarne (Nordenfalk 1947) | Typographic hierarchy rule (§9) |

Rulings on the research contradictions, decided here once:

- **Default border grammar is the angular fret/meander family.** Chevron and
  key-fret are one family and it owns all small chrome. Borre ring-chain,
  though the most geometric Viking style, keeps circles; it is demoted to
  optional large-scale single-use art. Curvilinear interlace is reserved for
  large art. Rationale: below roughly 24dp of band height, woven curves turn
  to mush; frets have no curves to lose.
- **No zoomorphic terminals anywhere in chrome.** Beast-headed interlace
  (Borre/Urnes vocabulary) is illegible at chip scale and is the highest
  kitsch-risk motif. Zoomorphic work is permitted only as future large hero
  art, explicitly marked, never as a repeating element.
- **Younger Futhark only.** Elder Futhark ends c. 800 and predates every
  Viking art style above; pairing it with this grammar is an anachronism. The
  16-rune Younger row is the set contemporary with the Dvergatal's poem.
- **No blackletter, ever.** Fraktur was decreed the official German "Volk"
  typeface in 1933 and served Nazi identity through 1941; the association is
  documented and disqualifying regardless of the family's angular kinship
  ([history](https://www.sitepoint.com/the-blackletter-typeface-a-long-and-colored-history/)).
- **No Tolkien scripts.** Cirth/Angerthas are Estate-protected invented
  scripts with an enforcement history (fan fonts pulled ~2014). Real Futhark
  is the legally clean, historically superior substitute.

## 3. What already exists and is kept

Ink `#0C0D0F`, DeepSurface `#15171A`, RaisedSurface `#202329`, Bone `#F3F0E8`,
Muted `#AAA69D`, Gold `#D6A85F`, Ember `#E46C55`, Moss `#76B082`, Frost
`#78A9C6` (`MainActivity.kt`). These are validated, not replaced: Ink sits
correctly above the ~`#050505` OLED black-smear floor; DeepSurface and
RaisedSurface fall on the Material dark-elevation overlay curve computed from
Ink (5% and ~9% white; verified `#18191B` / `#222325` against actuals); the
four accents fill the 2–4 accent-hue budget a small app can carry. The Nord
palette's near-identical structure (dark ladder + cool accents) confirms this
is the established Nordic-terminal convention. What was missing is the system
around them — and two defects fixed below: `SHELL` and `RUNNING` share Frost
today (`DashboardScreen.kt` `statusColor`), and the terminal ANSI table is
One-Dark defaults with no relation to the brand (`terminal.js`).

## 4. Voice

Labels, domain, error, and destructive copy stay literal (architecture §2).
The dwarven voice lives entirely in geometry, material, and type — never in
wording. No "ye olde" register, no rune-transliterated labels, no themed
error messages. "Kill skidbladnir-work-1 on Devbox?" stays exactly that.

## 5. Color

### Surfaces (stone strata)

Elevation is generated, not hand-picked: blend Ink toward white by the
Material dark-overlay percentage for the tier. Never animate through a tone
darker than Ink; never use `#000000` anywhere (OLED smear).

| Token | Hex | Tier |
| --- | --- | --- |
| Ink | `#0C0D0F` | 0dp; app background, terminal background |
| DeepSurface | `#15171A` | ~1dp; cards at rest, dialogs |
| RaisedSurface | `#202329` | ~4–6dp; raised card elements, chips ground |
| Overlook | `#2E2F31` | 12dp (14% white); reserved for a future topmost sheet |
| ForgeGlow | `#28231A` | 12dp blended toward Gold instead of white; The Forge sheet and the lit Forge seal (§13) — the app's only firelit surfaces |

### Text and accents

All ratios are WCAG 2.1 against Ink, computed and verified locally.

| Token | Hex | Ratio | Use |
| --- | --- | ---: | --- |
| Bone | `#F3F0E8` | 17.07 | Primary text |
| Muted | `#AAA69D` | 8.01 | Secondary text |
| Gold | `#D6A85F` | 8.91 | Primary accent; `IDLE`; cursor; selected/armed states |
| Ember | `#E46C55` | 6.09 | Error; destructive; `HOT` |
| Moss | `#76B082` | 7.69 | `WORKING`; healthy |
| Frost | `#78A9C6` | 7.67 | `RUNNING`; informational |
| Bronze | `#CD7F32` | 6.18 | `SHELL` — new; Gold-family variant, no fifth hue. Fixes the Frost double-duty defect |
| Orpiment | `#E8B923` | 10.55 | Attention badge only. The loudest color owned; spending it on exactly one mark keeps attention pre-attentive |

Status mapping becomes: `WORKING` Moss · `RUNNING` Frost · `IDLE` Gold ·
`SHELL` Bronze · `UNKNOWN` Muted. Pressure history and detail rows keep Normal
Moss · Warm Gold · Hot Ember · Unknown/missing Muted · Informational Frost. The
collapsed pressure rail is deliberately quieter: labels and `i/N` marks are
Muted, informational/normal values are Bone, and only Warm/Hot values and marks
spend Gold/Ember.

### Severity tones

Severity has exactly one owner (`noticeToneColor`, D6). Ember is not a colour
a surface may reach for; it is what one tone resolves to.

| Tone | Colour | Means |
| --- | --- | --- |
| Failure | Ember | An attempt failed, trust broke, or you are ending something |
| Degraded | Muted | Knowledge is absent or old; nothing is broken |
| Armed | Gold | A recovery is waiting on you |

Two rules follow, and both are load-bearing:

- **Staleness is absence, not failure.** Degraded is Muted, matching `UNKNOWN`
  → Muted above and the honesty law (§1.4): absence is displayed, not alarmed.
  Ember spent on routine staleness is Ember spent on nothing — in a federation
  one host is out often enough that the alarm becomes the resting state.
- **Trust events are the loud ones.** A broken bearer or a changed identity is
  Failure even when that machine's inventory is perfectly current, because
  what failed is knowing who we are talking to.

The bare `Ember` token survives in exactly two places in the app: its
definition beside `noticeToneColor`, and `pressureColor`, where the table
above assigns Ember to `HOT`. Host load is a genuinely separate axis with its
own owner; every other Ember arrives through a tone.

### Gem fills (cloisonné)

Pigment-accurate gem colors fail as foreground on Ink (garnet `#830E0D`
1.88:1) but pass richly as filled grounds under Bone text — exactly how
garnet works in cloisonné: a dark stone in a bright metal cell. Fill-only,
never text/icon foreground:

| Token | Hex | Bone-on-fill ratio | Source |
| --- | --- | ---: | --- |
| Garnet | `#830E0D` | 9.07 | Almandine cloisonné red |
| Hematite | `#6D2114` | 9.82 | Iron-ore red-brown |
| BlueGlass | `#26619C` | 5.64 | Sutton Hoo boar-tusk glass |

### Forge-heat ramp (ordinal only)

`#400B0B → #A12424 → #EE7B06 → #FFA904 → #FFDB00` follows the physical
tempering sequence; contrast against Ink rises monotonically (1.18 → >10), so
the ramp encodes "intensifying" for free. Use only for a future continuous
meter, never categorical pressure history or chips. Interpolate between stops
in Oklab, not sRGB, to avoid muddy midpoints.

### Budget

Hard cap: four accent hue families in UI chrome (Gold+Bronze+Orpiment are one
family; Ember; Moss; Frost). New needs are met with fill-variants inside an
existing family. The terminal ANSI table (§10) is exempt: it emulates a fixed
external 8-hue protocol that git/ripgrep/pytest assume exists.

## 6. Shape grammar

- **Corners are cut, not rounded.** `CutCornerShape` everywhere a radius
  exists today. Facet unit: cards 10dp cut; chips and keys 4dp; sheets 12dp on
  the top corners only. All cuts are 45°.
- **One shape is allowed to disagree with itself.** The kill control's `Cleft`
  keeps the 4dp chip facet on three corners and cuts 14dp on the fourth
  (top-start). It is the only asymmetric shape in the product and it is
  reserved to destructive controls, which is what makes it legible as
  meaning rather than as a mistake: architecture's guarantee that detach and
  kill are visibly different actions has to survive greyscale, and §15
  forbids the icon that would otherwise carry it. A second asymmetric shape
  would spend the distinction, so adding one reopens this clause.
- **Portrait frames are octagons.** A square with all corners cut at 29% of
  the side is an octagon; circles are elvish in this grammar. The exactly
  regular cut is (2−√2)/2 ≈ 29.29%; 29% is the shipped round number, which puts
  each vertex 0.16dp from its regular position on a 56dp frame and leaves the
  axis edges 2.4% longer than the diagonals (23.52dp against 22.97dp). Both are
  below the perceptual floor for a hairline outline, and not worth re-cutting
  every seal already struck. Whatever draws the octagon reads the same shipped constant
  the clip does, so a frame and its clip can never disagree.
- **Badges are lozenges.** The attention badge is a rotated square (diamond),
  the fret family's atom, in Orpiment.
- **Faceting replaces gradients.** Where a surface needs richness, split it
  into 2–3 flat planes differing by one elevation step — the faceted-gemstone
  armor rule — never a smooth gradient or soft shadow.
- **Strata edges.** A card may carry a single 1dp inner hairline in
  Gold at 25% opacity along its top edge — light catching the carved lip —
  and nothing else. No outer glows, no multi-edge strokes.
- Implementation: `CutCornerShape(dp)` for the standard cuts; the octagon
  is `CutCornerShape(29%)` on a square and the lozenge a rotated square, so
  no geometry dependency is needed. `androidx.graphics:graphics-shapes`
  (Apache-2.0) is adopted only if a genuinely irregular silhouette ever
  outgrows `CutCornerShape`.

## 7. Ornament system

Two families, split by scale, never blended:

- **Fret (angular meander): the default.** Chevron bands, key patterns, step
  patterns. Owns all repeating chrome: dividers, sheet headers, the terminal
  key-deck bezel, card frames if ever framed. Rules: monoline stroke, 45°/90°
  turns only; band height ≤ 16dp in chrome; drawn in Muted or Gold at ≤ 40%
  opacity so it never competes with text; corners resolve by the
  reversal-at-midpoint method, which requires an even count of pattern units
  per side — sizing to that count is part of the component spec, not optional.
- **Interlace (woven ribbon): large art only.** Permitted where stroke ≥ 3dp
  and loop radius ≥ 8dp: empty states or a future About/Dvergatal catalogue
  view. Constructed, never drawn freehand: Bain
  grid-and-dot — dot grid at ribbon-width spacing, breaks as uncrossable
  barriers, diagonal strands, strict alternating over/under (parity rule).
  Breaks are first-class: model real layout edges (safe areas, text bounds)
  as breaks so ornament weaves around content instead of colliding with it.
  Geometric interlace only; no beast heads (§2 ruling).

Both families are generated at build time by a deterministic script emitting
checked-in path constants ([ornament-pipeline.md](ornament-pipeline.md) owns
the pipeline; an SVG → [Valkyrie](https://github.com/ComposeGears/Valkyrie)
route is an acceptable alternative), and drawn by stroking the
checked-in segments from cached geometry (a `ShaderBrush` image tile is the
fallback if profiling ever demands it).
Knot topology is never computed on-device at runtime. License notes for
reference implementations: jamis/celtic_knot public domain (concept source),
bezborodow/celtic-knot BSD-3 (usable), rspencer01/celtic MIT (readable);
codeplea GPL-3 and all unlicensed repos are off-limits for code reuse.

## 8. Iconography and runes

- **The Hlíðskjálf mark** is the tricursal valknut: three interlocked
  triangles, Borromean topology — the seat that overlooks every world. It marks
  the Dwarves surface wherever that surface is named — the dashboard title
  lockup, every affordance that returns to it, and the empty grid — and it is
  never the launcher, which carries its own mark below. Pure straight lines. Its
  stroke is a fraction of the mark's own size, never a fixed dp: a fixed stroke
  is a larger share of a smaller mark, so it closes the baked crossings as the
  mark shrinks. Legibility is therefore an invariant and not a size — every
  strand is at least as long as its own stroke is wide, and every baked break at
  least twice as wide — and it is proved on the geometry rather than eyeballed.
  [hlidskjalf-mark.md](hlidskjalf-mark.md) owns the constants and the proofs.
- **The launcher mark** is Skíðblaðnir under sail: cut stems, a square sail in
  three gores, a Garnet shield row at the sheer, a vane at the masthead.
  Straight segments only, authored as constants and never traced. Its
  legibility floor is stated in dp at the smallest launcher size — 2dp of
  daylight at 48dp — rather than as a stroke ratio, because the icon has no
  stroke and a known size range, while the valknut is drawn at arbitrary sizes
  against a stroke that scales with it.
  [`launcher-mark.md`](launcher-mark.md) owns its constants and proofs.
- **Runes are ornament, never text.** Two mechanisms, deliberately distinct:
  rune *glyphs* — at most one inert divider glyph per screen — render as
  real code points (U+16A0–16FF) in Noto Sans Runic (OFL), never a
  Latin-substitution novelty font; no delta currently ships one, and the
  font bundles only when one does. The dwarf seal's bind-rune (§11) is not
  text at all: a *drawn* monogram, strokes merged on one shared stave per
  the historical construction — a merged bind-rune cannot be produced by
  placing sequential font glyphs — and [dwarf-seals.md](dwarf-seals.md)
  owns its geometry. Never transliterate labels, never rune-encode meaning
  a user must read.
- **The unstruck seal** is the create mark: the §11 seal with every trait at
  zero — an octagon frame around a bare stave crossed by one horizontal bar,
  and nothing else. No mineral, no beard, no facet mask, no initial. It is the
  blank a dwarf is struck from, and it is honest: the catalogue identity is
  assigned after creation, so the client cannot draw the seal it is about to
  make. Its crossbar is perpendicular to the stave, which no Younger Futhark
  branch ever is, so the mark cannot be read as a rune — that is enforced as a
  proof, not asserted. [forge-seal.md](forge-seal.md) owns its geometry.
- **No horned helmets, drinking horns, axes, or hammer clip-art.** The horned
  helmet is a 19th-century invention with zero archaeological support; the
  rest is tourist-shop register. The app's dwarvenness is structural, not
  propped. Reconsidered once, for the create action (D5), and re-rejected:
  provenance was never the objection — Skíðblaðnir and Mjölnir come from one
  wager (*Skáldskaparmál* 35) — but Skíðblaðnir loses that wager, Mjölnir marks
  the wielder rather than the smith, and the silhouette dies at 24dp monoline.
  The register is the objection, and it does not improve.

## 9. Typography

Four roles, all SIL OFL 1.1, bundled as single variable files (~0.9 MB
total). Every face guarantees ð/þ (Google Fonts Latin Core); only Junicode
covers ǫ, which appears solely in verbatim Old Norse.

| Role | Face | Use | Never |
| --- | --- | --- | --- |
| Display | Big Shoulders (condensed industrial gothic; caps, +4% tracking, weight 600–700) | Wordmark `SKÍÐBLAÐNIR`, screen titles, dwarf signatures on cards | Body copy, sentences |
| UI body | System Roboto (deliberate: quiet, zero bytes, Android-native) | All running text, labels, buttons, errors | Display duty above `titleLarge` |
| Data | JetBrains Mono (~293 KB variable) | Status bays, ages, cwd paths, session ids, key-deck labels, pressure numerals — every machine fact | Prose |
| Scholarly | Junicode 2 (subset to the quotation repertoire) | Verbatim Old Norse only: Dvergatal stanza epigraphs in a future About/catalogue view | UI chrome, dynamic text |

The Display role's caps direction applies only where the source is caps, such
as the wordmark and applicable headings. Dwarf signatures preserve exact
catalogue casing.

- The choice *against* Cinzel/Trajan-likes is deliberate: lapidary Roman caps
  are the fantasy-app default and read imperial Rome, not the forge. Big
  Shoulders' flat terminals and condensed verticals read stamped metal and
  echo the fret grammar. The choice against decorative "Viking" faces is
  licensing and register (§2).
- **Machine facts speak mono.** The dashboard borrows the terminal's own
  voice for everything tmux/proc reported — a quiet, honest bridge between
  the two surfaces.
- **Work-first hierarchy** on the session card: tmux name (Data, largest and
  highest contrast) → dwarf display-name signature (Display, smaller and
  quieter) → named status bay → objective → directory and conditional
  machine/profile context (Data, smallest, ≥ 11sp). A stepped decay around the
  operator's work label, not a flat metadata stack.
- Berkeley Mono is explicitly unusable (its standard tiers exclude terminal
  apps). Norse by Joël Carrouché is unusable (embedding rights ambiguous).

## 10. Terminal theme

The current `terminal.js` theme is One Dark's ANSI table on brand chrome —
an inherited default. Replaced by a derived table. Design: **bright slots are
the four brand accents verbatim** (bold=bright is the emphasis signal
git/pytest/ripgrep rely on — the reason Nord's bright=base collapse is
rejected); **base slots are the same hues systematically darkened** (S−6pp,
L≈44%, with red/blue/magenta lifted to L52/50/54 because those hues fail
4.5:1 on true near-black at the flat recipe); magenta and cyan are new hues
(H322, H174) that exist only here — git's `--color-moved` load-bears them.
All ratios verified against Ink; all base slots ≥ 4.5:1, all brights ≥ 6:1.

```js
theme: {
  background: "#0c0d0f",            // Ink
  foreground: "#f3f0e8",            // Bone
  cursor: "#d6a85f",                // Gold — default is hardcoded white
  cursorAccent: "#0c0d0f",          // Ink  — default is hardcoded black
  selectionBackground: "#f3f0e84d", // Bone 30%
  selectionInactiveBackground: "#f3f0e826", // Bone 15%
  // selectionForeground unset: glyph SGR colors stay visible when selected
  // scrollbarSlider*: unset — auto-derive from foreground correctly
  overviewRulerBorder: "#aaa69d",   // Muted — default is hardcoded white
  black: "#15171a",                 // DeepSurface (fill-only slot)
  red: "#d74e33",     green: "#4f925c",  yellow: "#ac7e35",
  blue: "#538bac",    magenta: "#bb5897", cyan: "#459c93",
  white: "#aaa69d",                 // Muted
  brightBlack: "#5c6370",           // de-emphasis; intentionally recessive
  brightRed: "#e46c55",             // Ember
  brightGreen: "#76b082",           // Moss
  brightYellow: "#d6a85f",          // Gold
  brightBlue: "#78a9c6",            // Frost
  brightMagenta: "#cd70ab", brightCyan: "#64c4ba",
  brightWhite: "#f3f0e8",           // Bone
}
```

- `cursor`, `cursorAccent`, `selectionBackground`, `overviewRulerBorder` must
  be set explicitly: xterm.js `ThemeService.ts` falls back to hardcoded
  `#ffffff`/`#000000` for them, not to the theme's own colors.
- Supply `extendedAnsi` for indices 232–255: the 24-step grayscale remapped
  as a linear Ink→Bone interpolation, so 256-color tools (bat, delta,
  rich-based TUIs) stay on-palette. The 6×6×6 cube (16–231) stays library
  default; remapping 216 cells buys nothing for one user. Mechanics: xterm
  consumes `extendedAnsi` as one contiguous array anchored at index 16, so
  this ships as a sparse 240-slot array with entries only at offsets
  216–239 ([terminal-theme.md](terminal-theme.md) owns the exact contract).
- Set `minimumContrastRatio: 3` as a safety net for arbitrary truecolor agent
  output; the designed table already exceeds it, so it never mutates
  designed colors. xterm.js halves the requirement for SGR-dim cells by
  design.
- `fontFamily` becomes vendored JetBrains Mono (the WebView CSP must admit
  the bundled font asset; scripts stay bundle-only).
- Spot-check on the physical S22+ panel: WCAG's formula has a documented bias
  near black (APCA critique), so ratios here are floors, not proof of
  comfort.

## 11. Dwarf seals (procedural portraits)

The portrait becomes a **seal**: the octagonal frame (§6), a mineral field, a
bind-rune maker's mark, and angular figure geometry — replacing the current
circle-arc face. Deterministic, pure function of the catalogue key, per the
[automatic dwarf identity](automatic-dwarf-identity.md) contract (no color
fields on the wire; Android derives everything from `character.key`).

Generator model (jdenticon-style channel slicing):

```text
h = SHA-256(character.key)
traits: field mineral (one of 8 fill-only tones: Garnet, Hematite,
        BlueGlass, and 5 desaturated stone variants);
        bind-rune (normally 2–3 Younger Futhark runes merged on one shared
        vertical stave per the historical monogram construction; a rarer
        shorter draw is defined, not an error);
        beard/figure geometry (angular, straight segments and 45° facets);
        frame accent (which octagon facets carry the metal hairline);
        metal (Gold | Bronze).
Every trait is a pure function of the digest. The exact byte slices are
frozen in dwarf-seals.md; changing them re-rolls every visible identity
and is a design event, never a silent tweak.
```

Rules: metal linework in Gold or Bronze on the mineral field; Bone initial
retained as a legibility anchor; no curves except none. Acceptance criterion
for the implementing slice: all 101 catalogue seals pairwise distinguishable
at 48dp on the physical panel — the research pass never verified this; treat
it as a red test, not an assumption.

## 12. Motion and interaction states

- **Stone has mass.** Spatial motion (sheet expand, grid reorder) uses
  damped springs: `dampingRatio` 0.85–1.0, `stiffness` 400–1500 — never
  Material 3 Expressive's bouncy defaults. Effects motion (color, opacity,
  state layers) never bounces: `tween(100ms, EasingStandard)` or a
  no-bounce spring. Nothing overshoots; nothing wobbles.
- **State layers** use Material's exact constants — hover 8%, focus 10%,
  pressed 10%, dragged 16% — as Bone over the component's surface. Disabled:
  38% content / 12% container, plus one non-opacity cue (drop the chip
  hairline; desaturate the seal).
- **The press flash is angular.** The circular ripple is off-grammar; the
  standard ripple only clips to a circle. Implement one shared
  `IndicationNodeFactory` that flashes an inset copy of the component's own
  cut-corner outline at pressed-state alpha (the documented custom-indication
  path since `rememberRipple` deprecation). One implementation, used
  everywhere.
- **Attention pulse**: the Orpiment lozenge may pulse opacity 100%→55% on a
  ~1.6s no-bounce loop. When `ANIMATOR_DURATION_SCALE == 0` it renders
  static at 100% — a designed static state, not a frozen frame. Opening the
  card clears attention (existing behavior), which is the WCAG 2.2.2
  stop-mechanism; document it as such in the implementing slice.
- **The forge breathes once.** Exactly one ambient animation is budgeted in
  the whole app: the Forge sheet's ForgeGlow surface may warm from
  DeepSurface to ForgeGlow over 400ms when opened. Everything else moves
  only when state moves.

## 13. Component register

- **Session card**: DeepSurface, 10dp cut corners, Gold lip hairline at 25%,
  work-first text stack (§9), fixed 12dp status facet at top right with the
  conditional attention lozenge immediately left, then 48dp seal beside the
  named status bay. Machine/profile form one quiet unbadged footer line;
  machine renders there only in `All`.
  Attention-first grid order is architecture-owned and unchanged.
- **Status bay**: 4dp cut corners, fill = status color at 18% over surface,
  1dp hairline + label in the status color, JetBrains Mono caps ≥ 11sp,
  signal + age line in Muted mono. Colors per §5 mapping.
- **Status facet**: 12dp `Chip` shape, solid status color, no border, motion,
  text, or semantics. It is a redundant scanning cue; the status bay owns
  meaning and accessibility.
- **Attention badge**: Orpiment lozenge, 10dp across, §12 pulse rules.
- **The Forge sheet**: ForgeGlow surface, 12dp top cuts, single fret band
  under the title (Gold 40%, even unit count), Display-face title, mono cwd
  field (autocorrect off is architecture-owned). Invalid drafts preserved;
  error text plain Ember body — no ornament near errors (§1.4).
- **The Hlíðskjálf mark**: 24dp Gold leading the `Dwarves` title, 18dp at the
  button's own content colour leading each affordance that returns there, 48dp
  Muted at 40% on the empty grid. One composable, semantics-silent everywhere.
  The detach control carries no mark: it names what happens to the session, not
  where the button goes.
- **The Forge seal**: the create control. A 56dp octagon anchored at the
  dashboard's bottom trailing corner over the grid, carrying the unstruck
  seal (§8). Lit = ForgeGlow field, Gold frame and mark; cold = DeepSurface
  field, mark in Muted at §12's disabled content alpha — field and hue both
  change, never opacity alone. Icon-only, spoken `New dwarf`, no ornament,
  and the lit/cold flip does not animate. [forge-seal.md](forge-seal.md) owns
  its geometry.
- **Terminal key deck**: RaisedSurface bezel with an optional 8dp fret edge
  (unscheduled — no delta ships it; adopting it later is its own small
  slice), keys 4dp cut, mono labels, 48dp/8dp geometry and spoken Ctrl
  semantics owned by [terminal-key-deck.md](terminal-key-deck.md). Ctrl
  armed = Gold 18% fill + Gold label + selected semantics.
- **Pressure rail**: RaisedSurface card, 10dp cuts, one split-colour header, one
  horizontally scrolling non-wrapping flat Data-face metric row, then the
  unchanged 16dp categorical history band with no title. The machine name is
  Bone; normal/absent header status is Muted; reading/warm is Gold; hot is
  Ember. Metric labels and `i/N` marks are Muted, informational/normal values
  are Bone, Warm values/marks are Gold, and Hot values/marks are Ember. Metrics
  have no fill, border, shape, separator, or nested action. The whole rail is
  one disclosure control. The machine-bound details sheet keeps the existing
  12dp top-cut sheet and panel language, with no second history band.

## 14. Accessibility floor

48dp targets and 8dp spacing where architecture/key-deck specs demand;
`minimumInteractiveComponentSize()` where a drawn control is smaller. Text ≥
11sp. Every text/surface pair in this document ≥ 4.5:1 except the two
declared fill/de-emphasis slots (ANSI black, brightBlack), which never carry
prose. Reduced-motion fallbacks per §12. Ornament is always
`contentDescription`-silent decoration; semantics live on the literal labels.

## 15. The forbidden list

Never: rounded corners; circular ripples; drop shadows; smooth decorative
gradients (Oklab ordinal ramps excepted); `#000000`; zoomorphic chrome;
Elder Futhark; rune-transliterated UI text; blackletter; Tolkien scripts;
horned-helmet/axe/horn/hammer clip-art (reconsidered for the create
action and re-rejected, §8); Cinzel-register imperial caps; more than one
ambient animation; ornament on error or destructive surfaces; unlicensed or
GPL knot code; fonts outside §9's table without a license entry here.

## 16. Asset licenses

| Asset | License | Status |
| --- | --- | --- |
| Big Shoulders, JetBrains Mono, Junicode 2, Noto Sans Runic | SIL OFL 1.1 | Bundle freely; Junicode and Noto Sans Runic only once a consumer ships |
| androidx.graphics:graphics-shapes, Valkyrie | Apache-2.0 | Optional — only if a need outgrows `CutCornerShape`/direct generation |
| jamis/celtic_knot (Mercat method) | Public domain | Reimplement concept |
| bezborodow/celtic-knot | BSD-3-Clause | Build-time use OK |
| jdenticon (channel-slicing model) | MIT (LICENSE verified) | Model only; own implementation |
| Berkeley Mono, Norse (Carrouché), Cirth fonts, Book of Kells facsimile imagery, WETA/film artwork | Restricted or unusable | Never embed |

## 17. Adoption deltas (not scheduled here)

Each is a separate slice with its own red proofs; roadmap owns ordering.
The grain: **(1) terminal theme** ([terminal-theme.md](terminal-theme.md) —
`terminal.js` table §10 + vendored mono + CSP admit); **(2) chrome tokens**
([chrome-tokens.md](chrome-tokens.md) — shape grammar, status remap incl.
Bronze, state layers, angular indication, fonts); **(3) seals**
([dwarf-seals.md](dwarf-seals.md) — §11 generator + distinguishability red
test); **(4) ornament** ([ornament-pipeline.md](ornament-pipeline.md) —
build-time fret pipeline, Forge/empty-state art, app icon);
**(5) the Forge seal** ([forge-seal.md](forge-seal.md) — §8's unstruck seal and
the bottom-anchored create control, on the octagon geometry it extracts);
**(6) destructive and notice chrome**
([destructive-chrome.md](destructive-chrome.md) — the §5 severity tones, the
`Cleft` kill geometry, one shared notice panel, and `AngularIndication`
generalized to the component's own shape as §12 already required);
**(7) the launcher mark** ([launcher-mark.md](launcher-mark.md) — Skíðblaðnir
under sail, replacing D4's prow icon, with the whole adaptive-icon set
generated and drift-gated); **(8) the Hlíðskjálf mark**
([hlidskjalf-mark.md](hlidskjalf-mark.md) — §8's stroke rule and legibility
invariants, the mark on every Dwarves surface); **(9) detach chrome**
([detach-chrome.md](detach-chrome.md) — the stock long terminal action hard-cut
to one symmetric, literal `Detach` control using the existing D2/D6 grammar).
The numbers are delta numbers, not positions; D1–D9 each have one owner, and
the roadmap's D-numbers and these agree.
Until a delta lands, this document binds nothing; after it lands, this
document is the review reference for that surface.

## 18. Open questions

- Seal distinguishability at 48dp across the full catalogue (§11) — red test.
- On-panel comfort of `brightBlack #5c6370` for agent dim-text on the S22+
  at night brightness; WCAG bias near black is documented.
- Whether Junicode's subset ships in v0 at all, or waits for the About/
  catalogue view that would use it.
