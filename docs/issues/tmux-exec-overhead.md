# recurring tmux and recovery work needs an energy measurement

problem: the hub's attention lane lists every machine every 5 s whether or not a
client is in the foreground (`internal/notifier/observer.go:265-268`), so
inventory's tmux exec cost is continuous, and it grows with each foreground phone
or browser. batching removes the ordinary per-pane spawn cost, but polling,
screen observation and recovery still consume resources.

impact: standing cpu and wakeup costs even without a visible client. macos also
attributes provider work to skid ([launch attribution](macos-process-attribution.md)),
so skid's grouped energy display cannot measure gateway overhead alone.

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
`fda4d11`, present in the observed v0.23.0) make a steady list 2 execs at any K: an isolated
counting-shim run on 2026-10-22 measured K = 0, 1, 18, 19 at 2, 4, 38, 40 execs
before and 2 after, capturing only the animated pane's rows.

current evidence: 2026-10-29, macos 27.0.1, loaded gateway v0.23.0 (`677bbb2`),
tmux 3.8. a 30 s `proc_pid_rusage(RUSAGE_INFO_V4)` delta, converted with
`mach_timebase_info`, measured serve at 1.007% of one core, its reaped children
at 1.012%, and tmux at 1.169% (3.188% combined). serve had 145.59 interrupt
wakeups/s. two active codex processes separately used 9.359% combined; the
notification app used 0.038%. a separate 20 s sample of serve's children using
`proc_listpids(PROC_PPID_ONLY)` every 10 ms observed 20 tmux and 3 skidbladnir
children: at least 1.0 tmux exec/s. this did not reproduce the earlier runaway
cost. session count, client activity and provider workload were uncontrolled;
neither these cpu figures nor the spawn lower bound qualify the acceptance target
or explain an earlier activity-monitor peak.

remaining mechanisms: the hub inventories every 5 s; clients add inventory
requests; each attached web terminal polls presence every 2 s. recovery runs
every 30 s and after mutations. its steady path now has five tmux execs (two
server inspections, inventory, and two workspace captures); the budget now records
both captures because the final one validates changes during provider discovery.
recovery also reads provider metadata within a shared 2 s discovery deadline;
codex scans its index and matching session files, and claude can fall back from
its pid registry to scanning project metadata. their individual cost is unmeasured.

follow-up: measure the specified baseline and incremental client/checkpoint costs
separately from provider work. reduce only measured unnecessary work while preserving observation
and recovery correctness. moving resource attribution is not an energy saving.

resolved when: measured on the macbook at N >= 18, with the hub lane active and
no foreground phone or browser: at most 1.5 tmux execs/s and under 5% of one core
for serve, its children and the tmux server, with inventory and status results unchanged
([exec budget](../tmux-exec-budget.md)).
