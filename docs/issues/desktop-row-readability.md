# desktop session records lose visual boundaries

2026-10-15: user-reported failure; source review at `aaf4691`. the replacement
[spec and plan](../desktop-session-rows.md) is implemented; source/model and
isolated darwin/linux runtime checks pass. installed usability remains pending.

problem/impact: all-view headings, names and narrow metadata share an axis;
consecutive sessions lack structural separation. needs input puts membership
before identity. sessions become difficult to distinguish when narrow or unstyled.

evidence: baseline `layout.go`, `tableLines`, follows the preceding same-axis,
uniform-two-line contract; `view.go`, `View`, strips styles under `NO_COLOR`.
this is a presentation-contract defect. the new spec owns exact replacement
geometry, costs and work boundaries, including the user-approved intact interior
group transitions. no inventory, execution or targeting change is needed.

resolved when: the spec's rendered-content, viewport/identity and isolated
runtime criteria pass, then the user confirms hierarchy and density in the
matching installed tui after separate deployment. source and installed evidence
belong in [desktop acceptance](desktop-browser-runtime-acceptance.md); installed
review is `NOT_RUN`. delete this record only when both milestones pass.
