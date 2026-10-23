# terminal hello identity has no consumer

problem: the terminal hello frame still carries `paneId` and `foreground`
(`internal/terminal/protocol.go:48`). nothing reads them: a visit acknowledges
what the hub's state read at entry captured, and the hub's equality check fences a
replaced pane or agent.

impact: every terminal entry pays for identity nothing reads. `Greeting` samples
the pane twice and inspects its foreground process twice
(`internal/sessions/attachment.go:67-84`), the gateway encodes the result
(`internal/gateway/terminal.go:105-110`), and the desktop decodes it
(`internal/terminal/protocol.go:145-146`) only to match `terminal.HelloFrame` and
ignore it (`internal/terminalclient/terminal.go:139`). the phone reads only
`attachedClients` (`TerminalConnection.kt`).

cutover: an apk older than terminal-trust requires `paneId` and fails every attach
on a hello without it. dropping the members in the release that first ships
terminal-trust's apk makes that release a
[lockstep cutover](../architecture.md#lockstep-cutovers); in any later release it
needs no install order.

resolved when: the hello frame carries only `attachedClients`, the greeting
performs one pane read with no foreground inspection, and `terminal.HelloFrame`
holds neither member.
