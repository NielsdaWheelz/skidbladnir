# codex status mostly unrecognized

problem: codex's terminal detector does not recognize the current codex screen.
most observed codex agents report `unknown (terminal)` with reason `unrecognized`.
[agent control](../agent-control.md) defines this as the honest result for
unfamiliar chrome, so this is a coverage gap, not a false claim.

impact: the desktop agents view ranks unknown above working, because an
unrecognized screen may be waiting on the operator. with most codex agents
unknown, it cannot separate codex agents that are waiting from those still
working. the phone shows the same undifferentiated status.

evidence: a 2026-09-27 read-only browser run against the three-host fleet, with
user approval, reported 6 of 7 codex agents as `unknown (terminal): unrecognized`
and the seventh as working. claude agents reported native states. no terminal
bytes were recorded.

resolved when: on the currently installed codex version, a live read shows codex
agents at an idle prompt as idle, and busy ones as working or blocked. explicit
dialogs must still be recognized. unfamiliar screens must remain unknown. any
chrome samples used to build the detector stay out of logs and evidence, per
AGENTS.md.
