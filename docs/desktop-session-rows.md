# desktop session rows

accepted and implemented 2026-10-14. [scoped qualification](issues/desktop-browser-runtime-acceptance.md#session-row-qualification)
passes for the row/info change on darwin; disposable probes are removed.
the owner selected rows only; [model/effort observation](issues/session-model-effort.md)
is deferred.

## scope and final behavior

make sessions easier to scan and give names the recovered width. replace the
desktop table's combined agent, machine and directory columns with separate
provider/profile cells and correct info's missing-label profile wording.
the owner explicitly included that consistency fix. [desktop browser](desktop-browser.md)
retains navigation, selection, footer, other info, confirmations and attachment; [terminal
continuity](terminal-continuity.md) retains execution-context meaning.

| view | row order and group context |
| --- | --- |
| `all` | `name  status  provider  profile`; existing heading per group, including `unassigned` |
| named group / unassigned | same four columns; selected tab supplies group, no repeated heading |
| `needs input` | `group  name  status  provider  profile`; unassigned cells blank; omit group column only when every row is unassigned |

retain one line per session, existing heading/viewport accounting, ordering and
two-cell cursor gutter. add no column-header row or blank lines. no repeated
terminal-owner or cwd column. the selected footer retains terminal owner,
directory and remote destination; scrolling info retains complete values. the
machine filter and captured confirmation still identify the terminal owner.

## content contract and fit

the content/interaction designer owns this section's recommendations and reviews
the actual renders. good content preserves exact facts and uncertainty, makes
name/status dominant, and states remote execution explicitly. the root owns
the approved wording here. external spelling/case stays intact.

| fact / condition | rendered content and source |
| --- | --- |
| name | actual `row.session.Name`, sanitized and right-truncated; no alternate identifier |
| status | existing `m.tableStatus(row)` text and tone, whole; preserve checking, last-observed wording and queue menu explanations |
| provider | `m.current(&row).Agent.Provider`, exactly |
| profile | nonempty current `Agent.Profile` key, exactly; `unknown` only when an agent exists without a profile |
| no detected agent | blank provider/profile; status still explains terminal/unknown/unavailable evidence |
| known remote | current remote provider/profile; append ` on <destination>` inside the name column |
| unknown remote | `remote unknown` across the otherwise-empty provider/profile span; no remembered destination, provider or profile |

profile keys remain usable when the configured display label is absent. never
split `Agent.Label`, consult the source machine's profile catalogue, or substitute
`LaunchProfile`. info's profile fact shows the configured `Agent.Label` when
present, otherwise the known `Agent.Profile` key, otherwise `profile unknown`.
this replaces the existing false-unknown claim when only the label is absent;
it introduces no alternative identity source. `m.current` already
converts unavailable/checking remote rows to unknown; reuse it. local stale
metadata remains explicitly last-observed through the existing status/freshness
presentation. a remote provider does not imply remote activity: keep the existing
source-terminal status projection and action target.

keep existing styling: names plain, unavailable names faint, selected name bold
with its cursor; metadata/group/remote qualifier faint. the qualifier remains
separate from bold name text. status keeps its existing tone. `NO_COLOR` preserves
words and markers. sanitize with `attention.SingleLine` before measuring; use
`ansi.StringWidth`, `ansi.Truncate` and `cell`, then style finished fragments.
never sanitize composed ansi or change canonical names/groups to fit.

one calculation in the existing renderer owns widths across **all current
rows**, not just the visible slice:

1. body width is terminal width minus two outer cells. reserve the two-cell
   gutter and two cells between columns.
2. reserve provider `6` and profile `11` cells, even for plain terminals. these
   fit [architecture's closed production profile table](architecture.md#2-fixed-contract),
   including `unknown`; generic string types do not establish this bound.
3. status gets the widest complete `tableStatus` label in the current rows.
4. names receive the remainder, without the old 24-cell cap. the needs-input
   group column exists when any canonical group is assigned; its width is
   `min(16, max(1, longestGroupWidth), availableGroupAndNameSpace - 12)`.
   the final term is measured after its additional separator. this reserves
   twelve name cells without repairing invisible labels.
5. never drop/truncate provider, profile or status to fit. the current admitted
   vocabulary fits at the existing 80×24 minimum; any later vocabulary change
   must recheck this budget. preserve the existing below-minimum behavior.

for a known remote row with name-column width `n`, cap the destination at
`min(12, n - 16)` cells, including ellipsis. reserve ` on ` (four cells) and that
rendered destination; the remaining name budget is at least twelve cells. this
qualifier is presentation only. unknown remote uses fourteen of the nineteen
metadata cells and consumes no name space. complete destination remains in
scrolling info; the existing footer can also truncate.

| 80-column scene | status width | group width | name-column width |
| --- | ---: | ---: | ---: |
| all/group, ordinary maximum | 18 | absent | 35 |
| all/group, stale maximum | 33 | absent | 20 |
| needs input, longest menu notice and long group | 30 | 9 | 12 |
| known remote beside the stale-maximum row | 33 | absent | 20 = 12 name + 4 prefix + 4 destination |

scoped qualification verifies these budgets. remote rows remain excluded
from needs input. width depends on admitted rows and may change after refresh;
cursor movement/scrolling alone must not change it.

## composition, interfaces and invariants

```text
existing gateway inventory -> fleetclient decoding/context resolution
  -> sessionui model admission/rebuild -> current + tableStatus -> tableLines
  -> existing selected footer/info and captured terminal actions
```

no public/internal service api, wire dto, persisted schema, polling lane or
configuration changes. the table above is a presentation mapping over existing
`listedRow`, `Session` and `ExecutionContext`, not a new runtime schema. keep
derivation local to the renderer; no stored row projection, column registry or
parallel profile table. reuse `opensGroup`, `current`, `tableStatus`, `cell`,
existing styles and viewport accounting.

selection and actions retain exact session references and `SessionEqual`; names,
truncated hosts and row positions never target operations. remote destination
never substitutes for terminal owner. inventory admission, failed-host notices,
pending-operation guards and attention acknowledgement remain unchanged. info
keeps its captured lifetime through rename/regroup. no collision-specific machine
suffix or new status copy is introduced.

## files, ownership and hard cut

| owner | exclusive write scope / responsibility |
| --- | --- |
| content/interaction designer | read-only content/fit proposals and render review; root writes this contract |
| presentation builder | `internal/sessionui/view.go`: row layout/formatting; `internal/sessionui/details.go`: only profile-fact wording in `details` |
| root integrator / verification author | owning docs listed below; disposable `internal/sessionui/session_rows_test.go` and `internal/sessionui/session_rows_live_test.go`; run checks and remove probes |
| independent verifier | reads specification, test sensitivity, diff and results; writes no test or production file |

one production builder is sufficient: the change has one cohesive owner. design
and verification review can run independently; nobody edits the builder's file.
root owns this document, `desktop-browser.md`, `design-language.md`,
`terminal-continuity.md`, `architecture.md`, `roadmap.md`, and
`issues/{desktop-browser-runtime-acceptance,session-model-effort}.md`,
all under `docs/`. widen ownership only for a demonstrated requirement,
before edits, not as incidental cleanup. the separate terminal-usage plan also
assigns `view.go`: serialize those desktop builders; the second integration must
rebase and recheck composed 80×24 geometry. there is no functional dependency.

replace the old layout in place. delete table-only `agentText` and `whereText`
after caller searches, the combined-label path, machine/cwd column sizing, the
24-cell name cap, widest-first shrinking and whole-column dropping. replace info's
label-missing/unknown branch with the stated fact precedence. retain `tail`
for the selected footer and all still-used primitives. no old/new branch, density
setting, compatibility reader or new dependency. do not generalize small local
formatting into a shared layout framework.

## red, green, refactor and acceptance

follow [testing policy](rules/testing.md). the completed workflow below used
frozen authored dummy fixtures: names up to 64 cells, long/unicode groups, two
identical names on different peer identities, and local/remote sessions sharing
an account. probes passed unchanged after the production edit, then were removed.

1. **red:** root writes the disposable probes against the unchanged renderer.
   use actual fleetclient decoding, model `Update`/`View` and existing message
   paths; assert rendered facts and selected/captured targets, not a second
   renderer or private helper wiring. demonstrate the intended failure of the
   old combined/where columns, name cap and info's false unknown. verifier
   challenges sensitivity and confirms the failure is the requested behavior.
2. **green:** the builder changes only its owned presentation. rerun the same probes
   and inspect actual authored renders. content designer checks wording/hierarchy;
   verifier checks fit, unknown/stale semantics and exact targets.
3. **refactor:** remove obsolete paths and unnecessary local duplication; rerun
   affected probes after meaningful changes. delete both temporary test files and
   all fixture artifacts; run `scripts/check verify host`. verifier checks the
   final diff and cleanup. static/build success is not behavioral acceptance.

| acceptance | required evidence |
| --- | --- |
| content and fit | actual `View` at 80×24 and widths 100/120, in color and `NO_COLOR`: normal agents, missing profile, missing display label with known key, plain terminal, observation failure, long/unicode text, longest menu notice, stale/checking; exact provider/profile/status, one row per session, no overflow, name uses recovered width; info shows label/key/unknown by the stated precedence |
| remote | known remote with long destination beside a 33-cell stale row; at least 12 name cells, destination qualifier, complete provider/profile; unknown and stale/checking remote have `remote unknown` with no old remote facts; source owner remains in footer/confirmation |
| grouping and continuity | all headings, no selected-group heading, group first in needs input, mixed/unassigned-only queue, empty selected group; scroll/resize preserve selected lifetime and row order; duplicate names reveal their owners on selection; pending confirmation retains its original reference after refresh |
| real terminal boundary | real browser/event loop -> real loopback gateway -> isolated tmux: list two authored plain terminals, navigate, open info, attach/detach with `ctrl-] d`, use the first navigation key, refresh and retain selection/cwd; both terminals survive. inspect the browser's rendered grid at 80×24 and wider |
| deletion and engineering | no row machine/cwd path or combined-agent helper remains; footer still uses its existing helpers; probes removed; scoped engineering checks pass |

the first three rows use disposable tls peers and the real client/model, including
remote-context resolution; they do not qualify provider discovery. for the live
row, compose existing `sessions.New(Config{SocketName: ...})`, its ordinary
agent-control service and `gateway.New` under a loopback tls server. seed two
plain `/bin/sh` commands, bypassing startup files/integrations, in temporary cwds.
set only those targets to the required `window-size latest`,
`destroy-unattached off`, and `detach-on-destroy on`.

run a disposable test-process entry into `sessionui.Run` in an owned browser pane
on a **second** unique tmux socket. disable its status bar and use actual pane
dimensions so 80×24 reaches the browser. inspect tmux's rendered grid only in
assertion memory. stripping ansi from an incremental pty stream is not screen
evidence; build no emulator. do not use the gateway cli for socket isolation:
it intentionally selects the default server. no provider launch, terminal-exec
test shim or new production seam is needed.

name the probes `TestSessionRows` and `TestSessionRowsLive`. run separately with
`go test ./internal/sessionui -run '^TestSessionRows$' -count=1` and, only after
execution-turn approval, `go test ./internal/sessionui -run '^TestSessionRowsLive$' -count=1 -timeout=60s`.
the live probe's disposable child entry shares its fixture setup; it is not a
production command. unchanged continuity checks pass before and after; only the
new row/info contract must supply the intended red failure.

all probe state, checkpoints, auth material and `XDG_STATE_HOME` use temporary
directories; never repurpose `HOME`. route every fixture request to owned loopback
peers and refuse any other destination. use fixed empty/unavailable quota replies
for whichever usage source has been integrated; do not inspect real accounts.
loopback tls/dial setup belongs only to the serial temporary probe and is restored
in each process. cleanup cancels/reaps the browser, calls
`gateway.CloseLiveTerminals` with a fresh bounded context, closes the tls server
and removes only recorded owned sessions on their sockets. server close alone
does not close hijacked websockets; no `kill-server` or broad process cleanup.
retain only credential/content-free outcomes,
cell bounds and source revision; no terminal bytes or live account data in logs.

tmux/live execution requires explicit approval in its execution turn under
`AGENTS.md`; planning approval alone grants none. qualification used execution-turn
approval. every tmux call uses the probe's
explicit owned `-L` socket; touch only sessions/panes it created. run the real terminal
smoke on the implementation host; record its os. this presentation-only change does
not require a second platform or phone run and cannot close their existing gaps.
missing boundaries are `NOT_RUN`, never passes. record scoped results in
[desktop runtime acceptance](issues/desktop-browser-runtime-acceptance.md); keep
unrelated open acceptance there intact.
the profile-label issue passed its focused acceptance and is deleted; the
model/effort follow-up remains deferred.

## costs and exclusions

fixed metadata columns leave unused room on plain terminals. headings consume
one row per group. machine/cwd comparisons require selection or info; identical
unselected rows are possible. exact profile keys repeat provider in `claude-work`.
remote qualifiers reduce visible name space and can truncate hosts to four cells;
neither name nor host truncation guarantees uniqueness. deleting probes leaves
no retained regression protection. these costs are accepted for the simpler row.

excluded: model/effort, provider/native observation changes, status vocabulary,
mobile/cli output, footer redesign, new keys, sort/filter changes, configuration,
persistence, collision disambiguation, icons, telemetry, generic columns, retained
test infrastructure, release/deployment, and unrelated cleanup. the existing
[invisible group-label issue](issues/group-label-invisible-characters.md) remains
with its owner; this renderer does not rewrite labels to hide it.
