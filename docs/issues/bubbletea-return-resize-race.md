# bubble tea's resize check can outlive a detached browser

bubble tea v2.0.9 starts `checkResize` in a goroutine when it restores the
terminal after a visit. cancellation can let `Program.Run` return while that
goroutine can still call `Fd` on the borrowed output. a caller closing the file
after `Run` can race that read.

the go race detector reproduced the race on macos in an isolated pty with a
loopback terminal socket. it reported `os.File.Close` against `Fd` from
`tty.go:115`, started by `RestoreTerminal` at `tea.go:1371`. the same race
reproduced at `9beec7a`, before the browser return changes. a production effect
has not been observed.

to reproduce, attach and detach through the real browser in a private pty,
cancel the browser immediately after sending detach, wait for `Run`, and close
the pty under the race detector. resolve when the terminal restore path joins
its resize work before returning ownership of the output. verify detach
followed by quit and by cancellation on macos and linux.

the [canceled input race](bubbletea-canceled-input-race.md) concerns a separate
worker in the same dependency.
