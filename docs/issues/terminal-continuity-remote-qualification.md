# remote terminal continuity qualification

problem: the matching `v0.10.2` generation and scoped forwarding are installed
on macbook, arch, and devbox. ordinary macbook-to-devbox ssh/mosh works, but
the broader remote lifecycle has not been qualified.

impact: nested hops, suspension, reconnect, gateway restart, and source loss
may expose stale or unavailable remote context. those paths remain unclaimed.

evidence: `v0.10.2` source and published-release checks passed; fleet verify
passed all three installed hosts. an isolated mac gateway on a test-owned tmux
socket proved ssh/mosh source markers, remote registration, cwd change,
configured stock codex home, `/exit` retaining the remote shell, and source
shell rejection while connected. the physical phone also reached the devbox
over mosh and displayed its live remote cwd and provider. no devbox-to-macbook
ssh route was added. distinct simultaneous connections, nested hops,
suspension/resume, reconnect, gateway restart, and source loss are `NOT_RUN`.
`docs/terminal-continuity.md` names the full acceptance set.

resolved when: prove distinct simultaneous connections, nested hops,
suspension/resume, reconnect, gateway restart, source loss, and source-only
controls on the intended routes with content-free evidence. remove this record
then.
