# Terminal key deck

[terminal shift](terminal-shift.md) owns the shift extension, implementation
plan and current qualification. the contracts below include that extension;
historical qualification stays attributed to its original source.

The [readable-sizing target](terminal-readable-sizing.md) supersedes the
80-column viewport requirement and earlier packaged-page versions. Key/deck
behavior remains; historical evidence below does not prove the new sizing.

historical status, 2026-08-28: both owner proofs were observed red; routine
verification and the signed same-version 54-test S22+ candidate suite are green
on the then-joined source tree. the complete 54-test release-bound platform
gate is also green; pairing remained unchanged and the runner restored the
exact public release. Hands-on Gboard, TalkBack, Switch Access, reach, haptic,
and `80 x 5` viewport checks remain `NOT_RUN`. [`architecture.md`](architecture.md) owns
product behavior and acceptance; this document owns the implementation
boundary. [`roadmap.md`](roadmap.md) owns delivery status.
[`terminal-touch-scroll.md`](terminal-touch-scroll.md) supersedes this
document's former blanket gesture non-goal and hard-cuts the lifecycle command
to `ResetInputState`; its page-owned wheel routes do not change key or
modifier semantics.

## Outcome

The terminal screen exposes one fixed, aligned input deck:

```text
Esc   /     Shift Home   ↑     End    PgUp
Tab   Ctrl  Alt   ←      ↓     →      PgDn
```

The deck is always present on the terminal screen and is disabled whenever the
terminal cannot accept input. It contains no navigation, attachment lifecycle,
destructive action, provider semantic, local history action, or macro. `Detach`,
Android Back, and the session sheet's confirmed closures remain separate lifecycle controls.

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

| Row | Visible labels |
| --- | --- |
| 1 | `Esc / Shift Home ↑ End PgUp` |
| 2 | `Tab Ctrl Alt ← ↓ → PgDn` |

- `/` is ASCII. direction labels are true arrows.
- keys carry no spoken names, traversal order or state descriptions
  ([design language §14](design-language.md#14-accessibility-floor)).
- ctrl, alt and shift each render off or armed (the Gold treatment). all eight
  combinations are valid; disabled modifiers render off.
- Each enabled activation uses system `KEYBOARD_TAP` haptics. A disabled key
  dispatches nothing and produces no haptic.
- Reuse `RaisedSurface`, `NidavellirShapes.Chip`, `NidavellirType.Data`, `Bone`,
  `Muted`, `Gold`, and the existing disabled colors and button primitive.
- Outer padding is `4dp`; row and column gaps are `2dp`; every equal-width cell
  is at least `48dp x 48dp`.
- the minimum target footprint is `356dp x 106dp`; measured content can require
  more width, including five-letter `Shift` at normal font scale.
- below the measured content width, including font scale up to `2.0`, both
  rows widen and move together through one shared horizontal scroll state.
  Targets never shrink, rows never wrap or scroll independently, and labels do
  not clip.
- `TerminalScreen` remains the sole inset owner ([terminal chrome](terminal-chrome.md)).

## Capability contract

### Base values

| Accessory | Emitted value |
| --- | --- |
| `Escape` | `ESC` (`0x1b`) |
| `Slash` | `/` (`0x2f`) |
| `Tab` | `HT` (`0x09`) |
| `Left/Up/Down/Right` | xterm normal or application-cursor sequence |
| `Home/End` | xterm normal or application-cursor sequence |
| `PageUp/PageDown` | `CSI 5~` / `CSI 6~` |
| `Control/Alt/Shift` | toggle only; no input |

Page accessories are raw terminal input. They never scroll local history or
invoke a tmux macro. Gboard Enter continues to emit CR.

### Modifiers and encoding

- ctrl, alt and shift are independent one-shot modifiers. a modifier tap toggles
  only itself and never consumes another modifier.
- the next terminal-emitting unit clears all three before publishing its
  single `Input` message.
- For a proven printable key or literal deck key, apply Ctrl first, then prefix
  `ESC` when Alt is armed.
- Ctrl first uppercases lowercase ASCII; `@` through `_` emit
  `codepoint & 0x1f`; `?` emits `0x7f`; other values remain unchanged.
- with any modifier, arrows, home, end, pgup and pgdn use
  `m = 1 + (Shift ? 1 : 0) + (Alt ? 2 : 0) + (Ctrl ? 4 : 0)`.
  emit `ESC [ 1 ; m A/B/C/D/H/F` or
  `ESC [ 5 ; m ~` / `ESC [ 6 ; m ~` without spaces.
- shift changes deck tab to backtab `ESC [ Z`; ctrl leaves tab/backtab unchanged,
  and alt prefixes `ESC`. escape/slash ignore shift. the
  [exact encoding table](terminal-shift.md#terminal-encoding) owns combinations.
- shift never changes typed/composed/pasted text or hardware input.
- A typed unit is modifier-eligible only when xterm's `onKey` provenance
  proves one discrete printable ASCII key. A deck accessory is
  intrinsically proven.
- IME commit/composition, dictation-shaped input, paste, Unicode,
  multi-character text, and uncertain xterm input remain literal and consume
  all three modifiers.

### State and lifecycle

modifier state belongs to the page and is published atomically for all three
fields. on port bind, publish all-off before `Ready`; consuming armed state
publishes all-off before `Input`. unchanged all-off snapshots are deduplicated.

Reset without input on page failure, disconnect or reconnect, window-focus
loss, backgrounding, rotation or recreation, close confirmation opening, detach, Back,
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
  Escape | Slash | Shift | Home | Up | End | PageUp |
  Tab | Control | Alt | Left | Down | Right | PageDown

TerminalModifierPhase = Off | Armed
TerminalModifiers = { control: TerminalModifierPhase,
                      alt: TerminalModifierPhase, shift: TerminalModifierPhase }

TerminalPage.sendAccessory(TerminalAccessory)
TerminalPage.resetInputState()
TerminalPageListener.onModifiersChanged(TerminalModifiers)
```

Native to page:

```json
{"kind":"Accessory","key":"Escape|Slash|Shift|Home|Up|End|PageUp|Tab|Control|Alt|Left|Down|Right|PageDown"}
{"kind":"ResetInputState"}
```

Page to native:

```json
{"kind":"ModifierState","control":"Off|Armed","alt":"Off|Armed","shift":"Off|Armed"}
{"kind":"Input","value":"..."}
```

the page port (version 6, [exact list](terminal-selection-copy.md#exact-packaged-page-api))
ships atomically with native and is internal. Unknown kinds, keys, states, missing or
extra fields, and malformed values fail closed. There is one protocol shape,
one input path, and no alias, fallback, negotiation, or compatibility decoder.

`TerminalKeyDeck` owns one local two-row item matrix, measured grid width, one
scroll state, selected rendering, and enabled state. `TerminalScreen`
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

1. The exact fixed matrix, labels, alignment, targets,
   shared overflow, normal-font height, large-text non-clipping, haptics, and
   disabled behavior.
2. Every base and modified key emits its specified value exactly once through
   the existing ordered `Input` route.
3. page-owned ctrl/alt/shift truth is atomic, visible, independently
   toggleable, jointly consumable, and cleared at every owned boundary.
4. IME, composition, dictation, paste, Unicode, CR behavior, queue bounds,
   focus, viewport containment, CSP, WSS, and tmux behavior remain unchanged.
5. the terminal viewport retains zero horizontal drift and the existing
   [readable-sizing](terminal-readable-sizing.md) fitted-grid/recovery contract.
6. Only the final two-row, atomic-modifier API and protocol remain.

## Non-goals

settings, customization, persistence, profiles, typed-text shift, a separate Meta key,
modifier lock, repeat, popups, long press, macros, snippets, gesture controls
other than terminal touch scroll, trackpads, hardware-keyboard adaptation,
dynamic ordering, content parsing, local-history controls, a native composer,
Kitty/CSI-u, xterm version upgrades,
telemetry, new logging, gateway/tmux/public-API
changes, and compatibility surfaces.
