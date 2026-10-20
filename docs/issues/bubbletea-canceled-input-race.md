# canceled browser shutdown races its terminal input reader

problem: bubbletea v2.0.9's killed shutdown closes cancelreader before its input
worker has joined. ultraviolet's pinned `StreamEvents` also returns without
joining its internal stream worker. this predates the latency implementation:
both versions of `sessionui.Run` pass their caller context to `tea.WithContext`.

impact: canceling a browser can race `os.File.Close` against `Fd` on darwin's
kqueue reader; the input worker can outlive `Program.Run`. the observed race is
real. no crash, lost input or production occurrence was established.

evidence: 2026-10-20 temporary actual-pty browser run under `go test -race`:
`TestLatencyBrowserPublishesHealthyPeer` published the healthy peer and its next
tick while the other peer/notification owner stalled, then reported the race
during cancellation. `tea.go:1265` closed cancelreader's signal pipe while
`terminal_reader.go:124` / `cancelreader_bsd.go:139–141` read its file descriptor;
closing the fixture input after `Run` returned exposed the same unjoined worker.
pins: bubbletea `v2.0.9`, ultraviolet `f5a850f9c2b7`, cancelreader `v0.2.2`;
go 1.26.5, darwin. full race run: FAILED; latency assertions had already passed.

upstream inspection: ultraviolet commit
[`0b88c25f3fff`](https://github.com/charmbracelet/ultraviolet/commit/0b88c25f3fff665a5f9dfd226ee71868f5e8d51a)
joins its internal worker. bubbletea v2.1.0/head `f9df43c4f2c0` still skips its
reader wait on killed shutdown; its graceful wait also has a 500 ms timeout.
cancelreader head `245609eb8557` retains the same bsd implementation. upgrading
only ultraviolet or routing ordinary cancellation through quit is incomplete.

reproduce: create a private pty, run the real browser with a cancelable context,
cancel after rendering, await `Run`, then close the owned pty under the race
detector. keep the input reader active; retain only stack/timing evidence.
no tmux or phone boundary is needed.

resolved when: repair the upstream reader ownership so every shutdown path
cancels and joins both workers before closing reader resources/returning; qualify
normal quit, caller cancellation, input error and killed shutdown on darwin/linux
with bounded cleanup and no goroutine/file leak. prefer an upstream release;
a maintained fork/vendor change requires its own explicit scope. the owner
explicitly deferred this fix from the latency cutover on 2026-10-20. no race
suppression, dependency patch or partial workaround was applied.
