# desktop session rows

2026-10-15: source implemented and [qualified](#qualification) at `6974851`.
supersedes the 2026-10-14 width rules, whose [qualification](issues/desktop-browser-runtime-acceptance.md#session-row-qualification)
remains historical. [terminal model and effort](terminal-model-effort.md) owns
observation and transport; this document owns presentation and delivery.

## outcome and content

one borderless list. keep identity, status and related metadata close together;
leave unused width on the right. cards spend space without adding meaning to
this keyboard roster. retain ordering, keys, footer, forms, captured info lifetime
and exact session-reference actions. no headers or density setting.

| view | order and grouping |
| --- | --- |
| all | `name  status  provider  profile  observed label`; existing group headings, including unassigned |
| named group / unassigned | same columns; tab supplies group; no repeated heading |
| needs input | optional group column first; unassigned cells blank; omit only when every row is unassigned |

the layout designer owns hierarchy, spacing and rendered examples; the
observation designer owns label/absence copy in the linked contract. good
content preserves native spelling and uncertainty, keeps name/status dominant,
and makes every continuation visibly belong to its session. both review actual
renders before acceptance; neither changes product scope.

use `current`, `tableStatus` and existing profile precedence: provider/profile
keys with the width truncation below; `unknown` only for an agent without a
profile; plain terminals leave metadata blank. info uses configured profile
label, then known key, then `profile unknown`. known remote retains
`name on destination`; unknown remote
uses `remote unknown` across the provider/profile span. remote context never
becomes source-terminal activity or an action target.

group-heading text and names share the axis after the two-cell cursor gutter.
the selected entry's first line alone carries the cursor; its name alone is
bold. preserve current status tones, faint metadata/remote qualifiers and
unavailable names. `NO_COLOR` preserves words, cursor and spacing. sanitize with
`attention.SingleLine` before cell measurement; reuse `ansi.StringWidth`,
`ansi.Truncate` and `cell`, styling only finished fragments.

## deterministic geometry

body width is terminal width minus two outer cells. gaps are two cells.

| view | one line per session | otherwise |
| --- | --- | --- |
| all / named group / unassigned | terminal width ≥120 | two lines |
| needs input | terminal width ≥135 | two lines |

mode depends only on width and view. below 80×24, keep existing minimum-size
behavior. in two-line mode, every session uses two lines, including terminals
with an empty second line; metadata starts at the name axis:

```text
[group]  name  status
         provider  profile  observed label
```

measure across **all current rows**, never the visible slice. omit globally
empty metadata columns and their gaps. cursor/scroll cannot change widths;
admitted inventory may. use these content widths, with no elastic filler:

| column | width |
| --- | --- |
| group | `min(16, max(1, widest sanitized group))`, when present |
| name | widest required displayed name/remote qualifier, capped at 28 |
| status | widest complete existing `tableStatus`; never truncate |
| provider | widest actual content; closed contract bound 6 |
| profile | widest actual content, capped at 11; longer accepted keys truncate |
| observed label | widest label or absence dash, capped at 30; truncate once, preserving full value in info |

if any row has unknown remote context, reserve at least six cells each for
provider/profile: their combined span is at least fourteen, enough for `remote unknown`.
in narrow needs input, also cap name at
`body − 2 − (group + 2 if present) − status − 2`.
future status/provider vocabulary changes must recheck these budgets.

remote desired width is `min(28, actualNameWidth + 4 + min(12, destinationWidth))`.
after shared name width `n` is known, destination budget is
`min(12, n − 4 − min(12, actualNameWidth))`; remaining cells after ` on ` and
the rendered destination belong to the name. long remote names keep twelve
cells; short names create no artificial minimum gap. full destination stays
in info. remote rows remain excluded from needs input.

worst occupied widths, including outer margins, use status 33 in all/group,
status 30 and group 16 in needs input, provider 6, profile 11 and label 30:

| terminal | all/group | needs input |
| ---: | --- | --- |
| 80 | two lines: 67 / 55 | two lines: 80 / 73; name cap 26 |
| 100 | two lines: 67 / 55 | two lines: 82 / 73 |
| 120 | one line: 120 | two lines: 82 / 73 |
| 160 | one line: 120 | one line: 135 |

## composition and scrolling

derive one local layout from view, dimensions and rows; reuse its entry height
and row spans for total height, `fitViewports`, rendering and footer scroll
indication. no stored row projection, column registry or general layout engine.
reuse existing group, context, style, sanitizing and session-identity primitives.

retain selection by lifetime through resize/refresh/regrouping while the session
remains in the current view; otherwise keep the existing fallback. captured info
and operations retain `SessionEqual` targeting independently. render whole
entries, never an orphan metadata line or a heading without an entry. include
an immediately preceding heading when it and its first entry fit; otherwise show
the complete entry without a heading. add no sticky/repeated headings. when the
final entry fits, backfill complete entries from above into newly available
space while retaining selection. leave remaining spare space blank.
when body capacity is below entry height, leave it blank and
retain selection. `i of n` counts sessions, excluding headings and continuation lines.

## exclusive implementation boundaries

all builders read both feature contracts first. transport follows the frozen
observation contract; presentation follows transport. independent file work may
overlap in time; edits never overlap in ownership.

| owner | exclusive paths and responsibility |
| --- | --- |
| observation builder | `internal/sessions/{types,usage,footer}.go` (`footer.go` new); `internal/agentcontrol/{detect,terminal,codex,claude,claude_usage}.go`, optional new `internal/agentcontrol/footer.go`; existing `frames_test.go` and `testdata/frames/`; `internal/gateway/dto.go`: value, extraction, attachment validation and host serialization |
| client builder | `internal/fleetclient/{response,client}.go`; `android/app/src/main/java/dev/niels/skidbladnir/ProductModel.kt`: strict ingress, projection, list/create/shell parity, explained-inspection clearing; no phone visuals |
| presentation builder | `internal/sessionui/{view,details,session}.go`, optional new `internal/sessionui/layout.go`: row layout, info, all height accounting and clearing creation metadata before insertion into older inventory; the new file only if it absorbs shared calculation |
| root integrator | owning feature docs, provider grammar docs, architecture/roadmap/browser/design links and existing acceptance issues; disposable probes below, orchestration and cleanup |
| two feature designers | read-only content/interaction review of their respective feature and actual output |
| independent verifier | read-only challenge of specification, red sensitivity, boundaries, results and final diff; no test or production writes |

replace the existing fill-the-width layout in place. remove its surplus name
padding, independent one-line height assumptions and superseded calculations.
search callers before deleting helpers; retain still-used footer and context
primitives. no compatibility branch, old renderer, feature flag, dependency,
profile catalogue copy or unrelated cleanup. serialize any concurrent work in
these files; only root edits shared scope documents.

## red green refactor and acceptance

follow [testing policy](rules/testing.md). root authors disposable
`session_rows_test.go` / `session_rows_live_test.go` in `internal/sessionui/`,
`terminal_footer_integration_test.go` in `internal/gateway/`, and disposable
`android/app/src/test/java/dev/niels/skidbladnir/TerminalFooterDecodeProbe.kt`.
run its ordinary jvm entry point against the actual decoder with existing gradle
compile/runtime dependencies; no junit/framework dependency or permanent task.
observation builder
extends the retained dummy frame corpus/replay instead of duplicating it.

1. **red:** freeze target assertions first. use actual decoding, `Update`/`View`
   and gateway composition. baseline must compile and fail the requested
   behavior; absent new symbols are not a red result. use raw json to probe the
   new field against baseline. designers approve content; verifier challenges
   misleading positives, missing boundaries and test sensitivity.
2. **green:** implement each exclusive slice; run the same assertions and
   inspect actual renders. qualify source extraction separately from transport,
   layout and live providers. do not weaken assertions to fit the code.
3. **refactor:** consolidate only demonstrated duplication, remove replaced
   paths and rerun affected checks. verifier reviews final composition. delete
   disposable probes/artifacts; retain the explicitly excepted provider corpus
   and replay. run `go test ./internal/agentcontrol` and `scripts/check verify all`.

| boundary | acceptance |
| --- | --- |
| row content | actual views at widths 80/100/119/120/134/135/160, 24 rows, color and `NO_COLOR`; correct axes, exact statuses/keys, known/missing/stale labels, long/unicode names, remote contexts, no overflow; unused width remains right |
| viewport and targeting | all/group/needs-input/empty views; long groups and worst statuses; footer growth down to 0/1 body lines; top/bottom scrolling, final entry, resize across thresholds; no partial entry/dangling heading; selection retains admitted lifetime or existing fallback; pending/info/attach targets stay pinned |
| observation and transport | linked capability acceptance: retained dummy frames plus actual gateway → go and actual android decoder, including list/create/shell and failure paths; valid phrases survive exactly, invalid evidence never reaches presentation |
| live browser | actual event loop → loopback gateway → owned isolated tmux; grid at 80×24 and wide; navigate/info/attach/`ctrl-] d`/first navigation key/refresh preserve selection, cwd and both terminal lifetimes |
| live providers | installed codex and claude on darwin/linux: two same-account sessions with different settings, interactive changes, hidden/custom/clipped footers and process replacement; content-free receipts distinguish exercised provider/version/host from omissions |
| hard cut and cleanup | no old layout/schema path, changed polling or mutation behavior; disposable probes removed; retained replay and engineering checks pass; designer and verifier findings resolved |

live tests require execution-turn approval under `AGENTS.md`. compose existing
`sessions.New(Config{SocketName: ...})` and `gateway.New` against unique owned
`-L` sockets; the gateway cli uses the default socket and is unsuitable. use a
second owned socket for the browser grid, temporary state/cwds and only owned
sessions. inspect terminal bytes in assertion memory, never logs. cleanup reaps
children, closes gateway live terminals with a fresh bounded context, closes tls
and removes only recorded sessions; browser probes never use `kill-server`.
the existing frame generator retains its own isolated-socket cleanup. no
production session or phone operations. provider/account access and each execution host must be in
the approved live scope. missing boundaries are `NOT_RUN`, never passes.

record source/version, outcomes and cleanup in the existing
[desktop acceptance](issues/desktop-browser-runtime-acceptance.md) and
[capability qualification](terminal-model-effort.md#qualification). delete the
resolved source issue. do not recreate gates or a proof ledger. prior
qualification and engineering checks prove no new live behavior.
release/deployment remain separate.

## costs and non goals

narrow views show fewer sessions; plain terminals retain an empty continuation.
fixed thresholds can use two lines even when current content would fit one.
long names, profile keys and observed labels truncate even on wide screens;
info retains full values under its existing profile label/key precedence.
columns can shift after inventory changes, never while scrolling.
terminal evidence has the limitations in its contract; strict schema deployment
requires coordinated consumers. deleting probes leaves no lasting integration
regression protection beyond the retained detector corpus.

excluded: cards, footer redesign, new keys, sorting/filtering, phone presentation,
cli text changes, effective per-inference settings, native getters, new hooks or
pollers, persistence, configuration, remote model attribution, release/deployment
and unrelated cleanup. the [invisible group-label issue](issues/group-label-invisible-characters.md)
keeps its existing owner.

## qualification

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
