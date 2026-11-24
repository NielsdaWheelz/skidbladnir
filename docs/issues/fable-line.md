# Fable underline

Status: plan, settled with the owner on 2026-11-23 and 24. Nothing is built and no
contract has changed. Every reference is to ddc9e9f. Evidence, specimens and the renderer
are in `~/.local/state/skidbladnir-fable-line-20261123/`. `look.ansi` is the look the
owner approved in Ghostty. `plan/` holds the worked content: `f-rail.md` (every cell, the
cases, the mutants), `f-docs.md` (every sentence), `spike.md` (the harness, run on
83178fe and ddc9e9f) and `review.md`. They call the mark "the rail" and its two counts
`rail` and `solid`. Where they differ from this file, this file rules.

Problem: the usage band draws Fable on half scale, laid over the 7d gauge
(`weekGauge`, `scaled(fable, usageGauge/2)`). A full Fable allowance ends at the gauge's
midpoint while the stem marks time on the full scale, so Fable's end cannot be set
against the stem. At full scale the overlay would cover the 7d stroke wherever Fable is
the longer.

Impact: the owner cannot read Fable's pace against the week, the reading he wants most.

Evidence:

- A cell is one glyph in one ink over one ground, plus one underline with its own colour
  and style. Ghostty paints the underline beneath the glyph and under blank cells. At the
  owner's metrics (Ghostty 1.4.0, MesloLGS NF 13 pt, 2x, cell 16 x 33 px) `━` covers rows
  14-17 and a single underline rows 28-29. Overline and strikethrough take the text
  colour, so no second colour can sit above a mid-cell line.
- Bubble Tea 2.0.9 carries underline style and colour per cell, blank cells included.
- Anthropic's plan terms: Fable's allowance is 50% of the weekly limit, from the same
  pool. `min(F, 2G)` is the Fable a row can still spend.
- Eight blind readers read the underline within 1 point and all named the rows with
  unfundable Fable. None could tell Fable at 0% from no Fable.
- The owner's look: calibration line c drew a solid gold line under the 7d bar and line d
  a dotted one.
- The spike, on ddc9e9f: a terminal emulator sees gold `🬋` and no underline on gauge
  cells, and the work line as a double underline in frost. On a hacked copy it sees
  single and dotted underlines with their colour, under blanks and stems.
- ddc9e9f carries a second underline mechanism, `span.underline bool` with its own
  `frostInk`, for the status signs' work line.

## Goal

Fable is a gold underline beneath the 7d gauge: the same 24 cells, the same 0-100% scale,
the same stem. The 7d grammar does not change, and nothing hides anything. A span has
one way to carry an underline.

## Non-goals

The phone, the CLI's `fable N%`, quota collection, the collapsed rule, the 5h columns,
the grid's widths, the reveal's timing, a legend for the dotted part, a spent track for
Fable, any forecast or rate, tmux underline passthrough, and any change to what the
status signs draw.

## Decisions

The owner's:

1. An underline, not the touching pair: 7d keeps its glyphs.
2. A single underline.
3. Single to `min(F, 2G)`, dotted beyond to `F`.
4. Fable at 0% draws nothing; the Fable column says `0%`.
5. Status signs landed first. This change moves its work line onto the span fields
   below; `span.underline` and `frostInk` go. (2026-11-24, replacing "this lands first".)
6. Done is the merged pull request. The owner's look stays `NOT_RUN` in
   [unrun acceptance](unrun-acceptance.md). It merges when ready and rides the next
   release, the [status sign cutover](status-sign-cutover.md) included.
7. Temporary tests may assert underline style and colour: the style is the meaning.

The plan's, each open to his overrule:

8. Stems keep their glyphs. The underline runs beneath them, so one column marks both.
9. The underline is independent of the glyph: stem, chomper, pellet and blank cells
   carry it.
10. All-single is reserved for `F ≤ 2G`, as `scaled` reserves empty and full: when Fable
    exceeds twice the 7d amount, at least the last cell is dotted.
11. One palette for both paths: `ground.pigments` starts as the plain colours and
    `settle` replaces them with the terminal's own, so an underline's role resolves as a
    text role does. The package styles and `renderLine`'s table go; faint and reverse
    video stay as two role cases on the plain path.
12. The contract's name for the mark is "the Fable underline": "rail" already names two
    phone surfaces, and every limit it has is an underline's.

## Target behaviour

`scaled` is today's rule: `round(left × cells / 100)`, at least 1 above 0, at most
`cells − 1` below 100. `G` and `F` are percent left, `T` hours to the weekly reset;
cells are 0-23.

| Row | G | F | T | 7d cells | Fable underline | Stem |
| --- | --- | --- | --- | --- | --- | --- |
| fable-first | 62 | 31 | 80 | 15 heavy | 0-6 single | 11 |
| opus-first | 28 | 90 | 80 | 6 heavy, `╾` | 0-12 single, 13-21 dotted | 11 |
| squeeze | 12 | 70 | 26 | 3 heavy | 0-5 single, 6-16 dotted | 3 |
| week-out | 0 | 40 | 51 | none | 0-9 dotted | 7 |
| fable-out | 45 | 0 | 44 | 11 heavy | none | 6 |
| blind | unknown | 64 | 101 | blank track | 0-14 single | 14 |
| codex | 71 | none | 121 | 17 heavy | none | 17 |
| fable-lapsed | 33 | 58, reset passed | 80 | 8 heavy | 0-13 single, quiet | 11 |
| edge | 25 | 52 | 80 | 6 heavy | 0-10 single, 11 dotted | 11 |

The head's Fable sample is one blank cell carrying the gold single underline. A row's 7d
cells, stem, numbers, countdowns and reveal are what they are today. A status sign's work
line draws the cells it draws today.

## Contract

### Span underline: `internal/sessionui/view.go`, `ground.go`, `session.go`, `layout.go`, `keys.go`

```go
type span struct {
	background   color.Color
	text         string
	role         role
	under        ansi.Underline // ansi.UnderlineNone: none
	underRole    role           // the underline's colour
	bold, italic bool
}
```

- The model literal (`session.go:106`) starts `ground.pigments` as `plainPigments`, one
  package variable in `ground.go`, which owns terminal colours: muted `#AAA69D`, gold
  `ansi.BrightYellow`, ember `ansi.BrightRed`, moss `#76B082`, frost `#78A9C6`; ink,
  quiet and bronze nil. `settle` replaces it on the full path, as today.
- `renderLine` builds each span's style from `pigments[role]`; on the plain path
  (`raise == nil`) quiet adds faint and bronze adds reverse video. A span whose `under`
  is not none adds `UnderlineStyle(under)` and, when `pigments[underRole]` is not nil,
  `UnderlineColor` of it. A span without `under` adds nothing.
- The work line is `under: ansi.UnderlineDouble, underRole: frostRole`, set where
  `layout.go:257` and `keys.go:386` set `underline` today.
- Every cell of a frame without the band is what it is today, on both paths.

### Gauge: `internal/sessionui/usage.go`

In `weekGauge`, with `known` as today (a 7d window with an amount):

```
fable  = scaled(F, 24)                      when Fable has an amount, else 0
funded = min(scaled(2G, 24), fable − 1)     when known, Fable has an amount and F > 2G
       = fable                              otherwise
cell i: under = ansi.UnderlineSingle when i < funded
              = ansi.UnderlineDotted when funded ≤ i < fable
              = none otherwise
        underRole = gold, or quiet when the Fable window has faded
```

- The glyph and its role come from the 7d window, the stem and the reveal alone. The
  Fable branch of the glyph (`🬋`, `╌`) and the half scale go.
- Adjacent cells merge into one span only when role and `under` both match.
- `usageBand`'s head replaces `🬋` with
  `span{text: " ", under: ansi.UnderlineSingle, underRole: goldRole}`.

### Paths

| Path | Fable underline |
| --- | --- |
| full | gold or quiet pigment: `4` or `4:4` with `58;2;r;g;b` |
| plain | palette 11, `58;5;11`; a faded one has no colour of its own |
| `NO_COLOR` | none, in rows and head: the frame is stripped, as today |

## Rules that bind the build

- [Rules](../rules.md): hard cutover, no fallback grammar; one concern, one owner; a field
  needs a current consumer; inline what is used once.
- Build in a new worktree from `origin/main`; never edit the main checkout, which holds
  other sessions' uncommitted edits to files this change touches.
- Docs are edited in place, in this change, from `f-docs.md`. Only the root integrator
  edits `docs/architecture.md`.
- "Rebuild no harness and add no production seam." One declared exception, temporary and
  test-only: assertions on underline style and colour.
- Tests assert emulator cells, never raw bytes, and compare a colour by type and value.
  They run without `-race`: Bubble Tea's shutdown trips it in a dependency.

## Slices

Paths do not overlap. A slice acts only inside its paths.

| Slice | Paths | Delivers | Lands |
| --- | --- | --- | --- |
| P primitive | `internal/sessionui/view.go`, `ground.go`, `session.go` (the model literal), `layout.go` (the sign's underline), `keys.go` (the legend's sample) | span underline, one palette, the work line moved | first |
| G gauge | `internal/sessionui/usage.go` | the Fable underline, the head sample | after P |
| D docs | `docs/design-language.md`, `docs/issues/unrun-acceptance.md` | `f-docs.md` clauses D and U | beside G |
| R root | `docs/architecture.md`, `docs/issues/fable-line.md`; temporary: `internal/sessionui/zz_fable_test.go`, two lines of `go.mod`, `go.sum` | `f-docs.md` clause A, the tests, checks, the pull request | last |

The branch's first commit adds this file and its last removes it.

## Red, green, refactor

Per [testing](../rules.md#testing): temporary tests at the terminal boundary, run against
ddc9e9f and against the change, sensitivity reviewed, deleted before the commit. A red is
a failed assertion on a surface both versions have: the cells of a terminal screen.

- Harness, as `spike.md` built it: `sessionui.Run` on a pty, read through
  `charmbracelet/x/vt` v0.1.0, against one loopback fake serving `/v1/sessions` and
  `/v1/profile-usage`. No export, no config file, no tmux, never port 7341.
- Cases, as `f-rail.md` numbers them. 1-4, 6 and 8-12 are red on ddc9e9f, asserting
  gauge and head cells only. 5 (fable-out) and 7 (codex) guard. 10 samples the reveal
  every 20 ms under the emulator's write lock, since its clock cannot be pinned. 14, new:
  the edge row, cell 10 single, 11 dotted, 12 none.
- Case 13, the proof for P: every cell of a frame without the band, serialised with
  colours by type and value, is identical on ddc9e9f and the change. Three runs: plain
  at 256 colours; plain at truecolor with the ground probe unanswered, so pigments
  arrive exact; full dark. The fixture holds a session with the work line, and the
  frame is taken once with the keys legend shown. A fourth frame opens the band on a
  codex-only fixture, which has no Fable column and so must not change either.
- Sensitivity: each of `f-rail.md`'s mutants turns a case red, with one more for
  decision 10: the reservation dropped, killed by 14.
- Refactor to the rules and to `f-rail.md`'s "good", rerun green, delete the test and
  restore the module files from the branch's base commit with `git checkout`, not
  `go mod tidy`.

## Acceptance

1. Every row of the target table draws as stated on the full path.
2. A stem cell and a chomper cell draw their own glyph and carry their column's underline.
3. A faded Fable window's underline is quiet; a row without Fable, or with Fable at 0%,
   carries none.
4. The head's sample is a blank cell with the gold single underline.
5. On the plain path the underline is palette 11 in the same styles; under `NO_COLOR` no
   cell carries a style and the Fable column still prints.
6. Case 13's four frames are cell-identical before and after, the work line among them.
7. These are empty: `git grep -n -e '🬋' -e '╌' -e 'half scale' -e 'Fable stroke' -e
   'dashed Fable' -e 'Thick Fable' -- docs internal/sessionui
   ':!docs/terminal-observation-claude.md'`, and `git grep -nE
   'frostInk|underline:|\.underline([^A-Za-z]|$)' -- internal/sessionui`. `view.go`
   declares no package style.
8. `scripts/check verify host` passes.
9. Unrun acceptance carries `f-docs.md`'s entry U1.

## Deletions

`fableCells` and its half scale; the `🬋` and `╌` glyph branch; `span.underline`,
`frostInk`, `plain`, `faint`, `wordmark`, `here`, `frost`, `moss`, `muted`, `ember` and
`renderLine`'s `styles` table and underline branch; the architecture's half-scale,
sextant and dashed-Fable sentences (`f-docs.md`).

## Rollout

Desktop only, no wire change. It merges when ready and rides the next release.

## Trade-offs taken

- The lines do not touch: 10 px of ground lie between them at the owner's font.
- Fable sits below the 7d line, against the owner's first wish.
- The underline resolves whole cells, about 4.2 points; on the full path 7d keeps half
  cells. Decision 10 can show one dotted cell for an excess of a single point.
- Fable at 0% and no Fable draw alike; the column tells them apart.
- The dotted part starts at twice the 7d amount, a point the gauge does not mark, and
  rests on the plan's 50% rule: a promotion or plan change misplaces it until the code
  changes.
- The underline is a style. Under `NO_COLOR` it is gone, with its sample, and only the
  number remains. A terminal, or a tmux between, without coloured underlines draws it in
  the text colour; one without styled underlines draws the dotted part single or not at
  all.
- A terminal that does not parse SGR 58 reads its colour parameters as other attributes:
  `58;5;11` as blink and an alternate font, `58;2;214;168;95` as faint and bright
  magenta text, the light theme's gold as a grey ground, across the whole underlined run. Not verified on any terminal. The work
  line already carries this risk in ddc9e9f; this change widens it to the band.
- On the plain path faint can dim the underline in every quiet cell, and a faded
  underline takes the text colour: it changes hue rather than fading.
- The font places the underline: another font or size moves it within the row.
- The palette change touches every span the browser draws. Case 13 is its proof; moss
  on the plain path is proved by reading alone, since a ready sign needs a hub the
  fixture lacks.
- The reveal's clock cannot be pinned, so the chomper cell is proved by sampling.
- The owner's look at the built band is not a gate.
