# agent control

[native conversations](native-agent-observation.md) owns schemas, behavior and
acceptance. [qualification](native-agent-qualification.md) records actual evidence;
[roadmap](roadmap.md) indexes delivery. stock source passes isolated acceptance;
installed-fleet acceptance remains open.

## targets and ownership

a terminal and native conversation are separate targets. tmux owns terminal
lifetimes/processes; providers own conversations, work and history. exact native
commands capture machine/profile/history scope/conversation id. terminal switches
never retarget them. terminal commands retain exact tmux/process identity.

codex uses ordinary upstream npm installation and its native owning daemon.
creation starts/reuses that owner and launches a stock remote new conversation.
skid never names providers or pre-creates their conversations. new codex sessions
remain unassociated. manual association, replacement and clearing are unavailable.
existing recorded bindings and direct native conversation commands remain valid.
no fork, selected-view protocol, newest-history inference
or separate provider service. CLI and daemon may upgrade independently; validate
consumed native methods/fields and fail affected capability closed.

claude's identity-only hook supplies its exact current conversation. native saved
history/results and matched background stop remain; native input is unavailable
following failed exact-recipient qualification. terminal prompting stays explicit.

## operations and composition

```text
client -> target-machine gateway -> sessions/tmux/kernel
                                -> agentcontrol -> short-lived helper -> provider
client -> device-local unread store
```

native inspect/read/send/stop/results use `/v1/conversations/{operation}` and an
explicit Conversation. start uses creation; wait is client-only. session metadata
association is read-only existing metadata; no writer remains. foreground Agent describes presence only;
Session.conversation owns native state/capabilities. no duplicate state projection.

native read defaults to bounded latest assistant output; history and terminal
capture are explicit choices. reads never resume. send distinguishes native peer
and user input; experimental queue is unavailable before dispatch. stop captures
exact active turn; idle proves no task completion. mutations are one attempt,
native admission receipts never imply completion, and uncertain delivery is not
replayed. native failure never sends keys or selects another provider server.

close explicitly targets a terminal and optional conversation halt. outcomes are
separate; closure preserves saved history and cannot assert provider queues or
another displayed conversation stopped. terminal-only close reuses exact deletion.

## presentation and acknowledgement

phone cards and `skid list` explicitly show tracking <conversation id>; the
desktop shows it on the selected row's facts line and in details. their
status/unread describes that recorded conversation, which may differ from
terminal contents. unassociated codex has no asserted native state or unread.
blocking reasons are omitted.

native output is a separate view replies action. opening captures known unread
ids; first successful native output presentation acknowledges exactly those ids.
failed reads/presentation or precommit storage failures acknowledge nothing;
postcommit errors report acknowledgement unconfirmed. later replies stay unread.
terminal Hello/attachment/reconnect and staying open never acknowledge.
reuse local reply-id stores and serialized merges; no copied content or cross-device
sync. claude same-pane previous-agent recovery retains its existing contract.

## boundaries

provider-runtime-control reads one strict bounded stdin request and emits one
result/error envelope. account environments derive from host profiles; callers
supply no command/path/endpoint. no prompt, output or credential enters logs.

[deployment](dev-server-handoff.md) owns ordinary provider installation and pinned
helper source. coordinated gateway/client schemas hard-cut together; immutable
v0.10.4 remains historical output. rollback restores the prior complete release.
[testing policy](rules/testing.md) owns temporary behavioral test retirement.
