# claude task-panel rows block idle

problem: a claude composer can be visibly ready while a non-main agent or
workflow row sits in the task panel below it. `claudeReadFooter` marks every
non-main panel row `extra` (`internal/agentcontrol/claude_composer.go:454`) and
`claudeComposer` refuses idle on `!footer.extra` (`:92`), even when the row reads
`idle`, `waiting`, `awaiting approval` or `N idle agents`. without independent
work evidence, activity stays unknown with reason `partial`. row 11 of the
[claude grammar](../terminal-observation-claude.md#1-frozen-capability-table)
specifies this, so it is an accepted limit, not a regression.

impact: the session shows status unknown after a successful capture. idle wait,
ready attention and ordinary guarded send, which admits only working or idle
activity (`internal/agentcontrol/terminal.go:133`), all refuse it. dropping the
veto without stronger evidence is unsound: a ready composer says nothing about
background work.

evidence, 2026-10-20, read-only on the installed macbook (v0.19.5, `fef6347`):
an affected claude 2.1.300 pane (119x60, unclipped) read unknown/partial while
every other idle prerequisite passed; removing only the panel tail from the
captured observation in memory made it idle. a 2.1.302 session without an extra
row read idle. the veto dates from `78b3aa4` (2026-10-01) and is unchanged on
main. the row's real background state and why it appeared are unverified.

reproduce: with a ready claude composer above a non-main panel row whose state
the grammar cannot establish, run `skid info <handle> --explain`; expect unknown,
reason partial. live inspection needs current-turn tmux approval.

direction: a plan agreed in scope on 2026-10-21 but never merged is at
[`dc43e3a`](https://github.com/NielsdaWheelz/skidbladnir/blob/dc43e3af23fedba4017dd1ad20d2287b159f06a7/docs/claude-panel-status.md)
(branch `docs/claude-panel-status`). it reads complete, qualified stock
idle/waiting fields and `N idle agents` summaries as panel quiescence, reduced
as working > unknown > quiescent; hidden rows, nonzero queues, pauses and
unreadable status stay unknown. it also lets ordinary guarded send proceed under
unknown activity when an empty, receptive composer is independently proven,
while initial prompt delivery still requires idle. its explain detail line and
row-region diagnostic predate #133, which cut `info --explain` to reason and
rule ids.

resolved when: the accepted panel evidence for running, waiting and idle is
qualified against real claude and ordinary inventory, genuinely ambiguous
background work stays unknown, and any fixture holds authored dummy content.
