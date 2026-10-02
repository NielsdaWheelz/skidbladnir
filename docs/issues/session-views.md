# session views implementation

problem: the accepted [session views specification](../session-views.md) is not
implemented. phone navigation still combines machine/group filters and a
request-only toggle; the desktop first view is the broader agents roster.

impact: neither client provides the agreed ready/request/error/interruption
queue. hiding phone machine controls alone would retain hidden scope and remove
access to pressure diagnostics.

evidence: 2026-10-02 survey of `internal/sessionui/{session,navigation,view}.go`,
`internal/fleetclient/status.go` and android `DashboardScreen.kt`,
`DashboardEntryState.kt`, `Groups.kt`, `ProductModel.kt`,
`SkidbladnirController.kt` and `PressurePresentation.kt`. desktop row rebuilding
currently precedes attention observation and attachment acknowledgement does not
rebuild, which must change before readiness determines membership/order.
phone inventory publishes before asynchronous attention completes; restoring a
ready-row anchor must wait for the matched attention outcome or modeled failure.
the spec and plan are documentation only; behavioral acceptance is `NOT_RUN`.

resolved when: both clients implement the spec, predecessor controls/state are
removed, and its projection, attention/return, scope/recovery, restoration and
interaction acceptance are recorded for the changed source. retain explicit
limits for unperformed device/live cases. delete this record when resolved.
implementation has not been requested; tmux/device work requires explicit
current-turn approval. the independent [exit-attention gap](agent-exit-attention.md)
is not a completion gate for this change.
