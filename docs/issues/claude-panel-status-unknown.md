# claude task-panel rows block idle

problem: a claude composer can be visibly ready while a non-main agent or
workflow row sits in the task panel below it. `claudeReadFooter` marks every
non-main panel row `extra` (`internal/agentcontrol/claude_composer.go:454`) and
`claudeComposer` refuses idle on it (`:92`), even when the row reads `idle`,
`waiting`, `awaiting approval` or `N idle agents`. without other work evidence,
activity stays unknown, reason `partial`. row 11 of the
[claude grammar](../terminal-observation-claude.md#1-frozen-capability-table)
specifies this; it is an accepted limit.

impact: the session shows status unknown after a good capture. idle wait, ready
attention and guarded send (working or idle only,
`internal/agentcontrol/terminal.go:133`) all refuse it. dropping the veto without
stronger evidence is unsound: a ready composer says nothing about background work.

evidence: 2026-10-20, read-only on the macbook (v0.19.5): a claude 2.1.300 pane
(119x60) read unknown/partial with every other idle prerequisite met; removing only
the panel tail from the captured observation made it idle. a 2.1.302 session
without the row read idle. the row's real background state is unverified.

reproduce: with a ready claude composer above a non-main panel row whose state the
grammar cannot establish, run `skid info <handle> --explain`; expect unknown,
reason partial. live inspection needs current-turn tmux approval.

direction: an unmerged plan at
[`dc43e3a`](https://github.com/NielsdaWheelz/skidbladnir/blob/dc43e3af23fedba4017dd1ad20d2287b159f06a7/docs/claude-panel-status.md)
reads complete stock idle/waiting fields and `N idle agents` summaries as panel
quiescence (working > unknown > quiescent); hidden rows, nonzero queues, pauses and
unreadable status stay unknown. it also lets guarded send proceed under unknown
activity when an empty, receptive composer is proven, while initial prompt delivery
still requires idle. its explain detail and row-region diagnostic do not fit the
current `info --explain`, which prints reason and rule ids only.

resolved when: the panel evidence for running, waiting and idle is qualified
against real claude and ordinary inventory, ambiguous background work stays
unknown, and any fixture holds authored dummy content.
