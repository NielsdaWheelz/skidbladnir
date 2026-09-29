# terminal runtime ownership remains undecided

problem: process awareness, terminal output access and tui reading/selection
friction motivated replacing tmux, but they have different responsible owners.
we have not established whether a control-mode client or an owned pty runtime
best satisfies the remaining terminal requirements.

impact: choosing from the shared symptom could create a new process lifetime
owner without solving conversation identity/history, or retain a bridge whose
state-restoration complexity exceeds the runtime it avoids.

evidence: the [2026-09-29 research](../terminal-runtime-research.md) records current
source, product precedents, specialist disagreement, explicit trade-offs and
candidate experiments. ordinary attachment currently reads tmux's rendered
output; control mode exposes pane output. an owned pty still does not establish
provider turn state or complete fullscreen history. no candidate was exercised;
live/provider/device qualification remains `NOT_RUN`.

resolved when: record the required ordinary-terminal escape hatch, manual-session
discovery, output/retention meaning, shared geometry/input policy and process
survival guarantees. choose an approach from the remaining unmet requirements
and evidence at the relevant attachment/reconnect boundary, or explicitly defer
replacement after higher-layer changes resolve the need. any implementation
requires the architecture's scope and acceptance change before shipment.

next step: settle those requirements and, if a terminal integration remains
necessary, qualify control mode against the small journey set in the research.
compare an owned runtime where a demonstrated limitation or greater conceptual
complexity justifies it. existing native-agent observation work is independent.

execution constraint: isolated tmux/live/device experiments need current-turn
approval and content-free evidence. this issue authorizes no runtime changes,
test-harness restoration, or live operations.
