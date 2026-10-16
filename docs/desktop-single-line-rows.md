# single-line desktop session rows

2026-10-16: implemented, source/live qualified and [deployed in v0.17.3](#installed-qualification).
installed owner readability review remains `NOT_RUN`. this spec owns row geometry, hierarchy and overflow
rules in [desktop browser](desktop-browser.md), [browser simplification](desktop-browser-simplification.md),
[session rows](desktop-session-rows.md) and [design language §19](design-language.md#19-terminal-browser).
their other behavior and source-attributed qualification remain independent.
[architecture](architecture.md), [rules](rules/index.md) and
[testing policy](rules/testing.md) govern implementation. no product question remains open.
planning and implementation content/layout/adversarial reviews pass.
[source qualification](#source-qualification) records the exercised boundaries.

## target and content contract

one session occupies one physical content line at every supported size
(at least 80×24). names lead; metadata, including machine/path, uses existing
`faint`. selected names remain bold with the existing cursor; status retains
its complete wording and tone. no blank line between ordinary sessions.
retain all-view group headings/boundary spacing, view ordering, selection,
footer, full scrollable `space` info and the below-minimum-size screen.

the owner chose this descending priority and display order:
`name → status → group → provider → profile → model/effort → machine/path`.
machine/path is one last-priority field. omit lower fields before further
shortening higher fields; retain a priority prefix, never skip a nonfitting
present field to rescue a smaller later one.

one content designer owns this feature. good content supports name recognition,
complete attention/status meaning, predictable omission and truthful execution
context in both styled and plain output. the designer authors dummy fixtures
before implementation and reviews actual output after green and refactor.

| field | exact presentation |
| --- | --- |
| name | shared width across the selected collection, capped at 28 display cells; middle elision when necessary, preserving both ends |
| status | existing `tableStatus`, complete; use this row's actual width, never the collection's maximum |
| group | needs-input view only: `in ` plus the assigned label, end-elided at 16 cells; unassigned is absent. all/group views already express membership through heading/tab |
| provider, profile | existing `rowMetadata`, full sanitized strings or omitted whole; unknown profile keeps existing `unknown` |
| model/effort | existing `rowFooter`, full sanitized phrase or omitted whole; never reparse/rewrite it. a known agent's `—` is present; ordinary terminals and unresolved remotes have no agent fields |
| location | existing `current(row)`; local `<machine> · <path>`, known remote `<machine> (remote) · <path>`, unresolved remote `remote context unknown`; missing cwd replaces path with complete `directory unavailable` |
| omitted details | one faint `…` at the body's right edge, separated from preceding content by at least two cells; means a present whole field was suppressed for space |

normal name/group/path elision does not trigger the whole-field marker.
naturally absent fields create neither padding nor a marker. all literal
external spelling is retained. stale local context retains its sampled cwd;
stale/checking connected context uses `current`'s unknown-remote projection.
never infer a machine, cwd, profile or model from shortened text.

## deterministic geometry

sanitize with `attention.SingleLine`, then measure/cut with the existing ansi
display-cell primitives, then style. preserve grapheme boundaries; never use
byte/rune count as terminal width. no final whole-line truncation or wrapping.

1. body width `b = terminal width − 2`. retain the two-cell cursor gutter.
   measure names over all current rows, not just the viewport:
   `n = min(28, widest sanitized name)`. pad every name cell to `n`.
2. an overlong name keeps a prefix of at most `floor((n−1)/2)` cells, `…`,
   then a suffix of at most `ceil((n−1)/2)` cells. at 28 this is 13 + 1 + 14.
   do not split a grapheme to fill a budget; padding absorbs unused cells.
3. render gutter, name, two spaces, complete status. append present metadata
   in priority order, with two-cell gaps and no global column padding.
4. first try all present metadata plus location within `b`, with
   `location budget = b − width(primary + all metadata) − 2`.
   leading padding aligns the entire location fragment to the
   body's right edge, preserving its internal machine/path spacing.
   if that cannot fit, omit location and retain the longest metadata prefix
   fitting `b−3`, including every gap. align the omission marker to the right
   edge; reserve its three cells before admitting the prefix. never leapfrog.

the current longest status is 33 cells: primary content costs at most
`2 + 28 + 2 + 33 = 65`; with the omission marker, 68 fits the 78-cell body.
a status-vocabulary change must recheck this bound, not clip status silently.
only name/status starts align; later columns may vary by row. scrolling and
supported resizes preserve name width; changed collection content may alter it.

location has a real width contract. keep the sanitized machine label capped
at 16 cells by existing end elision; preserve full remote marker and separator.
keep the current last-two-path-components rule, without filesystem lookup,
home substitution, normalization or provider-text parsing. for a known path,
require framing plus `min(8, compact-path width)` suffix cells, including any
ellipsis. below that, omit the whole location. otherwise reuse `tail` with a
strictly positive suffix budget. absence/unknown prose must fit whole; never
truncate it as a path. do not call `tail` at zero/negative width: its existing
loop assumes positive space. full info retains full available names and paths.

each session contributes exactly one viewport line. retain the current rule
for an interior group transition: gap + heading + complete first session.
never leave a dangling heading or edge separator; the first heading may be
omitted when only its session fits. keep exact-lifetime selection visible,
tail backfill and footer pagination through the same viewport calculation.

## composition, interfaces and scope

```text
existing fleet decoding + attention → admitted model rows
  → shared name measurement + per-row packing → viewport → styled view
existing key handlers → captured reference → existing client/gateway operation
```

`sessionui` alone owns this presentation. reuse `current`, `tableStatus`,
`rowMetadata`, `rowFooter`, `opensGroup`, `tail`, styles and exact-reference
selection. public api, wire/config/persistence schemas, gateway behavior,
provider parsing and observation cadence do not change.

keep existing `tableLayout`, `viewport`, `tableLines` and `fitViewports` method
signatures. reduce the private derived layout to body/name widths and one
heading flag per row; retain `tablePlacement`/`tableViewport`. session height is
one; fitting's entry-height check becomes `height < 1`. change private
`rowDirectory(context, width)` to return `(text string, fits bool)`;
`("", false)` means the field cannot fit, never an unavailable observation.
its only caller is in `layout.go`; add no stored layout state.

hard-cut stacking, 120/138 breakpoints, mandatory directory continuations,
ordinary inter-session gaps, variable session heights, metadata-presence
bookkeeping, global secondary-column widths, and profile-11/phrase-30 caps.
delete redundant types/branches after checking callers. keep helpers still
used by forms/info; extract only substantial local formatting complexity.

excluded: android/cli output changes, smaller-terminal support, horizontal
scrolling, density settings, new themes/colours, selected-row expansion,
new controls, status/provider grammar changes, identity changes, transport,
polling, persistence, dependencies, generic layout machinery and unrelated cleanup.
release/deployment is separate. [quick terminal](desktop-quick-terminal.md) owns
`n` and its pages; compose serially and verify actual footer height if that work
lands first. this plan does not change creation behavior.

## exclusive ownership and workflow

| role | exclusive write paths / responsibility |
| --- | --- |
| layout builder | `internal/sessionui/layout.go`: packing, bounded location/name rendering, hierarchy, viewport simplification and dead-code removal |
| root integrator | this spec; `docs/{architecture,roadmap,desktop-browser,desktop-browser-simplification,desktop-session-rows,design-language}.md`; `docs/issues/desktop-single-line-rows.md`; disposable `internal/sessionui/single_line_rows_test.go` and `single_line_rows_live_test.go` |
| content designer | no writes; defines content fixtures and evaluates actual hierarchy, recognition, omission and context |
| independent adversarial verifier | no writes; challenges requirements, width proof, test sensitivity, implementation, refactor and final evidence at every step |

one production owner; do not split the same layout file between builders.
freeze this contract before edits. root resolves scope decisions and preserves
unrelated working-tree changes. implementation was authorized on 2026-10-16.

1. **red:** root authors two disposable probes before production edits. designer
   fixes fixtures; verifier checks independent expected outcomes. baseline must
   compile and fail intended one-line geometry and secondary-location hierarchy.
   unchanged invariants may already pass; compilation failure is not red.
2. **green:** implement the single slice; run the same probes. designer inspects
   styled/plain renders; verifier challenges priority, bounds and exact targets.
3. **refactor:** remove replaced branches and demonstrated duplication, checking
   every caller. rerun affected probes; designer/verifier review final output/code.
4. **retire:** reap owned processes and clean recorded sessions/sockets before
   deleting their ownership records. delete probes/artifacts, align active docs,
   run `scripts/check verify host` and check local links. verifier reviews final
   diff, removal, source-attributed evidence and remaining issues.

## acceptance and evidence

| boundary | required result |
| --- | --- |
| rendered integration | strict loopback inventory decoding → existing model/events → `View`; 80/81/100/119/120/137/138/160 columns, height 24; all/needs-input/group/unassigned/empty; one session line, bounded cells, complete status, correct faint spans and equivalent plain labels under `NO_COLOR` |
| content/overflow | long names sharing a prefix with differing numeric suffixes; wide/combining text; all local/remote/unknown/absent/stale/checking contexts; present dash versus absent metadata; exact priority prefix; long intact model/profile; no smaller lower field rescued after a blocked field; marker reservation that removes another field when necessary; full info despite omitted fields |
| width edges | longest stale status plus 28-cell name; seven/eight remaining path cells; short root path; full absence/unknown phrase or whole-field omission; path elision alone creates no omission marker; normal short fields all fit without premature marker reservation |
| viewport/identity | first/middle/last selection, body capacities 0–5, intact group transitions, footer growth/shrinkage, tail backfill, resize across former breakpoints, offscreen long name, same lifetime after refresh and colliding displayed labels; full info and captured action reference agree with selection |
| live composition | actual browser/event loop → pty → production client → one loopback tls gateway → owned isolated tmux on darwin and linux at 80×24/160×24; terminal-only dummy sessions spanning a group boundary plus enough rows to scroll; inspect real grids, scroll/resize, space info, exact attachment, detach/first navigation, refresh and original cwd/lifetime survival |

reuse [the isolated composition](issues/desktop-browser-runtime-acceptance.md#session-row-qualification);
add no harness or production seam. agent observations belong in rendered
integration; provider/account/phone work is unnecessary. live work requires
execution-turn approval under [AGENTS.md](../AGENTS.md): only recorded owned
sessions on unique `-L` sockets, never the default socket or `kill-server`.
keep assertions in memory; evidence contains no terminal bytes, prompts,
credentials or account data. missing boundaries are `NOT_RUN`.

record source/platform, red/green outcomes, designer/verifier conclusions and
cleanup here and in the [row issue](issues/desktop-single-line-rows.md); unrelated
findings get separate records. installed acceptance needs separate deployment
and owner review; previous qualification does not transfer.

## deliberate costs

- strict priority can hide the entire machine/path and leave unused cells;
  full inspection then takes `space`. no lower field leapfrogs a hidden higher one.
- whole model/profile text may disappear where an ellipsized fragment once fit.
- fixed shared names spend space on short names; middle elision loses title
  interiors, and shortened names/paths can still collide.
- the eight-cell path floor trades partial location visibility for useful suffixes.
- secondary columns lose alignment; names/status starts and the right edge remain stable.
- faint is terminal-dependent and may disappear under mosh; plain geometry and
  literal statuses still carry meaning.
- deleted probes give change-specific evidence without retained regression coverage.

## source qualification

2026-10-16: branch `codex/desktop-single-line-rows`, base `28660ab3e455`;
sole production file `internal/sessionui/layout.go`, git blob
`ed84e6cd8b05adb5a8b1a0e827e26fdcaf36a65c`.

- disposable rendered probe: strict loopback tls decoding/context resolution →
  model/events → view passes all eight widths, five collections, styled/plain
  output, prefix/marker/path edges, unicode, full info, status width, viewport,
  footer changes and exact-lifetime selection/action capture.
- disposable live probe: browser/event loop → real pty → production client →
  one loopback tls gateway → owned tmux passes on darwin/arm64 and linux/arm64
  (debian container), at 80×24 and 160×24. 32 dummy terminals cross a group
  boundary and scroll; resize, info, exact attachment, detach/first navigation,
  rename-refresh, original cwd/lifetimes and browser/attachment retirement pass.
- both probes compile on the original layout and fail the intended multiline,
  hierarchy, density, elision and packing assertions. unchanged transport and
  lifetime journeys reach cleanup. the candidate passes the same expectations.
- content and independent adversarial review pass. review used literal in-memory
  renders/style assertions and source; no installed-theme human acceptance is
  inferred. refactor review found no additional abstraction or extraction justified.
- probes and owned artifacts are removed after exact-session/socket cleanup,
  browser exit and gateway retirement. isolated client/notification state keeps
  the real fleet, accounts and phone outside this qualification.
- `scripts/check verify host`, whitespace and local documentation links pass
  after probe deletion. engineering checks do not substitute for behavior.

linux terminal retirement is asynchronous: the live probe awaits actual client
absence within a deadline, separately from browser process exit. no production
change or relaxed cleanup criterion was needed. deployment and installed owner
review were `NOT_RUN` at source qualification; subsequent installed evidence follows.

2026-10-16 composition: rebased onto `525c62a` (quick-terminal chooser) without
changing the qualified layout blob. a supplemental disposable tls inventory →
model/events → view probe passes all eight widths, scroll/selection visibility,
actual footer height, chooser resize/cancel, full info and refresh identity.
the previous layout at `525c62a` compiles and fails its 80-column density assertion.
this is rendered composition evidence; the earlier real-pty runs retain their
original source attribution. the supplemental probe was removed after review.

## installed qualification

2026-10-16: immutable [v0.17.3](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.17.3)
ships `11f89da683502182832d80ad2aa4cf5265963409` ([implementation pr](https://github.com/NielsdaWheelz/skidbladnir/pull/92)).
exact-source [hosted verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/38974005495),
signed build, independently downloaded five-asset/signature checks and
`scripts/check published-release` pass. [upstream pin](https://github.com/NielsdaWheelz/skidbladnir/pull/93)
and [installer pin](https://github.com/NielsdaWheelz/dev-server/pull/175) match the published digests.
installer `6083362ebbea440756d1772698d5612bb6aef0db` applied through gateway-only
commands on macbook, arch and devbox; `scripts/fleet verify` passes all three.

before/after samples preserve all 11 macbook and three devbox session references,
selected panes, locally reported cwd and listed foreground-agent pid/start identities;
arch has no local sessions. gateway bearer, machine id and desktop config bytes/modes
are unchanged. native provider entrypoint bytes/versions and helper revision
`d3c57298131ef6c21232949b0a23a50fb9458cef` are unchanged. this covers sampled
identities, not every descendant process or provider-history content. jarvis remains
disabled, inactive, successfully stopped and paused; its managed cli matches the gateway.

newly launched installed browsers on darwin/arm64 and both linux/amd64 hosts pass
80×24/160×24 read-only real-pty smokes: table, quick chooser, resize in both directions,
cancel, info, return and clean quit. assertions inspect in-memory terminal grids;
no terminal bytes are retained. no creation, attachment or production tmux control
was part of this installed smoke; the earlier isolated journeys own those claims.

android updated in place from `0.17.2` / `17002` to `0.17.3` / `17003` after host
verification. the installed apk matches the published digest; app id and original
install timestamp are unchanged. the production activity opens the paired dashboard
with needs-input/all tabs. no pairing reset or phone terminal operation occurred.

independent release/deployment review passes. disposable probes and snapshots are
removed after owned browser exit. installed theme/readability acceptance still needs
the owner's 80×24/160×24 review, tracked in the [row issue](issues/desktop-single-line-rows.md).
