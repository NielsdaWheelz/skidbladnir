# linux claude turn after guarded start

problem: on linux, public `start --stdin` into ordinary `claude-work` writes the
prompt and then sees a fresh empty composer, but working, a model response and a
ready notification never appear.

impact: linux claude's working, response and notification cells cannot be claimed.
darwin claude and codex on both hosts have working and ready proof.

evidence: 2026-10-20, claude 2.1.306 on an owned isolated linux tmux, classifier at
`096e9cc` (the dim-placeholder fix; an authored frame fails before and passes
after): the compound start wrote input and observed an empty composer; the working
wait timed out after 20 s with idle/recognized status; idle, cancellation, bounded
read, interrupt and close passed. that run's provider version and terminal
environment were not sampled. the cause is unknown; no provider or skid failure is
shown.

reproduce: authenticated public `start --stdin` -> owned isolated linux tmux ->
initialized shell -> `claude-work` in a trusted empty owned cwd, colour terminal,
`NO_COLOR` absent; request one bounded local-sleep task with a literal response and
observe the existing waits and notifier without changing cadence or treating idle
as completion. never replay an earlier target's input; an older lost send receipt
stays unknown.

resolved when: the behaviour at that boundary is explained, any demonstrated
defect is repaired at its owner, and guarded input, working, idle, response and the
hub's ready and acknowledgement are observed on a fresh target, with draft and
setup refusal and uncertain-write no-replay intact. no native-conversation
fallback, permission override or weakened admission.
