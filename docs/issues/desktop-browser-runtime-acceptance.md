# desktop browser runtime acceptance

pr 3's [a4](../desktop-browser.md#7-acceptance-and-delivery) requires the real
browser/pty/gateway/isolated-tmux journey on linux and darwin. unit tests and
cross-compilation cannot establish tty ownership or attachment return.

2026-09-17: the approved linux journey failed against the preceding browser at
immediate space selection, then passed on the composed candidate (2.660s).
the macbook is reachable, but ssh rejects the workspace's available key. the
user explicitly instructed skipping darwin; its native proof remains `NOT_RUN`,
not a pass. no further darwin execution is required for this implementation turn.

when darwin verification resumes with approval and a usable access route, run
`TestShellDesktopRealTTYCreateAttachDetachAndLostReply` with the existing integration
capability and isolated-socket harness on that host. it must prove exact creation/attachment, retained filters
and selection, the first navigation key after detach, source survival, and no
creation replay after a lost reply. record content-free results in the roadmap;
delete this issue when the darwin proof passes.
