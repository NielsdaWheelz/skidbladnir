# linux claude positive live qualification

problem: public linux `claude-work` start writes terminal input after the
placeholder repair, but working, a model response and ready notification remain
unproved.
full live acceptance is partial; source implementation and engineering pass.
[v0.20.0 deployment](../v0.20.0-installed-qualification.md) passes its installed
launch/continuity checks and does not close this provider-turn gap.

evidence: the original public start refuses `TerminalInputBlocked/not_sent`.
a private no-input cell probe of claude 2.1.306 finds every visible placeholder
glyph dim, with ordinary default-style internal spaces. `096e9cc` fixes that
responsible grammar; an authored dummy frame fails before and passes after,
alongside 465 other frames. no placeholder word supplies admission authority.

the post-fix public compound start writes input and observes a fresh empty
composer. working wait times out after 20 seconds with idle/recognized status;
idle, cancellation, bounded read and separate interrupt/close pass. no working/
armed observation, standalone literal response or ready/ack occurs. cause is
unknown; this does not prove a provider or skid failure. current-run provider
version/terminal environment were not sampled. classifier/gateway is `096e9cc`,
signed `2c2401e` child is launcher only; matching `096e9cc` signed archives pass
separate installer/package qualification.

impact: linux claude positive working/response/notification cells cannot be
claimed. darwin claude and both codex hosts have working/ready proof; the earlier
linux codex transient-state gap is closed. linux codex task completion is not
inferred from its terminal receipts.

reproduction boundary: authenticated public `start --stdin` → owned isolated
linux tmux → initialized shell → ordinary `claude-work`, trusted empty owned cwd,
supported colour terminal with inherited `NO_COLOR` absent. request one bounded
local-sleep task and literal response; observe existing waits/notifier without
changing cadence or treating idle as completion. previous owned targets are
closed; no replay of their input is permitted. one older lost send receipt stays
unknown/possibly used and is never reclassified by later evidence.

resolved when: establish ordinary provider/integration behavior at that boundary,
repair any demonstrated defect at its owner, then observe guarded input, working,
idle, response and actual hub ready/exact acknowledgement on a fresh target.
retain immutable references, draft/setup refusal, uncertain-write no-replay and
separate close facts. no native conversation fallback, permission override,
unbounded model campaign or weakened admission. delete disposable proof code.

[qualification](../terminal-only-control-qualification.md) records source and
limits. 13 content-free logs and cleanup receipt remain under
`/tmp/terminal-only-supported-results`; temporary probes/stages/owned sockets are
removed. no installed cutover, phone or native OS delivery is claimed. jarvis's
separate service-state access blocker remains outside this assignment.
