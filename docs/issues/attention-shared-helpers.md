# attention shared helpers

problem: `internal/attention` is the hub's notification contract, but it still
hosts helpers that have nothing to do with notifications. `SingleLine` is the
tui's control-character sanitizer, `StateDirectory` is the desktop client's
default state directory, and `ErrUnavailable` is a generic error for mac socket setup.

impact: the package boundary misstates ownership. a reader looking for the tui's
text sanitizer or the client's xdg resolution finds it in the notification
contract, and every one of those callers imports notification types it never
uses.

evidence: `SingleLine` has callers in `internal/sessionui` (`view.go`,
`layout.go`, `create.go`, `usage.go`); `StateDirectory` in
`cmd/skidbladnir/fleet_cli.go` and `internal/agentcli/run.go`, duplicating
`install.Paths().State`; `ErrUnavailable` in
`internal/macnotifications/{client.go,server_darwin.go}`, never checked with
`errors.Is`. the hub-attention cleanup moved the device store out and left these,
because their callers lay outside its paths.

resolved when: `internal/attention` holds only the notification wire contract
and its copy, the sanitizer lives with the tui's text helpers, and the client state directory comes from the install
paths.
