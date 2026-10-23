# canceled browser shutdown races its terminal input reader

problem: bubbletea v2.0.9's killed shutdown closes cancelreader before its input
worker has joined, and ultraviolet's pinned `StreamEvents` returns without joining
its stream worker. `sessionui` passes its caller context to `tea.WithContext`
(`internal/sessionui/session.go:100`), so canceling the browser takes that path.

impact: canceling a browser can race `os.File.Close` against `Fd` on darwin's
kqueue reader, and the input worker can outlive `Program.Run`. no crash, lost input
or production occurrence is known.

evidence: 2026-10-20, a temporary pty browser run under `go test -race` reported
the race during cancellation: `tea.go:1265` closed cancelreader's signal pipe while
`terminal_reader.go:124` and `cancelreader_bsd.go:139-141` read its descriptor.
pins (`go.mod`): bubbletea v2.0.9, ultraviolet `f5a850f9c2b7`, cancelreader
v0.2.2. upstream as of 2026-10-20: ultraviolet
[`0b88c25f3fff`](https://github.com/charmbracelet/ultraviolet/commit/0b88c25f3fff665a5f9dfd226ee71868f5e8d51a)
joins its worker; bubbletea v2.1.0 still skips the reader wait on killed shutdown
(its graceful wait times out at 500 ms); cancelreader head keeps the bsd reader.
upgrading ultraviolet alone, or routing cancellation through quit, is incomplete.

reproduce: in a private pty, run the real browser with a cancelable context,
cancel after rendering, await `Run`, then close the pty under the race detector.
no tmux or phone is needed.

resolved when: every shutdown path cancels and joins both workers before closing
reader resources, qualified for quit, cancellation, input error and killed
shutdown on darwin and linux with no goroutine or descriptor leak. the owner
deferred the fix and prefers an upstream release; a fork or vendored patch needs
its own scope. the [sigterm hang](tui-sigterm-hang.md) may share the cause.
