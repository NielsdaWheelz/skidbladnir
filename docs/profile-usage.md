# native codex profile usage

2026-10-20: [terminal-only control](terminal-only-control.md) supersedes native
conversation targeting, association and controls in this document. only terminal
refs remain. quota uses `accountScope` and fresh native-account evidence, then
fresh attributable terminal evidence, otherwise unknown. earlier qualification
retains its recorded source; it is not current capability evidence.

[terminal usage](terminal-usage.md) owns all-host collection, consolidated desktop
selection and claude terminal evidence. [desktop usage](desktop-usage.md) owns
presentation and freshness policy, with [isolated desktop qualification](desktop-usage.md#qualification).
this document owns codex's native acquisition, normalization and report clocks.
the 2026-10-14 cutover
supersedes the earlier claude publisher and devbox-only contracts.

## acquisition and normalization

the bounded `provider_runtime.agent_runtime.quota_cli` helper uses
skid's validated environment and `agentruntime.CodexEndpoint`. it initializes a
connection to the already-running account daemon and calls
`account/rateLimits/read`. no caller supplies a home, socket or conversation target.

consume the required `rateLimits` default snapshot; never substitute the first
map entry or another quota pool. within that snapshot, `windowDurationMins == 300`
identifies 5h and `10080` identifies 7d. primary/secondary position alone is
insufficient. omit unknown durations; duplicate recognized durations are
incompatible. normalize `usedPercent` and optional `resetsAt`.

the read creates no thread or inference turn. it never starts/restarts a daemon,
replaces an owner, answers another client's requests, logs in or forces token
refresh. provider-owned authentication remains unchanged. missing owners and
unsupported methods/fields make only that profile unavailable. no notification
subscription or extra service is part of the terminal cutover.

## route and report

authenticated, machine-bound `GET /v1/profile-usage` accepts no query; a body is ignored.
it invokes no tmux and returns only configured codex profiles, in configured
order, including unused profiles. zero codex profiles returns `[]`. native origin
follows from this capability; there is no `source` member or claude variant.

```text
{machine:{handle,platform}, observedAt, profiles:[{
  key, label, provider:"Codex", accountScope?,
  readState:"ok"|"unavailable",
  report?:{
    reportedAt,
    fiveHour?:{usedPercent,resetsAt?},
    sevenDay?:{usedPercent,resetsAt?}
  }
}]}
```

`ok` without a report or with no windows is successful absence. `unavailable`
carries no new report. ordinary failures affect only their profile;
auth/machine/request errors and owned-data defects retain existing policies.
the strict go consumer and host change together; legacy native replies are
rejected. the shared session schema changes with android under terminal usage.

percentages are finite and nonnegative. preserve precision and valid values
above 100; never infer admission. optional facts are omitted, never null or zero
substitutes. reset seconds become explicit utc. the existing five-second host
deadline covers concurrent profile reads, preserving completed results and
marking timed-out reads unavailable. the native response remains bounded at
64 kib. the gateway owns cancellation/completion, with no recurring task or cache.

## clocks and display lifetime

take `observedAt` after collection. native `reportedAt` is when the read
completed. derive initial age from `observedAt - reportedAt` and reset intervals
from `resetsAt - observedAt` within that host. advance both with client-local
monotonic elapsed time, anchored at each bounded response's receipt before fleet
aggregation. never compare host clocks with desktop wall time or other hosts.
a report dated after the gateway sample is unavailable.

the [desktop policy](desktop-usage.md#percentage-and-time-rules) owns the stale
threshold and failed-read presentation. retained reports keep their original
receipt, host time and descriptor; successful reads replace the whole report,
including omissions. descriptor/identity changes
discard the old report. source selection follows [terminal usage](terminal-usage.md),
including selection of a newer stale eligible report ahead of older healthy data.

at a known reset, stop presenting that window's capacity; never infer zero or
renewed capacity. absent resets remain unknown. [desktop usage](desktop-usage.md)
owns remaining conversion, expired-window copy and optional short-window display.

## evidence

the native acquisition was previously qualified with helper revision
`92cf72aa15817a6f0e14b4c5a861b8c6cdd6d589` and installed default-branch revision
`4d270abd48dd61240b22b9d741ce737fa611f67b`. the v0.13.0 and v0.14.0 checks qualify
their recorded generations only. their claude-file and devbox-only observations
do not qualify the replacement. current source/provider boundaries and remaining
deployment work are recorded in [terminal usage](terminal-usage.md#qualification).

the official [codex account contract](https://learn.chatgpt.com/docs/app-server#6-rate-limits-chatgpt)
defines the native quota/duration/reset fields. logs and evidence contain shape,
timing and typed outcomes, never actual quota values, account identifiers, homes
or raw provider payloads.
