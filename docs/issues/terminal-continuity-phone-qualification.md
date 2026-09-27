# phone terminal continuity qualification

problem: the matching signed apk ran on the physical phone, but the final
source-only control and test-session cleanup were interrupted before proof.

impact: the close and new-terminal actions have not yet been accepted on the
phone while the current execution context is remote.

evidence: the installed apk bytes matched the published `v0.10.2` asset.
the physical phone created a terminal from `z` results, changed cwd, detached
and reopened the same source session, entered mosh to devbox, displayed the
remote cwd and configured codex home, returned to the remote shell on `/exit`,
and replaced the provider. the phone dashboard showed the destination and
source labels. source-only controls remain `NOT_RUN`; two test-created source
sessions await cleanup.

resolved when: with explicit current-turn approval, prove the source-owned
new-terminal and close controls while remote, then close only test-created
phone sessions and verify preexisting sessions remain. remove this record then.
