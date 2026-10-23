# stock xterm awaits phone acceptance

problem: stock xterm.js 6.0.0 replaced the patched fork; `terminal.js` owns wheel,
tap and selection-range routing. engineering checks and emulator journeys are its
only qualification. `screenReaderMode` stays on (`terminal.js:55`; owner,
2026-10-22) because it also selects xterm's keydown and `insertText` input path.

impact: a Gboard typing, copy-mode scroll, tap or long-press-copy regression on the
phone surfaces only in use. a lost first letter after the keyboard opens would
corrupt typed commands.

evidence: an emulator differential (api 35, Gboard) ran the patched and stock
pages on the same fixtures in a scratch activity hosting the real
`LockedTerminalWebView.kt` and `TerminalSelection.kt`, relayed over an isolated
tmux server. rendering, resize, touch scroll, alternate-screen keys, mouse taps,
selection copy, a selection held while output scrolls, and touch containment
matched, apart from the page-owned routing trade-offs the architecture's terminal
section states, a keydown-less `insertText` that is now sent, and the absent
accessibility tree. Gboard typing is not established: emulator taps drop, so the
Gboard journey passed 11 of 13 patched and 5 of 10 stock runs. with
`screenReaderMode` off, stock lost one letter of the first word after the keyboard
opened in 4 of 72 runs, against 0 of 87 with it on; afaict a timing race in xterm's
keyCode-229 path. the app accepts only a `*.ts.net` gateway (`MachineStore.kt:34-35`),
so the full app, Back, dictation, real multi-touch and live tmux through the
gateway were not exercised. physical-phone acceptance is `NOT_RUN`, owner-accepted.

resolved when: one hands-on phone pass attached to tmux with mouse on shows the
first letter typed after Gboard opens arriving every time (twenty fresh openings),
Gboard typing with suggestions, Backspace in and out of composition, Enter,
dictation and paste unchanged; a drag scrolls copy-mode at the touched cell; a tap
clicks that cell and raises Gboard; a long press highlights a word and keeps it
while output streams, a drag extends it including past the bottom edge, and
release offers `Copy` whose clip equals the highlight; Back closes that toolbar
first; a second finger or a touch starting on the 4 px padding never shows
"Reconnect required"; Gboard composition at the right edge never shifts the page;
the phone WebView supports ArrayBuffer web messages (page port version 6 requires
them, `terminal.js:819,862`). at the release that ships it, if still unobserved,
this becomes one phone line in [unrun acceptance](unrun-acceptance.md) and this
file goes.
