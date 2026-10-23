# attention shared helpers

problem: `internal/attention` is the hub's notification contract, but it also
hosts the display sanitizer and the client state directory. `SingleLine`
(`internal/attention/attention.go:75`) cleans every single-line display: the
tui's rows, forms and tabs (`internal/sessionui/{view,layout,create,keys,usage}.go`)
and the hub's notice titles (`internal/notifier/transitions.go:163`).
`StateDirectory` (`attention.go:84`) duplicates `install.Paths().State`
(`internal/install/paths.go:23`) for `cmd/skidbladnir/fleet_cli.go:26`,
`internal/agentcli/run.go:425` and
`internal/macnotifications/{client.go:18,app_darwin.go:250}`.

impact: the package boundary misstates ownership: the tui and the fleet cli import
the notification contract for a sanitizer and a path.

resolved when: `internal/attention` holds only the notification wire contract and
its copy, `SingleLine` lives with the text it serves, and every client state
directory comes from `install.Paths`. `ErrUnavailable` stays: it is the
notification error.
