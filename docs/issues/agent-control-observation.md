# agent controls collect unused session presentation facts

problem: `internal/sessions/control.go` resolves an agent through
`enrichSession`, which also reads space, cwd, current command, launch profile
and objective for inventory presentation.

impact: every control revalidation issues five irrelevant tmux reads. control
consumers require the foreground agent and exact session/process lifetime.

resolved when: inventory and control share the agent observation without making
control collect presentation fields. retain the existing lifetime/pane/process
rechecks. characterize read/send/keys and stale-target rejection through a real
isolated gateway/tmux journey before changing ownership.
