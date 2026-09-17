# desktop browser runtime acceptance

pr 3's [a4](../desktop-browser.md#7-acceptance-and-delivery) requires the real
browser/pty/gateway/isolated-tmux journey on linux and darwin. unit tests and
cross-compilation cannot establish tty ownership or attachment return.

2026-09-17: the approved linux journey failed against the preceding browser at
immediate space selection, then passed on the composed candidate (2.660s).
ssh authentication initially failed, and the user explicitly instructed skipping
darwin. access was later restored for publication; the browser's native proof
remains `NOT_RUN`, not a pass. the user's skip instruction still stands.

when approved darwin verification resumes with a usable access route, use a
temporary real browser → pty → gateway → isolated tmux journey under
[testing policy](../rules/testing.md). prove exact creation/attachment, retained
filters and selection, the first navigation key after detach, source survival,
and no creation replay after a lost reply. record content-free results in the
change; delete this issue when that boundary passes.
