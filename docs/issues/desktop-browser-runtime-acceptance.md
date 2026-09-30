# desktop browser runtime acceptance

pr 3's [a4](../desktop-browser.md#7-acceptance-and-delivery) requires the real
browser/pty/gateway/isolated-tmux journey on linux and darwin. unit tests and
cross-compilation cannot establish tty ownership or attachment return.

2026-09-17: the approved linux journey failed against the preceding browser at
immediate space selection, then passed on the composed candidate (2.660s).
ssh authentication initially failed, and the user instructed skipping darwin
in that turn. access was later restored; a later turn approved isolated tmux
work, but the `v0.10.3` delivery still did not record a darwin browser journey.
that boundary remains `NOT_RUN`.

with current-turn approval, use a temporary real browser → pty → gateway →
isolated tmux journey under
[testing policy](../rules/testing.md). prove exact creation/attachment, retained
filters and selection, the first navigation key after detach, escape after
`ctrl-] d` staying in skid, `ctrl-c` quitting, source survival, and no creation
replay after a lost reply. record content-free results in the change; delete
this issue when the boundary passes.

the terminal-continuity extension adds final-release `n` quick creation,
`N` options, and `T` new-terminal-here to this live boundary, including target
and default selection, offline failure, duplicate suppression, and uncertain
creation without replay. [pr 4](https://github.com/NielsdaWheelz/skidbladnir/pull/4)
passed temporary component checks but explicitly left the rebased browser's
live fleet run unperformed. the `v0.10.3` delivery did not record this browser
journey. these cases remain `NOT_RUN` and belong in the same approved proof.

2026-09-30: browser `x` now interrupts then closes; `c` is removed and `s` retains
the terminal. temporary browser/client checks against a loopback tls fixture
pass confirmation/cancellation, exact-target dispatch, busy duplicate suppression,
partial-close feedback and removed-key behavior. provider/tmux closure for this
change remains `NOT_RUN`; include `s`/`x` in the approved live journey.
