# persistent claude usage conflict

problem: the owner reports `conflict` after reducing the apparent claude session
count to one. the installed cause is unverified.

impact: usage remains unavailable despite the owner's expectation of recovery.

evidence: the baseline [desktop selector](../../internal/sessionui/usage.go) at
`7b8aa2be16557a15e3b0e1b22251f372a5977080` derived conflict from differing eligible
session-row observations within one host/profile; there was no retained conflict
latch. one eligible observation in
an admitted successful inventory cannot produce it. idle/hidden sessions and
other hosts can contribute. pending admission or installed/source differences
are hypotheses, not findings. the [policy correction](../claude-usage-selection.md)
does not diagnose this symptom.

resolution: with approved read-only live access, identify installed source and
content-free contributor/admission facts; explain the observed state and verify
that a completed successful one-contributor refresh presents its usage. after
policy rollout, disappearance of the label alone does not prove normal refresh.

blocker: installed diagnosis is explicitly outside the approved policy scope.
current-turn tmux approval covered only newly created isolated acceptance resources;
no installed-fleet diagnosis ran. the replacement selector is
[isolated-qualified](../claude-usage-selection.md#qualification), not deployed.
