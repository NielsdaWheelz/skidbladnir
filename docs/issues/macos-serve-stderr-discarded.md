# macos serve's stderr is discarded

problem: on darwin, launchd discards `skidbladnir serve`'s stderr, where serve's
exit error (`skid service: ...`, `cmd/skidbladnir/main.go:59`) and any go panic
go: the `dev.niels.skidbladnir` plist sets no `StandardErrorPath`
(`internal/install/services.go:47-60`). the event log is unaffected: on darwin
serve writes it to `${XDG_STATE_HOME:-~/.local/state}/skidbladnir/serve.log`
(`cmd/skidbladnir/standalone.go:57`); linux keeps stderr in the user journal.

impact: a mac serve that exits at every start (bad yaml, unusable state, tmux
missing) or panics leaves no reason behind; launchd restarts it silently.

resolved when: the owner picks a bounded mac sink for serve's stderr and it is
verified on the installed host, or accepts the gap, which then becomes an accepted
limit in the architecture.
