# inventory and observation spawn a tmux process per step

problem: one session list spawns 6+2K tmux clients: inspection, epoch
admission, two inventory batches, two identity re-checks, and two per agent
pane (screen read, then a per-row capture). the hub's closed-client attention
lane lists every machine every 5 s, even with no client in the foreground, so
the cost is continuous. this lane is by design (`internal/notifier/observer.go:267`).
the per-step spawning is not.

impact:
- serve, its tmux children and the tmux server use about a third of one core
  around the clock on the macbook.
- the cost grows with each foreground phone or browser and with the number of
  agent panes.
- the metadata existence guard is O(N²) in sessions.

evidence: 2026-10-21, macbook, v0.20.0 (go sources identical at origin/main
`35de077`), 18-20 agent sessions, at most one attached terminal:
- `proc_pid_rusage` 30 s deltas: serve 13.6% of one core, its reaped children
  17.5%, the tmux server 5.0%.
- serve woke 770 times per second.
- 4 hz child sampling caught at least 7.1 tmux execs/s; the accounting in
  [the spec](../tmux-exec-budget.md#current-exec-accounting-originmain)
  predicts ~9.1/s.

resolved when [the exec budget spec](../tmux-exec-budget.md) option a is
implemented and its targets are measured on the macbook at N ≥ 18: at most
1.5 tmux execs/s, and under 5% of one core for serve, its children and the tmux
server, with inventory and status results unchanged.

progress: W2.7 landed a1, released in `v0.21.0`: a steady list is 2+2K execs; a2, a3 and the
macbook measurement remain.
