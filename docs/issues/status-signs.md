# Status signs

Status: plan, settled with the owner on 2026-11-23 and 24. Nothing is built and no
contract has changed. Mock of both clients:
https://claude.ai/artifact/SpMcwBnkwNexPcBgQBd7Ff. Terminal demo:
`python3 ~/Documents/code/status-signs-scratch/signs.py`. The worked content behind each
section (every string, file, line and check) is in
`~/Documents/code/status-signs-scratch/plan/`: `f1-host.md`, `f2-browser.md`,
`f3-phone.md`, `f4-docs.md`, the feasibility run `spike.md` and the adversarial
`review.md`. Where they differ from this file, this file rules.

Problem: status reaches the owner as words. The browser spends a column of 8 to 30 cells
on them, as wide as its widest label, so one row turning `interruption shown` shifts
every later column on every row. The phone card spends a line on the same words. "Who
needs me" is a reading task on both.

Impact: trailing columns drop at ordinary widths; a re-read flickers words on both clients
(`checking`, `last observed:`); a menu over an error is queued but reads `menu open`; a
request hides the notice behind it on every status surface.

Evidence, at v0.29.1 (83178fe):

- The status column holds 8 to 18 cells, 30 in needs input; 8 are reserved for `checking`
  (`internal/sessionui/layout.go:141-148`).
- The queue mark `◼` U+25FC is absent from JetBrains Mono, the phone terminal's font, and
  carries the Emoji property.
- The phone mutes a machine's cards and prefixes `last observed:` after each of its own
  writes until the next read (`InventoryState.Superseded`), exactly as for a failed read.
- With names 28, provider 6, profile 12, model 20 and machine 16 cells wide, model fits
  from 79 columns and machine and path from 107 once the status column is gone; today
  machine and path need 117 to 127.

## Goal

One sign per session, in one cell before its name, on both clients, from one host verdict.
The words stay only where attention already rests.

## Non-goals

- No change to observation: the detector, `TerminalStatus`, `wait --state`, guarded send,
  notification causes and bodies, the hub.
- No sign column in the CLI. No sections, pins or snoozes in needs input
  ([attention dispositions](attention-dispositions.md)).
- No motion, drawn glyphs, second typeface, phone legend or `contentDescription`.
- No change to `NO_COLOR`: the browser keeps stripping every style.
- The phone's second owner of ready eligibility stays
  ([phone ready eligibility](phone-ready-eligibility.md)). Screenshots and media are not
  re-shot ([stale demo media](stale-demo-media.md)).

## Decisions

The owner's. Each is closed.

1. One sign in one cell; no second lane. A browser row and an `all` or group card hold
   no status words.
2. Nothing moves, on either client.
3. Working is `▸`. Interruption is a grey `■`. Unknown is `—`. One `?` covers all five
   request kinds. Idle is `·`. A plain terminal is `$`. Ready is `●` on both clients.
4. When work continues under a request, error, interruption or menu, the sign carries a
   double frost line.
5. A notice outranks a menu.
6. A host whose read failed reads `—` on every row.
7. A re-read of a healthy host holds sign, ink and queue place, on both clients.
8. Needs input is one flat list: request, error, ready, interrupted. The `needs input`
   tab wears its first row's sign, on both clients.
9. The browser's `?` keys screen carries a legend.
10. The wire carries the sign only: `tone`, `queue` and `queueNotice` are cut, and the
    cutover window that costs is accepted.
11. The phone sets the same glyphs in its bundled Data face.
12. Tests are temporary and isolated. One test-only export may admit a private tmux
    server. The owner's look comes after the merge.

## Target behaviour

| Sign | Glyph | Ink | When |
| --- | --- | --- | --- |
| request | `?` U+003F, bold | Ember | interaction permission, question, confirmation, setup or input |
| error | `!` U+0021, bold | Ember | notice error, no request |
| ready | `●` U+25CF | Moss | client overlay: host sign idle and the hub's pending ready |
| interrupted | `■` U+25A0 | Muted | notice interrupted, no request |
| menu | `≡` U+2261 | Muted | interaction menu, no request or notice |
| working | `▸` U+25B8 | Frost | activity starting or working, none of the above |
| idle | `·` U+00B7 | Muted | idle, interaction none, notice none |
| unknown | `—` U+2014 | Muted | any other local agent; a failed sample; every row of a failed host |
| terminal | `$` U+0024 | Muted | no local agent in front |

Every glyph is one cell, has no Emoji property, and is in MesloLGS NF, Menlo and both
bundled JetBrains Mono builds. `● ■ ≡ · —` are East Asian Ambiguous.

- **Work line.** `workContinues` holds when activity is `working` under a request, error,
  interruption or menu. The sign then carries a double Frost underline. `starting` under
  a claim carries none, as today's `work continues` detail. The tab's sign carries none.
- **Ready.** A client draws `●` over host sign `idle` while the hub holds a pending ready.
- **Held.** While a healthy host is re-read (browser: checking; phone: REFRESHING after
  its own write), every row keeps sign, ink, work line and queue place.
- **Stale.** Every row of a host whose read failed reads `—`, Muted, with no work line,
  out of the queue. Its words read `<label> · stale`.
- **Needs input.** Rows signed request, error, ready or interrupted, in that order, then
  each client's stable session order. The tab shows the first row's sign in one reserved
  cell, blank when the list is empty.
- **Words.** Browser: the cursor row's caption and the details fold. Phone: needs-input
  cards and the session sheet. Also the CLI and notifications. Nowhere else.

## Contract

Flow: `TerminalStatus` → `api.PresentStatus` (host: sign, `workContinues`, words) → the
wire, or `fleetview.ProjectStatus` in Go → client overlay (ready, held, stale) → the
client's typeface (glyph and ink per sign).

| Concern | Owner | Where |
| --- | --- | --- |
| Which sign, the words, whether work continues | host | `internal/api/status.go` |
| The ready and stale overlays; queue rank | each client | `internal/fleetview`; `ProductModel.kt` |
| When a row is held | each client | `internal/sessionui` (`rowAttachable`, `heldReady`); `ProductModel.kt` |
| Glyph and ink per sign | each client | `internal/sessionui/view.go`; `Theme.kt` |

### Wire

```text
StatusPresentation = {sign: request | error | interrupted | menu | working | idle |
  unknown | terminal, workContinues: boolean, label, detail?, evidence?}
```

Inventory and creation rows carry it. `sign`, `workContinues` and `label` are always
encoded (no `omitempty` on `workContinues`: the phone requires it). `ready` is never on
the wire. Example:
`{"sign":"request","workContinues":true,"label":"needs answer","detail":"error shown · work continues","evidence":"inferred from terminal"}`.

### Host: `api.PresentStatus(status, localAgent)`

First match wins. R is `Interaction.RequestText()`; N is `Notice.Text()`. "Rest" means
activity idle, starting or unknown. Every local-agent row with source terminal carries
evidence `inferred from terminal`.

| Facts | Sign | wc | Label | Detail |
| --- | --- | --- | --- | --- |
| source unavailable | unknown | no | `status unavailable` | |
| not a local agent | terminal | no | `terminal` | |
| request; no notice; rest / working | request | no / yes | R | — / `work continues` |
| request; notice; rest / working | request | no / yes | R | N / N `· work continues` |
| notice; interaction none or unknown; rest / working | error or interrupted | no / yes | N | — / `work continues` |
| notice; menu; rest / working | error or interrupted | no / yes | N | `menu open` / `menu open · work continues` |
| menu; no notice; rest / working | menu | no / yes | `menu open` | — / `work continues` |
| starting; no claim | working | no | `starting` | |
| working; no claim | working | no | `working` | |
| idle, interaction none, notice none | idle | no | `idle` | |
| anything else | unknown | no | `status unknown` | |

Detail facts keep one order: N under a request, `menu open` under a notice, then
`work continues`. `Reason` and its `; open the terminal to inspect` suffix are unchanged.
This yields 46 classes over the 336 valid fact sets (26 today); only a notice over a menu
changes its label.

```go
type StatusSign string // SignRequest … SignTerminal; SignReady is the clients' alone
type StatusPresentation struct {
	Sign          StatusSign `json:"sign"`
	WorkContinues bool       `json:"workContinues"`
	Label         string     `json:"label"`
	Detail        string     `json:"detail,omitempty"`
	Evidence      string     `json:"evidence,omitempty"`
	Reason        string     `json:"-"`
}
```

`StatusTone`, `StatusQueue` and their constants are deleted.

### Go clients: `internal/fleetview`

```go
type StatusView struct {
	Sign          api.StatusSign
	WorkContinues bool
	Label, Detail string // the host's, under the overlays
	State         string // Label, then Detail, then the host's evidence, joined by " · "
	Reason        string
}
func ProjectStatus(session fleetclient.Session, fresh, ready bool) StatusView
func QueueRank(sign api.StatusSign) int // request 0, error 1, ready 2, interrupted 3, else -1
```

| Overlay | When, in this order | Sign | wc | Label | Detail |
| --- | --- | --- | --- | --- | --- |
| stale | `!fresh` | unknown | no | `<label> · stale` | host's |
| ready | host sign idle and `ready` | ready | no | `ready` | |
| fresh | otherwise | host's | host's | host's | host's |

`State` keeps the evidence under every overlay: `ready · inferred from terminal`,
`needs answer · stale · work continues · inferred from terminal`. `QueueCategory` and
`StatusView.Tone`, `.Queue`, `.QueueDetail` are deleted. `skid list` and `skid info`
print `State` (`internal/agentcli/run.go:709`, `:792`); `:709` passes `true` for
freshness, as `:791` does: a failed peer carries no sessions, so the CLI never reaches
the stale overlay.

### Browser: `internal/sessionui`

Columns here are line columns; the screen is one cell to the right (`view.go:120`).
`f.width` is the terminal width − 2.

- **Row.** 0 lip (`▌` Gold on the cursor row), 1 space, 2 sign, 3-4 spaces, 5 name (at
  most 28), then group (needs input only), provider, profile, model, machine and path as
  today. `measureColumns` and `tableLines` start at 5 and read no status, so no column
  depends on any row's status. Names are bold when selected or queued. A stale row draws
  `—`, a quiet name and no group or agent metadata. A checking, held-ready or guest row
  draws as a fresh one. No row holds a status word, `checking`, a note or `◼`.
- **Sign.** One table, `signSpan(sign) span`, serves rows, the tab and the legend, and
  panics on an unmatched sign. `span` gains `underline`; `renderLine` adds
  `UnderlineStyle(ansi.UnderlineDouble).UnderlineColor(c)`, where `c` is the frost
  pigment on the full path and otherwise `frostInk`, a `color.RGBA{0x78,0xA9,0xC6,0xff}`
  that the plain `frost` style is built from. Under `NO_COLOR` the frame is stripped as
  today and the line is gone.
- **Status view.** `statusView(row)` is `fleetview.ProjectStatus(row.Session,
  m.rowAttachable(row), row.Available && m.notificationQualified &&
  m.notificationReady(row.Session) || m.heldReady(row))`. The local ready override and
  the needs-input `· notice` suffix go. One `m.queue()` gathers rows with
  `QueueRank >= 0` and stable-sorts by rank; the needs-input view, the tab, next and
  previous, and bold names read it or `QueueRank`. One `rowChecking(row)` replaces the
  three `hostChecking || heldReady`.
- **Caption.** A styled `line` that `rule` and `weeklyQuota` take:
  `target   status[ · detail][   place][   position]`, at most `f.width − 5` cells (73
  at 80 columns, 93 at 100). `status` is `Label`, then ` · checking`, then
  ` · left the queue` or ` · left this group` for a guest. Status never drops: the
  name's tail gives way first. Then the host's detail, whole or not at all; then place,
  only when the table dropped it; then position, only when rows are hidden. The quota
  takes what room is left, as today. Label takes the sign's role (stale: Muted italic);
  the notes are Muted italic; detail Muted; place and position quiet.
- **Tab.** The needs-input tab is always 15 cells: its word, a space and one sign cell
  from `m.queue()`'s first row, blank when the queue is empty.
- **Details fold.** `state` is `State`; a checking row keeps its words and reads
  `availability  checking`; a stale row reads `availability  unavailable`.
- **Keys screen.** One legend after the key columns and before the shortcuts, glyphs
  from `signSpan`, words quiet, packed by whole entries two spaces apart into lines of
  at most the fold's width: `? request`, `! error`, `● ready`, `■ interruption`,
  `≡ menu`, `▸ working`, `· idle`, `— unknown`, `$ terminal`,
  `underlined: work continues`. `shift-tab previous mark` becomes `shift-tab previous`
  and moves beside `tab next needing input`.
- **Failed host.** With rows retained the notice reads
  `<host> unavailable (<code>); its listing is stale`; with none it is unchanged.
- **Media tape.** `scripts/media/desktop.tape:32` waits for `▌ api `; its pattern
  follows the new row.

### Phone: `android/app/src/main/java/dev/niels/skidbladnir`

```kotlin
@Serializable internal data class StatusPresentation(
    val sign: SessionSign, val workContinues: Boolean, val label: String,
    val detail: String? = null, val evidence: String? = null,
)
/** Declared in needs-input order: the first four queue. The host never sends Ready. */
@Serializable internal enum class SessionSign { Request, Error, Ready, Interrupted, Menu, Working, Idle, Unknown, Terminal }
internal data class SessionStatusContent(
    val sign: SessionSign, val workContinues: Boolean, val label: String,
    val detail: String?, val evidence: String?, val secondary: String?,
) { val queued: Boolean get() = sign <= SessionSign.Interrupted }
internal fun sessionStatusContent(machine: MachineState, target: SessionTarget): SessionStatusContent
```

Each `SessionSign` value carries its lowercase `@SerialName`. `detail` is the host's, or
the active command when the row has no evidence and no connection, as today; `secondary`
is `notifications unavailable` when the hub read failed, as today.

| `machineAvailability` | Sign | Work line | Label | Queued |
| --- | --- | --- | --- | --- |
| Ready, Refreshing | host's; `idle` with the hub's ready reads `ready` | host's | host's; `ready` | sign ≤ interrupted |
| Stale, AuthRequired, IdentityChanged | `—` | none | `<label> · stale` | no |
| Reading, Unavailable | no cards | | | |

The `when` over `MachineAvailability` names every variant.

- `SkidbladnirController.kt:1208` stops gating the hub's ready on `canMutate`; the
  overlay above is its only gate. `canMutate` still gates every action, so a queued
  REFRESHING card is listed but inert until the read lands.
- `VisibleSession` carries its `SessionStatusContent`. `dashboardItems` filters needs
  input by `queued` and sorts by `sign`, stable. The strip's sign is the first item of
  `dashboardItems(machines, NeedsInput)`, whatever view is selected.
- One table in `Theme.kt`, `signFace(sign)`: glyph, ink, bold. One composable,
  `StatusSign(sign, workContinues)`, serves the card and the sheet. The work line is two
  Frost rules drawn under the glyph: tops at baseline + 1.5t and + 3.5t, each t thick,
  t = font size / 12 in dp.
- **Card.** The sign in the Data face at 12sp, then the dwarf's name in its Display face
  as today. In needs input the host's label follows the sign, bold in its ink, then
  ` · <detail>` Muted, wrapping in full. A local plain terminal reads `$` and its
  command, Muted, one line; a remote one `$` alone; a stale one `—` alone.
  `ActivityFacet`, its inline placeholder, the label path and the queue-reason wrap go.
- **Sheet.** The sign, then `label · detail · evidence` in the sign's ink.
- **Strip.** `needs input`, a space, and one Data-face cell holding that sign in its
  ink, else U+00A0; `displayLabel()` stays `needs input`.
- The markers (`STALE`, `REFRESHING`, `AUTH REQUIRED`, `IDENTITY CHANGED`), machine
  notices and `notifications unavailable` are unchanged. Comments that describe the
  facet, tones or queue categories are rewritten with the code they sit on.

## Rules that bind the build

From [rules](../rules.md), as they apply here:

- "Releases are hard cutovers: no legacy paths, fallbacks, shims, dual decoders or
  migration code." "A field needs a current consumer."
- The host renders status copy; a client renders only its own outcomes (`ready`,
  `stale`, `checking`, the guest notes, the legend). No client compares rendered strings.
- Each client keeps one sign table. Nothing else maps a sign to a glyph or an ink.
- Colour never carries meaning alone: the nine glyphs differ in shape.
- Docs are edited in place, in this change. Only the root integrator edits
  `docs/architecture.md` and `docs/roadmap.md`.
- "Rebuild no harness and add no production seam." Two declared exceptions, both
  temporary and test-only: the admission export below, and assertions on the double
  underline, where the style is the meaning.

## Slices

Paths do not overlap. A slice acts only inside its paths.

| Slice | Paths | Delivers | Lands |
| --- | --- | --- | --- |
| G go | `internal/api/status.go`, `internal/fleetview/fleetview.go`, `internal/agentcli/run.go`, `internal/sessionui/**`, `scripts/media/desktop.tape`; temporary: `internal/sessions/*_test.go`, one line of `go.mod` | host projection, overlays, `QueueRank`, the browser | first |
| P phone | `android/**`; temporary: `android/app/src/test/**`, the Gradle test lines, a debug manifest | decode, overlay, card, sheet, strip | beside G |
| D docs | `docs/terminal-observation.md`, `docs/design-language.md`, `docs/codebase-map.md`, and by name `docs/issues/{attention-dispositions,unrun-acceptance,stale-demo-media,phone-ready-eligibility,status-sign-cutover}.md` | clauses T, D, M and I of `f4-docs.md` | drafted beside, landed after G and P, each quote checked against the built copy |
| R root | `docs/architecture.md`, `docs/roadmap.md`, `docs/issues/status-signs.md` | clauses A; integration; checks; the pull request | last |

Host and browser are one slice: `internal/agentcli` imports `internal/sessionui`, which
reads the deleted types in seven places, so neither compiles without the other. Inside
G the host comes first and is checked with `go build ./internal/api ./internal/fleetview
./internal/gateway`. Each slice starts from its designer's file. The branch's first
commit adds this file and its last removes it. D stages named files only: the working
tree holds other people's edits under `docs/issues/`.

## Red, green, refactor

Per [testing](../rules.md#testing): temporary tests at the integration boundary, run
against 83178fe and against the change, sensitivity reviewed, deleted before the commit.
A red is a failed assertion on a surface both versions have: an encoded document, a
terminal screen, CLI text, a rendered card. A test that names a new symbol cannot be red
and is not written.

G, one file, `internal/sessions/zz_signs_test.go`, grown from the feasibility run's
`spike_e2e_test.go`:

- A private `tmux -L` server (`TMUX` unset; `HOME`, `XDG_*` and a short `TMUX_TMPDIR` in
  scratch) whose panes run `scripts/media/stub`, built as `codex` and `claude`, over
  frames painted by `internal/agentcontrol/testdata/frames/paint.py`.
- The gateway wired in process as `serveStandalone` wires it, on 127.0.0.1:0, its tmux
  server admitted by `internal/sessions/export_zz_test.go`, the one test-only export.
- A second machine: one temporary handler that serves an inventory of every class,
  built with `api.PresentStatus` and `encoding/json`. The corpus yields 14 of the 46
  classes and none with a notice under a request or work under a notice.
- Clients: the real `skid list --config <scratch>` binary, and `sessionui.Run` on a pty
  read through `charmbracelet/x/vt`, with `TERM` and `COLORTERM=truecolor` set and the
  emulator answering the colour queries. No client directory holds the gateway's
  `fleet.json`; nothing dials 127.0.0.1:7341.
- Transitions: ready by repainting a pane's frame under the same process, with the hub
  observer wired; checking by holding the listener's accepts during a save; stale by
  closing the listener.
- It runs under a sandbox that denies port 7341, the owner's tmux directory and
  `launchctl`, as the feasibility run's `sandboxed` wrapper does.

| Check | Asserts | Red on 83178fe |
| --- | --- | --- |
| host table | all 336 valid fact sets through `api.PresentStatus` and `encoding/json` equal the host table | members differ |
| wire | acceptance 1 on the real gateway's document | `tone` present, `sign` absent |
| CLI | acceptance 15 from the real binary | no notice under a request; `menu open` over a notice |
| browser | acceptance 3 to 11 on the screen | a status column; `◼`; `checking`; `last observed:` |

P, under `android/app/src/test/`, reading the two documents the Go test writes at each
version:

| Check | Asserts | Red on 83178fe |
| --- | --- | --- |
| decode | every row of both documents decodes | green-only: the new document lacks `tone` there |
| queue | `dashboardItems` over a Fresh, a Superseded and a Stale machine: membership and order by tmux id | ready first; Superseded rows absent |
| card | `SessionCard` per sign in both views: acceptance 14 | two draws 300 ms apart differ; a host label in `all` |

The card test's call site drops `motionEnabled` when the card does; its assertions do not
change. Refactor until the code meets [rules](../rules.md) "Code shape". Then delete the
temporary test files, the admission export, the stub binaries and painted frames built
in scratch, the `x/vt` line in `go.mod`, the Gradle test lines and the debug manifest.
`scripts/media/stub` and the frame corpus stay.

## Acceptance

1. Every inventory and creation row's `status` has `sign`, `workContinues`, `label` and
   at most `detail` and `evidence`; never `tone`, `queue` or `queueNotice`.
2. All 336 valid fact sets project as the host table says.
3. Browser rows, in screen columns: lip at 1, sign at 3, name at 6. No row holds a
   status word, `◼`, `checking`, `last observed`, `left the` or a host label.
4. The sign cell is double-underlined exactly when `workContinues` holds.
5. Swapping one pane between two frames with equal footers, the new label the widest on
   83178fe, moves no column start and no tab.
6. Needs input lists requests, errors, ready, interruptions, ties in today's order; the
   tab's cell shows the first row's sign, blank when empty, and the tab is 15 cells.
7. With a host down: its rows read `—` without group or agent metadata and leave needs
   input; the notice reads `… ; its listing is stale`; the caption's status reads
   `<label> · stale`.
8. During a re-read: rows keep sign, work line and queue place; the caption shows
   `· checking`; the fold keeps `state` and reads `availability  checking`.
9. The caption is at most 73 cells at 80 columns and 93 at 100, always holds the label,
   and matches the grammar.
10. With `NO_COLOR=1` the stream sets no style and every sign and caption word is present.
11. The keys screen shows the legend, no entry split, and `shift-tab previous` beside tab.
12. The phone decodes the new documents; a document from 83178fe is a lost response, not
    a crash.
13. On the phone a Refreshing machine's rows keep their queue places and a Stale one's
    leave; needs input is ordered request, error, ready, interrupted.
14. A card shows the glyph and the dwarf's name; outside needs input no host label and no
    `last observed:`; in needs input the label and detail; a local terminal `$` and its
    command; Frost in the rows from baseline + 1.5t to + 4.5t exactly when
    `workContinues` holds; two draws 300 ms apart are identical.
15. `skid list` and `skid info` print `State`; the 24 classes with a notice under a
    request or a menu under a notice name both.
16. These find nothing in code or docs: `last observed:`, `queueNotice`, `ActivityFacet`,
    `◼` (outside the frame corpus), `StatusTone`, `StatusQueue`, `SessionStatusTone`,
    `SessionQueueCategory`, `QueueCategory`, `QueueDetail`, `previous mark`.
17. `scripts/check verify all` passes.

`NOT_RUN`, recorded in [unrun acceptance](unrun-acceptance.md):

- The owner's look in Ghostty and on the phone.
- The real `skid` browser binary and `skid serve`: the gateway address is the constant
  127.0.0.1:7341 and the browser command posts to its own machine there.
- The work line under an error or an interruption on the real gateway: the corpus has no
  such frame. The second machine's rows cover the drawing.
- The phone's strip (glyph, ink, reserved cell), its session sheet and the controller's
  ready gate: private composables behind a controller that needs the Firebase
  application. They are verified by reading the diff.

## Deletions

- Go: `api.StatusTone`, `api.StatusQueue` and constants; `StatusPresentation.Tone`,
  `.Queue`, `.QueueNotice`; `fleetview.QueueCategory`; `StatusView.Tone`, `.Queue`,
  `.QueueDetail`; `sessionui.statusRole`, `projectStatus`, `markColumn`, `statusColumn`,
  the `checking` reservation and row word, the row-borne guest note, the needs-input
  `· notice` suffix, the local ready override, the string `last observed: `, the
  freshness argument at `run.go:709`.
- Kotlin: `SessionStatusTone`, `SessionQueueCategory`, `sessionStatusColor`,
  `ActivityFacet` and its imports, `FACET`, `facetSize`, `detailExplainsQueue`,
  `showQueueReason`, the `motionEnabled` parameter of the card, the controller's
  `canMutate` gate on ready, the second status derivation per card.
- Found and left: `sessions.Config.SocketName` has no production setter; the
  remote-context flicker during REFRESHING (`Machines.kt:77-86`).

## Rollout

A [lockstep cutover](../architecture.md#lockstep-cutovers): the phone requires `sign`,
the installed phone requires `tone` and `queue`. Go clients, jarvis, the Mac
notification app and the hub read raw facts and work against either release. Every step
changes live state and needs the owner's approval in the turn that takes it.

1. Build the APK and have it ready.
2. Release, pin and verify as the [release](../architecture.md#release) runbook says;
   update every host.
3. Install the APK at once. Until then, for each updated host, the installed phone says
   it could not reach the machine over the Tailnet, marks its cards STALE until the app
   restarts and UNAVAILABLE after, opens no terminal there, and reads a create or shell
   it sends as unknown. A notification tapped in that window is marked handled on the
   hub and never served: tap none.
4. The owner's look in Ghostty and on the phone.

The change records these steps as `docs/issues/status-sign-cutover.md`. Rollback is
lockstep too: Android refuses an in-place downgrade, so the previous APK costs an
uninstall and a re-pair.

## Trade-offs taken

- A claim no longer stands alone in a dark lane: every row is lit, and a claim must
  outweigh its neighbours by ink, weight and height.
- Nothing shows that a host still answers; its notice and `—` say when one has stopped.
- A question pending on a failed host is not visible at a glance.
- The work line is a style. Under `NO_COLOR` it is gone and only the caption and the fold
  say `work continues`, for the cursor row alone; design-language §17 gains a clause.
  A terminal or tmux without styled or coloured underlines draws it single, in the
  sign's ink, or not at all.
- The request kind, `starting`, `unavailable`, a menu under a notice and `checking` are
  words only.
- Glyph, ink and queue rank live in two client tables, one in Go and one in Kotlin.
- `skid list` and `skid info` change text for 24 classes, and the widest state cell grows
  from 60 to 79 cells.
- The caption puts status and detail before the place, and the rule's weekly quota takes
  what is left: at 80 columns the path and the quota often leave the rule.
- At 80 × 24 the keys screen needs 12 lines against its cap of 11 and loses its colour
  note; with a notice showing the cap is 10 and the shortcuts line goes too.
- A held-ready row now shows `●` only while its sample is still idle. Today it reads
  ready whatever the host last said; the new reading is the truer one.
- AuthRequired and IdentityChanged machines read `—` like a failed read: neither can be
  read again until the owner acts.
- A queued card on a REFRESHING machine is listed but inert for up to one read.
- `?` and `!` are synthetic bold on the phone: its Data face is pinned at weight 500.
- The cutover window in Rollout, and a rollback that costs a re-pair.
- The temporary tests reach into `internal/sessions` to admit a private tmux server, and
  the host table test calls `api.PresentStatus` directly.
- The owner's look comes after the merge; a miss costs a second release.

Resolved when: the acceptance list holds on the merged code, the fleet runs the release,
the cutover issue is deleted, and the owner has taken his look or left it recorded in
[unrun acceptance](unrun-acceptance.md).
