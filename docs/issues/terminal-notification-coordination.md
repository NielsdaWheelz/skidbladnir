# terminal control and reply notifications need one release contract

problem: [terminal control](../terminal-agent-control.md) follows the current pane
using inferred status, while [reply notifications](../reply-notifications.md)
follows recorded native conversation/work identities and native working samples.
same-process `/resume` can change displayed work without changing that binding.

impact: a combined release could show a reply for an earlier conversation or
clear it using another conversation's inferred working state. replacing a native
status field mechanically would not preserve the notification contract.

evidence: [native qualification](../native-agent-qualification.md) records a -> b
-> a navigation with unchanged process and skid binding. the notification plan's
composition/reducer consumes `source:native,state:working` and captures that
binding for visits; the terminal plan replaces ordinary status with terminal
evidence. source review only; no new live qualification.

user decision: notification semantics remain with that pr. this plan neither
redesigns them nor treats heuristic idle as native completion evidence.

resolved when: the notification owner and root integrator freeze one compatible
contract/schema and qualify a terminal switching a -> b, exiting/restarting,
and receiving a's late result. unrelated work cannot clear or acquire another
work's notice. qualify visible and accessibility attribution and precedence with
terminal status: no notice may imply that recorded identity is the current pane's
conversation. amend the notification spec explicitly; no silent fallback or
new lifecycle machinery. blocks combined release acceptance, not independent
terminal implementation. live/device qualification remains NOT_RUN.
