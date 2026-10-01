# the identity hook's tmux commands have no wait bound

problem: `internal/agenthook/agenthook.go` runs two tmux commands
(`display-message` for the pane tty, `set-option` for the registration) through
`exec.CommandContext` without a `WaitDelay`. both have pipe stdout (`io.Discard`
is still a pipe), and a tmux client hands its stdio to the server while
identifying, so a stalled tmux server keeps the pipe open past the context's end.

impact: claude's SessionStart hook can hang until the tmux server resumes,
delaying the hook (and claude's startup, if claude waits on it) past its deadline.
`internal/tmux` bounds its own commands to 100 ms past their context; the hook
does not. this predates the observation cutover.

evidence: the same mechanism held inspect and send responses until a stopped tmux
server resumed (12.27 s under a 12 s freeze with a 2 s deadline) until
`internal/tmux` set `WaitDelay`; the hook's execs were not changed or probed.

resolved when: the hook's tmux commands return within a small bound of their
context ending (a `WaitDelay`, or no output pipe where none is read), shown by a
stalled-server probe on an isolated socket; then delete this record.
