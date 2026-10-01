# claude screen grammar

status: implemented in `internal/agentcontrol/claude.go`, whose comments cite
the sections below. qualified live for claude 2.1.286 on darwin and claude
2.1.284 on linux as §1 records; §2.6 rule 24 (theme setup) is qualified on both
versions, except §5 r1's narrow widths. the
[qualification](terminal-agent-control-qualification.md#terminal-observation-qualification)
holds the evidence and its limits, and [the spec](terminal-observation.md) owns
the status contract this grammar feeds.

sources: `S`, string and code inspection of the bundled javascript in the
executable's `__BUN` segment (minified identifiers are build-specific and serve
only as orientation); `D`, the public statusline and accessibility docs; live
runs of the stock tui against a local deterministic mock of the messages api. no
real screen, prompt, path, title or account datum was kept.

installed versions qualified:

| platform | tmux | claude | renderers run |
| --- | --- | --- | --- |
| darwin 25.4.0 arm64 | 3.7c | 2.1.286 (`~/.local/bin/claude` → `versions/2.1.286`) | fullscreen, classic, screen reader |
| linux: devbox (ubuntu, kernel 6.8, x86_64), arch (kernel 7.2.6, x86_64) | 3.4 (devbox), 3.7c (arch) | 2.1.284 (same layout) | fullscreen (the default without a `tui` setting); classic for idle, draft, work, a single question, Bash permission and tall panes |

## 1. frozen capability table

classes: `R` required-positive; `P` permitted unknown/none; `U`
upstream-unavailable in the qualified configuration; `X` excluded by spec.

`qualified` names the platforms where the family ran live against the installed
versions above, with a renderer or width in parentheses when only that part ran.
`capture` means authored rows painted through real tmux on that platform,
classified with the evidence dropped from the real capture (on darwin also by a
real byte cap): capture and parser mechanics only. `source-only` means `S`/`D`
and authored rows only. an `R` row is `NOT_RUN` on every platform it does not
name, and each such `NOT_RUN` is a blocker under spec §8
([darwin](issues/claude-observation-darwin-coverage.md),
[linux](issues/terminal-observation-linux-coverage.md),
[unruled dialogs](issues/claude-unruled-request-dialogs.md)). a `P` row's
`NOT_RUN` is informative.

| # | family / variant | class | result | rule id | qualified |
| --- | --- | --- | --- | --- | --- |
| 1 | generation spinner | R | working | `claude.activity.spinner` | darwin, linux |
| 2 | tool spinner | R | working | `claude.activity.spinner` | darwin |
| 3 | reduced motion (`●`, frozen timer) | R | working | `claude.activity.spinner` | darwin (classic), linux (fullscreen) |
| 4 | spinner at narrow width: details dropped; spinner row wrapped | R | working | `claude.activity.spinner` | darwin (drop); wrap source-only |
| 5 | spinner `⎿` child block: tip (wrapping, `spinnerTipLabel`), `Next: <task>`, expanded task list, compaction hint | R | working (S above the block) | `claude.activity.spinner` | darwin (wrapped tip); the others source-only |
| 6 | retry row `· Retrying in` | R | working | `claude.activity.retry` | darwin |
| 7 | retry with overload child `⎿  If it persists, check …` | R | working | `claude.activity.retry` | source-only: repeated 529s ended in an `API Error` row with no retry row |
| 8 | retry variants: stalled, no-response, low-priority, auto-mode check | R | working (no-response: hint only) | `claude.activity.retry` | source-only |
| 9 | interrupt hint in the mode row | R | working (conflict with a ready chevron) | `claude.activity.hint` | darwin |
| 10 | panel agent or workflow row with a running status | R | working | `claude.activity.background` | darwin (agent, workflow), linux (agent); nested rows source-only |
| 11 | panel row with status `idle`/`waiting`/`awaiting approval`, `N idle agents` row, paused workflow row, row without a readable status | P | no work claim; blocks idle | — | source-only (teams and named agents need more than an api-key session) |
| 12 | waiting row `✻ Waiting for …` as the slot row | R | working | `claude.activity.background` | darwin (agents, workflow-only); mixed source-only |
| 13 | work pill `N local agent(s)` | U | working | `claude.activity.background` | source-only: drawn only with the server-gated coordinator panel off, and the panel replaces it by default |
| 14 | process pill (2.4) | P | tolerated; no working claim | — | darwin (`N shell`) |
| 15 | unknown pill (MCP task, team, cloud, remote workflow, ultraplan, mixed, `↳ N background`) | P | blocks idle; no working claim | — | source-only |
| 16 | footer indicator `◆ <≤6 cells>` | P | tolerated; proves nothing | — | darwin |
| 17 | ready layout, default footer | R | idle | `claude.activity.idle` | darwin, linux |
| 18 | ready layout with a banner top rule (`--name`, `/rename`, `/color`, `--agent`) | R | idle; working as unbannered | `claude.activity.idle` | darwin (idle, colour); bannered working source-only |
| 19 | ready layout, custom multiline statusline | R | idle | `claude.activity.idle` | darwin (classic) |
| 20 | working with a statusline (hint suppressed) | R | working via spinner + loading chevron | `claude.activity.spinner` | darwin (classic) |
| 21 | loading chevron, no spinner/retry/hint | P | activity unknown | — | never observed |
| 22 | spinner/retry shape or hint with a ready chevron | P | activity unknown, `evidence_conflict` | `claude.activity.conflict` | source-only |
| 23 | screen-reader startup quiet (banner is the lowest row) | R | starting | `claude.activity.starting_sr` | darwin |
| 24 | fullscreen/classic startup | U | the REPL mounts ready; no startup structure | — | darwin |
| 25 | screen reader: working (hint) / idle | R | working / idle | `claude.activity.hint_sr` / `claude.activity.idle_sr` | darwin |
| 26 | screen reader: draft during work, retry, fresh session, statusline | P | activity unknown | — | darwin |
| 27 | screen reader: pill in the mode row | P | process tolerated | `claude.activity.idle_sr` | darwin (`1 shell`) |
| 28 | screen reader: wrapped mode row or completion row | P | unknown | — | darwin |
| 29 | screen reader: announcements below `$` | P | unknown until repaint | — | source-only |
| 30 | screen reader: agent/workflow panel | P | unknown | — | source-only (rendering unknown) |
| 31 | permission: Bash, Write, Edit, wrapped at 26/40/60/80 | R | permission | `claude.permission.dialog` | darwin, linux (Bash at 100 columns) |
| 32 | permission: MCP tool | R | permission | `claude.permission.dialog` | darwin |
| 33 | permission: dynamic workflow (`Run a dynamic workflow?`) | R | permission | `claude.permission.workflow` | darwin (100/60/40) |
| 34 | permission: Fetch, notebook, subagent- or plugin-sourced | R | permission | `claude.permission.dialog` | source-only |
| 35 | permission, screen reader | R | permission | `claude.permission.sr` | darwin |
| 36 | question: single, free-text focus, multi-question, multiSelect, wrapped hint, chat-row focus | R | question | `claude.question.form` | darwin, linux (single) |
| 37 | question with countdown `auto-continue in Ns · any key to stay` or plugin notice row | R | question | `claude.question.form` | source-only |
| 38 | question with option previews, chat focus | R | question | `claude.question.preview` | darwin (single question, including previews that draw their own boxes); the two-question hint source-only |
| 39 | question submit review | R | question | `claude.question.review` | darwin |
| 40 | question, screen reader | R | question | `claude.question.sr` | darwin |
| 41 | plan approval with plan, wrapped at 80/60/40 | R | confirmation | `claude.confirmation.plan` | darwin |
| 42 | exit plan mode without plan | R | confirmation | `claude.confirmation.exit_plan` | darwin |
| 43 | enter plan mode dialog (only under an `ask` rule naming `EnterPlanMode`) | R | confirmation | `claude.confirmation.enter_plan` | darwin (80/40) |
| 44 | mcp elicitation form | R | input | `claude.input.elicitation` | darwin (classic) |
| 45 | mcp elicitation url mode | U | input | `claude.input.elicitation` | source-only: gated, no dialog in an api-key session |
| 46 | transcript viewer over a waiting dialog (`dialog waiting · Showing detailed transcript · …`) | R | input | `claude.input.dialog_waiting` | source-only |
| 47 | setup: trust, api key, theme onboarding, security notes | R | setup | `claude.setup.<variant>` | darwin (2.1.286, unnumbered theme), linux (2.1.284, numbered theme; §5 r1) |
| 48 | setup: login method | R | setup | `claude.setup.login` | darwin |
| 49 | setup: project mcp server (single / several) | R | setup | `claude.setup.mcp_server` / `claude.setup.mcp_servers` | darwin (single at 80/40; several at 80/40/26) |
| 50 | setup: bypass-permissions warning | R | setup | `claude.setup.bypass` | darwin, linux |
| 51 | setup: settings error (invalid settings file) | R | setup | `claude.setup.settings_error` | darwin (80/40) |
| 52 | setup: auto-mode default offer | R | no rule: unknown/unknown | — | source-only (§5 c15) |
| 53 | setup/input: rate-limit options, consumer-terms update, trial expired | R | no rule: unknown/unknown | — | source-only (§5 c15) |
| 54 | setup, screen reader: theme, api key, security notes, trust | R | setup | `claude.setup.sr` | darwin; bypass, mcp and login source-only |
| 55 | provider announcement modal | P | interaction unknown | — | source-only |
| 56 | digit band above the composer (2.3) | P | composer unknown | `claude.composer.band` | source-only |
| 57 | usage-limit wait, in the pinned column (2.3, 2.5) | P | never idle; interaction and composer unknown; activity from spinner, hint or panel | `claude.limit.wait` | darwin (the pinned column's placement); the wait itself source-only (§5 c17) |
| 58 | menus: model, settings, theme, resume, help, /btw | R | menu | `claude.menu.<variant>` | darwin |
| 59 | menus: transcript viewer, agents view | R | menu | `claude.menu.transcript` / `.agents_view` | darwin |
| 60 | menu whose hint is obscured (narrow or short pane) | P | unknown/unknown | — | darwin |
| 61 | viewed subagent (viewed glyph on a non-main panel row) | R | menu; activity from the panel | `claude.menu.subagent` | darwin, linux |
| 62 | panel focus footer `↑/↓ to select …` | P | void surface: unknown | — | darwin, linux |
| 63 | left-labelled top rule (`── History N/M ──…`) | P | interaction and composer unknown | — | source-only |
| 64 | other panes (/mcp, /agents, /permissions, /effort, task detail) | P | unknown unless a menu rule matches | — | source-only |
| 65 | interruption row `⎿  Interrupted · What should Claude do instead?` | P | notice none (transcript row) | — | darwin |
| 66 | error row `⏺ API Error: …` | P | notice none (transcript row) | — | darwin |
| 67 | footer notifications: effort (right-aligned), tmux notices | P | `notification` rows (2.0) | — | darwin (tmux notices drawn because the pane's own `TMUX` is set) |
| 68 | composer empty / draft / blocked / unknown | R | §3 | `claude.composer.*` | darwin, linux |
| 69 | quoted chrome in the transcript | R neg | ignored | — | source-only (authored) |
| 70 | historical completion/waiting/interrupt/error rows | R neg | ignored | — | darwin, linux (completion rows) |
| 71 | stale frame of a dead claude under a relaunched claude | R neg | unknown/unknown | — | darwin |
| 72 | teammate view, cloud sessions, ultraplan | P | unknown | — | source-only |
| 73 | vim editor mode: `-- INSERT/VISUAL/VISUAL LINE --` prefix / NORMAL | R / U | prefix tolerated / NORMAL invisible | — | source-only |
| 74 | compaction / hook spinners | R | working | `claude.activity.spinner` | source-only |
| 75 | title, progress (osc 9;4), bell | X | never read | — | — |
| 76 | classic tall pane, composer in the top region | R | as bottom | `claude.layout.composer` (region top) | darwin, linux |
| 77 | tall pane, pre-REPL dialog in the top region or straddling it | R | as bottom | family rule (region top / compound) | darwin (trust at 100 and 70 rows) |
| 78 | pane taller than 256 rows: rows `192..h−65` absent; a read reaching them | P | unknown, `evidence_clipped` | `claude.region.clipped` | darwin (classic and fullscreen at 300 rows), linux (classic at 300 rows) |
| 79 | byte-capped region | P | `evidence_clipped` only where a read reaches the cut | `claude.region.clipped` | capture: darwin, linux |
| 80 | colour level 0 (`NO_COLOR`, `FORCE_COLOR=0`, `TERM=dumb`) | P | never idle; working via hint/background only | — | darwin (`NO_COLOR`); the others source-only |
| 81 | bash mode `!` | P | never idle; composer draft | — | darwin |
| 82 | fullscreen scrollback pill | P | chevron still decisive | — | darwin |
| 83 | narrow mode row hiding an item | P | unknown unless the slot is proven (2.4) | — | darwin |
| 84 | classic notifications below / beside the mode row | R | handled | — | darwin |
| 85 | OSC 8 inside rows | parser | zero-width | — | darwin, linux |
| 86 | requests during background work | P | the dialog replaces panel and pill: activity unknown | — | darwin |
| 87 | linux rendering (`●` bullets, viewed glyph, spinner frames) | R | same grammar | — | linux: frames `· ✢ * ✶ ✻ ✽` read working, `●` bullets and viewed glyph read as on darwin; `✳` not seen in 40 samples per host |
| 88 | gated unified footer (`tengu_copper_thistle`) | R | vocabulary covers it | — | source-only |
| 89 | fullscreen stale cells after a resize | P | exact match fails: unknown until repaint | — | darwin |
| 90 | other sessions needing input: footer `← N agent(s)` | P | interaction none (§5 c16) | — | source-only |
| 91 | pinned notices (`⚠ ` items of the pinned column: transcript saving off, transcript writes failing) | P | fullscreen/classic: opaque footer rows, tolerated; screen reader: idle lost (§5 c4) | — | darwin |

## 2. screen grammar

### 2.0 primitives

**cells and style.** parse each physical row into cells. use only attribute
presence:

- `FG`: explicit foreground (30–37, 90–97, `38;5;n`, `38;2;r;g;b`).
- `BG`: explicit background.
- `DIM`: sgr 2.
- `INV`: sgr 7.
- `BOLD`: sgr 1.

never compare colour values. claude's `dimColor` renders as the theme's
`inactive` foreground, not sgr 2. `chalk.dim` placeholders render as sgr 2.

**OSC 8.** OSC 8 open and close are zero-width in every row; tmux writes them
ST-terminated. any other escape tmux's writer never emits makes only that row
unparseable, never the sample. an unparseable row is present but unrecognized: it
is not blank, has no column, matches no shape and ends every block, and it never
counts as clipped evidence. an unparseable slot row leaves activity unknown;
among the footer's opaque rows it is tolerated like any opaque row.

**rows are self-contained.** raw `capture-pane -e` carries style across rows; the
capture takes each row on its own, so every row parses from the default style
([terminal control §2](terminal-agent-control.md#2-observation-and-schemas)).

**columns.** a column is a cell index. every glyph used here is one cell in
claude's layout and in tmux (`⏸ ⏵ ← ↓ ✔ ☐` measured).

**tokens.**

| token | code point |
| --- | --- |
| `·` | U+00B7 |
| `…` | U+2026 |
| NBSP | U+00A0 |
| rule `─` | U+2500 |
| edge `▔` | U+2594 |
| dashed `╌` | U+254C |
| `⎿` | U+23BF |
| `❯` | U+276F |
| `⏸` | U+23F8 |
| `⏵⏵` | U+23F5 U+23F5 |
| `⏺` | U+23FA (darwin bullet, viewed glyph) |
| `●` | U+25CF (reduced motion; linux bullet and viewed glyph; survey bullet) |
| `◯` | U+25EF |
| `├` / `└` | U+251C / U+2514 |
| `☐` / `☒` / `✔` | U+2610 / U+2612 / U+2714 |
| `↳` | U+21B3 |
| `◇` / `◆` | U+25C7 / U+25C6 |

**helpers.** these are the only helpers the grammar uses.

- `plain(row)`: the text with NBSP→space and trailing spaces removed.
- `col(row)`: the first non-space cell.
- `blank(row)`: `plain` is empty.
- `RULE`: a row of exactly `W` cells, all `─`.
- `BANNER`: a row of exactly `W` cells whose `plain` matches `^─+ \S(.*\S)? ─$`: a
  right-aligned label followed by exactly one `─` (session name, colour, agent,
  teammate hint).
- `LEFT_LABEL`: a row of exactly `W` cells that starts `── `, ends `─` and is not
  `BANNER` (history recall).
- `EDGE`: a row of `W` cells beginning `▔`; a notification may be embedded
  (`▔▔… ◐ medium · /effort ▔`).
- `BOXROW`: a non-blank row containing only `─ ╌ ▔` and spaces.
- `MARKER`: the stripped text begins with `❯`, `N. ` or `[ ] `/`[✔] `.
- `notification(row)`: at least 8 leading spaces (a right-aligned notification:
  classic footer, fullscreen top margin), or `col` 2 and `plain` begins
  `  tmux detected · ` or `  tmux focus-events off · `. this is the only
  notification predicate; 2.5 applies the same copies to joined screen-reader
  rows.

**logical block `blk(c, end)`.** the rows from `end` upward while each row is
non-blank, has `|col − c| ≤ 1`, and is not a `MARKER` or `BOXROW`; its topmost row
may sit at any of those columns. its text is the stripped rows joined by one
space. this is the wrapping contract: claude wraps at word boundaries and
continues at the block's column; the `Syntax theme:` block continues one column
left, which the ±1 tolerance covers. a word longer than the line is hard-broken,
which can insert a spurious space inside a path; every matched token is
word-bounded, so matching never depends on those spaces.

**`texts(c)`.** `{ blk(c, r) }` for every row `r` that ends a block (the row below
`r` cannot extend it) and whose block's topmost row is at exactly column `c`. a
2.6 hint or anchor block `blk(c)` holds under the same condition (2.6: every
column is exact), which keeps col-2 transcript text out of lower columns. `above`
takes the plain `blk`.

**numbered options `opt(c, end)`.** skip blank rows above `end`, then take the
contiguous rows that are either an option start `^ {c}(❯| ) N\. label` or a
continuation (`col ≥ c+5`: a wrapped label or a description). numbers run 1..n,
at most one `❯` sits at column `c`, and labels are joined across their
continuations.

**unnumbered options `uopt(c, end)`.** the contiguous rows `^ {c}(❯| ) \S` or
`^ {c+2}\S` with at most one `❯` at column `c`. a wrapped label continues at
`c+2`, where an unselected option also starts, so labels are not separable: the
result is the whole block's joined text and anchors are word-bounded substring
checks on it.

**text above `above(c, row)`.** the plain block `blk(c, ·)` ending at the first
non-blank row above `row`.

### 2.1 region selection

`B` is the bottom region and `T` the optional top region
([spec §3](terminal-observation.md#3-composition-and-observation-boundary)). `T`
holds every row above `B` up to row 191, so the captured rows are one row space
`0..h−1` unless a byte cap clipped a region or the pane is taller than 256 rows.
the rows a region dropped are absent; they form one run between `T`'s kept rows
and `B`'s kept rows. `floor` is the first row below that run (0 when nothing is
absent).

- **`L`.** the lowest non-blank row at or below `h−1` and at or above `floor`. if
  the scan reaches an absent row first, the lowest drawn row may lie in the cut:
  unknown/unknown, `evidence_clipped`, rule `claude.region.clipped`. if every row
  is blank and none is absent: unknown/unknown, `layout_unknown`, no rule. `L` in
  `T` and surfaces straddling `T` and `B` read like any other rows.
- **dispatch** (2.2) runs on `L`.
- **reads past the captured rows.** every read moves upward from `L`, so the first
  absent row it can meet is `floor−1`; there it fails:
  - the dependent dimension is unknown with `evidence_clipped` and rule
    `claude.region.clipped`. required reads are the slot walk (activity), the
    composer anchor search, the screen reader's pinned-column read (interaction,
    2.5) and a family's anchors (interaction). an intact lower composer whose walk
    ends at or below `floor` is unaffected.
  - if no dispatch step matches and some read reached the cut: unknown/unknown,
    `evidence_clipped`, `claude.region.clipped` (a classic pane taller than 256
    rows whose lowest drawn row lies above the bottom region). a cut that no read
    reached does not make the reading clipped.
- **diagnostic region.** each rule names the region of the rows that decided it:
  `top`, `bottom`, or `compound` when they span both. `claude.region.clipped` names
  the captured row just below the cut, or the bottom region when it kept no row at
  all.

### 2.2 dispatch (first match)

1. screen-reader startup (2.5).
2. screen-reader surface: the first cell of `L` is `$` and its second cell, if any,
   is NBSP (2.5). a shell prompt `$ ` (ordinary space) never qualifies.
3. screen-reader request (2.5).
4. composer surface, when the composer anchor holds (2.3).
5. families at `L′`, in the order of 2.6.
6. nothing matched: unknown/unknown/none, composer unknown, `layout_unknown`. an
   unknown pane is never promoted to `input`.

### 2.3 composer surface (fullscreen, classic)

**anchor (`claude.layout.composer`).** `b` is the first `RULE` at or above `L`,
within 30 rows. above `b`, skip continuation rows (col ≥ 2 or blank; at most 40)
to the first other row `c`. the anchor holds iff:

- `c` is a chevron row: `❯` NBSP at col 0 (`❯` alone once tmux trims the NBSP), or
  `!` NBSP in bash mode. a `❯` + ordinary space, or a `❯` with `BG`, is a
  historical user prompt, never a chevron;
- the row above `c` is the top rule `t`: a `RULE`, a `BANNER` or a `LEFT_LABEL`.

otherwise the anchor fails and dispatch continues at step 5. no other `RULE` is
tried: a dead process's frame above a newer surface can never capture it. rules
indented by 2 are quoted text.

`STYLE_LIVE` holds iff the first cell of `t` has `FG`. the rule is
`promptBorder`-coloured in every theme and takes the banner colour under `/color`;
it has no `FG` at colour level 0.

**top rule.** `RULE` and `BANNER` are the ordinary composer. `LEFT_LABEL` is not:
interaction unknown, composer unknown; activity steps 1–2 below still apply, idle
never does.

**footer.** the rows `b+1 .. L` are **accounted** iff they form one of:

- **ordinary:** zero or more opaque rows (any content: first claude's pinned
  column, pinned notices such as `⚠ Transcript saving is off …` and the usage-limit
  wait, then the statusline rows), the mode row `M` (the lowest row matching 2.4),
  then the **tail**: zero or more `notification` rows, then optionally exactly one
  blank row and the panel.
- **bash:** row `b+1` has `plain == "  ! for shell mode"`, the chevron is `!`, then
  the tail. claude returns this footer early in bash mode, so no hint or pill
  segment can coexist with it (S).
- **help grid (`claude.menu.help`):** the first footer row starts
  `  ! for shell mode` followed by more columns, and a footer row contains
  `/ for commands`. claude draws the grid instead of the mode row (S), so `M` is
  absent. interaction `menu`, composer `blocked`.

any other footer leaves the surface **void**: activity, interaction and composer
unknown, idle impossible, rules `claude.layout.composer` only. this covers
autocomplete, history search, `Press … again to exit`, `Pasting…`,
`paste again to expand`, the panel focus footer, any block drawn under the prompt,
an unrecognized col-2 notification, and the resume hint and shell prompt printed
under a dead process's frame while a relaunched claude has not yet painted.

**panel.** claude draws it one blank row below the footer (`marginTop 1`, S).
rows, in order:

| row | `plain` | note |
| --- | --- | --- |
| main | `^(  \|❯ )[◯⏺●] main( +↑ \d+ more)?$` | drawn only while an agent row exists or an agent is viewed; absent above workflow-only panels |
| agent | `^(  \|❯ )((  )*[├└] )?[◯⏺●] \S` | nested rows carry a tree connector (S) |
| idle summary | `^(  \|❯ )◯ \d+ idle agents?$` | checked before the agent shape |
| more | `^  ↓ \d+ more$`, or one blank row in its place | the blank row appears with more than 5 agent rows (S) |
| workflow | `^(  \|❯ )[◯⏸] \S` | `⏸` while paused on a rate limit (S); after agent rows |

the panel is a contiguous run of these rows (one blank row allowed where `more`
stands). an agent or workflow row is a **work row** iff its glyph is not `⏸` and
`plain` ends with a running status:

```text
\s(\d+/\d+ · )?\d+[dhms]( ?\d+[dhms])*( · [↑↓] \S+ tokens)?( · \d+ queued)?$
```

(`15s · ↓ 27 tokens`, `1m 26s`, `0/1 · 57s`, `1/1 · 2m05s · ↓ 2 tokens`). claude
draws `idle` for an idle teammate and a finished named agent, `waiting` for an
idle unfinished local agent, `awaiting approval` for a teammate awaiting plan
approval (S): rows ending `\s(idle|waiting|awaiting approval)( · \d+ queued)?$`,
the idle summary, `⏸` rows and rows whose status is unreadable (a
`subagentStatusLine` body, a status cut at a narrow width) are accounted but make
no work claim; they block idle. a finished agent or workflow keeps its duration and
a status-coloured glyph until evicted (S: 30 s after its completion notification;
~25 s observed), so it counts as work meanwhile (§5 c8). style cannot separate
running from finished rows.

**viewed subagent (`claude.menu.subagent`).** a non-main agent row carries the
viewed glyph `⏺` (`●` on linux). interaction `menu`, composer `blocked`, activity
from the panel. typing there messages the subagent. the banner on the top rule is
not part of the test: named sessions draw the same banner.

**slot row `S`.** start at `t−1`:

1. skip up to 6 `notification` rows. the composer's one-row top margin holds
   them. a live spinner cannot be passed: its child block begins `⎿` or sits at
   col ≥ 5.
2. skip blank rows.
3. if the row is at col 2 beginning `⎿` or at col ≥ 5, it is the bottom of the
   spinner's child block: walk upward over rows at col ≥ 5 to the row at col 2
   beginning `⎿`, within 12 rows including it, and continue above that row. the
   block is a tip (wrapping at col 5), `Next: <task>`, the expanded task list (≤ 5
   tasks of ≤ 2 rows each plus `… +N pending`), the compaction hint, or the
   overload row under a retry (S). if no `⎿` row is found within 12 rows, stop at
   the original row.
4. `S` is the current row. if `S` is at col 0, `plain(S)` matches `^\(.*\)$` and
   the row above is an `SP`-shaped row without its details, `S` is that row above
   (a wrapped spinner, S).

**shapes of `plain(S)`, at col 0, style ignored.**

| shape | regex |
| --- | --- |
| `SP` | `^[·✢✳✶✻✽*●] \S.*?(…\|\.\.\.)( \(.*)?$` and not `DONE`/`WAIT`. messages are free (spinnerVerbs, `Compacting conversation`, hook runs, an in-progress task's `activeForm`); never whitelist verbs |
| `RT` | `^✻ .*( · Retrying in \| · will retry in \| · next try in \|No response from the API after )` |
| `DONE` | `^✻ (Baked\|Brewed\|Churned\|Cogitated\|Cooked\|Crunched\|Sautéed\|Worked) for ` |
| `WAIT` | `^✻ Waiting for( \d+ background agents?)?( and)?( \d+ dynamic workflows?)? to finish$`, with at least one count |

`WAIT` is the end-of-turn duration row drawn instead of `DONE` while agents or
workflows the turn launched are still running or unnotified. its counts are a
snapshot taken when the row mounts (S). it counts as current only while it is the
slot row: the completion notification of the awaited work starts a new turn whose
rows land below it. with `showTurnDuration` off claude draws no row at all.

**digit band (`claude.composer.band`).** claude draws optional prompts in the area
above the composer (each with `marginTop 1`, the composer's own top margin below
them), and a single key typed into the composer goes to them: one character that
is a digit after NFKC, an AZERTY digit key (`& é " ' ( - § è _ ç à`) or, for the
transcript-share prompt, `y`/`n`/`d` (S). the shared capture has exactly six call
sites in 2.1.286; each band's legend is its lowest row, so it is `S` (in brief mode
the composer has no top margin and the legend sits directly above `t`, which is
why step 1 never skips it). if `S` matches a legend at its exact column, the
composer is unknown:

| call site | column | legend (`plain`) |
| --- | --- | --- |
| rating surveys (session, memory, plugin, post-compact, long-context) | 2 | `^  1: Bad\b` |
| transcript-share prompt | 2 | `^  y: Yes\b` |
| thanks row after a rating | 0 | `^\(Optional\) Press \[1\] to tell us (what went well\|what went wrong\|more) · /feedback$` |
| feedback-draft card | 0 | `^1 to review · 2 to send · 0 to dismiss`; `^Send without reviewing \(full draft \+ env, no transcript\)\? 2 to send · Esc to back$`; `^.* 1 to review & retry · Esc to dismiss$`; `^Turn off Claude-drafted feedback\? 0 to turn off · Esc to keep$` |
| web-setup offer after a push | 2 | `^  1: Yes, run /web-setup\b` |
| plugin `AbovePrompt` band | — | arbitrary plugin copy: undetectable (§5 c11) |

the survey follow-up text box (`^  Enter to (send|skip) · Esc to (clear|skip)\b`,
col 2) owns every key while open and is treated the same. the band changes
neither activity nor interaction. a col-2 transcript row reading like a legend
directly above the margin costs only an unknown composer.

**usage-limit wait (`claude.limit.wait`).** claude builds the quota auto-resume
state as up to three items (`limit-status`, `limit-next`, `limit-promo`; S) and
draws them only in its pinned column, a `flexDirection: column, paddingX: 2` box
between the editor's bottom rule and the footer that first holds the pinned
notices (S; 2.1.286 reads the limit context in that one component). the first item
is `⚠ ` plus its copy at col 2, each later item has `paddingLeft 2` (col 4), and
every item wraps in its own column (`wrap: "wrap"`), never right-aligned. the
ordinary footer accounts the column as opaque rows from `b+1`, above the
statusline rows (observed with a pinned notice in fullscreen at 80 and 40 columns
and in classic under a two-row statusline). the check reads the rows `b+1 .. L` as
one text (stripped rows joined by one space, a blank row adding an empty text),
since a wrapped item splits its copy across rows. the copies, any of:

- the fixed status copy and every default wrap-up lead: `Usage limit reached`,
  `Your usage limit has reset`;
- the default next-line copy: `Continuing automatically `,
  `Continuing shortly · esc to cancel`, `Press enter to continue`.

a match makes idle impossible and interaction and composer unknown; activity steps
1–2 still apply (a wrap-up turn keeps its spinner). no row above the composer is
read: claude also appends a transcript notice when the wait arms
(`Usage limit reached · continuing automatically {when} · esc to cancel`, a dimmed
`⏺` system row; S), and that row is history. the wrap-up lead
(`tengu_lantern_sconce_copy`, ≤ 100 chars) and the waiting copy
(`tengu_functional_llama`) are remotely configurable and may drop every anchor
(§5 c17). a statusline or panel row printing the same words costs an unknown.

**chevron classes** (cell 0 of `c`, `❯` only):

| class | condition |
| --- | --- |
| `LOADING` | `FG` or `DIM` (`dimColor: isLoading`) |
| `READY` | `STYLE_LIVE` and none of `FG DIM INV BG` |

otherwise neither (style not live). a bash `!` is always `bashBorder` `FG`; busy
and ready differ only in colour value, so it is never `READY` or `LOADING`.

**activity** (accounted surfaces only; first match):

1. **background:** a work row, or `S` is `WAIT`, or a `work` segment in `M`:
   working (`claude.activity.background`). this runs before the conflict test: the
   panel and the waiting row are independent of the chevron and slot.
2. **corroborator:** `S` is `SP` or `RT`, or `M` has an `interrupt` segment.
   - `READY`: unknown, `evidence_conflict` (`claude.activity.conflict`).
   - `LOADING`: working (`claude.activity.spinner`/`.retry`/`.hint`, first
     corroborator in that order).
   - otherwise, with an `interrupt` segment: working (`claude.activity.hint`).
   - otherwise unknown.
3. **idle** (`claude.activity.idle`) requires all of:
   - `READY`;
   - an ordinary footer with a recognized `M`, and no usage-limit wait;
   - no panel row other than `main`;
   - the pill slot proven (P0, P1 or P2, 2.4);
   - every `M` segment in class `tail`, `agents`, `process`, `indicator` or
     `other` (2.4);
   - none of steps 1–2 matched.

   tolerated: the statusline, a banner, process pills, the indicator,
   notification rows, the band, placeholder, draft, and interrupt/error/completion
   rows in the slot.
4. otherwise unknown: `LOADING` with no corroborator, a left-labelled rule, bash
   mode, colour level 0, the usage-limit wait, a non-work panel row, an unproven
   slot, a `work`/`unknown_pill`/`unlisted` segment, an unparseable slot row.

**interaction.**

- `unknown`: a void surface, a `LEFT_LABEL` top rule, or the usage-limit wait.
- `menu`: the viewed subagent, or the help grid.
- `none`: an ordinary footer with `M` recognized, or the bash footer. other
  sessions' requests (`← N agents`, teammates) remain outside this terminal's
  interaction (§5 c16).

### 2.4 mode row, pill slot and footer vocabulary

**grammar** (over `plain(M)`):

```text
^  (-- (INSERT|VISUAL|VISUAL LINE) -- )?(⏸|⏵⏵) (manual mode|plan mode|accept edits|auto mode|bypass permissions|don't ask) on( \((\S+) to cycle\))?
```

then zero or more ` · <segment>`, where a segment contains no ` · `.

- a run of ≥ 3 spaces followed by text starts a right-aligned notification suffix
  (classic), not parsed.
- a trailing ` ·` means a later item was cut.

**layout** (S, and measured). the footer container has `paddingX 2`; the row is
`height:1, overflow:hidden`, items in order

```text
mode · [indicator] · [bg-detach ←] · [PR] · [links…] · [task pill] · [artifacts] · Hs
```

every item before `Hs` is a `flexShrink:0` box. the mode item's separator is its
own box; every other item carries its trailing ` · ` inside its box. when the row
overflows, either the row is cut at `W − 2` (a character prefix: `1 shel` at 40,
`1 she` at 39) or a box receives fewer cells than its natural width and shows only
the first line of its word-wrapped text, its other cells blank, while later boxes
vanish. measured in manual mode with indicator `a bcde` and a shell pill:
`1 shell` complete + 3 blanks at 42; `1` + 5/4/3 blanks at 38/37/36; `◆ a` + 4 at
28; `⏸ manual mode` + 4 at 19; an 8-letter indicator left `◆` + 6 at 28. so the
blanks after a partial first line are unbounded, while a complete box leaves at
most 3: one interior cell when its trailing ` ·` wraps away, plus the 2-cell
padding. `Hs` is the only `wrap:"truncate"` child: it receives width only after
every preceding box has its full width, and it alone ends in `…` when cut. the
indicator is `◆ ` + text cut to ≤ 6 cells by claude itself (`tt(re(text,32),6)`),
so it may end in `…` without being `Hs`.

**`Hs` members** (instances: `<chord>` is one token; `N` is digits or `99+`):

`<chord> to interrupt`, `<chord> to return to team lead`, `<chord> to hide tasks`,
`<chord> to show tasks`, `/tasks to see subagents`, `? for shortcuts`,
`← for agents`, `← N agent`, `← N agents`, `← N done`, `N feedback draft`,
`N feedback drafts`, `keep holding…`, `ctrl+c to copy`,
`option+click to native select`, `shift+click to native select`,
`set macOptionClickForcesSelection in VS Code settings`, `hold <chord> to speak`,
`↓ to manage`, `<chord> to view tasks`, `<chord> to view artifacts`.

**segment classes** (closed; the vocabulary is the same for all renderers):

| class | labels | effect |
| --- | --- | --- |
| `interrupt` | `<chord> to interrupt` | corroborator (2.3) |
| `tail` | every other `Hs` member except the agents hints; and the bare `…` | tolerated; a complete one proves the slot |
| `agents` | `← for agents`, `← N agent(s)`, `← N done` | tolerated; proves nothing: bg-detach draws the same copy before the pill. `← N agent(s)` counts other sessions reporting `needsInput` (S; §5 c16) |
| `work` | `N local agent(s)` | working (drawn only with the coordinator panel gate off) |
| `process` | `N shell(s)`, `N shell(s), N monitor(s)`, `N monitor(s)`, `N Artifact comment monitor(s)`, `dreaming`, `auto-mode scan`, `memory import`, `memory import N/N` | tolerated; a surviving task count alone is not work |
| `unknown_pill` | `N MCP task(s)`, `N team(s)`, `◇ N cloud session(s)`, `◇ N remote dynamic workflow(s)`, `◆ ultraplan ready`, `◇ ultraplan`, `◇ ultraplan needs your input`, `↓ to view`, `N background task(s)`, `↳ N background`, `↳ N background (<chord> to manage)` | blocks idle; no working claim |
| `indicator` | fullscreen/classic only: `◆ ` then 1–6 cells (the last may be `…`) | tolerated; proves nothing |
| `other` | `PR #N`, `MR !N`, `gh auth login for PR status`, `install gh for PR status` | tolerated |

local workflows never reach the pill (S: its filter drops them unconditionally);
they are panel rows.

**classification.**

- a segment that is an exact instance of a label is in that label's class
  (instances of different classes never coincide). so is any segment but the last
  that has the indicator shape.
- only the last segment can be cut, in two ways: `Hs` truncates with `…`; a box is
  cut bare (character prefix or first wrapped line). a last segment that is not an
  exact instance collects the classes it could have been cut from:
  - ending in `…`: let `r` be the segment without the `…`, trailing spaces and one
    trailing `·`. an empty `r` is `tail`; otherwise the classes of the `Hs` members
    of which `r` is a character prefix;
  - bare: the classes of the box labels (`agents`, `work`, `process`,
    `unknown_pill`, `other`) of which the segment is a character prefix. the
    agents labels are both `Hs` members and box labels (bg-detach draws the same
    copy), so a bare `←` or `← for` is `agents`;
  - plus `indicator` if it has the indicator shape.

  exactly one class: that class. none, or several: `unlisted`, which blocks idle.
- every other segment is `unlisted`.
- a character prefix is taken token by token: earlier tokens equal (`<chord>` any
  token, `N` any count), the last token a character prefix. a first wrapped line is
  a word prefix, hence also a character prefix.

a cut `work` or `unknown_pill` pill stays in its class or becomes ambiguous (`1`
prefixes `1 shell`, `1 local agent` and `1 MCP task`); it never lands in a
tolerated class alone. `esc to i…` is `interrupt`. a bare last `◆ ultra` prefixes
`◆ ultraplan ready` and has the indicator shape: `unlisted`. user-configured footer
links and an extra `PromptHint` render hook are `unlisted` (§5 c4).

**pill slot proven** iff one of:

- **P0**: the mode is not `manual mode` and the mode item ends with a complete
  ` (<chord> to cycle)`. claude draws the cycle hint only when the count of
  {non-default mode or coordinator, task pill} is below 2, so in a non-default mode
  a visible hint proves that no task pill exists; the gated footer chooses its
  cycle hint only after the `manage` state, which a pill forces. observed: with a
  shell pill the cycle hint never showed in accept-edits, plan or auto mode; it
  appeared as soon as the pill ended.
- **P1**: some segment is a complete `tail` label, or the last segment ends in `…`
  and its `r` is empty or a character prefix of an `Hs` member instance. both mean
  `Hs` laid out, which happens only after every preceding box has its full width.
  the indicator's own `…` (`◆ Revie…`) proves nothing.
- **P2**: no trailing ` ·`; the last item (the last segment, or the mode item when
  there is none) is an exact, complete instance of a fixed-vocabulary label: not of
  the `indicator` class, not a bare prefix, not ending `…`; and at least 6 blank
  cells follow it up to the row end or the notification suffix. a complete box
  leaves at most 3 blanks behind it, and a complete label followed by any item
  shows its own ` ·` unless at most one cell of its box remains; bare and
  indicator-shaped last items are excluded because a wrapped first line leaves
  unbounded blanks (`PR` of `PR #123456`, `◆ a`).

measured: 60, 50 and 45 columns prove the slot (P1: `…`, `← for…`); 42
(`1 shell` + 3), 40 (`· 1 shel`), 35–38 (`· 1`), 33–34 (trailing ` ·`), 31–32
(indicator) and narrower do not.

### 2.5 screen reader

the screen reader has no sgr and no box drawing. it flattens the footer tree:
sibling boxes join with one space, so box separators read `  ·  `, while
separators inside `Hs` stay ` · `. every line wraps by word with hard breaks at the
pane width (S).

- **startup (`claude.activity.starting_sr`).** `plain(L)` at col 0 is exactly
  `[Screen Reader Mode: on via flag]`, `[Screen Reader Mode: on via env]`,
  `[Screen Reader Mode: on via settings]` or `[Screen Reader Mode: on]`. rows above
  are ignored: a launch from a shell prompt prints the banner under the shell's
  output. result starting/unknown/none, composer unknown. this is the startup
  quiet (S: `CLAUDE_AX_STARTUP_QUIET_MS`, default 3000, at most 600000); every
  mounted layout ends with a `$` row or a request row.
- **surface (`claude.layout.sr`).** `L` is the input row: `$`, then NBSP or
  nothing.
  - walk upward from `L−1` over recognized notification blocks: a row
    `^effort: (low|medium|high|xhigh|max)( · ultracode)? · /effort$`, or 1–3 rows
    whose texts joined by one space equal
    `tmux detected · scroll with PgUp/PgDn · or add 'set -g mouse on' to ~/.tmux.conf for wheel scroll`
    or
    `tmux focus-events off · add 'set -g focus-events on' to ~/.tmux.conf and reattach for focus tracking`.
    if the walk meets the cut while the rows joined so far, after a space, end such
    a text, the notice's head and `M′` lie in the cut: unknown/unknown,
    `evidence_clipped`.
  - the first other row is `M′`. replace every `\s+·\s+` in it with ` · ` and
    match
    `^(-- (INSERT|VISUAL|VISUAL LINE) -- )?(manual mode|plan mode|accept edits|auto mode|bypass permissions|don't ask) on( \(\S+ to cycle\))?( · <segment>)*$`.
    segments are classified by exact instance of the 2.4 labels (the screen reader
    never cuts; the indicator loses its aria-hidden `◆` and is `unlisted`).
  - `M′` unmatched, or any segment `unlisted`: interaction, activity and composer
    unknown. a wrapped mode row puts its continuation directly above `$` and lands
    here.
  - **pinned column.** claude draws its pinned column (2.3) directly above `M′`,
    above any statusline rows; the screen reader drops its padding, so its
    `⚠ `-led items and their wrapped rows sit at col 0. from `M′−1`, look at most 16
    rows up for the lowest row starting `⚠ `; the rows from it to `M′−1`, joined,
    are read for the 2.3 copies. a match: interaction and composer unknown,
    `claude.limit.wait`. a cut met before such a row may hide one: interaction and
    composer unknown, `evidence_clipped`. no such row within 16 rows: no column.
    the bound keeps a quoted `⚠ ` row higher in the transcript out of the read; a
    wait taller than 16 rows is part of §5 c17. activity below still applies.
  - otherwise interaction `none`, and activity is, first match:
    - working (`claude.activity.background`): a `work` segment;
    - working (`claude.activity.hint_sr`): an `interrupt` segment;
    - idle (`claude.activity.idle_sr`): no `unknown_pill` segment, an empty `$`
      row, and the row directly above `M′` is a completion row
      `^(Baked|Brewed|Churned|Cogitated|Cooked|Crunched|Sautéed|Worked) for ` or
      starts `claude: `;
    - otherwise unknown: a spinner line without the hint (a draft suppresses it),
      a retry row, a statusline, `you:`/`tool:`/`error:` neighbours, a wrapped
      completion row, the fresh-session header, a pinned notice.
- **requests.** if `plain(L)` is `Esc to cancel · Tab to amend` or
  `Enter to confirm · Esc to cancel`, or starts ` Syntax theme: ` (the theme
  preview stays below the prompt), the request row is the row above `L`, else `L`.
  the request row matches `^Select with numbers \[1-\d+\]`,
  `^Enter text for option \d+ `, `^Enter y/n:$` (api key, trust) or
  `^Press Enter to continue…$` (security notes). the anchors are rows above the
  request row, first match:

  | order | rule | condition |
  | --- | --- | --- |
  | 1 | `claude.setup.sr` | a row that, after an optional `Permission Required: ` prefix (trust), starts with `Accessing workspace:`, `Detected a custom API key in your environment`, `WARNING: Claude Code running in Bypass Permissions mode`, `New MCP server found in this project: `, `Select login method:`, `Let's get started.`, `Security notes:` or `Settings Error` |
  | 2 | `claude.permission.sr` | a row starting `Permission Required: ` and a row `^Do you want to .+\?$` |
  | 3 | `claude.question.sr` | a tab row `^←\s+[☐☒✔]` or header `^ [☐☒✔] \S`, or a row `^> .+\?$` |

  composer `blocked`. a request row with no anchor reads unknown/unknown. a wrapped
  transcript row that begins like an anchor can only swap the request kind, never
  invent a request: the request row is drawn only while a request is live.

### 2.6 request and menu families (no composer anchor)

**`L′`.** start from `L` and skip at most two trailing right-aligned rows (≥ 8
leading spaces). this covers the question's countdown and plugin-notice rows (S)
and fullscreen trailing notices.

every column below is exact. anchors are rows or joined blocks at their column;
transcript text sits at col ≥ 2 behind `⏺`/`⎿`. families that need a col-0 or
col-1 anchor or a full-width rule are therefore unreachable by quoted text, and so
is any family that needs a hint block at `L′`: in viewers the viewer footer
occupies `L′`. composer is `blocked` for every family.

| order | rule id | hint block at `L′` | required anchors | result |
| --- | --- | --- | --- | --- |
| 1 | `claude.input.dialog_waiting` | `blk(2)` starts `dialog waiting · Showing detailed transcript · ` | — | input |
| 2 | `claude.menu.transcript` | `blk(2)` starts `Showing detailed transcript · ` | — | menu |
| 3 | `claude.menu.agents_view` | `blk(2)` matches `^(.* · )?enter to return( · \|$)` | a row containing `N awaiting input · N working · N completed` | menu |
| 4 | `claude.menu.model` | `blk(3)` starts `Enter to set as default · ` | an `EDGE` above | menu |
| 5 | `claude.menu.settings` | `blk(3)` starts `Type to filter · ` or `Enter/Space to change · ` | a row `^   Settings +Status +Config +Usage` | menu |
| 6 | `claude.menu.theme` | `blk(3)` == `Enter to select · Esc to cancel` | a row starting `    Syntax theme: ` | menu |
| 7 | `claude.menu.resume` | `blk(5)` contains `to show all projects` and ends `Esc to cancel` | — | menu |
| 8 | `claude.menu.btw` | `blk(4)` ends `Esc to close` | an `EDGE` and a row starting `    /btw ` | menu |
| 9 | `claude.permission.dialog` | `blk(1)` matches `^\S+ to cancel( · \S+ to amend)?$` | `opt(1, above the block)` with option 1 starting `Yes`; in `above(1, options)` the last row starting `Do you want to ` joined with the rows below it ends `?` | permission |
| 10 | `claude.permission.workflow` | `blk(2)` matches `^\S+ to cancel( · \S+ to amend)?( \S+ to edit script in \$EDITOR)?$` | `opt(2, above the block)` with option 1 `Yes, run it` and last option `No`; the first non-blank row above the options with col < 2 is at col 1, reads exactly `Run a dynamic workflow?`, and has a `RULE` directly above it | permission |
| 11 | `claude.question.preview` | `blk(0)` starts `Enter to select · `, contains ` · n to add notes · `, ends `Esc to cancel` | one blank row, then `^(❯ \|  )Chat about this$`; directly above it a `RULE`; above that (skipping blanks) the side-by-side block, whose first row holds the preview box corner `┌` at column `p`: scanning up from the `RULE`, the first row whose leftmost `┌` has cells left of it that start option 1 (`^(❯\| ) 1\. `), since a lower `┌` belongs to a box the preview content draws; the cells left of `p` of its rows form `opt(0, ·)` from the block's first row, with ≤ 1 `❯`; at least one `❯` among the options and the chat row | question |
| 12 | `claude.question.form` | `blk(0)` starts `Enter to select · `, contains ` to navigate`, ends `to cancel`, does not contain `n to add notes` | one blank row, then `^(❯\| ) N\. Chat about this$`; directly above it a `RULE`; then `opt(0, ·)` with ≤ 1 `❯`; at least one `❯` among the options and the chat row | question |
| 13 | `claude.question.review` | none; `opt(0, L′)` has labels exactly `Submit answers`, `Cancel` | `texts(0)` contains `Ready to submit your answers?` and `Review your answers` | question |
| 14 | `claude.input.elicitation` | `blk(2)` starts `Esc to cancel · ` | a row `^  MCP server “.*” (requests your input\|wants to open a URL)$` | input |
| 15 | `claude.confirmation.plan` | optional `blk(3)` matching `^\S+ to edit in `, then `opt(3, ·)` | option 1 starts `Yes`; an option starts `Tell Claude what to change`; `above(3, options)` starts `Claude has written up a plan and is ready to execute.` | confirmation |
| 16 | `claude.confirmation.exit_plan` | none; `opt(4, L′)` (the hint lies below the pane) | option 1 starts `Yes, and switch to `, last option `No`; `above(4, options)` == `Claude wants to exit plan mode` | confirmation |
| 17 | `claude.confirmation.enter_plan` | none; `uopt(2, L′)` ends `No, start implementing now` (the confirm label names the mode: `Yes, enter plan mode` or `Yes, and switch to plan mode (…) for this session`) | the first non-blank row above the options with col < 2 is at col 1, reads exactly `Enter plan mode?`, and has a `RULE` directly above it | confirmation |
| 18 | `claude.setup.trust` | `blk(1)` == `Enter to confirm · Esc to cancel` | `uopt(1, ·)` contains `No, exit` and `Yes, I trust this folder`; `texts(1)` contains `Accessing workspace:`; a `RULE` above | setup |
| 19 | `claude.setup.apikey` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` present; `texts(2)` contains `Detected a custom API key in your environment` and `Do you want to use this API key?`; a `RULE` above | setup |
| 20 | `claude.setup.bypass` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` contains `No, exit` and `Yes, I accept`; `texts(2)` has a block starting `WARNING: Claude Code running in Bypass Permissions mode`; a `RULE` above | setup |
| 21 | `claude.setup.mcp_server` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` contains `Use this MCP server` and `Continue without using this MCP server`; `texts(2)` has a block starting `New MCP server found in this project: `; a `RULE` above | setup |
| 22 | `claude.setup.settings_error` | `blk(2)` == `Enter to confirm · Esc to cancel` | `opt(2, ·)` has options `Exit and fix manually` and `Continue without these settings`; `texts(2)` has a block starting `Settings Error`; a `RULE` above | setup |
| 23 | `claude.setup.mcp_servers` | `blk(1)` matches `^\S+ to select · \S+ to reject all$` | directly above it the submit row `       Enable selected` or `  ❯    Enable selected`; above that the contiguous checkbox rows `^(  ❯ \|    )\[[ ✔]\] \S`; ≤ 1 `❯` among them and the submit row; `texts(2)` has a block starting `N new MCP servers found in this project`; a `RULE` above | setup |
| 24 | `claude.setup.theme` | `blk(2)` starts `Syntax theme: ` | options at col 1: from the lowest row of either form above the block, the contiguous rows of either form or at col ≥ 6; one `❯`; all in one form: every row unnumbered (`^ (❯ \|    )\S`, none numbered or at col ≥ 6; `✔ ` before the current label; 2.1.286) or the run parsing as `opt(1, ·)` (labels wrapped at col 6, `✔` after the current label; 2.1.284); they include `Dark mode (ANSI colors only)`; `texts(1)` contains `Let's get started.` | setup |
| 25 | `claude.setup.login` | none; `opt(1, L′)` | option 1 starts `Claude account with subscription`; `texts(1)` contains `Select login method:` | setup |
| 26 | `claude.setup.security` | `blk(1)` == `Press Enter to continue…` | `texts(1)` contains `Security notes:` | setup |

hint and option copy uses remappable chords only where written `<chord>` or `\S+`;
everything else is the fixed copy of the qualified versions (2.1.286, and 2.1.284
where §1 names it).

the order resolves every shared `L′` signature. rules 19–22 share hint and column
(claude's shared dialog: rule at col 0, title, body, options and hint at col 2) and
differ by anchors. rule 18 differs by column. rule 23 has its own hint at col 1 and
checkbox options. rule 10 differs from rule 9 by column (options and hint at col 2,
title at col 1); rule 17 shares rule 10's column-1 title under a `RULE` but has no
hint. the theme menu (rule 6) and theme setup (rule 24) differ by hint and by
`Let's get started.`. rules 11 and 12 differ by `n to add notes` and by the chat
row's number. rule 1 precedes rule 2 because the `dialog waiting` prefix marks a
request held behind the viewer (S).

theme setup (rule 24) on 2.1.284 numbers the choices like any option (`  N. label`,
the selected one `❯ N. label`) and draws the check after the current label; 2.1.286
draws them unnumbered with `✔ ` before it. everything around them is the same:
`Let's get started.`, title and subtitle at col 1, the choices at col 1, a blank
row, the `╌` preview box (col 1, `W − 2` wide on 2.1.284) with its diff rows at col
2, and `Syntax theme: …` at col 2 as `L′`, wrapping to col 1 at 40 columns. below 38
columns 2.1.284 wraps labels at col 6 (a word wider than the label column is
hard-broken), so the run also takes continuation rows at col ≥ 6. claude draws one
form per version, so a run mixing the forms is not the picker. rule 6 reads no
option row, so numbering cannot change the theme menu.

**not implemented** (no rule; the pane reads unknown/unknown, §5 c15):

- auto-mode default offer: title `Make auto mode your default permission mode?`,
  unnumbered options `Yes, set auto mode as my default permission mode` /
  `No, keep <mode>` inside `paddingX 2` (S).
- rate-limit options, consumer-terms update, trial expired (S).

**question preview layout.** the options column is 30 cells wide and shrinks with
the pane (the preview box sat at col 34 at 100 columns, 33 at 60, 23 at 40), so the
rule reads options left of the box corner rather than at a fixed width. rows below
the options hold only preview cells. the preview is the focused option's markdown:
a fenced block may draw box glyphs inside the box, on any row from option 2 down; a
markdown table renders as plain `|` text; a preview taller than the box is cut by a
`├─── ✂ ─── N lines hidden ───┤` row above the bottom border. option descriptions
are not drawn. with focus on the chat row, the preview view keeps `❯` on the last
focused option and adds `❯ Chat about this` (two pointers); the ordinary form moves
the only `❯` to `❯ N. Chat about this`.

**wrapping and truncation, measured:**

- permission hint at 26: `Esc to cancel · Tab to` / `amend`.
- question hint at 40: `… · Esc` / `to cancel`; preview hint at 60 and 40: 2 rows.
- multi-question hint with editor at 80: `… · Esc to ` / `cancel`.
- plan sentence at 80, 60 and 40: 2–3 rows at col 3.
- trust at 26: an option label and the hint both wrap, with no blank row between
  options and hint.
- bypass and mcp titles at 40: 2 rows at col 2; mcp option labels continue at col
  4.
- workflow permission at 100/60/40: body wraps at col 2; options, hint and editor
  row unchanged.
- settings error at 40: body wraps at col 2; options unchanged.
- model-menu hint at 60: 2 rows. model menu at 40x30: the hint lies below the pane
  (§1 row 60).
- theme `Syntax theme:` at 40: continuation at col 1.

### 2.7 precedence, reasons, diagnostics

- **activity and interaction** are independent (2.3). a request or menu without a
  composer gives activity unknown, because dialogs replace the panel and pill.
  nothing is carried forward.
- **reason:**
  - `evidence_conflict` when `claude.activity.conflict` fired;
  - `evidence_clipped` when a read reached a clipped boundary and a dimension
    became unknown by it;
  - otherwise `recognized`, `partial` or `layout_unknown`, by the count of known
    dimensions.
- **diagnostics** (in this order, ≤ 5 entries): the region rule
  `claude.region.clipped` when a dimension became unknown by the cut; the layout
  rule of an anchored surface (`claude.layout.composer`, `claude.layout.sr`); the
  family, menu or limit rule; the activity rule; the composer rule
  (`claude.composer.empty|draft|blocked|band`). a 2.6 family has no layout rule;
  screen-reader startup emits only its activity rule; a void surface emits only its
  layout rule. each entry's region is that of its decisive rows (2.1). 48 rule ids,
  each at most 48 chars of `[a-z0-9_.-]`.

### 2.8 notices

none are qualified; both kinds are class `P` and emit `notice: none`.

- `⎿  Interrupted · What should Claude do instead?` and `⏺ API Error: …` are
  transcript rows, identical when historical and reachable by quoting.
- footer notifications are not task notices.

cost: an interrupted or failed turn reads `idle`/`ready` (§5 c2).

## 3. composer semantics

the composer comes from an accounted composer surface (2.3), the screen-reader
surface (2.5), a 2.6 family or a screen-reader request; the last two always read
`blocked`. first match:

| value | condition |
| --- | --- |
| `blocked` | interaction is a recognized request or menu (2.6 families, the help grid, the viewed subagent, screen-reader requests): a surface owns the keystrokes |
| `unknown` | no anchor; a void surface; a `LEFT_LABEL` top rule; the usage-limit wait; an unmatched `M′`; the digit band (`claude.composer.band`); an anchor read cut by clipping |
| `empty` | the cells after `❯`+NBSP are one of: nothing; one `INV` space (drawn cursor); a placeholder, i.e. an optional single `INV` non-space cell (cursor on the placeholder's first letter) followed only by `DIM` cells; and no continuation row has content. screen reader: nothing but spaces/NBSP after `$` |
| `draft` | anything else after the prompt: any non-`DIM` non-space cell, spaces before the cursor (a whitespace draft), an `INV` non-space cell with no `DIM` text after it (cursor on a typed character), coloured slash text, paste chips, content on a continuation row. also bash mode (`!`): a send would run a shell command. screen reader: any non-space character after `$` |

placeholders are recognized by `DIM` only, never by their words. at colour level 0
the placeholder loses sgr 2 and reads as draft (S); that is safe. queued messages
leave the composer as a placeholder. vim NORMAL mode is invisible (U). a
screen-reader draft of only spaces reads `empty`: there is no drawn cursor to
separate it (§5 c14). stale fullscreen cells after a resize can make any exact
comparison fail: unknown.

rule ids: `claude.composer.empty`, `claude.composer.draft`,
`claude.composer.blocked`, `claude.composer.band`. any other unknown emits no
composer rule.

## 4. settings knobs

display configuration is retained; skid overrides nothing.

| knob | effect on the grammar |
| --- | --- |
| `tui: "fullscreen" \| "default"`, `CLAUDE_CODE_NO_FLICKER`, `CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN` | renderer `full` vs `classic`; auto-disabled under `tmux -CC` and after crashed boots. setup dialogs before the REPL render at the top of the normal screen (alt=0). everything is positional. 2.1.284 on linux draws fullscreen with no `tui` setting |
| `statusLine {…}` | 0..n opaque rows between the bottom rule and `M`. suppresses `esc to interrupt`, `? for shortcuts` and voice in the default footer (D), so working needs spinner + loading chevron and idle needs P0, P1 via another `Hs` member, or P2. the gated footer does not suppress the hint. in the screen reader the statusline sits above `M′`, so idle_sr is lost |
| colour level 0: `NO_COLOR` (with `FORCE_COLOR` unset), `FORCE_COLOR=0`, `TERM=dumb` | no `FG`/`DIM`/`BG`; `INV` survives (S). `STYLE_LIVE` fails, so never idle; working only via hint or background; placeholder reads as draft |
| `prefersReducedMotion` | spinner glyph `●`, frozen timer |
| `axScreenReader`, `CLAUDE_AX_SCREEN_READER`, `--ax-screen-reader`; `CLAUDE_AX_STARTUP_QUIET_MS` | screen-reader renderer (ignores `tui`; gated remotely, default on); startup quiet length |
| `CLAUDE_CODE_ACCESSIBILITY=1` | native cursor; the drawn `INV` cursor may be absent. `empty` tolerates that |
| `theme` (dark, light, daltonized, ansi, custom) | colour values only |
| `spinnerVerbs`, `spinnerTipsEnabled`, custom tips, `spinnerTipLabel` | spinner message; the `⎿` tip row and its label, skipped structurally |
| `showTurnDuration` | completion and waiting rows present or absent. off: `WAIT` never renders and screen-reader idle needs a `claude: ` neighbour; linux `●` bullets can then sit in the slot as an `SP`-shaped row with a ready chevron (conflict) |
| keybindings (`chat:cancel`, `chat:cycleMode`, `app:toggleTodos`, `footer:*`, `scroll:*`) | chords inside hints; matched as `<chord>`/`\S+` (one token) |
| `editorMode: vim`, `statusLine.hideVimModeIndicator` | optional `-- MODE -- ` prefix in `M`; NORMAL invisible |
| `--name`, `/rename`, `/color <c>`, `--agent`, teammates | a `BANNER` top rule; `/color` recolours both rules and backs the label, `STYLE_LIVE` still holds. the prompt stays `❯`: the `@name` prompt glyph is reachable only in the dense prompt layout, whose gate is constant false in 2.1.286 (S) |
| permission mode (`defaultMode`, shift+tab, `--permission-mode`, `--dangerously-skip-permissions`) | mode item text; non-default modes draw the cycle hint (P0) when no pill exists. new configs default to auto mode with an onboarding notice (`hasSeenAutoDefaultNotice`). `--dangerously-skip-permissions` shows the bypass warning once per config |
| `prStatusFooterEnabled`, `footerLinksRegexes` | PR badge and helper tolerated; links `unlisted` |
| `CLAUDE_CODE_FOOTER_INDICATOR`, settings `footer_indicator`, remote per-model indicator | `indicator` class; fullscreen/classic only. in the screen reader it loses its glyph and is `unlisted` |
| `subagentStatusLine` | replaces panel row bodies: such rows show no readable status and make no work claim |
| `feedbackDrafts`, `feedbackSurveyRate`, `CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY` | the digit band (composer unknown) |
| plugin render hooks (`AbovePrompt`, `PromptHint`) | unsupported: an undetectable digit band / an `unlisted` footer segment (§5 c11) |
| `tengu_copper_thistle`, `tengu_coordinator_panel` (server-gated) | unified footer (§1 row 88); the panel instead of agent pills (§1 row 13) |
| `TERM_PROGRAM` (set by tmux) | enables OSC 8 links |
| `TMUX` plus tmux on `PATH` | enables the tmux notices |
| `CLAUDE_CODE_TMPDIR` | where background-task output lands |
| `disableDeepLinkRegistration` | `"disable"` stops claude registering its url-handler bundle with LaunchServices on every start |
| `preferredNotifChannel` | bell only; excluded |

## 5. accepted costs and residual ambiguity

accepted costs ([spec §9](terminal-observation.md#9-final-state-costs-and-completion)),
each unknown or none, never a false claim unless the item says otherwise:

- **c1** activity under every request/menu pane is unknown: the dialog hides panel,
  pill and spinner. for claude, `working + permission|question` is
  unrepresentable.
- **c2** interruption and error are never notices (2.8): interrupted or failed
  turns read idle and can raise `ready`.
- **c3** a loading chevron with no corroborator is unknown. teammate views never
  reach idle.
- **c4** idle is lost when:
  - the slot is unproven: manual-mode rows whose `Hs` is gone and whose last item
    is not a complete non-indicator label with 6 blanks after it, and every row at
    the narrow widths 2.4 measures;
  - the pane is taller than 256 rows and the slot walk reaches rows `192..h−65` (a
    fullscreen pane that tall with a short transcript), or a byte cap cut the walk;
  - a footer link, a render hook or a screen-reader indicator is configured;
  - the footer is non-ordinary or void (bash, history recall, help, autocomplete,
    an unrecognized col-2 notification under the mode row);
  - a non-work panel row is present: idle teammates, `waiting` or
    `awaiting approval` agents, the idle summary, a paused workflow, a
    `subagentStatusLine` body;
  - usage-limit copy appears below the bottom rule, including a statusline or panel
    row that prints it;
  - colour level 0;
  - a screen-reader session has no completion row or `claude:` neighbour directly
    above an unwrapped mode row: every fresh screen-reader session until its first
    turn, every screen-reader session with a statusline, every screen-reader
    session while claude pins a notice.
- **c5** working is lost when an unknown col-2 notification sits in the composer
  margin while a statusline suppresses the hint: the notification becomes `S`.
- **c6** vim NORMAL mode is indistinguishable from an ordinary empty composer (U).
  it reads `empty`, so guarded send is admitted and relies on the existing paste
  staging.
- **c7** process pills do not count as work: a long background shell allows idle,
  and an auto-resumed turn flips back to working.
- **c8** a finished agent or workflow row (until evicted, ≤ ~30 s) and a `WAIT` row
  whose awaited work was stopped without a notification count as working. idle is
  delayed, never false.
- **c9** a pane taller than 256 rows whose lowest drawn row lies above its bottom
  region (a classic or pre-REPL surface near the top) reads unknown/unknown,
  `evidence_clipped`.
- **c10** a gated brief-mode spinner (S) may not match `SP`; the chevron still dims,
  so unknown, never idle.
- **c11** the digit band makes the composer unknown while visible; a col-2
  transcript row reading like a legend directly above the margin does the same.
  plugin render hooks are unsupported display configuration: an `AbovePrompt` band
  with digit seats is undetectable, so a single-character guarded send can reach
  it, and a `PromptHint` hook makes the footer `unlisted`.
- **c12** stale fullscreen cells after a resize, screen-reader announcements below
  `$`, and a dead process's frame during a relaunched claude's first second give
  unknown until repaint.
- **c13** the no-response retry variant draws a second row under `RT`; without the
  hint it reads unknown.
- **c14** a screen-reader draft of only spaces reads `empty`, so guarded send is
  admitted and appends to it.
- **c15** human-request dialogs without a rule (auto-mode offer, rate-limit options,
  consumer terms, trial expired) read unknown/unknown. live qualification of each
  is an [open blocker](issues/claude-unruled-request-dialogs.md).
- **c16** `← N agent(s)` reports other sessions' pending input; this terminal keeps
  interaction `none`, and `ready` can fire beside it.
- **c17** remote usage-limit copy: only the default copies are anchored. a remotely
  configured wrap-up lead and waiting copy without any anchor make an armed wait
  indistinguishable from idle (a false idle), and in the screen reader remote copy
  long enough to push the `⚠ ` lead more than 16 rows above `M′` hides the wait.
  the stricter rule, any unrecognized notification blocks idle, is rejected: it
  kills idle while any other notification is visible and in every brief-mode
  session.

residual ambiguity without a ruling:

- **r1** 2.1.284's theme picker reads unknown/unknown/none, composer unknown,
  `layout_unknown`, never a false claim, in measured cases below 40 columns
  ([issue](issues/claude-theme-picker-narrow.md)).

## 6. requalification

run this after a claude upgrade, before claiming the new version. it reuses no
retained harness; every probe is temporary and deleted before commit
([testing](rules/testing.md)), and every record is content-free.

1. diff the new bundle's strings and components against the anchors of 2.3–2.6:
   the footer vocabulary, the dialog copy, the panel status shapes, the pinned
   column and the digit-band call sites. mark each §1 row whose rendering or
   trigger changed. a numbered/unnumbered change like rule 24's is the expected
   shape of a version drift.
2. drive the installed tui against a local SSE mock of the messages api: a dummy
   key approved by its last 20 characters in `customApiKeyResponses`,
   `ANTHROPIC_BASE_URL` on 127.0.0.1, `--permission-mode default` plus
   `hasSeenAutoDefaultNotice`, and the nonessential-traffic, telemetry,
   error-reporting and autoupdater switches off. the mock steps per main-loop
   request, because claude merges assistant turns, and matches subagent and
   workflow requests by content. mcp servers come from `--mcp-config` plus
   `permissions.allow`; an `ask` rule naming `EnterPlanMode` produces the
   enter-plan dialog; a project `.mcp.json` with two servers produces the
   multi-server approval; a malformed settings file produces the settings error.
3. isolate every run: a temporary `HOME`, `CLAUDE_CONFIG_DIR` and
   `CLAUDE_CODE_TMPDIR` (claude ignores `TMPDIR` for task output and creates its
   socket directory under `/tmp` regardless), settings
   `disableDeepLinkRegistration: "disable"` (otherwise each temporary home
   registers a url handler with LaunchServices), `HISTFILE` pointed away from the
   user, and an isolated tmux server (`tmux -L <own socket> -f /dev/null`). pass
   the pane's own `TMUX`/`TMUX_PANE`/`TERM_PROGRAM`, the realistic skid condition,
   so tmux notices and OSC 8 links render. pressing ← in the agents view moves the
   conversation into a detached daemon whose state survives the tmux server: kill
   it and remove its state afterwards.
4. qualify every renderer in use (fullscreen, classic, screen reader) on each
   platform and tmux version, at the default width and at the narrow widths 2.4
   and rule 24 measure. classify through the real capture and `detect`;
   expectations come from the mock step and the keys sent, never from skid's
   output.
5. for families the mock cannot produce (the usage-limit wait, teammates, the §5
   c15 dialogs), author frames from the bundle and paint them through real tmux:
   that proves parser mechanics only, so the family stays `NOT_RUN`. pair each new
   rule with a negative and confirm a mutation of the rule flips it.
6. update §1's qualified column and versions, record the run in the
   [qualification](terminal-agent-control-qualification.md#terminal-observation-qualification),
   and file each new `NOT_RUN` required family as an issue.
