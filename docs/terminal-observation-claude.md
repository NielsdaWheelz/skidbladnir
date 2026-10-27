# Claude screen grammar

Implemented in `internal/agentcontrol/claude.go` (2.0-2.2), `claude_composer.go`
(2.3, 2.4, 2.8) and `claude_families.go` (2.6), whose comments cite the sections
below. [The spec](terminal-observation.md) owns the status contract this grammar
feeds. Qualified live for Claude 2.1.286 on Darwin (tmux 3.7c; fullscreen and
classic) and Claude 2.1.284 on Linux (devbox tmux 3.4, arch tmux 3.7c; fullscreen,
and classic for idle, draft, work, a single question, bash permission and tall
panes), as §1 records; §2.6 rule 24 (theme setup) is qualified on both versions,
except the narrow widths §5 c18 accepts.

Sources: `S`, string and code inspection of the bundled JavaScript in the
executable's `__BUN` segment (minified identifiers are build-specific and serve
only as orientation); `D`, the public statusline and accessibility docs; live
runs of the stock TUI against a local deterministic mock of the messages API. No
real screen, prompt, path, title or account datum was kept.

## 1. Frozen capability table

Classes: `R` required-positive; `P` permitted unknown/none; `U`
upstream-unavailable in the qualified configuration; `X` excluded by spec.

`qualified` names the platforms where the family ran live against the installed
versions above, with a renderer or width in parentheses when only that part ran.
`capture` means authored rows painted through real tmux on that platform,
classified with the evidence dropped from the real capture (on Darwin also by a
real byte cap): capture and parser mechanics only. `source-only` means `S`/`D`
and authored rows only. An `R` row is `NOT_RUN` on every platform it does not
name, and each such `NOT_RUN` is a blocker under spec §7
([unrun acceptance](issues/unrun-acceptance.md),
[unruled dialogs](issues/claude-unruled-request-dialogs.md)). A `P` row's
`NOT_RUN` is informative.

| # | Family / variant | Class | Result | Rule ID | Qualified |
| --- | --- | --- | --- | --- | --- |
| 1 | Generation spinner | R | working | `claude.activity.spinner` | Darwin, Linux |
| 2 | Tool spinner | R | working | `claude.activity.spinner` | Darwin |
| 3 | Reduced motion (`●`, frozen timer) | R | working | `claude.activity.spinner` | Darwin (classic), Linux (fullscreen) |
| 4 | Spinner at narrow width: details dropped; spinner row wrapped | R | working | `claude.activity.spinner` | Darwin (drop); wrap source-only |
| 5 | Spinner `⎿` child block: tip (wrapping, `spinnerTipLabel`), `Next: <task>`, expanded task list, compaction hint | R | working (S above the block) | `claude.activity.spinner` | Darwin (wrapped tip); the others source-only |
| 6 | Retry row `· Retrying in` | R | working | `claude.activity.retry` | Darwin |
| 7 | Retry with overload child `⎿  If it persists, check …` | R | working | `claude.activity.retry` | source-only: repeated 529s ended in an `API Error` row with no retry row |
| 8 | Retry variants: stalled, no-response, low-priority, auto-mode check | R | working (no-response: hint only) | `claude.activity.retry` | source-only |
| 9 | Interrupt hint in the mode row | R | working (conflict with a ready chevron) | `claude.activity.hint` | Darwin |
| 10 | Panel agent or workflow row with a running status | R | working | `claude.activity.background` | Darwin (agent, workflow), Linux (agent); nested rows source-only |
| 11 | Panel row with status `idle`/`waiting`/`awaiting approval`, `N idle agents` row, paused workflow row, row without a readable status | P | No work claim; blocks idle | — | source-only (teams and named agents need more than an API-key session) |
| 12 | Waiting row `✻ Waiting for …` as the slot row | R | working | `claude.activity.background` | Darwin (agents, workflow-only); mixed source-only |
| 13 | Work pill `N local agent(s)` | U | working | `claude.activity.background` | source-only: drawn only with the server-gated coordinator panel off, and the panel replaces it by default |
| 14 | Process pill (2.4) | P | Tolerated; no working claim | — | Darwin (`N shell`) |
| 15 | Unknown pill (MCP task, team, cloud, remote workflow, ultraplan, mixed, `↳ N background`) | P | Blocks idle; no working claim | — | source-only |
| 16 | Footer indicator `◆ <≤6 cells>` | P | Tolerated; proves nothing | — | Darwin |
| 17 | Ready layout, default footer | R | idle | `claude.activity.idle` | Darwin, Linux |
| 18 | Ready layout with a banner top rule (`--name`, `/rename`, `/color`, `--agent`) | R | idle; working as unbannered | `claude.activity.idle` | Darwin (idle, colour); bannered working source-only |
| 19 | Ready layout, custom multiline statusline | R | idle | `claude.activity.idle` | Darwin (classic) |
| 20 | Working with a statusline (hint suppressed) | R | working via spinner + loading chevron | `claude.activity.spinner` | Darwin (classic) |
| 21 | Loading chevron, no spinner/retry/hint | P | activity unknown | — | Never observed |
| 22 | Spinner/retry shape or hint with a ready chevron | P | activity unknown, `evidence_conflict` | `claude.activity.conflict` | source-only |
| 24 | Fullscreen/classic startup | U | The REPL mounts ready; no startup structure | — | Darwin |
| 31 | Permission: Bash, Write, Edit, wrapped at 26/40/60/80 | R | permission | `claude.permission.dialog` | Darwin, Linux (Bash at 100 columns) |
| 32 | Permission: MCP tool | R | permission | `claude.permission.dialog` | Darwin |
| 33 | Permission: dynamic workflow (`Run a dynamic workflow?`) | R | permission | `claude.permission.workflow` | Darwin (100/60/40) |
| 34 | Permission: Fetch, notebook, subagent- or plugin-sourced | R | permission | `claude.permission.dialog` | source-only |
| 36 | Question: single, free-text focus, multi-question, multiSelect, wrapped hint, chat-row focus | R | question | `claude.question.form` | Darwin, Linux (single) |
| 37 | Question with countdown `auto-continue in Ns · any key to stay` or plugin notice row | R | question | `claude.question.form` | source-only |
| 38 | Question with option previews, chat focus | R | question | `claude.question.preview` | Darwin (single question, including previews that draw their own boxes); the two-question hint source-only |
| 39 | Question submit review | R | question | `claude.question.review` | Darwin |
| 41 | Plan approval with plan, wrapped at 80/60/40 | R | confirmation | `claude.confirmation.plan` | Darwin (fullscreen, 30–80 columns); classic fails live: unknown/unknown at every width, its dialog drawn at col 1 where rule 15 reads col 3 |
| 42 | Exit plan mode without plan | R | confirmation | `claude.confirmation.exit_plan` | Darwin |
| 43 | Enter plan mode dialog (only under an `ask` rule naming `EnterPlanMode`) | R | confirmation | `claude.confirmation.enter_plan` | Darwin (80/40) |
| 44 | MCP elicitation form | R | input | `claude.input.elicitation` | Darwin (classic and fullscreen, 30–80 columns, with and without a task suffix: classic wraps the title, fullscreen cuts it) |
| 45 | MCP elicitation URL mode | U | input | `claude.input.elicitation` | source-only: gated, no dialog in an API-key session |
| 46 | Transcript viewer over a waiting dialog (`dialog waiting · Showing detailed transcript · …`) | R | input | `claude.input.dialog_waiting` | source-only |
| 47 | Setup: trust, API key, theme onboarding, security notes | R | setup | `claude.setup.<variant>` | Darwin (2.1.286, unnumbered theme), Linux (2.1.284, numbered theme; §5 c18) |
| 48 | Setup: login method | R | setup | `claude.setup.login` | Darwin |
| 49 | Setup: project MCP server (single / several) | R | setup | `claude.setup.mcp_server` / `claude.setup.mcp_servers` | Darwin (single at 80/40; several at 80/40/26) |
| 50 | Setup: bypass-permissions warning | R | setup | `claude.setup.bypass` | Darwin, Linux |
| 51 | Setup: settings error (invalid settings file) | R | setup | `claude.setup.settings_error` | Darwin (80/40) |
| 52 | Setup: auto-mode default offer | R | No rule: unknown/unknown | — | source-only (§5 c15) |
| 53 | Setup/input: rate-limit options, consumer-terms update, trial expired | R | No rule: unknown/unknown | — | source-only (§5 c15) |
| 55 | Provider announcement modal | P | interaction unknown | — | source-only |
| 56 | Digit band above the composer (2.3) | P | composer unknown | `claude.composer.band` | source-only |
| 57 | Usage-limit wait, in the pinned column (2.3) | P | Never idle; interaction and composer unknown; activity from spinner, hint or panel | `claude.limit.wait` | Darwin (the pinned column's placement); the wait itself source-only (§5 c17) |
| 58 | Menus: model, settings, theme, resume, help, /btw | R | menu | `claude.menu.<variant>` | Darwin (fullscreen); classic `/model` fails live: unknown/unknown at every width, its hint at col 2 under a `RULE` where rule 4 reads col 3 under an `EDGE`; the other menus not run in classic |
| 59 | Menus: transcript viewer, agents view | R | menu | `claude.menu.transcript` / `.agents_view` | Darwin |
| 60 | Menu whose hint is obscured (narrow or short pane) | P | unknown/unknown | — | Darwin (§5 c19: fullscreen `/model` reads menu only while its whole box fits the pane) |
| 61 | Viewed subagent (viewed glyph on a non-main panel row) | R | menu; activity from the panel | `claude.menu.subagent` | Darwin, Linux |
| 62 | Panel focus footer `↑/↓ to select …` | P | Void surface: unknown | — | Darwin, Linux |
| 63 | Left-labelled top rule (`── History N/M ──…`) | P | interaction and composer unknown | — | source-only |
| 64 | Other panes (/mcp, /agents, /permissions, /effort, task detail) | P | unknown unless a menu rule matches | — | source-only |
| 65 | Interruption row `⎿  Interrupted · What should Claude do instead?` as the transcript's last row above the composer | R | notice interrupted | `claude.notice.interrupted` | Darwin |
| 66 | Failure block (`⏺ API Error: …`, a model or usage-limit message) as the turn's last block: glyph and text in one explicit colour | R | notice error | `claude.notice.error` | Darwin |
| 67 | Footer notifications: effort (right-aligned), tmux notices | P | `notification` rows (2.0) | — | Darwin (tmux notices drawn because the pane's own `TMUX` is set) |
| 68 | Composer empty / draft / blocked / unknown | R | §3 | `claude.composer.*` | Darwin, Linux |
| 69 | Quoted chrome in the transcript | R neg | Ignored | — | source-only (authored) |
| 70 | Historical completion/waiting/interrupt/error rows | R neg | Ignored | — | Darwin, Linux (completion rows) |
| 71 | Stale frame of a dead Claude under a relaunched Claude | R neg | unknown/unknown | — | Darwin |
| 72 | Teammate view, cloud sessions, ultraplan | P | unknown | — | source-only |
| 73 | Vim editor mode: `-- INSERT/VISUAL/VISUAL LINE --` prefix / NORMAL | R / U | Prefix tolerated / NORMAL invisible | — | source-only |
| 74 | Compaction / hook spinners | R | working | `claude.activity.spinner` | source-only |
| 75 | Title, progress (OSC 9;4), bell | X | Never read | — | — |
| 76 | Classic tall pane, composer in the top region | R | As bottom | `claude.layout.composer` | Darwin, Linux |
| 77 | Tall pane, pre-REPL dialog in the top region or straddling it | R | As bottom | Family rule | Darwin (trust at 100 and 70 rows) |
| 78 | Pane taller than 256 rows: rows `192..h−65` absent; a read reaching them | P | unknown, `evidence_clipped` | `claude.region.clipped` | Darwin (classic and fullscreen at 300 rows), Linux (classic at 300 rows) |
| 79 | Byte-capped region | P | `evidence_clipped` only where a read reaches the cut | `claude.region.clipped` | capture: Darwin, Linux |
| 80 | Colour level 0 (`NO_COLOR`, `FORCE_COLOR=0`, `TERM=dumb`) | P | Never idle; working via hint/background only | — | Darwin (`NO_COLOR`); the others source-only |
| 81 | Bash mode `!` | P | Never idle; composer draft | — | Darwin |
| 82 | Fullscreen scrollback pill | P | Chevron still decisive | — | Darwin |
| 83 | Narrow mode row hiding an item | P | unknown unless the slot is proven (2.4; a non-default mode's cut cycle hint proves it, row 92) | — | Darwin |
| 84 | Classic notifications below / beside the mode row | R | Handled | — | Darwin |
| 85 | OSC 8 inside rows | parser | Zero-width | — | Darwin, Linux |
| 86 | Requests during background work | P | The dialog replaces panel and pill: activity unknown | — | Darwin |
| 87 | Linux rendering (`●` bullets, viewed glyph, spinner frames) | R | Same grammar | — | Linux: frames `· ✢ * ✶ ✻ ✽` read working, `●` bullets and viewed glyph read as on Darwin; `✳` not seen in 40 samples per host |
| 88 | Gated unified footer (`tengu_copper_thistle`) | R | Vocabulary covers it | — | source-only |
| 89 | Fullscreen stale cells after a resize | P | Exact match fails: unknown until repaint | — | Darwin |
| 90 | Other sessions needing input: footer `← N agent(s)` | P | interaction none (§5 c16) | — | source-only |
| 91 | Pinned notices (`⚠ ` items of the pinned column: transcript saving off, transcript writes failing) | P | Opaque footer rows, tolerated | — | Darwin |
| 92 | Narrow mode item: the cycle hint cut or wrapped away whole, or the row ending at `on` (2.4) | R | As the whole row: the hint proves the slot (P0); `on` with 4 blank cells to the row end does (P2) | `claude.activity.idle` / `.spinner` | Darwin (fullscreen and classic; production bypass arguments: idle, draft, spinner and tool at every width 30–60; auto mode and accept edits idle 20–60; plan mode idle 20–40; manual mode 16–40, empty and draft) |

## 2. Screen grammar

### 2.0 Primitives

**Cells and style.** Parse each physical row into cells. Use only attribute
presence:

- `FG`: explicit foreground (30–37, 90–97, `38;5;n`, `38;2;r;g;b`).
- `BG`: explicit background.
- `DIM`: SGR 2.
- `INV`: SGR 7.
- `BOLD`: SGR 1.

Never compare colour values. Claude's `dimColor` renders as the theme's
`inactive` foreground, not SGR 2. `chalk.dim` placeholders render as SGR 2.

**OSC 8.** OSC 8 open and close are zero-width in every row; tmux writes them
ST-terminated. Any other escape tmux's writer never emits makes only that row
unparseable, never the sample. An unparseable row is present but unrecognized: it
is not blank, has no column, matches no shape and ends every block, and it never
counts as clipped evidence. An unparseable slot row leaves activity unknown;
among the footer's opaque rows it is tolerated like any opaque row.

**Rows are self-contained.** Raw `capture-pane -e` carries style across rows; the
capture takes each row on its own, so every row parses from the default style
([spec §3](terminal-observation.md#3-composition-and-observation-boundary)).

**Columns.** A column is a cell index. Every glyph used here is one cell in
Claude's layout and in tmux (`⏸ ⏵ ← ↓ ✔ ☐` measured).

**Tokens.**

| Token | Code point |
| --- | --- |
| `·` | U+00B7 |
| `…` | U+2026 |
| NBSP | U+00A0 |
| Rule `─` | U+2500 |
| Edge `▔` | U+2594 |
| Dashed `╌` | U+254C |
| `⎿` | U+23BF |
| `❯` | U+276F |
| `⏸` | U+23F8 |
| `⏵⏵` | U+23F5 U+23F5 |
| `⏺` | U+23FA (Darwin bullet, viewed glyph) |
| `●` | U+25CF (reduced motion; Linux bullet and viewed glyph; survey bullet) |
| `◯` | U+25EF |
| `├` / `└` | U+251C / U+2514 |
| `☐` / `☒` / `✔` | U+2610 / U+2612 / U+2714 |
| `↳` | U+21B3 |
| `◇` / `◆` | U+25C7 / U+25C6 |

**Helpers.** These are the only helpers the grammar uses.

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
  `  tmux detected · ` or `  tmux focus-events off · `. This is the only
  notification predicate.

**Logical block `blk(c, end)`.** The rows from `end` upward while each row is
non-blank, has `|col − c| ≤ 1`, and is not a `MARKER` or `BOXROW`; its topmost row
may sit at any of those columns. Its text is the stripped rows joined by one
space. This is the wrapping contract: Claude wraps at word boundaries and
continues at the block's column; the `Syntax theme:` block continues one column
left, which the ±1 tolerance covers. A word longer than the line is hard-broken,
which can insert a spurious space inside a path; every matched token is
word-bounded, so matching never depends on those spaces.

**`texts(c)`.** `{ blk(c, r) }` for every row `r` that ends a block (the row below
`r` cannot extend it) and whose block's topmost row is at exactly column `c`. A
2.6 hint or anchor block `blk(c)` holds under the same condition (2.6: every
column is exact), which keeps col-2 transcript text out of lower columns. `above`
takes the plain `blk`. A family reads `texts(c)` at and above `L′`.

**Numbered options `opt(c, end)`.** Skip blank rows above `end`, then take the
contiguous rows that are either an option start `^ {c}(❯| ) N\. label` or a
continuation (`col ≥ c+5`: a wrapped label or a description). Numbers run 1..n,
at most one `❯` sits at column `c`, and labels are joined across their
continuations.

**Unnumbered options `uopt(c, end)`.** The contiguous rows `^ {c}(❯| ) \S` or
`^ {c+2}\S` with at most one `❯` at column `c`. A wrapped label continues at
`c+2`, where an unselected option also starts, so labels are not separable: the
result is the whole block's joined text and anchors are word-bounded substring
checks on it.

**Text above `above(c, row)`.** The plain block `blk(c, ·)` ending at the first
non-blank row above `row`.

### 2.1 Region selection

`B` is the bottom region and `T` the optional top region
([spec §3](terminal-observation.md#3-composition-and-observation-boundary)). `T`
holds every row above `B` up to row 191, so the captured rows are one row space
`0..h−1` unless a byte cap clipped a region or the pane is taller than 256 rows.
The rows a region dropped are absent; they form one run between `T`'s kept rows
and `B`'s kept rows. `floor` is the first row below that run (0 when nothing is
absent).

- **`L`.** The lowest non-blank row at or below `h−1` and at or above `floor`. If
  the scan reaches an absent row first, the lowest drawn row may lie in the cut:
  unknown/unknown, `evidence_clipped`, rule `claude.region.clipped`. If every row
  is blank and none is absent: unknown/unknown, `layout_unknown`, no rule. `L` in
  `T` and surfaces straddling `T` and `B` read like any other rows.
- **Dispatch** (2.2) runs on `L`.
- **Reads past the captured rows.** Every read moves upward from `L`, so the first
  absent row it can meet is `floor−1`; there it fails:
  - The dependent dimension is unknown with `evidence_clipped` and rule
    `claude.region.clipped`. Required reads are the slot walk (activity), the
    composer anchor search and a family's anchors (interaction). An intact lower
    composer whose walk ends at or below `floor` is unaffected.
  - If no dispatch step matches and some read reached the cut: unknown/unknown,
    `evidence_clipped`, `claude.region.clipped` (a classic pane taller than 256
    rows whose lowest drawn row lies above the bottom region). A cut that no read
    reached does not make the reading clipped.

### 2.2 Dispatch (first match)

1. Composer surface, when the composer anchor holds (2.3).
2. Families at `L′`, in the order of 2.6.
3. Nothing matched: unknown/unknown/none, composer unknown, `layout_unknown`. An
   unknown pane is never promoted to `input`.

### 2.3 Composer surface (fullscreen, classic)

**Anchor (`claude.layout.composer`).** `b` is the first `RULE` at or above `L`,
within 30 rows. Above `b`, skip continuation rows (col ≥ 2 or blank; at most 40)
to the first other row `c`. The anchor holds iff:

- `c` is a chevron row: `❯` NBSP at col 0 (`❯` alone once tmux trims the NBSP), or
  `!` NBSP in bash mode. A `❯` + ordinary space, or a `❯` with `BG`, is a
  historical user prompt, never a chevron;
- The row above `c` is the top rule `t`: a `RULE`, a `BANNER` or a `LEFT_LABEL`.

Otherwise the anchor fails and dispatch continues at step 2. No other `RULE` is
tried: a dead process's frame above a newer surface can never capture it. Rules
indented by 2 are quoted text.

`STYLE_LIVE` holds iff the first cell of `t` has `FG`. The rule is
`promptBorder`-coloured in every theme and takes the banner colour under `/color`;
it has no `FG` at colour level 0.

**Top rule.** `RULE` and `BANNER` are the ordinary composer. `LEFT_LABEL` is not:
interaction unknown, composer unknown; activity steps 1–2 below still apply, idle
never does.

**Footer.** The rows `b+1 .. L` are **accounted** iff they form one of:

- **Ordinary:** zero or more opaque rows (any content: first Claude's pinned
  column, pinned notices such as `⚠ Transcript saving is off …` and the usage-limit
  wait, then the statusline rows), the mode row `M` (the lowest row matching 2.4),
  then the **tail**: zero or more `notification` rows, then optionally exactly one
  blank row and the panel.
- **Bash:** row `b+1` has `plain == "  ! for shell mode"`, the chevron is `!`, then
  the tail. Claude returns this footer early in bash mode, so no hint or pill
  segment can coexist with it (S).
- **Help grid (`claude.menu.help`):** the first footer row starts
  `  ! for shell mode` followed by more columns, and a footer row contains
  `/ for commands`. Claude draws the grid instead of the mode row (S), so `M` is
  absent. Interaction `menu`, composer `blocked`.

Any other footer leaves the surface **void**: activity, interaction and composer
unknown, idle impossible, rules `claude.layout.composer` only. This covers
autocomplete, history search, `Press … again to exit`, `Pasting…`,
`paste again to expand`, the panel focus footer, any block drawn under the prompt,
an unrecognized col-2 notification, and the resume hint and shell prompt printed
under a dead process's frame while a relaunched Claude has not yet painted.

**Panel.** Claude draws it one blank row below the footer (`marginTop 1`, S).
Rows, in order:

| Row | `plain` | Note |
| --- | --- | --- |
| Main | `^(  \|❯ )[◯⏺●] main( +↑ \d+ more)?$` | Drawn only while an agent row exists or an agent is viewed; absent above workflow-only panels |
| Agent | `^(  \|❯ )((  )*[├└] )?[◯⏺●] \S` | Nested rows carry a tree connector (S) |
| Idle summary | `^(  \|❯ )◯ \d+ idle agents?$` | Checked before the agent shape |
| More | `^  ↓ \d+ more$`, or one blank row in its place | The blank row appears with more than 5 agent rows (S) |
| Workflow | `^(  \|❯ )[◯⏸] \S` | `⏸` while paused on a rate limit (S); after agent rows |

The panel is a contiguous run of these rows (one blank row allowed where `more`
stands). An agent or workflow row is a **work row** iff its glyph is not `⏸` and
`plain` ends with a running status:

```text
\s(\d+/\d+ · )?\d+[dhms]( ?\d+[dhms])*( · [↑↓] \S+ tokens)?( · \d+ queued)?$
```

(`15s · ↓ 27 tokens`, `1m 26s`, `0/1 · 57s`, `1/1 · 2m05s · ↓ 2 tokens`). Claude
draws `idle` for an idle teammate and a finished named agent, `waiting` for an
idle unfinished local agent, `awaiting approval` for a teammate awaiting plan
approval (S): rows ending `\s(idle|waiting|awaiting approval)( · \d+ queued)?$`,
the idle summary, `⏸` rows and rows whose status is unreadable (a
`subagentStatusLine` body, a status cut at a narrow width) are accounted but make
no work claim; they block idle. A finished agent or workflow keeps its duration and
a status-coloured glyph until evicted (S: 30 s after its completion notification;
~25 s observed), so it counts as work meanwhile (§5 c8). Style cannot separate
running from finished rows.

**Viewed subagent (`claude.menu.subagent`).** A non-main agent row carries the
viewed glyph `⏺` (`●` on Linux). Interaction `menu`, composer `blocked`, activity
from the panel. Typing there messages the subagent. The banner on the top rule is
not part of the test: named sessions draw the same banner.

**Slot row `S`.** Start at `t−1`:

1. Skip up to 6 `notification` rows. The composer's one-row top margin holds
   them. A live spinner cannot be passed: its child block begins `⎿` or sits at
   col ≥ 5.
2. Skip blank rows.
3. If the row is at col 2 beginning `⎿` or at col ≥ 5, it is the bottom of the
   spinner's child block: walk upward over rows at col ≥ 5 to the row at col 2
   beginning `⎿`, within 12 rows including it, and continue above that row. The
   block is a tip (wrapping at col 5), `Next: <task>`, the expanded task list (≤ 5
   tasks of ≤ 2 rows each plus `… +N pending`), the compaction hint, or the
   overload row under a retry (S). If no `⎿` row is found within 12 rows, stop at
   the original row.
4. `S` is the current row. If `S` is at col 0, `plain(S)` matches `^\(.*\)$` and
   the row above is an `SP`-shaped row without its details, `S` is that row above
   (a wrapped spinner, S).

**Shapes of `plain(S)`, at col 0, style ignored.**

| Shape | Regex |
| --- | --- |
| `SP` | `^[·✢✳✶✻✽*●] \S.*?(…\|\.\.\.)( \(.*)?$` and not `DONE`/`WAIT`. Messages are free (spinnerVerbs, `Compacting conversation`, hook runs, an in-progress task's `activeForm`); never whitelist verbs |
| `RT` | `^✻ .*( · Retrying in \| · will retry in \| · next try in \|No response from the API after )` |
| `DONE` | `^✻ (Baked\|Brewed\|Churned\|Cogitated\|Cooked\|Crunched\|Sautéed\|Worked) for ` |
| `WAIT` | `^✻ Waiting for( \d+ background agents?)?( and)?( \d+ dynamic workflows?)? to finish$`, with at least one count |

`WAIT` is the end-of-turn duration row drawn instead of `DONE` while agents or
workflows the turn launched are still running or unnotified. Its counts are a
snapshot taken when the row mounts (S). It counts as current only while it is the
slot row: the completion notification of the awaited work starts a new turn whose
rows land below it. With `showTurnDuration` off Claude draws no row at all.

**Digit band (`claude.composer.band`).** Claude draws optional prompts in the area
above the composer (each with `marginTop 1`, the composer's own top margin below
them), and a single key typed into the composer goes to them: one character that
is a digit after NFKC, an AZERTY digit key (`& é " ' ( - § è _ ç à`) or, for the
transcript-share prompt, `y`/`n`/`d` (S). The shared capture has exactly six call
sites in 2.1.286; each band's legend is its lowest row, so it is `S` (in brief mode
the composer has no top margin and the legend sits directly above `t`, which is
why step 1 never skips it). If `S` matches a legend at its exact column, the
composer is unknown:

| Call site | Column | Legend (`plain`) |
| --- | --- | --- |
| Rating surveys (session, memory, plugin, post-compact, long-context) | 2 | `^  1: Bad\b` |
| Transcript-share prompt | 2 | `^  y: Yes\b` |
| Thanks row after a rating | 0 | `^\(Optional\) Press \[1\] to tell us (what went well\|what went wrong\|more) · /feedback$` |
| Feedback-draft card | 0 | `^1 to review · 2 to send · 0 to dismiss`; `^Send without reviewing \(full draft \+ env, no transcript\)\? 2 to send · Esc to back$`; `^.* 1 to review & retry · Esc to dismiss$`; `^Turn off Claude-drafted feedback\? 0 to turn off · Esc to keep$` |
| Web-setup offer after a push | 2 | `^  1: Yes, run /web-setup\b` |
| Plugin `AbovePrompt` band | — | Arbitrary plugin copy: undetectable (§5 c11) |

The survey follow-up text box (`^  Enter to (send|skip) · Esc to (clear|skip)\b`,
col 2) owns every key while open and is treated the same. The band changes
neither activity nor interaction. A col-2 transcript row reading like a legend
directly above the margin costs only an unknown composer.

**Usage-limit wait (`claude.limit.wait`).** Claude builds the quota auto-resume
state as up to three items (`limit-status`, `limit-next`, `limit-promo`; S) and
draws them only in its pinned column, a `flexDirection: column, paddingX: 2` box
between the editor's bottom rule and the footer that first holds the pinned
notices (S; 2.1.286 reads the limit context in that one component). The first item
is `⚠ ` plus its copy at col 2, each later item has `paddingLeft 2` (col 4), and
every item wraps in its own column (`wrap: "wrap"`), never right-aligned. The
ordinary footer accounts the column as opaque rows from `b+1`, above the
statusline rows (observed with a pinned notice in fullscreen at 80 and 40 columns
and in classic under a two-row statusline). The check reads the rows `b+1 .. L` as
one text (stripped rows joined by one space, a blank row adding an empty text),
since a wrapped item splits its copy across rows. The copies, any of:

- The fixed status copy and every default wrap-up lead: `Usage limit reached`,
  `Your usage limit has reset`;
- The default next-line copy: `Continuing automatically `,
  `Continuing shortly · esc to cancel`, `Press enter to continue`.

A match makes idle impossible and interaction and composer unknown; activity steps
1–2 still apply (a wrap-up turn keeps its spinner). No row above the composer is
read: Claude also appends a transcript notice when the wait arms
(`Usage limit reached · continuing automatically {when} · esc to cancel`, a dimmed
`⏺` system row; S), and that row is history. The wrap-up lead
(`tengu_lantern_sconce_copy`, ≤ 100 chars) and the waiting copy
(`tengu_functional_llama`) are remotely configurable and may drop every anchor
(§5 c17). A statusline or panel row printing the same words costs an unknown.

**Chevron classes** (cell 0 of `c`, `❯` only):

| Class | Condition |
| --- | --- |
| `LOADING` | `FG` or `DIM` (`dimColor: isLoading`) |
| `READY` | `STYLE_LIVE` and none of `FG DIM INV BG` |

Otherwise neither (style not live). A bash `!` is always `bashBorder` `FG`; busy
and ready differ only in colour value, so it is never `READY` or `LOADING`.

**Activity** (accounted surfaces only; first match):

1. **Background:** a work row, or `S` is `WAIT`, or a `work` segment in `M`:
   working (`claude.activity.background`). This runs before the conflict test: the
   panel and the waiting row are independent of the chevron and slot.
2. **Corroborator:** `S` is `SP` or `RT`, or `M` has an `interrupt` segment.
   - `READY`: unknown, `evidence_conflict` (`claude.activity.conflict`).
   - `LOADING`: working (`claude.activity.spinner`/`.retry`/`.hint`, first
     corroborator in that order).
   - Otherwise, with an `interrupt` segment: working (`claude.activity.hint`).
   - Otherwise unknown.
3. **Idle** (`claude.activity.idle`) requires all of:
   - `READY`;
   - An ordinary footer with a recognized `M`, and no usage-limit wait;
   - No panel row other than `main`;
   - The pill slot proven (P0, P1 or P2, 2.4);
   - Every `M` segment in class `tail`, `agents`, `process`, `indicator` or
     `other` (2.4);
   - None of steps 1–2 matched.

   Tolerated: the statusline, a banner, process pills, the indicator,
   notification rows, the band, placeholder, draft, and interrupt/error/completion
   rows in the slot.
4. Otherwise unknown: `LOADING` with no corroborator, a left-labelled rule, bash
   mode, colour level 0, the usage-limit wait, a non-work panel row, an unproven
   slot, a `work`/`unknown_pill`/`unlisted` segment, an unparseable slot row.

**Interaction.**

- `unknown`: a void surface, a `LEFT_LABEL` top rule, or the usage-limit wait.
- `menu`: the viewed subagent, or the help grid.
- `none`: an ordinary footer with `M` recognized, or the bash footer. Other
  sessions' requests (`← N agents`, teammates) remain outside this terminal's
  interaction (§5 c16).

### 2.4 Mode row, pill slot and footer vocabulary

**Grammar** (over `plain(M)`):

```text
^  (-- (INSERT|VISUAL|VISUAL LINE) -- )?(⏸|⏵⏵) (manual mode|plan mode|accept edits|auto mode|bypass permissions|don't ask) on( \((\S+) to cycle\))?
```

Then zero or more ` · <segment>`, where a segment contains no ` · `.

- A run of ≥ 3 spaces followed by text starts a right-aligned notification suffix
  (classic), not parsed.
- A trailing ` ·` means a later item was cut.
- Outside manual mode, a narrow row may show only the mode item's first wrapped
  line (layout below): `on`, then either ` (` and a character prefix of
  `<chord> to cycle)` taken token by token, as a cut segment is (earlier tokens
  whole, the last possibly partial), or nothing. Claude wraps
  at word boundaries, so live rows show whole tokens (`(shift+tab to`,
  `(shift+tab`). After a cut hint come two or more spaces and the separator, a
  notification suffix, or the row end. After `on` alone, two or more spaces and
  then `·` mean the hint wrapped away whole; `on` before ` · ` or a notification
  suffix has no hint, and `on` at the row end is left to P2. Manual mode never
  draws the hint, so these forms do not parse there.

**Layout** (S, and measured). The footer container has `paddingX 2`; the row is
`height:1, overflow:hidden`, items in order

```text
mode · [indicator] · [bg-detach ←] · [PR] · [links…] · [task pill] · [artifacts] · Hs
```

Every item before `Hs` is a `flexShrink:0` box. The mode item's separator is its
own box; every other item carries its trailing ` · ` inside its box. When the row
overflows, either the row is cut at `W − 2` (a character prefix: `1 shel` at 40,
`1 she` at 39) or a box receives fewer cells than its natural width and shows only
the first line of its word-wrapped text, its other cells blank, while later boxes
vanish. Measured in manual mode with indicator `a bcde` and a shell pill:
`1 shell` complete + 3 blanks at 42; `1` + 5/4/3 blanks at 38/37/36; `◆ a` + 4 at
28; `⏸ manual mode` + 4 at 19; an 8-letter indicator left `◆` + 6 at 28. So the
blanks after a partial first line are unbounded, while a complete box leaves at
most 3: one interior cell when its trailing ` ·` wraps away, plus the 2-cell
padding. `Hs` is the only `wrap:"truncate"` child: it receives width only after
every preceding box has its full width, and it alone ends in `…` when cut. The
indicator is `◆ ` + text cut to ≤ 6 cells by Claude itself (`tt(re(text,32),6)`),
so it may end in `…` without being `Hs`.

The mode item is one text in its own box: the mode in its colour, then the
dimmed cycle hint. When the row cannot hold the item, the box shows the text's
first wrapped line. That line keeps its trailing space and the box is as wide as
the text's widest line, so one blank cell or more precedes the separator box
(`⏸ plan mode on` + 6 at 26–28 columns, where the hint's own line is the widest).
Measured with the production bypass arguments, fullscreen and classic alike,
empty composer (`Hs` is `← for agents`; working draws `esc to interrupt` there
instead, with the same cuts):

| Columns | Mode row after the padding |
| --- | --- |
| 53–60 | `⏵⏵ bypass permissions on (shift+tab to cycle) · ` + `Hs` cut with `…` (`…` at 53, `←…` 54, `← …` 55, `← f…` 56 … `← for a…` 60) |
| 51–52 | `… cycle) ·` |
| 49–50 | `… cycle)` |
| 48 | `… on (shift+tab to  · ←…` |
| 47 | `… on (shift+tab to  · …` |
| 45–46 | `… on (shift+tab to  ·` |
| 42–44 | `… on (shift+tab to` |
| 39–41 | `… on (shift+tab` |
| 36–38 | `… on  · ← f…` to `… on  · ← for…` |
| 34–35 | `… on  · ←…`, `… on  · ← …` |
| 33 | `… on  · …` |
| 31–32 | `… on  ·` |
| 30 | `… on` + 4 blank cells |
| 28–29 | `… on` + 2–3 blank cells |
| ≤ 27 | `⏵⏵ bypass permissions`: `on` wrapped away |

A draft hides `Hs`, so the row ends after the item: whole at 49 and wider, then
the same first lines, and `… on` + `W − 26` blank cells at 38 and narrower. Auto
mode, accept edits and plan mode show the same forms at widths narrower by about
the length their names save, their box wider than its first line where the
hint's own wrapped line is the widest (`⏸ plan mode on` + 6 then the separator at
26–28; `⏵⏵ auto mode on` + 6 then the separator at 26–28, + 7–8 to the row end
at 24–25). With a pill there is no hint: the row reads `… on · 1 shell · …` cut
further, `… on ·` at 30–31, `… on` + 2–3 blank cells at 28–29 and
`⏵⏵ bypass permissions` at 25–27, alike in fullscreen and classic. Manual mode
never draws the hint; with `Hs` it reads `… on` + 2–3 blank cells at 20–21 and
`… on ·` at 22–23, and a draft reads `… on` + `W − 18` blank cells. So a hint-less
item shows its separator's `·` whenever 4 cells follow it.

**`Hs` members** (instances: `<chord>` is one token; `N` is digits or `99+`):

`<chord> to interrupt`, `<chord> to return to team lead`, `<chord> to hide tasks`,
`<chord> to show tasks`, `/tasks to see subagents`, `? for shortcuts`,
`← for agents`, `← N agent`, `← N agents`, `← N done`, `N feedback draft`,
`N feedback drafts`, `keep holding…`, `ctrl+c to copy`,
`option+click to native select`, `shift+click to native select`,
`set macOptionClickForcesSelection in VS Code settings`, `hold <chord> to speak`,
`↓ to manage`, `<chord> to view tasks`, `<chord> to view artifacts`.

**Segment classes** (closed; the vocabulary is the same for all renderers):

| Class | Labels | Effect |
| --- | --- | --- |
| `interrupt` | `<chord> to interrupt` | Corroborator (2.3) |
| `tail` | Every other `Hs` member except the agents hints; and the bare `…` | Tolerated; a complete one proves the slot |
| `agents` | `← for agents`, `← N agent(s)`, `← N done` | Tolerated; proves nothing: bg-detach draws the same copy before the pill. `← N agent(s)` counts other sessions reporting `needsInput` (S; §5 c16) |
| `work` | `N local agent(s)` | working (drawn only with the coordinator panel gate off) |
| `process` | `N shell(s)`, `N shell(s), N monitor(s)`, `N monitor(s)`, `N Artifact comment monitor(s)`, `dreaming`, `auto-mode scan`, `memory import`, `memory import N/N` | Tolerated; a surviving task count alone is not work |
| `unknown_pill` | `N MCP task(s)`, `N team(s)`, `◇ N cloud session(s)`, `◇ N remote dynamic workflow(s)`, `◆ ultraplan ready`, `◇ ultraplan`, `◇ ultraplan needs your input`, `↓ to view`, `N background task(s)`, `↳ N background`, `↳ N background (<chord> to manage)` | Blocks idle; no working claim |
| `indicator` | Fullscreen/classic only: `◆ ` then 1–6 cells (the last may be `…`) | Tolerated; proves nothing |
| `other` | `PR #N`, `MR !N`, `gh auth login for PR status`, `install gh for PR status` | Tolerated |

Local workflows never reach the pill (S: its filter drops them unconditionally);
they are panel rows.

**Classification.**

- A segment that is an exact instance of a label is in that label's class
  (instances of different classes never coincide). So is any segment but the last
  that has the indicator shape.
- Only the last segment can be cut, in two ways: `Hs` truncates with `…`; a box is
  cut bare (character prefix or first wrapped line). A last segment that is not an
  exact instance collects the classes it could have been cut from:
  - Ending in `…`: let `r` be the segment without the `…`, trailing spaces and one
    trailing `·`. An empty `r` is `tail`; otherwise the classes of the `Hs` members
    of which `r` is a character prefix;
  - Bare: the classes of the box labels (`agents`, `work`, `process`,
    `unknown_pill`, `other`) of which the segment is a character prefix. The
    agents labels are both `Hs` members and box labels (bg-detach draws the same
    copy), so a bare `←` or `← for` is `agents`;
  - Plus `indicator` if it has the indicator shape.

  Exactly one class: that class. None, or several: `unlisted`, which blocks idle.
- Every other segment is `unlisted`.
- A character prefix is taken token by token: earlier tokens equal (`<chord>` any
  token, `N` any count), the last token a character prefix. A first wrapped line is
  a word prefix, hence also a character prefix.

A cut `work` or `unknown_pill` pill stays in its class or becomes ambiguous (`1`
prefixes `1 shell`, `1 local agent` and `1 MCP task`); it never lands in a
tolerated class alone. `esc to i…` is `interrupt`. A bare last `◆ ultra` prefixes
`◆ ultraplan ready` and has the indicator shape: `unlisted`. User-configured footer
links and an extra `PromptHint` render hook are `unlisted` (§5 c4).

**Pill slot proven** iff one of:

- **P0**: the mode is not `manual mode` and the mode item draws its cycle hint:
  a complete ` (<chord> to cycle)`, or the item's first wrapped line with the
  hint cut or wrapped away whole (grammar above). Claude draws the
  cycle hint only when the count of {non-default mode or coordinator, task pill}
  is below 2, so in a non-default mode any part of the hint proves that no task
  pill exists; the gated footer chooses its cycle hint only after the `manage`
  state, which a pill forces. Observed: with a shell pill the cycle hint never
  showed in accept-edits, plan, auto or bypass mode; it appeared as soon as the
  pill ended.
- **P1**: some segment is a complete `tail` label, or the last segment ends in `…`
  and its `r` is empty or a character prefix of an `Hs` member instance. Both mean
  `Hs` laid out, which happens only after every preceding box has its full width.
  The indicator's own `…` (`◆ Revie…`) proves nothing.
- **P2**: no trailing ` ·`; the last item (the last segment, or the mode item when
  there is none) is an exact, complete instance of a fixed-vocabulary label: not of
  the `indicator` class, not a bare prefix, not ending `…`; and at least 6 blank
  cells follow it up to the row end or the notification suffix. A complete box
  leaves at most 3 blanks behind it, and a complete label followed by any item
  shows its own ` ·` unless at most one cell of its box remains; bare and
  indicator-shaped last items are excluded because a wrapped first line leaves
  unbounded blanks (`PR` of `PR #123456`, `◆ a`). When the last item is the mode
  item and the row ends without a notification suffix, 4 blank cells suffice: the
  item's separator is a box of its own whose `·` shows once 4 cells follow a
  hint-less item (measured above, manual and bypass alike). A hinted item's box
  may hold those cells itself, but then its hint, whole, cut or wrapped away,
  proves the slot (P0; manual mode draws no hint).

Measured: 60, 50 and 45 columns prove the slot (P1: `…`, `← for…`); 42
(`1 shell` + 3), 40 (`· 1 shel`), 35–38 (`· 1`), 33–34 (trailing ` ·`), 31–32
(indicator) and narrower do not. In a non-default mode without a pill every row
of the table above from 30 columns proves it (P0; P2 at 30), and a manual-mode
draft proves it from 22 columns (P2: `on` + 4). Three narrow losses remain, each
by its own mechanism (§5 c4, c5):

- Where `Hs` is cut to a bare `←` (`←…`, `← …`: 34–35, 48 and 54–55 columns in
  bypass) the slot is proven, but the segment prefixes both the agents labels and
  `<chord>`-led members, so it is `unlisted` and blocks idle;
- At 28–29 columns `on` with 2–3 blank cells leaves the slot unproven: idle is
  lost, working still reads;
- At 27 columns and narrower `on` itself wraps away (`⏵⏵ bypass permissions`),
  so the mode row does not parse and the footer is void: idle and working are
  both lost.

### 2.6 Request and menu families (no composer anchor)

**`L′`.** Start from `L` and skip at most two trailing right-aligned rows (≥ 8
leading spaces). This covers the question's countdown and plugin-notice rows (S)
and fullscreen trailing notices.

Every column below is exact. Anchors are rows or joined blocks at their column;
transcript text sits at col ≥ 2 behind `⏺`/`⎿`. Families that need a col-0 or
col-1 anchor or a full-width rule are therefore unreachable by quoted text, and so
is any family that needs a hint block at `L′`: in viewers the viewer footer
occupies `L′`. Composer is `blocked` for every family.

| Order | Rule ID | Hint block at `L′` | Required anchors | Result |
| --- | --- | --- | --- | --- |
| 1 | `claude.input.dialog_waiting` | `blk(2)` starts `dialog waiting · Showing detailed transcript · ` | — | input |
| 2 | `claude.menu.transcript` | `blk(2)` starts `Showing detailed transcript · ` | — | menu |
| 3 | `claude.menu.agents_view` | `blk(2)` matches `^(.* · )?enter to return( · \|$)` | A row containing `N awaiting input · N working · N completed` | menu |
| 4 | `claude.menu.model` | `blk(3)` starts `Enter to set as default · ` | An `EDGE` above | menu |
| 5 | `claude.menu.settings` | `blk(3)` starts `Type to filter · ` or `Enter/Space to change · ` | A row `^   Settings +Status +Config +Usage` | menu |
| 6 | `claude.menu.theme` | `blk(3)` == `Enter to select · Esc to cancel` | A row starting `    Syntax theme: ` | menu |
| 7 | `claude.menu.resume` | `blk(5)` contains `to show all projects` and ends `Esc to cancel` | — | menu |
| 8 | `claude.menu.btw` | `blk(4)` ends `Esc to close` | An `EDGE` and a row starting `    /btw ` | menu |
| 9 | `claude.permission.dialog` | `blk(1)` matches `^\S+ to cancel( · \S+ to amend)?$` | `opt(1, above the block)` with option 1 starting `Yes`; in `above(1, options)` the last row starting `Do you want to ` joined with the rows below it ends `?` | permission |
| 10 | `claude.permission.workflow` | `blk(2)` matches `^\S+ to cancel( · \S+ to amend)?( \S+ to edit script in \$EDITOR)?$` | `opt(2, above the block)` with option 1 `Yes, run it` and last option `No`; the first non-blank row above the options with col < 2 is at col 1, reads exactly `Run a dynamic workflow?`, and has a `RULE` directly above it | permission |
| 11 | `claude.question.preview` | `blk(0)` starts `Enter to select · `, contains ` · n to add notes · `, ends `Esc to cancel` | One blank row, then `^(❯ \|  )Chat about this$`; directly above it a `RULE`; above that (skipping blanks) the side-by-side block, whose first row holds the preview box corner `┌` at column `p`: scanning up from the `RULE`, the first row whose leftmost `┌` has cells left of it that start option 1 (`^(❯\| ) 1\. `), since a lower `┌` belongs to a box the preview content draws; the cells left of `p` of its rows form `opt(0, ·)` from the block's first row, with ≤ 1 `❯`; at least one `❯` among the options and the chat row | question |
| 12 | `claude.question.form` | `blk(0)` starts `Enter to select · `, contains ` to navigate`, ends `to cancel`, does not contain `n to add notes` | One blank row, then `^(❯\| ) N\. Chat about this$`; directly above it a `RULE`; then `opt(0, ·)` with ≤ 1 `❯`; at least one `❯` among the options and the chat row | question |
| 13 | `claude.question.review` | None; `opt(0, L′)` has labels exactly `Submit answers`, `Cancel` | `texts(0)` contains `Ready to submit your answers?` and `Review your answers` | question |
| 14 | `claude.input.elicitation` | `blk(2)` starts `Esc to cancel · ` | A block in `texts(2)` reading `MCP server “<name>” ` then `requests your input` or `wants to open a URL`, alone or followed by a space and more text (a task's elicitation appends ` (task <id>)`, S), or, cut by fullscreen, a character prefix of either and `…` | input |
| 15 | `claude.confirmation.plan` | Optional `blk(3)` matching `^\S+ to edit in `, then `opt(3, ·)` | Option 1 starts `Yes`; an option starts `Tell Claude what to change`; `above(3, options)` starts `Claude has written up a plan and is ready to execute.` | confirmation |
| 16 | `claude.confirmation.exit_plan` | None; `opt(4, L′)` (the hint lies below the pane) | Option 1 starts `Yes, and switch to `, last option `No`; `above(4, options)` == `Claude wants to exit plan mode` | confirmation |
| 17 | `claude.confirmation.enter_plan` | None; `uopt(2, L′)` ends `No, start implementing now` (the confirm label names the mode: `Yes, enter plan mode` or `Yes, and switch to plan mode (…) for this session`) | The first non-blank row above the options with col < 2 is at col 1, reads exactly `Enter plan mode?`, and has a `RULE` directly above it | confirmation |
| 18 | `claude.setup.trust` | `blk(1)` == `Enter to confirm · Esc to cancel` | `uopt(1, ·)` contains `No, exit` and `Yes, I trust this folder`; `texts(1)` contains `Accessing workspace:`; a `RULE` above | setup |
| 19 | `claude.setup.apikey` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` present; `texts(2)` contains `Detected a custom API key in your environment` and `Do you want to use this API key?`; a `RULE` above | setup |
| 20 | `claude.setup.bypass` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` contains `No, exit` and `Yes, I accept`; `texts(2)` has a block starting `WARNING: Claude Code running in Bypass Permissions mode`; a `RULE` above | setup |
| 21 | `claude.setup.mcp_server` | `blk(2)` == `Enter to confirm · Esc to cancel` | `uopt(2, ·)` contains `Use this MCP server` and `Continue without using this MCP server`; `texts(2)` has a block starting `New MCP server found in this project: `; a `RULE` above | setup |
| 22 | `claude.setup.settings_error` | `blk(2)` == `Enter to confirm · Esc to cancel` | `opt(2, ·)` has options `Exit and fix manually` and `Continue without these settings`; `texts(2)` has a block starting `Settings Error`; a `RULE` above | setup |
| 23 | `claude.setup.mcp_servers` | `blk(1)` matches `^\S+ to select · \S+ to reject all$` | Directly above it the submit row `       Enable selected` or `  ❯    Enable selected`; above that the contiguous checkbox rows `^(  ❯ \|    )\[[ ✔]\] \S`; ≤ 1 `❯` among them and the submit row; `texts(2)` has a block starting `N new MCP servers found in this project`; a `RULE` above | setup |
| 24 | `claude.setup.theme` | `blk(2)` starts `Syntax theme: ` | Options at col 1: from the lowest row of either form above the block, the contiguous rows of either form or at col ≥ 6; one `❯`; all in one form: every row unnumbered (`^ (❯ \|    )\S`, none numbered or at col ≥ 6; `✔ ` before the current label; 2.1.286) or the run parsing as `opt(1, ·)` (labels wrapped at col 6, `✔` after the current label; 2.1.284); they include `Dark mode (ANSI colors only)`; `texts(1)` contains `Let's get started.` | setup |
| 25 | `claude.setup.login` | None; `opt(1, L′)` | Option 1 starts `Claude account with subscription`; `texts(1)` contains `Select login method:` | setup |
| 26 | `claude.setup.security` | `blk(1)` == `Press Enter to continue…` | `texts(1)` contains `Security notes:` | setup |

Hint and option copy uses remappable chords only where written `<chord>` or `\S+`;
everything else is the fixed copy of the qualified versions (2.1.286, and 2.1.284
where §1 names it).

The order resolves every shared `L′` signature. Rules 19–22 share hint and column
(Claude's shared dialog: rule at col 0, title, body, options and hint at col 2) and
differ by anchors. Rule 18 differs by column. Rule 23 has its own hint at col 1 and
checkbox options. Rule 10 differs from rule 9 by column (options and hint at col 2,
title at col 1); rule 17 shares rule 10's column-1 title under a `RULE` but has no
hint. The theme menu (rule 6) and theme setup (rule 24) differ by hint and by
`Let's get started.`. Rules 11 and 12 differ by `n to add notes` and by the chat
row's number. Rule 1 precedes rule 2 because the `dialog waiting` prefix marks a
request held behind the viewer (S).

Theme setup (rule 24) on 2.1.284 numbers the choices like any option (`  N. label`,
the selected one `❯ N. label`) and draws the check after the current label; 2.1.286
draws them unnumbered with `✔ ` before it. Everything around them is the same:
`Let's get started.`, title and subtitle at col 1, the choices at col 1, a blank
row, the `╌` preview box (col 1, `W − 2` wide on 2.1.284) with its diff rows at col
2, and `Syntax theme: …` at col 2 as `L′`, wrapping to col 1 at 40 columns. Below 38
columns 2.1.284 wraps labels at col 6 (a word wider than the label column is
hard-broken), so the run also takes continuation rows at col ≥ 6. Claude draws one
form per version, so a run mixing the forms is not the picker. Rule 6 reads no
option row, so numbering cannot change the theme menu.

**Not implemented** (no rule; the pane reads unknown/unknown, §5 c15):

- Auto-mode default offer: title `Make auto mode your default permission mode?`,
  unnumbered options `Yes, set auto mode as my default permission mode` /
  `No, keep <mode>` inside `paddingX 2` (S).
- Rate-limit options, consumer-terms update, trial expired (S).

**Question preview layout.** The options column is 30 cells wide and shrinks with
the pane (the preview box sat at col 34 at 100 columns, 33 at 60, 23 at 40), so the
rule reads options left of the box corner rather than at a fixed width. Rows below
the options hold only preview cells. The preview is the focused option's Markdown:
a fenced block may draw box glyphs inside the box, on any row from option 2 down; a
Markdown table renders as plain `|` text; a preview taller than the box is cut by a
`├─── ✂ ─── N lines hidden ───┤` row above the bottom border. Option descriptions
are not drawn. With focus on the chat row, the preview view keeps `❯` on the last
focused option and adds `❯ Chat about this` (two pointers); the ordinary form moves
the only `❯` to `❯ N. Chat about this`.

**Wrapping and truncation, measured:**

- Permission hint at 26: `Esc to cancel · Tab to` / `amend`.
- Question hint at 40: `… · Esc` / `to cancel`; preview hint at 60 and 40: 2 rows.
- Multi-question hint with editor at 80: `… · Esc to ` / `cancel`.
- Plan sentence at 80, 60 and 40: 2–3 rows at col 3.
- Trust at 26: an option label and the hint both wrap, with no blank row between
  options and hint.
- Bypass and MCP titles at 40: 2 rows at col 2; MCP option labels continue at col
  4.
- Workflow permission at 100/60/40: body wraps at col 2; options, hint and editor
  row unchanged.
- Settings error at 40: body wraps at col 2; options unchanged.
- Model-menu hint at 60: 2 rows. Model menu at 40x30: the hint lies below the pane
  (§1 row 60).
- Theme `Syntax theme:` at 40: continuation at col 1.
- MCP elicitation title (a 5-letter server name): classic wraps it at col 2, to
  2 rows at 41 columns and narrower (`MCP server “<name>”` / `requests your input`
  at 30) and never cuts the name. Fullscreen keeps one row and cuts the name
  first (`“zzm…”` at 43, `“…”` at 40), then the ending (`requests your inp…` at 39
  down to `requests…` at 30). A blank row separates the title from the message in
  both. A task's elicitation (`_meta` naming a related task) appends
  ` (task <id8>)`: classic always, wrapping it at 56 columns and narrower;
  fullscreen only while it fits uncut beside a name with room (live: at 70 and 80
  columns, dropped at 60 and narrower, then cut as above).

### 2.7 Precedence, reasons, diagnostics

- **Activity and interaction** are independent (2.3). A request or menu without a
  composer gives activity unknown, because dialogs replace the panel and pill.
  Nothing is carried forward.
- **Reason:**
  - `evidence_conflict` when `claude.activity.conflict` fired;
  - `evidence_clipped` when a read reached a clipped boundary and a dimension
    became unknown by it;
  - Otherwise `recognized`, `partial` or `layout_unknown`, by the count of known
    dimensions.
- **Diagnostics** (in this order, ≤ 5 entries): the region rule
  `claude.region.clipped` when a dimension became unknown by the cut; the composer
  surface's layout rule `claude.layout.composer`; the family, menu or limit rule;
  the activity rule; the composer rule (`claude.composer.empty|draft|blocked|band`).
  A 2.6 family has no layout rule; a void surface emits only its layout rule. 43
  rule IDs, each a literal of `[a-z0-9_.-]`.

### 2.8 Notices

The latest turn's ending is the notice. Walking up from the composer's top rule
past at most six notification rows and blank rows (as the slot walk does):

- The first row is `  ⎿  Interrupted…` at column 2 → notice interrupted
  (`claude.notice.interrupted`);
- The turn's last block (above a completion row `✻ <verb> for …` when one is drawn,
  read from its column-0 head past wrapped continuation rows) is a `⏺` block whose
  glyph and text share one explicit colour → notice error (`claude.notice.error`).
  Claude draws API errors, model and usage-limit failures that way (red, the
  warning colour, the inactive grey); replies, tool calls and agent notices never
  colour glyph and text alike;
- Otherwise none. The same rows above a later prompt or turn are history and set
  nothing; footer notifications are not task notices.

The notice is read on the ordinary composer surface only. The interrupted or
failed turn itself still reads idle, so it raises ready under its notice label
(spec §6).

## 3. Composer semantics

The composer comes from an accounted composer surface (2.3) or a 2.6 family,
which always reads `blocked`. First match:

| Value | Condition |
| --- | --- |
| `blocked` | Interaction is a recognized request or menu (2.6 families, the help grid, the viewed subagent): a surface owns the keystrokes |
| `unknown` | No anchor; a void surface; a `LEFT_LABEL` top rule; the usage-limit wait; the digit band (`claude.composer.band`); an anchor read cut by clipping |
| `empty` | The cells after `❯`+NBSP are one of: nothing; one `INV` space (drawn cursor); a placeholder, i.e. an optional single `INV` non-space cell (cursor on the placeholder's first letter) followed only by `DIM` cells or ordinary default-style spaces; and no continuation row has content |
| `draft` | Anything else after the prompt: any non-`DIM` non-space cell, spaces before the cursor (a whitespace draft), an `INV` non-space cell with no `DIM` text after it (cursor on a typed character), coloured slash text, paste chips, content on a continuation row. Also bash mode (`!`): a send would run a shell command |

Placeholder glyphs require `DIM`; ordinary default-style spaces between them are
invisible and need no dim attribute. Other non-dim cells remain draft evidence.
Placeholder words never supply authority. At colour level 0
the placeholder loses SGR 2 and reads as draft (S); that is safe. Queued messages
leave the composer as a placeholder. Vim NORMAL mode is invisible (U). Stale
fullscreen cells after a resize can make any exact comparison fail: unknown.

Rule IDs: `claude.composer.empty`, `claude.composer.draft`,
`claude.composer.blocked`, `claude.composer.band`. Any other unknown emits no
composer rule.

## 4. Settings knobs

Display configuration is retained; skid overrides nothing.

| Knob | Effect on the grammar |
| --- | --- |
| `tui: "fullscreen" \| "default"`, `CLAUDE_CODE_NO_FLICKER`, `CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN` | Renderer `full` vs `classic`; auto-disabled under `tmux -CC` and after crashed boots. Setup dialogs before the REPL render at the top of the normal screen (alt=0). Everything is positional. 2.1.284 on Linux draws fullscreen with no `tui` setting |
| `statusLine {…}` | 0..n opaque rows between the bottom rule and `M`. Suppresses `esc to interrupt`, `? for shortcuts` and voice in the default footer (D), so working needs spinner + loading chevron and idle needs P0, P1 via another `Hs` member, or P2. The gated footer does not suppress the hint |
| Colour level 0: `NO_COLOR` (with `FORCE_COLOR` unset), `FORCE_COLOR=0`, `TERM=dumb` | No `FG`/`DIM`/`BG`; `INV` survives (S). `STYLE_LIVE` fails, so never idle; working only via hint or background; placeholder reads as draft |
| `prefersReducedMotion` | Spinner glyph `●`, frozen timer |
| `axScreenReader`, `CLAUDE_AX_SCREEN_READER`, `--ax-screen-reader`, `CLAUDE_AX_STARTUP_QUIET_MS` | Unsupported: reads unknown. The renderer draws no SGR or box glyphs, so no 2.2 surface anchors: unknown/unknown/none, composer unknown, `layout_unknown`, or `evidence_clipped` with `claude.region.clipped` when a read met the cut (2.1); no status, attention, ready, queue entry or guarded send (§5 c14). The corpus keeps its frames as negatives |
| `CLAUDE_CODE_ACCESSIBILITY=1` | Native cursor; the drawn `INV` cursor may be absent. `empty` tolerates that |
| `theme` (dark, light, daltonized, ansi, custom) | Colour values only |
| `spinnerVerbs`, `spinnerTipsEnabled`, custom tips, `spinnerTipLabel` | Spinner message; the `⎿` tip row and its label, skipped structurally |
| `showTurnDuration` | Completion and waiting rows present or absent. Off: `WAIT` never renders; Linux `●` bullets can then sit in the slot as an `SP`-shaped row with a ready chevron (conflict) |
| Keybindings (`chat:cancel`, `chat:cycleMode`, `app:toggleTodos`, `footer:*`, `scroll:*`) | Chords inside hints; matched as `<chord>`/`\S+` (one token) |
| `editorMode: vim`, `statusLine.hideVimModeIndicator` | Optional `-- MODE -- ` prefix in `M`; NORMAL invisible |
| `--name`, `/rename`, `/color <c>`, `--agent`, teammates | A `BANNER` top rule; `/color` recolours both rules and backs the label, `STYLE_LIVE` still holds. The prompt stays `❯`: the `@name` prompt glyph is reachable only in the dense prompt layout, whose gate is constant false in 2.1.286 (S) |
| Permission mode (`defaultMode`, shift+tab, `--permission-mode`, `--dangerously-skip-permissions`) | Mode item text; non-default modes draw the cycle hint (P0) when no pill exists. New configs default to auto mode with an onboarding notice (`hasSeenAutoDefaultNotice`). `--dangerously-skip-permissions` shows the bypass warning once per config |
| `prStatusFooterEnabled`, `footerLinksRegexes` | PR badge and helper tolerated; links `unlisted` |
| `CLAUDE_CODE_FOOTER_INDICATOR`, settings `footer_indicator`, remote per-model indicator | `indicator` class |
| `subagentStatusLine` | Replaces panel row bodies: such rows show no readable status and make no work claim |
| `feedbackDrafts`, `feedbackSurveyRate`, `CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY` | The digit band (composer unknown) |
| Plugin render hooks (`AbovePrompt`, `PromptHint`) | Unsupported: an undetectable digit band / an `unlisted` footer segment (§5 c11) |
| `tengu_copper_thistle`, `tengu_coordinator_panel` (server-gated) | Unified footer (§1 row 88); the panel instead of agent pills (§1 row 13) |
| `TERM_PROGRAM` (set by tmux) | Enables OSC 8 links |
| `TMUX` plus tmux on `PATH` | Enables the tmux notices |
| `CLAUDE_CODE_TMPDIR` | Where background-task output lands |
| `disableDeepLinkRegistration` | `"disable"` stops Claude registering its URL-handler bundle with LaunchServices on every start |
| `preferredNotifChannel` | Bell only; excluded |

## 5. Accepted costs and residual ambiguity

Accepted costs ([spec §8](terminal-observation.md#8-accepted-costs)),
each unknown or none, never a false claim unless the item says otherwise:

- **c1** Activity under every request/menu pane is unknown: the dialog hides panel,
  pill and spinner. For Claude, `working + permission|question` is
  unrepresentable.
- **c2** Notices (2.8) come from the turn's last row or block, so a reply that ends with
  a quoted `⎿  Interrupted` row reads interrupted, and any turn-ending system message
  drawn in one colour (an auto-continuing usage limit included) reads error.
  Claude cancels a turn interrupted before any output or tool call silently,
  restoring the prompt as a draft, so that turn reads idle without a notice.
- **c3** A loading chevron with no corroborator is unknown. Teammate views never
  reach idle.
- **c4** Idle is lost when:
  - The slot is unproven: rows showing no cycle hint whose `Hs` is gone and whose
    last item is neither a complete non-indicator label with 6 blanks after it
    nor the mode item with 4 to the row end (manual-mode rows and rows with a
    pill at the narrow widths 2.4 measures); in bypass, rows of 28–29 columns,
    where `on` has only 2–3 blank cells after it (the other non-default modes
    lose the same row at narrower widths);
  - The last segment is `Hs` cut to a bare `←` (`←…`, `← …`: 34–35, 48 and 54–55
    columns in bypass with an empty composer, shifted narrower in the other
    non-default modes): the slot is proven, but `←` prefixes both the agents
    labels and `<chord>`-led members, so the segment is `unlisted`;
  - The pane is taller than 256 rows and the slot walk reaches rows `192..h−65` (a
    fullscreen pane that tall with a short transcript), or a byte cap cut the walk;
  - A footer link or a render hook is configured;
  - The footer is non-ordinary or void (bash, history recall, help, autocomplete,
    an unrecognized col-2 notification under the mode row, a mode row that does
    not parse: bypass at 27 columns and narrower, where `on` wraps away);
  - A non-work panel row is present: idle teammates, `waiting` or
    `awaiting approval` agents, the idle summary, a paused workflow, a
    `subagentStatusLine` body;
  - Usage-limit copy appears below the bottom rule, including a statusline or panel
    row that prints it;
  - Colour level 0.
- **c5** Working is lost when:
  - An unknown col-2 notification sits in the composer margin while a statusline
    suppresses the hint: the notification becomes `S`;
  - The mode row does not parse (bypass at 27 columns and narrower, `on` wrapped
    away): the footer is void and the pane reads unknown/unknown,
    `layout_unknown`, never a false claim.
- **c6** Vim NORMAL mode is indistinguishable from an ordinary empty composer (U).
  It reads `empty`, so guarded send is admitted and relies on the existing paste
  staging.
- **c7** Process pills do not count as work: a long background shell allows idle,
  and an auto-resumed turn flips back to working.
- **c8** A finished agent or workflow row (until evicted, ≤ ~30 s) and a `WAIT` row
  whose awaited work was stopped without a notification count as working. Idle is
  delayed, never false.
- **c9** A pane taller than 256 rows whose lowest drawn row lies above its bottom
  region (a classic or pre-REPL surface near the top) reads unknown/unknown,
  `evidence_clipped`.
- **c10** A gated brief-mode spinner (S) may not match `SP`; the chevron still dims,
  so unknown, never idle.
- **c11** The digit band makes the composer unknown while visible; a col-2
  transcript row reading like a legend directly above the margin does the same.
  Plugin render hooks are unsupported display configuration: an `AbovePrompt` band
  with digit seats is undetectable, so a single-character guarded send can reach
  it, and a `PromptHint` hook makes the footer `unlisted`.
- **c12** Stale fullscreen cells after a resize and a dead process's frame during a
  relaunched Claude's first second give unknown until repaint.
- **c13** The no-response retry variant draws a second row under `RT`; without the
  hint it reads unknown.
- **c14** Screen-reader mode is unsupported (§4): its screens read
  unknown/unknown/none, composer unknown. Such a session gets no status,
  attention, ready or needs-input entry; guarded send refuses, `start --stdin`
  ends in `readiness_timeout` and `wait` never matches. Stop and close are
  unaffected.
- **c15** Human-request dialogs without a rule (auto-mode offer, rate-limit options,
  consumer terms, trial expired) read unknown/unknown. Live qualification of each
  is an [open blocker](issues/claude-unruled-request-dialogs.md).
- **c16** `← N agent(s)` reports other sessions' pending input; this terminal keeps
  interaction `none`, and `ready` can fire beside it.
- **c17** Remote usage-limit copy: only the default copies are anchored. A remotely
  configured wrap-up lead and waiting copy without any anchor make an armed wait
  indistinguishable from idle (a false idle). The stricter rule, any
  unrecognized notification blocks idle, is rejected: it kills idle while any
  other notification is visible and in every brief-mode session.
- **c18** 2.1.284's theme picker below 40 columns reads layout_unknown: accepted.
  In the measured cases it reads unknown/unknown/none, composer unknown, never a
  false claim.
- **c19** A menu taller than the pane draws its hint below it and reads
  unknown/unknown (§1 row 60). Fullscreen `/model` at 30 rows reads `menu` from 58
  columns and unknown at 56 and narrower; at 40 columns it reads from 50 rows
  (unknown at 45), so a phone-sized pane of about 40×29 never shows it as a menu.

## 6. Requalification

Run this after a Claude upgrade, before claiming the new version, with the
current-turn approval [spec §7](terminal-observation.md#7-requalification)
requires for runs on real provider accounts. The authored frames are retained as
fixtures ([testing](rules.md#testing)): `internal/agentcontrol/testdata/frames/`
holds `claude.txt`, its painter and `capture.py`, and `go test ./internal/agentcontrol`
replays their captures; add a frame for every new or changed layout and rerun
`capture.py`. Other probes are temporary and deleted before commit, and
every record is content-free.

1. Diff the new bundle's strings and components against the anchors of 2.3–2.6:
   the footer vocabulary, the dialog copy, the panel status shapes, the pinned
   column and the digit-band call sites. Mark each §1 row whose rendering or
   trigger changed. A numbered/unnumbered change like rule 24's is the expected
   shape of a version drift.
2. Drive the installed TUI against a local SSE mock of the messages API: a dummy
   key approved by its last 20 characters in `customApiKeyResponses`,
   `ANTHROPIC_BASE_URL` on 127.0.0.1, `--permission-mode default` plus
   `hasSeenAutoDefaultNotice`, and the nonessential-traffic, telemetry,
   error-reporting and autoupdater switches off. The mock steps per main-loop
   request, because Claude merges assistant turns, and matches subagent and
   workflow requests by content. MCP servers come from `--mcp-config` plus
   `permissions.allow`; an `ask` rule naming `EnterPlanMode` produces the
   enter-plan dialog; a project `.mcp.json` with two servers produces the
   multi-server approval; a malformed settings file produces the settings error.
3. Isolate every run: a temporary `HOME`, `CLAUDE_CONFIG_DIR` and
   `CLAUDE_CODE_TMPDIR` (Claude ignores `TMPDIR` for task output and creates its
   socket directory under `/tmp` regardless), settings
   `disableDeepLinkRegistration: "disable"` (otherwise each temporary home
   registers a URL handler with LaunchServices), `HISTFILE` pointed away from the
   user, and an isolated tmux server (`tmux -L <own socket> -f /dev/null`), with
   `TMUX`/`TMUX_PANE` unset in the harness's environment. Inside the pane, let
   the provider inherit the pane's own `TMUX`/`TMUX_PANE`/`TERM_PROGRAM`, the
   realistic skid condition, so tmux notices and OSC 8 links render. Pressing ←
   in the agents view moves the conversation into a detached daemon whose state
   (`/tmp/cc-daemon-<uid>/<hash>`) survives the tmux server: kill it and remove
   that directory afterwards. Remove the socket directory
   (`/tmp/cc-socks-<uid>`) only if the probe created it.
4. Qualify every renderer in use (fullscreen, classic) on each platform and
   tmux version, at the default width and at the narrow widths 2.4 (its mode-row
   table: every width from 30 to 60) and rule 24 measure. Classify
   through the real capture and `detect`; expectations come from the mock step
   and the keys sent, never from skid's output.
5. For families the mock cannot produce (the usage-limit wait, teammates, the §5
   c15 dialogs), author frames from the bundle and paint them through real tmux:
   that proves parser mechanics only, so the family stays `NOT_RUN`. Pair each new
   rule with a negative and confirm a mutation of the rule flips it.
6. Update §1's qualified column and versions; [unrun acceptance](issues/unrun-acceptance.md)
   tracks every required row that column leaves unqualified.

## 7. Displayed statusline suffix

`claudeReadStatusline` reads the managed formatter's final opaque row, the one
immediately above the ordinary composer's mode row, using only the composer's
established `bottom` and `footer.mode` anchors. Picker and other non-ordinary
branches omit. Both facts are displayed text, not account reports or execution
settings; each keeps its own admission, and neither changes status, composer,
rules or clipping.

```text
<prefix> │ <model/effort phrase> │ ctx <integer%|-->
<prefix> │ <model/effort phrase> │ ctx <integer%|--> │ <quota>
quota = 5h P% | 7d P% | 5h P% · 7d P%
```

`integer` is `0|[1-9][0-9]*`; `P` is canonical 0–100.

- **Usage** (`terminalUsage`): the whole ` │ ctx … │ <quota>` suffix ends the row's
  plain text; used percentages are `P` as written, a real `0%` included. The prefix
  bytes plus the suffix's runes must stay below the pane width; a candidate on an
  earlier opaque row refuses usage.
- **Footer phrase** (`terminalFooter`): the row's parsed cells, NBSP kept, split on
  ` │ ` into exactly three or four fields; the phrase is the second field, kept
  exactly (native aliases and effort or tier wording included, never split on its
  last word), and the text after it must be the ctx suffix with optional quota. The
  bound counts every prefix and phrase byte plus the suffix's runes, below the pane
  width. An unparsed row, or a candidate on an earlier opaque row, refuses it; so do
  missing or wrapped fields, incomplete boundaries and custom layouts. Fragments are
  never joined and nothing is inferred from an ellipsis. Quota may be absent.

The delimited clause comes from the owner's
[managed formatter](https://github.com/NielsdaWheelz/dev-server/blob/6339edc3a923e3f926e855e60a0f07c7814b2ede/assets/claude/statusline.sh),
which joins native display name and effort. A custom formatter reproducing exactly
this grammar is indistinguishable, an accepted limit. The retained frames qualify
these readers separately from status behaviour.
