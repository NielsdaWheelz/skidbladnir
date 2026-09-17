# obsolete gateway construction paths

problem: the gateway duplicates twelve session methods in a private interface
with one concrete supplier, and supports omitting agent control even though its
sole production constructor always supplies the real service.

evidence: `cmd/skidbladnir/main.go` constructs both owners before `gateway.New`.
the nil-service branch in `gateway/agent_control.go` is the only producer of
`AgentUnavailable`; its logging and android decoding cases therefore describe
an unreachable production response. the desktop's lowercase `agent_unavailable`
has a different, reachable producer and must remain.

resolved when: use the concrete session owner and remove both nil-service paths,
the dead gateway error, its logging code and android wire consumers together.
characterize strict error decoding before and after; preserve all current
target, blocked and invalid-input outcomes. no compatibility reader or new
constructor guard. verify actual composition and all callers. the deliberate
cost is losing the duplicated narrower compile-time session method set.
