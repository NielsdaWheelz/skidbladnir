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
- spec §8's quoted old chrome, a negative with no capability-table row (authored
  frames only).

the families that did run on darwin ran embedded or against a temporary-home
0.159.2 daemon. only the question auto-resolve and idle ran against the installed
pairing, the 0.159.2 TUI with the installed 0.159.3 daemon, which grammar §7
step 3 makes part of qualification. a representative set, working, a permission
and a picker, is accepted as qualifying that pairing; the other families need
not rerun against it.

impact: the listed families read by source and authored frames only. a rendering
difference would misclassify one, usually as unknown; the stale-`Ready` windows
are where the grammar's conflict is unproven live. a pairing difference would
affect every family alike.

evidence: the grammar's capability table and versions, and the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification).
server-driven families are recorded [separately](codex-server-driven-families.md);
linux coverage [separately](terminal-observation-linux-coverage.md).

resolved when: each listed family runs live on darwin through the real capture and
classifier with its expected values (or a narrow width or window is shown
unreachable and the grammar records it), working, a permission and a picker also
pass against the installed pairing, and the capability table marks them (the
qualification records the quoted-chrome negative); then delete this record.
