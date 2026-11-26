# The rail

Status: plan. Decisions 1 to 13 are the owner's, 2026-11-25 and 26, and closed; a to
i are the author's and stand until he strikes them. Nothing is built and no contract
has changed. Base: `origin/main` `3a02df2` (v0.31.1). Worked content, in
`~/Documents/code/rail-scratch/`: `plan/f1-rail.md` (copy, the prose corpus),
`plan/f2-stage.md` (every outcome line, launcher errors), `plan/f3-docs.md` (every
contract sentence, old and new), `review2/` (the reviews of this plan and the tmux
commands they ran), `proto/` (a runnable prototype of an earlier draft:
`proto/demo.sh`). Where a plan file differs from this file, this file rules.

Problem: entering a session lends it the whole terminal. The browser stops drawing,
so the other sessions' signs are out of sight exactly while the owner works, and
every hop to another session passes through a detach.

Impact: a request elsewhere is seen only after leaving the current session.
`architecture.md` records it as an accepted limit: "While attached the browser's
chrome and status are gone; navigation needs detach."

Evidence:

- `enter` runs `tea.Exec` of `terminalclient.Run` (`internal/sessionui/session.go:505`);
  Bubble Tea blocks its event loop for the whole visit.
- Drawn by the real layout code at 28 columns, the table is already `▌?  name` under
  group headings (`rail-scratch/audit/frames/`).
- A private outer tmux server compositing a 28-column pane beside a session:
  0 cells differing across every rendering scene, +0.15 ms median key echo, 0 torn
  frames at 60 Hz (`spikeA/`). An emulator inside the browser needs a false
  terminal identity, a raw input router and 12 ms of echo (`spikeB/`).
- The prototype: production +568 −11 lines; 57.9 ms median from `enter` to the new
  session's first frame; the full palette inside the outer server when it starts
  attached with `-T RGB`. The owner ran it in Ghostty and resized it: clean.
- Run on tmux 3.8 (`review2/opus-recheck.md`, with the commands): one layout command
  holds through every transition; a visit started by kill, select, layout, set id,
  respawn in one invocation starts at its final size, and the old process gets its
  hangup before anything else; a `pane-died` hook hands the dead pane's status to
  the client through `detach-client -E` and prints nothing; `window-active-style
  fg=default` inherits the inactive style; `client-attached` does not fire for an
  attached `new-session`; a `sandbox-exec` rule refuses one loopback port.
- The gateway sends `ReconnectRequired` for a detach, a session's end and its own
  pty, resize and presence faults alike (`internal/gateway/terminal.go:154-224`);
  the browser shows it as an ember `Reconnect required.`
- tmux 3.4 loads the option set with errors drawn over the pane and runs on without
  underline styles. The mac and dev-server run 3.8.

Resolved when: A1 to A17 pass on the branch, the contract states the behaviour, and
this file is deleted in the docs commit.

## Goal

Attaching no longer hides the browser; it narrows it. On a window of 110 columns or
more, `enter` puts the session beside a 28-column rail of the same table, and one
key moves the keys between them.

## Non-goals

- No change to a gateway, the wire, the phone, the hub or acknowledgement. `skid
  enter` is unchanged: banner, both chords, sentences, exit codes, resize timing.
- No preview: moving the cursor never shows, attaches, resizes or acknowledges a
  session. No `attach -f read-only,ignore-size` mode.
- No guard on the table: after any end its keys act at once, as today.
- No terminal emulator in the browser. No second viewer, grid, gutter of signs under
  110 columns, back key, hidden-row counts or status-line tally.
- No clickable rows. No media reshoot ([stale demo media](stale-demo-media.md)).
- No fallback: no browser outside the compositor, no switch to turn the rail off, no
  second option set for an older tmux.

## Decisions

The owner's:

1. tmux composites: `skid` starts a private tmux server with the browser in one pane
   and one visit in the other.
2. The stage changes only on `enter` or a creation. The cursor roams free.
3. Beside a visit the rail navigates only. Acts happen in the table.
4. `ctrl-]` gives the keys to the rail and sends the session no key. The browser has
   no `ctrl-] d` and no literal `ctrl-] ctrl-]`.
5. Whichever side lacks the keys is dimmed as far as tmux can: default-coloured text.
6. The rail is 28 columns; the split needs 110. Tuned after use.
7. Under 110 columns a visit fills the window, and `ctrl-]` ends it and shows the table.
8. Tests are temporary and isolated. His look comes after the merge, `NOT_RUN` until
   then.
9. Hard cutover: the lent-terminal path leaves `internal/sessionui` in this change.
10. `ctrl-d` ends a visit from the rail.
11. Beside a visit only `j`, `k`, `tab` and `shift-tab` move the cursor.
12. `Launch` refuses a tmux older than 3.8 and says so.
13. No return to the table starts frozen. The dim and the lip are the guard; a
    mistake is accepted. Revisited after use.

The author's:

a. Any end of a visit returns the table with a notice. No dead stage stays on screen.
b. A return to the table starts one coalesced read of every host (`refresh(true)`).
   No row is marked checking for it. A switch starts none.
c. When the session gains the keys the cursor selects the shown lifetime, as a guest
   when the view excludes it and inventory still lists it; otherwise it stays.
d. By decision 13's reasoning the rail has no frozen state either: beside a visit a
   key that is not a rail key is discarded and draws one line. Arrows, `ctrl-c`,
   `escape`, `q`, `h`, `l` and the act keys are such keys.
e. The stage owns its outcome sentence. When the host ends the connection it reads
   `the host ended the terminal connection; work may continue`, in ink: a detach, a
   session's end and three gateway faults are one observation.
f. The plain palette path draws a bright-yellow divider while the session has the
   keys and no dim. `NO_COLOR` draws neither.
g. `terminalclient.Run` is untouched. The stage calls a second entry point without
   the chord decoder and with a resize settle.
h. One temporary non-test file in `internal/sessions` may admit a private tmux
   server for the harness, as ruled for status signs. If the harness needs a second,
   the build stops and reports.
i. One sentence about the rail joins the README's canonical paragraph and the site.

## Target behaviour

Concepts:

- **Table**: the browser across the whole window. Every key is today's.
- **Visit**: one terminal connection to one reference, held by one stage process.
- **Stage**: the right pane, holding the one live visit.
- **Shown row**: the row whose lifetime the visit holds, by `SessionEqual`.
- **Rail**: the browser while a visit is live, 28 columns wide.
- **Keys**: tmux's active pane. One side has them.
- **Compositor**: the browser's private tmux server. No prefix, no status line; in
  no inventory; gone when its terminal or its browser is.

Layout:

| Visit | Columns | Layout | Keys |
| --- | --- | --- | --- |
| none | any | the table alone | table |
| live | 110 or more | split: rail 28, divider 1, stage the rest | either side |
| live | under 110 | the stage alone | session |

Transitions:

| From | Event | To | Effects |
| --- | --- | --- | --- |
| table | `enter` on an attachable row; a creation whose stage size is valid | session keys | today's presentation resets on entry; one stage process for that reference |
| table | a creation whose stage size is not valid | table | today's `adoptCreated` outcome: the row selected, the refusal, no dial |
| session keys | `ctrl-]`, 110 or more; a click on the rail | rail keys | no key and no resize reaches the session |
| session keys | `ctrl-]`, under 110 | table | the visit ends inside the key binding, before any layout moves; notice |
| rail keys | `enter` on the shown row; `ctrl-]`; a click on the stage | session keys | no dial, no acknowledgement, no attach guard; closes the keys map; decision c |
| rail keys | `enter` on another attachable row | session keys | the old stage process is killed first, then the new one starts, in one tmux invocation |
| rail keys | `ctrl-d` | table | the visit ends; notice |
| rail keys | the window narrows under 110 | table | the visit ends inside the resize hook; notice |
| either | the stage process exits | table | notice from its outcome |
| live | any other resize | same | layout re-applied; the session is sent a new size only if its size changed |

Rail keys beside a visit:

| Key | Does |
| --- | --- |
| `j`, `k`, `tab`, `shift-tab` | today's row movement |
| `enter` | as above; on another row, today's refusals for an unattachable row |
| `ctrl-]` | keys to the shown session |
| `ctrl-d` | ends the visit |
| `?` | the keys map; `?`, `q` and `escape` close it, `ctrl-]` closes it and returns the keys |
| `ctrl-r` | today's refresh |
| every other key; any bracketed paste | discarded; draws the line below |

The discard line clears on the next rail key or when the keys leave the rail; a
second stray key redraws nothing; with the keys map open it is drawn and the map
stays. Every return to the table closes the keys map.

The rail learns who has the keys, and whether the visit lives, from the
compositor, never from a key's arrival. Beside a visit it reads `State` before it
acts on any key or paste: an ended visit is applied first and the key is then the
table's; with the stage active the key is discarded; with the rail active the key
is handled. A key read while a movement of the keys it asked for is pending is
discarded.

Copy. `<s>` is the row's name, ` on `, the listing host's label. The rail's
sentences end with a period; labels do not; text after `<s>: ` keeps its
producer's punctuation.

| Where | Text | Ink |
| --- | --- | --- |
| hint, session keys | `ctrl-] browse` | |
| hint, rail keys | `ctrl-] back  ctrl-d detach` | |
| hint, keys map | `ctrl-] back  escape close` | |
| a discarded key | `the rail has the keys; nothing was sent.` | ember |
| notice, `ctrl-d` | `detached from <s>.` | ink |
| notice, `ctrl-]` or narrowing under 110 | `detached from <s>; the window was under 110 columns.` | ink |
| notice, stage exit 3 | `<s>: <outcome>` | ink |
| notice, any other stage exit | `<s>: <outcome>` | ember |

The keys map beside a visit, 23 cells: `rail keys`; `ctrl-] back`; `enter attach`;
`ctrl-d detach`; `jk move`; `tab next needing input`; `shift-tab previous`;
`ctrl-r refresh`; `escape close`; `▌ rail has keys`; `▏ session has keys`;
`▐ shown at right`; `other keys do nothing`. The table's map loses its attach-chord
line and gains `ctrl-] browse`.

Drawing:

- The rail is the table's own frame at 28 columns. Notices, caption and hints have
  26 cells.
- The shown row carries `▐` in screen column 26: gold while the session has the
  keys, quiet otherwise. The cursor's lip is `▌` gold while the rail has the keys
  and `▏` quiet otherwise. A view that excludes the shown row draws no `▐`.
- Full palette path: the divider is gold while the session has the keys and quiet
  otherwise, and the side without the keys draws default-coloured text in the quiet
  pigment. Plain path and `NO_COLOR`: decision f. The lip's shape, `▐`, the terminal
  cursor (tmux shows it only in the pane with the keys) and the hint line carry the
  state without colour.

## Contract

### Processes

```
skid                      the launcher: starts the compositor attached, waits, cleans up
└ tmux -S DIR/tmux        the compositor (server and this terminal's client)
  ├ skid rail [--config STATE] --compositor DIR --tmux PATH   pane %0: the browser
  └ skid stage [--config STATE] --outcome FILE --ref REF      pane %1: one visit
```

`rail` and `stage` are private, with one producer each. They are matched before
public parsing and take these named flags in any order, each at most once and the
required ones exactly once; an unknown, repeated, empty or missing flag, or stdin
or stdout that is not a tty, exits 2 with no output.

### `internal/compositor` (new): every control invocation on the browser's server

```go
const (
	RailColumns  = 28
	SplitColumns = 110
	StageEnded   = 3 // a stage's exit status: the host ended the connection
)

// Launch runs the browser as pane %0 of a new private server attached to this
// terminal and returns its exit status. command is the installed command's path.
func Launch(ctx context.Context, tmux, command, config string, input, output *os.File) (int, error)

// StageSize is the size a visit from a window of columns × rows gives a session.
func StageSize(columns, rows int) (int, int)

type State struct {
	Columns, Rows int    // the window
	Visit         string // the id of the last visit started; "" before the first
	Live          bool   // that visit's stage process has not ended
	End           string // why it ended: "rail", "narrow" or "exit"
	ExitStatus    int    // for End "exit"; -1 when tmux reports none
	StageKeys     bool   // the stage is the active pane
	Returns       int    // times the stage was given the keys
}

func Open(directory, tmux, command, config string) (*Client, error)
func (*Client) Close()
func (*Client) Woken() <-chan struct{}                 // coalesced; read State after each
func (*Client) State() (State, error)                  // one tmux call that lists panes
func (*Client) Visit(ref string) (visit string, err error)
func (*Client) Keys(visit string, stage bool) error    // acts only while visit is live
func (*Client) Table(visit string) error               // ends that visit, End "rail"
func (*Client) Outcome(visit string) (string, error)   // reads and removes the line; "" when none
func (*Client) Paint(gold, quiet string, dim bool) error
```

- `Launch` reads `tmux -V`, takes the first `major.minor` as integers, and refuses
  under 3.8 or on no match, before creating anything. `DIR` is
  `os.MkdirTemp("", "skid-rail-")`, mode 0700: `tmux` (socket), `conf`, outcome
  files. It arms `ctx` and SIGHUP before creating `DIR`; on every return it stops
  the server, reaps the client and removes `DIR`.
- The server starts attached with `-T RGB` and `-f DIR/conf`, `TMUX` and
  `TMUX_PANE` dropped, holding `%0` only; `Visit` creates `%1` on first use, so the
  browser opens at every window size as today. `compositor.conf` is embedded and
  rendered with the constants: the measured set
  (`rail-scratch/proto/tree/internal/agentcli/compositor.conf`), plus
  `assume-paste-time 0`, no colour before `Paint`, and the three items below.
- The server records what it did, in the same command sequence that did it: the
  visit's id, `Live`, `End` and `Returns`. `State` reads those and the active pane;
  nothing is inferred from a pane's death or a placeholder's exit.
- The layout rule is one tmux command that tests the pane count, `Live`, the
  active pane and the window's width inside the server and guards every zoom
  (`resize-pane -Z` toggles). The `window-resized` and `window-pane-changed` hooks
  run it; `Visit`, `Keys`, `Table`, the `C-]` binding and the stage's `pane-died`
  hook end with it. No layout is computed from a width read beforehand.
- `Visit` is one tmux invocation that, in order: kills any live stage process and
  records that visit's end as `rail`; selects `%1` and applies the layout a live
  visit has at the current width, given to the rule explicitly; starts the stage
  process; records the new id and `Live`. So no key reaches the old session once
  the invocation starts, the new process starts at its final size (91 columns of a
  120-column window, 131 of 160), and a failed start records no new visit.
- A visit ends by its process exiting (`End` `exit`), by `Table` (`rail`), or by
  the `C-]` binding or the resize hook under 110 columns (`narrow`). The last three
  act only while the visit they name is live, and replace the process before they
  run the layout rule; tmux's own resize of the panes may come first. `Keys` is
  conditional in the same way. An `exit` already recorded is never overwritten.
- The root key table holds only: `C-]` (in `%0`: passed to the rail; in `%1`: the
  transitions above), `Home`/`End` as SS3, and the mouse bindings (a click gives the
  keys to the live pane under it; wheel and drag reach only the active pane's
  program, and only if it asked for the mouse; no copy mode, menu or border drag).
- Wakes: the hooks above and the stage's `pane-died` send the rail SIGUSR1, armed
  by `Open` after its handler is installed. A wake carries no data and wakes
  coalesce.
- The conf's own `pane-died` hook hands a dead `%0`'s status to the terminal's
  client with `detach-client -E` (a missing status as 1), so the launcher gets the
  rail's exit status even when the rail dies before `Open`, and tmux prints nothing.
  `exit-unattached` and `destroy-unattached` are armed by `after-new-session`.
- `Paint`: full path, `("#rrggbb", "#rrggbb", true)`: divider colours by active
  pane, `window-style fg=quiet`, `window-active-style fg=terminal`. Plain path,
  `("brightyellow", "", false)`. Not called under `NO_COLOR`.

### `internal/agentcli`

- No command: today's tty refusal, then today's configuration check, then tmux from
  `PATH`, then `Launch`. `command` is `exec.LookPath(os.Args[0])` made absolute,
  symlinks unresolved, so later visits follow `skid update`. A nonzero rail status
  prints today's `session browser ended with an error` and exits 1. Launcher
  errors: `plan/f2-stage.md` §4.
- `rail`: today's browser body, the one `Apply` included; `compositor.Open`;
  `sessionui.Run(ctx, client, input, output, compositor)`.
- `stage`: decodes `--ref` once, calls `terminalclient.RunStage`, writes the outcome
  line to a temporary name and renames it, then exits `StageEnded` for
  `terminalclient.ErrClientEnded`, else 1. A setup failure writes its line and
  exits 1; a publication failure exits 1.
- `enter`: unchanged.

### `internal/terminalclient`

```go
// RunStage is Run without the chord decoder: every input byte reaches the session.
// The first Resize is immediate; a later one is sent after the size has held for
// settle, and never equal to the last sent.
func RunStage(ctx context.Context, client *fleetclient.Client, ref fleetclient.Reference,
	input, output *os.File, settle time.Duration) Result
```

- `Run` keeps its signature and behaviour.
- `ErrClientEnded` wraps an `Error` frame with code `ReconnectRequired` received
  after `Hello`, and nothing else; its text is the frame's message.
- The stage's `settle` is the smallest value that passes A9 and A10: tmux can
  resize a pane before its resize hook ends the visit. An end cancels a pending send.

### `internal/sessionui`

- `Run` takes the `*compositor.Client`; there is no browser without one.
- New `rail.go`: the visit (reference, name, host, id), who has the keys, a pending
  key movement, the key and paste filter, applying `State`, the hints. Touch
  points: `session.go` (`enter`, `adoptCreated`, `Update`, `Run`); the size checks
  in `keys.go`, `create.go` and `session.go`, which read `compositor.StageSize` of
  the window; `layout.go` (`▏`, `▐`); the keys map.
- The rail passes `ref.Encode()` to `Visit` and keeps the attempted reference with
  the returned id. On an error from `Visit` it reads `State`: a live visit with
  that id is adopted; an ended one is applied as any end; with no such id nothing
  started: the old visit stays if `State` still shows it live, else the table
  returns and says so. If `State` itself fails the rail exits 1.
- A visit's end is applied once, when `State.Visit` is the current id and `Live` is
  false: `rail` is the rail's own; `narrow` and `exit` take their notices.
- A change of `Returns` means the session had the keys since the last read:
  decision c and the map's closing apply even if the keys are back.
- Reads keep their lanes and ticks during a visit.

### Outcome line

Written by `stage`, mapped row by row in `plan/f2-stage.md` §1, which rules on
any case this paragraph omits: each part of `result.Err`, one level of `Unwrap()
[]error` deep and in order, joined with `; ` (decision e's sentence for
`ErrClientEnded`, `fleetview.ErrorMessage` for a typed dial failure, else its
text; `terminal detached` for nil), then `; notifications unavailable` when the
acknowledgement failed. Sanitized with `attention.SingleLine`, then cut to 512
bytes on a rune boundary. The rail prints it after `<s>: `; a missing or empty line
reads `attachment ended; no cause was recorded; work may continue`.

## Rules that bind the build

- [Rules](../rules.md) and [AGENTS](../../AGENTS.md) bind. No production seam beyond
  decision h, no retained test, no environment-variable switch. The plan commit
  amends AGENTS's isolation rule to admit a private `-S` socket under scratch, and
  its `kill-server`, beside `-L`; the harness runs after that.
- One owner per fact: geometry and every compositor control call in
  `internal/compositor`; the outcome sentence in `agentcli`; the rail's copy in
  `sessionui`; the visit in `terminalclient`.
- A reference is bound to a stage process for that process's whole life.
- Keys move only by his key or click, or because a visit ended.
- Apart from decision b's read, the rail sends a gateway nothing the table would not
  send for the same key.
- Only the root integrator edits `docs/architecture.md`.

## Slices

| Slice | Paths | Delivers |
| --- | --- | --- |
| S1 visit | `internal/terminalclient/` | `RunStage`, `ErrClientEnded` |
| S2 compositor | `internal/compositor/` | the package above |
| S3 commands | `internal/agentcli/`, `cmd/skidbladnir/` | the launcher, `rail`, `stage`, help text |
| S4 rail | `internal/sessionui/` | rail mode, its copy, the deletions |
| S5 docs (root) | `docs/`, `AGENTS.md`, `README.md`, `site/`, `scripts/media/` | `plan/f3-docs.md` |

Order: S1 and S2 each build alone; S3 and S4 land together, since `rail` calls the
new `Run`.

## Red, green, refactor

1. Harness, temporary, deleted before the pull request, run on the mac. Everything
   runs under `sandbox-exec` with a profile that denies `127.0.0.1:7341`, and step
   0 proves the denial before any `skid` code starts: the fleet client clones its
   transport at package load, so nothing inside the process can stop the browser's
   `Apply`. The harness is a test in `cmd/skidbladnir` whose `TestMain` dispatches
   the launcher, `rail` and `stage` to the real `run`; a scratch symlink stands in
   for the installed command. The parent test serves a gateway in process on
   `127.0.0.1:0` over a private inner tmux server (decision h), behind a proxy that
   records every request and `Resize` and can drop one terminal connection, and
   writes a scratch `--config` whose `fleet.json` lists it as another machine. The
   compositor runs on a pty the test owns and answers as a terminal. Each session
   records every byte it receives. Seeds: `rail-scratch/proto/{fixture,glass,
   frames}.sh` and the commands in `review2/opus-recheck.md`.
2. Red: A1 to A17 written first. On the base A2 to A9, A13, A17 and A10's rail
   clauses fail for want of a rail; A1, A10's direct clause, A12, A14 and A15 hold
   on both; A16 exists only on the branch.
3. Green: the slices in order.
4. Refactor to the rules: no `nil` rail, no screen scraping, no duplicated constant.
5. Review: codex GPT-6.2-Astra and GPT-6.2-Sol at xhigh through skid on the `work`
   profile, on the diff; Claude subagents on style. Then delete the harness and
   decision h's file, and record the human checks.

## Acceptance

Isolated. Steps are paced on the observed result of the one before; only A7 sends a
burst. "No request" excludes the browser's own ticks.

- A1 At launch the table equals the base's text frame at 160×45, 120×36, 100×30
  and 10×5; `q` restores the terminal, exits 0 and prints nothing more than the base.
- A2 `enter` at 120 and 160 columns: the rail in columns 0–27 with `▐` on the shown
  row, the divider in 28, the session beyond; the first `Resize` is 91 and 131
  columns and no other follows; typed bytes, `ctrl-b` and a bracketed paste holding
  the bytes `0x1d 0x64` reach the session intact, and the connection stays open.
- A3 `ctrl-]`, then `enter`, then `ctrl-]` twice: the session receives no key and no
  `Resize`, no connection opens, and the lip and hints follow the keys. A program
  that asked for focus reports receives exactly those.
- A4 Beside a visit, each of `x s n N c T r g u i q h l`, space, `escape`, the
  arrows, `ctrl-c` and a paste draws the discard line, sends no request and no byte
  to a session, and leaves the next `j` working.
- A5 Every row of the corpus (`plan/f1-rail.md` §7), from its stated start, ends
  with the view, the cursor, the keys, the stage's session, the ordered list of
  connections, the gateway writes and the session bytes its row states.
- A6 `enter` on another row: the new session's first frame within 150 ms at p95 of
  30; no byte typed after `enter` reaches the old session; its connection closes
  within 2 s.
- A7 `ctrl-] j enter fix` as one write, at 20 start timings: exactly one switch, to
  the next row; no byte of `fix` reaches the old session or acts in the rail or the
  table; the new session receives a suffix of `fix`, possibly empty.
- A8 `ctrl-b d`: the table, with decision e's sentence in ink; the inner session
  lives; `j` moves at once. `ctrl-b x` then `y` on a one-pane session: the same,
  and the row leaves. The proxy dropping the connection: the outcome in ember and
  one read of each host. A stage killed while the rail has the keys, then `q`
  before the wake: skid quits.
- A9 `ctrl-d`, with the keys map open and without: the table, its notice, the map
  closed. At 100 columns `ctrl-]`: the table and the narrow notice. Narrowing 120
  to 100 with the rail's keys: the same. In all three the trace shows the entry's
  `Resize` and none after it, to the connection's close.
- A10 Resizes with the session's keys, at constant height. A settled burst sends
  one `Resize` if the stage's final size differs from the last sent and none
  otherwise, and the rail stays at 28: 120 to 130, one; 120 to 121 to 120, none;
  100 to 129, none, since the session stays at 100; 100 to 138, one, to 109; 120 to
  100, one. A direct `skid enter` sends what the base sends.
- A11 Closing the terminal, `kill -9` of the tmux client and SIGTERM to the
  launcher, each during a visit, and `q` in the table: the launcher exits, and
  within a second no compositor or rail process and no `DIR` remain; no stage
  process within seven; the inner sessions accept a new visit. A rail that exits 5
  makes the launcher print its line and exit 1.
- A12 A terminal focus-out and focus-in move no mark and no key.
- A13 A click on the stage gives it the keys; a click on the rail takes them; a
  wheel over the rail changes nothing; a wheel reaches a session that asked for the
  mouse.
- A14 Cursor movement opens no connection and sends no `notifications/handled`.
- A15 `skid enter`: banner, both chords, sentences, exit codes and resizes as the base.
- A16 A `tmux` stub reporting 3.7, and one reporting nonsense: the refusal, exit 1,
  nothing started, no `DIR`. One reporting 3.10, passing every other call to the
  real tmux, starts.
- A17 With the cursor moved off the shown row and the shown row gone from the view,
  `ctrl-]` keeps the view, selects the shown lifetime as a guest and opens no
  connection. A creation from the table at 120 columns enters the split; one that
  returns when the stage would be too small stays in the table with today's notice.
  Two `skid` at once have separate servers, and after one ends the other still
  switches, types and resizes.

Check, not acceptance: `scripts/check verify host`.

Human, after the merge, `NOT_RUN` until run, each an entry in
`unrun-acceptance.md`: Ghostty dark and light, with the divider, the dim on both
sides and `NO_COLOR`; Claude Code and Codex on the stage (paste, wheel, copy,
`ctrl-b [`); the phone attached to the shown session; a run on dev-server over ssh;
a visit to a row with a pending ready.

## Deletions

`internal/sessionui`: `attachment`, its `tea.Exec`, `attachedMsg`,
`model.attachment` and its key and paste guards; `cancelObservations` and the
attachment clause of `observing`; the fence of reads in `enter`; the colour-probe
refusal of `enter`, `T` and creation; `invalidateAccess` on a visit's failure; the
chord line of the keys map; the `terminalclient` import. Kept: the presentation
resets of `enter`, mutation fencing, the usage reveal's reset, search cleanup.
`docs/issues/bubbletea-return-resize-race.md`, with its links.

## Rollout

One pull request `rail`: plan, visit, compositor, commands and rail, docs. Each
step waits for the owner's word. One release; `skid update` on each computer; no
phone build. Rollback is the previous release.

## Trade-offs taken

- A visit from the split is 29 columns narrower for every client of that session,
  the phone included, until another client is used.
- A switch costs one blank frame and a process start.
- A second tmux server and a private directory live while the browser runs; every
  key is re-encoded once more; a `kill -9` of the launcher leaves both.
- The browser needs tmux 3.8, which stock Ubuntu 24.04 does not ship, and over ssh
  it needs the terminal's terminfo on that host. A host whose service definition
  changes its tmux path to gain a newer tmux needs a drain at that update.
- A literal `ctrl-]` cannot be typed into a session from the browser; a pasted one
  arrives.
- `escape` and `ctrl-c` do not reach an agent while the rail has the keys.
- Text typed into the rail by mistake moves the cursor on its `j`, `k` and tabs, and
  an `enter` then enters that session; what is typed next goes there.
- After any end the table's keys act at once: text typed during an unexpected end
  can create, interrupt or close a session, or quit skid, as today.
- A detach, a session's end and three gateway faults read alike in the notice.
- Keys typed in the instant of a switch are discarded, and keys typed before a new
  visit is ready are held by the terminal; `ctrl-c` there ends the visit, as today.
- A visit in the split follows a window resize one settle interval late.
- The dim is partial and exists only on the full palette path.
- Narrowing the window under 110 columns while the rail has the keys ends the visit.
- The rail and the table differ in their movement keys. A view that excludes the
  shown row shows no mark for it.
- The terminal's own mouse selection spans the rail.
- A browser open across `skid update` starts later visits with the new release.
- A visit's access rejection reaches the rail as a sentence; its host is fenced one
  read later.
- A killed visit can outlive its pane by a few seconds while it joins its
  acknowledgement.
- A terminal that answers tmux's two size queries out of order types the reply into
  the pane with the keys. Ghostty did not in the owner's run.
