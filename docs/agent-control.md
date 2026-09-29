# agent control

[native interaction and unread replies](native-agent-observation.md) owns
observation, capabilities, commands, schemas and acceptance. it replaces terminal
inference, automatic fallback, compound `stop` and `interrupt`/`kill` commands.
[isolated source qualification](native-agent-qualification.md) records evidence
and limits; [the roadmap](roadmap.md) records delivery. no fleet deployment is claimed.

## outcome and scope

find a named terminal on a configured machine, read native agent output, send
peer or explicit user input, wait for sampled state, stop current work, or close
the terminal. attachment and terminal text/keys remain explicit operations.
names use inventory; opaque references preserve the captured target. no
coordinator role, addressing store, message service or skid scheduler.

phone interaction remains the provider's terminal. cards add native status and
device-local unread. cli and browser share gateway contracts. jarvis and herdr
remain independently configured products.

## ownership and composition

```text
client -> existing target-machine gateway -> sessions / tmux / kernel
                                         -> agentcontrol -> native helper -> provider
client -> local unread store
```

`sessions` owns terminal/process identity; `agentcontrol` enriches and dispatches
after releasing the session lock. handlers decode, call and map. providers own
execution, history and queues. the helper is a short-lived json process; it
neither resumes conversations nor owns provider lifetime. codex's direct unix
tui connection publishes its selected view. claude's identity-only registration
supplies its exact session association.

## identity, state, and dispatch

native controls capture machine, tmux lifetime, active pane, foreground pid/start,
conversation and codex view revision. stop captures the active turn when required.
validate before dispatch and revalidate observations afterward. changed targets
reject; never retarget, infer readiness or retry a mutation.

status and finalized reply identity are independent. idle is neither a completion
receipt nor proof of an empty queue. unavailable history/status never becomes
empty history, idle or terminal inference. blocking reasons are omitted.

## capability and api contract

reuse `/v1/sessions/{id}/agent/{operation}` for `read`, `send`, `text`, `keys`,
`stop`, `close`, `results`. start uses creation; wait is client-only. read requires
native or explicit terminal mode. send distinguishes peer, user and native queue
capabilities. stop retains the terminal; close reports halt and closure separately.
terminal-only close retains identity/name-guarded deletion.

the [feature contract](native-agent-observation.md#5-api-and-dispatch) defines
fields, outcomes, deadlines and uncertainty. select the advertised method before
dispatch; native failure never substitutes keys. terminal/socket writing alone
confirms no provider admission or cancellation.

## output and provider command

`skidbladnir-provider-runtime-control` reads one strict stdin request and returns
one result/error envelope. operations reuse native inspect/read/send/interrupt/stop
plus result enumeration. process/view/conversation targets are explicit; account
environments and endpoints come only from validated host profiles. envelopes are
bounded to 64 kib. no prompt, output, account path or credential enters logs.

## clients, configuration, and presentation

`NAME --machine HOST` resolves once; `--ref` retains exact identity. native latest
output is the default read. history and terminal capture are explicit choices.
send defaults to peer; `--input user --queue` selects the provider queue.
text/keys remain deliberate terminal input. every mutation is one attempt.
claude native input is unavailable after failed exact-recipient qualification;
its native status/history/results and matched background stop remain available.

unread stores reply ids and acknowledgements on this device. first complete
enumeration establishes baseline. activation captures known unread ids; the
first successful terminal Hello acknowledges only that capture. later replies
stay unread. same-pane shell recovery retains a previous-agent badge; associations
never follow a different pane, unbound agent or deleted session.

## delivery boundaries

dev-server installs the fixed codex source patch and direct frozen helper using
existing accounts and sockets. coordinated release hard-cuts schemas; rollback
restores the previous complete release. [deployment](dev-server-handoff.md) owns
installation; [testing policy](rules/testing.md) owns temporary-test retirement.
no mixed-version reader, fallback, compatibility alias or new retained harness.
