# session card

[compact roster](dashboard-compact-roster.md) owns the implemented card layout,
crest, wrapping, headings and spacing. this document owns the action
surface. [architecture §4](architecture.md#4-product-behavior) owns product
behavior; [observation §6](terminal-observation.md#6-content-attention-and-filtering)
owns status copy/tone; [agent control](agent-control.md) owns terminal effects;
[groups](groups.md) owns assignment; [design language](design-language.md) owns
visual values.

## invariants

- exact tmux name is primary; the complete dwarf name is a quieter signature.
- status stays literal, including stale fences. the fixed 12dp facet is decoration.
- every view shows machine context. actions and confirmations retain exact targets.
- the card abbreviates cwd; the terminal's session sheet shows it in full.
- one body tap opens its captured terminal; other verbs use the overflow.
- touch targets remain at least 48dp and meaningful text at least 11sp.
- absent facts are omitted. status, dwarf, host and warnings may grow the card.
- profile/host/path use the full-width footer; selected groups use their tab as scope.

## status line

`sessionStatusContent` owns visible status. ordinary optional detail ellipsizes
on one line; the primary label and needs-input queue reasons wrap completely.
the inference qualifier (`evidence`) is undrawn on cards; the terminal's session
sheet shows it.

## collection

one `GridCells.Adaptive(300dp)` layout serves phone and wide windows. only `all`
has full-span group headings; selected groups omit them, and needs input stays
flat with a group line per card. vertical gaps are 4dp, horizontal gaps 8dp.
[refresh boundary](dashboard-refresh-boundary.md) and [forge seal](forge-seal.md)
retain the 12dp collection insets and 84dp trailing content clearance. the final
row can scroll above the seal; intermediate rows may pass beneath it.

## actions

the card's verbs are one ordered list of `MenuAction`s
(`ActionMenu.kt`), which the overflow menu renders:

| order | label | enabled when | tone | then |
| --- | --- | --- | --- | --- |
| 1 | `change group` | machine can mutate | Bone | group sheet ([groups](groups.md)) |
| 2 | `send interrupt` | can mutate and no terminal control pending | Bone | sent unconfirmed |
| 3 | `close terminal only` | same | Ember | close confirmation |
| 4 | `interrupt and close terminal` | same | Ember | close confirmation |

`terminalLifetimeActions` owns rows 2–4, their order and tones; the card and
the terminal's [session sheet](terminal-chrome.md#session-sheet) both render
them from it through `ActionRows`. the dashboard alone adds row 1.

- **overflow**: a 48dp target at the card's top end carrying a drawn mark of
  three 3dp studs (squares: §6 has no circles), centred on the tmux name's first
  line. Muted at rest, Gold while its menu is open, Muted at 38% when disabled.
  it is enabled exactly when some action is. having no text, it keeps the
  `contentDescription` `session actions for {tmux} on {machine}`.
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
- no long-press, swipe or drawer. the overflow is the one path.

the card carries no curated narration
([design language §14](design-language.md#14-accessibility-floor)): a screen
reader gets the body's merged visible text and the overflow's description.

## verification and costs

[roster verification](dashboard-compact-roster.md#verification) records the
change-specific integration/device evidence. temporary probes are removed under
[testing policy](rules.md#testing); no behavioral regression suite remains.

interrupting takes two taps; closures retain their confirmations. inference
stays undrawn on cards, so only the session sheet names it. compact-crests/wrapping and
section-spacing costs belong to the roster specification.
