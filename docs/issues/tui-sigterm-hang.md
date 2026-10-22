# the tui sometimes ignores sigterm

problem: the session browser sometimes does not exit within 5 s of SIGTERM.

impact: a terminated `skid` browser can linger until it is killed again. no lost
state is known; the browser owns no durable state.

evidence: 2026-10-22, the desktop-core characterization harness's `sigterm`
journey (fake loopback gateways, a pty) hung at "no exit within 5000 ms" in about 1
run in 7, on both 3ef8c2e and the desktop-core rewrite. the cause is unconfirmed;
afaict it sits in bubbletea's own signal handling during shutdown, near
[the canceled input race](bubbletea-canceled-input-race.md).

resolved when: the cause is found and 50 consecutive sigterm runs of the real
browser in a private pty exit within 1 s.
