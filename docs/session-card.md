# session card

status: current contract for the phone dashboard's session card, implemented
2026-10-02. it replaces `dashboard-card-refactor.md` (the 2026-08-27 delivery
recipe, retained in git history). hands-on device acceptance is `NOT_RUN`
([issue](issues/session-card-hands-on.md)).

[architecture §4](architecture.md#4-product-behavior) owns product behavior and
wins on conflict. [design language](design-language.md) owns visual identity;
[observation §6](terminal-observation.md#6-content-attention-and-filtering)
owns status copy and tone; [agent control](agent-control.md) owns what the
terminal lifetime controls do; [groups](groups.md) owns the group action;
[dashboard refresh boundary](dashboard-refresh-boundary.md) owns collection
spacing; [forge seal](forge-seal.md) owns the create control floating over the
grid. this document owns the card's structure, its action surface and its
spoken account.

## purpose

a card answers, at a glance and in this order: what work is this, what state
is it in, who is doing it, and where. one tap opens its terminal. every other
verb is one tap further, behind the overflow.

## invariants

1. the tmux name is the primary identity: exact, Data face, bold Bone, at most
   two lines. nothing aliases, prefixes or generates it.
2. status is literal. its label and tone come from observation §6 unchanged,
   read without colour, and own the card's status meaning in sight and speech.
   the facet beside it is decoration.
3. the dwarf display name is visible on every card, exact, in the Display face,
   quieter than the tmux name.
4. in `All`, the machine is visible; in a machine filter it is visually omitted
   because the filter names it once. speech, the overflow's name, every routed
   action and every confirmation always name it.
5. the complete cwd is spoken exactly once. the visible directory may be
   abbreviated but never rewritten.
6. tapping the body opens the terminal and does nothing else. no verb is
   reachable only by gesture.
7. targets are at least 48dp; text is at least 11sp.
8. a stale card keeps its last observed status, muted and still, its verbs
   fenced, and names the fence on its face.
9. the card synthesizes no fact. absent values are omitted, never filled.
10. at the default font scale a card's height does not change when its status
    changes, unless the status label itself needs a second line.

## structure

```text
┌──────────────────────────────────────────────────┬──────┐
│ ⬡⬡⬡⬡  skid-terminal-chrome                       │ ▪▪▪  │  what
│ ⬡48⬡  ◆ needs permission · work continues        │      │  state
│ ⬡⬡⬡⬡  STALE · actions disabled            (opt.) │      │
│       notifications unavailable           (opt.) │      │
│       objective, ≤2 lines                 (opt.) │      │
│       Thorin Oakenshield · Claude · Work         │      │  who
│       macbook · …/code/skidbladnir-terminal-chrome│      │  where
└──────────────────────────────────────────────────┴──────┘
```

- one row: the 48dp seal (top-aligned, exact
  [dwarf seal](dwarf-seals.md) rendering), a 12dp gap, then the text column.
- padding 10dp start, top and bottom. the end inset is the overflow's 48dp
  column, so text never runs beneath it. lines are 2dp apart. omitted lines
  leave no gap.
- **what**: tmux name, `titleSmall`, Data face, bold, Bone, ≤2 lines.
- **state**: the status line (below), `labelMedium`, Data face.
- **availability**: the existing conditional marker
  (`sessionAvailabilityContent`), its copy and tone unchanged.
- **secondary**: `notifications unavailable` when observation §6 requires it.
- **objective**: present only when set, `bodySmall`, ≤2 lines.
- **who**: the dwarf signature in the Display face at `labelMedium` size, then
  ` · {profile}` in Data `labelSmall`, Muted, one line. the profile is the
  resolved runtime profile label, `<provider> · profile unknown` for an agent
  without one, and absent for a pane without an agent: its status already
  reads `terminal`.
- **where**: Data `labelSmall`, Muted, one line. a local card in `All` shows
  `{machine} · {directory}`; in a machine filter, `{directory}`. a remote card
  shows `running on {X} · terminal on {Y} · {directory}`, with
  `directory unavailable` when the remote cwd is unknown; an unknown remote
  context shows `remote context unknown · terminal on {Y}`. the host never
  yields width to the path. the directory is `abbreviatedDirectory` (two
  trailing segments behind `…/`) and yields further from its head
  (`StartEllipsis`), so the segment that names the work stays legible.
- the surface is DeepSurface with 10dp cut corners and the 25% Gold top lip.
  the whole body presses with the angular flash.

the common card (one-line name, no objective, no notices) is 94dp at font
scale 1.0, measured 93dp in the 2026-10-02 render.

### status line

`[facet] {label}[ · {detail}]`. the facet is the line's first glyph: an inline
12dp (fixed, not font-scaled) `Chip`-cut square in the status tone, so it holds
the first line at every font scale and wraps with the words. only a fresh
working observation turns its notch (design-language §12). the label is bold in
the status tone and never yields: it wraps to a second line only when it alone
cannot fit. `detail` is Muted: `work continues` when a request is shown over
visible work, or the active command of a plain local pane. it takes the width
the label leaves and ellipsizes, so a status change cannot reflow the list;
speech carries it whole.

the inference qualifier (`inferred from terminal`) is spoken on the card and
shown on the terminal surface. it is not drawn on the card: it is identical on
every agent card, so it distinguishes nothing there.
`SessionStatusContent` carries it as `evidence`, apart from `detail`.

## grid

`GridCells.Adaptive(300dp)`: one column on a portrait phone, further columns
of the same card in wider windows. there is one card layout at every width.
spacing and the trailing clearance for the floating create seal belong to the
[refresh boundary](dashboard-refresh-boundary.md) and
[forge seal](forge-seal.md) documents. cards pass beneath the seal while
scrolling, and the clearance lets the last row's overflow scroll clear of it.

## actions

the card's verbs are one ordered list of `SessionAction`s
(`SessionActions.kt`), and both presentations read it:

| order | label | enabled when | tone | then |
| --- | --- | --- | --- | --- |
| 1 | `change group` | machine can mutate | Bone | group sheet ([groups](groups.md)) |
| 2 | `send interrupt` | can mutate and no terminal control pending | Bone | sent unconfirmed |
| 3 | `close terminal only` | same | Ember | close confirmation |
| 4 | `interrupt and close terminal` | same | Ember | close confirmation |

`terminalLifetimeActions` owns rows 2–4, their order and tones; the card and
the terminal's [session sheet](terminal-chrome.md#session-sheet) both render
them from it through `SessionActionRows`. the dashboard alone adds row 1.

- **overflow**: a 48dp target at the card's top end carrying a drawn mark of
  three 3dp studs (squares: §6 has no circles), centred on the tmux name's first
  line. Muted at rest, Gold while its menu is open, Muted at 38% when disabled.
  it is enabled exactly when some action is. it speaks
  `session actions for {tmux} on {machine}`.
- **menu**: an anchored `DropdownMenu`, Card cut, RaisedSurface, no tonal or
  shadow elevation. rows are 48dp, built with the angular flash (M3's item
  hardcodes a circular ripple, which §12 forbids), body face `labelLarge`.
  destructive rows are Ember and follow a hairline rule. disabled rows render
  at 38%.
- an open menu follows its list as the card recomposes: a fence that lands
  while it is open disables rows in place. it never retargets, and a late tap
  is re-checked by the controller.
- **destruction**: both closures end the tmux session and both confirm through
  `CloseConfirmation`, whose commit button carries the Cleft. the Cleft marks
  the commitment, not the menu row that leads to it.
- no long-press, swipe or drawer. the overflow is the one visual path, and the
  TalkBack actions below mirror it.

## speech

the card is two accessibility nodes.

1. **body**: a single node with click label `open terminal`, cleared of child
   semantics, whose description is
   `{tmux}. {status accessibility label}. {dwarf}. Machine {m}.[ Profile {p}.][
   Directory {cwd}.][ {availability}.][ Objective: {objective}]`.
   a remote card says `Running on {X}. Terminal on {Y}.` in place of
   `Machine {m}.`, and `Directory unavailable.` when the remote cwd is
   unknown; an unknown remote context says
   `Remote context unknown. Terminal on {Y}.` the status accessibility label
   already carries `work continues`, the inference qualifier and
   `notifications unavailable`. its custom actions are the enabled
   `SessionAction`s, with bare labels in list order.
2. **overflow**: role button, named as above.

## verification

behavioral tests are retired ([testing](rules/testing.md)). this change's
evidence was a temporary Robolectric render and semantics harness (deleted
before commit) run against the old and new source on 2026-10-02: a three-machine
fixture at 384×832dp, font scales 1.0, 1.3 and 2.0, landscape 832×384dp, the
list scrolled to its end beneath the seal, the open menu, and the semantics
tree. it is not device evidence.

## trade-offs

- interrupting from the dashboard takes two taps. the terminal surface is
  one tap away.
- the Cleft no longer appears on the card face. the destructive signal is the
  rule, the Ember rows and the Cleft on the confirmation.
- the inference qualifier is spoken but not drawn on the card.
- one column on a portrait phone, where there were two. at about 94dp a row,
  the list shows more sessions than the old two-column grid, with nothing
  truncated, at the cost of the tiled hall.
- a card's overflow can sit beneath the floating create seal mid-scroll.
