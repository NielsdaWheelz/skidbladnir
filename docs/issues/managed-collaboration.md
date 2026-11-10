# durable collaboration needs a work owner and recipient protocol

status: architectural proposal from the [workspace review](workspace-coherence.md).
provider conversation control, inboxes, task protocols and durable command receipts
are current [non-goals](../architecture.md#non-goals). recording this proposal does
not reintroduce them or authorize retired runtime/database machinery.

problem: skid already provides cross-host terminal control, but terminal delivery
does not establish that an agent received, understood or completed a request. the
desired journey "tell that agent this and know it received it" crosses that boundary.

impact: a parent can lose an instruction, repeat an effect or mistake idle for
completion if its workflow treats terminal receipts as semantic acknowledgements.
a persistent mailbox on the sender alone does not fix this.

## current boundary and source owners

`internal/agentcli/run.go` documents cross-machine replies, `--json`, full `--ref`
capture and separate creation/prompt outcomes. `internal/fleetclient/client.go`
routes a captured target directly; `start.go` owns one bounded create/readiness/send
composition. `internal/agentcontrol/terminal.go` observes and guards a send;
`internal/sessions` and `internal/tmux` own the exact terminal effect.

`written` means terminal delivery, `unknown` is never replayed, and wait matches
screen-derived state rather than completed work. a handle resolves the current pane
of a session lifetime; deferred operations need the originally captured full ref.
provider exit/replacement in the same pane does not itself create a new session
target. a screen guard reduces risk but does not atomically couple input with a
provider's consumption. preserve these contracts even if managed work is added.

## recommendation and ownership decision

first improve [existing action discovery](agent-action-discovery.md). for actual
durable collaboration, define a narrow managed-work boundary above terminal control.
ordinary interactive sessions and native providers remain usable outside it.

| owner | responsibility |
| --- | --- |
| tmux / native service manager | terminal/process lifetimes and their supervision |
| provider | execution, conversation, account and native approvals/history |
| skid gateway | host-local exact-target operations and sampled observation |
| persistent work coordinator, if selected | submitted work, outgoing intent, message/result relationships and reconciliation |
| cooperating recipient or supported provider channel | destination receipt, consumption acknowledgement and supported task events |
| clients | navigation, local drafts and presentation |

jarvis is the first existing orchestration owner to inspect, not an established
solution. this review did not assess its current implementation or deployment.
an ephemeral parent agent cannot own recovery after it disappears. name the
persistent owner, its access boundary, retention, shutdown, cancellation and restart
behavior before selecting storage or adding endpoints. the existing
[consumer/trust contract](../architecture.md#trust-invariants) also records that a
separate unix user does not automatically have supported installed skid access.

one coordinator is proportionate for one user. accepted cost if selected: managed
dispatch and reconciliation stop while it is unavailable; ordinary direct terminal
access need not. do not build elections, a broker cluster or multi-user permissions.

## minimum semantic contract to decide

- distinguish work id, message id, sender, recipient and exact execution target.
  display grouping must not define supervision or authorize a request.
- persist outgoing intent before any external effect. a timeout must leave enough
  durable information to reconcile the same request, rather than create another.
- a destination that promises deduplication must durably record message identity
  before admitting its effect. sender records alone cannot supply that guarantee.
  decide bounded retention and what a retry beyond retention means.
- distinguish accepted/persisted receipt, consumption, execution, result and owner
  acceptance. an agent's reported result is evidence, not automatic proof of success.
- keep recipient restart/replacement and cancellation explicit. interrupting a
  terminal is not confirmed cancellation of a task or its remote/background effects.
- use cooperative acknowledgement or a supported provider channel when dependable
  semantic delivery is required, even if the interface remains terminal-first.
  native adapters need not entail a chat renderer or copied conversation history.
- preserve uncertainty at a non-transactional terminal boundary. claiming a message
  before injection can leave it unsent after a crash; recording it after injection
  can leave delivery unknown. no inbox/outbox design makes those operations atomic.
  automatic retries of terminal bytes remain forbidden.

the owner must choose between improved supervision with uncertain semantic delivery
and managed collaboration with a supported recipient protocol. both can coexist,
but a durable dispatch-attempt record is not a reliable agent-delivery guarantee.

## verified t3 evidence

the review's "zero route" complaint is obsolete for the examined release. t3
v0.1.8 contains cross-host launch/message/wait/stop/questions and remote delegation,
marked beta. both hosts enable linked environments; grants have access ceilings
and a documented 30-day expiry. lower-access links restrict mutation to threads
they created and skip project setup scripts. the receiving host revokes the grant;
locally forgetting a link only removes the local relationship, not that grant
([released guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/remote-access.md#link-another-environment)).

source findings, not runtime qualification:

- forwarding reuses the destination's mcp command boundary, preserves the caller's
  access ceiling and namespaces idempotency keys per caller sharing a peer grant.
  attachments are refused. learn local execution and explicit delegation authority;
  do not infer process isolation within a trusted user
  ([forwarding](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/apps/server/src/peer/PeerForwarding.ts#L100)).
- remote `delegate_task` rejects worktree isolation and directs callers to an
  explicit remote launch. launch occurs before the parent task record; the code
  explicitly acknowledges that a crash between them can leave untracked remote
  work until the same request id is retried. recorded tasks can be followed across
  restarts; that does not close the launch/record gap
  ([stable remote delegation](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/apps/server/src/peer/RemoteDelegation.ts#L215)).
- the local orchestrator commits events, projections, accepted receipt and outbox
  effects together, then executes effects outside the transaction. acknowledgement
  means committed intent, not finished provider work. its pruned event log cannot
  rebuild all projections. steal the transaction boundary, not event sourcing by
  reflex; recovery cannot blindly replay effects tied to a lost provider process
  ([stable architecture](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/internals/overview.md#durable-intent-and-side-effects)).

## provider integration and rejected backend transplant

t3's provider driver identifies an integration; an instance identifies configured
account/runtime choices. its claude adapter uses the agent sdk's native tool/prompt
presets plus t3 instructions and mcp, normalizing approvals and events. this
preserves substantial provider behavior but is not an unmediated cli. the inspected
main adapters were about 12,467 claude lines and 8,128 codex lines: evidence of
integration obligation, not proof of bad code
([provider contract](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/docs/internals/providers.md),
[claude adapter](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/apps/server/src/orchestration-v2/Adapters/ClaudeAdapterV2.ts#L1807)).

do not adopt t3 as skid's backend casually. conversation orchestration and native
terminal ownership would otherwise compete over lifecycle. choose supported
provider delivery integrations only for a named requirement; keep capability
differences explicit and ordinary terminals available. t3's root is
[mit licensed](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/LICENSE);
retain its notice for substantial copied source. its contracts/ideas are generally
more portable than its orchestration implementation.

resolved when: the owner either explicitly retains uncertain terminal supervision
or approves managed-work semantics, selects a persistent coordinator and recipient
protocol, updates the scope and qualifies parent loss, recipient restart, lost
answers, duplicate requests, stale targets, cancellation and result recovery.
completion needs evidence at the supported recipient/provider boundary. a green
build, a durable sender row or a terminal `written` receipt cannot close this issue.
