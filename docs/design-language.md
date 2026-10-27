# Niðavellir: the Skíðblaðnir design language

This document owns visual identity: color, shape, ornament, typography,
iconography, motion, the terminal theme and the dwarf catalogue's curation. The
[architecture](architecture.md) owns behavior and wins on any conflict; status
copy and tone come from [terminal observation §6](terminal-observation.md#6-content-attention-and-filtering).

The name: *Vǫluspá* 37 places a hall of gold of Sindri's line on Niðavellir,
the "dark fields" — the same poem whose stanzas 10–16 supply the Dvergatal
([Vǫluspá, literal text](https://www.voluspa.org/literal/voluspa.htm)). A hall
of gold on dark fields is this app's entire visual thesis: warm metal accents
on near-black stone. The name appears in docs only; UI text stays literal
([product language](architecture.md#product-language)).

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
   glyphs of text faces. Rounded corners do not exist in this language; corners
   are cut.
4. **Engineering over magic.** The angular Key to Erebor was designed to
   suggest a race relying on engineering and mechanical skill rather than magic
   (same source). Visual honesty is the app's law and the aesthetic reinforces
   it: sampled status is named literally, freshness stays separate, absence is
   displayed rather than guessed, no glow or motion implies unsampled work, no
   ornament reads as an affordance, and errors or destructive copy receive no
   decoration.
5. **Carved, not printed.** Surfaces are rock strata: elevation is a tonal
   step computed from one formula (§5), never a drop shadow. Ornament reads as
   cut into or out of the surface. Motion has mass: stone does not bounce (§12).
6. **One style per component, provenance for every motif.** Blending Viking art
   styles, faking runes onto Latin letters, or sourcing clip-art is exactly
   what separates kitsch from a considered style. Every motif in this language
   has a primary-source anchor in §2, matching the Dvergatal's curation bar
   (§18). A motif without provenance does not ship.

## 2. Provenance ledger

| Motif | Source | Role here |
| --- | --- | --- |
| Gold-on-dark ground | *Vǫluspá* 37, Sindri's hall on Niðavellir | Palette thesis |
| Gold-and-garnet cloisonné | Sutton Hoo shoulder-clasps / Staffordshire Hoard ([British Museum](https://www.britishmuseum.org/collection/object/H_1939-1010-2-a-l)) | Gem fill colors; gold-line-on-dark card grammar |
| Angular key/fret (meander) | Pictish stones, Class II–III edges ([overview](https://en.wikipedia.org/wiki/Pictish_stone)); Greek-key corner methods ([Edkins](https://www.theedkins.co.uk/jo/greekkey/corners.htm)) | The repeating border family (§7) |
| Chevron / V-motif banding | Film-dwarven Art Deco borrowing (Culture on Call, above) | Simplest member of the fret family; dividers |
| Faceted planar surfaces | Iron Hills armor, planar like a cut gemstone ([Nick Keller](https://www.nickkellerart.com/projects/1N6G82)) | Cut-corner shape grammar (§6) |
| Valknut, tricursal | Tängelgårda / Stora Hammars I stones, beside Odin figures ([Valknut](https://en.wikipedia.org/wiki/Valknut)) | Hlíðskjálf mark: Odin's seat overlooking all sessions |
| Ship under one square sail | Gotland picture stones, same corpus as the valknut row: the Viking-age slabs carry a ship under one large sail (Tjängvide I, Alskog — [Gotlands Museum catalogue](https://www.gotlandicpicturestones.se/s/index/item/2052)); the sail itself is unbroken vertical lengths of cloth sewn together ([Viking Ship Museum, Roskilde](https://www.vikingeskibsmuseet.dk/en/professions/boatyard/experimental-archaeological-research/maritime-crafts/maritime-technology/wool-sailcloth)) | Launcher mark: hull, mast, yard, and the sail's three gores (§8) |
| Shield row along the gunwale | Gokstad ship burial, c. 890: 32 round shields to each side, painted alternately yellow and black ([Museum of the Viking Age](https://www.vikingtidsmuseet.no/english/the-collection/the-gokstad-ship/the-gokstad-ship.html)) | Launcher mark: the Garnet row at the sheer (§8). §6 overrides the round shape with its lozenge and §5 the alternating paint with one fill, so the row renders uniform |
| Ship vane | Söderala vane, gilt bronze, c. 1050: worn from use on a ship before it was reused as a church weathervane ([Söderala vane](https://en.wikipedia.org/wiki/S%C3%B6derala_vane)) | Launcher mark: the vane at the masthead, the mark's one asymmetry (§8) |
| Bind-runes on a shared stave | Runestone/coin monograms ([Bind rune](https://en.wikipedia.org/wiki/Bind_rune)) | Dwarf-seal maker's mark (§11) |
| Younger Futhark | Viking-age script, c. 800–1100, Unicode U+16A0–16FF ([Runic block](https://en.wikipedia.org/wiki/Runic_(Unicode_block))) | The only runes that appear, ornament-only (§8) |
| Diminuendo | Insular initial-to-text size decay, Kells/Lindisfarne (Nordenfalk 1947) | Typographic hierarchy rule (§9) |

Rulings, decided once:

- **The border grammar is the angular fret/meander family.** Chevron and
  key-fret are one family and it owns all small chrome. Borre ring-chain keeps
  circles and curvilinear interlace weaves curves; below roughly 24dp of band
  height woven curves turn to mush, while frets have no curves to lose. Neither
  ships; either would need large single-use art and its own provenance review.
- **No zoomorphic terminals anywhere in chrome.** Beast-headed interlace
  (Borre/Urnes vocabulary) is illegible at chip scale and is the highest
  kitsch-risk motif.
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

## 3. Palette basis

Ink `#0C0D0F`, DeepSurface `#15171A`, RaisedSurface `#202329`, Bone `#F3F0E8`,
Muted `#AAA69D`, Gold `#D6A85F`, Ember `#E46C55`, Moss `#76B082` and Frost
`#78A9C6` are defined in `Theme.kt`. Ink sits above the ~`#050505` OLED
black-smear floor; DeepSurface and RaisedSurface fall on Material's
dark-elevation overlay curve from Ink (5% and ~9% white); the four accents fill
the 2–4 accent-hue budget a small app can carry. The Nord palette's
near-identical structure (dark ladder + cool accents) confirms this is the
established Nordic-terminal convention.

## 4. Voice

Labels, domain, error, and destructive copy stay literal. The dwarven voice
lives entirely in geometry, material, and type — never in wording. No "ye
olde" register, no rune-transliterated labels, no themed error messages.
Destructive and error copy names the tmux session and its machine.

## 5. Color

### Surfaces (stone strata)

Elevation is generated, not hand-picked: blend Ink toward white by the
Material dark-overlay percentage for the tier. Never animate through a tone
darker than Ink; never use `#000000` anywhere (OLED smear).

| Token | Hex | Tier |
| --- | --- | --- |
| Ink | `#0C0D0F` | 0dp; app background, terminal background |
| DeepSurface | `#15171A` | ~1dp; cards at rest, dialogs, sheets |
| RaisedSurface | `#202329` | ~4–6dp; raised card elements, chips ground |
| ForgeGlow | `#28231A` | 12dp blended toward Gold instead of white; the Forge sheet and the lit Forge seal (§8) — the app's only firelit surfaces |

### Text and accents

All ratios are WCAG 2.1 against Ink.

| Token | Hex | Ratio | Use |
| --- | --- | ---: | --- |
| Bone | `#F3F0E8` | 17.07 | Primary text |
| Muted | `#AAA69D` | 8.01 | Secondary text |
| Gold | `#D6A85F` | 8.91 | Primary accent; cursor; selected/armed states |
| Ember | `#E46C55` | 6.09 | Error; destructive; `HOT` |
| Moss | `#76B082` | 7.69 | ready; healthy |
| Frost | `#78A9C6` | 7.67 | working; informational |
| Bronze | `#CD7F32` | 6.18 | Warm material accent; no runtime-state meaning |

Status tones draw `working` in Frost, `attention` in Ember and `muted` in
Muted; only ready is Moss, and only while the hub's pending ready holds and the
row is fresh. Pressure history and detail rows keep Normal Moss · Warm Gold ·
Hot Ember · Unknown/missing Muted · Informational Frost. The collapsed pressure
rail is deliberately quieter: labels and `i/N` marks are Muted,
informational/normal values are Bone, and only Warm/Hot values and marks spend
Gold/Ember.

### Severity tones

Severity has exactly one owner (`noticeToneColor` in `Theme.kt`). Ember is not
a colour a surface may reach for; it is what one tone resolves to.

| Tone | Colour | Means |
| --- | --- | --- |
| Failure | Ember | An attempt failed, trust broke, or you are ending something |
| Degraded | Muted | Knowledge is absent or old; nothing is broken |
| Armed | Gold | A recovery is waiting on you |

Two rules follow, and both are load-bearing:

- **Staleness is absence, not failure.** Machine-freshness notices are
  Degraded; stale cards retain their last observed status with motion and
  actions off, but never green readiness. Ember spent on routine staleness is
  Ember spent on nothing — in a federation one host is out often enough that
  the alarm becomes the resting state.
- **Trust events are the loud ones.** A broken bearer or a changed identity is
  Failure even when that machine's inventory is perfectly current, because
  what failed is knowing who we are talking to.

Notice severity, sampled agent status and host pressure have separate semantic
colour owners; surfaces consume those owners rather than assigning raw colours.

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

### Budget

Hard cap: four accent hue families in UI chrome (Gold+Bronze are one
family; Ember; Moss; Frost). New needs are met with fill-variants inside an
existing family. The terminal ANSI table (§10) is exempt: it emulates a fixed
external 8-hue protocol that git/ripgrep/pytest assume exists.

## 6. Shape grammar

- **Corners are cut, not rounded.** `CutCornerShape` everywhere a radius
  would exist. Facet unit: cards 10dp cut; chips and keys 4dp; sheets 12dp on
  the top corners only. All cuts are 45°.
- **One shape is allowed to disagree with itself.** `Cleft` keeps the 4dp
  chip facet on three corners and cuts 14dp on the fourth (top-start). It is
  the only asymmetric shape in the product and it is reserved to the commit
  button of a destructive confirmation, which is what makes it legible as
  meaning rather than as a mistake: detach and close must stay visibly
  different in greyscale, and §15 forbids the icon that would otherwise carry
  it. A second asymmetric shape would spend the distinction, so adding one
  reopens this clause.
- **Portrait frames are octagons.** A square with all corners cut at 29% of
  the side is an octagon; circles are elvish in this grammar. The exactly
  regular cut is (2−√2)/2 ≈ 29.29%; 29% is the shipped round number, which puts
  each vertex 0.16dp from its regular position on a 56dp frame and leaves the
  axis edges 2.4% longer than the diagonals (23.52dp against 22.97dp). Both are
  below the perceptual floor for a hairline outline, and not worth re-cutting
  every seal already struck. Whatever draws the octagon reads the same constant
  the clip does, so a frame and its clip can never disagree.
- **Activity facets are cut, never badged.** The card's status line leads with
  a 12dp facet in the cut-corner chip geometry. Activity has no unread marker
  or lozenge.
- **Faceting replaces gradients.** Where a surface needs richness, split it
  into 2–3 flat planes differing by one elevation step — the faceted-gemstone
  armor rule — never a smooth gradient or soft shadow.
- **Strata edges.** A card may carry a single 1dp inner hairline in Gold at 25%
  opacity along its top edge — light catching the carved lip — and nothing
  else. No outer glows, no multi-edge strokes.
- The octagon is `CutCornerShape(29%)` on a square and the lozenge a rotated
  square; no geometry dependency ships.

## 7. Ornament system

The fret band — chevron bands, key patterns, step patterns — is the only
repeating ornament. Rules: monoline stroke, 45°/90° turns only; band height ≤
16dp; drawn in Muted or Gold at ≤ 40% opacity so it never competes with text;
corners resolve by the reversal-at-midpoint method, which requires an even
count of pattern units per side. It ships once: under the Forge sheet's title,
Gold 40%, whole units centred.

The fret, valknut and seal tables are frozen constants in `Ornament.kt`, and
the launcher is frozen in `res/`. They were computed once by
`scripts/gen-ornament` (last present at `12b1516`); a change is a hand edit or a
regeneration from git history. No gate checks legibility, mask envelope or
drift, so the invariants in §8 and §11 are checked by hand. Topology is never
computed on the phone, and no third-party knot code is vendored.

## 8. Iconography and runes

- **The Hlíðskjálf mark** is the tricursal valknut: three interlocked
  triangles, Borromean topology — the seat that overlooks every world —
  drawn as twelve frozen straight segments with its six crossings baked as
  breaks, three of them interior. Its stroke is 0.055 of the mark's size,
  never a fixed dp, because a fixed stroke is a larger share of a smaller mark
  and closes the crossings as the mark shrinks. Legibility is therefore a
  property of the geometry at every size: every strand (shortest 0.084) is at
  least one stroke long and every break (0.1997) at least two strokes wide. It
  leads `Dwarves` (24dp Gold), leads `Back to Dwarves` on the terminal's
  reconnect panel (18dp in the button's content colour, so it dims with the
  button) and marks the empty hall (48dp, Muted at 40%). One composable draws
  it. It never marks detach, which names what happens to the session, not
  where the button goes, and never the launcher.
- **The launcher mark** is Skíðblaðnir under sail, the ship that takes a fair
  wind once its sail is raised (*Gylfaginning* 43), not a dragon-prowed
  longship. Cut stems are integral to the hull polygon (separate hooks read as
  a face); a square sail in three gores; a Garnet shield row at the sheer; a
  masthead vane is the one asymmetry. Straight polygons on the 108-unit
  adaptive canvas, authored in the launcher resources and never traced. Its
  floor is stated in units because it has no stroke and a known size range:
  every gap and mass width is at least 3 units (2dp at the smallest 48dp
  launcher; tapering apexes exempt), and every point lies within 34 units of
  centre (the circular mask clips at 36; the yard's top corners sit at 33.97).
  The monochrome layer is its own drawable whose separations are geometric
  (the shields become notches cut into the sheer), because one flat tint
  erases colour. The foreground's 1.7-unit gap between shields is knowingly
  below the floor; colour (Gold under Garnet, 4.73:1) carries it. There is no
  `roundIcon` and no density mipmap; the label is `Skíðblaðnir`.
- **Runes are ornament, never text.** The dwarf seal's bind-rune (§11) is a
  *drawn* monogram, strokes merged on one shared stave per the historical
  construction — a merged bind-rune cannot be produced by placing sequential
  font glyphs. No rune font ships. Never transliterate labels, never
  rune-encode meaning a user must read.
- **The unstruck seal** is the create mark, on the Forge seal: a 56dp octagon
  around a bare stave crossed by one centred horizontal bar, and nothing else.
  It is honest: a dwarf is assigned after creation, so the client cannot draw
  the seal it is about to make, and the blank is the only true picture. It
  cannot read as a rune: no segment of the Younger Futhark table is
  horizontal, and since íss is the bare stave, the crossbar is the whole
  argument. Lit (some machine can act): ForgeGlow field, Gold frame and mark.
  Cold: DeepSurface field, Muted mark at §12's disabled content alpha; field
  and hue change, never opacity alone, and the flip does not animate. The cold
  mark's 2.13:1 over its field is intended for an inactive control. Strokes are
  fixed dp (frame 1.5, mark 3), so the control is not reused below about 40dp;
  its frame is stroked unclipped because no mineral ground hides a clipped
  half. Icon-only, described `New dwarf`.
- **No horned helmets, drinking horns, axes, or hammer clip-art.** The horned
  helmet is a 19th-century invention with zero archaeological support; the
  rest is tourist-shop register. The create action takes no hammer either:
  provenance is not the objection — Skíðblaðnir and Mjölnir come from one
  wager (*Skáldskaparmál* 35) — but Skíðblaðnir loses that wager, Mjölnir
  marks the wielder rather than the smith, and the silhouette dies at 24dp
  monoline. The register is the objection.

## 9. Typography

Two bundled faces, both SIL OFL 1.1 (`res/font/`), plus the system face.
Every face guarantees ð/þ.

| Role | Face | Use | Never |
| --- | --- | --- | --- |
| Display | Big Shoulders (condensed industrial gothic; caps, +4% tracking, weight 600–700) | Wordmark `SKÍÐBLAÐNIR`, screen titles, dwarf signatures on cards | Body copy, sentences |
| UI body | System Roboto (quiet, zero bytes, Android-native) | All running text, labels, buttons, errors | Display duty above `titleLarge` |
| Data | JetBrains Mono | Status lines, freshness, cwd paths, session IDs, key-deck labels, pressure numerals — every machine fact | Prose |

The Display role's caps direction applies only where the source is caps, such
as the wordmark and applicable headings. Dwarf signatures preserve exact
catalogue casing; group labels keep their authored case in body or data type.

- The choice *against* Cinzel/Trajan-likes is deliberate: lapidary Roman caps
  are the fantasy-app default and read imperial Rome, not the forge. Big
  Shoulders' flat terminals and condensed verticals read stamped metal and
  echo the fret grammar. The choice against decorative "Viking" faces is
  licensing and register (§2).
- **Machine facts speak mono.** The dashboard borrows the terminal's own
  voice for everything tmux/proc reported — a quiet, honest bridge between
  the two surfaces.
- **Work-first hierarchy** on the card: tmux name (data, largest and highest
  contrast), literal status with a quieter display-face dwarf signature,
  conditional notices/objective, then profile/host/directory (data, ≥11sp).
  Readable status and full dwarf names may wrap.
- Berkeley Mono is explicitly unusable (its standard tiers exclude terminal
  apps). Norse by Joël Carrouché is unusable (embedding rights ambiguous).

## 10. Terminal theme

`terminal.js` uses a derived ANSI table: **bright slots are the four brand
accents verbatim** (bold=bright is the emphasis signal git/pytest/ripgrep rely
on — the reason Nord's bright=base collapse is rejected); **base slots are the
same hues systematically darkened** (S−6pp, L≈44%, with red/blue/magenta lifted
to L52/50/54 because those hues fail 4.5:1 on true near-black at the flat
recipe); magenta and cyan are hues that exist only here — git's
`--color-moved` load-bears them. All base slots ≥ 4.5:1 and all brights ≥ 6:1
against Ink. `brightBlack` is intentionally recessive and `black` is the
DeepSurface fill slot.

- `cursor` (Gold), `cursorAccent` (Ink), `selectionBackground` (Bone 30%) and
  `overviewRulerBorder` (Muted) must be set explicitly: xterm.js falls back to
  hardcoded `#ffffff`/`#000000` for them, not to the theme's own colors.
  `selectionForeground` stays unset so glyph SGR colors stay visible when
  selected.
- `extendedAnsi` remaps the 24-step grayscale (232–255) as a linear Ink→Bone
  interpolation, so 256-color tools (bat, delta, rich-based TUIs) stay
  on-palette. xterm consumes `extendedAnsi` as one contiguous array anchored at
  index 16, so it ships as a sparse 240-slot array with entries only at offsets
  216–239; the 6×6×6 cube stays library default.
- `minimumContrastRatio: 3` is a safety net for arbitrary truecolor agent
  output; the designed table already exceeds it, so it never mutates designed
  colors. xterm.js halves the requirement for SGR-dim cells by design.
- `fontFamily` is the vendored JetBrains Mono.
- WCAG's formula has a documented bias near black (APCA critique), so these
  ratios are floors, not proof of comfort; spot-check on a physical panel.

## 11. Dwarf seals (procedural portraits)

A seal is a pure function of `character.key`: the octagonal frame (§6), a
mineral field, a bind-rune maker's mark and angular figure geometry. No colour
travels on the wire. With `h = SHA-256(UTF-8(key))` as unsigned bytes:

- Mineral `h[0] mod 8` (one of 8 fill-only tones: Garnet, Hematite, BlueGlass
  and 5 desaturated stones);
- Rune count `2 + (h[4] and 1)`; runes the first distinct `h[5+i] mod 16` for
  `i` in 0..7, stopping at the count (a shorter list is defined, not an error);
- Beard teeth `3 + h[8] mod 4`; beard depth `h[9] mod 4`;
- Facet mask `h[12]` (bit `e` gilds octagon edge `e`);
- Metal Gold when `h[14]` is odd, else Bronze.

Changing a slice re-rolls every visible identity: a design event, never a
tweak. Check values (mineral, runes, teeth, depth, mask, metal):
`norse.modsognir` 6, [12, 9], 6, 1, 229, Bronze; `norse.durinn` 4, [9, 10], 3,
0, 24, Bronze; `tolkien.gimli` 4, [5, 10], 3, 0, 239, Gold. Over the 101
catalogue entries every seal is distinct and (mineral, runes) alone never
collides; nothing checks a new entry.

Draw order is frozen: mineral, two facet planes, beard, the bind-rune on one
shared stave, the frame (grey, then Gold on masked edges), the Bone initial. A
card's seal is 36dp, clipped to the octagon; the initial is fixed decorative
lettering (12dp glyph, 16dp line, 6dp inset) while the neighbouring name
scales. Seals at 36dp are not individually distinguishable: recognition is
seal plus names. A seal has no state, cache or meaning; the name beside it is
the text. No raster portrait, manifest or rights claim ships.

## 12. Motion and interaction states

- **Stone has mass.** Nothing overshoots; nothing wobbles. Effects motion
  (color, opacity, state layers) uses `tween(100ms, EasingStandard)`.
- **State layers.** Pressed is Bone at 10% over the component's surface.
  Disabled is 38% content / 12% container, plus one non-opacity cue (drop the
  chip hairline; desaturate the seal).
- **The press flash is angular.** The circular ripple is off-grammar. One
  shared `IndicationNodeFactory` flashes an inset copy of the component's own
  cut-corner outline at the pressed alpha, used everywhere skid draws its own
  control. Material buttons, text fields and filter chips keep their platform
  ripple.
- **Status motion is literal and bounded.** Only a fresh working local agent
  may turn the status facet's notch. Stale cards, other states and
  reduced-motion mode are static. The label stays fixed; no pulse, glow,
  countdown or unread-result motion exists.
- **The forge breathes once.** Exactly one ambient decorative animation is
  budgeted in the whole app: the Forge sheet warms from DeepSurface to
  ForgeGlow over 400ms when opened. The status facet is state-bound, not
  ambient ornament. Everything else moves only when state moves.

## 13. Component register

- **Session card**: DeepSurface, 10dp cut corners, Gold lip hairline at 25%,
  36dp seal. Its verbs sit behind a drawn three-stud overflow whose menu is
  RaisedSurface, card cut, shadowless, with Ember destructive rows after a
  hairline rule.
- **Status line**: unboxed; the status facet, then the literal status label,
  bold in its tone, and an optional Muted detail, Data face ≥ 11sp. No
  transition age or unread signal; the evidence qualifier is not drawn on
  cards. A plain pane reads `terminal`.
- **Status facet**: 12dp chip shape, solid status colour, no border, text or
  semantics, inline as the status line's first glyph.
- **View strip**: each tab is a 48dp-minimum target centring a 32dp-minimum
  field; selected is RaisedSurface with a 1dp Gold border and Gold text,
  inactive is transparent with Bone text, both in the same weight so selection
  never changes width.
- **Refresh line**: transient feedback spends no resting space, so the refresh
  mark is an active-only 2dp Gold line with butt ends, inset 12dp at the
  collection's top edge, with no track, surface, elevation, glow, ornament or
  reserved height: determinate while pulling, indeterminate while the pulled read
  runs, and a complete static band when motion is off. It
  is less conspicuous than Material's circle, an accepted cost.
- **The Forge sheet**: ForgeGlow surface, 12dp top cuts, the fret band under
  the Display-face title `Create dwarf`, mono cwd field. Error text is plain
  Ember body — no ornament near errors (§1.4).
- **Terminal rail**: the terminal's only top chrome. One DeepSurface stratum
  carried under the status bar, one row of at least 48dp: Gold body-face
  `Detach`, then the Data-face tmux name over a Muted `place · presence` line
  (Ember only for frozen input) and a trailing Gold `⋯`, all one disclosure
  control. No chips, hairlines or ornament; the tonal step alone separates it
  from the terminal.
- **Session sheet**: the standard 12dp top-cut DeepSurface sheet. Data-face
  facts, then the session card menu's rows; the two closures are Ember text
  without ornament and commit only through the Cleft confirmation.
- **Terminal key deck**: RaisedSurface bezel, keys 4dp cut, mono labels, equal
  48dp-minimum cells with 2dp grid gaps. An armed modifier uses Gold 18% fill
  and a Gold label. A RaisedSurface band continues the bezel through the
  navigation-bar or keyboard inset; the terminal between rail and deck is Ink.
- **Pressure rail**: RaisedSurface card, 10dp cuts, one split-colour header,
  one horizontally scrolling non-wrapping flat Data-face metric row, then the
  categorical history band with no title. The machine name is Bone; the
  header state is Muted for normal, unknown, stale and unavailable, Gold for
  reading and warm, Ember for hot; recovering takes its level's colour.
  Metrics have no fill, border, shape, separator, or nested action; the whole
  rail is one disclosure control. The history band is frozen: 16dp tall, 5dp top
  pad, bar heights normal .25, warm .58, hot 1, unknown .42. The details sheet
  keeps the 12dp top-cut sheet and panel language, with no second band.

## 14. Accessibility floor

48dp targets and 8dp spacing where the architecture demands; the terminal key
deck's dense aligned grid uses its 2dp gaps without shrinking targets;
`minimumInteractiveComponentSize()` where a drawn control is smaller. Text ≥
11sp. Every text/surface pair in this document ≥ 4.5:1 except the two declared
fill/de-emphasis slots (ANSI black, brightBlack), which never carry prose, and
the cold Forge seal (§8). Reduced-motion fallbacks per §12. Ornament is always
`contentDescription`-silent decoration.

The phone has no screen-reader support (owner cut). It ships no curated
narration, custom actions, roles, headings, live regions, traversal order,
state descriptions or terminal wheel actions; TalkBack gets only Compose's
merged visible text. xterm's `screenReaderMode` stays on only because it
selects the IME input path the phone relies on; with it off, Gboard lost the
first letter after the keyboard opened in 4 of 72 emulator runs. Labels carry
no bidi isolates, so an RTL machine, group or folder name may reorder
neighbouring punctuation; paths keep their explicit left-to-right direction.
Drawn controls with no text keep a `contentDescription`: the overflow studs and
the Forge seal.

## 15. The forbidden list

Never: rounded corners; circular ripples on skid-drawn controls; drop shadows;
smooth decorative gradients; `#000000`; zoomorphic chrome; Elder Futhark;
rune-transliterated UI text; blackletter; Tolkien scripts;
horned-helmet/axe/horn/hammer clip-art (§8); Cinzel-register imperial caps; more than one ambient
animation; ornament on error or destructive surfaces; unlicensed or GPL knot
code; fonts outside §9's table without a license entry here.

## 16. Asset licenses

| Asset | License | Status |
| --- | --- | --- |
| Big Shoulders, JetBrains Mono | SIL OFL 1.1 | Bundled; the APK carries their license texts in `assets/licenses/`, as the OFL requires |
| jdenticon (channel-slicing model) | MIT (LICENSE verified) | Model only; own implementation (§11) |
| Berkeley Mono, Norse (Carrouché), Cirth fonts, Book of Kells facsimile imagery, WETA/film artwork | Restricted or unusable | Never embed |

## 17. Terminal browser

The desktop browser is this language with every ornament deleted (§1.2), drawn
in the operator's own terminal. The operator's theme supplies the ground and
default ink; the browser paints no background. Its behavior is the
[architecture's](architecture.md#browser).

| Intensity | SGR | Carries |
| --- | --- | --- |
| Reverse | 7 | The `skid` wordmark; the form caret |
| Bold | 1 | The cursor row's name, the current view's label, keys, page titles, the rule's target |
| Plain | — | Everything that must be read: names, status words, key labels, notices, the exclusive `needs input` view label |
| Faint | 2 | What may recede: group headings/cells, provider/profile/model text, every row's machine/path, omission markers, the rule, names of unavailable and checking rows, a checking row's `checking` cell, form chevrons |

| Accent | Slot | Its one meaning |
| --- | --- | --- |
| Gold | Bright yellow | You are here: the cursor bar `▌`, the focused form field and the current view's `‹ ›` at the table |
| Ember | Bright red | A request or error status, a failed outcome, the stop/kill question |
| Frost | RGB `#78A9C6` | starting, working |
| Moss | RGB `#76B082` | ready |
| Muted | RGB `#AAA69D` | Every other status |

Status colour follows the projected tone (§5), never the printed word. A stale
row's status cell reads Muted `last observed: <label>` and is never ready, even
while its failed host is re-read; a row of any other host being re-read reads
faint `checking`; both names recede faint. Ember never appears in hints, so
requests and shown errors stay the table's only red statuses and are found at a
glance. Colour never carries meaning alone: every status is a word; the cursor
is a glyph plus bold; the current view is bold, between Gold `‹ ›` at the table;
form chevrons stay faint because the `▌` gutter marks focus. `NO_COLOR` strips
styling and preserves every label. Faint varies by terminal and disappears
under mosh. Section and session hierarchy and omission survive complete style
stripping; one content line identifies each session, with gaps only at group
boundaries. One rule separates content from controls. No boxes, fret, runes,
icons or motion.

## 18. Dvergatal curation

`internal/catalog/characters.json` is itself the inventory: every entry carries
its key, display name, tradition, and source work and locus. These are the
admission rules behind it.

### Acceptance for good content

An entry is admissible only when a primary text names the individual and either
calls the individual a dwarf/Nibelung or places the individual in an explicitly
dwarven company. The source citation is the shortest stable work plus chapter,
scene, or stanza that proves that fact. A name is not admitted from a fan wiki,
adaptation-only list, generic race label, disputed attribution, or an inferred
translation.

The Old Norse set uses the named individuals in the `Dvergatal` of *Vǫluspá*
stanzas 10–16. The repeated `Eikinskjaldi` in stanzas 13 and 16 is one figure
and ships once. Lofarr ships once because stanza 14 places him at the end of
Dvalinn's dwarven kindred and stanza 16 names that lineage as Lofarr's. The
Tolkien set uses distinct textual characters from *The Hobbit*, *The Lord of
the Rings* Appendix A, *The Silmarillion*, and *The Children of Húrin*. Display
suffixes such as `of Erebor` and `Ironfoot` are curatorial disambiguators, not
additional figures. Tolkien's `Mîm` and Wagner's `Mime` are separate authored
characters and are retained under their separate works; neither is a
spelling-variant duplicate of the other.

The Germanic-operatic set is limited to named Nibelung dwarfs in Wagner's own
*Der Ring des Nibelungen* libretti: Alberich and Mime. Other Nibelungs are an
unnamed collective, and giants, humans, gods, dragons, and stage-only names are
excluded.

Keys use the catalogue's ASCII asset-safe grammar. Display names preserve a
single normalized source spelling or an explicit primary-text epithet and are
globally unique. `internal/catalog` embeds the file and panics at process start
on an invalid or duplicate key; nothing checks the file before release. The
other rules here (unique NFC display names, no `FairyTale` or `ModernMedia`
tradition, one primary-text locus per entry) are applied by review; no gate
re-checks them.

### Primary sources

- *Vǫluspá*, Codex Regius text, sts. 10–16. For a convenient transcription and
  translation see [Vǫluspá, literal text](https://www.voluspa.org/literal/voluspa.htm);
  the controlling scholarly editions are Ursula Dronke, *Poetic Edda II:
  Mythological Poems* (Oxford, 1997), and Edward Pettit, *The Poetic Edda: A
  Dual-Language Edition* (Oxford, 2023).
- J. R. R. Tolkien, *The Hobbit* (1937), ch. 1; *The Lord of the Rings*
  (1954–55), Appendix A III, “Durin's Folk”; *The Silmarillion* (1977), ed.
  Christopher Tolkien, chs. 16 and 20; and *The Children of Húrin* (2007), ch.
  11, “Of Mîm the Dwarf”. These are the primary authored texts, not adaptation
  guides.
- Richard Wagner, *Der Ring des Nibelungen* libretti (1848–74), *Das
  Rheingold*, Scene 3, and *Siegfried*, Act I. The [Naxos Siegfried
  libretto](https://www.naxos.com/libretti/660175.htm) is a convenient
  publisher-hosted text; the German libretto is authoritative for the
  character classification.

### Dedupe and exclusion record

- `Eikinskjaldi` appears twice in *Vǫluspá* but is one shipped Old Norse entry.
- `Lofarr` is named through the lineage clauses in stanzas 14 and 16 and is
  retained as the terminal member of the explicitly dwarven company.
- Diacritics are normalized, not multiplied into variants: each Old Norse
  source spelling appears once.
- Tolkien entries with names cognate with Old Norse names are separate authored
  characters only where the Tolkien text identifies a distinct individual;
  their display suffixes make the global display-name set unambiguous.
- `Mîm` and Wagner's `Mime` are separate characters in separate primary works;
  their source/locus and display forms remain distinct.
- No `FairyTale`, `ModernMedia`, generic Nibelung, unnamed dwarf, disputed dwarf,
  giant, dragon, human, god, elf, or invented catalogue filler is included.
