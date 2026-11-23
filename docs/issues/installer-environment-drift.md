# An update from a different PATH demands a terminal drain

Problem: the installer writes the caller's `PATH` verbatim into the terminal
service definition (`environment` in `internal/install/services.go`), and
`runtimeChanged` in `internal/install/terminal.go` compares that definition
byte for byte. An update run from a shell whose `PATH` differs from install time
therefore counts as a terminal runtime change, and `skid update` refuses until
tmux is drained.

Impact: a gateway-only update looks as if it needs a drain, and following the
refusal with `--restart-terminals` would stop running agents for nothing.

Evidence: on 2026-11-23 the v0.28.0 update refused on macbook (an interactive
shell with version-manager shims on `PATH`) and on dev-server (a non-interactive
SSH shell), and went through on arch, whose SSH `PATH` matched. Rerun with the
`PATH` recorded in each definition, the macbook and dev-server updates reported
`terminal service unchanged`.

Workaround: run `skid update` with the `PATH` recorded in the definition. On
macOS it is under `EnvironmentVariables` in
`~/Library/LaunchAgents/dev.niels.skidbladnir.terminals.plist`; on Linux it is
the `Environment="PATH=…"` line in
`~/.config/systemd/user/skidbladnir-terminals.service`.

Resolved when: the runtime-change decision no longer depends on the caller's
environment, and an update from a shell with a different `PATH` completes as
gateway-only.
