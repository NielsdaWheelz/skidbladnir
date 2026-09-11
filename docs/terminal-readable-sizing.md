# Readable terminal sizing

Status: implemented 2026-09-08 (merged with its paired dev-server change that
makes `window-size latest` explicit) and published in immutable `v0.2.30`,
upstream-pinned. Routine verification (`./scripts/test verify`) is green and the
deletion set is gone from production code. The `published-release` gate is
green; the MacBook, devbox, and arch hosts and the phone run `v0.2.30` (the
arch apply was the owner-run interactive step on 2026-09-10). On 2026-09-10
`scripts/fleet verify` is red on all three hosts for a reason outside this
target: dev-server now names generation directories by runtime identity
(2026-09-09) while the fleet operator reads the pinned artifact digest from
that name; that cross-repository contract break is separate scope. The owner
runtime proofs are
`NOT_RUN`: the isolated gateway+tmux integration scenario, the live fixture,
the Android instrumentation matrix (`platform`), the hands-on S22+ journey,
and the designer readability review all await explicit approval. `NOT_RUN`
is not a pass. Two tmux assumptions this contract relies on were checked
against tmux source rather than a running server: a client resize message
updates the window's latest client before sizes are recalculated, and the
`#{window-size}` format resolves the window option (falling back to the global
window option) for the targeted session's current window.
[Architecture](architecture.md) owns scope/acceptance; this document owns the
closed implementation contract. Follow [Testing Standards](rules/testing.md)
(the repository's testing-standard file) and [codebase rules](rules/index.md).

## Target and scope

The phone displays the shared terminal at a chosen readable text size. Whole
cells fit the available viewport; tmux's `window-size latest` selects the active
client's dimensions. Desktop input can return sizing to the desktop. Only input
that reaches tmux counts; local emulator scrollback and OS focus are not handoff
guarantees. Process, draft, independent pane selection, detach, and no-replay
semantics remain intact.

Ship three capabilities: saved text size, responsive whole-cell fitting, and
normal tmux sizing participation. Retain JetBrains Mono, theme, padding, key
deck, gesture/selection owners, FitAddon, PTY transport, and grouped shadows.
This supersedes the 80-column guarantee, protected desktop geometry,
Owner/Constrained presentation, and packaged-page version 3.

## Architecture and capability contract

```text
phone DataStore -> native SP conversion -> packaged xterm font
native available bounds -> FitAddon -> Resize -> existing ordered WSS
                                              -> PTY -> tmux latest -> TUI
```

**Text preference.** One application-scoped `TerminalTextSize` owner stores an
integer nominal size: default `16sp`, range `12..24sp`, step `1sp`. Reset writes
16. The setting is phone-wide and survives reconnect/recreation; orientation,
keyboard, machine, and session never rewrite it.

Use one `Context.dataStore` delegate and pinned
`androidx.datastore:datastore:1.2.1`. `Serializer<Int>` owns
`filesDir/datastore/terminal-text-size`: exactly two ASCII digits encoding
12–24, followed by EOF. Missing file means the initial default; invalid bytes
raise `CorruptionException`. No migration, corruption replacement, or pairing
storage access. Load before creating the terminal page. Serialize updates
through `updateData`; display/apply only committed state. Disable mutation
controls before page Ready and while a write is pending. Read failure blocks
initialization with an explicit reread action; write failure retains the prior value. No automatic
retry. [DataStore durability](https://developer.android.com/reference/kotlin/androidx/datastore/core/DataStore),
[release pin](https://developer.android.com/jetpack/androidx/releases/datastore).

Native computes effective CSS size once:
`TypedValue.applyDimension(COMPLEX_UNIT_SP, nominalSp, displayMetrics) / displayMetrics.density`.
Use current configuration; keep WebView `textZoom=100` and page zoom disabled.
Do not multiply by `fontScale` or `scaledDensity`. Font-scale changes recompute
the font; keyboard/rotation changes only refit its grid. Verify the CSS/native
unit correspondence on the supported WebView. [Android nonlinear scaling](https://developer.android.com/about/versions/14/features#non-linear-font-scaling).

**Viewport.** xterm owns cell measurements. Keep font-load ordering, actual
post-inset bounds, animation-frame coalescing, and integer-grid deduplication.
Fit whole cells, cap downward at `240 × 120`, and never clamp upward. Below
`20 × 5`, including zero area, publish `ViewportTooSmall` instead of Resize.
Recovery publishes Resize even when equal to the last valid dimensions.

Native retains `Pending | Fitted(columns,rows) | TooSmall` independently of
connection status. A screen-level recovery overlay must not change measured
WebView bounds; controls must remain reachable even at zero terminal height.
Before first fit, do not connect. After attachment, keep the
connection/output alive while too small; gate user touch, focus, accessories,
typing, and paste. Continue xterm's automatic terminal replies: never gate the
shared `onData`/`onInput` egress or use xterm `disableStdin`. Reuse the existing
user-ingress and input-reset owners. Recovery refits before admitting user input.

Opening text controls reuses native overlay/input-reset and selection lifecycle
handling. Android owns composition completion; never inject a key, force a
commit, or reopen Gboard on dismissal. Preserve exact input text and Android's
authority over composition through font/layout changes. A font operation emits
geometry, never user terminal bytes. Resize the existing view/attachment; retain
existing recreation behavior without replay.

**Shared sizing.** Attach with `active-pane` only. Effective `window-size latest`
is a deployment/operator prerequisite for supported windows. In the existing
identity-gated tmux queue, check the initial source window's effective policy
before shadow creation; a mismatch creates no resources and maps to the existing
content-free `InternalError`. Do not mutate/save/restore options, scan all
windows, add a watchdog, or synthesize handoff input. Later navigation follows
ordinary tmux configuration; conflicting overrides are unsupported.

Require the first WSS client frame to be a valid Resize before `OpenTerminal`
creates a shadow/PTY. Use existing bounded frame parsing and a named
`terminalInitialResizeTimeout = 5s`. Start the PTY at those dimensions; delete
the guessed `80 × 24` startup size. Revalidate target identity at creation.
Invalid/oversized first frames, timeout, or close create no attachment resources;
an initial Detach closes cleanly. Reuse current error/close handling. Hello still
precedes server terminal bytes. Subsequent resizing/input use the existing
ordered queue; no lease, acknowledgment protocol, or geometry headers.

## Exact schemas and composition

| Boundary | Single accepted shape / API |
| --- | --- |
| Native → page initialization | `{"kind":"PagePort","version":4,"longPressMilliseconds":500,"fontSizeCssPx":16}`; numeric examples are platform-derived |
| Native → page size update | `{"kind":"FontSize","fontSizeCssPx":16}`; finite positive number |
| Page → native usable geometry | Existing `{"kind":"Resize","columns":40,"rows":18}` within `20..240 × 5..120` |
| Page → native insufficient room | `{"kind":"ViewportTooSmall"}` |
| Gateway → phone presence | Exactly `{"kind":"Hello","attachedClients":2}` or `{"kind":"Presence","attachedClients":2}`; positive integer count |
| Phone → gateway geometry | Existing `{"kind":"Resize","columns":40,"rows":18}`; also mandatory first client frame |
| Kotlin page API | Extend `TerminalPage` with typed font update; extend `TerminalPageListener` with too-small notification. Existing Resize callback means Fitted. |

All decoders reject extra/missing keys, wrong types, unknown variants, and
numeric-string coercion. Page version 4 is the only accepted version; existing
messages otherwise retain their exact contracts. No WSS font/policy field,
protocol negotiation, new endpoint, generated client, or compatibility reader.

`Ready` retains bridge-readiness meaning and clears its startup timer even when
the viewport is too small. Only `Ready + Fitted` starts WSS: queue the measured
Resize before `TerminalConnection.start()`. Reuse its existing open-time flush
and resize-before-input ordering. Geometry recovery reuses the active connection.
Late events remain attempt-gated. A too-small event never reaches the gateway.

## Designer-owned content

The product/content designer owns every row below; builders implement these
literal strings using existing native surfaces, tokens, and accessible controls.
Good content states the action/consequence, exposes the current value, and never
claims permanent sizing authority. No copy-generation framework or new theme.

| Feature | Content and interaction acceptance |
| --- | --- |
| Text size | Header `Aa`, spoken `Terminal text size`, between identity and Kill; ≥48dp target. Sheet title `Text size`; `−`, current nominal integer, `+`; spoken `Decrease terminal text size`, `Terminal text size, N`, `Increase terminal text size`. Bound buttons disable correctly. |
| Saved setting | `Reset to 16`, disabled at 16; `Done`. `Saved on this phone. Android’s text-size setting also applies.` Applied changes survive dismissal; no Save/Cancel or editable numeric field. |
| Shared sizing | Sheet: `Screen size is shared with other attached terminals.` Connected header: `1 client` / `N clients`; retain existing connection-failure presentation. |
| Insufficient room | `Terminal needs more room`. `Hide the keyboard, reduce text size, or make the app window larger.` Actions: `Hide keyboard` when visible; `Text size` always. Keep Detach accessible; no Retry or automatic shrinking. |
| Preference failure | Read: `Text size unavailable.` and `Retry` (reread only). Write: `Text size could not be saved.`; previous committed size remains. |

Use existing sheet/notice treatment and Back precedence. At system text scale
2.0, controls remain reachable without clipped labels; identity may ellipsize
visually while retaining its full accessible name. The designer checks reading
comfort, action discovery, and visible final cells on the actual phone.

## Non-overlapping implementation ownership

Paths below are exhaustive assignments, not permission to refactor neighboring
features. Root must assign any discovered additional path before it changes.

| Owner | Files |
| --- | --- |
| Root integrator | This document, `docs/architecture.md`, `docs/roadmap.md`; only required supersession references in `docs/terminal-theme.md`, `docs/terminal-key-deck.md`, `docs/terminal-selection-copy.md`. `scripts/test` only if existing discovery misses a proof. |
| Host/transport builder | `internal/tmux/attachment.go`, `internal/tmux/client_test.go`; `internal/sessions/attachment.go`, `internal/sessions/attachment_test.go`; `internal/gateway/terminal.go`, `internal/gateway/terminal_test.go`; `internal/terminal/protocol.go`, `internal/terminal/protocol_test.go`, `internal/terminal/queue_test.go`; `tests/integration/terminal_test.go`, `tests/live/tmux_grouped_live_test.go`. |
| Android builder | `android/app/build.gradle.kts`; `android/app/src/main/assets/terminal/terminal.js`, `terminal.css`; main Kotlin `TerminalTextSize.kt` (new), `TerminalScreen.kt`, `LockedTerminalWebView.kt`, `ProductModel.kt`, `TerminalConnection.kt`, `SkidbladnirController.kt`; corresponding test files listed below. |
| Product/content designer | Owns the content table and hands-on design review; no builder-file edits. |
| Independent verifier | Read-only; no production files or tests. |

Kotlin paths are under `android/app/src/main/java/dev/niels/skidbladnir/`.
Android test ownership: existing JVM `ProductContractTest.kt`,
`MultiMachineContractTest.kt`; existing instrumentation
`TerminalInstrumentedTest.kt`, `TerminalChromeInstrumentedTest.kt`,
`MultiMachineUiInstrumentedTest.kt`. They live under the corresponding
`src/test/java/` and `src/androidTest/java/` package directories. Extend these
owners and `src/debug/java/dev/niels/skidbladnir/TerminalTestActivity.kt` for the
changed page/listener contract; add no parallel terminal harness. CSS changes
require an observed box-model failure. One Android builder keeps preference, bridge, and input
arbitration changes together.

Hard-delete `ignore-size`, `OwnsGeometry`, `Geometry`/`TerminalGeometry`, the
gateway mapper, geometry presence fields/labels, font auto-shrink constants and
loop, upward fit clamps, guessed PTY dimensions, version-3 decoder, and their
superseded assertions/comments. Keep one fitter, one stored preference, one
SP conversion, and one resize transport. Do not generalize credential rollback,
duplicate native cell measurement, edit generated xterm assets, or upgrade xterm.

## Red / green / refactor and acceptance

Before implementation, run routine baseline verification. Each builder writes
and observes its own behavioral red before changing production behavior. A
missing symbol/compile error is not red. Do not mock owned components or use
source-text assertions to prove sizing. Extend existing fixtures; assertions
identify case/dimensions/counts, never terminal bytes or credentials.

| Ownership boundary | One owning proof and acceptance |
| --- | --- |
| Strict transport | Existing Go encoding and Kotlin decoding tables own count-only presence, geometry bounds, and exact shapes; extend the existing packaged-page decoder matrix for v4. No separate tests solely preserving dead formats. |
| Gateway + tmux | One real gateway/WSS/isolated-tmux scenario: small first Resize → phone-sized shared window → desktop input → desktop-sized window → phone input/resize → phone-sized window. Account for tmux status rows; preserve pane/PID/draft, independent selection, unrelated sessions, and detach lifetime. Same owner covers invalid startup, policy mismatch, and zero created resources. Run the same proof on Linux/Darwin; remove duplicate sizing claims from the live fixture. |
| Android storage | One real DataStore adapter case inside the component suite: change/reset, reopen/recreate, durable value, and a real storage failure with truthful error/unchanged committed value. No fake DataStore, credential transaction, or separate persistence framework. |
| Android presentation | One real production-layout Compose/locked-WebView matrix: 16sp default; smaller/larger/reset; final full row/column including bold/wide glyphs; keyboard/rotation with unchanged nominal size; system scaling; Ready/Resize ordering; TooSmall/recovery at unchanged last-valid geometry; continued automatic replies and blocked user ingress. Reuse existing IME, selection, modifiers, and touch owners for collateral behavior. |

Green implements only the contracted paths. Refactor removes the deletion set
and consolidates through the named existing owners; rerun the owning proofs.
Review that restoring auto-shrink, skipping initial geometry, or suppressing
too-small recovery would fail their behavioral assertions.

The joined candidate needs `./scripts/test verify`, the two owner runtime
proofs, and one hands-on S22+ journey through actual Codex/Claude text, menus,
approvals, diffs, selection, Gboard/dictation, and desktop handback. Use the
existing production layout and isolated test-created session; no new E2E
framework or duplicated provider matrix. Default 16sp is accepted only after
that readability check; compare 14/16/18 through the implemented control.

Every tmux/integration/live invocation and every device/ADB operation requires
explicit current-turn approval under `AGENTS.md`; otherwise report `NOT_RUN`.
Only isolated test-created socket/session identities may be resized or removed.
Signed candidate component runs preserve pairing and restore the pinned APK;
they are not release-bound `platform` passes. Publication/deployment and their
gates are separate scope. No required unrun proof is a pass.

## Explicit trade-offs and non-goals

- Readable text costs columns; keyboard costs rows; simultaneous devices can
  alternate shared dimensions. Narrow-width TUI defects require a supported
  larger viewport or chosen smaller size, never hidden shrinking.
- 16sp and 12–24sp are bounded starting product choices, not scientific optima.
  Maximum system scaling can require the recovery surface.
- One DataStore dependency buys serialized durable writes and surfaced failure;
  it avoids inventing file transactions or involving pairing storage. Applying
  after commit trades a brief disk wait for truthful saved state.
- `Aa` uses header width; identity may ellipsize. Explicit buttons cost taps;
  the unchanged key deck retains its height. Initial attach waits for usable
  geometry, and custom tmux policy overrides require operator correction.
- Reflow can move TUI content. Exact ebook-style reading-position preservation
  across arbitrary application redraws is not promised.

Non-goals: pinch, canvas pan/zoom, overview mode, auto typography, fullscreen or
key-deck redesign, per-session profiles, fonts/themes/spacing controls, tmux
policy management, semantic readers, transcripts/OCR, provider APIs/hooks,
history/replay, notifications, new fleet topology, telemetry, settings platform,
compatibility paths, and unrelated cleanup.
