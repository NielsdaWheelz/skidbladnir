# terminal status heuristics miss supported provider screens

problem: ordinary status could report unknown, unavailable or idle during work.
the former detector read an 8 KiB text tail of the visible screen, joined eight
lines, required one activity-row spelling, missed supported provider variants and
treated persistent composer chrome as idle evidence.

state: the [terminal observation](../terminal-observation.md) cutover replaces it
in source with bounded current-screen regions and the
[codex](../terminal-observation-codex.md) and [claude](../terminal-observation-claude.md)
grammars. recorded evidence
([qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification))
meets these resolution criteria: the capture observes the intended complete
regions; indistinguishable codex composer states never read idle, and
historical content cannot win, except as the false claims
[spec §9](../terminal-observation.md#9-final-state-costs-and-completion) accepts
(its false idles, and codex grammar §6 c11 and c13, where a transcript title or
a killed provider's screen wins); status recognition cannot weaken composer and
draft protection (the composer's own accepted limits are in spec §9); every
unknown or unavailable result has a content-free cause; and the cost of
conservative unknown and the unsupported idle waits are explicit (spec §9, the
grammars' cost sections).

two criteria are not met: positive live recognition of every supported family,
and qualification of the installed provider and tmux versions at the live
boundary.

impact: required families that never ran live may still misread on a real
screen, almost always as unknown; each remaining blocker names its families.

resolved when: [codex on darwin](codex-observation-darwin-coverage.md),
[claude on darwin](claude-observation-darwin-coverage.md),
[linux](terminal-observation-linux-coverage.md),
[claude dialogs without a rule](claude-unruled-request-dialogs.md),
[codex server-driven families](codex-server-driven-families.md) and the
[live provider smoke](terminal-observation-provider-smoke.md) are resolved; then
delete this record.
