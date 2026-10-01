# terminal control qualification

2026-09-30. qualified source: `92ee4a84b4701f780a9bc0377f56599ae2dc1403` on
`codex/terminal-agent-control`; red base:
`9590f3003566e5969b92de19fe3afa86ddb28f54`. that base captures the concurrent
source changes before this cutover. no release or deployment is implied.
the [qualification tag](https://github.com/NielsdaWheelz/skidbladnir/tree/qualification/terminal-control-2026-09-30)
preserves this historical source independently of branch cleanup.

temporary tests use real tls/http/websocket, cli, desktop ptys, isolated tmux,
stock codex 0.159.2 and claude 2.1.284, disposable homes and local model endpoints.
phone checks use a physical sm-s906w, android 36, with a separate qualification
application id and test certificate. production app/data remain separate.
evidence records outcomes and predicates, never terminal/model/account content.

| boundary | evidence |
| --- | --- |
| session/tmux | baseline failures for generic authority, rename and staged-input pane change; green exact lifetime, cancellation, server reuse, capture scope/utf-8, foreground change and owned-buffer cleanup |
| provider observation | actual idle/working sends, dialogs and draft refusals for both providers; 18×30 and 100×2 screens refuse guarded send, then restore idle; 36 parser counterexamples green; actual local/ssh context transition green |
| lifecycle | both stock providers: a→same-process resume b→exit→shell→other program→new agent, unchanged pane and recorded a, missing helper; real resumed request contains b history and excludes a; shell execution and program consumption require private receipts |
| gateway | 27 acceptance cases green: replacement routes, strict schemas, missing helper, real ssh/mosh, separate effects, uncertainty/cancellation, attachment ownership and deletion deadline; changed baseline behavior fails as intended |
| cli/desktop | baseline failures then green tls control, references, wait, opposite-kind decoding and removed flags/routes; actual 80×24 desktop s/c/x across all eight foregrounds, pinned rename and exact key operands green (172.34 s); duplicate/mixed-key sensitivity green (3.24 s); actual response-loss/no-replay checks green |
| android client | 13 changed executable cases red then green; native codec/exact-turn retention green on both generations; real tls/gateway/tmux, schema refusals, partial close and opposite-kind responses |
| physical phone | baseline generic controls absent; candidate s/c/x on shell, long command, both providers, unknown program, ssh/mosh and dead pane; stop retains, c closes independently, x sends no input; captured rename confirmation, terminal/card actions, inferred-source accessibility and exact written/unavailable accessibility feedback green; forge clearance red then green |
| native retention | actual codex creation/adoption, native receipts/read/results/queue refusal, exact stale/current-turn stop and daemon cleanup green; actual claude saved history/read/results green |

independent review challenged target authority, staging, deletion budgets,
response-loss wording, composer indentation/history/footers/separators/styling,
and real dialog/resize sensitivity. findings were repaired at their owning layer.
the reviewer writes and executes nothing; its `NOT_RUN` is not behavioral proof.

after temporary fixture removal, `scripts/check verify` passed: formatting,
syntax, dependency/catalogue/assets checks, go vet/build, android compile/lint/build.
these engineering checks supply no behavioral proof. final codex lifecycle passed
(7.45 s); claude lifecycle passed (6.65 s), with all eight independent
probe-oracle cases green. codex's post-interrupt screen lacked its usual footer
and correctly reported `unknown`; deliberate text still exited to the same shell.
owned backspaces prepared the fixture's command, separately from the one interrupt.
actual process absence and private shell execution proved handoff, not inferred idle.
claude seeds real interactive sessions; its
[print-mode sessions](https://code.claude.com/docs/en/sessions) are excluded from
the picker. cancellation restores a draft: deliberate text appends; guarded send
refuses. claude fixture exit preparation uses a fresh primary-model request and its
unique provider-written reply, independently of inferred idle and native id.
phone fixture cleanup passed; the separate test apk,
owned adb reverse, sessions and credentials were removed. expired fixture runs
are excluded from pass claims; remaining cases used a fresh final-source gateway.

limits: idle and working are terminal inferences; unrecognized interruption
screens can report unknown. one interrupt receipt proves
delivery only, never completion. recognition follows supported native executable
profiles; a node launcher without the canonical provider signature is a generic
terminal. model endpoints qualify provider interaction without paid network work.
claude native background-job stop is `NOT_RUN` in this cutover; its existing
[fleet qualification issue](issues/restoration-native-control.md) remains open.
this qualification predates the separately merged terminal-attention contract in
[pr 28](https://github.com/NielsdaWheelz/skidbladnir/pull/28). its former native
conversation dependency is retired; the [merged phone composition](issues/reply-notifications-phone-composition.md)
remains explicitly skipped/`NOT_RUN`. that waiver supplies no physical proof.
all 17 temporary go tests and the android verification fixtures were removed
before commit, forfeiting automatic regression
coverage as required by [testing policy](rules/testing.md).

## release composition

release integration starts from `98fe674f31679a7b16f3685c5e127677faa19f2c`.
that main already contains terminal control, automatic names/typed handles,
the browser view strip, descriptor ordering and terminal attention. this change
ports the remaining qualified detector, cancellation cleanup, deletion-budget
and content/viewport fixes without restoring retired native viewers or bindings.
original qualification above remains attributed to its original source.
focused composition uses real isolated tmux capture. 14 fullscreen cases pass,
including stock status/shortcut rows, passive title/warning metadata and guarded
refusal of footer-shaped wrapped drafts. adversarial review found historical
working chrome overriding a current composer; eight proximity cases failed
before the repair, then all 16 passed. current work requires adjacency through
whitespace or the paired claude border, or the final row without a composer.
review rechecked the repair and found no remaining defect in the release diff.
root `scripts/check verify` passed after the final production change.
real http/isolated tmux close checks use a 3500 ms parent deadline, reserving
the final 2000 ms. post-lock validation after a 600 ms wait takes 3501 ms on
the baseline and fails the reserve; the repair refuses at 1503 ms. lock timeout
refuses at 1500 ms. both preserve the target beyond the parent deadline without
late deletion. a fresh request after a 200 ms lock release deletes its exact
target in 247 ms, preserves both other sessions and releases the permit.
this fixture omits auth, provider interruption and attached-client cleanup;
those remain covered only by the earlier source-attributed matrix.
cancelled paste cleanup fails on baseline, then removes only its owned buffer;
foreground-change sampling fails on baseline, then retains fresh command,
identity and metadata with `unknown/terminal`. a zero-session mutation fails
the inventory oracle. all temporary composition fixtures and owned sockets
were removed; scoped vet/format/diff checks passed after their removal.
client composition: 12 real tls/http cases cover dropped/truncated responses
for combined/terminal-only close. eight wording cases fail on `98fe674`;
all 12 pass after the change, with exactly one mutation and no replay.
the phone update is explicitly pending by user direction; no fresh merged-source
device qualification or installation is claimed. existing pairings stay intact.
fresh stock-provider endpoint smoke is `NOT_RUN` in this composition; fullscreen
checks exercise real styled alternate-screen capture with fixture process
recognition. the earlier actual provider evidence remains source-attributed.

## terminal observation qualification

2026-10-01. this appends the [terminal observation](terminal-observation.md)
cutover; the records above stay attributed to their own sources. source: branch
`terminal-observation`, every slice merged at `4e1b737` over baseline `564d32f`,
then integration fixes `82c0589` (request predicate, resolve deadline, unparseable
codex rows), `5c59996` (stale table cell), `2599491` (tmux wait bound),
`e7b7695` (claude 2.1.284 theme picker), merged at `6859010`, and `131c1df`
(checking cell). no release or deployment is implied.

environments:

- darwin 25.4.0 arm64 (macos 26.4.1), tmux 3.7c, go 1.26.5. codex-cli 0.159.2
  native TUI, embedded or against a temporary-home 0.159.2 daemon; the product's
  question and idle rows also ran against a copy of the installed 0.159.3 daemon.
  claude 2.1.286.
- linux devbox: ubuntu, kernel 6.8.0 x86_64, glibc 2.39, tmux 3.4. linux arch:
  kernel 7.2.6 x86_64, glibc 2.44, tmux 3.7c. both run codex TUI 0.159.2 against
  the installed 0.159.3 daemon, the production pairing, and claude 2.1.284.

oracles: scripted local model endpoints (codex responses and claude messages
SSE, no credentials), the keys sent, the kernel's foreground facts, tmux's own
cli, and authored frames painted through real tmux, which prove capture and parser
mechanics only. every probe used its own isolated tmux socket and temporary
provider homes; evidence is content-free; every temporary test and harness was
deleted. the per-family record is each grammar's capability table
([codex §1](terminal-observation-codex.md#1-frozen-capability-table),
[claude §1](terminal-observation-claude.md#1-frozen-capability-table)).

| spec §8 group | darwin | linux |
| --- | --- | --- |
| capture | PASS: region bounds at 64–1000 rows; byte caps keep the bottom region's lowest rows and the top region's rows from row 0; rows self-contained; resize and alternate-screen races refuse the sample; OSC 8, SO/SI and tab cells parse; the worst-case 256-row guarded command fits tmux's 16 KiB command limit | PASS on 3.4 and 3.7c: the same; 3.4 writes no tab cell where 3.7c does; OSC 8, URI control bytes, SO/SI, trailing-space trimming and the command limit match (14326 bytes accepted, 16658 refused); 93 chrome code points parse at tmux's column |
| authored corpora (real tmux, parse, detect) | PASS: codex 160/160 at `82c0589` (`codex.go` unchanged since); claude 184/184 at `e7b7695`; a parser mutation fails them | PASS: codex 160/160 and claude 180/180 per host at `4e1b737`, rows exact, none unparseable |
| recognition | PASS: bare, relative, absolute and symlinked launches of a native image and of claude 2.1.286; copies, wrappers and unrelated programs unrecognized; suspend/resume, exit, exec replacement and respawn; a relink reaches the next launch and leaves the old image unrecognized; npm-launched codex is a generic terminal (accepted cost) | PASS on both hosts against `/proc`: on a native fake image, bare, relative, absolute, second-symlink and exec-wrapper launches, the negatives, exit, exec replacement both ways, suspend/resume, relink, replace-in-place, prune and a dangling link; the installed claude 2.1.284 recognized through its configured `executablePath` only, its other spellings `NOT_RUN` here ([since run](#2026-10-01-linux-claude-launch-spellings)); admission refuses dangling, non-executable and old-schema (`argument0`) configuration |
| provider behavior (live, scripted endpoints) | PASS for every family the grammars mark darwin | PASS for every family the grammars mark linux. claude 2.1.284's numbered theme picker read layout_unknown at `4e1b737` (FAIL, safe direction); it reads setup at `e7b7695` (live on `6618e75`, same behaviour) on both hosts at 26–100 columns, except the [narrow residue](terminal-observation-claude.md#5-accepted-costs-and-residual-ambiguity), an accepted cost |
| negatives | PASS: composer during codex work never idle; menu versus request (codex `/model` and f2 warnings, claude `/model` read `menu open` outside the filter); provider → shell → provider for codex (exit, and SIGKILL with tmux keeping the working title) and claude (working, question); historical errors under current work. quoted chrome: authored frames only | PASS: composer during codex work; daemon disconnect; provider → shell. the rest `NOT_RUN` |
| composition (inventory, inspect, send) | PASS: 340/340 frames through list and inspect; send on the 326 plain frames, and on the 14 clip frames under a real byte cap (14/14); 326/326 again with enrich; every refusal writing no bytes; live idle, working, draft, menu, question, permission, changed target and clipped for both providers. baseline pairs: codex read idle during a held turn and claude read unknown throughout | PASS: the managed create path (daemon-backed codex `--remote`, claude with production arguments and its identity hook) and inspect with same-sample valid diagnostics. guarded send `NOT_RUN` |
| full product (gateway → cli/desktop) | PASS except the stale and checking cells (live re-run `NOT_RUN`, [issue](issues/terminal-observation-stale-live.md)): every label and tone, concurrent work + question, unknown, unavailable, filter membership and order, creation clearing the filter, visits, empty copy, 80×24, `info`, `info --explain`, wait states. the stale table cell FAILED at `4e1b737` (faint `unavailable`); `5c59996` renders the projection, and `131c1df` keeps faint `checking` for a host being re-read while a failed host keeps the stale cell, each proven red → green through the real desktop model (`131c1df`: 38/38 checks, the 8 checking-cell checks red at `6859010`) | `NOT_RUN` |
| attention | PASS except pending ready → notice → idle (`NOT_RUN`: no grammar emits a current notice), recorded in [terminal attention](reply-notifications.md#disjoint-delivery-and-verification) | `NOT_RUN` |
| controls | PASS: guarded send into an empty composer (idle, and queued while working) for both providers; draft, dialog, menu, clipped, unavailable and changed-target refusals without bytes (claude's dialog refusal recorded, its byte check `NOT_RUN`); text, keys, stop and close unchanged | `NOT_RUN` |
| diagnostics and logs | PASS: explanation agrees with status in all 32 explained samples; rule ids are literals; envelopes, delivered receipts and gateway logs hold no content; `Terminal.ObservationFailed` logs `capture_failed` and `observation_timeout` | inspect diagnostics valid; logs `NOT_RUN` |
| cost (two-second enrichment budget) | PASS at 16 represented sessions including 120×256 and 100×150 panes: enrich median 27.4 ms, max 41.2 ms, against baseline 471 / 666 ms; one session 8.4 ms; no timeout; gateway inventory median 928 ms against 1307 ms. the production log's peak is 15 sessions | PASS: 16 sessions enrich median 25.8 ms (devbox) and 25.4 ms (arch) against baseline 378 / 289 ms; one session 6.7 ms (devbox) and 8.1 ms (arch); tall panes max 102 ms; no timeout |
| deadlines | at `4e1b737` a resolve-stage deadline read `TerminalTargetChanged` (14/15); `82c0589` reads `TerminalUnavailable` (15/15). live `observation_timeout` returns 200 with unavailable status and same-sample diagnostics. a stopped tmux server held responses until it resumed (pre-existing); at `2599491` each tmux command returns within 0.1 s of its deadline | `NOT_RUN`; tmux 3.4's identify path matches the source the fix relies on |
| physical phone | `NOT_RUN` ([pending](issues/terminal-observation-phone-acceptance.md)) | — |

client changes passed temporary red → green probes: go
ingress, projection, needs-input, wait, `info --explain`, 15 attention sequences
and the desktop filter and order through the real model, with 21 mutations
killed; android ingress, projection, attention, the needs-input chip, schema-3
capsules and narrow/large-text layout under robolectric. these are model-level
proofs, not device or terminal journeys.

cost found, not a regression: inventory's list spends about 54 ms per session on
serial tmux commands on darwin (36–57 ms on linux), unchanged from baseline and
outside the enrichment budget ([issue](issues/session-list-latency.md)).

each `NOT_RUN` above is an evidence gap recorded as an issue, except pending
ready → notice → idle, which no grammar can reach while none emits a notice; the
[roadmap](roadmap.md#non-native-status-and-needs-input--source-implemented)
indexes them with every other open item.
`scripts/check static host`, `go vet`, `go build` and `gofmt` passed at every
integration commit; these supply no behavioral proof.

### 2026-10-01 linux claude launch spellings

source: a root run on 2026-10-01, appended; the table above stays as recorded.
on devbox (tmux 3.4) and arch (tmux 3.7c), the installed claude 2.1.284 was
launched bare through `PATH`, by absolute path, by relative path and through a
second symlink, each in its own isolated tmux pane with a temporary `HOME` and
`CLAUDE_CONFIG_DIR`. each time it was the pane's foreground process-group leader,
and its `/proc/<pid>/exe` equalled the configured path's resolution
(`versions/2.1.284`); a script named `claude` did not match. recognition compares
only that kernel fact, through the matching path the linux recognition cell
qualifies at `4e1b737` with native fake images and relinks (the exec wrapper ran
only there); darwin met the same criteria with the real claude. the installed
claude's linux launch spellings therefore pass recognition. linux controls
through it remain `NOT_RUN`
([linux coverage](issues/terminal-observation-linux-coverage.md)).
