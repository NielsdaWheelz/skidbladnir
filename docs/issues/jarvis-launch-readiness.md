# initial prompt readiness for jarvis

recorded 2026-10-02; high priority; [input sequence](../jarvis-orchestration.md#readiness-and-input).

source status: bounded client composition and fresh initial-profile/idle/composer
guard are implemented. temporary fake checks cover dialogs/drafts, profile/state
refusal, literal input, remaining-budget expiry and unknown delivery. real cold
startup and both-provider literal input pass in the isolated darwin stock fixture;
the providers return to recognized idle. retained draft and exact changed-pane
refusal pass without replay. encoded admission and closed readiness reasons are
corrected. linux providers pass literal input/read/wait/refusal/stop/close;
paired installed-fleet cold startup
and every supported startup-dialog layout remain unqualified, keeping this record
open. terminal delivery still does not prove atomic program consumption.

baseline problem: the source assigns readiness/input to skid without defining what is
ready, the submission mechanism or the overall deadline. baseline creation
promises no readiness and sends no prompt. guarded send can accept an empty
composer during working as well as idle, and confirms terminal bytes only.

impact: blind input can hit setup/draft state; readiness can exhaust the current
budget after creation; an input receipt can be mistaken for provider admission.
a terminal ref also survives provider replacement within its captured pane.

baseline evidence at skid `79679b4`, before the local changes:
[start](../architecture.md#start-the-forge),
[guarded send](../../internal/agentcontrol/terminal.go) lines 171–210,
[terminal identity and wait](../terminal-agent-control.md#4-api-and-client-commands).
skid's client cap is 15 seconds in [client.go](../../internal/fleetclient/client.go)
lines 23–27; jarvis's child/start budgets are 20 seconds in `agent_tools.py`.

counterexamples: create into a trust dialog; create with a retained composer
draft; exhaust readiness after a successful creation; replace a provider in the
same pane before input. none permits a second launch or blind submit.

selected contract: bounded guarded terminal input. fresh requested provider/profile,
idle activity, no interaction/notice and empty composer are checked at the gateway
input boundary. ordinary working-state send stays available for later steering.
retain 15 seconds total for skid start and jarvis's 20-second child cap; cold
startup can return an inspectable created terminal without sending the prompt.

resolved when: implement the selected predicate, deadline and partial result,
distinguishing terminal delivery from native admission. qualify literal multiline
input, cold startup, dialogs/drafts,
replacement and unknown delivery. observations may repeat within the declared
budget; input dispatches once. no automatic dialog answers, forced text/keys or
native-failure fallback. no claim of atomic program consumption from screen checks.
