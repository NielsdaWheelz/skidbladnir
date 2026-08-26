# Terminal key deck

Status: implemented; council source review, routine verification, the 19-test
S22+ platform gate, and user-reported hands-on acceptance are green.

[`architecture.md`](architecture.md) owns product behavior and acceptance.
This document owns the implementation boundary and delivery plan for the v0
terminal-key-deck delta. [`roadmap.md`](roadmap.md) owns ordering.

## Outcome

The phone exposes one stable terminal-input deck:

`Esc | Ctrl | Tab | Line break | Left | Up | Down | Right | Home | End`

The deck contains no app navigation, attachment lifecycle, destructive action,
provider semantic, or macro. Top-bar `Agents` and Android Back remain the only
primary return-to-inventory controls; leaving detaches the phone and keeps the
tmux session running. Kill remains separate and confirmed.

## Goals and rules

- Compose terminal primitives; never guess what an opaque terminal program
  means by a byte.
- Give each concern one owner: Compose renders; `terminal.js` owns Ctrl,
  accessory encoding, and the ordered input ingress from xterm's authoritative
  terminal-mode state; the locked WebView validates transport; the controller
  attempt-gates dispatch.
- Preserve Gboard composition, dictation, paste, Unicode, LF/CR distinction,
  viewport containment, and 80-column portrait geometry.
- Use one-shot Ctrl with visible and spoken state. Never persist, time out, or
  replay an incomplete chord.
- Keep positions fixed. The one row may scroll; it never wraps, shrinks touch
  targets, or reorders itself.
- Preserve the single ordered `Accessory -> terminal.js -> Input` route and
  validated page-port version. Add no raw-byte bypass, second input protocol,
  compatibility decoder, alias, fallback, or version negotiation.
- Log no key, byte, input source, terminal content, or modifier transition.

## Capability contract

### Keys

| Key | Observable result |
| --- | --- |
| `Escape` | Emit `0x1b`. |
| `Control` | Toggle `Off <-> Armed`; emit nothing. |
| `Tab` | Emit `0x09`. |
| `LineFeed` | Emit `0x0a`; Gboard Enter remains `0x0d`. |
| `Left/Up/Down/Right` | Emit xterm's normal or application-cursor sequence for the current mode. |
| `Home/End` | Emit xterm's normal or application-cursor sequence for the current mode. |

`Control` is one-shot. It binds to the next unit accepted by the page's single
ordered input ingress. Any subsequent typed, pasted, or terminal-emitting deck
unit returns it to `Off`. A key-proven single ASCII unit maps as follows:

- lowercase ASCII letters first normalize to uppercase;
- `@` through `_` emit `codepoint & 0x1f`;
- `?` emits `0x7f`;
- every other input passes through unchanged.

Composition, dictation, paste, multi-character text, Unicode, and input whose
origin is uncertain are literal units: they pass unchanged and clear `Armed`.
In particular, a one-character Android `commitText`/`beforeinput` is uncertain;
only a real discrete xterm `onKey`/keyboard event may make it key-proven.

The WebView and key deck are one logical terminal-input surface, so moving
focus from the WebView to an accessible deck button does not clear `Armed`.
Page loss, disconnect, reconnect, background/window-focus loss, navigation,
opening Kill confirmation, rotation/recreation, and disposal clear `Armed`
without emitting input.

### Content and interaction

Each content owner approves its row before implementation. Good content names
the literal control or byte-level outcome, avoids provider semantics, is terse
on screen, and is complete when spoken.

| Feature | Content owner | Required content |
| --- | --- | --- |
| Return to inventory | Product/content designer | Visible `Agents`; spoken `Return to Agents; session keeps running`. |
| Momentary keys | Terminal interaction designer | `Esc`, `Tab`, arrow glyphs, `Home`, `End`; spoken names use `Escape`, `Tab`, and `<direction> arrow`. |
| Line feed | Product/content designer | Visible `Line break`; spoken `Line break; sends line feed`. Never label it Enter, Send, or claim how the opaque program responds. |
| Ctrl | Interaction/accessibility designer | Visible `Ctrl`; accessible name `Control`; selected styling while armed; state description `Off` or `Armed`. Never label it Stop or Interrupt. |

All targets are at least `48dp x 48dp`, separated by at least `8dp`. Order and
TalkBack traversal match the capability order. Ctrl has toggle semantics and a
state description. Each enabled deck-key activation uses one subtle system
keyboard-tap haptic; disabled keys and app-navigation controls do not. Haptics
never claim the remote program acted.

## Architecture and internal API

```text
TerminalKeyDeck -- TerminalAccessory --> controller --> LockedTerminalWebView
                                                          | Accessory / reset
                                                          v
                                                 terminal.js queue ingress
                                                    |              |
                                               ControlState       Input
                                                    v              v
                                             locked WebView --> existing WSS
```

- `TerminalKeyDeck` owns order, labels, touch geometry, semantics, selected
  rendering, and enabled state. It owns no byte encoding or modifier truth.
- `terminal.js` owns `TerminalControlState`, control mapping, lifecycle reset,
  xterm mode-sensitive sequences, and the only ordered input ingress. It
  classifies input at the IME boundary; uncertain input is literal.
- `LockedTerminalWebView` owns exact same-system message validation and
  transport. It reports page-owned control state but never derives or mutates a
  Kotlin copy of modifier truth.
- `SkidbladnirController` attempt-gates and dispatches `TerminalAccessory`; it
  owns no key rules, bytes, queue, or mirrored modifier state.
- Gateway, WSS, PTY, tmux, session lifecycle, and public API are unchanged.

Every deck key, including the Ctrl-modified next key, travels the same single
ordered page input queue as typed and pasted input; the one-shot Ctrl binds to
the next unit of that queue and is applied inside the page, never as a
Kotlin-side byte transform.

Owned Kotlin types:

```text
TerminalAccessory = Escape | Control | Tab | Left | Up | Down | Right | Home | End | LineFeed
TerminalControlState = Off | Armed

TerminalPage.sendAccessory(TerminalAccessory)
TerminalPage.resetControl()
TerminalPageListener.onControlStateChanged(TerminalControlState)
```

Native-to-page messages retain `Output`, `Focus`, and the validated exact
`{"kind":"PagePort","version":1}` handshake. Deck input extends the existing
exact `Accessory` shape; lifecycle owners use one exact reset command:

```json
{"kind":"Accessory","key":"Escape|Control|Tab|Left|Up|Down|Right|Home|End|LineFeed"}
{"kind":"ResetControl"}
```

`Control` crosses the same native-to-page port as every deck key, toggles state
at the ordered page ingress, and emits no input. Page-to-native retains the one
existing socket-bound input message and adds state notification:

```json
{"kind":"Input","value":"..."}
{"kind":"ControlState","state":"Off|Armed"}
```

The page internally classifies each ingress unit as key-proven or literal,
applies and clears Ctrl, then emits exactly one final `Input` message. Existing
UTF-8 and size checks remain. Unknown kinds, unknown keys, extra fields, and
malformed values fail the page boundary. Native and packaged page assets ship
atomically at page-port version `1`; there is no compatibility path.

## Hard cut and cleanup

Delete in the same change:

- bottom-row `Agents` and `Detach`, their parameters, and duplicate callbacks;
- fixed `Ctrl-C`, `CtrlC`, `Newline`, and `AccessoryButton`; replace the two
  enum variants with `Control` and `LineFeed`;
- any direct or Kotlin-transformed deck-byte path; PR #5's removed `onBytes`
  bypass stays absent;
- the superseded Kotlin Ctrl reducer and proposed `Key`/`KeyInput`/`TextInput`
  parallel protocol;
- tests and comments that assert the retired shapes.

Retain and adapt `TerminalAccessory`, `sendAccessory`, the versioned page port,
the single `Input` message/socket writer, exact JSON-key validation,
cursor-mode encoder, UTF-8 bounds, terminal connection, top `Agents`, Back,
confirmed Kill, LF/CR behavior, and IME/paste containment. Do not add a generic
keyboard framework, registry, settings store, telemetry, or gateway endpoint.

## Work split

The API above freezes before builders start. Input and UI builders use disjoint
paths and may work concurrently; neither slice lands alone. The root integrator
joins them and owns cross-slice compilation. A builder writes and observes its
own red proof before production implementation.

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| Contract | Root integrator with product/content/interaction review | `docs/terminal-key-deck.md`, `docs/architecture.md`, `docs/roadmap.md` | Document consistency and residue review. |
| Terminal input | Android terminal-boundary builder | `LockedTerminalWebView.kt`, `terminal.js`, `SkidbladnirController.kt`, `TerminalInstrumentedTest.kt`, `TerminalTestActivity.kt`; delete the superseded untracked `TerminalControlTest.kt` | Real WebView ordered input, protocol, Ctrl, and reset behavior. |
| Key deck | Compose UI builder with interaction/accessibility review | `TerminalKeyDeck.kt` (new), `TerminalScreen.kt`, `TerminalKeyDeckInstrumentedTest.kt` (new), `android/app/build.gradle.kts` | Rendered order, content, state, touch bounds, traversal, and absence of lifecycle controls. |
| Verification | Read-only verifier | No files | Review target behavior and report routine/platform status; write no test or production file. |

No builder edits `catalog/`, gateway/tmux code, `scripts/test`, architecture, or
roadmap. A newly discovered need outside these paths reopens scope; it does not
authorize a cross-slice edit.

## Red/green/refactor and acceptance

### Red

1. WebView component: the retained versioned handshake and exact
   `Accessory`/`Input` route; full Ctrl mapping, one-shot consumption,
   second-tap cancel, queue order, logical-lifecycle reset, normal/application
   cursor encoding, the Enter-CR versus Line-break-LF distinction, and literal
   single-character `commitText`, paste, composition, dictation-shaped,
   Unicode, and multi-character input.
2. Compose component: the user sees the fixed order and no `Agents`, `Detach`,
   or `Ctrl-C`; Ctrl announces and renders `Off/Armed`; all keys disable with a
   frozen terminal; target bounds and traversal are accessible.

### Green

- Implement only enough production behavior to satisfy those proofs.
- Run routine `scripts/test verify`; it invokes no tmux or device.
- Run the separately approved `platform` gate once for the joined Android
  boundary. Without explicit approval in that implementation turn it is
  `NOT_RUN`, never pass.
- Do not run `integration` or `live`: public transport, tmux, and gateway
  behavior do not change.

### Refactor

- Collapse all deck input through `TerminalAccessory` and all Ctrl transitions
  through the page's one ordered ingress.
- Delete every retired symbol/path before re-running routine verification.
- Add no abstraction that has only a speculative second consumer.

### Physical S22+ acceptance

One approved pass proves: one-handed portrait reach and scrolling; Ctrl then
`c`, `b`, and `j`; Gboard Enter sends CR while `Line break` sends LF; second-tap
cancel; Armed survives actual deck focus but clears on Kill open, background,
rotation/recreation, reconnect, `Agents`, and Back; Gboard typing, composition,
dictation, and paste remain literal when required; TalkBack state/order; Switch
Access reachability; subtle haptics; zero horizontal drift; and at least 80
columns. A one-character `commitText` is never promoted to a key: if stock
Gboard cannot supply discrete key provenance for the Ctrl chord, this
acceptance fails and the product contract reopens. Retain no entered content or
terminal bytes.

## Non-goals

Alt/Meta/Shift, Page Up/Down, function keys, a dedicated `^C`, macros, overflow
menus, customization, persistence, usage counters, automatic reordering,
modifier lock, arrow repeat, gestures/trackpads, hardware-keyboard adaptation,
native composer, Kitty/CSI-u, xterm/tmux upgrades, terminal-content parsing,
and any gateway/tmux/public-API change.
