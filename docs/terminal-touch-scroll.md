# v0 terminal touch-scroll delta

Status: implemented, released, and deployed. The unchanged-source owner
reds, rejected synthetic-wheel feasibility result, final five-owner signed S22+
green, complete 60-test signed same-version S22+ candidate green, two clean
161-test xterm rebuilds with identical output, and routine verification are
recorded. On 2026-09-04 the operator explicitly waived the outstanding targeted
mutation rerun, final-candidate hands-on journey, and live tmux/Claude Code
journey for shipment; those are waivers, not passes. PRs #54 and #55 are merged,
the exact `v0.2.27` source and artifacts are public, all three fleet hosts run
the pinned release, and the complete release-bound 60-test S22+ platform gate
is green with pairing unchanged and the exact public APK restored. Product,
second-phone, and reboot-persistence acceptance remain `NOT_RUN`.
[`architecture.md`](architecture.md) owns product behavior and acceptance;
[`roadmap.md`](roadmap.md) owns delivery order. This document owns the closed
implementation boundary. Testing follows [`rules/testing.md`](rules/testing.md).

## Outcome

A primary one-finger vertical drag over the live terminal behaves like a real
wheel at the same xterm location. xterm alone chooses the result for every
normalized line-wheel input:

| Current xterm state | Observable result |
| --- | --- |
| Application requested wheel-capable mouse tracking | xterm emits its negotiated mouse-wheel report |
| Active buffer has no scrollback capability and mouse does not own wheel | xterm emits its normal/application-cursor Up or Down sequence |
| Active buffer supports scrollback and mouse does not own wheel | xterm attempts phone-local viewport movement and emits no terminal input, including at a scroll boundary |

This is Termux-like routing, drag-only: compare Termux's immutable
[`doScroll`](https://github.com/termux/termux-app/blob/3b66f8799635a4dba4a206563048ff0e6792c487/terminal-view/src/main/java/com/termux/view/TerminalView.java#L573-L589).
It works around xterm 6.0.0's confirmed
[touch regression](https://github.com/xtermjs/xterm.js/issues/5489) with one
source-pinned public line-wheel API. That API delegates to one xterm-owned
internal router shared with physical DOM wheel handling; its outcomes use
xterm's existing mouse, cursor-fallback, and viewport primitives. It fixes live
terminal interaction; it is not a history subsystem.

## Goals, decisions, and rules

- One provider-neutral gesture delivers ordinary xterm wheel semantics to
  shells, tmux, Codex, Claude, and future opaque terminal programs; downstream
  software may consume or ignore them.
- The terminal page owns touch recognition. xterm remains the sole owner of
  buffer state, terminal modes, wheel routing, cursor encoding, mouse protocol
  encoding, and local viewport movement.
- Every API call takes exactly one xterm wheel route. Skíðblaðnir
  never reads xterm modes to choose a route and never emits terminal escape
  sequences itself.
- The existing ordered `terminal.onData` -> page `Input` -> WSS path remains
  the only terminal-input path. A local scroll emits no `Input`.
- The outer document and native WebView remain locked at `(0, 0)`. Only the
  xterm surface owns direct vertical touch manipulation.
- No touch coordinate, terminal byte, buffer content, or gesture trace enters
  logs or evidence.

## Capability contract

### Direct manipulation

- Accept only a trusted primary pointer with `pointerType == "touch"` that
  starts inside the existing xterm screen.
- At pointer-down, reject an existing `terminal.hasSelection()`. Otherwise
  snapshot the coordinate clamped inside the screen and the row height as
  `screen.getBoundingClientRect().height / terminal.rows`. A non-finite or
  non-positive height cancels without output. Never read xterm private fields.
- Use one named `8 CSS px` touch slop. Pending ends when either axis first
  exceeds slop: claim only when absolute vertical displacement is also greater
  than horizontal displacement; otherwise cancel. A second touch pointer or a
  selection observed before claim also cancels. Every pre-claim cancellation
  emits zero.
- Once claimed, acquire pointer capture before emitting, prevent the pointer's
  remaining default actions, and suppress matching touch-derived `mousedown`,
  `mousemove`, `mouseup`, `click`, and `contextmenu` at capture phase through
  the post-`pointerup` compatibility tail. Capture failure cancels with zero
  output; `lostpointercapture` cancels future output. Below-slop taps stay
  xterm-owned and retain focus and long-press selection behavior.
- Accumulate each move as signed incremental travel `previousY - currentY`,
  then update `previousY`; reversal therefore cancels un-emitted opposite debt.
  Convert the accumulator by the snapshotted row height. On each
  animation frame, invoke `terminal.handleWheelInput(...)` at most once with
  the snapshotted coordinate, false modifier flags, and signed whole-row
  `deltaLines`: negative for finger-down/backward, positive for
  finger-up/forward. Clamp magnitude to `terminal.rows`, drop excess whole-row
  debt, and retain only the fractional remainder for that gesture. No DOM
  wheel event is constructed or dispatched.
- On claimed `pointerup`, cancel the pending frame and derive the bounded
  whole-row target directly from `startY - finalY`. Emit at most one bounded
  correction from the already emitted row total to that target, then release
  capture and discard all debt. This final absolute reconciliation removes
  Chromium resampling overshoot without adding a second remote-input call.
  Nothing is emitted after release.
- After claim, any second pointer, selection, `pointercancel`, native
  `ACTION_CANCEL`, `lostpointercapture`, xterm textarea blur, window blur,
  resize, rotation, page hide, `ResetInputState`, page failure, or disposal
  immediately prevents future events, cancels a pending frame, and discards all
  debt. Events already dispatched remain authoritative.
- On a live enabled -> disabled transition, `ResetInputState` is synchronously
  enqueued before native disable. Initial pre-ready disable neither sends nor
  queues a command and is not a failure. A disabled native WebView accepts
  neither touch nor accessibility wheel actions. Reconnect starts with fresh
  gesture state and the attachment's fresh local buffer.
- Do not blur/refocus the terminal, dismiss/reopen the IME, clear selection, or
  move the horizontal/page viewport.
- Local scrolling leaves armed Ctrl/Alt unchanged. If xterm emits terminal
  input, the existing input reducer consumes the modifiers without applying
  them to unproven data, exactly as it does for other xterm-generated input.

The scoped screen rule is `touch-action: none`; delete the current global
`body { touch-action: pan-y; }` claim. Pointer capture and `touch-action`
follow the [Pointer Events contract](https://www.w3.org/TR/pointerevents3/).

### Accessibility

When the page is live and the native WebView is enabled, it exposes exactly two
resource-backed custom actions labelled `Terminal wheel backward` and
`Terminal wheel forward`.
Each posts the exact direction, cancels any direct-touch gesture, and invokes
one line-wheel input with magnitude `max(1, terminal.rows - 1)` at the current
screen center. Only those two action ids are intercepted; all other actions
delegate to `super`. Unready, disabled, unavailable, rejected-port, and
disposed states return false.

The WebView narrowly wraps Chromium's existing `AccessibilityNodeProvider` and
adds the actions to the currently accessibility-focused virtual terminal node;
every non-owned node query and action delegates unchanged. Availability changes
publish a subtree-content change so services refresh cached actions, never a
synthetic scroll event. The actions must be discoverable on the node TalkBack
and Switch Access actually focus inside Chromium's virtual tree. They are deliberately not
Android `ACTION_SCROLL_*`: a wheel may become local movement, remote input, or
a no-op, so emitting `TYPE_VIEW_SCROLLED` without true delta/max state would
violate Android's
[`TYPE_VIEW_SCROLLED` contract](https://developer.android.com/reference/android/view/accessibility/AccessibilityEvent#TYPE_VIEW_SCROLLED).
Touch exploration remains Android-owned. Existing `PgUp` and `PgDn` stay raw
terminal keys.

### Feedback and shared effects

- Local scrollback uses xterm's existing viewport thumb and position.
- Downstream-owned routes use only whatever redraw the opaque terminal stack
  produces.
- No mode, badge, toast, snackbar, setting, route label, or persistent scroll
  state is added.
- A wheel routed into tmux or the foreground TUI is ordinary shared terminal
  input and may change what the concurrently attached laptop sees. The product
  does not claim phone-private history.

## Architecture and API design

```text
trusted touch PointerEvent ----\
                                > page-private TerminalTouchScroll
native accessibility action --/             |
                                              v
                           xterm handleWheelInput(deltaLines, x, y)
                                              |
                    +-------------------------+-----------------------+
                    |                         |                       |
             local viewport          cursor Up/Down          mouse report
                                              |                       |
                                              +---- xterm onData -----+
                                                        |
                                                existing Input/WSS/PTY
```

`terminal.js` owns one private lifecycle handle:

```text
createTerminalTouchScroll({ terminal, screen }) ->
  { cancel(), scroll(direction), dispose() }

direction = Backward | Forward
state = Idle | Pending | Dragging
```

The state is page-memory only. `scroll(direction)` is the accessibility entry:
it cancels direct-touch state, then calls the same private line-wheel sender.
Touch calls that sender on animation frames.

The pinned xterm fork adds one public semantic ingress and one internal route
owner:

```text
terminal.handleWheelInput({
  deltaLines: integer,
  clientX: finite CSS pixel,
  clientY: finite CSS pixel,
  ctrlKey: boolean,
  altKey: boolean,
  shiftKey: boolean
}) -> void
```

`deltaLines` is non-zero, positive forward/down and negative backward/up. The
public boundary rejects non-integer/non-finite deltas and non-finite
coordinates. Before `open()` or after disposal it defects without queuing.
Inside xterm, physical DOM wheel input and this semantic ingress share one
private router which atomically selects wheel-capable mouse reporting first,
then exact local viewport `scrollLines(deltaLines)` when scrollback exists,
otherwise one normal/application-cursor sequence. The selected route is never
returned. Physical wheels retain their existing browser normalization and
smooth-scroll policy; semantic line input never enters that pixel/classifier
pipeline.

Native accessibility adds `Scroll`. The existing lifecycle reset is hard-cut
from modifier-specific naming to the one input-state owner:

```text
Scroll          = {"kind":"Scroll","direction":"Backward|Forward"}
ResetInputState = {"kind":"ResetInputState"}
```

The native/page protocol remains version `1` because its two packaged ends ship
atomically. Unknown, missing, extra, or differently cased fields fail closed.
There is no new page-to-native message, public HTTP/WSS schema, DTO, setting,
persistence, gateway operation, or tmux command.

`LockedTerminalWebView` should consolidate its repeated Focus/Accessory/
ResetInputState/Scroll message-port send boilerplate into one private exact
page-command sender. `failPage()` disposes the gesture owner, permanently
rejects later native messages, then publishes failure. Do not expose a generic
command API or production test seam.

## Hard cut and final state

- One Pointer Events implementation; no parallel Touch Events, Kotlin gesture
  detector, feature flag, legacy branch, fallback, or compatibility decoder.
- `ResetInputState` and `resetInputState()` replace `ResetModifiers` and
  `resetModifiers()` outright; no alias or dual decoder remains.
- One page line-wheel sender; no application-owned `terminal.scrollLines`,
  buffer/mode branch, hand-encoded mouse/cursor sequence, synthetic
  `WheelEvent`, PageUp remap, or tmux `send-keys`.
- Replace the official xterm JavaScript artifact with one visibly fork-suffixed
  artifact reproducibly built from tag `6.0.0`, commit
  `f447274f430fd22513f6adbf9862d19524471c04`, exact upstream source-tree and
  lock digests, exact post-patch manifest and lock digests, pinned Node/npm, an
  explicit Darwin arm64 build platform, the exact
  integrity-checked esbuild platform package omitted by that upstream lock,
  and one reviewed source patch. Install scripts remain disabled. Generation
  audits the full exact lock, including every compiler, linter, test, and
  bundler dependency, and defects on any known advisory; production-only or
  severity-class omissions are forbidden.
  The source patch also owns the minimum build-only dependency remediation
  needed to reconcile the tag's stale `5.5.0` lock metadata with its `6.0.0`
  manifest and make the complete executable build lock audit clean; those
  compiler/linter/test/bundler dependencies do not enter the shipped bundle as
  application dependencies. `terminal.lock` records both sides of that
  transformation, the toolchain, build platform, esbuild source/integrity,
  patch path/digest, generation command, and generated output digest. Never
  edit minified output. Preserve the MIT license and official CSS. Do not apply upstream's local-only
  [touch patch](https://github.com/xtermjs/xterm.js/commit/40f2eefec70577f9e8d3eda08f95028f3f04380a)
  beside this owner.
- Keep native/page horizontal containment, CSP, geometry, IME, paste, queue,
  WSS, PTY, attach, detach, and reconnect behavior unchanged.
- Delete obsolete `pan-y`, gesture experiments, imports, comments, and tests in
  the same change. Add no source-text test for their absence.

Before production edits, the first real-WebView red must prove that a trusted
drag in SGR mouse mode emits no compatibility button/motion report before its
first wheel report. Separate mouse-off cases prove a below-slop tap still
focuses and long press still selects. If Chromium's event order cannot satisfy
all three, stop and reopen gesture arbitration; do not ship a partial
pointerdown workaround.

The exact S22/WebView proof established that a constructed line-mode wheel
event exposes `deltaY == wheelDeltaY == -2`; xterm's copied VS Code normalizer
treats the deprecated field as a legacy `/120` value, the viewport consumes
the event, and local position does not move. That rejected seam must not
return. The source-pinned semantic API above is the hard cut. Stop if its
physical and semantic callers cannot share one internal router, if the exact
Android pin cannot route all three branches, or if the two custom actions
cannot appear on the actually focused virtual node.

## Files and non-overlapping ownership

| Owner | Paths | Proof |
| --- | --- | --- |
| Root integrator | `docs/architecture.md`, `docs/roadmap.md`, `docs/terminal-key-deck.md`, this document, `scripts/build-terminal-xterm`, `scripts/check-terminal-assets`, static-only composition in `scripts/test` | Scope, authority, and reproducible-generation policy |
| Android terminal-boundary builder | `android/xterm-6.0.0-skidbladnir-wheel.patch`, `android/terminal.lock`, generated fork-suffixed xterm JavaScript under `android/app/src/main/assets/terminal/vendor/`, `android/app/src/main/assets/terminal/index.html`, `terminal.js`, `terminal.css`, new `android/app/src/main/res/values/terminal_touch_scroll.xml`, `android/app/src/main/java/dev/niels/skidbladnir/LockedTerminalWebView.kt`, the exact `resetInputState` call-site rename in `android/app/src/main/java/dev/niels/skidbladnir/SkidbladnirController.kt`, and `android/app/src/androidTest/java/dev/niels/skidbladnir/TerminalInstrumentedTest.kt` | Owns the patch regression and every product behavioral red and green |
| Read-only verifier | none | Diff, residue, dependency, and gate review |

The runtime slice cannot split further without duplicating the gesture owner or
sharing its real-WebView proof. No builder touches other Compose/controller
code, gateway, tmux, `catalog/`, or `scripts/test` composition.

## Red / green / refactor and 80/20 proof

**Red:** add one boundary-weighted proof per owner to the existing real locked-WebView
instrumentation, using `UiAutomation`-injected native `MotionEvent` input and
fixed control fixtures:

| Owned boundary | One behavioral proof |
| --- | --- |
| Trusted touch -> xterm wheel | First prove feasibility: an SGR `1003` + `1006` drag yields wheel only—no touch-derived compatibility button, motion, click, or context-menu report through its tail—while separate mouse-off cases prove a `4 CSS px` below-slop move-and-tap retains native WebView and xterm textarea focus, long press selects, and a script-created pointer is rejected. Then parameterize the three xterm routes: a scrollback-capable normal buffer changes numeric accessibility-row position by the exact line delta away from bounds, stays fixed when pushed outward at top/bottom, and always emits no `Input`; an alternate buffer emits exactly one application-cursor sequence per API call; mouse mode takes precedence over real normal-buffer scrollback and emits exactly one SGR wheel report at the latched start cell without local position change. |
| Native action -> page command | Through `UiAutomation`, focus two distinct Chromium virtual terminal rows in turn and prove exactly two app-resource-id actions with the exact labels transfer to only the currently focused source while live and enabled; invoke their discovered ids and map backward/forward once through all three routes at the exact center cell. One stock Chromium action retains its semantic outcome, unavailable returns false, PgUp/PgDn stay raw, and missing/extra/wrong-type/wrong-case/unknown-direction forms of both new commands plus invalid-then-valid input fail closed. Availability changes refresh the subtree without a scroll event. |
| Input-state lifecycle -> containment | Pre-claim cancellation emits zero; real `ACTION_CANCEL` after claim retains prior output but emits no more. Cover horizontal-first, reversal, second pointer, active selection, the maximum in-screen jump followed by a tiny move, pointer-up flush, disabled fresh touch, active composition during drag, background/focus loss, rotation, page failure with queued Scroll, pending-gesture disposal, and recreation followed by a successful fresh gesture. Initial pre-ready disable still reaches Ready without sending or queuing reset or becoming unavailable. Armed modifiers survive local scroll; cursor/mouse input publishes Off/Off first without modified bytes. Focus, InputConnection, IME/composition, page/WebView/horizontal position, and `80 x 5` geometry remain unchanged. |

Assertions expose only user-visible numeric accessibility position/state and
content-safe protocol equality. Failure messages name case, route, direction,
count, index, and length—never raw bytes, payloads, JavaScript expressions, or
rendered content. Touched shared helpers must meet the same rule. Real
`ACTION_CANCEL` owns capture-loss behavior; capture-acquisition failure remains
a code-reviewed defensive branch unless a real platform trigger is found. The
`terminal.rows` clamp is likewise code-reviewed: two honest in-screen points
are always less than `terminal.rows` row heights apart. Do not fabricate
off-screen coordinates, blur, frame timing, or capture failure. The black-box
residual proof owns completion and post-up debt discard; synchronous
`pointerup` correction attribution remains code-reviewed. Initial pre-ready non-enqueue is
also code-reviewed because resetting an already-Off state is behaviorally
silent; the red owns Ready, no extra event/input, and no unavailability. No
test-only terminal global, xterm mock, production seam, or internal callback
assertion is allowed.

The three behavioral reds were authored first and observed separately on the
approved S22+ against unchanged source. The first synthetic-wheel candidate
then remained red at the local route while its content-free diagnostic proved
xterm consumed the event with conflicting modern/legacy semantics; that result
triggered the source-API cut above. Merely compiling a later test remains
`NOT_RUN`, not red.

The final signed candidate passed its five focused ownership tests and the
complete 60-test instrumentation suite on the approved S22+. Both transactions
restored the byte-identical public `v0.2.25` APK and removed the test package.
The final content-free encrypted-pairing digest still matched its pre-candidate
value after those runs.
This is candidate/component evidence, not the clean exact-release-source
`platform` gate. A user-observed basic finger-scroll success occurred on an
earlier candidate; subsequent lifecycle and amplification corrections mean it
does not substitute for the final-candidate hands-on journey.

On 2026-09-04 the operator explicitly accepted that remaining evidence gap and
directed shipment without the outstanding targeted mutation rerun, final
hands-on matrix, or live tmux/Claude Code journey. Those boundaries are
`WAIVED`, not green, and no lower-layer result is promoted to replace them.

PR #54 merged the terminal touch-scroll hard cut. The first two release-bound
`v0.2.26` platform attempts failed closed at the harness's stale fixed
90-second instrumentation deadline: the second had passed all 45 tests it had
started, but had not started all 60. The harness owner red then proved that no
suite-scaled deadline existed; PR #55 replaced the fixed ceiling with one
discovered-suite budget, retaining the existing strict completeness checks and
a finite timeout. Exact-head hosted CI passed for the final source
`6bba344d3e965285fe0a9b8535924962ecb52c62`.

The immutable `v0.2.27` release was published from that source. Its public APK
then passed the complete release-bound platform gate on the approved S22+:
`OK (60 tests)` in `199.959` seconds. Cleanup removed the test package,
restored the byte-identical public APK with its pinned signer and
`0.2.27`/`2027` version, preserved the encrypted pairing, and relaunched the
production activity. The release pin was merged in the deployment repository;
MacBook, DevServer, and Arch acceptance and final fleet doctor passed on
`v0.2.27`. Reboot persistence was not exercised and remains `NOT_RUN`.

**Green:** implement only the source-pinned xterm line-wheel API/router, page
gesture owner, scoped CSS ownership, two native custom actions, exact commands,
and lifecycle reset rename required by those reds. The source patch carries
focused upstream-style route/API regressions; the real-WebView matrix remains
the authoritative product proof.

**Refactor:** leave one gesture state machine, one row accumulator, one
cancellation path, one page API sender, one xterm internal router, and one
native page-command sender. Mutation sensitivity must show that replacing the
xterm router with local-only scrolling breaks the cursor/mouse route proof,
reversing the sign breaks direction, bypassing or duplicating the router breaks
route precedence/exact counts, and removing cancellation leaks remainder. The
maximum honest in-screen jump has an exact numeric bound; a tiny follow-up and
pointer-up must not replay debt or amplify remote input. Review the unreachable
move-frame excess-clamp branch directly. Remove every runnable mutation.

Verification is deliberately boundary-weighted:

- the focused real-WebView instrumentation owns gesture, xterm routing, native
  accessibility, and shell containment;
- `./scripts/test verify` owns static analysis, packaged-asset integrity,
  compilation, lint, and unit regressions, but does not prove touch behavior;
- `./scripts/build-terminal-xterm --check` performs a clean source fetch,
  verifies tag/commit/tree/lock/patch/toolchain pins, runs the focused upstream
  tests and linters, rebuilds the bundle, and byte-compares the committed
  artifact. Two clean invocations with one digest own reproducibility;
- separately approved focused S22+ runs own the unchanged-source red, focused
  green, and each mutation red; the complete signed same-version platform suite
  then owns platform regression;
- a separately approved hands-on S22+ component journey owns threshold jitter,
  diagonal/slow/fast direction, long-range drag-only usability, long press and
  selection handles, active Gboard composition, IME/focus stability, action discovery and
  feedback in TalkBack and Switch Access, rotation, and normal/alternate/mouse
  route fixtures;
- a separately approved live-terminal journey—explicitly a
  tmux/live-host/provider-live boundary—owns actual phone -> tmux -> Claude Code
  scrolling with concurrent laptop geometry and focus unchanged.

No separate integration, provider-hook, release, product, or second-phone gate
re-proves these owners. Every unapproved device/tmux/live-host boundary is
`NOT_RUN`, never pass.

## Acceptance criteria

1. A one-finger vertical drag over the terminal produces the three exact xterm
   outcomes above, in natural direction, with exactly one route per line-wheel
   call; a normal-buffer boundary never becomes cursor input.
2. Gesture arbitration, cancellation, amplification bound, accessibility
   actions, modifier behavior, and lifecycle match this contract.
3. Terminal focus, selection, IME, composition, paste, color, geometry,
   horizontal containment, transport, and attachment lifecycle do not regress.
4. Codex and Claude remain opaque. No content, transcript, provider, gateway,
   tmux, persistence, or product/network API capability is added.
5. Only the final Pointer Events owner, exact internal `Scroll` and
   `ResetInputState` commands, one reviewed xterm source patch, and its
   reproducibly generated fork artifact remain.

## Non-goals and explicit trade-offs

No history before attach or across reconnects; no durable, canonical,
searchable, or provider transcript; no Skíðblaðnir-owned tmux copy-mode
command or guarantee;
`capture-pane`; replay; retention changes; mouse-policy changes; provider
detection; scroll setting; scrollbar-drag fix; pinch zoom; stylus gesture;
kinetic fling; new visual chrome; telemetry; xterm version upgrade; or terminal-content
logging. Existing opaque tmux mouse bindings may enter shared copy mode.

- **Context sensitivity:** one swipe may be local or shared application input.
  This is accepted terminal parity; calling it “history” would be false.
- **Maintained xterm fork:** the semantic API fixes ownership and exact-row
  behavior but adds source-build and upgrade burden. The accepted guard is one
  reviewable patch, pinned reproducible inputs and output, upstream-style
  regressions, and the black-box S22 three-route matrix; a future xterm upgrade
  cannot land without all of them.
- **Build-lock remediation:** the official `6.0.0` source tag carries stale
  `5.5.0` root lock metadata and a development toolchain that does not meet the
  required complete zero-advisory audit. The source patch therefore includes a
  pinned, build-only lock refresh plus the minimum manifest/lint compatibility
  delta. This enlarges dependency review, but is accepted over excluding dev
  dependencies, weakening audit severity, running an unpinned lock rewrite, or
  trusting install scripts. Separate upstream and post-patch digests make the
  transformation fail closed and visible at upgrade time.
- **Declared build platform:** the source audit is reproducible on the one-user
  release workstation's Darwin arm64 boundary. Supporting another build host
  requires an explicit integrity-pinned esbuild package and identical-output
  proof; an implicit platform download or install-script fallback is rejected.
- **Bounded batching:** one frame or accessibility action can produce at most
  one remote cursor/mouse unit even when its event carries several local lines.
  This sacrifices remote velocity fidelity to prevent input amplification.
- **Truthful accessibility:** custom wheel actions sacrifice Android's generic
  scroll gesture and may require the actions menu. Standard scroll actions were
  rejected because this owner cannot truthfully report local delta/max state
  when xterm routes the wheel into terminal input or a no-op.
- **No fling:** scrolling stops at release. This sacrifices momentum to avoid
  post-gesture terminal input and a second timing/velocity policy in the 80/20
  slice; long-output usability must pass hands-on.
- **Finite local lifetime:** local scrollback contains only bytes retained by
  the current xterm attachment. The gain is zero replay, storage, or content
  ownership.
