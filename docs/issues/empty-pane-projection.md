# empty active pane drops session facts

problem: `inspectRequired` treats a legitimate zero pane pid like an invalid
combined anchor and returns before assigning attached-client count or pane
identity. `enrichSession` then skips the session's launch profile and objective.

impact: attached clients become a fabricated zero and valid session metadata
disappears until a process-backed pane becomes active. no authority bypass is
established.

evidence: `internal/sessions/manager.go` returns early for `panePID <= 0` before
reading independent session facts. tmux's empty-pane spawn path creates no
process; `pane_pid` is zero while `session_attached` remains independent.

reproduction, not yet executed: create a session on a unique test-owned `-L`
socket; set valid objective/profile options; attach one owned client; make
`split-window -t <session> ''` the active pane. compare real tmux facts with
actual manager/gateway inventory. objective/profile and the client count must
survive the empty pane.

resolved when: a temporary real tmux → manager → gateway proof retains exact
session-level facts for empty panes, omits unavailable process identity, and
conserves ordinary panes and vanished-session reconciliation. genuinely malformed
required fields must not become fabricated successful facts.
