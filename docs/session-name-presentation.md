# session name presentation

2026-10-19: implemented; [isolated qualification](#qualification) passes.
this contract owns desktop name overflow
and interrupt/close target disclosure, superseding those clauses in
[single-line rows](desktop-single-line-rows.md) and
[desktop browser](desktop-browser.md). their other contracts remain authoritative.
[architecture](architecture.md), [rules](rules.md) and
[testing policy](rules.md#testing) govern implementation.

## outcome and scope

preserve readable openings in the list; disclose the complete captured target
at confirmation while keeping the action and consequences visible.

`sessionui` owns the correction. canonical tmux names, handles, references,
availability/dispatch checks, ordinary cli output, android, paths, provider
behavior, observation and persistence retain their contracts. no public api,
wire/config schema, model field, dependency or service is added. names remain
labels; only the captured reference targets an operation.

## list names

retain collection-wide measurement and the 28-display-cell cap. sanitize with
`attention.SingleLine`, apply `ansi.Truncate(name, layout.name, "…")`, then
style and pad as today. a fitting name stays complete; overflow preserves the
longest fitting grapheme prefix plus `…`. never slice bytes/runes to measure
terminal width. preserve one physical row, status, ordering and exact selection.
external tmux names are not constrained by skid's 64-character input grammar.
keep suffix-preserving `tail` for paths and full names in info.

## confirmation content and interaction

one content designer owns both features' authored fixtures and final review.
good content preserves external spelling after existing display sanitization,
reveals distinguishing suffixes on inspection, names literal effects and makes
no stopping/completion promise. styled and `NO_COLOR` output carry the same facts.

| region | exact content |
| --- | --- |
| pinned body | existing danger-style title: `send interrupt?` or `interrupt and close terminal?`; blank line; wrapped effect below; blank line |
| scrolling facts | `session name`, `terminal host`, `selected pane`, in that order; complete captured values, using existing `field` wrapping; no renderer-added ellipsis |
| pinned footer | existing abbreviated captured-target rule and notices; unconditional `↑↓ scroll`, `pgup pgdn page`, existing enter/action and escape/cancel hints |

interrupt effect: `send one interrupt to the selected pane; retain this session. stopping is unconfirmed.`

close effect: `send one interrupt to the selected pane, then close this entire session. closure proceeds even if interruption fails. work shared elsewhere or running remotely may continue.`

use existing `pendingName`, `pendingLabel` and `pending.Ref`; derive pane from
that reference. sanitize values before wrapping/concatenation. refresh, rename,
reordering and pane changes never replace these captured values or the request.
retain current session-lifetime availability admission and exact dispatch;
the gateway continues to validate the captured pane. unavailable/replaced
lifetimes cannot dispatch. do not substitute a refreshed row's reference.

| input/state | contract |
| --- | --- |
| opening | existing `s`/`x` capture; reset shared `m.offset` to zero |
| navigation | up/down or k/j move one fact line; pgup/pgdown move one actual fact viewport; clamp at both ends |
| confirmation | enter/y retains existing admission and dispatch at every offset; no read-to-end gate |
| cancellation | escape/q/n retains existing cancellation; reopening starts at zero |
| resize/notice/refresh | rewrap and clamp the line offset using current geometry; captured facts remain frozen |
| undersized/in flight | retain existing 80×24 minimum and input refusal/cancellation behavior |

## composition and private interface

```text
existing inventory → model → shared name width → end-elided list
selected exact row → existing pending capture → confirmation content
  → pinned consequences + fact viewport → existing execute/cancel
```

freeze this private interface before splitting work:

```go
func (m *model) confirmationContent() (fixed, facts []string)
```

it derives wrapped/styled rows from captured values and current width; it never
reads offset/footer or performs effects. reuse `field`, `wrapped`, styles and
`m.offset`; do not store derived content or introduce a layout framework.

extend `pageCapacity()` for confirmation:
`max(0, m.height - 2 - len(m.footerLines()) - len(fixed))`.
the two rows are the existing header and blank row. rendering, key movement
and clamping use this same fact capacity. scroll hints stay unconditional so
`footerLines → hints` never calls back into capacity calculation.

in the existing `Update` defer, for supported confirmation geometry, clamp
offset after message-specific updates and before `fitViewports` to
`[0, max(0, len(facts)-capacity)]`. render with a locally bounded offset; clip
fixed content to the allocated body before appending the fact slice. zero
capacity renders no facts and never panics; positive capacity makes every fact
line reachable. retain line-index position across reflow, without text anchors.
arbitrary footer exhaustion is outside this correction; do not claim it solved.

delete the sole-use `capturedHeading` after replacing its caller. retain
`target` for abbreviated footer legends. preserve info/usage behavior.

## exclusive work boundaries

| owner | files / responsibility |
| --- | --- |
| list implementer | `internal/sessionui/layout.go`: name rendering only |
| confirmation renderer | `internal/sessionui/view.go`: content, capacity, viewport, hints; delete `capturedHeading` |
| confirmation controller | `internal/sessionui/session.go`: open/reset, scroll keys, deferred clamp; preserve dispatch |
| root integrator / acceptance author | temporary `internal/sessionui/name_display_integration_test.go` and `name_display_live_test.go`; docs, composition and cleanup |
| content designer / independent verifier | read-only review; no test or production writes |

renderer/controller consume the frozen interface; integrate before acceptance.
only root changes shared specifications/indexes. keep user changes intact.

## red → green → refactor → delete

1. designer authors dummy cases and literal expected text independently of
   production formatting helpers before implementation;
   independently challenge recognizability, suffix disclosure and effect copy.
2. root freezes two disposable probes using existing boundaries below. run both
   against the original source; require compilation and failures for the intended
   end-ellipsis/full-disclosure behavior, never missing helpers or setup errors.
3. implement the three disjoint slices; independent adversarial review checks
   ownership, frozen targets, geometry, content and test sensitivity. pass the
   unchanged probes; refactor and rerun only for changed behavior/remaining concerns.
4. review final renders and refactor, delete probes and owned artifacts, run
   `scripts/check verify host`, and check whitespace/local links. record concise
   source/platform/results/limits here. engineering checks prove no behavior.

| boundary | required acceptance |
| --- | --- |
| rendered integration | real loopback tls response → production fleet decoding/client → existing model/key events → `View`; widths 80/160, height 24; literal expected end ellipsis for 27/28/29-cell and long names, prefix twins, wide/combining text and literal `…`; complete info; unchanged one-row geometry/status/paths and plain/styled meaning |
| confirmation integration | both actions; full action/effect pinned whenever the allocated body fits fixed content; at positive fact capacity, full late suffix/host/pane reachable across several viewports; arrows/page keys, prior nonzero info offset, cancel/reopen, footer growth/shrink, resize including undersized/restored; insufficient/zero capacity clips safely and restores reachability when space returns; no overrun where existing header/footer fit |
| captured dispatch | test-only loopback effect receiver observes original machine header, session path, identity token and pane after refresh/rename/reorder/pane change; cancel/scroll send nothing; unavailable/replaced lifetime sends nothing; no production seam |
| isolated live | actual browser/event loop → real pty → production client → loopback production gateway → owned isolated tmux, on darwin and linux at 80×24/160×24; dummy terminals only; list, full info, both confirmations, paging, resize, cancel and clean quit; sessions/lifetimes remain intact until owned cleanup |

reuse [the isolated composition](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/issues/desktop-browser-runtime-acceptance.md#session-row-qualification).
use owned tmux as the browser grid renderer on a second unique `-L` socket;
add no emulator/dependency or retained harness. choose a live dummy name long
enough to require paging at both widths. live confirmation is cancel-only;
the receiver qualifies exact dispatch, not real interruption/closure effects.
no provider, account, installed-fleet or phone acceptance is claimed.

any tmux/live execution needs explicit user approval in that execution turn
under [AGENTS.md](../AGENTS.md). never use the
default socket, `kill-server`, or another session/pane. keep grids/assertion
values in memory; evidence uses content-free case outcomes. retire only owned
sessions/processes/artifacts, verifying exit before removing inert sockets.
missing/unexecuted boundaries are `NOT_RUN`, never passes.

## final state, costs and non-goals

hard-cut middle elision and abbreviated confirmation-body names; retain no mode,
compatibility path or fallback. full inspection and the footer keep their roles.
exclude adaptive name widths, naming changes, aliases, suffix badges, marquees,
selected-row expansion, read gates, new controls beyond scrolling, generic page
refactors, provider/gateway/android changes, release and deployment.

accepted costs: end ellipsis hides distinguishing suffixes in the list; the
28-cell cap still truncates on wide screens. pinned consequences reduce fact
space; long names can place host/pane several pages away. resize may shift the
visible text through reflow. zero fact capacity requires more terminal space.
deleted tests provide change-specific evidence without retained regressions.

the confirmation issue is resolved and removed. the
[row issue](issues/unrun-acceptance.md) retains the separate installed
owner readability review.

## qualification

2026-10-19: source based on `0c252894ead739c0342dff3aa568516eeab85de1`;
production changes are confined to `layout.go`, `view.go` and `session.go`.

- frozen disposable integration and live probes compile against the original
  three files through a toolchain source overlay and fail the intended name
  elision, complete disclosure, reset and scrolling assertions. the candidate
  passes the same probes on darwin/arm64 (go 1.26.5, tmux 3.8) and linux/arm64
  (same go compiler, debian 13 container, tmux 3.5a).
- the integration matrix above passes, including literal independent unicode
  fixtures, full info/fact reconstruction, pinned effects, zero-capacity
  recovery, resize/footer clamping, cancellation and exact captured dispatch
  after rename/reorder/pane change. unavailable/replaced lifetimes refuse it.
- actual browser/pty → client → real loopback gateway → owned tmux passes
  both 80×24 and 160×24, full info, both confirmations, exact captured host/pane,
  paging, resize in both directions while confirmation stays open,
  cancel/reopen/reset, undersized/restored geometry and clean exit. original
  session references, names and directories survive; no stop/close request or
  native provider invocation occurs. real interruption/closure effects and
  installed release/owner acceptance are outside this evidence.
- independent adversarial architecture, content and probe-sensitivity reviews
  pass. refactor review found no additional extraction or machinery justified;
  the obsolete helper is removed and captured dispatch is unchanged.
- temporary probes and owned runtime artifacts are removed after exact-session
  cleanup and browser/gateway exit. `scripts/check verify host`, whitespace and
  local documentation-link checks pass after probe deletion.
