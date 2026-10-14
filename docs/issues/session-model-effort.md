# session model and effort display

2026-10-14: deferred by the owner; outside the accepted
[desktop row cleanup](../desktop-session-rows.md). not a release or acceptance gate.

problem: session inventory exposes no observed model/effort. launch overrides
and account defaults do not establish a running terminal's current selection.
recorded codex conversations may differ from the terminal; new terminals have no
binding. claude's existing quota report is per-account, not per-session.

impact: the browser cannot display a truthful current setting for each session.
reusing a launch value, another conversation or another session's quota report
would silently mislabel work.

evidence: `internal/agentruntime/launch.go`,
`internal/fleetclient/response.go`, [native identity](../native-agent-observation.md),
and [quota scope](../profile-usage.md). the existing bounded terminal sample in
`internal/agentcontrol/terminal.go` is a candidate for optional rendered-label
extraction, but neither provider grammar currently extracts these facts.

resolution requires separately accepted scope and qualified observation semantics.
recommended first surface: selected-session details, meaning only model/effort
shown in that exact sampled terminal. hidden, clipped, ambiguous, remote or
changed-target samples omit values; never carry earlier values forward. verify
interactive switches, concurrent sessions sharing an account, and custom/hidden
footers. effective per-inference model identity is a different capability.
