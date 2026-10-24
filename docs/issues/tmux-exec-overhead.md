# inventory and observation spawn a tmux process per step

problem: the hub's attention lane lists every machine every 5 s whether or not a
client is in the foreground (`internal/notifier/observer.go:265-268`), so
inventory's tmux exec cost is continuous, and it grows with each foreground phone
or browser and with agent panes.

evidence: 2026-10-21, macbook, v0.20.0, 18-20 agent sessions: serve used 13.6% of
one core, its reaped children 17.5% and the tmux server 5.0% (`proc_pid_rusage`,
30 s); serve woke 770 times per second; at least 7.1 tmux execs/s. a steady list
then cost 6+2K execs for K agent panes.

live evidence: 2026-10-23, macbook, the running gateway's executable still came
from v0.20.0 (`acf4e17`). a 30 s `proc_pid_rusage(RUSAGE_INFO_V4)` sample measured
serve at 36.21% of one core, its reaped children at 70.83%, and the shared tmux
server at 9.66% (116.70% combined); serve had 1,614.5 interrupt wakeups/s.
sampling its children every 10 ms found 471 distinct pids (at least 15.69 child
spawns/s); a separate 10 s sample identified 148 tmux children. a concurrent
5 s stack sample showed repeated forks. session and client counts were not
controlled, so these diagnose the running workload rather than qualify the fix.

progress: a1 shipped in v0.21.0 (a steady list is 2+2K execs). a2 and a3 (#156,
`fda4d11`, unreleased) make a steady list 2 execs at any K: an isolated
counting-shim run on 2026-10-22 measured K = 0, 1, 18, 19 at 2, 4, 38, 40 execs
before and 2 after, capturing only the animated pane's rows.

resolved when: after the release carrying #156, measured on the macbook at
N >= 18: at most 1.5 tmux execs/s and under 5% of one core for serve, its children
and the tmux server, with inventory and status results unchanged
([exec budget](../tmux-exec-budget.md)).
