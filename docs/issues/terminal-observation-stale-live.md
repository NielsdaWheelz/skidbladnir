# the desktop's stale and checking cells have no live run

problem: spec §8's full-product row requires stale and unavailable status through
the real gateway → desktop. the only live run, at `4e1b737`, failed: with the
gateway stopped, every row read faint `unavailable` instead of muted
`last observed: <label>`, and the never-ready check passed only vacuously,
because the table returned before the projection's freshness gate ran.
`5c59996` renders the projection for a failed host and `131c1df` the faint
`checking` cell for a host being re-read; both are proven red → green through the
real desktop model, not live.

impact: the stale cell, its never-ready gate and the checking cell are unproven
where inventory failure, scoped reads and the table meet a real gateway. a
composition defect there would show a wrong status, or a stale ready, unseen.

evidence: the full-product row of the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification).

resolved when: with current-turn approval, a live desktop against a real gateway
on an isolated tmux socket shows a failed host's rows as muted
`last observed: <label>` and never ready (a pending-ready row included), a host
being re-read as faint `checking` with `checking; last observed: <label>` facts,
both outside the needs-input filter, and the pending ready back after recovery,
recorded in the qualification; then delete this record.
