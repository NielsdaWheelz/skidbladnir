# desktop cwd truncation depends on action availability

2026-10-15: source finding at `aaf4691`. the replacement
[preceding correction](../desktop-session-rows.md#selected-directory) was
source-qualified. [browser simplification](../desktop-browser-simplification.md)
now owns acceptance and moves the directory into every row. v0.17.1 is installed
and its desktop smoke passes. owner review requests one compact machine/current-path
label with full paths in info; final path-readability acceptance remains pending.
the [refinement source](../desktop-browser-simplification.md#location-refinement-qualification)
now passes its row/context matrix and isolated darwin cwd/transport journey.
matching deployment and owner review remain pending.

problem/impact: selected-directory presentation preserves the cwd tail only
when the new-shell action is available; checking/unavailable local and known
remote paths preserve the head, hiding their identifying final components.

evidence: baseline `view.go`, `rowDetailLines`, applies `ansi.Truncate` first
and uses `tail` only inside `row.available && row.session.Connection == nil`.
full cwd already survives in wrapped info. this establishes the source defect,
not which installed row the user saw.

the current spec owns path/absence/remote separation and suffix truncation within
the row's directory budget. installed owner feedback adds current machine to that
compact label; full machine/path details remain in info. cwd already comes from
tmux or resolved remote context, independently of provider status text. no
filesystem inference or stored-value change.

resolved when: the spec's cwd criteria pass for all availability/context cases,
with complete remote framing, truthful absence, Unicode cell bounds and full info;
then the user confirms long-directory readability in the matching installed tui.
record both milestones in [desktop acceptance](desktop-browser-runtime-acceptance.md).
installed review requests that refinement; delete this record only when both pass.
