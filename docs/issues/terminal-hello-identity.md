# terminal hello identity has no consumer

problem: the terminal hello frame still carries `paneId` and `foreground`. their
only consumers were the notification visit fences, which the hub-owned attention
cleanup deleted: a visit now acknowledges what the hub's state read at entry
captured, and the hub's equality check fences a replaced pane or agent.

impact: every terminal entry pays for identity nothing reads. the gateway's
greeting samples the pane twice and inspects its foreground process twice, the
gateway encodes the result, and the desktop and android decoders validate it.

evidence: `internal/sessions/attachment.go` `Greeting` (two pane reads, two
`paneForeground` inspections); the gateway's hello encoding;
`internal/terminal/protocol.go` `HelloFrame{PaneID, Foreground}`;
`internal/terminalclient/terminal.go` matches `terminal.HelloFrame` and ignores
it; android `TerminalConnection.kt` decodes `paneId`/`foreground` and the
controller's `onHello` ignores both. `internal/terminal`, `internal/sessions` and
`TerminalConnection.kt` were outside the hub-attention cleanup's paths.

resolved when: the hello frame carries only `attachedClients`, and the greeting
performs one pane read with no foreground inspection.
