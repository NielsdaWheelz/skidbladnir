# terminal-only control qualification

status: source implementation and isolated qualification in progress, 2026-10-20.
[the approved spec](terminal-only-control.md) owns scope and acceptance. installed
services, desired private yaml, provider daemons and phone remain unchanged.
`NOT_RUN` is not a pass. no installed cutover is claimed.

## source composition

| repository | isolated branch | baseline |
| --- | --- | --- |
| skidbladnir | `feat/terminal-only-control` | `bc93dd2baf97abee3ef080cd3a4192fe4e89b049` |
| llm-calling | `feat/quota-only-helper` | `a984d71f2a8a41aa0b4174f8dfa74d3e0dafe00a` |
| dev-server | `feat/plain-skid-launches` | `62e0e2396e1c1a6b80bc5ea95e143574fde40150` |
| jarvis | `feat/terminal-only-workers` | `92092f8ce5f043b010acef8ee9eddc9962a9f291` |

quota entrypoint source: `e227c46254956860a1ffabff5e1d157c7c1ba716`.
jarvis composes existing v8 generalized-consumer commit `2353878` before v9;
upstream security fixes and resolved issue deletion remain. source commits and
artifacts are staged for review, not released or merged by this evidence.

## disposable red/green evidence

| boundary | observed baseline failure | candidate proof and limits |
| --- | --- | --- |
| host removal | real HTTP conversation routes differ from unknown route; valid inspect invokes old helper; inventory exposes history scope, planted association and argv name | isolated tmux/authenticated HTTP checks pass: ordinary unknown routes, no helper invocation, no conversation/provider-session facts; same lifetime/pane/pid/handle and inert options preserved |
| cli/client | six retired syntaxes admitted before config; valid native ref accepted; old scope accepted; empty scope accepted | public CLI/HTTP/ref/closed-schema tests pass; obsolete syntax rejects before effects; captured terminal read/wait and uncertain write remain; no retry |
| quota presentation | expired native report masks fresh terminal; stale numeric quota retained; source absent from rendered output | actual desktop selection/cache/render tests pass, including same-key home-scope invalidation and whole-report selection; terminal estimates carry `~` and source |
| android decoder | eight expected schema/error failures | compiled product-model JVM proof passes; shared strict notification json remains; lint/debug assembly pass; no device acceptance claimed |
| quota helper | subprocess entrypoint absent | 30 subprocess + real unix/websocket fixture checks pass: strict request, normalized windows, no conversation methods/approval answers, no claude SDK import, timeout/cancellation closes connection |
| personal deployment | actual isolated apply publishes plugin and flagged commands | three convergence checks pass, default/private override twice, complete unrelated private bytes/mode preserved; wrapper account/memory/environment/literal arguments retained; external downloads/provider executables are fixtures |
| packaging | old bundle lacks quota entrypoint/launcher; old failure envelope and exclusion manifest | candidate proof pending |
| ordinary launch/live providers | pending | `NOT_RUN` |
| real existing-account quota | pending | `NOT_RUN` |
| jarvis/postgres | pending | `NOT_RUN` |

quota library baseline: 989 passed, 1 skipped, 40 deselected. candidate with
temporary proof: 1019 passed, 1 skipped, 40 deselected; existing pydantic warning
unchanged. ruff/format and entrypoint pyright pass. quota/deployment tests were
removed after independent code/sensitivity/refactor review. remaining disposable
tests stay only until their final review and green rerun. the retained 452-frame
classifier corpus passes; it remains independently authorized.

## decisions and limits

- quota helper read timeout is two seconds, reserving the transport's existing
  up-to-two-second close within the authoritative five-second host budget. this
  is tighter than the independent public helper's ten-second usage allowance.
- native quota retains managed python/uv and provider-runtime base dependencies;
  the optional claude SDK and all skid conversation dispatch are removed.
- desktop source disclosure fills an existing copy gap through the current
  status column and estimate marker, without redesigning rows.
- fixtures establish integration at the stated boundary, never stock-provider,
  signed-release, installed-device or external-delivery behavior by implication.
- installed coordinated cutover is separate and blocked in part by jarvis's
  existing service-state access boundary; no bridge is assigned here.
