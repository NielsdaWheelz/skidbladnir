# failed association reads can substitute the projected target

problem: `internal/sessions/manager.go:560` ignores errors reading the stored
conversation option and projects no conversation. `internal/agentcontrol/service.go:84`
then derives a claude conversation if foreground claude identity is available.
an unreadable codex association is therefore indistinguishable from its absence.

impact: a successful inventory may lose tracked state or substitute foreground
claude for recorded codex. fresh actions can capture that substituted target;
already-captured commands are not retargeted. the substitution requires subsequent
foreground observation to succeed, not a universally cancelled request.

evidence: source review only. `sessionOption` at `manager.go:825` returns command
errors, while `enrichSession` returns a session without an error result. explicit
associations otherwise take precedence over claude projection. no live failure
was reproduced.

resolved when: a read error cannot establish absence or permit another conversation
projection. prefer propagating required association-read failure through inventory
using existing failure handling. this temporarily costs whole-host inventory
availability; per-session association-unavailable state is an alternative only if
that demonstrated cost warrants a schema change. successful empty reads remain
distinct from failures; invalid metadata policy remains a separate contract.

temporary verification: simulate an association-read failure with a recorded codex
target and independently available claude identity; verify no successful substituted
projection, no native mutation, and recovery to the original association after a
successful read. also verify genuinely absent association still permits claude
projection. follow the testing policy; no retained harness or live operation is
authorized by this record. live/provider/device verification is `NOT_RUN`.
