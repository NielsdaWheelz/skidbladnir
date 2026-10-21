# terminal-only control qualification

2026-10-21 deployment amendment: the following records isolated source evidence,
not current installed status. [v0.20.0 installed qualification](v0.20.0-installed-qualification.md)
owns the subsequent merged release, private configuration and fleet rollout.
jarvis remains stopped with its separate access issue; linux claude's open live
cells remain open. historical source/result attribution below is unchanged.

status: source implemented; isolated qualification recorded 2026-10-20/21.
full live acceptance remains partial: [open cells](issues/terminal-only-live-qualification.md).
[the approved spec](terminal-only-control.md) owns scope and acceptance. installed
services, desired private yaml and phone remain unchanged. no manual provider
daemon lifecycle/settings operation was performed; ordinary provider launches
retain their own behavior.
`NOT_RUN` is not a pass. no installed cutover is claimed.

## source composition

| repository | isolated branch | baseline |
| --- | --- | --- |
| skidbladnir | `feat/terminal-only-control` | `bc93dd2baf97abee3ef080cd3a4192fe4e89b049` |
| llm-calling | `feat/quota-only-helper` | `a984d71f2a8a41aa0b4174f8dfa74d3e0dafe00a` |
| dev-server | `feat/plain-skid-launches` | `62e0e2396e1c1a6b80bc5ea95e143574fde40150` |
| jarvis | `feat/terminal-only-workers` | `92092f8ce5f043b010acef8ee9eddc9962a9f291` |

quota entrypoint source: `e227c46254956860a1ffabff5e1d157c7c1ba716`.
skid product/artifact source: `096e9cc4bd65ce5fe8d01a5c0d466b58bdba3bbd`;
this adds the reviewed placeholder correction to `2c2401e`.
jarvis v9 source: `2d281a039b9c49e6cb206b0264d7369f826faa7b`.
dev-server source: `8a30da728f0f7c7f3c00c38d39981f2e5e960d5b`
plus its following documentation correction.
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
| packaging | old bundle lacks quota entrypoint/launcher; old failure envelope and exclusion manifest; old host rejects narrow request | actual remote immutable quota source, frozen base wheels/managed runtime; real signed darwin/linux archives pass public installer admission; actual host→packaged helper→real websocket fixture passes; no SDK/plugin/old launcher; signed android and complete eight-asset release checker pass |
| ordinary launch | preservation already passes baseline | real authenticated gateway→isolated tmux→candidate shell entry passes bash/zsh on darwin/linux: alias/function/PATH, quoting, inherited environment/home despite differing metadata, cwd, once-only evaluation, return/failure shell survival and exec replacement; baseline/candidate both pass |
| live controls | preservation proof | stock codex and exact claude-work on darwin/linux pass foreground/read, websocket identity/output/detach, specific draft/setup guard refusal, stop and separate close. codex literal stdin turns pass initial write, working/idle waits, cancellation, timeout and actual hub ready/device acknowledgement across the recorded probes. darwin claude passes guarded input, working/idle, literal response, cancellation, ready/acknowledgement and close. corrected linux claude public guard writes initial input, then idle/cancellation/read/close pass; working, response and ready remain unproved; see limits below |
| claude placeholder | actual linux public initial input refuses `TerminalInputBlocked/not_sent`; all visible placeholder glyphs are dim but internal spaces have default style; authored dummy frame reproduces draft classification | exact default-style ASCII spaces now permitted among dim glyphs; 466-frame race replay passes, including new visible-undim, reverse-space and background-space refusals. actual public initial input changes to written; no word matching or weakened draft/unknown guard |
| real existing-account quota | preservation proof | exact signed packaged helper reads already-running codex daemon once on each darwin/linux host; normalized recognized reports; no provider lifecycle, conversation call, prompt or history access |
| jarvis/postgres | old native schemas/compact output accepted; retired v8 decoded as current; current/unknown old-argument escape | actual postgresql 16.15 plus public CLI/signed state/authenticated TLS/isolated tmux fixture passes read/action recorders and executor, send, wait/cancel, changed pane and lost-reply no-replay; finalized v8 matrix remains opaque, unfinished waits/required events block; 13 distinct checks and strict engineering verification pass |

quota library baseline: 989 passed, 1 skipped, 40 deselected. candidate with
temporary proof: 1019 passed, 1 skipped, 40 deselected; existing pydantic warning
unchanged. ruff/format and entrypoint pyright pass. quota/deployment tests were
removed after independent code/sensitivity/refactor review. all other disposable
tests/controllers, test-only modules/binaries, owned sockets, database/forward
and fixture credentials are removed after final reviewed proof. the retained 466-frame
classifier corpus passes; it remains independently authorized.

## codex requested-directory live check

2026-10-21: all six cases pass with stock codex 0.163.0: `codex`, `codex-work`
and `codex-work2` on both darwin and linux. actual public `start --cwd --stdin`
uses authenticated isolated gateways, genuine signed enrollment and owned tmux
servers. gateway/parent cwd differs from each fresh requested physical directory,
whose name contains spaces and lies outside a git project. configured shell,
environment and account metadata remain ordinary; only the temporary config's
commands take the approved plain values. no injected `--cd` or `--remote`.

each case submits one request for `/bin/pwd -P`, redirecting tool output to a
unique initially absent owned receipt outside the requested directory. expected
cwd is omitted from the prompt. both the exact rendered codex tool block and
independent receipt bytes must match; model response and pane cwd cannot pass.
root and the independent reviewer privately verify all six physical cwd receipts
and owner markers. all cases also show matching foreground profile, written
input, idle and separate interrupt-written/terminal-closed receipts. six inputs,
zero retries, no setup answers or unexpected requests. compact tool presentation
does not independently expose a hidden tool `workdir` argument; this establishes
actual tool execution in the requested project directory, not that internal fact.

the controller links source `337246e` (documentation after product `096e9cc`),
and its ordinary shell launcher is the exact signed `096e9cc` binary. reviewed
controller source sha256: `6f24ea432bb9f801ef324d6a24d9bfe8a2c990367700a42371dd607d3d533ce9`.
six hashed content-free case logs, process outcomes and cleanup receipt remain in
`/tmp/terminal-only-cwd-results`; concise evidence is
`/tmp/terminal-only-client-cwd-evidence.txt`. temporary controller sources,
modules/binaries, six owned roots/sockets and the remote stage are deleted after
independent review; final owned-residue checks are zero on both hosts.
this is isolated live qualification,
not an installed deployment or a before/after reproduction of the retired remote
launch defect. the separate linux claude issue remains open.

## artifacts and engineering

local prospective `v0.20.0` artifacts are in
`/tmp/skid-terminal-only-placeholder-release-assets`: exactly eight owned assets. both host
archives have the pinned ed25519 signature, mac app has its pinned signing
certificate, and android has the pinned public signing identity. production
`scripts/check-release --source <clean-source> <assets> v0.20.0 <096e9cc-full-sha>`
passes. signed release android assembly from that source also passes.
full `scripts/check verify all` passes from the clean exact source;
`go test -race` passes the affected disposable boundaries and retained frames.
jarvis `scripts/verify` and strict pyright pass; dev-server bash/shellcheck and
ansible syntax pass. source/documentation link and diff checks pass.

these are signed local candidates, not a tag, hosted release, published pin or
installed receipt. the exact quota source is reachable on its feature branch,
not merged. merge/release composition must precede a new dev-server release pin;
the existing published pin is intentionally not replaced with a fictitious one.

both matching host archives pass pinned-signature verification and actual public
installer admission without installation. matching darwin host→packaged quota
helper→owned real websocket fixture also passes under race. real account reads
were once per host using the earlier signed `2c2401e` artifacts; quota source
`e227c462` is unchanged. they are not relabeled as new artifact account reads.
earlier signed artifacts remain separate evidence.

raw terminal/provider/account bytes and credentials never enter qualification.
provider versions: codex 0.163.0 on both tested hosts; claude 2.1.304 darwin and
2.1.288 linux for initial controls; later diagnosis on both hosts and successful
darwin response sample 2.1.306 after ordinary upstream version changes. no test
provider update command was issued. normal native per-directory trust and
session history writes occur only for the empty owned test cwd. existing user
sessions/services and unrelated settings remain intact; no manual global policy
or authentication edit.

## live proof limits

the later positive probes use an explicitly supported-display fixture: inherited
`NO_COLOR` is removed before ordinary shell startup; no provider command, global
setting or account policy changes. its origin was not established. unsupported
unstyled placeholder text remains a draft, never an admission exception.

the notifier starts after the written receipt. darwin claude and linux codex
then prove the same captured foreground working/armed before the later ready
event and exact device acknowledgement, under unchanged production cadence.
linux codex's standalone literal-response predicate is false; task completion
is not claimed. darwin claude's literal response is observed.

post-fix linux claude uses the `096e9cc` gateway/classifier with the earlier signed
`2c2401e` child as ordinary launcher only; matching signed `096e9cc` packaging is
proved separately. public compound start writes input, fresh composer is empty,
working wait times out after 20 seconds with idle/recognized status, idle matches,
cancellation is `cancelled/not_sent`, bounded read succeeds, and interrupt/close
are written/closed. no working/armed observer, literal response or ready/ack is
observed. this run does not sample provider version or terminal environment;
the separate preceding no-input linux 2.1.306 cell probe is not substituted.
written and idle establish neither provider acceptance nor task completion.

one older linux claude guarded-send attempt lost its detailed receipt in a
temporary controller. its input remains possibly used/unknown and was never
replayed. subsequent positive probes are separately recorded requests; the
post-fix target follows a definite `not_sent` refusal. all original failures and
13 content-free supported-display logs remain, with hashes and cleanup receipt,
under `/tmp/terminal-only-supported-results`; concise earlier evidence remains
in `/tmp/terminal-only-client-live-evidence.txt`. temporary controllers, owned
servers, stages and sockets are removed. no additional paid-model campaign.

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
- linux claude working/response/ready acceptance remains open after admitted
  input. the responsible placeholder grammar defect is resolved; the remaining
  observation does not establish a new defect. no native fallback, relaxed
  admission, forced permission or repeated uncertain input substitutes for proof.
- installed coordinated cutover is separate and blocked in part by jarvis's
  existing service-state access boundary; no bridge is assigned here.
