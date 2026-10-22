# serve's launchd job clamps every agent to priority 20

problem: the `dev.niels.skidbladnir` plist declares no `ProcessType`, so
launchd runs serve as a standard job (`launchctl print`: `spawn type = daemon
(3)`). serve starts the tmux server, and the tmux server starts every pane
shell, agent and tool call. all of them inherit priority 20, while ordinary
login shells run at 31.

impact: when cpu is scarce, skid's terminal relay, tmux and every agent lose
scheduling to default-priority work, such as virtual machines, builds and apps.
the agents the operator types into feel slow first. linux is unaffected.

evidence: 2026-10-21, macbook:
- `ps -o ni,pri`: serve (pid 20187), the tmux server (pid 1328) and every
  claude/codex tui and agent shell report 0/20. ghostty login shells report 0/31.
- throwaway launchd agents with an absent `ProcessType`, with `Standard`, and
  with `Interactive` gave their children priority 20, 20 and 31.
- under contention, a priority-20 busy thread got 74% of one core over 3 s.

resolved when the plist change in
[the exec budget spec](../tmux-exec-budget.md#priority-macbook-capacity-d7) is
released and installed, the installer's priority check passes, and after the
maintenance window's tmux restart, serve, the new tmux server and a new pane
shell all report priority 31.
