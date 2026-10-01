# codex required screen families not run on darwin

problem: these required (RP or AC) rows of the
[codex grammar](../terminal-observation-codex.md#1-frozen-capability-table) are
`NOT_RUN` live on darwin with codex 0.159.2, although a local endpoint or
configuration can produce them:

- interactive transcript footers for selection and disclosure (they need a mouse
  drag, or an activity focus that did not engage);
- the hook row whose status message holds a cut paren group;
- `Ready` beside a hinted status row during `/compact`, review entry or the mcp
  pending turn (millisecond windows);
- an approval with the cancel key unbound (`tui.keymap.list.cancel`);
- the shortcut overlay at 60 and 40 columns, which read unproven;
- a goal pursued in plan mode, where the collaboration-mode indicator replaces
  the goal indicator.

impact: each reads by source and authored frames only. a rendering difference
would misclassify the family, usually as unknown; the stale-`Ready` windows and
the plan-mode goal are the cases where the grammar's conflict or withheld idle is
unproven live.

evidence: the grammar's capability table and the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification).
server-driven families are recorded [separately](codex-server-driven-families.md);
linux coverage [separately](terminal-observation-linux-coverage.md).

resolved when: each listed family runs live on darwin through the real capture and
classifier with its expected values (or a narrow width or window is shown
unreachable and the grammar records it), and the capability table marks it; then
delete this record.
