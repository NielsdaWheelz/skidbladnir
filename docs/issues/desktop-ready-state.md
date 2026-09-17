# redundant desktop initialization flag

problem: `sessionui.model.initialized` repeats information already carried by
`scopeReady` and has no separate responsibility.

evidence: both begin false. the only `initialized = true` immediately follows
the sole `scopeReady = true`; every other scope readiness assignment sets false.
its sole read is `!initialized || !scopeReady`, equivalent to `!scopeReady` for
every reachable model. there are no independent construction or mutation paths.

resolved when: remove the field/assignment and simplify that condition; verify
the complete caller/state inventory, unchanged refresh/action admission and
builds. this redundant-state deletion does not require a new behavioral harness.
