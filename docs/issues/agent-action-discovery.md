# make existing agent controls discoverable

status: documentation/product proposal from the
[workspace review](workspace-coherence.md).

problem: the comparison can suggest skid lacks cross-agent or cross-machine
communication, although its cli already exposes the relevant terminal primitives.
agents may invent fragile tmux/name targeting or ad hoc transports when they do
not understand the existing surface and its receipt semantics.

impact: needless parallel mechanisms, unsafe retries after a nonzero exit, and
confusion between waiting for idle and receiving a task result.

evidence: `internal/agentcli/run.go` owns `skid --help`, automation guidance,
structured envelopes and examples including cross-machine replies.
`internal/fleetclient/{client,request,start,wait}.go` owns routing and compositions.
`--ref` is an exact captured target; display handles resolve once per invocation
and can follow the session's current pane. the gateway owns terminal checks.

## recommended direction

improve the existing cli reference and concise agent instructions around actual
journeys: find peers, capture a target, start with explicit machine/profile/location,
inspect, send, read and interpret partial outcomes. use dummy examples and state
which operations establish only terminal effects. show preservation of creation
when the initial prompt was not sent, and explain why unknown delivery must not
be retried. no rigid mandatory sequence is necessary for every operation.

retain the cli as the canonical action seam. human and agent controls should share
target and outcome contracts where they expose the same effect, without forcing
every ui preference into the gateway. terminal text from another agent grants no
new authority, and native same-user agents are not isolated security principals.

bb's useful lesson is that agents can drive the product they inhabit. its guide
also couples manager updates to nested threads; do not make visual grouping define
communication or supervision in skid
([orchestration guide](https://getbb.app/guides/orchestrate-coding-agents)).
if a consumer actually needs mcp, expose a thin adapter over existing operations
with identical guarantees. a tool protocol is not a reason to add another
orchestrator, receipt store or lifecycle owner.

## costs and limits

maintained examples and a discoverable contract are a small ongoing obligation.
they cannot make guarded terminal input reliable semantic messaging; that requires
the separate [managed-collaboration decision](managed-collaboration.md).
do not promise all provider capabilities through a common denominator or force
native agents into a reduced harness. retain the native terminal escape hatch.

resolved when: a fresh agent can use documented operations to address the intended
host/session/pane, distinguish successful creation from prompt delivery, and handle
unavailable/unknown/partial outcomes without inventing transport or replaying input.
proof should exercise the public surface with isolated dummy work where sufficient;
provider acceptance is never inferred from those terminal-only results.
