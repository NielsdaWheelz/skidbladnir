# macos serve's event log is discarded

problem: `skidbladnir serve` writes its content-free event log to stdout
(`cmd/skidbladnir/main.go:57`). the standalone launchd plist
(`services` in `internal/install/services.go`) sets no `StandardOutPath`, so launchd
discards those events on the macbook. linux keeps them in the user journal.

the dev-server-era gateway wrote the same events to
`~/.local/state/skidbladnir/gateway.log`. that file was orphaned at the
standalone cutover and is still there.

impact:
- the mac has no record of failed observations, sessions-observed phases or
  recovery changes when diagnosing a fault.
- the orphan occupies 249 mb.
- an unbounded sink would grow quickly: the hub lane alone logs a request
  every 5 s.

evidence: 2026-10-21, macbook:
- `gateway.log` is 249 mb, with 1,979,905 json events, last written
  2026-10-20 20:45, one minute before the standalone `install.lock` and
  enrollment file (20:46).
- its final events are `Request.Completed` for `/v1/sessions`.
- the installed plist has no output keys.

resolved when the owner chooses a mac sink with a size bound, or explicitly
accepts discarding (W2.5 logging-slog reshapes logging first). the chosen
behavior must be verified on the installed host, and the operator removes the
orphan file.
