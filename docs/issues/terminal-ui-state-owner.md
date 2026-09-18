# misplaced terminal screen state

problem: the controller file owns the terminal screen's shared state vocabulary
and control availability, despite the screen and controller both consuming it.

evidence: `TerminalUiStatus`, `TerminalViewport`, and the five terminal action,
input, page and text-size admission/projection helpers precede the controller.
`TerminalScreen` consumes them throughout its rendered controls; the controller
uses the same rules before acting. they require no controller internals.

resolved when: move that unchanged block into the existing `TerminalScreen.kt`
feature owner, without new interfaces, visibility changes or a new state file.
verify exact declarations, consumers and compiled behavior. the screen file
will own shared feature policy alongside rendering, as session rename already
does. retain attachment ordering, mutation fences, access-loss recovery, forge
carry and credential reconciliation with the controller that coordinates them.
