# desktop cwd truncation depends on action availability

2026-10-15: source finding at `aaf4691`. the replacement
[spec and plan](../desktop-session-rows.md#selected-directory) is accepted;
implemented; source/model and isolated darwin/linux runtime checks pass.
installed usability remains pending.

problem/impact: selected-directory presentation preserves the cwd tail only
when the new-shell action is available; checking/unavailable local and known
remote paths preserve the head, hiding their identifying final components.

evidence: baseline `view.go`, `rowDetailLines`, applies `ansi.Truncate` first
and uses `tail` only inside `row.available && row.session.Connection == nil`.
full cwd already survives in wrapped info. this establishes the source defect,
not which installed row the user saw.

the spec owns path/absence/remote separation, width budgets and the accepted
24-cell remote-label cap. no filesystem inference or stored-value change.

resolved when: the spec's cwd criteria pass for all availability/context cases,
with complete remote framing, truthful absence, Unicode cell bounds and full info;
then the user confirms long-directory readability in the matching installed tui.
record both milestones in [desktop acceptance](desktop-browser-runtime-acceptance.md).
installed review is `NOT_RUN`; delete this record only when both pass.
