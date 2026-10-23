# terminal runtime ownership is undecided

problem: process awareness, terminal output access and tui reading and selection
friction motivated replacing tmux, but they have different owners. whether a
control-mode client or an owned pty runtime best meets the remaining terminal
requirements is not established.

impact: choosing from the shared symptom could create a new process-lifetime
owner without solving conversation identity or history, or keep a bridge whose
state-restoration complexity exceeds the runtime it avoids.

evidence: the [research](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/terminal-runtime-research.md)
records source, product precedents, specialist disagreement, trade-offs and
candidate experiments. ordinary attachment reads tmux's rendered output; control
mode exposes pane output; an owned pty still establishes neither provider turn
state nor complete fullscreen history. no candidate was exercised.

resolved when: the owner records the required ordinary-terminal escape hatch,
manual-session discovery, output and retention meaning, shared geometry and input
policy and process-survival guarantees, then chooses an approach from the unmet
requirements, or explicitly defers replacement. any implementation needs the
[scope](../architecture.md#1-scope) decision first. experiments need current-turn
approval and content-free evidence; this record authorizes no runtime change.
