# P0 tmux grouped-client behavior

Recorded: 2026-08-25T05:03:14Z

Result: **PASS**

## Artifacts

- Host platform: Linux
- tmux: `3.4`
- PTY fixture: `/usr/bin/script`

## Procedure

```text
go test -tags=live ./tests/live -run '^TestTmuxGroupedBehavior$' -count=1 -v
go test -race -tags=live ./tests/live -run '^TestTmuxGroupedBehavior$' -count=1 -v
go test -tags=live ./tests/live -run '^TestTmuxGroupedBehavior$' -count=20
```

Each run created one uniquely named alternate tmux server, two grouped
sessions over the same two panes, and two real PTY clients: an unflagged
120×40 laptop client and an 80×24 phone-shadow client carrying
`active-pane,ignore-size`. The proof retained kernel PID/start-time, TTY, and
file-descriptor identity for every owned process.

## Observed result

- Each client's keystrokes reached only its selected pane; the shadow client's
  selection did not steal the laptop client's active pane.
- The unflagged laptop controlled geometry while attached. The sole
  `ignore-size` phone client then sized the window, and the reattached laptop
  retook sizing. Phone flags remained exact through both transitions.
- Killing the non-last shadow session preserved both pane PID/start-time/TTY
  identities. The surviving laptop client then selected and wrote the second
  pane, proving that it remained live, before returning to the first pane.
- Killing the last grouped session destroyed the panes and fixture processes.
  A fresh client attachment had already reproduced the current screen.
- Focused, race, 20-repeat, and complete host-live runs passed. No alternate
  tmux process, PTY helper, fixture, or socket remained after cleanup.

This record proves the tmux 3.4 grouped-client, focus, sizing, repaint, and
link-lifetime mechanics with deterministic shell fixtures. It does not claim a
Skíðblaðnir gateway, a stock Codex draft-preservation journey, or lifecycle
reconciliation; those remain in their later ownership proofs.
