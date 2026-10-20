# workspace cwd framing reads its length and value separately

problem: `internal/tmux/recovery.go:625` captures cwd with
`#{n:pane_current_path}:#{pane_current_path}`. upstream tmux invokes the builtin
callback for each occurrence; the cwd callback reads the operating system.
the prefix and payload can therefore describe different values when the shell
changes directory between those reads. `n:` counts bytes; utf-8 character width
is not the concern.

impact: unequal byte lengths could invalidate the workspace transcript and
delay checkpoint saving despite an otherwise valid live workspace. this is a
source-level race finding, not a reproduced failure or an established cause of
the observed inventory latency.

evidence: 2026-10-20 read-only source review of the current capture expression
and upstream tmux 3.4 [cwd callback](https://github.com/tmux/tmux/blob/3.4/format.c#L843-L856),
[callback dispatch](https://github.com/tmux/tmux/blob/3.4/format.c#L3589-L3596),
and [length calculation](https://github.com/tmux/tmux/blob/3.4/format.c#L4756-L4761).
no tmux command or live reproduction ran in this review. other raw fields owned
by tmux's synchronous command execution do not establish this same os-read race.

resolved when: each dynamic field is evaluated once and framed reversibly,
without delimiter loss for arbitrary utf-8, newlines, pipes or backslashes;
temporary isolated darwin/linux capture checks cover rapidly changing cwd with
different path lengths and prove valid capture or the existing explicit source
change outcome. preserve local metadata scope, server-lifetime checks and
checkpoint semantics; then delete this record.
