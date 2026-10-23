# terminal shift specification and plan

status: accepted design, 2026-10-20; implemented and qualified 2026-10-21 on
`feat/terminal-shift`.

## goal and scope

make codex's pending questions reachable from the phone: tap shift, then ← to
open/cycle questions; re-arm shift, then → to step back towards the prompt. the same
generic keys support claude's shift+tab mode control. providers interpret keys;
skid never detects a question to decide what input to send.

the owner chose a one-shot `Shift` key replacing `-`, affecting custom-bar
navigation and tab only. gboard owns text, punctuation and capitalisation.
this supersedes the changed matrix, modifier, width and page-version clauses in
[key deck](terminal-key-deck.md) and [readable sizing](terminal-readable-sizing.md).
their other contracts remain; [testing policy](rules.md#testing) governs proofs.
[architecture](architecture.md) owns shared invariants.

## target interface and content

```text
Esc   /     Shift Home   ↑     End    PgUp
Tab   Ctrl  Alt   ←      ↓     →      PgDn
```

one fixed `2 × 7` deck; `Shift` occupies row-major index `2`. all other cells
retain their order, labels and behaviour. hyphen remains available through gboard.

the product/content/accessibility designer owns this content contract and its
rendered review. good content uses conventional key names, exposes actual state,
and stays readable without provider jargon or explanatory chrome.

| element | exact content and behaviour |
| --- | --- |
| new key | visible `Shift`; off or armed like ctrl/alt |
| armed | existing gold text/fill and `2dp` border; state follows the page snapshot |
| disabled | all modifiers render off; no dispatch or haptic |
| activation | existing button, keyboard-tap haptic and ≥`48dp × 48dp` target |
| layout | existing mono typography, `4dp` outer padding, `2dp` gaps and equal measured cell widths; both rows share horizontal overflow |

keep labels on one line without clipping, shrinking targets or reducing type.
`356dp` is only the minimum geometric footprint: five-letter `Shift` can require
wider measured cells. content measurement determines the actual no-scroll width.
check the supported phone at font scales `1.0` and `2.0`; terminal sizing retains
its existing fitted-grid/recovery contract, with no restored 80-column promise.

## modifier lifetime

`terminal.js` alone owns `{control, alt, shift}`, each `Off | Armed`.

1. tapping a modifier toggles only itself, emits no terminal input and consumes
   no other modifier. all eight combinations are valid; no lock or hold mode.
2. encode a non-modifier accessory from one synchronous snapshot. consume all
   three modifiers before publishing its single existing `Input` message.
3. any existing page `Input` emission consumes all modifiers. shift changes
   only custom-bar arrows, home/end, page up/down and tab. slash, escape, typed
   text, ime/composition, dictation, paste, unicode and hardware input acquire
   no additional shift effect. existing ctrl/alt eligibility is preserved.
4. therefore shift → typed `a` sends `a` and disarms shift. local-only scroll
   preserves modifiers; scroll or xterm replies that emit `Input` consume them
   through their existing route, without applying deck shift.
5. when consumption changes armed state, publish one atomic all-off snapshot
   before `Input`. unchanged all-off snapshots remain deduplicated. on port
   bind, publish all-off before `Ready` unconditionally.
6. extend existing reset paths to all three fields: page/connection failure,
   disconnect/reconnect, window-focus loss, backgrounding, rotation/recreation,
   disabling user input, close confirmation, detach/back/navigation and disposal.
   reset emits no terminal input. deck ↔ webview focus movement preserves state.
   reset/disable cannot resurrect an old armed snapshot on re-enable.

compose only mirrors snapshots for the current attachment attempt. it never
optimistically toggles, persists or derives modifier truth from gboard.

## terminal encoding

`ESC` means `0x1b`; `HT` means `0x09`; `CSI` means `ESC [`.
spaces in the notation below are explanatory and are never emitted.
use `m = 1 + S + 2A + 4C`, where armed shift/alt/control contribute `1`, else `0`.

| armed modifiers | none | shift | alt | shift+alt | ctrl | shift+ctrl | alt+ctrl | all |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `m` | `1` | `2` | `3` | `4` | `5` | `6` | `7` | `8` |

| accessory | emitted value |
| --- | --- |
| left/up/down/right/home/end, no modifiers | existing normal/application cursor sequence, selected by xterm's cursor mode |
| same, any modifier | `CSI 1 ; m D/A/B/C/H/F`, respectively; independent of cursor mode |
| page up/down, no modifiers | `CSI 5 ~` / `CSI 6 ~` |
| page up/down, any modifier | `CSI 5 ; m ~` / `CSI 6 ; m ~` |
| tab, none or ctrl | `HT` |
| tab, alt or ctrl+alt | `ESC HT` |
| tab, shift or shift+ctrl | `CSI Z` |
| tab, shift+alt or all | `ESC CSI Z` |
| slash/escape | existing ctrl-then-alt composition; shift ignored but consumed |

navigation is already encoded in `accessoryInput()`; retain its
`modifiersEligible:false` result so `acceptInput()` cannot apply ctrl/alt twice.
tab chooses `HT` or backtab `CSI Z`, then uses existing literal ctrl/alt
composition once. do not apply navigation's parameter formula to tab.
retain `keyIsProven()` and its ctrl/alt rules; never add shift to text transforms.

the [xterm reference](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html)
defines modified navigation and backtab. the pinned physical-key evaluator
collapses shift+tab to backtab even with ctrl/alt; our deck instead preserves its
existing alt-prefix composition. that deliberate combined-tab difference is
bounded to the table above; do not alter vendor xterm to erase it.

## ownership and exact page api

```text
TerminalKeyDeck → controller attempt/admission gate → LockedTerminalWebView
                                                        ↓ Accessory
                                     terminal.js ordered input owner
                                           ↓ ModifierState    ↓ Input
                                       native snapshot     existing WSS
                                                              ↓
                                                      gateway PTY → tmux → TUI
```

reuse `sendAccessory()`, `resetInputState()`, `onModifiersChanged()` and
`onInput()`; add no method, queue, controller reducer or alternate input route.

```text
TerminalAccessory = Escape | Slash | Shift | Home | Up | End | PageUp |
                    Tab | Control | Alt | Left | Down | Right | PageDown
TerminalModifierPhase = Off | Armed
TerminalModifiers(control: TerminalModifierPhase,
                  alt: TerminalModifierPhase,
                  shift: TerminalModifierPhase)
```

the packaged page port hard-cuts from actual version `4` to `5`:

```json
{"kind":"PagePort","version":5,"longPressMilliseconds":500,"fontSizeCssPx":16}
{"kind":"Accessory","key":"Shift"}
{"kind":"ModifierState","control":"Off","alt":"Off","shift":"Armed"}
```

the first two messages are native → page; the third is page → native.
initialisation numbers above are examples of existing platform-derived fields.
terminal-page (2026-10-22) moved the port to version 6;
[selection copy](terminal-selection-copy.md#exact-packaged-page-api) lists it.
`Accessory.key` accepts exactly the enum above; the three modifier fields are
required and each accepts exactly `Off` or `Armed`. `ResetInputState`, `Ready`
and `Input` retain their current shapes. retain strict missing/extra-field,
type and enum validation and the existing unavailable/page-failure outcome.
reject version `4`, `Hyphen` and two-field snapshots; no default shift, alias,
compatibility decoder or version negotiation. kotlin and assets ship in one apk.
this internal version is independent of gateway, websocket and device protocols.

## implementation slices and deletion

paths below are exclusive assignments. kotlin paths are relative to
`android/app/src/main/java/dev/niels/skidbladnir/`.

| owner | files and responsibility |
| --- | --- |
| designer | content table above and rendered/accessibility review; supplies content to root; no builder-file edits |
| terminal boundary builder | `android/app/src/main/assets/terminal/terminal.js`, `LockedTerminalWebView.kt`, `TerminalScreen.kt`: state/encoding, enum, strict snapshot, port version, all-off initial mirror |
| deck builder | `TerminalKeyDeck.kt`: replace third cell and extend existing modifier-state presentation |
| android acceptance author | temporary `android/app/src/androidTest/java/dev/niels/skidbladnir/TerminalShiftAcceptanceTest.kt`, `android/app/src/debug/java/dev/niels/skidbladnir/TerminalShiftAcceptanceActivity.kt`, debug manifest/gradle setup and debug tls resources named below |
| live fixture author | temporary `internal/gateway/terminal_shift_acceptance_test.go` and `scripts/tmp-terminal-shift-live.py` for joined/provider proof |
| root integrator | this spec, narrow supersession notes in the two predecessor specs, architecture/roadmap and `docs/issues/terminal-shift.md`; integrate slices, record qualification and remove temporary support |
| independent verifier | reads contracts, diffs and proof design/results; writes no tests or production files |

the temporary gateway fixture composes `sessions.New` with `SocketName` and
`Gateway.ServeHTTP` under a temporary tls listener, with disposable auth,
checkpoint and working-directory state. create the exact owned `tmux -L`
session externally, then use ordinary inventory/identity admission. do not call
`gateway.ListenAndServe` (starts recovery) or `manager.Create` (launches the go
test binary's `os.Executable()` as a terminal helper). add no production seam.
guard the fixture with build tag `terminal_shift_acceptance` and invoke only its
named test after approval; ordinary `go test ./...` must never start tmux.

use a separate qualification app id and disposable provider homes. seed that
app's complete collection through `MachineStore.installFixedFleet`, using exact
`FLEET_LABELS` order `["Arch", "Devbox", "MacBook"]`: one localhost target, two
distinct reserved `.invalid` peers and distinct disposable identities/bearers.
let the real controller admit the target through its normal
dashboard/open path; the other peers become unavailable normally. mount the
production `TerminalScreen`; do not wire accessory callbacks directly to a socket.

phone origin is `https://localhost:8443/`. wrap the real gateway with a
localhost-valid leaf certificate from a disposable ca. acceptance owns temporary
`android/app/src/debug/res/xml/terminal_shift_network_security.xml` and
`android/app/src/debug/res/raw/terminal_shift_ca.crt`; trust that ca for localhost
only in the qualification build. retain certificate/hostname verification and
cleartext prohibition. use approved `adb reverse --no-rebind tcp:8443
tcp:<fixture-port>`; refuse an occupied mapping. no production trust change,
system/user ca installation or production phone data access. provision fixture
credentials privately, never through instrumentation arguments or logs.

delete `Hyphen` from the enum, deck and page literal table. replace two-field
snapshots/initialisers and version-4 page-port checks. update conflicting active
predecessor clauses or mark them superseded; retain source-attributed history.
reuse the current measured grid, button, modifier helper, reducer and bridge.
no generic key framework or broad file cleanup is justified.

`SkidbladnirController.kt`, `TerminalConnection.kt`, host production code,
xterm vendor/patch, public schemas, dependencies and `scripts/check`
composition need no production changes.

## sequence and acceptance

1. adversarial scope/content review: confirm the owner's choices, exact tab
   table, one-shot lifetime and measured-width trade-off. settle schema and
   ownership before builders start; a reviewer cannot authorise scope changes.
2. acceptance author writes the smallest temporary real integration/live proof.
   no retained android runner exists: add only the change-specific temporary
   instrumentation/fixture setup needed, then remove it with the tests.
   reviewer checks its real boundary and failure oracle before execution.
3. establish **RED** on unchanged production source after terminal readiness:
   discover `Shift` through accessibility, or tap the original third cell then
   left and assert the controlled receiver's expected shifted-left action.
   keep this core proof baseline-compatible: importing the future `Shift` enum
   or `shift` field makes compilation failure, not behavioural red.
4. implement the two builder slices against the agreed api; integrate together.
   run the same core proof **GREEN**, then the compact matrix below.
5. verifier challenges sensitivity to a missing shift bit, double encoding and
   a modifier that fails to clear. inspect/refactor at the responsible layer;
   rerun affected proofs after changes. designer reviews rendered content.
6. record exact source, environment, behavioural red/green results and remaining
   limits here. delete temporary tests, fixture, runner, dependencies, manifest
   entry, gradle changes, tls resources and driver; preserve the existing debug
   gallery/manifest. remove only owned reverse mapping, qualification/test apks,
   certificates/keys, provider processes/daemons, tmux resources and temp homes.
   run `scripts/check verify android` after cleanup. it proves engineering
   integrity, not behaviour. delete the issue only when resolved.

| proof boundary | required acceptance |
| --- | --- |
| real compose + locked webview | exact matrix; shift/ctrl/alt independent and double-tap disarm; page-owned atomic snapshots; all-off before ready and, when armed, before input; no optimistic state |
| real page encoding | all eight masks for navigation and tab; unmodified normal/application cursor modes; each navigation suffix/page key; exact-once emission and unchanged unshifted ctrl/alt behaviour |
| same input owner | shift → slash/escape/typed text/ime commit/paste/unicode clears without shifting text; ctrl/alt retain their existing proven-key/literal rules; local-only scroll preserves state and scroll/reply `Input` consumes it |
| native/page boundary | reject old version, hyphen, missing/extra/invalid modifier fields; disabled/failure/reconnect and existing reset boundaries clear all three fields; focus within deck/webview preserves them |
| joined phone → WSS → real gateway → PTY → isolated tmux | controlled in-memory receiver proves shifted left/right/tab arrive exactly once and next unarmed key is plain; compare bytes in memory only |
| installed providers on owned isolated sessions | codex with two nonblocking pending questions: shift+← enters question 1, re-armed shift+← reaches question 2, re-armed shift+→ returns to question 1, another re-armed shift+→ returns to the prompt, then plain arrow stays plain; claude ordinary question remains navigable and shift+tab changes mode once |
| physical phone and designer | font scales 1.0/2.0, shared overflow at narrow width, ≥48dp targets, no clipping, armed/disabled appearance, haptics and talkback state; gboard capitalisation remains independent |

stage provider requests with disposable local model endpoints using the existing
[codex](terminal-observation-codex.md) and [claude](terminal-observation-claude.md)
recipes; painted frames alone cannot prove interaction. test the installed
versions and record them; do not expand this into provider-family qualification.
codex needs an actual `request_user_input_async` call containing two questions,
with the turn kept running after the tool returns. the default-mode
`request_user_input` feature opens a different, auto-resolving view; it and an
already-open blocking plan-mode dialog do not reproduce this bug. the
[installed-version handler](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/core/src/tools/handlers/request_user_input_async.rs)
defines the async tool's schema.
the owner accepted the installed 0.163 navigation sequence on 2026-10-21:
backward navigation visits prior questions before collapsing to the prompt.
[stock navigation](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/chatwidget/questions.rs)
owns that behaviour; skid sends each requested key once.

no-device, missing provider state, missing permission, runner/build failure or
unexecuted boundary is `NOT_RUN`, never red or pass. each future tmux/device run
requires explicit current-turn approval under `AGENTS.md`. never target any
session/pane except the exact one the fixture created on its isolated socket.
do not recreate retired gates. retain only content-free case outcomes/counts,
versions and source references; never bytes, prompts, objectives or credentials.

## qualification, 2026-10-21

behavioural source: `4b6b3ff815161c76569b21e70b6efbe25ea2dd57`, based on
`b104979da9599217a3c845a97e2bee3010f3218f`. integration rebases only the shift
commit onto `bc93dd2`; all four affected production files remain byte-identical
to the qualified source. engineering verification covers the integrated tree;
live/device qualification was not repeated after rebase. the temporary fixture
recipe above records the original fixed-fleet baseline; standalone generalization
owns the current fleet contract.

device: samsung s22+ (`SM-S906W`), android api `36`, webview
`155.0.8059.43`. font qualification uses activity-local configuration. owner
manual checks temporarily enabled touch vibration/talkback and restored both
off; the installed production app remains untouched.

- behavioural **RED**: baseline reached the real terminal through the dashboard,
  controller, WSS, gateway and isolated tmux, then failed because `Shift` was
  absent. the same core proof is **GREEN**, including shifted-left then plain-left.
- **GREEN**: 152 page-encoding cases; six invalid native snapshots; three page
  protocol cases; local scroll and alternate-screen input; composition before
  and after paste, native commits, unicode, dictation-shaped text, hardware,
  paste, terminal replies, reset/disable/focus/rotation, armed failure/disposal.
- **GREEN**: 13 joined modifier cases and real connection failure, frozen all-off
  state, explicit reattachment and plain input through the controlled receiver.
  five joined lifecycle cases cover backgrounding, recreation, close/cancel,
  detach and android back, each followed by plain input.
- **GREEN**: installed codex `0.163.0` follows all six approved question states;
  claude `2.1.306` follows five question/composer/mode states. actual native tools
  use disposable local model endpoints/homes; this is installed-version evidence.

rendering passes at font scales `1.0` and `2.0`: native targets remain at least
`48dp`, both rows scroll together, local deck scrolling preserves shift, and
disabled controls show all-off and dispatch nothing. the tested phone needs
slight shared horizontal overflow even at scale `1.0`; rightmost keys become
fully readable at the scroll end, without clipping text inside cells.
independent designer review passes all eight key-only crops across both scales.
the owner confirms tactile feedback, spoken shift/off/armed state, lowercase
input consuming bar shift and independent gboard capitalisation all pass.

temporary tests, fixtures, runner/dependencies, manifest additions, debug trust
resources and driver were removed before commit. original phone settings were
verified restored; qualification apps, owned reverse mapping, sessions/processes,
model homes, certificates/keys and scratch were removed. gradle/debug manifest
match the baseline and the existing gallery is preserved.

`scripts/check verify android` passes after `:app:clean`; this is engineering
evidence only. independent final review found no blockers. the temporary proofs
above qualify this source and leave no retained behavioural regression suite.

## costs and exclusions

- replacing `-` costs direct bar access to hyphen; gboard still supplies it.
  this preserves deck height, column count and target size. the word `Shift` can
  introduce shared horizontal overflow on narrow screens.
- one-shot operation requires re-arming for each shortcut; intervening emitted
  input, including terminal replies, consumes shift. preserving the existing
  input owner avoids a second sticky-modifier lifecycle.
- alt+shift+tab deliberately differs from physical xterm as specified above.
- deleting temporary proofs leaves no automatic behavioural regression suite;
  retain honest qualification, not a new harness.

no provider-specific question button/detection, native question ui, typed-text
shift mapping, keyboard remapping, synthetic android/dom modifiers, modifier
lock/repeat/timeout, long-press menu, persistence, settings, new endpoint,
websocket change, xterm upgrade, telemetry, release or deployment work.
