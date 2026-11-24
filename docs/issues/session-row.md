# Session row and the browser's size floor

Status: plan. The owner approved the design on 2026-11-24 in its lean form (every
row prints its own facts; headings carry none). Nothing is built in this
repository, no contract has changed and there is no build order. Terminal mock on
the live fleet: `python3 ~/Documents/code/row-redesign-scratch/show.py`. The
worked content is in `~/Documents/code/row-redesign-scratch/`:

- `plan/docs.md`: every contract sentence the change replaces, with its
  replacement.
- `spike/`: the design built in a copy of `ddc9e9f` (`tree`, four commits), its
  report `spike.md` and the test that proves it.
- `plan/review.md`: the adversarial review of this file. `notes/review.md` and
  `notes/feasibility.md`: the review and code audit behind the design.

Where they differ from this file, this file rules.

Problem: the browser's row spends its cells on repetition and cuts the one fact
that identifies a session. Below 80×24 it draws a size notice and nothing else.

Impact: 18 of 51 names are cut at every width while provider, machine and path
repeat down the table; at 80 columns machine and path are already gone; a split
pane or a narrow SSH window shows no sessions at all.

Evidence, at HEAD `ddc9e9f`, on the live fleet read with `skid list --json` on
2026-11-24 (51 sessions, 20 groups):

| Fact | Bits | Bits given the group | Note |
| --- | ---: | ---: | --- |
| Provider | 0.97 | 0.12 | Zero given the profile |
| Machine | 0.24 | 0.04 | 49 of 51 on one host |
| Directory | 2.95 | 0.04 | 22 rows share one |
| Profile | 1.86 | 0.33 | |
| Model/effort | 2.66 | 0.73 | 15 absent |

- Names: median 26 cells, 90th percentile 41, longest 48. The name column is
  the longest name capped at 28 (`internal/sessionui/layout.go:154`) and never
  shrinks (`:165-166`). Head-kept cuts are unique from 23 cells and both-ends
  cuts from 18; below 18 every rule collides.
- `tooSmall` (`layout.go:239`) guards three things: drawing (`view.go:119`), the
  layout after every message (`session.go:271`) and input (`keys.go:17,44,59`).
  Deleting it alone leaves eight panic sites (`notes/feasibility.md` a).
- The question fold is capped at five lines (`fold.go:216`) and half the body
  (`layout.go:82`), and `enter` sends whatever it shows (`keys.go:108-116`).
- The trial drew every window from 0×0 to 200×40 in 24 states and two
  palettes: 395,568 frames with no panic, no line wider than the window and no
  line too many. The same sweep on `ddc9e9f` misses the cursor's row at 5,943
  sizes in each state.

## Goal

A row that is a sign, a name with room, and the facts that differ, at every
window size. Looking has no floor. Acting has one: no key sends an effect the
window does not show.

## Non-goals

- No change to the wire, the host, the phone, the CLI's tables or the status
  projection. `skid list` keeps its columns.
- No new fact in a row: no status age, last activity, active command, objective
  or request kind. The open row's facts keep provider and the full path.
- No heading facts, running heading or per-row suppression of a repeated value.
- No reflow of the usage band, the key map or the facts. They are cut.
- No new fold, key or view. On the fixture at 80×24 and 120×40 every line but
  the session rows and the caption is today's (acceptance 18). Screenshots and
  media are not re-shot ([stale demo media](stale-demo-media.md)).

## Decisions

The owner's. Each is closed.

1. The sign keeps its one cell before the name (#232).
2. Lean: every row prints its own profile, model and place. A heading is a
   label and nothing else.
3. Provider leaves the row. Machine folds into place and is named only when the
   work runs off the browser's own machine.
4. Names take the room: the lane is the fleet's widest name, capped at 40, and
   later lanes are admitted only while names keep 32 cells.
5. Nothing in a row moves with a status or a footer.
6. The caption is one line and names its session first.
7. No floor to look. A floor to act: an effect key is refused, with the reason,
   when the window cannot show what it will do.
8. The request kind stays in the caption.

## Decided here

Mine, made while writing this file and answering its review. Each stands unless
the owner strikes it before the build.

1. Needs input admits place by the same rule as the other views. The mock
   omitted it.
2. A directory prints its last component, literally. The mock printed `~/` for
   a path shaped like a home, which is the client's guess: `/Users/Shared` would
   read as home. A home now reads `nnandal/`.
3. A stale row and a row without an agent print blanks, as today. The mock's
   `stale` word, `—` for agentless rows and a terminal's command were repairs
   for heading facts.
4. The profile lane is as wide as the widest configured profile, not the widest
   one in use. Measured from the sessions, as today, it would move every later
   lane when an agent exits or a host fails, against decision 5.
5. Place is the last lane, so once admitted it takes the rest of the line.
   At a fixed 24 cells, 30 of 51 directories are cut at every width.
6. The host in a place is cut to 12 cells and the directory keeps its tail. A
   missing directory off this machine keeps its `<host>:`.
7. A question loses its five-line cap and keeps the half-body cap of every
   inline fold. With five lines, closing needs 58 columns at any height.
8. A session created while the window shrank below attach's size is selected,
   not entered, and says why. Today it would fence every read and then fail.
9. A host notice keeps its host: the label is cut to 16 cells and the line
   loses its tail. Today the label gives way first, and no floor hid that.
10. Hints keep today's rule, so 80×24 shows what it shows now. Only the kept
    entries learn to leave.
11. One pull request with two commits: the row behind the existing floor, then
    the floor. The row alone cannot prove its lanes below 80 columns.

## Target behaviour

Columns are screen columns from 0. W and H are the window's columns and lines.
The design language counts from 1, so its column 5 is column 4 here.

### Spine

| Line | 0 | 1 | 2–3 | 4 on |
| --- | --- | --- | --- | --- |
| Session row | `▌` Gold on the cursor's row, else blank | sign | blank | name, then lanes |
| Fold line, unchanged | `▌` Gold | blank | blank | text |
| Group heading, unchanged | blank | label | | |
| Strip, notice, rule, band, hints, empty state, unchanged | blank | text | | |

Today a row has a one-cell gutter and a space after the lip: sign in column 3,
name in column 6. Without them the cursor's lip and name stand on the fold's lip
and text. From 4 columns every line leaves column W − 1 blank and the cursor
row's raise fills columns 0 to W − 2. Below 4 columns no line keeps a margin.

### Lanes

| Lane | Views | Width | Holds | Cut |
| --- | --- | --- | --- | --- |
| name | all | by admission | the session's name | by name mode |
| group | needs input | the fleet's widest heading, at most 16 | `fleetview.GroupHeading` | tail, `…` |
| profile | all | the widest configured profile key, at least `unknown`, at most 16 | the agent's profile, `unknown` when empty | tail, `…` |
| model | all | 20 | the footer verbatim, `—` when absent | tail, `…` |
| place | all | the rest of the line, at least 24 | see Place | the directory's head |

The fleet is every listed session of every peer, a failed host's rows included,
as `measureColumns` reads it today. Configured profiles are every peer's
`Profiles`, which a failed read keeps. So `all` and every group view share lane
starts, and a start moves only with the window, the view's kind, a longer or
shorter widest name or group, or the configured profiles, once as each host
first reports them. Two cells separate lanes. Lane text is Quiet.

| Row | group | profile | model | place |
| --- | --- | --- | --- | --- |
| Agent | heading | profile | footer or `—` | place |
| No agent | heading | blank | blank | place |
| Stale | blank | blank | blank | place |

### Admission

```text
avail  = W − 5                      spine 4, last column 1
target = min(40, widest name in the fleet)
keeps  = min(32, target)
for lane in [group,] profile, model, place:       place costs its least, 24
    need = target if lane is place else keeps
    stop unless avail − cost − (width + 2) ≥ need
    cost += width + 2
name lane = min(target, avail − cost) if a lane is admitted, else avail
starts    = 4 + name lane + 2, then each lane's width + 2
```

On the measured fleet (profile 12, group 16, target 40), in the trial:

| View | group | profile | model | place |
| --- | ---: | ---: | ---: | ---: |
| `all`, group | | 51 | 73 | 107 |
| needs input | 55 | 69 | 91 | 125 |

The name lane is 39 cells at 80 columns and 40 from 81.

### Names

An `automatic` name keeps its head and ends in `…`: its tail repeats the
directory. Any other name keeps both ends around one `…`, the head taking the
odd cell: its tail is a role. Under three cells of room either keeps its head. A
row draws a name from 8 columns; from 2 to 7 it is lip and sign; at 1 its sign.
Ink, bold and quiet are today's.

Of the 51 measured names 9 are cut at 80 columns and 6 from 81; today 18 are
cut at every width.

### Place

- The directory is the path's last component and a trailing `/`; `/` is root. A
  missing directory reads `directory unavailable`.
- `<host>:` precedes either when the execution context's machine handle is not
  the browser's own. The host is the context's label cut to 12 cells.
- When the cell is wider than its lane, what follows the host loses its head:
  `dev-server:…corpus-notes/`. The host is never cut further.
- An unresolved remote reads `remote context unknown` and names no host.

Place says where the work runs. `on <host>` in the caption says who lists it.

### Headings

Only `all` draws them, as today, and a group's first row shows under its
heading whenever the body has two lines. Two changes: a heading draws only from
14 columns (12 cells of label), and the blank line before each heading but the
first draws only from 24 lines.

### Caption

```text
caption  = subject "   " status ["   " fact]… ["   " position]
subject  = name " on " listing host
status   = label [" · checking"] [" · left the queue" | " · left this group"] [" · " detail]
fact     = group | profile | model | "in " place          in this order
position = n " of " N                                       while rows are hidden
```

A fact is present when its lane belongs to the view, is not admitted, and the
cursor's own cell would hold a value. `—` is not a value and `remote context
unknown` takes no `in`. Roles are today's; facts are Quiet.

When the caption is wider than its room these go, in order, until it fits:
position; the place's head, down to 12 cells with its `in`; the place; model;
profile; group; ` on <host>`; the name's tail, down to 6 cells; the name. Then
the status is cut from its tail, so the detail goes before `checking` and the
label last. The rule's dashes frame the caption from 12 cells of room and the
weekly quota takes what is left, as today. A foot draft's or terminal chooser's
caption is cut whole.

### No floor to look

The browser draws at every size, cuts what does not fit and never wraps a line.

| From W | Arrives |
| ---: | --- |
| 1 | each row's sign |
| 2 | the lip |
| 4 | the margin and the blank last column |
| 6 | the strip, as the needs-input sign |
| 8 | names; the first hint |
| 10 | the current view in the strip |
| 14 | group headings |
| 15 | the other tabs, `needs input` still its sign |
| 27 | `needs input` in words |
| 34 | the wordmark |
| 51 | profile |
| 73 | model |
| 80 | the usage band may show (71 without a Claude profile) |
| 107 | place |

The strip's widths are for `all` current on the measured fleet; a longer
current label moves them.

| From H | Arrives |
| ---: | --- |
| 1 | the cursor's row |
| 2 | the caption rule |
| 3 | the strip |
| 8 | the hint line |
| 12 | the blank line under the strip |
| 24 | the blank line before each heading |

Each line arrives where it costs the body nothing: the body is 1 line to H 3,
5 at 7 and 8, 8 at 11 and 12, 20 at 24. Below 6 columns the strip's line and
its blank go to the body. The hint line keeps its line when it is empty.

- **Cursor.** Its row never gives its line to chrome or a notice. A foot fold
  can cover it, as today, and a disabled fold takes a one-line body: it holds
  the keys.
- **Strip.** As width falls: group tabs shorten and hide as today; then the
  wordmark goes; then `needs input` becomes its sign; then only that sign and
  the current view, its label keeping 3 cells; then the sign alone.
- **Hints.** Today's rule, by rank while they fit. The kept entries are
  reserved first and leave from the front when they alone do not fit. An entry
  draws whole or not at all, and one a predicate refuses is not drawn.
- **Notices.** They take lines while the body keeps two, as today. With none
  to take, an outcome or refusal replaces the caption and the next key press
  clears it. A host notice with no line shows nowhere.
- **Usage band.** It hides below its grid's width as it hides below three
  lines, and keeps its state. The rule's quota returns while it hides.
- **Folds.** An inline fold takes at most `max(1, min(16, (body − 1) / 2))`
  lines and a foot fold 11/20 of the body, as today, and an inline fold never
  moves the cursor's row. In a one-line body it draws nothing. Fold text is cut
  to the window.

### A floor to act

One predicate per effect, over the fold it would show, checked by the key that
opens the fold and by every key that sends it.

| Effect | Opens | Sends | Needs |
| --- | --- | --- | --- |
| Question | `x`, `s` | `enter`, `y` | the whole question and every line of the effect within its cap; no word of the question wider than the fold's text |
| Rename | `r` | `enter`, `ctrl-s`, `ctrl-a` | its field: 1 line, 14 cells |
| Group | `g` | `enter`, `ctrl-s` | field and save: 2 lines, 23 cells |
| Seeded draft | `c` | `create` | five fields and create: 6 lines, 19 cells; to send, attach's size |
| New session | `N` | `create` | title, fields and create: 7 lines, 19 cells; to send, attach's size |
| Terminal chooser | `n` | `enter` | its 4 lines, 19 cells; to send, attach's size |
| Attach | | `enter`, `T`, an adopted creation | 20–1024 columns, 5–512 lines |
| Usage band | | `u` to show | its grid and a margin, 80 columns or 71; three free lines |
| Every other key | | | nothing |

Lines are of the fold's cap and cells of its text, W − 5. The counts are fixed
and leave out a fold's note, verdict or refusal line, so a notice cannot block
a save and a draft opens at 80×24 with the usage band showing. A field keeps 8
cells for its value. An inline fold needs a body of two lines to draw at all.

Least windows in the trial, columns × lines, no notice showing:

| Effect | Least window |
| --- | --- |
| Rename | 19×4 |
| Group | 28×7 |
| Terminal chooser | 24×11 |
| Seeded draft, new session | 24×17 |
| Attach | 20×5 |
| Usage band | 80×9 |
| Close, 26-cell name | 40×23, 80×13, 120×10 |
| Interrupt, 26-cell name | 40×17, 80×10, 120×7 |

A question's height falls as its width grows, because its effect must show
whole within half the body and closing's effect is 171 characters. At 24
columns closing needs 33 lines. A notice adds a line or two and a 64-cell name
up to five: closing one needs 80×17.

At 80×24 with the usage band showing, every effect is admitted with no notice
or one. With two, `c` and `N` are refused, where today they open clipped. With
the band hidden every effect is admitted under two notices.

- A refusal names the action and the reason, sends nothing and fences no read.
- A fold that stops fitting, by a shrink, a notice or the band, stays open and
  keeps its text. Its send is refused and says why; room rearms it.
- The focused text field of an editor or draft shows the tail of what was
  typed, behind a leading `…`. A value the window cuts elsewhere in a draft is
  sent whole, as at 80 columns today.
- A disabled fold arrives unbidden and cannot be refused. It is cut to the
  window and escape dismisses it.
- `space`, `i`, `?`, movement, view steps, typing, paste, closing keys,
  `ctrl-r` and quitting act at every size.

Copy, the browser's own:

- `window too small to confirm <action> of <name> on <host>; nothing was sent.`
- `window too small to rename <name> on <host>; nothing was sent.`
- `window too small to change the group of <name> on <host>; nothing was sent.`
- `window too small for the new session draft; nothing was sent.`
- `window too small for the terminal chooser; nothing was sent.`
- `attach refused: the window must be 20–1024 columns and 5–512 rows; nothing was sent.`
- `terminal created on <host>; attach refused: the window must be 20–1024 columns and 5–512 rows.`
- `window too small to show usage: the band needs <n> columns and three free lines.`

Each begins with what happened, because a small window shows only a refusal's
first cells: at 19×5 a refused attach reads `attach refused:`.

## Contract

| Concern | Owner | Where |
| --- | --- | --- |
| This machine's handle | `internal/fleetclient` | `config.go` |
| Lanes, admission, cuts, the ladders | `internal/sessionui` | `layout.go` |
| Lines drawn, the strip, the rule | `internal/sessionui` | `view.go` |
| Which key acts, and the floor to act | `internal/sessionui` | `keys.go`, each fold's `can` |
| Attach's size | `internal/terminal` | `ValidSize`, unchanged |

Symbols are the trial's, at `spike/tree`. They are a finding, not a
prescription.

### `internal/fleetclient`

`(*Client).Self() string` returns the handle `Adopt` already keeps in
`client.self` (`config.go:49`). The browser compares it with
`ExecutionContext.Machine`. Labels are never compared (invariant 2).

### `internal/sessionui`

- **Columns.** `providerColumn`, `machineColumn` and `pathColumn` go and
  `placeColumn` arrives; `rowNameStart` is 4. `measureColumns` takes the room
  after the spine, follows the admission rule and no longer returns early.
  `rowCells` drops provider. `executionPlace` and `shortPath` become `place`
  and `fitPlace`. `cutName` cuts by `Session.NameMode`.
- **Rows.** `tableLines` builds each row in screen columns and guards every
  pad. `View` no longer prefixes body lines; headings, the empty state and an
  empty body line carry their own margin. The trial dropped the last and
  acceptance 18 caught it.
- **Caption.** `tableCaption` builds the grammar and the order of sacrifice.
  `rule` clamps its dash run at zero. `target` stays for folds.
- **Chrome.** `frame` is split: `chrome` lays out the ladder, the notices, the
  band and a disabled fold and reads no key; `frame` adds the table, the fold
  and the hints. Every predicate reads `chrome`. It cannot read `frame`, whose
  hint line calls every key's `can` (`keys.go:320`), as the key map does
  (`:355`).
- **Predicates.** Each takes the fold it would show, since the opening key
  judges a fold not yet open, and runs after the key's existing guard, which
  is what proves a row is selected. The trial has two: `foldRoom(fold)`, one
  switch over the fold's kind giving lines, cells, cap and copy, and
  `attachRoom()`.
- **Floor.** `tooSmall` and the size notice go. `settle` lays out after every
  message. `onKey` and `onPaste` lose their size checks; `onKey` clears a
  notice that has no line. `adoptCreated` checks attach's size before `enter`.
- **Strip.** `header` runs `strip` with the wordmark, without it, and with
  `needs input` as its sign, then falls to the sign and the current view.
- **Folds.** The question's five-line cap goes. The focused field keeps its
  tail. `foldLines`, `keyMap`, `usageBand` and `draftLines` clamp their widths.
- **Help.** `internal/agentcli/run.go:124` stops naming 80x24.

The model is 100×30 until the window's size arrives (`session.go:106`); a key
in that instant is judged at that size, and `terminalclient` measures again. A
predicate judges the current size, not the last frame painted.

### Docs, in the same change

Every sentence and its replacement is in `plan/docs.md`: `docs/architecture.md`
§11 Browser (size, strip, table, rule, notices, folds, the question, keys),
Desktop usage, accepted limits and rationale; `docs/design-language.md` §17
(quiet's `provider`, `text begins in column 5`, `fit within 78 cells`);
`docs/install.md:117-118`; `docs/issues/unrun-acceptance.md`.

## Rules that bind the build

From [rules](../rules.md) and the
[desktop invariants](../architecture.md#desktop-invariants), where they bite:

- A rendered or shortened place is never an identity or an input (invariant 2).
- The host's words are verbatim: label, detail, footer. The copy above is the
  browser's own outcome.
- The eight panic sites become arithmetic that cannot go negative, not a
  `recover`.
- No option for the old row, no flag, no second layout.
- Only the root integrator edits `docs/architecture.md`, so the root builds
  this.

## Change

One pull request, two commits, one builder. Both rewrite the same functions.

| Commit | Paths | Delivers | Trial |
| --- | --- | --- | --- |
| The row behind the existing floor | `internal/fleetclient/config.go`, `internal/sessionui/{layout,view}.go`, `scripts/media/desktop.tape` | spine, lanes, admission, names, place, caption | +195 −147 |
| The floor | `internal/sessionui/**`, `internal/agentcli/run.go`, the docs | the ladders, the floor to act, `tooSmall` deleted | +401 −142 |

The first commit adds this file to the branch and the last removes it.

The trial took two more commits to reach this file's rules, +99 −111 and
+10 −23. It ended at +546 −264 in nine files against `ddc9e9f`: net +282, and
net +229 without comment lines. It left the help text, the tape and the docs
alone. My estimate to the owner was 150 to 250 lines. Only the net falls inside
it; the floor's predicates and their twelve call sites are the difference.

## Red, green, refactor

Per [testing](../rules.md#testing). A red is a failed assertion on a surface
both versions have: the string `View()` returns, or the notice a key leaves.

One file, `internal/sessionui/zz_row_test.go`, grown from the trial's
`spike_test.go`, which compiles against `ddc9e9f` and the change. It is in the
package because the surface is a frame for a given size and state, and the
sweep draws 395,568 of them in about two minutes. It needs no
production seam: `fleetclient.New` over a scratch directory, peers assigned to
the model, then `Update` with keys and sizes, `settle()` and `View()`. The
fleet is the 51 measured rows and seven edge rows: a 64-cell manual name, a
resolved remote, an unresolved remote, an agent with no directory, a plain
shell, and two retained rows of a failed host.

| Check | Asserts | Red on `ddc9e9f` |
| --- | --- | --- |
| row | acceptance 1 to 7 | provider in every row; names cut at 28; `macbook` on every row |
| sweep | acceptance 8 to 10, every size 0–200 × 0–40, 24 states, plain and full palette | no cursor row at 5,943 sizes a state; a line too many at 201; a line too wide at 63 |
| floor | acceptance 11 to 17 | 49 assertions: every refusal is silent and `space i j l ctrl-r ? q` do nothing |
| unchanged | acceptance 18 and 19 | green on both: it compares the change with frames written on `ddc9e9f` |

The trial ran every check but acceptance 19, whose rule it did not touch
(`view.go:150`).

Requests are counted at three `httptest` origins. This machine always routes to
the constant `127.0.0.1:7341`, the live gateway, so the test swaps
`http.DefaultTransport`, which `fleetclient.New` clones, for one that sends that
address to a test listener and refuses every other. A key's returned command
runs before the count is read, and an admitted send is the control. Attach's
own request runs inside `tea.Exec`, which no test reaches: a refused attach is
proved by its notice and by no row reading `checking`.

Refactor until the code meets [rules](../rules.md) "Code shape". Then delete
the test file.

## Acceptance

1. Every session row has the lip in column 0 on the cursor's row only, the sign
   in column 1 and the name from column 4. No row holds a provider.
2. On the fixture each lane first draws at the width in Admission and is absent
   one column narrower.
3. At each width, lane starts are equal in `all` and every group view. No lane
   start moves when a row's status or footer changes, when the agent with the
   widest profile exits, or when a host's read fails.
4. An automatic name's cut keeps its head; a manual one keeps both ends. Of the
   51 measured names 9 are cut at 80 columns and 6 at 120.
5. A place is `dir/`, or `host:dir/` only off this machine. A cut place keeps
   its host, and a host longer than 12 cells is cut to 12. A missing directory
   off this machine reads `host:directory unavailable`. Of the 51 measured
   places 21 are whole at 107 columns and all from 120.
6. A stale row holds no group, profile or model.
7. The caption matches the grammar and, as the window narrows from 200 to 4
   columns, loses its parts in the stated order.
8. No size panics. No line is wider than the window. A frame has at most H
   lines. From 4 columns the last column is blank.
9. With a session selected, its row is drawn at every size from 1×1, except
   under a foot fold or a disabled fold.
10. The lines arrive at heights 2, 3, 8, 12 and 24, and the parts at the widths
    in the table, and not one line or column earlier. A group's first row has
    its heading above it whenever the body has two lines.
11. Each refusal in the floor to act leaves its notice, word for word, and no
    request reaches a peer. After a refused attach no row reads `checking`.
12. A question opened where it fits refuses `enter` and stays open after a
    shrink, after `u` shows the band over it, and after a notice takes its
    lines. With room again it sends once.
13. A draft and a rename keep their text through a shrink, and the focused
    field shows the last cells typed.
14. `u` refuses to show a band the window would hide and hides one at any
    size. The band's state survives a shrink and a widen, and the rule shows
    the quota while the band hides.
15. Where notices have no line, a refusal draws on the caption's line and the
    next key press restores the caption.
16. A disabled fold is drawn at every size from one line and escape dismisses
    it.
17. A session created while the window is below 20×5 is selected and not
    entered, and the notice says why.
18. On the fixture at 80×24 and 120×40, in every state of the sweep, every
    line that is not a session row or the caption rule equals `ddc9e9f`'s. A
    session row is a line with a sign in its sign cell and a name two cells
    on; the rule is the line that begins with `─` after its margin. Opening
    `x`, `space`, `r` or `c` leaves the cursor's row on its line, as on
    `ddc9e9f`.
19. Under `NO_COLOR` the stream sets no style and every sign and word is
    present.
20. These find nothing in code or docs: `tooSmall`, `providerColumn`,
    `machineColumn`, `pathColumn`, `shortPath`, `80 × 24 minimum`.
21. `scripts/check verify host` passes.

`NOT_RUN`, recorded in [unrun acceptance](unrun-acceptance.md):

- The owner's look in Ghostty: the row on the fold's spine, the raise fill's
  edge, a split pane, a resize by hand, a light theme.
- The installed browser against the live fleet, and a real resize through
  Bubble Tea: the tests feed sizes to the model and read fixture peers.
- The media tape: its pattern is edited, the shot is not re-recorded.

## Deletions

`tooSmall`; the size notice; `providerColumn`, `machineColumn`, `pathColumn`;
`executionPlace`; `shortPath`; the ` (remote)` suffix; the name cap of 28; the
question's five-line cap; the three size checks in `keys.go`; the help text's
and the install guide's 80 × 24.

## Rollout

A Go client change with no wire, host or phone part. It ships by the
[release](../architecture.md#release) runbook and needs no lockstep. Status
signs are on main and unreleased; the release that carries this carries them,
so it is that [cutover](status-sign-cutover.md) unless one ships first.

## Trade-offs taken

- At wide windows a directory repeats down a group's rows.
- Names shorten when a lane is admitted. Widening from 50 to 51 columns cuts 12
  names where 1 was cut, and from 72 to 73 cuts names at 32 cells for 40. With
  no lane a name takes the whole line.
- Below about 18 cells of name, siblings collide; the caption has the name.
- Six of today's names lose part of their `-<directory>` tail at every width.
- The row no longer says that a session reaches its work over a connection.
- A home directory reads as its own name.
- Closing, interrupting, editing and creating are refused in a small window,
  and their keys leave the hints and go quiet in the key map without a reason.
  A narrow window must be tall to close a session, 23 lines at 40 columns: the
  question keeps the half-body cap so the cursor's row never moves. If a
  narrow, short window should close, the lever is the effect's 171 characters,
  not the cap.
- A refusal's notice takes body lines like any notice, so in a small window a
  refused key can scroll the table a line or three.
- At 80×24 with the usage band and two notices showing, `c` and `N` are
  refused until one of them goes. Today they open clipped.
- A small window cuts a refusal to its first cells, and an admitted editor or
  draft can clip its own note or verdict.
- The profile lane moves once as each host first reports its profiles.
- At one line a refusal shows nothing. At one column under `NO_COLOR` nothing
  marks the cursor. In a one-line body an open row or key map draws nothing and
  still takes its closing key.
- A caption narrower than its status cuts `checking` before the label.
- A draft's unfocused value can be cut by the window and is sent whole.
- A widen and an `enter` inside one repaint send what was last seen clipped.
- The key map, the facts and the usage band are cut, not reflowed.
- Every key's `can` runs for the hints, as today, and each predicate now lays
  out the chrome: about six more layouts a frame, a third of a millisecond a
  frame in the trial. The usage band's reveal repaints fifteen times a second.
- The sweep is an in-package test: it reads the frame, not a terminal.

## Declined

Majority norms on headings with per-row deviations; heading facts by
unanimity; any factoring of model (a footer blink rewrote other rows in 35 of
102 trials); a wrapping caption; dropping the name from the caption; headings
aligned with names; names running on through blank cells; a request-kind lane;
frequency-based name cuts; `~/` for a home; lane widths measured per view;
refusing `space` and `?` where their fold has no line; moving the cursor's row
to seat a taller question (the trial built it: it broke "without moving the
selected row" and pushed the row's heading off the top); a second pull request.

Resolved when: the pull request is merged with its docs, the acceptance list
holds on the merged code, and the owner has taken his look or left it recorded
in [unrun acceptance](unrun-acceptance.md).
