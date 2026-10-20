# session latency qualification

2026-10-20. source: `codex/session-latency`, based on
`15f6b538692346fe1ecd9bdb405070db8e42ecb3`; this record belongs to the same change.
[the spec](session-latency.md) owns requirements. disposable tests exercised real
http/tls/websocket, unix socket, filesystem locking, pty, tmux and android
boundaries. fixtures used dummy content and isolated state. no production service,
session, notification memory, pairing or provider account was changed.

## diagnosis and red

the bottleneck was serial native work and publication/entry dependencies. tmux
itself was responsive. historical production observation found macbook scoped
inventory at 8.8–12.3 s and fleet inventory at 10.2–15.0 s; direct tmux lists
took 7.7–11.7 ms in the later interleaved sample. loaded host measurements did
not isolate scheduling, lock contention and subprocess cost.

baseline temporary tests established these failures before their corrections:

- sixteen unchanged rows: 230 native children, 1,860 ms; bounded-batch check failed.
- real browser pty: a healthy tls peer stayed unpublished behind a delayed peer.
- real terminal stream: synchronous presentation held the following payload.
- new greeting rejected, old count-only greeting accepted; actual phone baseline
  apk rejected the new greeting too.
- inherited character metadata prevented local repair; diagnostics lacked stages.

later adversarial witnesses found and repaired lost secondary notification errors,
an internal owner request's premature three-second deadline, truncated checking
copy, and wrong failure classification when a server disappeared inside a batch.
each affected witness passed again after correction/refactor. phone negative-ack
assertions wait for owner processing, rather than racing its asynchronous queue.

## behavioral acceptance

| spec | boundary and result |
| --- | --- |
| a1 | darwin and linux, native tmux 3.4/3.7c/3.8: exact framing, local/inherited/malformed metadata, naming/groups/agent/remote rows and surviving empty server pass. unicode, pipe, backslash and literal format text survive; raw newline cwd survives. tmux rejects control characters in titles, so none is falsely claimed. |
| a2 | native title/pane/manual-name/collision/disappearance races, server replacement/loss and interrupted rename pass; guarded repair returns actual outcomes. |
| a3 | expired queued mutation starts zero children; stopped isolated server fails http inventory in 5.15–5.19 s, then recovers after resume. startup timeout removes only its owned client; successful attachment accepts new input/output after startup cancellation. |
| a4 | actual browser pty and controlled tls: healthy row appears within 800 ms and advances on the next five-second tick during another peer's 15 s stall and stalled notification owner. failed retained rows disable controls. selection/drafts and affected-peer post-write fences pass; stale completions cannot settle them. |
| a5 | desktop/phone publish base before held context; replacement refresh completes before old context is released. new context eventually paints. desktop also delivers a completed obsolete result after source replacement and rejects it. source/lifetime/connection and android runtime/credential/read fences reviewed. |
| a6 | actual tls gateway/tmux/pty entry with stalled unix notification owner produces output in 81/86 ms, accepts detach, reports secondary failure and makes zero inventory/context/producer reads. full/empty/partial output, late/during-write seed, newer generation, reset, mismatched foreground, uncertain rpc reconnect and pre-greeting abandonment pass. real locked-store readers share one durable reset identity; contention is cancelable, malformed memory fails. |
| a7 | strict go and compiled/device android codecs reject old/null/unknown/malformed greetings. actual phone main queue orders hello before binary application. real page output, rotation, background/return, focus loss, websocket reconnect, reset/re-admission and notification-click intent pass. retained click/reconnect/post-create inventory admissions and separate rich-pane/attention identity reviewed. |

the phone was sdk 36, user 0. two device tests passed in 6.505 s using private
loopback tls servers and separate `latencytest`/`latencyred` application ids.
rotation changed actual orientation and sent a new resize without replacing the
attempt. native window focus loss and activity backgrounding were exercised.
notification click used the native intent's uri, flags and extras; this proves
click ingress, not a notification-shade tap or push delivery. no activity
recreation, touch/accessibility redesign or installed-production qualification is
claimed. android concurrency caps and duplicate-callsite composition were checked
in source; the device run does not independently measure three-source saturation.

## performance and bounded work

forty measured samples after five warmups; sixteen unchanged sessions. each entry
opened a new real terminal connection. no concurrent qualification job ran on its
measured host. ordinary host workloads remained; neither machine was forcibly
quiesced.

| host / boundary | p50 ms | p95 ms | max ms |
| --- | ---: | ---: | ---: |
| darwin, tmux 3.8, complete local http inventory | 52 | 59 | 143 |
| darwin, captured terminal first output | 96 | 169 | 330 |
| linux, tmux 3.4, complete local http inventory | 28 | 43 | 47 |
| linux, captured terminal first output | 38 | 57 | 66 |

both p95 targets are below 500 ms. darwin: 18 logical cpus, sampled load
7.57/5.92/6.05 and 81.44% idle. linux: four cpus, load 2.29/0.82/0.67. these are
ordinary-load reference runs, not an idle-host or reboot-cold benchmark.
the separate cold-connection/stalled-owner entry samples above used an already
warmed host. isolated real codex/claude launches, without authenticated prompts,
also match inventory/greeting foreground identity: inventory 70/64 ms.

unchanged inventory uses six native children: three `display-message`, one
`if-shell`, two `source-file`; no per-row read children. a final sixteen-row
sample took 44 ms. a separate successful diagnostic sample reported admission
12 ms, capture 16, normalization 6, foreground 10, terminal 0, projection 0,
total 45; zero means millisecond resolution. all performed stages were present.
failure/deadline outcomes remain failures, without a fresh partial inventory.
sixty-four sessions/four readers overlapping a real checkpoint completed with
maximum 559 ms, without unintended mutations. this is separate from warm p95.

## review, retirement and limits

independent reviewers challenged native predicates/framing, source publication,
wire/notification contracts, rendered copy and test sensitivity before retirement.
the long-host checking suffix remains visible; retained rows identify their last
observation; pending remote context hides source cwd/provider. no geometry or
visual redesign was introduced.

the final race run passed terminalclient, macnotifications, fleetclient, terminal,
tmux and sessions. sessionui's canceled-pty teardown reported a preexisting
[dependency race](issues/bubbletea-canceled-input-race.md): FAILED, explicitly
deferred by the owner. it does not invalidate the earlier publication assertions;
it is not reported as a successful race check. no dependency workaround was added.

all eleven temporary test files and their harnesses were retired after review.
`scripts/check verify all` then passed: host checks/builds, android lint and debug
assembly; no behavioral suite was retained. the four private phone test packages
have been removed; the original app was untouched by that qualification.
production installation, v4 memory cutover and publication were unperformed at
source qualification; [v0.18.0 installed qualification](v0.18.0-installed-qualification.md)
owns the subsequent coordinated rollout. installed production-entry/greeting
acceptance remains `NOT_RUN`. phone model splitting, phone interaction and visual
redesign retain their separate issue records.
