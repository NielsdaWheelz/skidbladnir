# profile usage in the desktop browser

accepted 2026-10-02; source implemented and qualified; installed composition
remains `NOT_RUN`. show each configured profile's
5-hour and 7-day quota usage in the main tui, with an expandable disclosure for
reset times, source and report age. codex uses its existing account daemon;
claude uses the structured input to its managed statusline.

this document owns the capability and implementation plan.
[architecture](architecture.md) owns shared invariants;
[desktop browser](desktop-browser.md) owns the surrounding frame and actions.
session views remain an independent navigation change.
[implementation](issues/profile-usage.md) and the
[inactive-claude follow-up](issues/claude-inactive-profile-usage.md) track open work.
qualification below distinguishes source, candidate-host and installed evidence.

## scope and invariants

usage belongs to a configured provider home on one machine. include declared
profiles with no active sessions. matching profile keys/labels across machines
do not prove the same authenticated account. keep the sampling machine visible;
never sum, deduplicate or average their readings.

quota observations do not determine terminal status, ready attention, completion,
launch admission or whether another turn can run. percentages alone cannot prove
exhaustion or recovery; credits, spend controls and model-specific pools may
affect provider admission independently.

scope is the desktop browser and the provider's default short/weekly quotas.
phone presentation, a public `skid usage` command, other quota pools/credits,
enforcement, notifications and automatic profile switching are excluded. no
screen fallback, `/usage` keystrokes, private account-http client, experimental
claude getter, hook extension, transcript parsing, watcher, provider fork or new
background service.

## acquisition

### codex

extend the existing `llm-calling` native helper with one profile-scoped `usage`
operation. reuse skid's validated profile environment and
`agentruntime.CodexEndpoint`; no caller home/socket or conversation/turn target.

initialize a connection to the already-running account daemon and call
`account/rateLimits/read`. consume its required `rateLimits` default snapshot;
do not pick the first map entry or substitute another quota pool. within that
snapshot, `windowDurationMins == 300` identifies 5h and `10080` identifies 7d.
primary/secondary position alone is insufficient. omit unknown-duration windows;
duplicate recognized durations are incompatible. normalize `usedPercent` and
optional `resetsAt`; other durations are outside scope.

the read creates no thread or inference turn. it never starts/restarts a daemon,
replaces an owner, answers another client's requests, logs in or forces token
refresh. native authentication remains provider-owned. missing owners and
unsupported methods/fields make only that profile unavailable. no notification
subscription is needed.

### claude

extend `dev-server/assets/claude/statusline.sh`, which already reads the documented
`rate_limits.five_hour` and `rate_limits.seven_day` fields. before formatting its
ordinary display, export only each window's `used_percentage` and optional
`resets_at`, preserving numeric precision. never save the whole stdin payload,
session id, cwd, model, prompt, transcript path or credentials.

publish one replaceable `skidbladnir-usage.json` in the existing absolute
`CLAUDE_CONFIG_DIR`; without that explicit home, publish nothing. the gateway
reads the fixed path under its configured `claude-work` home. the file contains
one flat json object with `schemaVersion: 1`, `reportedAt`, and optional
`fiveHour`/`sevenDay` fields in the normalized shape below. it is a latest
observation, not history or an account registry, and may survive restart with
its original age.

use a mode-0600 sibling temporary file and atomic replacement; clean the owned
temporary file on failure/cancellation. export failure must not alter or break
the statusline or print provider input. replace the whole report, including
omissions; never merge absent fields with an earlier report. omit expired windows.
an overridden statusline or an unsupported account yields no usable quota report.

`reportedAt` is the host time when the callback receives these fields, not proof
of a backend fetch. callbacks can repeat cached data. same-home writers use
last-completed-publication semantics; an older provider snapshot may replace a
newer one. label this source `last reported`. no producer lock, per-session store
or process-lifetime registration. idle/absent claude may have only a stale report
or none; the experimental getter remains a separate follow-up.

## gateway, data and clocks

add authenticated, machine-bound `GET /v1/profile-usage`, with no body or query.
it invokes no tmux and is independent of inventories, terminal observation and
conversation routes. return `{machine, observedAt, profiles}`, including every
configured profile even with zero sessions; an empty declared table stays empty.
reuse existing machine/profile descriptors. each profile additionally carries:

```text
source: native | statusline
readState: ok | unavailable
report?: {
  reportedAt: utc instant,
  fiveHour?: {usedPercent: number, resetsAt?: utc instant},
  sevenDay?: {usedPercent: number, resetsAt?: utc instant}
}
```

`ok` without a report is successful absence, such as no claude publication.
a report without windows means no displayable quotas were supplied.
`unavailable` carries no new report. expected read failures affect only that
profile. auth/machine/request errors and owned-data defects retain their existing
policies; no permissive reader or compatibility recovery.

the helper normalizes codex; `agentcontrol` reads claude and composes usage;
the gateway maps the wire record; `fleetclient` decodes it once; `sessionui`
presents that owned value. add no quota facts to sessions, terminal status,
conversation bindings or tmux options. existing `historyScope` identifies a
home, not an authenticated account.

percentages are finite and nonnegative. preserve valid codex values above 100
rather than clamp or infer admission. optional facts are omitted, never null or
zero substitutes. convert reset seconds to explicit utc. read one snapshot
version; incompatible versions are unavailable. bound the file at 4 kib and the
response at the existing 64 kib control-response limit.

each host request has a shared five-second deadline. read the closed profiles
concurrently, retain completed results and mark timed-out profiles unavailable.
the gateway owns cancellation/completion, with no recurring task or codex cache.
logs/evidence retain timing, shape and typed outcomes, never quota values,
account identifiers, homes or raw provider payloads.

take `observedAt` after collection. native `reportedAt` is when its read completed;
claude retains its producer timestamp. derive initial age from
`observedAt - reportedAt` within that host response and each reset interval from
`resetsAt - observedAt`. after receipt, advance with browser monotonic elapsed
time, anchored at each host response's receipt before fleet aggregation. never
compare remote timestamps with the desktop wall clock or another host's clock,
or replace source times with receipt time. a report dated after
the gateway sample has uncertain recency and is unavailable.

stale means report age at least 120 seconds or a failed latest read. even a
recent claude report may contain older backend data. retain stale values dimmed
until their known reset. at `now >= resetsAt`, show unknown and
`awaiting report after reset`, never inferred zero or renewed capacity.
an absent reset reads `reset unknown` and permits no predicted reset.

## refresh and browser interaction

reuse the existing five-second browser tick. schedule a separate usage read at
startup, when entering a new usage scope, and every 60 seconds while the
browser is active. measure cadence from attempt start with monotonic due times
and one current in-flight usage batch; coalesce repeated refresh intent without
a queue. this read lane does not use
the pending-mutation slot or await inventory completion. slow usage cannot delay
list results, keyboard handling, attachment or controls.

the table reads its filtered/default machine; the usage page reads its
filtered/all-machine scope. opening/closing usage or changing the machine scope
recomputes this set, retires/cancels its old batch and marks the needed hosts due
immediately. read only machines needed by the visible disclosure.

capture each batch's scope/identity and a monotonically increasing read
generation. only the current generation may admit results or clear the in-flight
slot; returning to an earlier scope cannot revive an old completion. retire and
cancel the batch before fullscreen attachment, stop scheduling while attached,
and read when due on return. cancel with the browser. retain last reports in
memory by machine handle and profile descriptor. failures may retain that report as
stale; successful reads replace it completely, including absent windows. clear
reports when machine identity or profile descriptors change. only claude's latest
report is stored on disk.

keep the view strip on row 1. directly beneath it, reserve a compact summary
labelled `5h/7d used` and the sampling machine. use the explicit desktop machine
filter, otherwise the configured default machine labelled `default`; never the
first reachable host. session selection, group and needs-input filtering do not
change usage scope or hide profiles.

show declared profile keys in configured order. display whole percentages rounded
down, with the unit once in the summary heading `5h/7d used (%)` and one `~`
for a stale profile item. use `—` for unknown and `*` for claude last-reported
values; teach the markers in the summary. the full page retains each value's
percent sign. use quiet typography, not ready-green or an
account-readiness badge. at 80 columns, the legend and four compact
`profile 5h/7d` items use two rows with ordinary values; wrap only between whole
items when needed. truncate long host labels by existing display rules. zero
profiles reads `no agent profiles`; failures keep profile names and details.

add `u usage` to table hints and `--help`. `u` opens a read-only page in the
existing frame without changing session/view selection. its rows follow the
desktop machine scope: one filtered machine, otherwise all configured peers,
grouped by machine and retaining outages. include unused profiles, both `used`
columns, source, report age, reset time/countdown and `no report`/`unavailable`.
up/down or j/k scroll; q/escape returns to the previous table selection/scroll,
subject to ordinary inventory reconciliation. the page has no session-action
keys or captured session target. forms/attachment show no usage strip.

ctrl-r on the table requests inventory and usage independently; on the usage
page it requests usage only. automatic inventory reconciliation continues there.
coalescing never relabels an old report as newly fetched. all values, legends,
outages and return controls remain usable at 80×24/no color. usage failure never
changes session action admission.

## implementation plan

2026-10-02: implementation is requested as one focused feature across these
owners, with temporary integration/live tests and adversarial review. installed
acceptance targets macbook, devbox and arch. public release and device work remain
separate operations.

1. **qualify sources.** verify the installed codex payload/default pool and
   supported claude input. prove the native usage-only path creates no thread or
   inference turn. preserve missing/reset fields and retain content-free results.
   schema generation/version replies alone are not live quota acceptance.
2. **helper.** add codex `usage` in `llm-calling`'s existing provider-runtime
   control owner; qualify its normalized result and record the exact source
   revision. installation follows that repository's default branch through the
   existing install mechanism; no revision pin or second native transport.
3. **producer.** extend `dev-server/assets/claude/statusline.sh` with the atomic
   report while preserving display. add no hooks or provider-auth/config changes.
   qualify source before any managed-script distribution or fleet apply.
4. **host and transport.** add usage beside `internal/agentcontrol/native.go`,
   the independent route/DTO in `internal/gateway`, and one decoded read operation
   in `internal/fleetclient`. reuse profiles, machine binding, deadlines/errors;
   leave inventory envelopes, existing profile DTOs and android untouched.
5. **tui.** update `internal/sessionui`'s model, existing tick, keys and frame,
   plus `internal/agentcli/run.go` help. summary/details share report presentation.
   add only required due/in-flight state, retained observations and page offset;
   no navigation history or persisted browser state.
6. **verify and close.** use a small set of temporary source/host/client checks
   for the acceptance below; remove them before commit under
   [testing policy](rules/testing.md), run `scripts/check verify`, and record
   exact-source delivery/remaining issues. release, deployment and installed
   acceptance remain separate operations.

## acceptance

| boundary | required evidence |
| --- | --- |
| codex | actual helper→account-daemon read, correct home/default pool/durations, optional fields; no owner start/restart, thread, inference, terminal operation or retargeting |
| claude producer | synthetic input exports quotas only and preserves display; absence/reset replaces prior fields; default home does not publish; failed/cancelled writes preserve a complete previous file and remove owned temporary files |
| report semantics | cached callbacks/conflicting same-home writers remain last-reported observations; restart preserves report age; reports contain no account identifiers or terminal content; logs/evidence contain no actual quota values |
| gateway/client | zero profiles/sessions, one failed or slow profile, unsupported helper/version, host outage/machine mismatch; omissions clear old values; no caller paths or inventory/status dependency |
| timing/lifetime | 60-second scheduling, one current batch, manual coalescing, independent inventory completion, per-host receipt/monotonic elapsed age, 120-second stale boundary, exact reset expiry, table/page and a→b→a scope changes, attach/return and cancellation without late admission or slot clearing |
| actual tui | 80×24/no color summary and full page, all profiles and machine/default/source/stale labels, resets, scroll/return, stable group/view/session scope and unaffected controls under usage failure |
| installed composition | qualified helper, gateway/browser and managed producer on intended hosts; real codex/claude data visible through gateway→browser, with inactive claude honestly stale/unknown |

use dummy quota values where possible and actual owning code paths. live evidence
records shape/outcomes/absence of effects, not actual usage percentages. tmux
requires explicit current-turn approval and exact isolated `-L` resources;
phone/adb work is outside scope. engineering checks do not establish behavior.

2026-10-02 implementation qualification: temporary source checks demonstrated
behavioural red before green, including quota-only atomic publication, cached
callbacks, concurrent writers, failure cleanup and unchanged statusline display.
gateway/client/timing checks passed with the race detector. the actual browser
passed at 80×24 without color, including scope changes, read-only scrolling with
a selected session, omissions, stale/reset presentation, independent refresh and
quit during an observed blocked usage request. adversarial review covered the
implementation and test sensitivity. these temporary source tests are removed
before commit; `scripts/check verify host` separately passed engineering checks.
integration with the subsequently merged session views and searchable group
entry passed focused race checks and the updated actual-terminal probe: view
changes leave quota scope/cadence intact, usage returns to the selected group
and session, and forms retain their explicit create/save actions.

qualified `llm-calling` helper source:
`92cf72aa15817a6f0e14b4c5a861b8c6cdd6d589`. the producer's qualified bytes
remain unchanged through the shared-native-installation merge.

candidate composition of skid source
`a2fea3e6de46b1eee06e8cb1cde05b83ddd380ea` passed on macbook, devbox and arch using their actual
installed profile configuration and existing account endpoints. the changed
helper, gateway and fleetclient read weekly codex usage; those native responses
omitted 5h. arch's absent work/work2 owners remained absent and unavailable.
claude had no publication on all three hosts, reported as successful absence.
only usage reads were allowed; endpoint identities were unchanged, terminal
operations were fenced, and the owned candidate stages were removed. this is
candidate evidence, not managed installation or installed browser acceptance.

the installed-composition row remains `NOT_RUN`: the qualified helper must
reach its repository's default branch, the gateway must reach an admitted
immutable release, and the managed producer must be distributed before that
boundary can be tested. no release, installation, service activation or phone
operation was performed during source qualification.

## research evidence and limits

2026-10-02 research checked local codex cli 0.159.2 schema and account-daemon
version replies (0.160.0 for personal/work/work2), claude cli 2.1.287 and the
deployed managed statusline. that research did not test live quotas, exports or
rendered usage; subsequent implementation qualification is recorded above.
versions describe evidence, not pin-parity admission requirements.

the official [codex app-server contract](https://learn.chatgpt.com/docs/app-server#6-rate-limits-chatgpt)
defines account reads and quota/duration/reset fields. the official
[claude statusline contract](https://code.claude.com/docs/en/statusline#rate-limit-usage)
defines 5h/7d data and subscription/first-response limits; render callbacks do
not promise a new backend read. the
[published claude sdk types](https://unpkg.com/@anthropic-ai/claude-agent-sdk@0.3.287/sdk.d.ts)
include an explicitly unstable usage getter, which this plan excludes.
