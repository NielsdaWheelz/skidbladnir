# macos serve's stderr is discarded

problem: on the macbook, launchd discards `skidbladnir serve`'s stderr. that is
where serve's exit error (`skid service: ...`) and any go panic go. the event
log no longer shares the gap: since w2.5 serve writes it to
`${XDG_STATE_HOME:-~/.local/state}/skidbladnir/serve.log` on darwin. linux keeps
stderr in the user journal.

impact: a mac serve that exits at every start (bad yaml, unusable state, tmux
missing) or panics leaves no reason behind; launchd restarts it silently.

evidence: the `dev.niels.skidbladnir` plist written by `services` in
`internal/install/services.go` sets no `StandardErrorPath`.

resolved when the owner picks a bounded mac sink for serve's stderr and it is
verified on the installed host, or explicitly accepts the gap.
