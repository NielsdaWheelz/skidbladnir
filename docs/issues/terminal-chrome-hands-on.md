# terminal chrome lacks physical-phone acceptance

problem: the one-row [terminal chrome](../terminal-chrome.md) was observed only
on an api-36 emulator shaped like the s22+. the physical phone, talkback and
switch access have not exercised it.

impact: rendered geometry, gboard interaction, reach and spoken output are
unproven on the owner's device. a frozen connection is coloured and spoken in
the rail's description but not announced: no live region reports the change to
`input frozen`.

evidence: the emulator measured 520 → 608 dp (keyboard hidden) and
241 → 330 dp (gboard up) at 384 × 832 dp; the s22+ was not touched.

resolved when: on the s22+, the rail stays one row through connect, frozen,
too-small and both sheets; talkback reads `Detach`, then
`<name> on <place>, <presence>` with `open session actions`, then the terminal;
switch access reaches every sheet row; a frozen connection is announced once.
device operations require current-turn approval.
