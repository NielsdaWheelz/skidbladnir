# duplicate terminal cleanup owner

problem: `terminal.Cleanup` wraps two attachment methods in callbacks and another
`sync.Once`, although its sole owner calls it exactly once on function exit.

evidence: `gateway.runTerminal` is the sole `NewCleanup` caller and invokes
`Close` only in its defer. `tmux.Attachment` already owns idempotent PTY and
client closure.

resolved when: inline ordered PTY/client closure and error joining in that defer,
delete the wrapper, and conserve websocket detach and peer-loss cleanup against
an isolated real tmux socket. preserve worker shutdown order and other sessions.
