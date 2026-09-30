# history error decoding loses dispatch uncertainty

problem: `internal/agentcontrol/native.go:154` decodes `history_changed` into a
sentinel without its dispatch field, although the shared decoder accepts both
`unknown` and `not_sent`. `internal/gateway/agent_control.go:24` and `:212` map
that sentinel to a public error with `dispatch: not_sent`.

impact: an unexpected helper error on a mutating operation can falsely claim
no dispatch. this is a protocol validation gap, not evidence that the current
helper emits that combination or that a mutation was replayed.

evidence: static review of the operation-independent decoder and gateway mapping.
the native caller already chooses conservative `unknown` for mutation failures.

resolved when: accept history-change restart semantics only for their valid
read-only result-enumeration context, and reject inconsistent envelopes without
weakening mutation uncertainty. keep this validation at the helper protocol
boundary rather than adding client-specific repairs.

temporary verification: exercise valid results/history-change handling plus
unexpected history-change errors with both dispatch values during mutation;
verify no mutation response incorrectly asserts no dispatch. remove temporary
tests under the testing policy. provider/live verification is `NOT_RUN`.
