# inventory list spends about 54 ms per session on serial tmux commands

problem: `sessions.Manager.List` runs about 14 tmux commands per session, one
after another: three scan reads done twice (`scanSessions`, then the unconditional
`scanSession` rescan in `normalizeNames`), the card anchor, and seven
`enrichSession` reads, plus about six per list. each tmux exec costs ~3.75 ms
median on the darwin host.

impact: an inventory request at 16 sessions takes about 0.9 s on darwin and
0.58–0.91 s on linux before enrichment, which itself stays near 30 ms. by
extrapolation, not measurement (about 54 ms per session, one client, no lock
wait), list alone reaches two seconds near 37 sessions. list holds the manager's
mutation lock throughout, so concurrent clients wait behind it and reach that
sooner. it is not a regression: the baseline spends the same.

evidence: darwin, 16 sessions, one client: list median 871 ms against baseline 918
ms, 230 tmux subprocesses per list on both trees; linux list median 0.58 s
(devbox) and 0.76–0.91 s (arch). production `Sessions.Listed` durations include
lock wait behind other polling clients and are not a sizing source.

resolved when: list reads each session's fields and options with one formatted
tmux read and drops the unconditional rescan (about 13 fewer execs per session),
and a 16-session darwin and linux measurement shows the per-session cost fall
without changing inventory content; then delete this record.
