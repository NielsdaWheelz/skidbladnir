# keyboard animation resizes the shared tmux window repeatedly

problem: raising or hiding gboard on the phone terminal can send a burst of
`Resize` frames instead of one. under tmux `window-size latest` each frame makes
the phone the window's latest client and resizes the shared window, so the
desktop client and the foreground tui redraw once per step.

impact: transient reflow on the desktop and the phone, and extra repaint work
for codex and claude on every keyboard toggle. a tui that leaks a copy of its
screen into scrollback on resize repeats the leak per step. the final geometry
is correct; only the intermediate steps are waste.

evidence (source-derived, not measured):

- `TerminalScreen` sizes the terminal from compose window insets, which follow
  the ime animation frame by frame, so the webview's bounds change on each
  animation frame.
- `terminal.js` refits on every `ResizeObserver` callback, coalesced per
  animation frame, and publishes `Resize` whenever the integer grid changes; it
  has no settle step.
- `SkidbladnirController.resizeTerminal` forwards each published grid
  unchanged; nothing coalesces natively. the former `imePadding` layout behaved
  the same, so this predates the terminal rail.
- tmux updates the window's latest client before recalculating sizes
  ([readable sizing](../terminal-readable-sizing.md) records this from tmux
  source), so every frame in the burst claims the window for the phone.
- prior art (read from source by a research pass, not run here): vs code's
  `terminalResizeDebouncer.ts` resizes rows at once and debounces columns;
  connectbot's compose console sizes from the ime animation target and
  suspends pty resizes during transitions.

resolved when: one owner publishes geometry. size the terminal box from the ime
animation's end state (or hold publication while an inset animation runs) and
publish one `Resize` per settled grid, keeping the initial resize immediate.
verify by counting `Resize` frames per keyboard toggle at the gateway: exactly
one per settled change, none for an unchanged grid. device and live-gateway
checks require current-turn approval.
