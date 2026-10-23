# one agent creation answered 500 InternalError

problem: once, an agent creation (`POST /v1/sessions`) on unmodified main
answered 500 `InternalError` with dispatch `not_sent` from `sessions.Create`. the
cause is unknown, and the gateway keeps none of it. `completeCreation` maps
every error that is neither `ErrCreateDispatchUnknown` nor a mapped
`sessions.Error` to `errorInternal` (`internal/gateway/sessions.go:165-176`,
`internal/gateway/gateway.go:266-269`), and `Request.Completed` logs only the
error code (`gateway.go:105-118`).

impact: a requested agent is not created and the client shows a generic
failure. `not_sent` says the manager judged nothing dispatched, so a retry
should be safe, if that judgement holds. one failure in 61 creations in one
harness; no live report.

evidence: go-wire-package (#157), 2026-10-22: an isolated `tmux -L
skidslop-go-wire-package-N` server behind a loopback gateway with scratch
HOME/XDG, on base `f5115e3`. it failed after 20 back-to-back creations; 41 more
did not reproduce it.

repro: unknown. the harness created agents back to back on one isolated server.

resolved when: the unmapped error at that 500 is logged, the cause is found and
fixed or mapped to a typed failure, and 200 back-to-back agent creations on an
isolated server all answer 201.
