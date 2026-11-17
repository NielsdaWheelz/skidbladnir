# the fold: browser cutover

status: approved by the owner on 2026-11-17 as one hard cutover. no code yet. this
file is the contract and the plan for branch `feat/browser-fold` and lives only
there: its first commit adds it, its last deletes it after moving what must outlive
it into [architecture §11](../architecture.md#11-desktop-clients) and
[design language §17](../design-language.md#17-terminal-browser). main never
carries it. until the merge those two describe what is shipped.

problem: the browser replaces the table to show anything about a session, asks its
destructive question a screen away from the row it will end, prints a status
inferred from a screen it never shows, and
[moves the cursor on return](return-queue-shuffle.md).

evidence: a keyboard-driven prototype on a demo fleet
(https://claude.ai/artifact/QsDMr6hyvREkh2fr5jDP59, claude-work account), a bubble
tea port of it, two adversarial reviews of the design and two of this file, and the
terminal mechanics read in source. nothing here has run in the real browser.

## rulings

owner, 2026-11-17. each closes a branch of the design; only the owner reopens one.

1. folds, not pages or sheets, with one hint line. no ornament, no motion.
2. one hard cutover: one branch, one PR, one release. no old path, flag or fallback
   survives the merge.
3. in: the return repair, the ground, the table, folds, `c`, `?`, the screen read.
   out: `a` (acknowledge), the served request subject, `/` find.
4. ghostty in truecolor on a known ground gets the full design. every other case
   gets one plain path: today's inks.
5. on return every row keeps its place and reads `checking` until its host answers;
   a ready row until the hub answers.
6. a `checking` row admits attach and nothing else.
7. the cursor's row stays as a guest however it leaves its view, the owner's own
   close and regroup included.
8. two inks, plain and quiet. a third is judged in ghostty from the port and, if
   kept, lands after this cutover.
9. proof is temporary tests, written before each slice's code, kept outside the
   repo across the slices and deleted before the PR opens; then the owner's hands.
10. two of those tests may read style (is a line filled; its contrast): an exception
    to `docs/rules.md`'s "never styling" for this branch only.
11. the earlier sheets proposal is retired; its record is archived outside the repo.

## goals and non-goals

- the table never leaves: detail opens under its row or rises from the foot.
- the cursor stays on the session the owner chose until a key moves it.
- the browser shows its evidence, the screen, beside its inference, the status.
- one key table drives dispatch, hints and the key map; whether an action can run
  is written once.
- `internal/sessionui` grows by at most 800 lines net (3,530 today). more needs its
  reason in the PR.

non-goals: the phone, the gateway, the wire; a new route, loop or file on disk; a
kept test suite; designed 256- and 16-colour tiers; the strip's hiding rule; the
status vocabulary; usage evidence rules; creation and editor semantics beyond those
listed; [unicode width after a visit](unicode-core-after-attach.md);
[one copy for the cli and the browser](cli-browser-copy.md);
[how name drafts are cleaned](draft-sanitization.md); [search](fleet-search.md).

## terms

- listed: the session is in its host's retained inventory. available: listed, and
  its host answered its latest inventory read (`Row.Available`). stale: its host
  failed that read. `checking`: r5.
- needs you: `fleetview`'s typed queue class is ready or action. never the text.
- machine: in the table, where the work runs (the execution context, as today). in
  the caption, a question and a notice, the host that lists the session.
- place: the machine and path columns.
- client note: words the browser adds to a row: `checking`, `last observed: …`, a
  guest's note.
- body: the lines between line 2 and the footer. known ground: g2's full path.

## behaviour

```
 ▜skid▛   needs input  ‹all›  atlas   beacon   cobalt   jarvis   skid   unassigned

 atlas
   review    ◼ needs permission  claude  unknown      —                 dev-server  …/demo/atlas
 ▌ migrate     idle              codex   personal     gpt-6-sol low     dev-server  …/demo/atlas
 ▌   t-01a9f727d4d8480a   pane %5   in /home/op/demo/atlas   runs codex
 ▌   ── screen of migrate, read just now ─────────────────────────────────────────────────────
 ▌       it has moved. I did not apply the migration. Run npx prisma migrate deploy when ready.
 ▌     › Ask Codex to do anything
   routes      working           codex   personal     gpt-6-sol low     macbook     …/demo/atlas
   schema      last observed: working                                   arch        …/demo/atlas

 ── migrate on dev-server   2 of 13 ───────────────────────────────── personal 72% of 7d left ──
 enter attach  tab next needing input  i all facts  ctrl-r read again  r rename  space fold
```

### surface

- s1. line 1 is the strip, line 2 blank, then the body; under it notices, one rule,
  one hint line. the body always holds the table.
- s2. an inline fold (open, confirm, rename, group, a draft from `c`) is inserted
  directly under the cursor's row and pushes later rows down. when the lines from
  that row to the body's end cannot hold it, it is inserted directly above and
  pushes earlier rows up. either way the cursor's row keeps its screen line, and
  `top`, the scroll of the table without folds, is unchanged. it is at most
  `min(16, (body − 1) / 2)` lines and at least one, so one side always can.
- s3. a foot fold (a draft from `N`, the terminal chooser, usage, keys) covers the
  body's last lines: at most `body × 11 / 20`, title included. the table does not
  scroll under it, so it can cover the cursor's row. no fold covers a notice.
- s4. one fold at a time. an inline fold's subject is the cursor's row. entering a
  terminal closes any fold.
- s5. every fold line carries the gold lip `▌` in column 1 and its text from column
  5; a foot fold begins with a bold title. on a known ground a fold is filled:
  raise, or forge for a draft.
- s6. a fold with more lines than its cap shows the stretch that holds its focused
  line (today's `window`), else its first lines.

### table

- t1. columns, in order, two cells apart: name (at most 28, end ellipsis), mark,
  status, group (needs input only, at most 16), provider, profile, model, machine (at
  most 16), path (the rest, at least 8). machine and path follow today's rules for
  remote rows; an unresolved one prints `remote context unknown` across both.
- t2. each width is the widest value among all listed rows' last observations, as
  the current view prints them. client notes set no width. `all` and the group
  views print alike, so stepping among them moves no column; needs input adds group
  and its longer status words.
- t3. when they do not fit, whole columns go from the end of t1, for every row.
  name and status never go. no per-row marker.
- t4. the mark `◼` is on every row that needs you, in its status's tone. that row's
  name is bold.
- t5. status is the projected label in its tone: ember for a request or error, moss
  ready, frost working, muted otherwise. client notes are muted italic, and a
  stale row's name is quiet. `checking` replaces the status word only. `last observed: <label>` and a guest's note are
  printed whole from the status column; that row prints no group or agent cells,
  and no place where the note needs the room.
- t6. path: the last two components, lexical, `…/` when more precede. too wide:
  `…/` and the last component, cut at its end.
- t7. headings (in `all` only) are bold, `unassigned` quiet. a heading is drawn only
  with its first row.
- t8. caption, the rule's left: `<name> on <machine>`, then what fits of the row's
  place when t3 dropped it, then `n of m` when rows are hidden. three forms, no
  more: all three parts, the first two, the first. a foot draft or chooser says
  `new session on <machine>` or `new terminal on <machine>`.
- t9. quota, the rule's right, in the room the caption leaves: every profile's
  weekly remainder, else the selected row's profile, else nothing. numbers, `~` and
  status words are today's footer summary's; a declaration state prints alone.
- t10. hints: one line. the current entries that can act, by rank, skipping what
  does not fit; kept entries always close it (`? keys  q quit` on the table).
- t11. strip: today's rule and markers, restyled. the wordmark is `▜skid▛`.
- t12. notices: today's order and words. a `checking` host prints
  `checking inventory` only while it has no listed row, and keeps its other notices
  as last observed.

### selection and return

- r1. selection is a lifetime, never an index. a rebuild keeps the cursor on it; a
  row that changes place inside its view takes the cursor with it.
- r2. guest: when the selected lifetime is no longer among the view's rows, its last
  row stays at its index until the cursor moves, the view changes or a created
  session is adopted. a guest still listed keeps live facts and actions; one no
  longer listed keeps its last facts, is unavailable and refuses every action in
  words. it keeps its last group, so no heading changes.
- r3. a guest's note: `left the queue: <label>` in needs input, `left this group` in
  a group view, `no longer listed` once it is not.
- r4. a move from a guest goes to the row drawn beside it, `j` below and `k` above;
  the guest then leaves.
- r5. `checking`: a listed row whose host has not been read since the browser's own
  write to it or since the browser regained the screen, and whose latest read did
  not fail. it is drawn from its last observation and only its status word changes:
  it keeps its queue class and order, its mark, its model, its remote context and
  its quota evidence. (today such a row loses the last three; that ends.)
- r6. ready needs the hub. from regaining the screen until the next hub read lands,
  a row that last read ready prints `checking` in place. a failed hub read hides
  ready, as today; a visit's own acknowledgement error no longer does.
- r7. on detach every host without a failed read becomes `checking`; view, selection
  and `top` are as left; the notice names the session and its host; one refresh
  follows. a visit that ends in error prints that error, as today.
- r8. attach is admitted iff the session is listed and its host's latest read did
  not fail. every other action needs an available row, as today. a session the
  owner just closed stays admitted until the post-write read removes it; the
  gateway refuses it.

### folds

| key | fold | where | lines | leaves by |
| --- | --- | --- | --- | --- |
| `space` | open | inline | summary (1 or 2), header, then the screen; `i` swaps in the facts | `space` `escape` `q` |
| `x` `s` | confirm | inline | the question, then its literal effect: 5 at most | `enter` `y` send; `escape` `n` `q` cancel |
| `r`, from an open row | rename | inline | the name field; a note when there is one | `escape`, to the open row |
| `g`, from an open row | group | inline | the group field with its choice; `save` with its verdict; a note when there is one | `escape`, to the open row |
| `c`, `N` | draft | inline, foot | machine, launch, name, directory, group, create: one line each, ending in its note | `escape` |
| `n` | terminal | foot | title; the machine choice, home and group; a refusal when there is one | `escape` `q` |
| `u` | usage | foot | title; today's column header and lines | `escape` `q` `u` |
| `?` | keys | foot | title; the table's entries in three columns; attach chords; the open row's keys; the ground: 10 at most | `escape` `q` `?` |

a note is the field's choice preview, state, verdict, failure or uncertainty, cut to
the line; the whole sentence prints as the refusal.

- f1. open works on any row, guest and stale included. summary: handle, pane,
  directory, command, attached clients when any. facts: state and availability
  first, then name (whole), running on, directory, handle, then the rest in today's
  order; a list that does not fit ends `… n more facts in a taller window`. only
  usage scrolls.
- f2. confirm opens from the table or an open row, on an available row, and
  captures the reference (lifetime and pane), the name and the host. the question
  is ember and bold; the effect is today's text; it lists no facts. `enter` sends if
  the captured lifetime is available now, else says what was not sent. sent or
  cancelled, it closes to the table.
- f3. rename and group keep today's keys, rules and reconciliation. an unresolved
  write keeps its draft and blocks another edit; the editor shows while the fold is
  on its session.
- f4. a draft keeps today's rules. `c` needs an available local row (a remote row
  refuses as `T` does) and seeds machine, group, directory (the row's cwd) and,
  only when known, launch: the recorded launch if that machine still declares it;
  `terminal` for a row with no agent and no recorded launch; otherwise none. with a
  launch it opens on `create`; without, on `launch`, and `create` refuses.
- f5. before anything is typed, a draft's directory chooser offers its own value,
  then home, then up to four directories that machine's sessions are in, most used
  first. a choice enters the draft only when accepted, as today.
- f6. usage keeps the strip, scrolls with `↑↓`, and asks one usage read on opening.
- f7. keys lists every table entry under session, create and browser, the ones that
  cannot act in quiet. its last line says which ground path is in use and why. a
  listed key closes it and acts.
- f8. `tab` and `shift-tab` go to the next and previous marked row of the view,
  wrapping; with none, a notice. an open row goes with them: facts off, one read.
- f9. in an open row, `j` `k` and the arrows close it, then move; `h` `l` close it,
  then step. in any other fold those keys are the fold's: nothing in a confirm,
  editing or choosing in a field. a view step or an adopted creation closes any
  fold.
- f10. `escape` closes a fold and does nothing on the table. `q` quits on the table,
  closes open, confirm, terminal, usage and keys, and is a letter in a text field.
  `ctrl-r` refreshes inventory and usage everywhere and, in an open row, also reads
  its screen. `ctrl-c` and the in-flight refusal are today's. below 80×24 the
  browser draws the size notice and the outcome and admits `ctrl-c` and the keys
  that close the fold.

### screen

- x1. one read: `client.Execute(ctx, Request{Operation: "read", Ref: ref, MaxBytes:
  4096})`, the existing route and budget. the host returns the visible screen under
  an alternate screen, else the retained tail; the fold prints `screen` for both
  and never branches on the scope. 4096 because the host captures that many lines
  of history and the fold shows sixteen.
- x2. a read is taken by a key: `space` opening a row, `tab` carrying it, `ctrl-r`
  in it. asked of a row that is not available, it waits for that host's next
  inventory and is taken then, once, if the fold is still on the row. never on a
  tick or a status change. one in flight, one owed.
- x3. dropped when the fold closes or moves, a terminal is entered, or the host
  leaves the roster or rejects access (which invalidates the host, as any call).
- x4. header, literal, one of: `screen of <name>, read <age>`; `screen of <name>,
  as read <age>; <machine> is unavailable`; `screen of <name>: reading`; `screen of
  <name>: <why>`, the row's availability word or the failure in `fleetview`'s
  words. age is the client's clock since receipt: `just now` under 5 s, `Ns ago`,
  `Nm ago`.
- x5. text: split on newlines; each line through `attention.SingleLine`; blank lines
  and lines of only box-drawing, dashes, underscores and spaces dropped; the last
  lines that fit, cut to the fold's width in display cells, two cells in from the
  fold's own text.
- x6. status and screen are separate samples and may disagree until the next one.

### ground

- g1. at `Init` one write: `OSC 10 ?`, `OSC 11 ?`, `CSI c`. never again. under
  `NO_COLOR` nothing is written and the ground is plain from the start.
- g2. the ground settles once, at the device-attributes reply or after 1 s. it is
  full iff the profile is truecolor, both colours came before that reply and the
  ink has 4.5:1 or more on raise; else plain. later replies are ignored.
- g3. until it settles the palette is plain and attach refuses with a notice, so a
  reply inside that second cannot be typed into a session.
- g4. full. dark iff the ground's luminance is under the ink's. raise =
  mix(ground, ink, 7%). forge = mix(ground, gold, 10%). quiet = the first 1% step
  from raise toward ink whose contrast on raise is at least 4.5:1 and at least half
  the ink's. pigments are fixed, never the theme's slots: on a dark ground gold
  `#D6A85F`, ember `#E46C55`, moss `#76B082`, frost `#78A9C6`, bronze `#CD7F32`; on
  a light one `#8A6416`, `#B23B24`, `#3B7448`, `#2C6688`, `#94561A`. a pigment under
  4.5:1 on raise steps 1% at a time toward white on a dark ground, black on a light
  one, until it clears. raise fills the cursor's row, a fold and the current view's
  label; forge fills a draft.
- g5. one palette of roles, two sets of values:

  | role | carries | full | plain |
  | --- | --- | --- | --- |
  | ink | names, labels, notices, the focused field | default | default |
  | quiet | group, provider, profile, model, place, the rule, hint labels, `unassigned`, field labels | derived quiet | SGR faint |
  | muted | a status without a pigment; client notes | derived quiet | `#AAA69D` |
  | gold | the lip, the current view's `‹ ›`, a focused chooser | pigment | bright yellow |
  | ember | a request or error status, a failed outcome, the question | pigment | bright red |
  | moss, frost | ready; starting and working | pigment | `#76B082`, `#78A9C6` |
  | bronze | the wordmark's block | pigment | reverse video |

  bold: the cursor row's name, names of rows that need you, the current view,
  headings, keys, titles, the question. italic: client notes. the plain values are
  today's.
- g6. `NO_COLOR`: as today, every style stripped and every character kept.
- g7. one renderer. a line is built from (text, role) spans and rendered once; a
  filled line carries its fill on every span and on the padding to the width. never
  by restyling finished strings, never with erase-line.

## design

### state

the model loses `page`, `offset`, `pageRow`, `pendingName`, `pendingLabel` and the
visit's acknowledgement error; `pending` keeps one role, the request in flight. it
gains `fold`, `guest bool` (r2), `screen`, `screenLane lane` and `ground`. the
editor, the form, the group selection and the directory search keep their state
and lifetimes.

```go
type foldKind int // none, open, confirm, rename, group, draft, terminal, usage, keys

type fold struct {
	kind          foldKind
	request       fleetclient.Request // confirm: stop or close on the reference captured at open; terminal: the start
	name, machine string              // confirm: the words captured at open
	facts         bool                // open: the facts instead of the screen
	seeded        bool                // draft: opened by c, drawn under its row
	offset        int                 // usage: first line shown
}

type screen struct {
	ref     fleetclient.Reference // whose
	lines   []string              // x5
	at      time.Time             // client receipt
	failure string                // why there is none
	asked   bool                  // a read waits for the host's inventory (x2)
}

type key struct {
	keys   []string            // as bubble tea names them, aliases included
	show   string              // in hints and the key map; "" hides it
	label  string              // literal
	group  string              // key map column: session, create, browser
	rank   int                 // hint order; 0 keeps it off the hint line
	keep   bool                // always on the hint line
	closes bool                // closes or cancels the fold: admitted below 80×24
	can    func(*model) string // "" when it can act now, else the refusal sentence
	act    func(*model) tea.Cmd
}
```

### keys

- `m.keys()` returns the entries for the current fold kind and focus, so a key that
  means different things per field has one entry per meaning and a static label.
  dispatch: the in-flight refusal, `ctrl-c`, the size filter, `ctrl-r`, the first
  entry holding the key, then a text field's own typing.
- a key whose `can` returns a sentence prints it as a failure and does nothing. the
  hint line lists an entry only while `can` is empty. a fold that prints a verdict
  (create, save) prints the same `can`. nothing else decides whether an action may
  run.
- table, by rank: `enter` attach, `space` open, `tab` next needing input, `x`
  interrupt and close…, `c` another like this…, `N` new session…, `n` new
  terminal…, `s` interrupt…, `T` terminal here, `u` usage. kept: `?` keys, `q` quit.
  off the line: `↑↓ jk`, `←→ hl`, `shift-tab`, `ctrl-r`.
- open, by rank: `enter` attach, `tab` next needing input, `i` all facts (or
  screen), `ctrl-r` read again, `r` rename, `g` group, `x`, `s`. kept: `space` fold.
- the other folds keep today's keys and meanings, one entry each per focus; their
  kept entry is `escape`.
- the cli's browser help keeps what the browser is, its size floor, the attach
  chords and that `?` lists every key.

### frame and composition

- `frame` is the one layout: columns (t1 to t3), footer lines, body height, the
  fold's lines and side (s2, s3), the visible rows. `View` builds it once and draws;
  `settle` uses it to keep the cursor's row in view. nothing else computes columns
  or the footer.
- `top` changes only on a cursor move, a view step, a resize, or when a change of
  rows or a footer that grew would hide the cursor's row; then by the least that
  keeps it drawn. the cursor's row is always drawn, except under a foot fold.
- flow: message, `Update`, the key table or a stream's accept, `rebuild` (rows and
  guest), `settle` (`top`), `View`, `frame`, lines.
- streams: the inventory, context, hub and usage lanes as today; the screen lane;
  the ground, once.
- reach: the browser reads inventory, context, hub state, usage, directory search
  and, new, one session's screen on a key; it writes stop, close, rename, group,
  start and shell; it lends the tty. nothing else.

### files

- new in `internal/sessionui`: `fold.go` (the fold value: open, close, carry,
  confirm), `ground.go` (query, settle, palette, spans), `screen.go` (the screen
  lane, its accept, its lines).
- rewritten: `layout.go` becomes `frame`; `keys.go` becomes the `key` type,
  `keys()`, dispatch, the hint line and the key map; `view.go` keeps `View`, the
  strip, the footer and the empty states.
- changed: `session.go`, `navigation.go` (rebuild, guest), `details.go`,
  `metadata.go`, `create.go`, `usage.go` (each now draws fold lines),
  `inventory.go`, `notifications.go`. untouched: `lane.go`, `group_entry.go`,
  `field_selection.go`.
- outside the package: `internal/agentcli/run.go` (the browser's help) and `go.mod`
  (ultraviolet becomes a direct requirement at the revision bubble tea pins, for
  the device-attributes event).

## content

copy is designed, not improvised. before a slice's tests are written, a designer
writes that slice's rubric (how its copy is judged under the rules below; it
changes no rule and no schema) and then its copy, and returns both to the root. the
root accepts or returns them and writes the table; the owner sees every string in
the pass and may change any. the builder and the tests take strings from the table.
on merge the rules move to design language §17 and the words live in the code.

rules: lowercase and literal. a string claims only what was sampled or done. an
action or outcome names its session and the host that lists it. the host's status
words and `fleetview`'s outcome words are never reworded; the browser adds only
client notes. a refusal says what was not sent and why. `…` ends a label only when
the key opens something that must be confirmed before anything is sent. no ember in
hints. everything fits 78 cells or wraps whole.

| schema | slice | shape | seed |
| --- | --- | --- | --- |
| guest note | 1 | the fixed three | r3 |
| detach | 1 | who, where | `detached from deploy on dev-server.` |
| refusal | 1 | target, what is disabled, why | `review on dev-server is unavailable; attach is disabled.` |
| key label | 2, 4, 5 | verb phrase, at most 24 cells | the lists under keys |
| caption | 2 | target, place, position | `migrate on dev-server   …/demo/atlas   2 of 13` |
| quota | 2 | profile and value | `7d left  personal 72%  work 89%`; `personal 72% of 7d left` |
| nothing next | 2 | scope and inference | `no other session in this view reads as needing input.` |
| empty state | 2 | what is absent and what that means | one each: all hosts checking, queue empty, no sessions, empty view |
| unsettled ground | 3 | cause and remedy | `reading the terminal's colours; press enter again.` |
| ground line | 5 | path and cause | `colours: plain; the terminal did not report its own.` |
| summary line | 4 | value, then `label value` pairs | `t-01a9… pane %5 in <cwd> runs <command>` |
| facts cut | 4 | count and remedy | `… 9 more facts in a taller window` |
| question | 4 | `<verb phrase> <name> on <machine>?`, then today's effect | `interrupt and close review on dev-server?` |
| field note | 4, 5 | state or verdict, one line | `blank: follows the terminal title` |
| unknown launch | 5 | whose, what is missing | `‹ choose ›  deploy does not say how it was started` |
| host refusal | 5 | one sentence for today's five | `arch is unavailable; nothing was sent.` |
| screen header | 6 | x4's four forms | `screen of migrate, read 40s ago` |

## contract amendments

the root makes these in the branch's last commits; no slice edits
`docs/architecture.md`.

- intro, owners, concepts: a session's screen joins the fact sources; one fold for
  one page; `checking` is a third status overlay; a confirmation captures its
  target, an open row follows the cursor's row.
- invariants: 2 folds for pages. 5 a `checking` row admits attach only. 6 one screen
  read, on a key. 7 a host is also `checking` until re-read after the browser
  regains the screen, and a `checking` row is drawn from its last observation. 9 `c`
  joins `n`, `N`, `T`. 10 becomes r1 to r8.
- interfaces: the browser's `read` among the gateway calls.
- views and the queue: in the browser a `checking` row keeps its last membership and
  a guest is drawn in a view it has left; `opening a row` for `opening info`.
- browser: the surface and strip sentences (wordmark, footer, `page`, `else clamps
  the former index`); rows become t1 to t7, the status bound and the omission
  marker included; keys become f8 to f10 and `the rest is in ?`; actions gain `c`,
  a confirmation lists no facts, and a directory offers choices before typing; info
  and editing become f1 and f3; a `checking` host's notice follows t12.
- usage: the weekly summary sits on the rule; the usage fold keeps the strip; usage
  is read each minute while the browser has the screen and once when the fold
  opens; `ctrl-r` works there.
- accepted limits: drop per-page hints drifting, unaligned secondary columns and the
  per-row omission; add `c` beside `n`, `N`, `T`; scope the faint and bright-yellow
  limits to the plain path; add `▜`, `▛` to the ambiguous-width glyphs; add each
  trade-off below that describes shipped behaviour.
- rationale: why folds; the mark repeats its own row's status and claims nothing of
  a row without one.
- design language §17: the browser fills three things on a known ground, from the
  operator's own colours; the role table (g5) replaces the intensity and accent
  tables, with the light pigments, bold for rows that need you and headings, italic
  for client notes, gold for the lip and a focused chooser, ember for the question,
  bronze for the wordmark; the lip, not the gutter, marks a fold, and a focused
  field is bold; folds add lines between rows and one rule inside an open row; the
  mark; the copy rules above; §14's floor now holds for derived tones. `no boxes,
  fret, runes, icons or motion` stands.
- also: `docs/codebase-map.md` if it names changed files; `docs/roadmap.md`;
  [unrun acceptance](unrun-acceptance.md)'s v0.17.3 row review, which h1 replaces;
  delete [the return shuffle](return-queue-shuffle.md) and this file.

## slices

in order, one builder at a time, in one worktree. it is one package around one
model, so the split is by responsibility and time, not by path: a slice may write
any file in `internal/sessionui` and does only its own job. the designer and the
test author of the next slice work ahead, outside the repo. each slice leaves the
branch building and the browser usable, and reruns every earlier slice's lines.

| slice | does | proves |
| --- | --- | --- |
| 0 groundwork | the harness; the invisible cleanup; an access rejection from any write invalidates its host | the journeys' frames identical before and after; a7 |
| 1 return | r1 to r8, t12 | a1 to a6 |
| 2 table | s1, t1 to t11; spans and roles on today's inks; `key`, `keys()` for the table, the hint line, `tab`; `frame` with empty slots | a8 to a11 |
| 3 ground | g1 to g6: the query, the settle, the derived values, the fills | a12 to a14 |
| 4 row folds | the fold value; s2, s4 to s6; open with summary and facts, confirm, rename, group; f8's carry, f9; the info, confirm and editor pages deleted | a15 to a18 |
| 5 foot folds and drafts | s3; draft, `c`, f5, terminal, usage, keys; the last pages and the page machinery deleted; the cli's help | a19 to a21 |
| 6 screen | x1 to x6; `i` | a22, a23 |
| 7 close | the amendments; the owner's pass; the harness deleted; the PR | h1 to h5 |

slice 0's journeys: the first inventory in each view at 80×24 and 120×40; info; each
editor opened and cancelled; the form tabbed through and cancelled; the chooser; a
confirmation cancelled; usage; a failed host; one write's receipt.

## method

- worktree `skidbladnir-fold`, branch `feat/browser-fold`, from `main`. no
  dev-server half: no deployment asset changes.
- each slice: designer (rubric, copy); red (the slice's acceptance lines as
  temporary tests, run on the branch tip, failing for the stated reason, output
  shown); green (code until they and `scripts/check verify host` pass); refactor (to
  `docs/rules.md` and the line budget); review (a fresh adversarial reviewer given
  the code and this file, not the builder's account; it writes nothing); the root
  reruns the tests and commits.
- the harness is a module outside the repo (`browser-redesign-scratch/foldtest/`
  in the claude-work project directory on the macbook):
  `github.com/NielsdaWheelz/skidbladnir/zzfold`, with a `replace` to the worktree,
  so no test, fake or requirement can be committed. probed 2026-11-17: it imports
  the internal packages, resolves `charmbracelet/x/vt` from the module cache and
  leaves the repo untouched.
- the harness holds: fake gateways (`httptest`, real `internal/api` documents for sessions,
  profile usage, hub state, read, each write's receipt, and a terminal socket that
  greets and ends on detach); `fleetclient.New` with a self handle outside the
  roster (never `skid --config`, which posts to 7341); `sessionui.Run` on a
  `pty.Open` pair; frames read through `x/vt`; the ground query answered by the
  harness. its fleet mirrors the port's, so a frame can be laid beside the port's
  `--dump`: evidence, not a gate. everything binds 127.0.0.1:0 in scratch
  directories, so it needs no approval. about 500 lines.
- reference, not contract, in the same archive under `fold/`: `SPEC.md`,
  `src/engine.js`, the port `agents/go/skidfold` (`go run . --dump --size 100x24
  --keys 'j space'`), `agents/verify/report.md`. where they differ from this file,
  this file wins: they still have `a`, `/`, a third ink and a notched fold.
- tests assert what the terminal shows and what the fakes received. the only style
  they read is ruling 10's.
- models: opus for designers, builders and reviewers; sonnet for mechanical edits
  only; three agents at once at most.
- the owner's pass: the owner builds from the worktree and runs it in ghostty
  against the installed fleet. agents do not: it reaches live state. its record is
  five verdicts, with no frame and no screen text.
- commits are the root's, one per slice, once the owner starts the build. push, PR
  and merge each wait for the owner's word. the release is the owner's, by the usual
  procedure, after the merge. a gate that did not run is `NOT_RUN`, never a pass.

## acceptance

by temporary test:

- a1. needs input fills the body with ready rows and a request; enter a ready row,
  detach. until a key: the cursor's row is the visited session; no row changes line
  unless the footer grew over the cursor's row; each row reads `checking` until its
  host answers, a ready row until the hub does; no host notice, model, remote
  context or quota blinks.
- a2. then the visited row reads `left the queue: idle`; `j` selects the row below
  it and the guest is gone.
- a3. the detach notice names the session and the host that lists it.
- a4. on a `checking` row `enter` attaches and `x` refuses in words. with the hub
  unreachable through a visit, ready rows read `checking` until the hub read fails.
- a5. a selected session closed elsewhere stays as `no longer listed`; its actions
  refuse; a move leaves it.
- a6. the owner's own `x` leaves its row as `no longer listed` under a notice naming
  what closed, and `enter` on it before the post-write read attaches nothing and
  says why; a regroup out of a group view leaves `left this group`.
- a7. `Unauthenticated` from stop, close, shell, rename or group reads the host
  unavailable at once.
- a8. at 80×24 and 160×50, in every view: no line exceeds the width, each session is
  one line, no heading ends the body, a cut path ends on a whole component.
- a9. stepping among `all` and the group views moves no column. when machine and
  path do not fit they leave every row, the caption carries the selected row's
  place, no row ends in `…`, and a stale row's note is whole.
- a10. the hint line is one line ending in its kept entries. every key on it acts; a
  key that cannot act is absent and, pressed, prints its refusal.
- a11. `tab` visits exactly the marked rows, wrapping. the rule carries every
  profile's weekly remainder when it fits, else the selected row's.
- a12. both colours and the attributes reply: the cursor's row is filled. that reply
  alone, 256 colours, or solarized light: nothing is. `NO_COLOR`: no query, no
  style sequence, the same characters.
- a13. `enter` before the settle refuses; after, it attaches. one query per run.
- a14. on niðavellir (`#0C0D0F`, `#F3F0E8`), gruvbox dark (`#282828`, `#EBDBB2`),
  tokyo night (`#1A1B26`, `#C0CAF5`), nord (`#2E3440`, `#D8DEE9`) and bone
  (`#F3F0E8`, `#0C0D0F`), every text cell of a filled line has 4.5:1 or more on its
  own background; rerun with each fold open.
- a15. opening each inline fold on the first, a middle and the last visible row, at
  80×24 and 100×30, leaves that row on its line and the fold whole; closing
  restores every body line.
- a16. a confirmation names the captured session and host; a rename elsewhere does
  not reword it; `enter` on a lifetime that left sends nothing and says so; `escape`
  sends nothing.
- a17. `tab` carries an open row to the next marked row; `j` closes it, then moves,
  and `l` closes it, then steps; in a rename `j` types; entering a terminal closes
  the fold.
- a18. a failed save keeps its draft and blocks a second edit; facts that do not fit
  end in the count cut.
- a19. `c` on a row with a recorded launch opens on `create` and creates one session
  with that launch, machine, directory and group; with an unknown launch it opens
  on `launch` and `create` refuses; a remote row refuses. the directory chooser
  offers its own value, then home, then that machine's directories.
- a20. `?` lists every table entry, the refused ones quiet, and the ground's path; a
  listed key closes it and acts. `u` keeps the strip and scrolls.
- a21. with any fold open the table is drawn around it and no notice is covered.
  below 80×24 only `ctrl-c` and the fold's closing keys act. `q` quits only on the
  table.
- a22. `space` sends one read; thirty seconds of ticks and a status change send
  none; `ctrl-r` sends one; a read asked of a `checking` row is sent once when its
  host answers; a reply for a row the fold has left is dropped.
- a23. the header ages; a host that fails after the read gives `as read …`; escape
  sequences in the text never reach the terminal.

by the owner, in ghostty on the installed fleet:

- h1. every string and the look are right on a dark and a light theme.
- h2. attach, detach, next, attach never waits and never lands on a row the owner
  did not choose.
- h3. the screen in an open row answers `what is it doing` more often than not.
- h4. the third ink, seen in the port: wanted or not.
- h5. over mosh, and under `NO_COLOR`, every word is legible.

## trade-offs

each is taken on purpose; none is absorbed.

1. one cutover: the largest blast radius, one large PR, no week of living with
   folds before the screen read ships.
2. the screen read ships unproven: an idle agent's tail is mostly its input box. h3
   is a first look; if a week of use says no, `screen.go` and its hooks go.
3. every return blinks the status column; until a host answers, its rows admit
   attach only.
4. a guest costs one key after the owner's own close or regroup, and shows a row in
   a view it has left. a row that moves inside its view moves under the cursor.
5. facts are cut in a short window and do not scroll (`skid info` has them all); a
   confirmation lists none; a form's long note is read whole only as the refusal.
6. at 80 columns machine and path usually leave every row, and only the selected
   row's place remains, in the caption. one long value anywhere costs every row.
7. the plain path is today's inks on the new layout, not designed further. every
   truecolor terminal that answers gets the full path, tested only in ghostty. a
   theme whose own text is under 4.5:1 on raise (both solarized) gets the plain
   path; on a low-contrast theme quiet sits close to ink.
8. on the full path the accents are the design language's, not the theme's, and
   bold marks rows that need you as well as the cursor.
9. the first frame is plain and repaints at the settle. the ground is read once: a
   theme change needs a restart. a terminal that never answers the attributes query
   delays an attach in its first second, and a colour reply later than that could
   be typed into a session. none is known.
10. a foot fold can cover the cursor's row; the caption still names it.
11. each read costs its host about five tmux commands and a tls handshake, spent
    only on a key. usage is read every minute while the browser has the screen.
12. `skid --help` no longer lists the browser's keys.
13. slices are sequential and share files, not the path-disjoint split asked for;
    `page` survives on the branch until slice 5.
14. the cli and the browser still word the same host outcomes separately, and name
    drafts are still cleaned in one form only: recorded, not fixed.
15. tests are temporary by repo rule: after the merge nothing guards the browser
    ([test reset](test-system-reset.md) stays open). real tmux capture and process
    recognition are first exercised in the owner's pass.

## cleanup

- slice 0, with no visible change: peer lookup written nine times becomes one by
  handle and one by label; dead code goes (`initial` and `retain` in `onSearch`,
  `!m.notificationFailed` in `statusView`, the unread clears of `pending`, the
  second `writing = false`); helpers used once are inlined; the package's
  `justify-*` tokens go as `docs/rules.md` directs; the loop variable shadowing
  `group` is renamed.
- absorbed by the slice that rewrites it: whether an action can run, written three
  times per action, becomes `can` (2, 4, 5); the captured target, held in five
  places, becomes the fold value (4); hints written per page and the cli's key list
  become the key tables (2 to 5); the footer and columns, computed three times a
  frame, become `frame` (2); the group field's choreography, written twice, becomes
  one value (5); the host-refusal sentence, written five ways, becomes one (5).
- deleted with the pages: the `page` string and its 77 references, `pageCapacity`,
  `settle`'s clamps, `hints`, `keyLines`, the omission marker, `target`,
  `confirmKey`, `detailsKey`, the usage page's key handler, the visit's
  acknowledgement error.
- left: text editing's three copies, whose cleaning differs.

## risks

- today's viewport admits whole records forward from `top`; an upward fold needs
  rows admitted backward from the cursor. slice 2 builds both slots empty so slice
  4 only fills them.
- a notice arriving under an open fold shrinks the body and can flip the fold's
  side. accepted; it must never move the cursor.
- the ground's plumbing (`tea.Raw` in `Init`, colour messages, the untranslated
  device-attributes event) was read in bubble tea 2.0.9's source and run only in
  the port under xterm.js. bubble tea's own keyboard query is answered after the
  attributes reply; a key pressed later arrives behind it.
- `x/vt` is the tests' eye; where it and ghostty differ, ghostty is right.
- the estimate is +600 ± 150 lines against the 800.

resolved when: slices 0 to 7 are merged, a1 to a23 passed on the branch's final
tree before the harness was deleted, the owner gave h1 to h5, and the contract
carries the amendments; or the owner retires the design.
