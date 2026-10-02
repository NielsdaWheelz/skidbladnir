# terminal chrome

status: implemented 2026-10-02. `scripts/check verify android` is green. a
temporary debug harness rendered the real `TerminalScreen` on an api-36
emulator shaped like the s22+ (1080 × 2340 px at 450 dpi, 384 × 832 dp) and was
deleted before commit. it observed the rail and the sheet, both keyboard states,
font scale 2.0 and landscape, and the app's system bars under a light system
theme. other connection states, the physical s22+ hands-on pass, talkback and
switch access are `NOT_RUN`.

[architecture](architecture.md) owns behavior and acceptance;
[design language](design-language.md) owns visual identity. this document owns
the android terminal screen's chrome: the rail, the session sheet, and inset
ownership. it supersedes the former detach-chrome contract and the two-row
header rule.

## why

the terminal is the product; chrome is paid for in rows. on the s22+-shaped
emulator the former chrome (two header rows, one to four status lines) left the
terminal 520 dp with the keyboard hidden and 241 dp with gboard up, about 24 and
11 rows at the default 16 sp. the second row existed to stop an unbounded close
label from collapsing the name control (a6308b7, 2026-09-29); that label left
the terminal screen the next day (98fe674). the rule outlived its cause.

after this change the terminal measures 608 dp and 330 dp: about 28 and 15
rows, +88 and +89 dp. the row counts assume jetbrains mono's 1.32 em line box;
the dp values are measured, and absolute values depend on the device's bars.

## invariants

```text
status bar ┐ rail stratum   DeepSurface   Detach | name / place · presence ⋯
rail 48dp  ┘
terminal                    Ink           everything left over
deck 2 x 7 ┐ deck stratum   RaisedSurface
inset band ┘                              navigation bar or keyboard
```

1. the rail is the only chrome above the terminal. it is one row, at least
   48 dp, and grows only with font scale.
2. the rail's height never depends on connection, status, or sheet state, so
   the rail never changes the terminal's measured bounds. only the keyboard,
   rotation, window size and font scale do; text size changes only the grid.
   under tmux `window-size latest` every grid change resizes the shared window,
   so chrome must never cause one. the one indirect exception is a modal sheet
   hiding gboard (see trade-offs).
3. `TerminalScreen` is the sole inset owner. the rail pads the top safe-drawing
   inset, the screen pads the horizontal one, and a RaisedSurface band fills the
   bottom inset (navigation bar or keyboard). each stratum reaches the top or
   bottom display edge; the terminal keeps the space between. `MainActivity`
   pins dark system bar styles, so the bars draw light content whatever the
   system theme.
4. the recovery overlay covers the terminal and the deck, bounded above the
   inset band, so its actions stay reachable and `Detach` stays outside it.
5. detach and closure never share a surface. `Detach` leads the rail and
   destroys nothing; both closures are Ember rows inside the session sheet and
   commit only through the existing Cleft confirmation. placement and that
   confirmation carry architecture's visibly-different guarantee.

## rail

| element | content | interaction |
| --- | --- | --- |
| detach | `Detach`; Gold, `labelLarge`, body face | top-leading, ≥ 48 dp, `Role.Button`, angular indication; Android Back is equivalent |
| identity | line 1: tmux name, Data, semibold, Bone, one line, ellipsis. line 2: `<place> · <presence>`, Data, `labelSmall`; place Muted and truncating first, presence in its colour | the region and a trailing Gold `⋯` (body face) are one button that opens the session sheet; spoken `<name> on <place>, <presence>`, click label `open session actions` |

place names where typed input runs, host first, so truncation keeps it:
`macbook` for a local pane, `devbox via macbook` for a resolved ssh/mosh pane,
`remote via macbook` when the destination is unknown.

presence: `verifying`, `preparing`, `connecting`, `input frozen`, `1 client` or
`N clients`. only frozen input is coloured (Ember); the resting and transit
states are Muted, because transit is absence, not an armed recovery. the
terminal area's own overlays carry the full sentences.

`Detach` is the literal word, as design language §13 asks: the control names
what happens to the session, which keeps running, and voice access can say it.
the `‹` glyph saved about 26 dp that automatic names, which are sentences,
would truncate into anyway.

## session sheet

one ModalBottomSheet: DeepSurface, sheet shape, fully expanded, 20 dp text
edge. its open flag is the only state it adds, scoped to the attachment attempt.

facts, in order:

- the tmux name, as heading;
- `<place> · <presence>`, coloured as in the rail;
- the shared status projection (`label · detail` in its tone colour, with its
  accessibility label);
- Muted Data lines, each when it applies: `running on <host> · <agent> ·
  terminal on <owner>` (the agent segment only when one is recognized) or
  `remote context unknown · terminal on <owner>`; `directory <cwd>` or
  `directory unavailable` (omitted for an unknown remote);
  `recorded native conversation; may differ from terminal`;
  `notifications unavailable`.

actions are the card menu's rows (`SessionActionRows` in `SessionActions.kt`):
body-face `labelLarge`, at least 48 dp, wrapping rather than truncating; Bone,
Ember when destructive, 38% when disabled, with a hairline rule before the
destructive run. they speak their visible labels.

| row | routes to |
| --- | --- |
| `rename` | the existing rename sheet |
| `new terminal on <owner>` | create-here on a local pane; the source-scoped forge on an ssh/mosh pane |
| `text size · N` | the existing text-size sheet |
| `send interrupt` | the existing stop; unconfirmed, toast receipt |
| `close terminal only` (Ember) | the existing confirmation |
| `interrupt and close terminal` (Ember) | the existing confirmation |

each row closes the sheet, then calls the existing controller entry, whose
admission rules are unchanged; the sheet only routes. the three lifetime rows
come from `terminalLifetimeActions`, the list the card renders. enablement uses the
predicates the header used, plus one: while a rename or close is pending,
rename and text size are disabled too, since their entries would refuse. their order (interrupt, close terminal only,
interrupt and close) reorders the old dropdown.

## deleted

the second header row; the status, conversation, notification and remote lines
above the terminal; the `A` chip; the drawn new-terminal pictogram; the `⋯`
dropdown; `HeaderChip`; `TerminalRenameControl`; the theme's bar colours and
light-status-bar flag, which `enableEdgeToEdge` overrides and api 36 ignores.

## trade-offs

- rename, new terminal, text size, interrupt and close each cost one more tap.
- inferred status is not visible while attached. the terminal is the evidence
  (and talkback can read it), and cards keep the glance.
- `Detach` is about 26 dp wider than `‹`; long names ellipsize sooner.
- the rail is 48 dp, not a 24 dp information strip with actions elsewhere: two
  explicit targets at the accessibility floor cost about one row. the targets
  abut; both are harmless.
- at font scale 2.0 the rail grows and the name and host ellipsize; sheet rows
  wrap.
- facts that sat above the terminal now sit behind a modal sheet. opening it
  takes window focus, so gboard hides, armed ctrl/alt reset, and the shared
  window resizes; raising the keyboard again resizes it back. the old dropdown
  cost the same for actions, never for facts.
- in landscape the side cutout and navigation-bar columns stay Ink rather than
  carrying the strata.

## rejected

- hiding the status bar: android reserves immersive mode for media, and the
  clock and notifications matter while agents run.
- an auto-hiding or collapsing rail: every show or hide would resize the shared
  tmux window, and the identity must stay visible while typing, when input to
  the wrong host is likeliest.
- the rail at the bottom: `Detach` would sit directly over `Esc`, the
  most-pressed key.
- a menu key in the deck: eight 48 dp keys with the deck's 2 dp gaps and 4 dp
  padding need 406 dp, wider than 384 dp.
- a Cleft-shaped close row: the Cleft belongs to the confirmation's commit
  control, which the card and the sheet share.
- pinch to zoom: each step changes the column count, the expensive reflow axis,
  and competes with selection.

## acceptance

1. exactly one rail row above the terminal in every connection, viewport and
   sheet state.
2. the rail shows the tmux name, the execution host, the terminal owner when
   remote, and presence.
3. every former header action is reachable in two taps from the rail, with
   the header's enablement (plus the pending-sheet rule) and unchanged
   confirmation.
4. `Detach` and Back keep the existing phone-only detach; `Detach` stays
   reachable in the too-small state.
5. both strata reach the top and bottom display edges, with light bar content
   on any system theme.
6. on the s22+-shaped emulator the terminal gains about 88 dp in both keyboard
   states.

the emulator observed 2 and 6, 1 and 5 in the states listed under status, and
the sheet's rows; 3 and 4 follow from unchanged controller entries and layout.
the rest is `NOT_RUN`.
