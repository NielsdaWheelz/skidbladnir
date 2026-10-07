# devbox usage reporting cutover

problem: the 2026-10-07 fixed-devbox remaining-quota footer and dev-server
publication switch are implemented in source but not installed on the fleet.

impact: installed v0.13.0 desktops retain the original used-quota strip and
filtered/default-machine source; installed statuslines retain all-host publication.
devbox needs a natural claude-work statusline callback before its report exists.

evidence: isolated source checks cover remaining/reset semantics, fixed source,
80×24 color/no-color layout and selected-row viewport. dev-server's isolated
installer/callback checks cover host selection, settings preservation and
unchanged display. no live apply, provider turn, tmux or device check was run.

resolved when: a reviewed skid release and dev-server revision are installed
through their managed paths, all desktops show devbox remaining quota in the
footer/details, workstation statuslines no longer publish reports, and an actual
devbox callback is read through its authenticated gateway with the original age
and reset semantics. verification retains content-free outcomes, not quota values.

blockers: release/managed installation and native callback acceptance are pending.
any tmux or phone work still requires explicit current-turn approval.
