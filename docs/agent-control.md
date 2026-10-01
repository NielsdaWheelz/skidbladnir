# agent control

[terminal control](terminal-agent-control.md) owns ordinary session operations.
[native conversations](native-agent-observation.md) owns explicit native capabilities.
[terminal attention](reply-notifications.md) owns notifications and terminal visits. [qualification](native-agent-qualification.md) records actual evidence;
[roadmap](roadmap.md) indexes delivery. stock source passes isolated acceptance;
installed-fleet acceptance remains open. the terminal-observation status below
is implemented and qualified on darwin and linux within its
[qualification](terminal-agent-control-qualification.md#terminal-observation-qualification);
physical-phone acceptance is pending.

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
client -> serialized device-local terminal notification owner
```

ordinary inspect/read/send/wait/stop/close use the captured terminal pane.
read returns rendered terminal text; send performs a fresh screen/composer check
before paste/submit. text/keys are deliberate input. stop sends one interrupt;
close then independently deletes the exact session. uncertainty never disables
stop or closure. terminal-only close skips interruption. receipts prove bytes
written or exact session closure, never provider admission or task completion.

explicit native inspect/read/send/stop/results use `/v1/conversations/{operation}`
and a captured Conversation. native resolution inspects identity metadata once;
ordinary inventory never calls native status/history. native read defaults to
bounded latest assistant output, never resumes, and never falls back to terminal.
experimental queue and Claude native input remain unavailable. native commands
never retarget after terminal switches or loss. uncertain mutations are not replayed.

## presentation and attention

ordinary status is inferred from the current local terminal as independent
activity, interaction and notice facts
([terminal observation](terminal-observation.md)): a working agent can also need
an answer, and ambiguity is unknown rather than guessed. native recorded identity
is secondary metadata. `ready` is exclusive green attention after a working-to-idle
transition with no request, menu or notice; it asserts no unseen text or task
result. [observation §6](terminal-observation.md#6-content-attention-and-filtering)
owns every other label and tone.
first actual terminal output presentation clears attention; the whole visit and
first qualified post-visit observation are quiet. unknown/stale/outage breaks
continuity. no reply viewer, result-id scan/store or human acknowledgement remains.
notification failure is secondary and never disconnects the terminal.
[terminal attention](reply-notifications.md) owns exact identities, revisions,
serialized merges, failure semantics, content and acceptance.

## boundaries

provider-runtime-control reads one strict bounded stdin request and emits one
result/error envelope. account environments derive from host profiles; callers
supply no command/path/endpoint. no prompt, output or credential enters logs.

[deployment](dev-server-handoff.md) owns ordinary provider installation and pinned
helper source. coordinated gateway/client schemas hard-cut together; immutable
v0.10.4 remains historical output. rollback restores the prior complete release.
[testing policy](rules/testing.md) owns temporary behavioral test retirement.
