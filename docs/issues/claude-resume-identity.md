# claude registration can survive a failed conversation switch update

problem: the identity registration proves a process lifetime, not the current
conversation within that lifetime. if claude switches conversations through
`/resume` or `/clear` and the next identity hook fails, the old pane registration
still passes projection. failure does not necessarily omit metadata as the
[identity contract](../agent-identity-projection.md) claims.

impact: optional provider-session metadata and the derived native conversation
can retain the previous conversation until a successful registration or process
replacement. ordinary terminal status and interruption are independent and keep
following the pane. already captured native targets must continue addressing
their original conversation; the problem is representing stale registration as
current foreground identity.

evidence, 2026-09-30:

- [upstream hooks](https://code.claude.com/docs/en/hooks#sessionstart) document
  session-start events for startup, resume, clear, compact and fork. the installed
  plugin has no source matcher, so successful events replace the registration.
- `cmd/skidbladnir/main.go` returns on input/configuration failures without
  invalidating previous registration. `internal/agenthook/agenthook.go` writes
  the pane option only after successful process/profile admission.
- `internal/agentruntime/runtime.go:acceptRegistration` checks provider, pid,
  kernel start and configured profile, without current conversation evidence.
- a temporary probe of the actual projection accepts the previous registration
  when the process observation is unchanged, accepts the new id after successful
  replacement, and omits registered identity after a changed kernel start.

the probe was removed. source proves the failure condition; a real failed-hook
conversation switch has not been reproduced. live qualification is `NOT_RUN`.

reproduction: with current-turn approval, use disposable conversations and an
isolated tmux socket. establish registration for a, switch the same process to
b while forcing only the identity publication to fail, and inspect projected
identity without logging ids or content. restore the hook and check recovery.

resolved when: the contract and implementation distinguish last registered
identity from proven current conversation, or a bounded authoritative check
prevents a failed switch update from being represented as current. prove the
failure/recovery sequence and preservation of previously captured native targets.
adding timestamps alone cannot detect a missed event. new lifecycle hooks or
ordinary native polling require an explicit scope/acceptance amendment.
