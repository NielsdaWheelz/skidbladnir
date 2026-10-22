# agent control

[terminal-only control](terminal-only-control.md) owns the capability boundary,
schemas and cross-system removal. [terminal control](terminal-agent-control.md)
owns exact targets and terminal operations; [terminal observation](terminal-observation.md)
owns screen classification and guarded input. [qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/terminal-only-control-qualification.md)
records actual proof, not promises from a build.

## targets and ownership

tmux owns terminal lifetimes/processes. providers own execution, accounts and
saved history. skid observes the current foreground provider and its account
environment; it does not discover or bind provider conversations.

`t-` handles select terminals through complete scoped inventory. captured refs
retain machine/session lifetime/pane identity through delayed actions; a changed
target refuses. names and launch profiles never supply target authority.
configured shell commands run once after ordinary startup, with normal aliases,
functions and environment. skid injects no provider policy or daemon lifecycle.

## operations and composition

```text
client -> target-machine gateway -> sessions/tmux/kernel
                                -> terminal screen classifier
quota client -> gateway -> internal/quota -> existing codex account daemon
```

read returns bounded rendered terminal text, identifying available scrollback
or visible-only scope. send performs a fresh screen/composer check before paste
and submit; text/keys are deliberate input. stop sends one interrupt. close
independently interrupts and deletes the exact session; terminal-only close skips
interruption. preserve separate receipt facts and refuse replay after uncertainty.
written, idle, interruption and closure prove neither task completion nor
provider acceptance/cancellation. waits observe the captured terminal only.

there is no native conversation selector, history/results API, native input,
turn interruption, conversation wait or registration hook. native account quota
does not supply conversation identity, readiness or execution authority.

## presentation and attention

status is independent activity, interaction and notice facts from the current
recognized local foreground and its screen. ambiguity is unknown. the terminal
attention owner retains its existing ready transition, durable acknowledgement
and visit semantics; native notification presentation remains independent.
no provider-session or recorded-conversation metadata appears in ordinary views.

quota uses one whole eligible report per profile: fresh native account report,
otherwise fresh attributable terminal evidence, otherwise unknown. claude has
only terminal evidence. account-scope changes invalidate previous reports.
retain source/freshness/estimate disclosure; never sum accounts or splice windows.

## boundaries

the quota client sends only initialize, initialized and account/rateLimits/read
and has no conversation dispatch. it uses existing already-running codex sockets without starting,
restarting or updating providers. no prompt, output or credential enters logs.
provider-runtime's independently published cognitive/archive APIs remain outside
skid. [testing policy](rules.md#testing) owns disposable acceptance tests; matching
host/client/worker schemas cut over together without a compatibility reader.
