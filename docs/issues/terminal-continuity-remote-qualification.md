# remote terminal continuity qualification

problem: the matching `v0.10.3` generation and scoped forwarding are installed
on macbook, arch, and devbox. ordinary macbook-to-devbox ssh/mosh works, but
the broader remote lifecycle has not been qualified.

impact: nested hops, suspension, reconnect, gateway restart, and source loss
may expose stale or unavailable remote context. those paths remain unclaimed.

evidence: published-release and fleet checks passed. an isolated mac gateway
proved ssh/mosh markers, remote registration and cwd changes, configured stock
codex identity, `/exit` retaining the remote shell, and source shell rejection
while connected. the physical phone reached devbox over mosh, changed remote
cwd, observed the configured codex profile, retained the shell after `/exit`,
replaced the provider, detached/reopened, and closed its exact source terminal.
macbook-to-devbox `SendEnv` and destination `AcceptEnv` remain scoped; sshd
configuration validated before reload. no devbox-to-macbook ssh route was
added. distinct simultaneous connections, nested hops, suspension/resume,
reconnect, gateway restart, and source loss are `NOT_RUN`. the accepted
[remote adversarial cases](../terminal-continuity.md#6-delivery-and-adversarial-acceptance)
also lack final-release proof for stale or reused registrations, wrong tty,
late replacement results, ambiguous matches, unrelated offline peers,
oversized environment, source-empty/destination-populated catalogues, inner
tmux, missing forwarding or integration, hop exit, and remote ctrl-z/fg. these
remain `NOT_RUN`.

resolved when: prove those six unqualified lifecycle paths on intended routes
and the listed adversarial cases with content-free evidence. preserve the
source-owned action boundary and remove this record then.
