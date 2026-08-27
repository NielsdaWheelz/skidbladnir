# Design delta D1: terminal theme

Status: implemented 2026-08-26; routine verification green on the
federated tree; the 33-test instrumented suite green on the physical S22+
(devbox debug-signed run — the signed deviceDebug platform gate is
MacBook-owned); the hands-on OLED comfort check stays `NOT_RUN`.

[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`design-language.md`](design-language.md) §10 owns the palette values and
their derivation; this document owns the implementation boundary and delivery
plan. [`roadmap.md`](roadmap.md) owns ordering.

## Outcome

The phone terminal renders agent output in the Niðavellir ANSI/ITheme table
and vendored JetBrains Mono instead of today's One Dark table and generic
`monospace`, with the bundle-only CSP posture, input semantics, geometry
guarantees, and key-deck behavior unchanged.

## Goals and rules

- Replace inherited defaults with the derived table; change nothing else on
  the page. The WSS protocol, page↔native messages, key deck, IME
  containment, and 80-column guarantee are untouched.
- Fonts are vendored pinned assets under the existing `terminal.lock` digest
  regime, exactly like xterm itself: upstream release bytes, no local
  subsetting or re-shaping, license file alongside.
- The CSP stays deny-by-default; the single change is `font-src 'none'` →
  `font-src 'self'`. Scripts remain bundle-only; every
  exfiltration-capable resource class stays denied.
- Glyph metrics must be settled before geometry is *published to native*:
  the page requests the vendored faces and gates the first geometry
  publication on that load, so the 80-column portrait guarantee is measured
  in the real font, not a fallback. Local pre-handshake fits are harmless —
  they publish nothing.

## Capability contract

### Theme

`terminal.js` constructs the terminal with exactly this theme (values are
design-language §10; all base slots ≥ 4.5:1 on Ink, verified there):

```js
theme: {
  background: "#0c0d0f", foreground: "#f3f0e8",
  cursor: "#d6a85f", cursorAccent: "#0c0d0f",
  selectionBackground: "#f3f0e84d",
  selectionInactiveBackground: "#f3f0e826",
  overviewRulerBorder: "#aaa69d",
  black: "#15171a", red: "#d74e33", green: "#4f925c", yellow: "#ac7e35",
  blue: "#538bac", magenta: "#bb5897", cyan: "#459c93", white: "#aaa69d",
  brightBlack: "#5c6370", brightRed: "#e46c55", brightGreen: "#76b082",
  brightYellow: "#d6a85f", brightBlue: "#78a9c6", brightMagenta: "#cd70ab",
  brightCyan: "#64c4ba", brightWhite: "#f3f0e8",
  extendedAnsi: EXTENDED_ANSI,   // sparse 240-slot array, see below
}
```

- `selectionForeground` and the three `scrollbarSlider*` fields stay unset:
  unset selection foreground preserves per-glyph SGR colors under selection,
  and the sliders derive from `foreground` in the library.
- `EXTENDED_ANSI`: the vendored xterm consumes `extendedAnsi` as ONE
  contiguous array anchored at ansi index 16 (`ansi[s+16] =
  v(extendedAnsi[s], DEFAULT[s+16])`, with `undefined` entries falling back
  to library default). It is therefore a 240-slot array whose offsets 0–215
  (the 6×6×6 cube, indices 16–231) are `undefined` and whose offsets
  216–239 (grayscale indices 232–255) carry the remap: linear per-channel
  sRGB interpolation from Ink `#0c0d0f` (index 232) to Bone `#f3f0e8`
  (index 255), rounded integer channels, computed inline. A naive
  24-element array would recolor cube indices 16–39 and leave the real
  grayscale untouched — the exact opposite of the intent; the red proofs
  below pin against this.
- Options: `minimumContrastRatio: 3` (safety net for arbitrary truecolor
  agent output; the designed table already exceeds it, so designed colors are
  never mutated), `fontFamily: '"JetBrains Mono", monospace'`. `fontSize`,
  `cursorBlink`, `scrollback`, `screenReaderMode`, and the existing adaptive
  glyph-scale logic are unchanged.

### Fonts

- Vendored under `android/app/src/main/assets/terminal/vendor/`:
  `JetBrainsMono-Regular.woff2` and `JetBrainsMono-Bold.woff2` byte-for-byte
  from the upstream release zip's `fonts/webfonts/` (v2.304 at drafting),
  plus its `OFL.txt` as `JetBrainsMono-2.304.LICENSE` — the lock's
  `<package>-<version>.LICENSE` convention; all three added to
  `android/terminal.lock` (`scripts/check-terminal-assets` enforces set
  equality plus digests).
- `terminal.css` declares two `@font-face` rules, family `JetBrains Mono`,
  weights 400 and 700, `font-display: block`. SGR bold resolves through
  xterm's default `fontWeightBold: 'bold'` to the real bold file.
- Page ordering: the page explicitly requests both faces with
  `document.fonts.load(...)` (regular and bold — a bare `fonts.ready` can
  resolve without ever requesting the font) and gates the first geometry
  *publication* — the existing `acceptPagePort`/`resizeTerminal` publish
  gate, the only path that sends geometry to native — on that load settling.
  Pre-handshake local `ResizeObserver`/window-resize fits may still run and
  are harmless: they publish nothing. On load failure the page proceeds with
  `monospace` — honest degradation, never a blocked terminal.
- The vendored `xterm-6.0.0.css` hard-codes `font-family: monospace` on the
  screen-reader accessibility tree; that tree is non-visual and the vendored
  file is not modified — accepted, noted here so review does not rediscover
  it.

### CSP

The `index.html` CSP meta changes one directive: `font-src 'none'` →
`font-src 'self'`. Every other directive is byte-identical.

## Hard cut

Delete the One Dark hex table entirely. No theme option, toggle, alternate
palette, or compatibility path remains.

## Work split

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Terminal page | Android terminal-boundary builder | `android/app/src/main/assets/terminal/terminal.js`, `terminal.css`, `index.html`, `vendor/JetBrainsMono-*`, `android/terminal.lock`, `TerminalInstrumentedTest.kt` (one expectation literal, below) | Lock/red proofs below |
| Verification | Read-only verifier | none | Review only |

No production Kotlin, gateway, tmux, or protocol file changes. One existing
instrumented test couples to the theme table:
`TerminalInstrumentedTest.trueColorEscapeSequenceProducesColoredPixels`
asserts SGR-31 indexed red renders `rgb(224, 108, 117)` (the One Dark red).
That assertion is this delta's ready-made red: it fails on the table swap,
and green updates the expectation to `rgb(215, 78, 51)`; its truecolor
assertions are theme-independent and unchanged. Any other need outside these
paths reopens scope.

## Red / green / refactor

Red (observed failing before implementation):

1. `scripts/check-terminal-assets` fails on the added-but-unlocked font files
   (set-equality red), then passes only with correct digests.
2. Platform instrumented proof: the existing indexed-red assertion fails
   against the new table (see work split), and a rendered ANSI bright-yellow
   cell matches Gold `#d6a85f` with a Gold cursor — fails against One Dark.
3. Platform instrumented proof: a cell colored by 256-color index 244
   (mid-grayscale) renders the remapped Ink→Bone tone, and a cell colored by
   cube index 21 renders the library-default cube color — fails under the
   naive contiguous-24 `extendedAnsi` array (which would recolor the cube
   and leave grayscale untouched).
4. Platform instrumented proof: reported portrait columns are ≥ 80 and
   *stable* across page load with the vendored font — fails if geometry is
   published before glyph metrics settle.

Green: implement only the contract above; routine `scripts/test verify`
(static, build, unit) stays green and invokes no device.

Refactor: none expected; the change is a table, two font files, one CSS
addition, one CSP directive, and one boot-ordering await.

## Acceptance and gates

Routine verification, plus one separately approved Android platform pass
covering the instrumented proofs above and re-running the existing rendered-
color, viewport-containment, and key-deck regressions. Hands-on: one S22+
look at real agent output (Codex + Claude) for dim-text comfort
(`brightBlack #5c6370`) and 256-color tool output (`bat`/`delta`-style)
staying on-palette; WCAG's known near-black bias makes this a named check,
not an assumption. No integration or live gate: transport is unchanged.

## Non-goals

WebGL/canvas renderer addons, remapping ANSI cube 16–231, Nerd Font patching
or ligature configuration, theme switching, Compose-side fonts (D2), key-deck
styling (D2), and any gateway/tmux/protocol work.
