# agent exit raises no attention

problem: attention is raised only by an observed request, notice or ready
transition. an agent that returns to its shell, crashes or is replaced leaves no
cause, so nothing reaches needs input unless a provider error or interruption
notice is still on screen.

impact: attention cannot promise to report every unexpected stop. process
disappearance alone does not tell crash, quit, normal exit and replacement apart.

evidence: the hub's `observe` (`internal/notifier/transitions.go:97`) raises
causes only while a recognized foreground agent shows terminal status; with no
agent, `fleetclient.NotificationForeground` is nil
(`internal/fleetclient/notifications.go:15`). source reading; no crash reproduced.

resolved when: the owner defines which observed exits need attention, with their
identity and acknowledgement, and an implementation is qualified for intended
exit, unexpected exit, replacement, outages and visits without claiming an
unavailable cause; or the owner accepts the limit, which then moves to the
architecture's accepted limits and this record goes. no supervisor, incident
ledger or provider-history inference is authorized.
