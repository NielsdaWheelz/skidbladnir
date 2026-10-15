# desktop browser simplification

2026-10-15: implemented on `codex/desktop-browser-simplification` from `b253c05`.
source qualification is recorded below; installed usability remains pending.
this contract supersedes the machine-scope,
selected-footer, remote-row attribution and affected creation-copy clauses in
[desktop browser](desktop-browser.md), [session rows](desktop-session-rows.md),
[session views](session-views.md), [terminal continuity](terminal-continuity.md)
and [terminal usage](terminal-usage.md), including affected group/design clauses.
their other contracts and historical
evidence remain source-attributed. no open product decision blocks implementation.

## scope and final state

one fleet-wide browser with the existing needs-input/all/named-group/unassigned
views. every session row owns its sampled directory; info owns host identity and
detailed facts. the resting footer owns notices, usage, pagination and actions.

| feature / content owner | good content and required result |
| --- | --- |
| rows / row designer | name, complete status, relevant metadata, then one readable directory line; local/remote/unknown and absent/stale facts remain truthful without color |
| footer and creation / interaction designer | keys name effects; creation defaults and contextual creation have distinct meanings; no selected-session summary or detached action hint |
| fleet browsing / interaction designer | views describe work collections; every view, group suggestion and failed-host notice sees the full retained fleet |

phone already has no machine collection filter. retain its tabs, restoration,
machines/pressure/reconnect overflow and explicit forge destination. no android
production change. host names remain in info, creation forms/outcomes, captured
confirmations and host-failure notices; only ordinary roster identity is deferred.

## row content and geometry

reuse `current(row)`, never raw launch cwd or parsed provider text:

| resolved execution context | final directory line |
| --- | --- |
| local with cwd | sampled path |
| local without cwd | `directory unavailable` |
| known remote with cwd | `remote: <sampled path>` |
| known remote without cwd | `remote: directory unavailable` |
| unknown remote | `remote context unknown` |

stale/checking local rows retain sampled cwd under their existing whole-row
status. stale/checking connected rows use `current`'s existing unknown-remote
projection; never retain their remote cwd. info preserves full available sampled
paths and known destinations; unknown remains unknown.
remove name `on <host>` qualifiers and the duplicate `remote unknown` metadata
placeholder, including their width reservations. preserve other metadata rules.

body width `b = terminal width - 2`; names start at body column 2, metadata and
directory at 4. directory gets `b - 4` cells; `remote: ` consumes eight of them.
sanitize before measuring; `tail` truncates real paths from the left, preserving
wide/combining characters. absence prose stays whole. no home abbreviation,
filesystem lookup, new path normalization, path/action coupling or extra prefix.
use plain directory text for an available row, faint when its owning host is
checking/unavailable. fresh absence/unknown uses the literal text above; path
absence does not introduce a separate styling or readiness state.

retain summary breakpoints: inline at 120 columns for all/group, 138 for needs
input. every record has two content lines, plus one for semantically present
stacked metadata. path length never changes height. retain existing gaps,
heading admission, column caps and complete status. the selected gutter covers
every content line; no blank metadata row, partial record or dangling heading.

```text
research:
▌ review-api  ready
▌   Codex  work  sample-model · high
▌   /work/project

  shell       terminal
    /work/other-project
```

`tableLayout` owns measured content heights. its existing `viewport` alone
admits records; rendering, fitting/backfill and pagination use that contract
with the actual footer height. preserve session-lifetime selection across
refresh and resize. when no complete selected record fits, retain selection
and render no partial record. keep the existing below-80×24 behavior.

## footer and commands

ordinary-table order: fleet notices, quota summary, unlabelled boundary
with `i of n` only when scrolling, ordinary key hints. remove the selected name,
status explanation, attached count and directory, including the busy table's
target legend. retain explicit captured targets in info/editors/confirmations
and destination-bearing operation notices.

```text
enter attach  space info  s send interrupt  x interrupt and close terminal
n quick terminal  N new terminal options  T new terminal here  u usage  q quit
```

these logical groups occupy 74/78 cells in the fully enabled state. reuse
`hint`, `hints` and `keyLines`, including whole-hint wrapping and wide merging;
no command registry or fixed-height footer. `T` denotes shift+t and appears in
the second group only for an available local selected session, on the table,
with no operation in flight. retain ordinary availability rules for other keys.
`f` and `m` have no table action or help entry; arrows/h/l still select views.
this does not remove letters from form input or machine arguments from the cli.

| command | existing request and exact behavior |
| --- | --- |
| `n` | `start`, `LaunchTerminal`, configured default machine, cwd `~`, selected named group or unassigned; immediately create and enter |
| `N` | existing machine/launch/name/directory/group form, initially default machine, terminal, empty automatic name/home, selected group; explicit create submits existing `start`; agent profiles remain available |
| `T` | `shell` with captured selected-session `Ref`; gateway samples that exact source's cwd/group on its host, creates independently and enters it; never use displayed/truncated cwd |

`n` never follows the cursor, last form choice or first reachable machine. an
unavailable default refuses. `N` permits explicit other-host selection. typing
`T` on an available remote row refuses without dispatch or fallback; copy:
`terminal here unavailable for a remote connection; use N to choose a machine`.
confirmed creation selects/reveals the returned lifetime, changes view only to
admit its actual group, then attaches. attachment failure leaves it selected;
retry attachment only. failed/unknown creation never changes view or replays.
help describes all three destinations and uses the same labels as the footer.
quick/source creation says `opening terminal on <host>…`; retain internal `shell`.
initial or post-write checking is not host failure. use existing peer facts for
`checking inventory; refresh before creating` / `checking inventory; create disabled`
and the equivalent `refresh before searching`; a failed host retains its existing
unavailable/error copy. a target peer's known error wins over checking, including
early usage-auth failure. only the initial empty collection keeps checking until
inventory admission; it still shows that host's fault notice. no new readiness
state or error classification.

## architecture, api and state

```text
existing fleet inventory + attention -> admission -> peers -> selected view
  -> rows -> measured layout/viewport -> view
key -> captured fleetclient.Request -> gateway -> tmux
  -> existing creation receipt -> exact selection -> terminal attachment
```

tmux/providers retain runtime/history ownership. retain `Peer`, `Session`,
`ExecutionContext`, `Request`, exact references and existing gateway routes.
no public api, wire/config/persistence schema, dependency or polling change.
formatted strings never become identities or mutation inputs.

- delete `model.machine`, picker state/page/handlers, `inventoryMsg.machine`,
  scoped-result comparisons, header scope suffix, creation-completion scope
  switching and `scopedPeers`; consumers use `peers` directly.
- preserve `selectedView`, `Groups`, `ObservedGroups`, exact group labels,
  queue ordering and view-change selection. rename `rebuildForFilter` to
  describe view changes if retained; it still serves that behavior.
- replace `scopeReady` with `inventoryReceived`, false initially and true
  after the first admitted inventory success/failure. use it only for initial
  empty-collection presentation; it never resets. row/action eligibility uses
  the owning peer's `OK`. initial peers remain unavailable. private
  `targetChecking(label)` derives checking copy for creation/search from a known
  peer with `!OK && Error == nil`; an absent peer is not checking. this predicate
  adds no stored state and never authorizes an action.
- retain one inventory in flight, five-second cadence, `refreshAfterAction`,
  stale peer retention, access invalidation, notification admission, metadata
  reconciliation and directory-search revisions/cancellation. a pre-write or
  rejected read cannot restore action eligibility.
- remove the machine-scope-only restriction on opening `N` during metadata
  reconciliation. keep the editor's captured request/draft and prohibition on
  starting another metadata edit while checking. creation must not retarget it.
- retain `pageRow`, pending requests, busy duplicate suppression, exact refs,
  form/search machine fields and uncertain-outcome handling. deleting browsing
  scope must not delete execution destinations or asynchronous ownership.

freeze existing `tableLayout`, `viewport`, `tableLines`, `current`, `tableStatus`
and `rowFooter` signatures before parallel edits. a private pure
`rowDirectory(context, width)` in `layout.go` may own the path-formatting table;
reuse `tail` in `view.go`. footer removal must retain it for the row builder.
metadata presence must be tested semantically, not inferred from `height == 2`.

## exclusive ownership and sequence

| role | exclusive write paths / responsibility |
| --- | --- |
| row builder | `internal/sessionui/layout.go`: directory content, summary cleanup, heights, measured viewport/rendering |
| footer builder | `internal/sessionui/view.go`, `internal/agentcli/run.go`: footer, hints/help, header/picker removal, full-fleet notice reads, initial/disabled copy; preserve shared helpers |
| browser-state builder | `internal/sessionui/{session,navigation,create,details,metadata,group_entry}.go`: scope deletion, readiness, creation/defaults/refusal, view projection, exact-target preservation |
| root integrator | this spec; `docs/{architecture,roadmap,desktop-browser,desktop-session-rows,session-views,terminal-continuity,groups,design-language,terminal-usage}.md`; the three linked desktop issue records; disposable `internal/sessionui/browser_simplification_test.go` and `browser_simplification_live_test.go` |
| row designer | read-only: define row fixtures/content, review actual geometry and plain/color output |
| interaction designer | read-only: define footer/creation/fleet fixtures and copy, review actual hints, help, notices and transitions |
| independent adversarial verifier | no file writes: challenge each contract, oracle, implementation, refactor and final evidence |

root freezes shared field/helper names first. builders may then work in parallel
only in their listed files; integrate before execution. designers approve content
before builders and actual output after integration. reviewers cannot change
scope/acceptance; root resolves findings at the responsible owner. no broad
refactor, production test seam or modification to `scripts/check` composition.

[desktop usage](desktop-usage.md) owns quota content and usage-page keys/chrome.
compose overlapping browser-file changes serially at integration; this contract
removes machine filtering on return. preserve the usage changes and rerun affected
acceptance on the composed source. no concurrent writers to shared files or new
cross-feature machinery.

## red, green, refactor and acceptance

follow [testing policy](rules/testing.md). root writes two disposable probes;
the verifier reviews their sensitivity before any production edit. exercise
actual decoding, `Update`/`View` and the live composition below, not a replica.

1. **red:** freeze baseline/source and designer fixtures. probes compile on the
   baseline and fail changed row-location, footer, removed-key and `N`-during-checking requirements.
   unchanged invariants may pass; never manufacture a failure or change the oracle
   merely to fit the implementation. review evidence before implementation.
2. **green:** integrate the three slices; run the same probes. both designers
   inspect actual output; verifier checks behavior, ownership and failure paths.
3. **refactor:** remove dead scope/picker/footer/remote-width branches and
   duplicate layout decisions; search every caller before deleting helpers.
   rerun affected probes after changes; independently review the composed result.
4. **retire:** root deletes probes, fixture artifacts and owned processes, aligns
   final docs/help and runs `scripts/check verify host`. verifier checks final
   diff, deletion, links and content-free evidence. no retained suite or new gate.

| boundary | acceptance |
| --- | --- |
| rendered integration | real strict loopback inventory decoding → model/events → view at 80/119/120/137/138/160 columns, height 24, color and `NO_COLOR`; all/group/needs-input/empty; exactly one directory per visible row, no selected-footer facts/duplicate remote text, complete status, bounded cells; long/shared-suffix paths, wide/combining characters, every context/absence/freshness case |
| viewport | complete 2/3-line records at body capacities 0–9; first/middle/last selection, group transitions, scrolling, breakpoint/height resize, tail backfill and footer pagination agree; path length changes neither height nor summary columns |
| interactions | table `f`/`m` do nothing and remain literal form input; arrows retain views; all hosts supply groups/rows/notices, including empty filtered views; `n` uses default/home/group despite another selected host, `N` uses chosen fields, `T` exact source/refuses remote; unavailable default never selects a reachable substitute; info retains full available paths and known hosts |
| asynchronous preservation | press `n`/open `N` before first inventory: checking copy, no create dispatch; early usage-auth failure keeps collection checking but target refusal/error notice truthful; discarded inventory cannot settle startup, admitted success/failure does so permanently; delayed inventory across effects cannot revive stale admission; late directory replies cannot enter another machine/draft; pending confirmation/info keep exact targets; `N` during metadata reconciliation preserves editor/form ownership, including other-host creation; busy keys cause no duplicate effect |
| isolated live | actual browser/event loop/pty → production client → loopback gateways → owned isolated tmux on darwin and linux, at 80×24 and 160×24; two distinct dummy machines/sockets, default different from selection; execute `n`, `N`, `T`, observe created cwd/group/host and exact auto-attachment, detach/first navigation, refresh and source survival; offline default and lost creation reply prove refusal/no replay; terminal-only fixtures suffice |
| installed usability | after separately authorized release/deployment, verify matching version and have the user review narrow/wide rows, footer, paths and acceptable density; source evidence alone does not pass this row |

lost-reply oracle: withhold one successful creation response, observe exactly one
created session, then allow inventory refreshes; no second creation or automatic
attachment occurs. a later deliberate user command is not automatic replay.

reuse the documented [isolated-browser composition](issues/desktop-browser-runtime-acceptance.md#session-row-qualification):
`sessions.New` with unique `SocketName`, `gateway.New`, temporary client/config/
credentials/directories, and isolated browser-grid socket if needed. no default
socket, provider accounts, phone or installed-fleet mutation. tmux requires
explicit current-turn approval under [AGENTS.md](../AGENTS.md); no approval is
inferred from a prior turn or an environment flag. remove only recorded owned
sessions, close gateways/terminals, reap children; no `kill-server`. terminal
assertions stay in memory; evidence contains no terminal bytes or credentials.
unexecuted boundaries are `NOT_RUN`; engineering checks prove no live behavior.

record source/platform, red/green outcomes, designer/verifier conclusions and
cleanup in this spec and [desktop acceptance](issues/desktop-browser-runtime-acceptance.md).
update the existing [row](issues/desktop-row-readability.md) and
[cwd](issues/desktop-cwd-truncation.md) records for relocated acceptance; deleting
the footer does not resolve them. delete resolved records only after their
source and installed criteria pass. preserve unrelated outstanding acceptance.

## deliberate costs and exclusions

- one extra directory line per session reduces density; the two removed footer
  detail lines do not generally offset it. no third packing mode or density setting.
- machine/attachment/detail inspection takes `space`; identical names/paths
  across hosts require it. full directory inspection also resolves truncated ties.
- `n` no longer shows its destination before dispatch; its fixed configured
  default and help own that meaning. `N` shows the destination before submission.
- host-only browsing disappears; reaching needs input takes tab navigation.
- deleted probes provide change-specific evidence without retained regression
  coverage. installed readability remains a separate, explicit acceptance step.

excluded: android changes, host/provider observation or launch changes, native
controls, protocol/persistence migrations, new commands/search/palettes,
registries/settings, live failover, extra pollers, unrelated cleanup and deployment.
hard-cut the replaced client paths; no compatibility aliases, hidden filters,
fallbacks or parallel renderer. release/deployment and installed usability remain
unperformed.

planning review: row and interaction designers approve the content contract.
independent review corrected available-path claims, startup/error precedence,
concurrent usage ownership and the lost-reply oracle. no product question remains.

## source qualification

2026-10-15, production source `dc268db` from `b253c05`, before usage composition:

- **red:** the final disposable probes compile against `b253c05` and fail the
  intended footer, removed keys, row geometry/directory, creation copy, startup
  copy and independent `N` requirements. both live platforms fail only their
  changed-presentation oracle; existing creation/attachment invariants pass.
- **green:** strict loopback decoding → model/events → actual view passes the
  six-width, color/plain, view/context/absence/freshness matrix; capacities 0–9,
  exact heading/gap placement, scrolling/backfill and identity survive. checking,
  error precedence, rejected/stale reads, late search replies, captured targets,
  independent metadata reconciliation and duplicate suppression pass.
- **live:** actual browser/event loop/pty → production client → loopback gateway
  → isolated tmux passes on darwin/arm64 and linux/arm64 (container), each at
  80×24 and 160×24. `n`/`N`/`T` produce the exact host/cwd/group and attachment;
  detach/first navigation, refresh, source survival, unavailable default and a
  lost successful creation reply pass without fallback, replay or auto-attachment.
  darwin live: 43.466s; linux combined model/live: pass. this is terminal-only
  acceptance, not provider authentication or installed-fleet evidence.
- **review/refactor:** three disjoint builders, row/interaction content reviews
  and an independent adversarial review pass. review tightened two oracles
  (literal heading/gap placement and exact directory text); both pass. removed
  machine-scope/picker/footer/remote-width paths and inlined the now-single-use
  view-change helper; no broader refactor or new public contract was justified.

both probes, fixture roots/binaries, the owned linux image/container and recorded
inert socket files are removed. browsers exited, gateways/terminals closed and
owned sessions were removed; no owned live process remains. local documentation
links and `git diff --check` pass. `scripts/check verify host` passes after probe
deletion: formatting/syntax/lint, generated assets, dependencies, go vet and build.

installed version/usability remains `NOT_RUN`; no source result substitutes for
it. the broader
live stop/close and metadata-editing gaps retain their existing issue record.
