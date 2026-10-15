# desktop session records lose visual boundaries

2026-10-15: user-reported failure; source review at `aaf4691`. the replacement
[preceding readability plan](../desktop-session-rows.md) was source-qualified.
[browser simplification](../desktop-browser-simplification.md) now owns acceptance:
every row adds its directory and the footer loses selected facts. source evidence
is recorded there. v0.17.1 is installed and its desktop smoke passes. owner review
is broadly positive but asks for one compact machine/current-path label; final
acceptance remains open through that refinement.

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
review requests the single-location refinement. delete this record only when
both milestones pass.
