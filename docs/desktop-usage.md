# desktop usage presentation

2026-10-20: [claude usage selection](claude-usage-selection.md) removes usage
conflict from the contract below; its [isolated qualification](claude-usage-selection.md#qualification)
passes. earlier qualification retains its recorded source.

2026-10-15: implemented and qualified at the isolated desktop boundary. one
remaining-capacity convention, compact profile records, exceptional-state labels
and automatic updates. this replaces the presentation, freshness cutoff and usage-page keys
in [terminal usage](terminal-usage.md) and [native usage](profile-usage.md).
acquisition, source selection and provider limitations retain those owners.
[qualification](#qualification) records source, evidence and limits.

## scope and capability contract

desktop footer, `u` page and browser help only. quota remains read-only evidence;
it never supplies readiness, attention, admission or session-control authority.
the same account per profile across hosts remains an owner-supplied invariant.

no public api, wire/config/persistence schema, provider formatter, parser,
gateway, android or deployment change. retain strict `GET /v1/profile-usage`
and `Session.terminalUsage` shapes, including raw used percentages, optional
windows and native resets. no new endpoint, cache, timer, dependency, provider
read, source picker, quota enforcement or account discovery.

## composition and internal representation

```text
existing inventory lane (5s) -> admitted claude terminal evidence
existing native lane (60s)   -> retained codex account reports
  -> existing whole-observation selection -> usageRows(now)
  -> common remaining/window projection -> footer and page
```

keep lane isolation, receipt timestamps, cancellation/generations, access
rejection, descriptor invalidation, profile declarations and source tie breaks.
convert percentages after selection. disagreement never excludes claude usage;
no splicing, health preference, new ranking or freshness-based
candidate rejection. a newer partial report replaces the whole older report.

reuse `usageRow` and reshape its existing `usageText` locally:

```text
usageText       = {fiveHour?: windowText, sevenDay?: windowText, stale: bool}
windowText      = {remaining: string, resetsAt: string, resetsIn: string,
                   expired: bool}
usageRow.state  = "" | "checking" | "unavailable"
```

these are ephemeral presentation values, never another retained model. go
pointers express optional windows. nil means the source omitted that window or
no selected/diagnostic source exists; a present expired window remains present.
strings are output only;
never parse them to make decisions. absent/expired cells render `—`.
remove the old age string and combined reset sentence. rename the existing
`usageWindowText` function if that name becomes the private record type.
keep source identity/times for selection even though their prose disappears.

## percentage and time rules

- codex: `floor(max(0, 100-usedPercent))`, then `%`; use original precision.
- claude: `(100-displayedUsedPercent)` + `%`, including zero and 100.
  the formatter floors used values, so the complement can overstate remaining
  by less than one percentage point.
- one named stale threshold is exactly one hour, inclusive. native age remains
  `observedAt-reportedAt + max(0, now-receivedAt)`; latest host/profile read
  failure also makes a retained eligible native report stale immediately.
- claude capture age is `max(0, now-receivedAt)`. pass the same `now` into
  `claudeUsage`. pending reads retain the receipt; successful captures replace
  it even when values are unchanged. failures/no-match/replacement
  withdraw eligibility as before; add no claude retention cache.
- claude capture age is not provider age. recently captured old cells may look
  unstale. no unchanged-value heuristic or implied provider-freshness guarantee.
- native reset distance remains `resetsAt-observedAt-elapsed`, using the host's
  timestamps and client monotonic elapsed time. never subtract desktop wall
  time or compare host clocks. datetime is `yyyy-mm-dd hh:mm:ss` in utc.
- for positive distance, ceil to whole seconds, then format `dd:hh:mm:ss`:
  days have at least two digits and no artificial cap; hours/minutes/seconds
  are zero-padded, with hours below 24. expiry is tested before rounding.
- at reset equality or later, clear that window's percentage and reset cells;
  retain its `expired` fact for `awaiting update`. never infer replenishment.
  missing resets remain `—`; claude supplies no resets.
- retain the five-second render tick, minute native cadence and immediate
  disclosure read. seconds advance in steps; no render-only timer is added.

## content and geometry

the quota content designer owns numbers, exceptional-state copy and dummy
examples; the interaction designer owns page chrome, hints and help. good
content preserves meaning without colour, distinguishes absence from stale
evidence, keeps whole values and units together, and explains only exceptions.
both designers review actual renders before acceptance.

always render the primary `7d` row, including unknown weekly values. render a
subordinate `5h` row iff the selected eligible or diagnostic report contains that
window, even when expired. blank its profile cell. preserve configured order/keys.

per-window status precedence:

1. nonempty `usageRow.state`;
2. otherwise, a known expired window: `awaiting update`;
3. otherwise, a present unexpired percentage from a stale source: `stale`;
4. otherwise, blank. nil windows never inherit `stale`.

an eligible source clears row state and suppresses other-host failures.
explicitly clear claude's initial checking state after selection; the old
renderer merely ignored it. without a candidate, retain existing precedence:
unavailable, successful absence, then checking; native failure
over expired-report diagnosis. successful absence is blank state plus dashes.
an expired diagnostic native report can still supply its actual optional windows.
retain fleet-empty/checking/declaration-unavailable distinctions from terminal
usage; do not fabricate profile rows.

illustrative data; spacing follows the budgets below:

```text
usage

profile      window                    reset (utc)          in (d:hh:mm:ss)
personal     7d   72%                  2026-10-20 14:30:00  05:04:18:09
             5h   93%                  2026-10-15 14:30:00  00:04:18:09
work         7d   61% stale            2026-10-18 09:00:00  02:22:48:09
work2        7d     — awaiting update  —                    —
claude-work  7d   57%                  —                    —
```

body width is terminal width minus two. columns have two-cell gaps: profile
11 cells; window/value/status 24; reset 19; countdown uses the remaining width.
inside the 24-cell group: window 2, space 1, percentage right-aligned 5, space 1,
status 15. the closed host profile table bounds keys at 11 cells; reuse that
contract, without a new consumer validator or arbitrary-profile layout. sanitize
with `attention.SingleLine`, measure with `ansi.StringWidth`, style last. the
complete header and longest status fit within 78 cells. no number, qualifier,
datetime or countdown truncation. profile blocks are adjacent, with no blank rows.

page title is only `usage`. hide the table's inactive view strip on this page,
preserving the view for return. [browser simplification](desktop-browser-simplification.md)
removes machine collection scope throughout the browser. retain the wordmark,
existing notices and unlabelled footer rule. delete source/terminal/pane/age
lines, repeated used/remaining prefixes, absence sentences, `all machines`,
`read only` and the routine provider-freshness warning.

footer items are weekly-only: `<profile> 7d <value> [status]`. use the same
projection/status rules as the page; no provider headings. separate items with
` · ` and wrap only between complete items. all four closed profiles fit within
two 78-cell lines, including exceptional states. no ellipsis or hidden profiles.
reuse existing width helpers; remove final whole-line truncation. actual footer
height continues to determine table capacity and selected-row visibility.

usage keys are up/down by one content line and `q`/escape back; `ctrl-c` retains
global exit semantics. ignore `j`, `k`, page-up/down and `ctrl-r` on this page.
dispatch usage keys before the global refresh branch. retain all table, info and
editor controls. the sole usage hint is `q escape back`.
title, blank and column heading consume three fixed body rows. content capacity
is available body height minus three, floored at zero; rendering and arrow bounds
use that same value. clamp the existing offset after content or size changes.
no new viewport owner. return preserves table view,
selection and scroll under existing reconciliation. minimum size remains 80×24.

## non-overlapping work

freeze this contract before implementation. assign each path to one writer;
read-only designers/reviewers can advise every slice.

| owner | exclusive files and responsibility |
| --- | --- |
| root integrator | this file; `docs/{terminal-usage,profile-usage,desktop-browser,architecture,roadmap}.md`; `docs/issues/desktop-usage.md`; contract, integration and evidence |
| quota implementer with quota designer | `internal/sessionui/usage.go`; projection, formatting, page/footer records and usage keys |
| browser implementer with interaction designer | `internal/sessionui/{session,view}.go`, `internal/agentcli/run.go`; dispatch, chrome, capacity, help and return integration |
| probe author | temporary `internal/sessionui/usage_presentation_test.go`, `internal/sessionui/usage_browser_test.go`, owned scratch directory; red/green probes and deletion |
| adversarial verifier | no writes; challenge contract, probes, implementation, renders and final cleanup |

quota and browser owners agree on content-line count/fixed-heading capacity
before parallel edits. reuse `usageRows`, selectors, `usageWindowAvailable`,
existing clock math, `wrapped`, `cell`, `keyLines` and current viewport ownership
where their contracts fit. extract only the small shared window/status formatting
needed by both renderers; no generic presenter, observer or table framework.

## red green refactor and acceptance

source acceptance ends at an isolated running desktop browser. deployment and
installed-fleet smoke are separate, as approved. the implementation followed
these sequential checkpoints:

1. **contract:** designers supply the state examples; verifier challenges
   arithmetic, truthfulness, geometry and scope before probes are written.
2. **red:** probe author freezes baseline-compatible assertions and runs them
   against baseline `b253c05`. changed behavior must fail for its intended reason;
   unchanged invariants may pass. verifier rejects compilation/setup failures
   and insensitive oracles as evidence.
3. **green:** implement the two desktop slices, then integrate. run the same
   probes and inspect rendered output. designers review content; verifier reviews
   changed behavior, ownership and unintended effects before refactoring.
4. **refactor:** remove replaced prose builders, unused presentation fields,
   provider-specific rendering and usage-only key branches. retain source facts
   and table refresh. no feature flags, old layout path, compatibility or fallback.
   rerun affected probes after changes; verifier reviews the final diff.
5. **retire:** probe author deletes temporary files/artifacts; root confirms no
   production test seams or dependencies remain and runs `scripts/check verify host`.
   record exact source, behavioral outcomes, engineering checks and unrun limits
   here; close the issue only when the scoped criteria pass.

| boundary | required evidence |
| --- | --- |
| projection and clocks | native fractional/zero/100/over-100 used; claude zero/ordinary/100 approximation; exact one-hour boundary; immediate failed-read stale; claude receipt aging without retention; nil windows, partial replacement, failed/omitted observations; reset equality and minute/hour/day/subsecond carries |
| shared content | footer/page agree on weekly remaining and state; 5h-only report leaves weekly unknown; reported expired 5h remains a continuation; valid claude never says checking; expired/no-source failure precedence and no stale dash |
| live desktop | scripted replies over real loopback tls through production `fleetclient` into `sessionui.Run`, actual pty keyboard input and rendered terminal grid; 80×24 and wider, colour and `NO_COLOR`; every required value, heading and state intact |
| interaction | table → usage → table preserves view/selection/scroll; view chrome absent only on usage; arrows move, removed keys do not; usage `ctrl-r` emits no refresh; table refresh survives; no mutation requests |
| automatic reads | changed claude fixture appears on inventory tick; changed codex fixture appears after an actual native interval without keypress; opening disclosure reads immediately; no new cadence or lane coupling |

use synthetic data, fixture certificates and private temporary config with
ordinary `fleetclient.Open`. existing pty/terminal facilities suffice; no
production transport/clock seam. exact clock edges use existing explicit-time
model projections, not hour-long sleeps. test-local transport setup is isolated,
restored and nonparallel. qualify the full live journey on darwin and the short
render/key journey on linux; do not repeat every arithmetic case per platform.

prove sensitivity: a known value must render before absence assertions; changed
fixture data must visibly change it. the model probe uses eight window rows and
existing wrapped notices to force overflow at an interior offset; arrows are
positive controls for ignored-key checks. the final continuation stays reachable,
headings stay fixed, and offset clamps when notices clear or the viewport grows.
exercise production `Update`/`View`; add no `Run` seam or invented profiles to
force live scrolling. distinguish scheduled
inventory reads from key-triggered reads; check native request counts before its
next deadline. fail unexpected methods/routes. inspect the rendered grid, not an
undecoded stream of ansi updates.

[testing policy](rules/testing.md) governs deletion. [AGENTS.md](../AGENTS.md)
requires current-turn approval for any tmux/live execution; planning approval
does not grant it. an emulator using tmux may touch only its exact test-created
session/pane on an isolated `-L` socket. no phone, stock-provider turns or user
sessions. evidence stays content-free. missing runtime is `NOT_RUN`, never a
pass. these checks qualify live desktop/pty with scripted peers, not actual
gateway/provider/fleet acquisition. no retained behavioral suite remains.

## accepted costs and final state

claude loses sub-percent precision and has unknown provider age/resets; missing
resets render as dashes. one hour relaxes the prior two-minute age
warning; native read failures immediately mark retained eligible quota stale.
five-hour continuations cost up to
four rows; they retain a distinct limit. inferred remaining semantics reduce
first-use explanation. hidden sources remove on-page diagnostics. five-second
countdown steps and no manual usage reread are accepted. whole-source replacement
can discard older complete windows. temporary tests leave no automatic regression
protection after deletion; isolated acceptance makes no installed-fleet claim.

final state: one desktop projection supplies consistent remaining values, short
exception labels and day-based countdowns to both surfaces. existing acquisition,
ownership, schemas and automatic reads remain. obsolete presentation paths and
temporary probes are removed; deployment remains separate.

## qualification

2026-10-15; production source `1cb2186697716a2632a7f9f1981c571def4776b1`, based on
`b253c05f8374d4db625a83c8494e9f7317022d23`. the following documentation change
leaves the verified production files unchanged.

- red: frozen probes ran against the unchanged baseline; 31 target cases failed
  for the intended behavior, while six unchanged cases passed. the full darwin
  run exercised actual tls, pty grids and the native minute without setup failures.
- green: all projection, clock, shared-state and browser probes passed on
  darwin/arm64 in 74.616s. the live journey covered 80×24 and 110×24, each with
  colour and `NO_COLOR`, actual five-second terminal updates and the native minute.
  linux/amd64 passed the four short render/key journeys in 6.21s under local
  docker emulation; the minute wait was not repeated there.
- assertions checked exact cells, whole footer items, unknown/stale/expired
  precedence, partial replacement, overflow, nondefault-view return, ignored
  keys, request counts and absence of mutation routes. the added tiny-resize
  probe passed at widths 0/1/79, restoration, escape and global quit.
- quota and interaction designers reviewed the actual-render oracles and passing
  outcomes; independent adversarial review approved probes, source and scope.
  no further refactor was warranted. both temporary test files, owned scratch,
  test binaries and container images were removed. no production seam or new
  dependency remains. `scripts/check verify host` passed after test retirement.

this qualifies live desktop/pty with scripted peers. gateway/provider acquisition,
installed-fleet smoke and deployment were not run for this revision. temporary
probe retirement leaves no automatic behavioral regression protection.
