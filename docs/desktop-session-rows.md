# desktop session rows

2026-10-15: [browser simplification](desktop-browser-simplification.md) now owns
row/directory/footer presentation. it retains this revision's summary geometry,
group transitions and viewport mechanism, adds a directory to every row and
removes selected-footer details and remote-name qualifiers. the plan below and
its [qualification](#qualification) describe the preceding readability revision;
superseded clauses are historical, not alternative renderers. installed usability
remains pending under the current spec.

## scope and capability contract

make sections, sessions and continuations distinguishable in plain text at
80×24. preserve complete status, session recognition and truthful directory
context. [terminal model/effort](terminal-model-effort.md) owns observed text;
[desktop browser](desktop-browser.md) owns navigation/actions;
[session views](session-views.md) owns membership/order.

composition remains `admitted fleet observations → sessionui model → local
layout/viewport → View`. reuse `current`, `tableStatus`, `rowMetadata`,
`rowFooter`, `GroupHeading`, `SessionEqual` and existing sanitizing/styles.
no public api, wire/config/persistence schema, endpoint or model state changes.
tmux/providers retain their owners. names, shortened paths and row positions
never become control identities. refresh, queue order, keys, captured info and
pending targets retain their contracts.

## content and design ownership

| surface | content and order |
| --- | --- |
| all | existing group order; quiet `label:` heading; sessions below |
| named group / unassigned | tab supplies scope; no heading or membership cell |
| needs input | existing flat ready/action order; name/status before membership |
| inline record | `name  status  [membership]  provider  profile  observed label` |
| stacked record | `name  status`, then `[membership]  provider  profile  observed label` |

row designer defines good content as recognizable section/record/continuation/
selection without styling, with complete status explaining queue inclusion.
directory designer defines it as explicit local/remote/unknown and path/absence
distinctions, preserving the path suffix. both own dummy examples and review
actual renders. retain native spelling, profile precedence, unknown/dash
semantics, status tones and remote attribution from existing owners.

## deterministic geometry

body width `b = terminal width − 2`; columns have two-cell gaps. body column 0
holds headings/cursor, names start at 2, stacked metadata at 4. all/group uses
inline records at width ≥120; needs input at ≥138. otherwise stack. mode depends
only on width/view; retain the existing minimum-size screen below 80×24.

measure across every current row, never the visible slice. omit globally empty
columns and their gaps. no elastic filler; leave unused width on the right.

| field | cell budget |
| --- | --- |
| name | widest displayed name/remote qualifier, capped at 28 |
| status | widest complete `tableStatus`; never truncate |
| membership | needs input only: `3 + min(16, max(1, widest assigned label))` |
| provider | widest actual content; current closed bound 6 |
| profile | widest actual content, capped at 11 |
| observed label | widest label/absence dash, capped at 30 |

sanitize with `attention.SingleLine` before `ansi.StringWidth`/truncation;
style finished fragments. preserve existing remote-name allocation: desired
width `min(28, nameWidth + 4 + min(12, destinationWidth))`; after shared name
width `n`, destination budget `min(12, n − 4 − min(12, nameWidth))` and remaining
space belongs to the name. unknown remote reserves ≥6 cells each for provider
and profile and displays `remote unknown` across their span. remove the old
narrow needs-input name cap; membership no longer occupies the primary line.

membership renders `in ` plus the label truncated to its label budget. unassigned
leaves that entire cell blank, including the prefix; retain padding/gap for
alignment. omit the column when every row is unassigned. a named group literally
called `unassigned` remains named. headings use existing heading text, truncated
to `b−1`, then append `:`; preserve authored punctuation and add the colon only
in tui presentation. selected names alone are bold; selected content lines carry
`▌` in the same gutter. retain faint headings/metadata/unavailable names.

stack only semantically present metadata: a dash and `remote unknown` count;
column-padding whitespace does not. every record therefore has one or two
content lines. no empty continuation or selection bar on a separator/heading.

| worst occupied width, including margins | primary / metadata when stacked | inline |
| --- | ---: | ---: |
| all / group | 67 / 57 | 120 |
| needs input | 64 / 78 | 138 |

these bounds use status 33/30 respectively. vocabulary changes must recheck them.
stacked records have one blank between entries. inline records have one blank
at an all-view group boundary and no gap elsewhere. needs input never acquires headings or
inline inter-record gaps. never insert a gap
between heading and first entry, double a boundary gap, or retain viewport-edge
separators. dummy stacked examples:

```text
research:
  review-api  ready
    Codex  work  sample-model · high

▌ shell       terminal
```

```text
▌ review-api     ready
▌   in research  Codex   work  sample-model · high

  inspect-cache  needs input
                 Claude  work  sample-model · max
```

the second queue row above is unassigned; padding preserves metadata columns.

## composition and viewport contract

keep one ephemeral `tableLayout` owning column widths, mode and per-session
content height/group boundary. replace uniform-height and suffix-slicing
assumptions. one private viewport calculation takes layout, first session and
body capacity; it returns admitted session indexes, explicit gap/heading choices
and the exclusive end index. this is a local value, not a stored projection or
new framework. rendering, fitting/backfill and footer pagination consume that
same calculation; `view.go` must not reimplement prefix admission.

from a candidate first session, scan forward:

1. suppress its leading gap. if it opens an all-view group, include its heading whenever
   heading plus complete content fits; otherwise admit content alone if it fits.
   never repeat a heading when starting midway through a group.
2. each later entry requires its whole content and the gap required by its mode.
   an all-view group transition additionally requires its heading. admit that entire block or
   stop; never hide an interior transition to squeeze in a session.
3. stop at the first non-fitting entry; do not skip sessions or emit partial
   content. no trailing gap or dangling heading.

`top`/cursor count sessions. before fitting, clamp top to the valid session range
and no later than cursor. if selected content cannot fit, set top to cursor so
the shared plan is empty; retain selection, including in footer accounting.
otherwise preserve top while selection is visible and the tail is not reached.
when selection is below the viewport, use the earliest preceding top whose
forward plan includes it. when the tail is visible, backfill from the earliest
top whose plan retains the final session and selection. if all entries fit,
reset top to zero. use the same plan rather than the old backward-height credit.
footer `i of n` counts sessions and uses actual remaining body capacity.

## selected directory

keep its existing single footer line and action eligibility. separate real cwd,
absence and remote qualification before formatting; preserve full cwd/destination
in existing wrapped info. no filesystem resolution/normalization or interpretation
of paths embedded in provider footer text.

| context | rendering and width allocation |
| --- | --- |
| local | real cwd uses existing `tail`; empty cwd reads `directory unavailable` |
| available local with shell action | reserve measured `shift+t new shell here` width + 2; remaining cells belong to cwd/absence; right-align the action with at least two spaces |
| checking/unavailable local | same suffix policy, all `b` cells available; no action |
| known remote | `running on <destination>: <cwd or absence>`; head-truncate sanitized destination to 24 cells, retain framing, tail-truncate only cwd within remaining cells |
| unknown remote | exact `remote context unknown`; no cwd or shell action |

absence is prose, not a path. keep the existing wide-grapheme correction in
`tail`; never byte/rune-slice text. at 80 columns the actionable local path gets
54 cells, the known remote path ≥41; all absence copy fits.

## exclusive implementation boundaries

freeze the private viewport input/output contract before parallel edits.

| owner | exclusive write paths and responsibilities |
| --- | --- |
| layout builder | `internal/sessionui/layout.go`: measurement, row content, viewport calculation, fitting/backfill and rendering |
| composition/directory builder | `internal/sessionui/view.go`: only table-body composition, footer pagination, selected-directory rendering and directly necessary helper edits; consumes layout result |
| root integrator | this spec; affected browser/design/architecture/roadmap clauses; the two source issues and existing desktop acceptance issue; disposable `internal/sessionui/readable_rows_test.go` and `readable_rows_live_test.go`; orchestration and evidence |
| row and directory designers | read-only content definitions/examples and actual-render review for their feature |
| independent verifier | read-only adversarial review at each stage; no test or production writes |

layout and composition work may overlap after contract freeze; root alone writes
shared docs/probes. no other source paths are assigned. seek a scope decision if
evidence requires one. search all callers before deleting/replacing helpers.
reuse existing cell/sanitizing/status/context primitives. hard-cut the old
renderer, group-leading column, empty-continuation rule, 135 threshold,
conditional cwd policy and duplicated height arithmetic; keep no parallel path.

## red green refactor and acceptance

follow [testing policy](rules/testing.md). root authors the two disposable
probes; designers freeze output
requirements, verifier challenges oracles and boundary coverage before execution.

1. red: actual strict loopback decoding and `Update`/`View`, not a replica renderer.
   baseline must compile and fail the new hierarchy/path assertions. unchanged
   attachment invariants may pass baseline; do not manufacture a live failure.
2. green: builders implement only their assigned slice; run the same probes.
   both designers inspect actual output; verifier challenges boundary clipping,
   content truth and targeting. fix findings at their owner without weakening
   acceptance. run the isolated live boundary below.
3. refactor: remove superseded/duplicate paths after caller checks; review final
   composition and rerun affected probes. root then deletes probes and temporary
   artifacts, runs `scripts/check verify host`, and records content-free evidence.
   verifier checks the final diff and deletion. engineering checks prove no live
   behavior. source completion and installed acceptance are separate milestones.

| boundary | required evidence |
| --- | --- |
| rendered content | actual all/group/needs-input/empty views at 80/119/120/135/137/138/160 columns, height 24; inspect color and `NO_COLOR`; long names/groups/statuses, assigned/unassigned, known/missing labels, mixed terminal/agent/remote records; exact status, axes, whole selected bars, no overflow |
| viewport/identity | first/middle/last selection; body capacities 0–7, group boundaries and 1/2-line records; strict interior heading admission, optional top heading, no edge gaps; threshold/height resize and tail backfill; refresh/regroup/filter preserve surviving lifetime; captured info/pending/attach targets remain pinned |
| cwd | actual selected footer and info for fresh/checking/unavailable local, known/unknown remote, missing cwd, overlong destination, long/shared basenames and wide/combining graphemes; suffix/framing/absence/action eligibility/full values preserved |
| isolated runtime | real browser/event loop → loopback gateway → owned isolated tmux on darwin and linux at 80×24 and 160×24; navigate/info/attach/`ctrl-] d`/first navigation/refresh preserve selection, original cwd and both lifetimes; terminal-only fixtures suffice here, agent metadata/status combinations belong to decoded model/view fixtures |
| installed usability | after separate release/deployment, verify matching installed version; user reviews actual all/needs-input at narrow/wide sizes in their normal terminal, including group boundaries, two-line selection and long directories; explicitly confirm readable hierarchy and acceptable density |
| cleanup | no old presentation branch, changed schema/runtime owner or retained probe; both designer reviews and independent final review resolved; engineering checks pass |

live execution requires current-turn approval under [AGENTS.md](../AGENTS.md).
reuse the [isolated-browser composition](issues/desktop-browser-runtime-acceptance.md#session-row-qualification):
`sessions.New` with unique `SocketName`, `gateway.New`, temporary client/state/cwd/
credentials, and an owned browser-grid socket if needed. never use the gateway
cli's default socket or unrelated sessions/provider accounts/phone. bounded
cleanup closes terminals/tls, reaps children and removes recorded owned artifacts;
no `kill-server`. terminal assertions stay in memory; evidence stays content-free.
unexecuted boundaries are `NOT_RUN`. installed review does not authorize deployment.

record source/version, checks, review outcomes and cleanup below and in the
existing [desktop acceptance issue](issues/desktop-browser-runtime-acceptance.md).
keep [readability](issues/desktop-row-readability.md) and
[cwd truncation](issues/desktop-cwd-truncation.md) open until their source and
installed criteria pass; delete resolved records. no proof ledger/new test gate.

## costs and non-goals

fifteen body lines hold about five separated agent records instead of seven;
group headings and intact transitions may leave more spare lines. mixed heights
recover empty-terminal space but can shift rows on provider appearance/exit;
selection remains by lifetime. membership is less immediate than a leading
column; blank unassigned cells retain alignment. `in ` costs three cells and
moves the queue breakpoint to 138. fixed modes can stack otherwise fitting
content; inventory can change intrinsic widths, scrolling/selection cannot.

existing field caps still truncate even on wide screens. path suffixes can
collide after ancestor loss; full info resolves ambiguity. remote labels cap at
24 even on wide screens to reserve useful path space. no sticky heading means
mid-group scrolling can hide group context. deleting probes leaves no retained
automatic regression coverage. installed review adds a separate delivery step.

excluded: cards/backgrounds/icons, density settings, selected-row expansion,
new keys/sorting/filtering, footer redesign beyond cwd, phone/cli presentation,
provider observation/schema changes, pollers/state/dependencies, filesystem
inference, unrelated cleanup and release/deployment in this implementation.

## qualification

### readability revision

2026-10-15: source in this commit, from `aaf4691`, implements the revision on
`codex/desktop-row-readability`. the frozen strict tls/model/view probes compile
on the baseline and fail the intended hierarchy, viewport and cwd assertions.
the changed source passes:

- actual decoding and `Update`/`View` at 80/119/120/135/137/138/160 columns,
  color and `NO_COLOR`; all/group/needs-input/empty views, maximal status and
  field budgets, unicode, known/unknown remotes and assigned/unassigned labels.
- body capacities 0–9, mixed one/two-line entries, optional leading headings,
  intact interior transitions, selected gutters and footer counts. dense resize
  sequences preserve middle scrolling and maximally backfill the selected tail.
  regroup/filter/refresh keep surviving selection, captured info and pending refs.
- selected local/checking/unavailable/remote/unknown cwd, exact ascii suffixes,
  wide/combining endings, capped destination framing, absence, action eligibility
  and full info values.
- real browser/event loop → loopback gateway → owned isolated tmux on darwin
  and linux arm64 at 80×24 and 160×24. info, exact attachment, detach, first
  navigation, visible rename/refresh, cwd and both original lifetimes pass.
  unchanged darwin attachment invariants also pass on the baseline.

both designers approve actual dummy output and raw styles, including maximal
inline needs input at 138. independent adversarial review approves interfaces,
oracles, source and cleanup design; no further structural refactor is warranted.
both disposable repository probes, detached baseline worktree, render artifacts,
linux binary and owned container/image are removed. browser helpers exit, gateway
terminals close, and exact owned sessions are removed before fixture teardown.
`scripts/check verify host` passes after probe deletion; local documentation links
and diff checks pass. no new behavioral suite or production test seam remains.

release/deployment and matching installed usability are `NOT_RUN`. the two source
issues stay open for that separately agreed acceptance; no installed pass is
inferred from source or isolated runtime.

### preceding compact layout


2026-10-15: source `6974851` on `codex/terminal-model-effort`, from `16a6ed9`.
the compiling baseline fails compact row/footer assertions and the actual
80-column browser geometry. the final temporary probes pass:

- `Update`/`View` at 80/100/119/120/134/135/160 columns, 24 rows; all/group/
  needs-input/empty views, long and unicode names, dense groups/statuses,
  remote contexts, globally empty metadata and color/`NO_COLOR`. both designers
  reviewed 36 actual renders, including full info pages and bottom scrolling;
  explanatory prose wraps at words and native text stays exact.
- complete entries and heading admission with 0/1/2/3/4/5/7/10 available lines,
  every selected row and threshold resize. actual info scrolling reaches the
  native value's tail. regroup/refresh moves the selected row while preserving
  its lifetime; captured info and creation metadata keep their own contracts.
- actual event loop → loopback tls gateway → owned isolated tmux at 80×24 and
  160×24 on darwin and a linux container. info, exact selected attachment,
  `ctrl-] d`, first navigation, visible renamed-inventory refresh, clean exit,
  cwd and both original session lifetimes pass. real create/shell responses also
  pass the go client. gateway live terminals and all recorded sessions close.
- [footer qualification](terminal-model-effort.md#qualification): retained
  detector frames, strict go/actual android decoding and installed provider
  rendering, interactive changes and old-sample invalidation on both platforms.

independent adversarial review passes. replaced renderer/height assumptions and
all four disposable repository probes are removed; external probe sources,
renders and container/image are removed. retained replay and
`scripts/check verify all` pass after repository test deletion. the
[broader browser journeys](issues/desktop-browser-runtime-acceptance.md#remaining-browser-acceptance)
retain their existing scope; no release or installed-fleet deployment is claimed.

### layout follow-up

2026-10-15: source `78d66e1`, from merge `6c70ee0`, carries the two layout
corrections identified in `be996c5`. the preceding qualification missed long
accepted profile keys and maximal tail backfill after viewport growth.

the compiling merged base fails the core 24 profile and six backfill cases.
the corrected source passes 36 profile cases across all/group/needs-input,
80/119/120/134/135/160 columns and color/`NO_COLOR`, with full-key and
configured-label info precedence. seven backfill cases cover threshold and
height growth, named groups and selection above the visible tail; resized
output matches navigation at its final geometry. middle scrolling stays put,
and zero/one-line body capacity retains selection without partial entries.

independent adversarial review and design review of eleven actual dummy renders
pass. the temporary model integration probe and render artifacts are removed;
`scripts/check verify host` passes after deletion. transport and provider code
are unchanged; no new native/live run is claimed. the separately reported
[native cold-home startup failure](issues/codex-cold-home-startup.md) remains
outside these layout fixes, with no runtime workaround.
