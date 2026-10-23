# terminal hello identity has no consumer

problem: the terminal hello frame still carries `paneId` and `foreground`
(`internal/terminal/protocol.go:48`). nothing reads them: a visit acknowledges
what the hub's state read at entry captured, and the hub's equality check fences a
replaced pane or agent.

impact: every terminal entry pays for identity nothing reads. `Greeting` samples
the pane twice and inspects its foreground process twice
(`internal/sessions/attachment.go:67-85`), the gateway encodes the result
(`internal/gateway/terminal.go:105-110`), and both clients decode it: the desktop
matches `terminal.HelloFrame` and ignores it (`internal/terminalclient/terminal.go:139`),
the phone validates `paneId`/`foreground` (`TerminalConnection.kt:35-52`) and
`onHello` ignores both (`SkidbladnirController.kt:825-826`).

resolved when: the hello frame carries only `attachedClients`, and the greeting
performs one pane read with no foreground inspection.
