# agent control

[terminal control](terminal-agent-control.md) owns ordinary session status, input,
waits and closure. [native conversations](native-agent-observation.md) owns explicitly
addressed native operations. [qualification](native-agent-qualification.md) records native evidence;
[roadmap](roadmap.md) indexes delivery. stock source passes isolated acceptance;
installed-fleet acceptance remains open.

## targets and ownership

a terminal and native conversation are separate targets. tmux owns terminal
lifetimes/processes; providers own conversations, work and history. exact native
commands capture machine/profile/history scope/conversation id. terminal switches
never retarget them. terminal commands retain the exact tmux lifetime and pane;
foreground process facts select heuristic rules and interruption keys only.

codex uses ordinary upstream npm installation and its native owning daemon.
creation starts/reuses that owner, captures thread/start's id, names the native
conversation and prepares it with exact native resume without a turn, records it in tmux session metadata
and launches stock remote/resume. manually launched codex terminals remain
unassociated; there is no manual association, replacement or clearing action.
direct native conversation addressing remains independent of terminal cards.
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
association is written only during creation. foreground Agent describes presence only;
Session.conversation is recorded identity metadata. Session.terminalStatus is a
terminal inference; explicit native inspect supplies native runtime/capabilities.

ordinary terminal read captures bounded rendered text; guarded send uses one
fresh heuristic check. text/keys are deliberate input. stop sends one interrupt
key on any terminal. close attempts that key then independently closes the exact
session; terminal-only close skips input. neither delivery nor closure proves
work stopped. inferred idle is advisory; wait never proves task completion.

explicit native read defaults to bounded latest assistant output; native history
remains explicit. reads never resume. native send distinguishes peer
and user input; experimental queue is unavailable before dispatch. stop captures
exact active turn; idle proves no task completion. mutations are one attempt,
native admission receipts never imply completion, and uncertain delivery is not
replayed. native failure never sends keys or selects another provider server.

native stop and terminal deletion are separate capabilities. no session metadata
selects a native stop automatically. uncertain outcomes are never replayed.

## presentation and acknowledgement

cards follow terminal evidence for both providers and retain controls for shells,
unknown programs and remote transports. details label any recorded native
identity as potentially different from the terminal. metadata is never a gate.

native output is a separate view replies action. opening captures known unread
ids; first successful native output presentation acknowledges exactly those ids.
failed reads/presentation or precommit storage failures acknowledge nothing;
postcommit errors report acknowledgement unconfirmed. later replies stay unread.
terminal Hello/attachment/reconnect and staying open never acknowledge.
reuse local reply-id stores and serialized merges; no copied content or cross-device
sync. claude same-pane previous-agent recovery retains its existing contract.
notification semantics remain with their separate pr; the
[combined-release dependency](issues/terminal-notification-coordination.md) must
be resolved before joint release. inferred working is no native work identity.

## boundaries

provider-runtime-control reads one strict bounded stdin request and emits one
result/error envelope. account environments derive from host profiles; callers
supply no command/path/endpoint. no prompt, output or credential enters logs.

[deployment](dev-server-handoff.md) owns ordinary provider installation and pinned
helper source. coordinated gateway/client schemas hard-cut together; immutable
v0.10.4 remains historical output. rollback restores the prior complete release.
[testing policy](rules/testing.md) owns temporary behavioral test retirement.
