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
