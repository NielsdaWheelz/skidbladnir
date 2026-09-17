# unused native failure result

problem: the native helper caller fabricates and returns structured failure
details that none of its four callers inspect. they only branch on success.

evidence: `Service.native` in `internal/agentcontrol/native.go`; callers in
`service.go` and `actions.go`. the helper's wire error remains part of strict
envelope admission, but its details have no internal consumer.

resolved when: expose a boolean success result while preserving subprocess
cancellation, output bounds, strict envelope admission and existing read/stop
behavior. characterize the actual subprocess boundary, including malformed,
failed, oversized and cancelled results. do not weaken the external protocol or
change unknown-outcome handling.
