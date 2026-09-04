# Terminal key deck

Status: implemented 2026-08-28. Both owner proofs were observed red; routine
verification and the signed same-version 54-test S22+ candidate suite are green
on the joined source tree. The current complete 54-test release-bound platform
gate is also green; pairing remained unchanged and the runner restored the
exact public release. Hands-on Gboard, TalkBack, Switch Access, reach, haptic,
and `80 x 5` viewport checks remain `NOT_RUN`. [`architecture.md`](architecture.md) owns
product behavior and acceptance; this document owns the implementation
boundary. [`roadmap.md`](roadmap.md) owns delivery status.
[`terminal-touch-scroll.md`](terminal-touch-scroll.md) supersedes this
document's former blanket gesture non-goal and hard-cuts the lifecycle command
to `ResetInputState`; its source-pinned xterm line-wheel API does not change key
or modifier semantics.

## Outcome

The terminal screen exposes one fixed, aligned input deck:

```text
Esc   /     -     Home   ↑     End    PgUp
Tab   Ctrl  Alt   ←      ↓     →      PgDn
```

The deck is always present on the terminal screen and is disabled whenever the
terminal cannot accept input. It contains no navigation, attachment lifecycle,
destructive action, provider semantic, local history action, or macro. `Detach`,
Android Back, and confirmed Kill remain separate lifecycle controls.

## Rules

- Compose owns presentation and dispatches typed accessories.
- `terminal.js` alone owns modifiers, key encoding, cursor-mode interpretation,
  and the ordered terminal-input reducer.
- The locked WebView validates the exact page protocol. The controller only
  attempt-gates typed dispatch and reset commands.
- Keyboard, IME, composition, dictation-shaped, paste, and deck input retain one
  ordered page ingress and one `Input` egress.
- The terminal remains opaque. No layer parses terminal content or guesses what
  the foreground program wants.
- No key, byte, input source, modifier transition, or terminal content enters a
  log or evidence artifact.

## UI contract

| Row | Visible labels | Spoken labels |
| --- | --- | --- |
| 1 | `Esc / - Home ↑ End PgUp` | `Escape`, `Slash`, `Hyphen`, `Home`, `Up arrow`, `End`, `Page up` |
| 2 | `Tab Ctrl Alt ← ↓ → PgDn` | `Tab`, `Control`, `Alt`, `Left arrow`, `Down arrow`, `Right arrow`, `Page down` |

- `/` and `-` are ASCII. Direction labels are true arrows.
- TalkBack traversal is row-major `0..13` and matches the table.
- Ctrl and Alt expose toggle semantics and state description `Off` or `Armed`.
  Both may be armed. A disabled deck presents both as `Off`.
- Each enabled activation uses system `KEYBOARD_TAP` haptics. A disabled key
  dispatches nothing and produces no haptic.
- Reuse `RaisedSurface`, `NidavellirShapes.Key`, `NidavellirType.Data`, `Bone`,
  `Muted`, `Gold`, and the existing disabled colors and button primitive.
- Outer padding is `4dp`; row and column gaps are `2dp`; every equal-width cell
  is at least `48dp x 48dp`.
- At font scale `1.0`, the minimum complete grid is `356dp x 106dp`. At
  `>=356dp`, all seven columns fill the rail without scrolling.
- Below `356dp`, or when text needs more width at font scale up to `2.0`, both
  rows widen and move together through one shared horizontal scroll state.
  Targets never shrink, rows never wrap or scroll independently, and labels do
  not clip.
- `TerminalScreen`'s existing `imePadding` remains the sole inset owner.

## Capability contract

### Base values

| Accessory | Emitted value |
| --- | --- |
| `Escape` | `ESC` (`0x1b`) |
| `Slash` | `/` (`0x2f`) |
| `Hyphen` | `-` (`0x2d`) |
| `Tab` | `HT` (`0x09`) |
| `Left/Up/Down/Right` | xterm normal or application-cursor sequence |
| `Home/End` | xterm normal or application-cursor sequence |
| `PageUp/PageDown` | `CSI 5~` / `CSI 6~` |
| `Control/Alt` | Toggle only; no input |

Page accessories are raw terminal input. They never scroll local history or
invoke a tmux macro. Gboard Enter continues to emit CR.

### Modifiers and encoding

- Ctrl and Alt are independent one-shot modifiers. A modifier tap toggles only
  itself and never consumes the other modifier.
- The next terminal-emitting unit clears both modifiers before publishing its
  single `Input` message.
- For a proven printable key or literal deck key, apply Ctrl first, then prefix
  `ESC` when Alt is armed.
- Ctrl first uppercases lowercase ASCII; `@` through `_` emit
  `codepoint & 0x1f`; `?` emits `0x7f`; other values remain unchanged.
- With any modifier, arrows, Home, End, PgUp, and PgDn use
  `m = 1 + (Alt ? 2 : 0) + (Ctrl ? 4 : 0)`: Alt `m=3`, Ctrl `m=5`, and both
  `m=7`. Emit `ESC [ 1 ; m A/B/C/D/H/F` or
  `ESC [ 5 ; m ~` / `ESC [ 6 ; m ~` without spaces.
- Ctrl leaves Escape and Tab unchanged. Alt prefixes their base value, so
  Alt+Escape emits `ESC ESC` and Alt+Tab emits `ESC HT`.
- A typed unit is modifier-eligible only when xterm's trusted `onKey`
  provenance proves one discrete printable ASCII key. A deck accessory is
  intrinsically proven.
- IME commit/composition, dictation-shaped input, paste, Unicode,
  multi-character text, and uncertain xterm input remain literal and consume
  both modifiers.

### State and lifecycle

Modifier state belongs to the page and is always published as one atomic Ctrl
and Alt snapshot. On page-port bind, the page publishes `Off/Off` before
`Ready`. On consumption, it publishes `Off/Off` before `Input`.

Reset without input on page failure, disconnect or reconnect, window-focus
loss, backgrounding, rotation or recreation, Kill opening, detach, Back,
navigation, and disposal. Moving focus between the WebView and the deck does
not reset. There is no timeout, persistence, replay, lock, repeat, or recovery
branch.

## Architecture and internal API

```text
TerminalKeyDeck -- TerminalAccessory --> controller attempt gate
                                             |
                                             v
                                    LockedTerminalWebView
                                      | Accessory / reset
                                      v
                              terminal.js ordered reducer
                               | ModifierState | Input
                               v               v
                         locked WebView --> existing WSS
```

Owned Kotlin types and calls:

```text
TerminalAccessory =
  Escape | Slash | Hyphen | Home | Up | End | PageUp |
  Tab | Control | Alt | Left | Down | Right | PageDown

TerminalModifierPhase = Off | Armed
TerminalModifiers = { control: TerminalModifierPhase, alt: TerminalModifierPhase }

TerminalPage.sendAccessory(TerminalAccessory)
TerminalPage.resetInputState()
TerminalPageListener.onModifiersChanged(TerminalModifiers)
```

Native to page:

```json
{"kind":"Accessory","key":"Escape|Slash|Hyphen|Home|Up|End|PageUp|Tab|Control|Alt|Left|Down|Right|PageDown"}
{"kind":"ResetInputState"}
```

Page to native:

```json
{"kind":"ModifierState","control":"Off|Armed","alt":"Off|Armed"}
{"kind":"Input","value":"..."}
```

The page-port version remains `1`: native and packaged page assets ship
atomically and the port is internal. Unknown kinds, keys, states, missing or
extra fields, and malformed values fail closed. There is one protocol shape,
one input path, and no alias, fallback, negotiation, or compatibility decoder.

`TerminalKeyDeck` owns one local two-row item matrix, measured grid width, one
scroll state, semantics, selected rendering, and enabled state. `TerminalScreen`
remembers the latest page-owned snapshot per attempt; it owns no reducer.
`LockedTerminalWebView` parses the exact snapshot without deriving or persisting
modifier truth. `SkidbladnirController` adds no key logic or test seam.

Gateway, WSS, PTY, tmux, session lifecycle, persistence, public API, xterm
version, IME containment, paste sanitization, viewport fitting, and the
terminal queue are unchanged.

## Verification ownership

The change has one proof per ownership boundary:

- real Compose: exact matrix, row-major dispatch/traversal, state, disabled
  behavior, geometry, alignment, shared overflow, and text non-clipping;
- real locked WebView: exact protocol, base/cursor/modifier byte tables, atomic
  ordering, literal uncertain ingress, and reset boundaries;
- physical S22+: the joined deck, WebView, Gboard, accessibility, reach, haptic,
  rotation, and viewport behavior.

Routine verification never substitutes for the signed physical boundary. Any
device or hands-on boundary not run is `NOT_RUN`, never a pass.

## Acceptance

Acceptance requires:

1. The exact fixed matrix, labels, spoken names, traversal, alignment, targets,
   shared overflow, normal-font height, large-text non-clipping, haptics, and
   disabled behavior.
2. Every base and modified key emits its specified value exactly once through
   the existing ordered `Input` route.
3. Page-owned Ctrl/Alt truth is atomic, visible, spoken, independently
   toggleable, jointly consumable, and cleared at every owned boundary.
4. IME, composition, dictation, paste, Unicode, CR behavior, queue bounds,
   focus, viewport containment, CSP, WSS, and tmux behavior remain unchanged.
5. Portrait retains zero horizontal drift and at least `80` columns by `5`
   rows with the deck and Gboard present.
6. Only the final two-row, atomic-modifier API and protocol remain.

## Non-goals

Settings, customization, persistence, profiles, Shift, a separate Meta key,
modifier lock, repeat, popups, long press, macros, snippets, gesture controls
other than terminal touch scroll, trackpads, hardware-keyboard adaptation,
dynamic ordering, content parsing, local-history controls, a native composer,
Kitty/CSI-u, xterm version upgrades beyond the touch-scroll source patch,
telemetry, new logging, gateway/tmux/public-API
changes, and compatibility surfaces.
