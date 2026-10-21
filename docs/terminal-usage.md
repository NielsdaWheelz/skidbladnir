# terminal usage and consolidated profile disclosure

2026-10-20: [terminal-only control](terminal-only-control.md) supersedes native
conversation targeting, association and controls in this document. only terminal
refs remain. quota uses `accountScope` and fresh native-account evidence, then
fresh attributable terminal evidence, otherwise unknown. earlier qualification
retains its recorded source; it is not current capability evidence.

2026-10-20: [claude usage selection](claude-usage-selection.md) amends same-host
selection below; its [isolated qualification](claude-usage-selection.md#qualification)
passes. earlier qualification applies to its recorded source.

2026-10-15: [desktop usage](desktop-usage.md) supersedes this document's
presentation, stale threshold and usage-page keys; its [qualification](desktop-usage.md#qualification)
covers the isolated desktop boundary. this document retains acquisition,
whole-source selection and their qualification.

accepted 2026-10-14; source and installed v0.15.0 qualification below record the
exercised boundaries. this document supersedes the claude acquisition,
devbox-only sourcing and presentation clauses in [profile usage](profile-usage.md).
that document retains codex's native acquisition, normalization and clock rules.
[terminal observation](terminal-observation.md) retains status/control semantics;
[desktop browser](desktop-browser.md) retains navigation and action admission.

## outcome, scope and decisions

the usage page and footer each use the latest received eligible observation per
configured profile from any paired host. [desktop usage](desktop-usage.md) owns
their common remaining presentation. codex supplies native account reports;
claude supplies literal displayed used percentages or unknown. usage never
determines readiness, attention, provider admission, launch, switching, stop
or close.

the owner confirms that each profile denotes the same authenticated account on
macbook, devbox and arch. grouping by profile key/provider rests on that explicit
fleet invariant, not inferred equivalence of homes or labels. changing accounts
behind a profile reopens this contract; no account registry or discovery is added.

accepted costs: all-host inventory/native reads even in a group view;
bounded batches can wait for a slow host; receipt-order source changes may reflect
network timing rather than newer provider data. claude loses full precision,
backend report age, reset times and inactive-session coverage. custom-format,
narrow-screen and unicode ambiguity produce unknown. independent observations
are not an atomic fleet/account snapshot. foreground identity cannot prove who
painted retained cells; a matching old frame may survive until the provider repaints.

non-goals: extra service, poller, quota cache/store, screen fallback, private
account http, experimental getter, transcript parsing, `/usage` input, forced
callback, provider startup, identity-hook extension, new profile attribution,
phone quota ui, public usage command, notifications, enforcement or source picker.

## composition and ownership

```text
existing all-host inventory lane (5s)
  gateway -> sessions: exact foreground + existing focused capture
          -> agentcontrol: parse once; status/composer + optional terminal usage
          -> shared session dto -> fleetclient -> desktop claude candidates

existing all-host native-usage lane (60s)
  gateway -> agentcontrol -> existing helper/account daemon: codex only
          -> native profile reports -> fleetclient -> desktop codex candidates

desktop: select one whole observation per profile -> desktop usage projection
android: accept the new shared session field; present no quotas
```

the gateway owns no aggregate or recurring quota task. both client lanes keep
their current deadlines, cancellation, coalescing and generation admission.
ordinary inventory failures affect terminal candidates; ordinary native failures
affect native candidates. neither read lane may clear, stale or retimestamp the
other's facts. authenticated access/identity rejection revokes that host's current
eligibility across both through existing access admission/recovery; add no health
owner or cross-lane cache wipe.

## claude recognition

extend the existing claude composer/footer grammar; reuse `parseScreen`, the
composer anchor and `claudeReadFooter`. quota recognition adds no focused capture,
process read or native call per observed pane. require a recognized local claude
foreground and an ordinary anchored composer/footer. unknown profile does not
prevent transport; only proven profiles enter the desktop summary.

accept the managed formatter's exact final suffix:

```text
suffix     = " │ ctx " context " │ " windows
context    = nonnegative-integer "%" | "--"
windows    = "5h " percentage
           | "7d " percentage
           | "5h " percentage " · 7d " percentage
percentage = integer-in-[0,100] "%"
```

integers use ascii digits and canonical decimal spelling: zero or a nonzero
leading digit; no sign, leading zeroes, decimals or exponents. context is only an
anchor, not quota. the current formatter floors percentages; values above 100
are deliberately unsupported screen evidence, not an impossibility assertion.
the documented [statusline fields](https://code.claude.com/docs/en/statusline#rate-limit-usage)
define consumed quota; the displayed string is our formatter's convention.

the suffix must fit one parsed physical row immediately before the recognized
mode row, within the opaque footer band. no row joining, token salvage, alternate
format guessing or search outside that band. trailing text, changed separators,
reversed/duplicate windows or multiple complete formatter suffixes in the band
omit the whole pane observation. preserve real `0%`; independently absent windows
stay absent. unrelated percentages, quota-wait messages and exhaustion notices
never supply numerical values.

require a conservative spare column after the suffix: normalized arbitrary prefix
utf-8 byte length plus known suffix cell width must be less than pane width
(`│` and `·` each occupy one known suffix cell). do not use `len(row.cells)` as
native width: authored graphemes can disagree with tmux. this rule rejects some
fitting unicode rows deliberately. prefix metadata may wrap; the known suffix
must remain intact on one row. missing required anchor/footer/suffix rows omit
quota; clipping unrelated transcript rows does not.

quota recognition must not mutate `claudeScreen.cut`, status causes or diagnostics.
activity, interaction, notice, reason, composer and rule ids remain unchanged.
unsupported evidence is successful omission, never an inventory error.

## schemas and capability contract

all optional wire fields are omitted, never null. reuse handwritten strict codecs
and existing machine/session identities; no schema version, negotiation or codegen.

```text
Session += {
  terminalUsage?: {
    fiveHourUsedPercent?: integer 0..100,
    sevenDayUsedPercent?: integer 0..100
  }
}
```

the object must contain at least one window. reject null, empty objects, unknown
members and values outside the codecs' integer type/range at each transport
decoder; canonical spelling is a screen requirement, not new JSON machinery.
presence requires local `agent.provider == "Claude"`, no remote connection,
`terminalStatus.source == "terminal"`, and reason `recognized`, `partial`,
`layout_unknown`, `evidence_clipped` or `evidence_conflict`. failed or invalidated
capture cannot carry usage. activity/interaction may still be unknown.
timestamps, resets, homes, account ids,
raw fragments and formatter metadata are excluded; containing inventory and
terminal identity supply its observation context. internal pointers/presence flags
preserve zero without an optional-value framework.

the shared session dto carries this field in `GET /v1/sessions` and existing
create/new-shell responses. go and android accept/validate it together. native
bindings, terminal references and operation request/receipt schemas are unchanged.
explicit terminal inspect does not gain quota output: `info --explain` must clear
its earlier `terminalUsage` when replacing status/time with the inspect sample.
raw terminal reads remain raw reads.

`GET /v1/profile-usage` remains authenticated, machine-bound and empty-body/query;
it becomes codex-only:

```text
{machine:{handle,platform}, observedAt, profiles:[{
  key, label, provider:"Codex", accountScope?,
  readState:"ok"|"unavailable",
  report?:{reportedAt, fiveHour?:{usedPercent,resetsAt?},
                      sevenDay?:{usedPercent,resetsAt?}}
}]}
```

include declared codex profiles in configured order, including unused profiles;
zero profiles returns `[]`. remove the obsolete `source` member and statusline
variant; native origin follows from this capability. unavailable carries no new
report. preserve native default-pool/duration validation, full precision, values
above 100, optional resets, five-second host deadline and 64-kib response bound.
no helper change or daemon start. old claude/source-bearing replies are rejected.

## collection, selection and lifetime

1. desktop inventory reads the full fleet. views remain local projections;
   machine filtering is removed by [browser simplification](desktop-browser-simplification.md).
   preserve result/admission and post-write fences. peer `OK` owns admission;
   monotonic `inventoryReceived` owns initial empty-state presentation only. a
   globally failed batch marks every requested peer unavailable.
2. stamp each successful host response with client-local monotonic `ReceivedAt`
   at the existing bounded-body receipt boundary, before fleet aggregation.
   extend inventory `Peer` with this non-wire value, following native usage.
   retaining failed inventory preserves profiles, sessions, host time and original
   receipt together.
   compare no remote `observedAt`/`reportedAt` values across hosts.
3. claude candidates derive from current admitted successful inventories, never
   from table-filtered rows or a new cache. require current foreground-observed
   `agent.profile` to match that host's configured claude descriptor. historical
   `launchProfile`, cwd and provider presence are insufficient. unknown-profile
   terminals may transport usage but supply no profile row.
4. within one host/profile, choose one actual pane snapshot by most known windows,
   then numeric session id and pane id. disagreement never excludes a candidate;
   [claude usage selection](claude-usage-selection.md) owns this correction.
   complementary one-window panes remain one window; never union them.
5. across hosts, select the whole eligible candidate with latest original client
   receipt; equal receipts break by machine handle. never splice windows. a newer
   partial candidate replaces the entire selected record, including omissions.
   another host may win after the former source disappears or becomes ineligible.
   empty/no-displayable evidence removes only that host's candidate; it may reveal
   an older eligible whole observation elsewhere.
6. successful no-match, capture/host failure, loss of recognition, pane/process/
   profile replacement, closure or identity change withdraws claude eligibility.
   retained unavailable inventory supplies no displayed quota. a pending refresh
   keeps the preceding admitted sample under existing inventory semantics; it
   neither renews its receipt nor creates a backend-freshness claim. replacement
   withdraws the former identity's observation; a new matching capture can still
   contain retained earlier display cells until repaint.
7. codex candidates retain successful reports by machine handle/full local profile
   descriptor. failures preserve original times and mark them stale. success
   replaces the whole report; omissions clear that host's old fields. discard
   descriptor/identity changes. select one whole fresh native report with at least
   one currently displayable window; receipt order breaks ties. otherwise select
   fresh attributable terminal evidence; without either, show no numeric quota.
   preserve stale/expired diagnostics, never a third numeric source. known reset
   expiry omits that window. [desktop usage](desktop-usage.md) owns stale presentation; preserve
   host-clock/monotonic age math. access/identity rejection follows the common
   admission rule.
   failure diagnoses only profiles declared on that host or still unknown there;
   known-empty or unrelated declarations cannot contaminate another profile.
8. derive profile rows from the union of admitted inventory/native declarations
   in the closed configured order. native declarations can supply codex rows even
   without a successful inventory. join only equal key/provider; do not compare
   host-local
   `accountScope` as account identity. retain declarations through ordinary outage.
   show `no agent profiles` only when every paired host has supplied a full
   inventory declaration and their retained union is empty. while initial full
   declarations are in flight and no profiles are known, show checking; after
   that batch completes with unknown/failed declarations, show
   `profile declarations unavailable`. native-only empty replies cannot establish
   the whole fleet's absence.

the native read requests all peers at startup, every 60 seconds and disclosure
refresh; filter changes cannot retarget it. retain one current usage generation,
independent of inventory and the mutation slot. any relevant host codex descriptor
change retires its whole-fleet batch. forms hide usage; attachment/browser exit
cancel it under existing ownership. no new poller or cache is introduced.

table `ctrl-r` requests inventory and native usage independently. automatic
inventory continues on the usage page; [desktop usage](desktop-usage.md) removes
its manual refresh and owns its keys. return preserves selected view/session/scroll.
no session action keys or captured control target exist on that page.

## content and layout

[desktop usage](desktop-usage.md) is the sole owner of remaining conversion,
approximation, compact states, time formatting, page/footer geometry and keys.
it retains the acquisition limitations and source-selection rules above.

## non-overlapping implementation slices

the following slices and sequence describe the completed 2026-10-14 acquisition
cutover. the new desktop revision has its own narrower ownership and acceptance
in [desktop usage](desktop-usage.md); these historical slices do not expand it.

freeze these contracts before edits. each owner edits only its listed paths;
read-only adversarial reviewers write neither tests nor production files.

| owner | exclusive paths/responsibility |
| --- | --- |
| root integrator | `docs/terminal-usage.md`, `docs/profile-usage.md`, `docs/architecture.md`, `docs/roadmap.md`, `docs/desktop-browser.md`, `docs/terminal-observation.md`, `docs/terminal-agent-control.md`; shared value in `internal/sessions/types.go` and `internal/sessions/usage.go`; compose `internal/agentcontrol/terminal.go`, native-only `usage.go`; session/native dto in `internal/gateway/dto.go`, `usage.go`; final integration/evidence |
| classifier | `internal/agentcontrol/claude.go`, `detect.go`, optional focused extractor file, `frames_test.go`, `testdata/frames/`; exact grammar and unchanged status/composer/rules |
| go client | `internal/fleetclient/response.go`, `client.go`, `usage.go`; strict shared transport, per-host receipt times, codex-only native decoding, explain clearing |
| android protocol | `android/app/src/main/java/dev/niels/skidbladnir/ProductModel.kt`; strict inventory/create/shell acceptance; no phone display |
| desktop/content designer | `internal/sessionui/usage.go`, `session.go`, `view.go`, `internal/agentcli/run.go`; both existing lanes, consolidated selection, copy/layout/help |
| deployment producer | sibling `dev-server/assets/claude/statusline.sh`, `lib/ai-tools.sh`, `ansible/roles/ai_tools/tasks/main.yml`, `README.md`, `SPEC.md`; remove publishing/flags; preserve display/settings and ordinary install |

test authors own only explicitly assigned temporary files under their slice or
owned scratch directory. root assigns each path once; dependencies integrate
sequentially. parallel work may implement independent slices after value/schema
freeze; no peer can authorize a contract change. reuse existing optional JSON
validation, local home/profile registration, native normalization, receipt
boundary, polling generations, filter projection and presentation helpers.
extract only duplicated policy or substantial incidental complexity; no new
generic observer, candidate framework or provider adapter.

## sequence, adversarial review and acceptance

1. **red:** create temporary baseline-compatible end-to-end/integration probes
   using existing entrypoints and generic wire decoding. the old source must fail behavioral assertions
   for terminal transport, consolidated/filter-independent reads, literal used
   copy and removal of publishing. compilation failure is not red. extend the
   retained authored detector corpus/replay for grammar; do not rebuild retired
   harnesses or add production seams. new typed fixture assertions follow the
   shared value; baseline missing-field probes supply the behavioral red.
2. **implementation:** classifier/shared host value first; then independent
   go/android/deployment consumers; desktop after receipt/schema integration.
   remove reader/publisher/flags and replace legacy sourcing before green.
   review each boundary's code and test sensitivity before dependent work proceeds.
3. **green:** run the same probes against the change; qualify isolated actual
   gateway→tmux→client→browser and managed installer boundaries, then actual stock
   provider rendering/native reads under separately approved live scope.
4. **refactor:** remove unreachable imports/branches and duplicated selection or
   presentation policy left by the cutover. preserve ordinary statusline output,
   native helper/reads and existing control semantics. reviewers recheck invariants
   and any changed behavior before repeating only affected checks.
5. **finish:** remove temporary behavioral probes before commit under
   [testing policy](rules/testing.md); retain only its explicit detector
   fixtures/pure replay exception. run `go test ./internal/agentcontrol` and
   `scripts/check verify`; engineering success is not live acceptance. record exact
   source/boundaries and remaining issues. release/deployment require their own
   coordinated qualification; no fallback, compatibility reader or mixed-generation
   claim. one-time operator cleanup may remove only fixed owned quota files;
   no runtime cleanup/migration machinery remains.

| boundary | acceptance required |
| --- | --- |
| grammar | zero, each/both windows, canonical numbers, duplicates/conflicts, trailing text, wrong region, identical transcript/draft/notice copy, right edge, unicode bound, wrapping/clipping; status/composer/rules unchanged |
| host/transport | exact foreground revalidation and pane/profile replacement clear facts; list/create/shell strict shape; malformed/null/empty/foreign-provider objects rejected; no extra focused capture or side effect |
| pooling | skewed host clocks, ordered/tied receipts, same-host disagreement, most-complete actual snapshot, partial/empty replacement, unavailable source and cross-host recovery, access/identity rejection, partial fleet declarations; no stitched windows or renewed failed timestamps |
| native lane | codex-only/empty declarations, unused profiles, one slow/failed owner, no daemon start/thread/turn, native precision/reset/staleness retained; native/inventory failures cannot erase each other's candidates |
| browser | machine/view/group filters and forms/attach/return retain targets and generations; all-host failures invalidate all requested peers; both refresh lanes, 80×24/no-colour footer/page, one row/profile, source/time/uncertainty copy, scroll/return |
| producer/cutover | synthetic documented stdin preserves display; no publication with any home/old flag; ordinary installer preserves unrelated settings and converges on repeat; old producer flags/source variants absent; coordinated host/go/android consumers |

tmux, device and live checks require explicit current-turn approval under
`AGENTS.md`; planning approval supplies none. only the test's exact isolated `-L`
resources may be changed. absent approval/device/live boundary is `NOT_RUN`, never
a pass. authored dummy data may be retained in fixtures; actual evidence contains
only categories, typed outcomes, call counts and timings, never screen bytes,
prompts, real percentages, homes, account data or credentials.

## final state

claude usage is ephemeral terminal evidence. codex remains a native account read.
the desktop owns consolidated latest-received selection; gateways remain
independent. no quota file, publisher, forwarding or new lifecycle owner remains.
legacy qualification remains historical and does not satisfy this acceptance.
the installed qualification below closes the coordinated cutover. the old
inactive-claude acquisition and devbox-publication
issues are retired by explicit supersession; inactivity stays an
accepted unknown, not a request to add an account getter.

## qualification

2026-10-14 qualification: skid implementation snapshot `6c9abc8` was exercised in
a worktree based on `8551cd3f9345429053c3d606d425746cc15ad3f9`. sibling dev-server
snapshot `9c67f11` was exercised in a worktree based on
`1e2fe9c4aa72fa1b0b72eba03783acfa04983fd4`. source checks do not qualify an
installed release.

pr integration rebases this change onto `362ac1f`, preserving the intervening
desktop-row and dashboard changes. quota mechanisms and the shared decoder
changes are unchanged; only documentation ownership needed conflict resolution.
the merged composition passes detector replay, engineering verification and a
temporary pure-model 80×24 render check: both exact quota footer rows, overflowing
grouped/remote/outage rows, selected-row visibility and colour/plain bounds, with
negative absence/offscreen controls. that temporary probe is deleted.
earlier live results retain their original source boundary; no new live or device
pass is claimed by the rebase.

| boundary | exercised evidence |
| --- | --- |
| classifier | 34 new authored quota frames; all 388 isolated-tmux captures replayed; earlier status/composer/rule observations unchanged |
| strict transport | original source compiled and failed target assertions; changed go list/create/shell/explain probes passed with race; actual android inventory/create/shell decoders passed 28 positive/negative cases |
| desktop selection | eleven temporary tests passed with race; baseline behavioral red and focused red for both review findings; skewed clocks, whole records, conflict/completeness/ties, reset/stale/failure/recovery, declaration completeness, both refresh lanes, generation retirement, creation and return targets |
| actual composition | isolated tmux foreground/registration/process/pane transitions through authenticated tls gateway and fleetclient; actual `sessionui.Run` through 80×24 pty and separate owned tmux emulator, colour/plain, source/uncertainty/scroll/return, changed values through both real read routes, no mutating http |
| host costs/native | one existing 14-row focused capture; native route invokes no tmux, ignores a corrupt retired file, preserves precision/over-100/reset omission, isolates failed profiles and returns empty declarations without dispatch |
| producer/installer | same four temporary probes red on baseline then green: unchanged display projections, no publication even with old argv, real settings/file installer preserves unrelated fields and converges on repeat; shell syntax/lint and ansible role syntax pass |
| stock sources | installed codex 0.162.1: passive helper reads supplied three available profiles and three windows in 941 ms; claude 2.1.299: actual callbacks supplied both window fields, passive resume omitted them; stock rendering with the managed profile/user settings, runtime display-only formatter and authored quota fields supplied both session windows with captured `partial` status |

darwin composition passed with race; linux/arm64 composition passed in a disposable
container with tmux 3.5a. an initial linux environment with tmux 3.3a failed before
quota admission because it lacks the existing required `server_sessions` format;
that run was not acceptance. stock claude field availability and authored rendering
are separate evidence: no live quota values or provider-freshness claim are retained.
all stock panes/workspaces were test-owned; no existing terminal or phone was used.
qualification used three minimal turns: the second exceeded the initial one-turn
limit and was disclosed; the final turn was separately approved. later rendering
checks were passive and used authored values. owned provider test state was purged.

adversarial reviews corrected the omission oracle, source-selector dead code,
expired-report failure disclosure and irrelevant-host failure contamination.
the publisher, claude reader and native `source` variant are hard-cut. temporary
behavioral probes are removed before commit; only the explicit detector corpus
and pure replay remain. final `go test ./internal/agentcontrol` and
`scripts/check verify` passed after retirement, including go vet/build and
android lint/build. release, fleet activation
and physical-device acceptance are `NOT_RUN` under this source-only implementation.

## installed qualification

2026-10-14: immutable [v0.15.0](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.15.0),
source `f92cb454b2484a5a0ddf88f14d385ab6b16ccbe1`, passes signed artifact and
published-release verification for all five assets. exact source passes
[hosted engineering verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/38636265945).
merged dev-server `6339edc3a923e3f926e855e60a0f07c7814b2ede` owns activation;
upstream and deployment pins agree. managed gateway installation and the scoped
existing claude settings installer ran on macbook, devbox and arch.

each installed cli and running gateway maps to the exact release/source;
`scripts/fleet verify` passes all three hosts. the shared display-only formatter
and both managed settings match on each host. authenticated native usage has only
the three declared codex profiles and the strict new schema. ordinary inventory
contains no claude usage windows on any host: accepted omission, not positive
stock rendering or inactive-account coverage.

each installed desktop passes table → usage → table → clean quit in an 80×40
plain pty: four consolidated profile headings, three native source disclosures,
one explicit claude unknown, codex remaining and claude used copy, and no legacy
per-host usage page. the original v0.14.0 mac client exposed the legacy title and
failed the target assertion before installation. quota pooling/filter/scroll
semantics retain the separately recorded source qualification. probe review
corrected viewport and undrained-exit oracles; authored controls reject legacy,
malformed, duplicate and empty-source displays. all temporary probes are removed.

all 14/3/0 original macbook/devbox/arch session lifetimes and observed provider
bindings survive. sampled auth, gateway identity, host/client configuration and
unrelated provider settings are unchanged. macbook gained one session; its mutable
claude global state also changed, recorded separately rather than treated as a
preservation witness. provider homes/history were not relocated or removed;
this claim rests on the reviewed managed write allowlist, not history digests.
macbook's managed retention setting converged to the existing 36500-day policy;
unrelated settings remained intact. only the fixed obsolete skid quota files in
configured claude homes were removed; no runtime cleanup path exists.

the connected android device updates in place to `0.15.0` / `15000`; its installed
apk digest matches the published pin and its original install time remains.
normal paired inventory renders without an enrollment prompt or protocol error.
phone quota presentation, new provider turns, reboot/recovery, native notification
delivery and rollback are outside this rollout's exercised boundary.
