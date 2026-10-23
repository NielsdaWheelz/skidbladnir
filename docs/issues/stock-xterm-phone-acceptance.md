# stock xterm awaits phone acceptance

problem: the 2026-10-21 cut to stock xterm.js 6.0.0 moved wheel, tap and
selection-range routing into `terminal.js`. engineering checks and emulator
journeys are its only qualification. `screenReaderMode` stays on (owner,
2026-10-22): with it off the emulator showed a Gboard first-letter loss.

impact: a Gboard typing, copy-mode scroll, tap or long-press-copy regression on
the phone surfaces only in use. a lost first letter after the keyboard opens
would corrupt typed commands.

evidence: an emulator differential (api 35, Gboard) ran the patched and stock
pages on the same fixtures inside a scratch activity hosting the real
`LockedTerminalWebView.kt` and `TerminalSelection.kt`, with a relay in the
gateway's pty role over an isolated tmux server. rendering, resize, touch
scroll, alternate-screen keys, mouse taps, selection copy, a selection held
while tmux scrolls output, and touch containment matched, except the trade-offs
in [touch scroll](../terminal-touch-scroll.md) and [selection
copy](../terminal-selection-copy.md), plus a keydown-less `insertText` that is
now sent and the absent accessibility tree. Gboard typing is not established.
its on-screen taps drop on the software-rendered emulator, so the Gboard
journey passed in only 11 of 13 patched runs and 5 of 10 stock runs, with
Backspace or Enter missing a tap on both builds. separately, stock lost one
letter of the first word typed after the keyboard opened in 4 of 72 runs (2
journey runs, 2 of 62 event-traced runs in which Gboard delivered both keydowns
and both `insertText` events), against 0 of 87 runs with `screenReaderMode` on
(patched, and stock with only that option flipped). afaict xterm drops the
letter in either mode when its keyCode-229 `setTimeout` reads the textarea
before the `input` event lands, so the difference looks like timing, not logic
(p about 0.04); the owner kept the option on. physical-phone acceptance is `NOT_RUN`, owner-accepted. the
full app, its Back, dictation, real multi-touch and live tmux through the
gateway were not exercised: the app accepts only a `*.ts.net` gateway.

resolved when: one hands-on phone pass attached to tmux with mouse on shows the
first letter typed after Gboard opens arriving every time (twenty fresh
openings), Gboard typing with suggestions, Backspace in and out of composition,
Enter, dictation and paste unchanged; a drag scrolls copy-mode at the touched
cell; a tap clicks that cell and raises Gboard; a long press highlights a word
and keeps it while output streams, a drag extends it including past the bottom
edge, and release offers `Copy` whose clip equals the highlight; Back closes
that toolbar first; a second finger or a touch starting on the 4 px padding
never shows "Reconnect required"; Gboard composition at the right edge never
shifts the page; the phone WebView supports ArrayBuffer web messages (the page
port's version 6 requires them). at release, if still unobserved, this becomes one
phone line in [unrun acceptance](unrun-acceptance.md) and this file goes.
