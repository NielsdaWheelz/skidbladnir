# claude 2.1.284's theme picker reads unknown at narrow widths

problem: rule 24 (`claude.setup.theme`) misses 2.1.284's fresh-config theme
picker in two measured narrow cases, which read `layout_unknown`
([claude grammar §5 r1](../terminal-observation-claude.md#5-accepted-costs-and-residual-ambiguity)).
below 40 columns claude draws its welcome art above the picker, and once the
focus leaves option 1 it leaves a dot row at col 0 directly above
`Let's get started.`; at 26 columns a `.` also lands at col 0 of the anchor's own
row. the dot row joins the anchor's block, because `blk(1)` admits col 0, so the
block's top is not at col 1. at 33 columns `Dark mode (ANSI colors only)` is one
column too wide and the next option overwrites its tail, so the label anchor is
missing.

impact: a fresh claude config's first dialog shows `status unknown` instead of
`needs setup` in a narrow pane, once per config. never a false claim. 2.1.286 is
unobserved at these widths.

evidence: live on devbox and arch with claude 2.1.284 at 26–40 columns, after the
numbered-picker fix; the fix reads setup at the other measured widths.

resolved when: the user accepts the cost (it then moves to the grammar's accepted
costs and this record is deleted), or a remedy (an art-row-tolerant
`Let's get started.` anchor, a `Dark mode (ANSI` prefix anchor) lands with a paired
negative and reads setup live at 26–40 columns.
