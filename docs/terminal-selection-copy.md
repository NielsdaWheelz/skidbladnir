# v0 phone-local terminal selection copy

Status: source implemented. The original xterm and unchanged-runtime API-36
reds are complete, the source xterm matrix is green at `171` tests, routine
verification is green, and focused signed same-version API-36 owner proofs are
green across selection/copy, exact protocol, generation ordering, Back,
lifecycle, size, accessibility, queued-copy liveness, and composition-first
intent arbitration.
Physical API-36 feasibility proved that Chromium stops DOM PointerEvents after
long-press promotion while native MotionEvents and DOM TouchEvents continue;
the reviewed gesture owner is therefore hard-cut to TouchEvents below. The
complete `38`-test signed same-version candidate matrix is green after the
intent-arbitration correction; the hands-on pass remains `NOT_RUN`. After every
completed or aborted device transaction the exact pinned release was restored
and the test package was absent; the latest correction runs relaunched the
production app. Immutable `v0.2.28` was then published and installed, where
hands-on use exposed that an
unfocused terminal tap no longer summoned Gboard. The 2026-09-07 correction has
an executed unchanged-runtime API-36 red, a focused `5/5` signed same-version
green covering tap intent and the exact protocol, and green routine
verification. Its complete candidate, release-bound platform, `v0.2.29`
publication/deployment, and hands-on acceptance remain unclaimed.
[`architecture.md`](architecture.md) owns the product delta and acceptance;
[`roadmap.md`](roadmap.md) owns delivery order. This document owns the
implementation boundary. Testing follows [`rules/testing.md`](rules/testing.md);
there is no separate `testing-standards.md`.

## Outcome and scope

On the live Android terminal, a primary-touch long press starts an xterm cell
selection even when the foreground TUI has enabled mouse reporting. Holding and
dragging extends it. Release shows one native floating action mode containing
`Copy`. Pressing `Copy` places exactly `terminal.getSelection()` on that phone's
Android primary clipboard, then clears the selection. xterm documents
[`getSelection()`](https://xtermjs.org/docs/api/terminal/classes/terminal/#getselection)
for copy behavior outside the emulator.

“Guaranteed” means: while the packaged page is live, foreground, enabled, and
the selected UTF-8 text is `1..262144` bytes, the visible `Copy` action makes
the phone's real primary plain-text clip equal the exact release snapshot
before success cleanup. No success is claimed early. No terminal input, WSS
frame, tmux command, provider behavior, host clipboard, or network request
participates.

An unfocused, below-slop primary tap is also guaranteed to acquire the native
WebView and xterm helper focus and request the phone IME. An unfocused drag
does not request it. This restores terminal input acquisition without creating
a second native gesture classifier.

This is one phone-local presentation capability. Android, Gboard, or an OS
cross-device service may retain or synchronize the resulting system clip; that
policy is outside Skíðblaðnir's control.

Open decisions: none. Changing the cap, selection gesture, post-release editing,
clipboard policy, or any provider/tmux path requires a new scope and acceptance
change.

## Capability contract

### Selection

- Replace the page's PointerEvent path with one page-owned TouchEvent state
  machine; do not add a Kotlin gesture detector or a second touch path. Define
  an eligible primary touch as exactly one trusted DOM touch contact that
  starts inside the xterm screen. Screen-targeted untrusted TouchEvents are
  consumed and ignored before xterm can observe them. Stylus behavior is not
  claimed.
- On eligible `touchstart`, synchronously require a cancelable event, call
  `preventDefault()`, require `defaultPrevented`, and stop immediate propagation
  before arming any gesture. Consume every remaining event in that owned stream
  the same way. Otherwise fail the page closed. This makes the page the sole
  touch owner and prevents Chromium compatibility mouse/context-menu promotion.
- Retain the already-proven `8 CSS px` slop for both scrolling and selection.
  Native supplies only `ViewConfiguration.getLongPressTimeout()` during the
  exact page-port handshake; the page validates it once.
- If composition is active at an eligible `touchstart` and no released
  selection already owns the interaction, latch the complete stream as
  `Composing`. Consume its tail without a timer or any tap, scroll, mouse,
  cursor, selection, modifier, or viewport effect. Composition ending during
  the stream does not reclassify it. Android/WebView alone owns composition
  finalization through the existing literal xterm input path; the next fresh
  gesture resumes normal arbitration.
- Before long-press expiry, the existing vertical-drag routing remains
  authoritative. Movement beyond slop cancels the timer. A below-slop release
  focuses the terminal, submits one semantic primary tap to xterm, then emits
  one content-free `ImeRequested` intent. Native exact-decodes that intent and,
  only while the page is live, visible, enabled, attached, window-focused, and
  selection-idle, acquires WebView focus and requests `WindowInsets.Type.ime()`.
  xterm's negotiated mouse protocol remains the sole authority for resulting
  terminal input. An accessibility-wheel action during any selection
  generation clears that selection and emits no wheel or terminal input.
- Track the sole `Touch.identifier` and resolve every move and final coordinate
  from the matching touch, including `changedTouches` on end. Never index
  `touches[0]`. A second contact cancels and drains all touches before a new
  gesture may start; missing identifiers and invalid streams fail closed.
  Live lifecycle reset also preserves blocked ownership until the still-down
  stream drains; only disposal may abandon the tail.
- At expiry, select the nearest xterm word (one cell when blank) and request one
  native `LONG_PRESS` haptic. Mouse-reporting mode is intentionally bypassed
  only for this claimed selection gesture.
- While held, movement extends by xterm cells and uses xterm's selection
  auto-scroll at the viewport edge. Touch target retention carries an active
  contact outside the screen. Release first reconciles the matching final
  `changedTouches` coordinate, then retains the range, snapshots the exact xterm
  selection in page memory, and opens the native action mode at the normalized
  release point.
- Once selection exists, one new primary tap, action-mode dismissal, second
  contact, cancellation, attachment loss, backgrounding, disable, rotation,
  page failure, or disposal clears it and emits zero input. Android Back first
  dismisses the native action mode/selection; a subsequent Back retains the
  existing phone-detach contract.
- Physical API-36 proof showed that the framework floating action mode does not
  consume system Back. While and only while `Selected`, register one
  view-resolved `OnBackInvokedCallback` at `PRIORITY_OVERLAY`; committed Back
  enters correlated clearing before the default-priority Compose detach
  handler. Unregister it before every transition out of `Selected` and on
  failure/disposal. No Activity cast, Compose callback, key interception, or
  legacy `onBackPressed` path exists.
- xterm alone owns buffer coordinates, wrapped rows, wide/combining cells,
  scrollback, selection rendering, and returned text. The page never rebuilds
  text from the DOM, accessibility tree, or buffer rows.
- Selection is attachment-local and limited to the active xterm buffer retained
  since this attachment. There is no post-release handle editing in this 80/20
  slice; start again to refine a released range.

### Copy

- Copy is explicit; selection changes never write the clipboard.
- The native action mode has one resource-backed action, `Copy`. It is
  positioned with Android's
  [`ActionMode.Callback2`](https://developer.android.com/reference/android/view/ActionMode.Callback2)
  and remains framework-accessible.
- A native `Idle | Selecting | Selected | Clearing` state machine accepts one
  immutable release snapshot only after a matching `SelectionStarted`
  generation. The user-visible
  `Copy` action is the only transition that may write it to the clipboard.
- The page sends the transient release snapshot once in `SelectionAvailable`.
  Native retains it only while `Selected`. This prevents redraw, reflow, or
  scrollback trimming from changing what Copy returns. Copy means “text visible
  at release”: later output may change cells under the retained highlight, but
  never the snapshot. Preserve the string exactly: no trim, ANSI parsing,
  newline conversion, reflow, sanitization, or Unicode normalization.
- Empty selection clears quietly. Valid text is a Unicode scalar sequence (no
  unpaired UTF-16 surrogate) and at most `256 KiB` in UTF-8. Page and native
  independently apply the existing bounded UTF-8-counting pattern before the
  side effect. Oversize selection is cleared and produces one resource-backed
  failure toast; there is no file/share fallback.
- Native creates `ClipData.newPlainText("Terminal selection", text)`, sets
  `ClipDescription.EXTRA_IS_REMOTE_DEVICE = true`, and calls
  `setPrimaryClip` on the main thread. It never reads the production clipboard.
- Do not set `EXTRA_IS_SENSITIVE` for an explicit manual selection: Android's
  standard preview is the success feedback. Add no success toast or snackbar
  on the API-36-only client, following Android's
  [copy-feedback guidance](https://developer.android.com/develop/ui/views/touch-and-input/copy-paste#Feedback).
- `setPrimaryClip` is the sole irreversible effect and is called once per
  explicit `Copy` activation, on the main thread, only while `Selected`; at
  most one write may succeed for a selection. Immediately before constructing
  the clip, Copy must pass the WebView owner's authoritative live, enabled,
  attached, and foreground authorization. Its unavailable decision linearizes
  under the existing output-state lock; failure committed before the queued
  action executes forbids the write. A later failure is ordered after the
  authorization, without holding a private lock across ClipboardManager IPC.
  After `setPrimaryClip` returns, native enters
  `Clearing`, drops the snapshot, sends `ClearSelection(generation)` once, and
  programmatically closes the action mode. A `SecurityException` triggers no
  automatic retry: it retains the selection and snapshot in `Selected`, shows
  one content-free failure toast, and permits a new explicit Copy attempt.
- Opening an action mode is a reentrancy-safe transaction. If lifecycle loss or
  synchronous framework destruction occurs before `startActionMode()` returns,
  the returned mode cannot resurrect `Selected`; finish it and retain the
  already-chosen clearing/failure state. If `startActionMode()` returns null,
  enter `Clearing` and send one correlated clear. Empty or vanished selection
  sends `SelectionCleared` and closes quietly. Programmatic finish marks state
  and suppresses its reentrant destroy callback before closing, so
  `ClearSelection` is sent at most once.
- Clipboard text, selected text, hashes, previews, prompts, and terminal bytes
  never enter logs, analytics, saved state, crash context, or evidence.

## Architecture and composition

```text
trusted primary touch
        |
        v
TerminalTouchInteraction -- semantic selection input --> pinned xterm
        |                                                   |
        | Started / Available(snapshot) / Cleared           | getSelection()
        v                                                   |
LockedTerminalWebView --> TerminalSelectionController <-----+
                                |
                    Android floating ActionMode
                                |
                         explicit Copy tap
                                v
                       Android ClipboardManager
```

`terminal.js` hard-renames `createTerminalTouchScroll` to
`createTerminalTouchInteraction`; its one TouchEvent state machine owns the
mutually exclusive `Composing`, `Pending`, `Scrolling`, `Selecting`, `Selected`,
and drained multi-touch states. A released `Selected` interaction retains
precedence over a new composition observation because it already owns native
modal state. The existing wheel API and routes remain unchanged.

The pinned xterm fork adds one atomic semantic tap ingress:

```text
terminal.handleTapInput({ clientX: finite CSS px, clientY: finite CSS px }) -> void
```

It maps coordinates once through xterm's mouse service and submits fresh
primary-left `DOWN` and `UP` records to `CoreMouseService`; that existing owner
decides what the negotiated protocol can emit. Mouse-off emits no mouse input,
X10 emits its press-only form, and protocols supporting release retain their
normal press/release form. The page never reads terminal modes, encodes escape
bytes, or fabricates DOM events. A completed tap focuses before this call;
negotiated DEC focus reporting remains ordinary terminal behavior.

The pinned xterm fork adds one semantic API that delegates to its existing
internal selection owner and can never emit `onData`:

```text
terminal.handleSelectionInput(input) -> void

input =
  { kind: StartWord, clientX: finite CSS px, clientY: finite CSS px } |
  { kind: Extend,    clientX: finite CSS px, clientY: finite CSS px } |
  { kind: End,       clientX: finite CSS px, clientY: finite CSS px } |
  { kind: Cancel }
```

Invalid shape, coordinates, order, pre-open use, or post-disposal use defects.
`StartWord` deliberately takes local selection precedence over DEC mouse mode;
all mouse and wheel protocol behavior remains xterm-owned.

Native transition table:

| State + event | Next | Effect |
| --- | --- | --- |
| `Idle + SelectionStarted(g)` | `Selecting(g)` | One haptic |
| `Selecting(g) + SelectionAvailable(g, valid)` and action mode starts | `Selected(g)` | Retain snapshot/action mode |
| `Selecting(g) + SelectionAvailable(g, valid)` and action mode returns null | `Clearing(g)` | Drop snapshot; send one correlated clear |
| `Selecting(g) + SelectionCopyRejected(g)` | `Clearing(g)` | Failure toast; send one correlated clear |
| `Selected + Copy` and clipboard call returns | `Clearing` | One clipboard write; drop snapshot; one clear; suppressed programmatic finish |
| `Selected + Copy` and clipboard call throws `SecurityException` | `Selected` | No automatic retry; keep snapshot/action mode; failure toast |
| Any live non-idle state for `g` + `SelectionCleared(g)` | `Idle` | Drop snapshot; suppressed finish |
| `Selected + user dismissal` or overlay Back | `Clearing` | Drop snapshot; unregister Back; send one clear and await its acknowledgement |
| `Selecting`/`Selected + live lifecycle reset` | `Clearing` | Drop snapshot; send one clear; suppressed finish; await acknowledgement |
| `Clearing + live lifecycle reset` | `Clearing` | Send nothing; await the existing acknowledgement |
| Any state + page failure/disposal | `Idle` | Drop snapshot; finish; accept no later message; never replay |

`SelectionCleared(g)` is the one acknowledgement that completes `Clearing(g)`.
The page assigns `1`, then each exact positive safe-integer successor, encoded
as a canonical decimal string. Native retains the page-lifetime high-water,
accepts only that sequence, and echoes the exact token in its clear command. A
crossed clear for an already acknowledged older generation is
idempotently ignored by the page and can never touch a newer selection. Every
page-originated mismatched, duplicate, future, or out-of-order message fails
native closed. Exhausting the safe-integer space fails the page closed. The
page keeps no copy snapshot after `SelectionAvailable`; native drops its
snapshot immediately on entering `Clearing`, page failure, or disposal.

`LockedTerminalWebView` remains the exact message decoder and lifecycle owner.
One new `TerminalSelectionController` owns the `ActionMode`, overlay-priority
Back registration, state gate, clipboard metadata/write, haptic, and failure
feedback. It exposes no generic clipboard service or test seam.
`TerminalScreen`, the controller/model, key
deck, Gboard paste path, gateway, WSS, PTY, and tmux remain unchanged.

## Exact packaged-page API

The hard-cut handshake accepts only version `3`:

```json
{"kind":"PagePort","version":3,"longPressMilliseconds":500}
```

The numeric values above are examples; native supplies the current validated
platform values. Native to page:

```json
{"kind":"ClearSelection","generation":"7"}
```

Page to native:

```json
{"kind":"ImeRequested"}
{"kind":"SelectionStarted","generation":"7"}
{"kind":"SelectionAvailable","generation":"7","anchorX":0.50,"anchorY":0.50,"text":"..."}
{"kind":"SelectionCleared","generation":"7"}
{"kind":"SelectionCopyRejected","generation":"7","reason":"TooLarge"}
```

Anchors are finite normalized WebView coordinates in `[0, 1]`; native maps and
clamps them only when producing the view-local content rectangle. All objects
reject missing, extra, differently cased, wrongly typed, noncanonical,
non-finite, or unknown fields/values. A generation is the canonical decimal
rendering of an integer in `1..9007199254740991`: no sign, leading zero,
exponent, fraction, or numeric JSON value is accepted. Versions `1` and `2`,
DOM `copy`, `navigator.clipboard`, and unordered or unsolicited selection
snapshots have no decoder or fallback. The protocol is internal to one APK and
has no negotiation path.

## Hard cut and cleanup

- Replace the wheel-named xterm patch/artifact with one generic
  `xterm-6.0.0-skidbladnir` patch/artifact containing the existing wheel work
  and new selection ingress. Delete the old names, lock entries, and hard-coded
  checks; never edit generated JavaScript.
- Retain one source pin, one reviewed patch, the existing complete dependency
  audit, upstream-style focused tests, deterministic rebuild, and byte compare.
- Delete the PointerEvent/pointer-capture path, compatibility mouse/context-menu
  suppression state, and old pass-through long-press path. The sole TouchEvent
  owner prevents the trusted start before recognition; xterm/Chromium DOM copy
  handlers cannot become a second clipboard writer.
- Consolidate gesture cancellation, multi-touch draining, lifecycle reset, and
  page-command encoding with their existing owners. Remove every superseded
  branch, import, comment, and test in the same change.
- No flag, legacy decoder, alias, compatibility message, browser fallback,
  provider special case, or lab-only JavaScript bridge survives.

## Files and non-overlapping ownership

| Owner | Paths | Proof |
| --- | --- | --- |
| Root integrator | `docs/architecture.md`, `docs/roadmap.md`, `docs/terminal-touch-scroll.md`, this document, `scripts/build-terminal-xterm`, `scripts/check-terminal-assets` | Scope/acceptance authority and reproducible-generation policy |
| Android terminal builder | renamed `android/xterm-6.0.0-skidbladnir.patch`, `android/terminal.lock`, renamed generated xterm asset, terminal `index.html` and `terminal.js`, `LockedTerminalWebView.kt`, new `TerminalSelection.kt`, new `res/values/terminal_selection.xml`, `TerminalInstrumentedTest.kt` | Owns both reds and all production behavior |
| Read-only verifier | none | Diff, residue, security, accessibility, and gate review only |

The runtime slice stays with one builder because gesture arbitration, packaged
protocol, native action mode, and the only real-device red form one behavioral
boundary. No owner touches `TerminalScreen.kt`, controller/model, key deck,
gateway, tmux, profiles, `catalog/`, or `scripts/test`. Root changes test
composition only if the existing automatic discovery demonstrably misses a new
proof; no speculative edit is planned.

## Red / green / refactor

**Red 1 — pinned xterm owner:** in the source patch, add one focused
upstream-style matrix proving `StartWord -> Extend -> End` in both directions
selects the exact wrapped Unicode fixture while SGR mouse tracking is active
and emits zero data. Releasing without a final move must include the final End
cell. The same matrix owns edge auto-scroll start/stop, cancel, invalid
order/shape/coordinates, and post-disposal rejection. Observe it fail before
the xterm implementation. To avoid calling a compile error red, first add only
the smallest typed API shell that deterministically rejects all input, then run
the behavioral matrix and record its executed assertion failure before adding
selection behavior. Assertions report only case/phase, never selected text.

**Red 2 — product/platform boundary:** extend the existing real locked-WebView
instrumentation with one shared user journey exposed as two independently
runnable tests (`mouse off`, `SGR mouse on`), so one expected red cannot
prevent the other from executing. Inject native touch, hold-drag a known range,
discover and invoke the
visible `Copy` action, and prove the real Android primary plain-text clip equals
the fixture, selection/action mode clear, terminal input count stays zero, and
ordinary pre-threshold tap/vertical scroll behavior is unchanged. Copy the
prior test-device `ClipData` in memory and best-effort restore that payload in
`finally`, or clear the clipboard if it was empty; never coerce or print either
clip. Re-setting a clip cannot restore its original timestamp, source
attribution, classification, URI grants, synchronization state, or absence of
system UI, so this gate requires explicit device-clipboard mutation approval
and a dedicated test device. Observe both parameters fail against the unchanged
packaged runtime; Red 1's source-only rejecting API shell is not packaged.

In that same platform owner, drive the production selection controller through
its real `ActionMode` and `ClipboardManager` boundaries with one exact
`262144`-byte ASCII snapshot and one exact-limit snapshot containing multibyte
scalars. Prove exact clips through content-safe Boolean assertions without a
fake platform or production test seam. The pure exact decoder proof owns the
adjacent oversize and unpaired-surrogate rejection.

Extend the existing strict page-protocol proof, rather than adding a duplicate,
with one representative wrong-shape, noncanonical/mismatched generation,
unsolicited/duplicate snapshot, oversize, and invalid-Unicode case. Force a
crossed `ClearSelection(g)` to arrive after `SelectionStarted(g+1)` and prove it
cannot clear or acknowledge the newer generation. Assertions identify only
case/state/count, never content.

One selection-lifecycle matrix owns primary-tap dismissal, action-mode
dismissal/Back ordering, second contact, real `ACTION_CANCEL`, disable,
background, rotation, page failure, disposal, and fresh recreation. It also
proves output after release cannot change the copied snapshot. Existing
platform owner proofs—not duplicate assertions—must remain green for all three
wheel routes, composition-first arbitration and fresh-gesture recovery,
focus/IME/paste, modifiers, geometry, viewport, and unavailable containment.

**Corrective red — API-36 event owner:** physical feasibility must prove that
the current PointerEvent owner loses move/up after long press while one
prevented trusted TouchEvent stream retains start/move/end and emits no
compatibility mouse/context-menu tail. Add a typed, rejecting xterm tap shell
before its focused NONE/X10/VT200/SGR shape, lifecycle, coordinate, and
no-selection matrix; observe behavior fail, not compilation. Update the
existing component owners to prove exact mouse-off/SGR sub-threshold taps,
touch scroll, long-press selection, final `changedTouches`, multi-touch drain,
real cancel, lifecycle reuse, edge extension, and scripted-event rejection.
The real system-global Back proof must first show the framework floating action
mode does not intercept before the Activity fallback; green then proves the
selected-only view-scoped overlay callback clears selection/action without
invoking that fallback, and the next Back reaches the existing handler.

**Green:** implement only the contracted xterm tap/selection ingress, unified
TouchEvent owner, version-3 exact messages, native controller, resources, and
canonical ClipboardManager write. No tmux or live terminal is needed.

**Refactor:** leave one gesture machine, one selection state machine, one
clipboard writer, one page-command encoder, and one lifecycle clear path.
Mutation review must show that mouse-mode precedence, text normalization,
snapshot state gating, or zero-input containment changes break an owner
proof. Remove runnable mutations; review defensive platform-failure and honest
payload-cap branches directly.

## Acceptance and 80/20 verification

1. Long-press and hold-drag select the intended xterm cells with mouse reporting
   both off and on; selection is visible and the floating `Copy` action is
   discoverable by touch and accessibility.
2. Copy writes the exact release snapshot to the phone's real plain-text
   clipboard, once, then clears; Android supplies the only success feedback.
3. Selection/copy emit no terminal input, WSS, network, tmux, or provider
   traffic and create no durable app state or content-bearing evidence; only
   Android's action-mode, haptic, and clipboard services participate.
4. A touch beginning during active composition is wholly composition-owned and
   has zero terminal-gesture effect; any platform composition completion stays
   literal and exact, and the next fresh gesture routes normally. An unfocused
   drag does not summon the IME; the next eligible tap does, with native and
   helper focus retained and zero mouse-off input or selection. Wheel,
   touch-scroll, paste, modifier, geometry, reconnect, background, rotation,
   and unavailable behavior retain their existing contracts.
5. Only the generic pinned xterm patch/artifact and exact version-3 packaged
   protocol remain; stale names, PointerEvent selection/scroll ownership, and
   old/alternate copy paths are absent.

Pre-publication verification is the focused xterm source test and reproducible
byte-identical rebuild; the signed, same-version S22+ candidate owner proof;
`./scripts/test verify` on the joined SHA; and one hands-on S22+ pass for
selection precision, edge extension, haptic, floating-toolbar placement,
TalkBack/Switch Access discovery and invocation after a touch selection,
rotation/background clearing, and clipboard preview. The candidate run must
preserve pairing, restore the exact pinned release, and be reported as a
component run, never as `platform`.

The complete release-bound `platform --allow-device-mutation` gate runs only
after a separately scoped publication has made the immutable joined SHA and
artifacts the committed release pin. Its clean-tree, source-SHA, APK, signer,
version, and complete-suite predicates are never weakened for a feature
candidate. Publication is not part of this slice. End-to-end screen-reader
selection construction is not claimed. Without explicit current-turn approval,
device work is `NOT_RUN`, never pass. Integration, tmux, live-host,
provider-live, product, second-phone, publication, and release gates do not own
this local capability and must not run.

The builder may author the device red without approval, but no functional
production work beyond Red 1's nonfunctional type shell begins until explicit
current-turn device approval exists and the unchanged-packaged-runtime
behavioral red has executed and failed. Without approval that red is `NOT_RUN`,
never a compile failure or inferred failure.

## Non-goals, rejected designs, and trade-offs

Non-goals: Codex/Claude `/copy`; OSC 52/5522; tmux buffers/copy-mode/config;
host or multi-client clipboard routing; clipboard reads/sync/history; semantic
code-block copy; `capture-pane`; transcript parsing; OCR; Share/Open URL/Select
all; post-release drag handles or loupe; mouse/stylus/hardware-keyboard copy;
end-to-end screen-reader selection construction; one-time hints; settings;
telemetry; xterm upgrade; iOS.

- Native `ActionMode` adds a small page/native state protocol, accepted for
  platform positioning, dismissal, accessibility, and familiar interaction.
- A selected-only platform `PRIORITY_OVERLAY` Back callback is required because
  the pinned API-36 framework floating toolbar physically routes Back past
  itself. It adds one lifecycle resource but preserves view-local ownership and
  precedence over Compose without deprecated keys or Activity coupling; the
  overlay dismissal intentionally has no back-to-home predictive animation.
- The exact queued-Copy race proof invokes the real framework callback through
  narrow test-only reflection after queueing it behind a main-thread barrier.
  This couples that one oracle to Android controller structure, accepted because
  a real coordinate tap cannot establish delivery order: the targeted mutation
  survived that public-input variant. `ClipboardManager` and the rest of the
  product boundary remain real, and production exposes no test seam.
- API-36 exposes the floating Copy node with exact accessible semantics, but
  `AccessibilityNodeInfo.performAction(ACTION_CLICK)` rejected automation before
  entering the controller. The component journey therefore discovers the node
  semantically and taps its real bounds; TalkBack/Switch Access invocation stays
  in the honest hands-on gate and is `NOT_RUN`.
- The maintained xterm fork grows, accepted to keep cell/Unicode/wrap/mouse
  semantics in xterm rather than duplicating private buffer logic in app code.
- TouchEvents replace the more portable PointerEvents because the pinned
  API-36 WebView physically withholds PointerEvent move/up after long press but
  retains TouchEvents. The app accepts this Android-specific adaptation and a
  narrow semantic tap ingress over a native gesture detector, fabricated DOM
  events, timing offsets, or duplicate touch owners.
- Preventing the trusted touch start removes Chromium's compatibility tap.
  xterm therefore receives one semantic tap and remains the sole mouse-protocol
  encoder. X10 stays press-only; a completed tap may emit negotiated DEC focus
  input. Stylus behavior and generic-browser portability are not claimed.
- A gesture beginning during active composition is intentionally unavailable
  for terminal manipulation. Requiring one fresh gesture after composition
  completion is accepted to prevent a single touch from both committing text
  and scrolling, selecting, or emitting mouse input. No timer, buffer, replay,
  or forced IME finish attempts to hide that trade-off.
- Removing pointer capture makes outside-screen extension depend on Android
  View delivery and DOM Touch target retention; the physical edge-extension
  proof owns that dependency.
- Explicit Copy costs one tap, accepted to prevent clipboard clobber during
  selection adjustment.
- The `256 KiB` ceiling can reject unusually large Unicode selections, accepted
  as a bounded allocation/message policy; clearing with truthful failure is
  preferred to truncation or a fallback transport.
- Omitting post-release handles reduces refinement quality, accepted as the
  principal 80/20 cut; the initial hold-drag remains exact and edge-scrollable.
- Clearing after success prevents immediate re-copy, accepted to restore normal
  TUI touch routing and avoid stale selections.
- The immutable snapshot keeps selection usable in a continuously redrawing
  TUI, but later output may make the retained highlight show different cells.
  This visual divergence is accepted over cancelling every active selection;
  Copy is explicitly defined as the text visible when the finger released.
- Manual copies keep Android's preview, accepting shoulder-surfing/screenshot
  exposure in exchange for standard confirmation. The remote-device extra is a
  rendering hint, not a security or locality guarantee.
- The Android system clipboard may retain or synchronize text after the write;
  preventing that would require a private clipboard incompatible with ordinary
  destination apps.

Browser clipboard APIs, DOM copy, auto-copy, a persistent Compose Copy button,
a generic JavaScript bridge, terminal-screen scraping, and any server/provider
path are rejected because each creates a second authority, weaker lifecycle or
permission semantics, content coupling, or a false remote-targeting claim.
