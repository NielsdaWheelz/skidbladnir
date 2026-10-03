# partial launch evidence for jarvis

recorded 2026-10-02; high priority; [launch result](../jarvis-orchestration.md#launch-evidence).

source status: compound cli start evidence and host post-capture target retention
are implemented; android's strict creation-error decoder retains the new optional
target. fake-command probes reproduce original target loss and pass after repair;
client transport probes preserve phase facts on nonzero exit. real lost-create
and lost-input acknowledgements each dispatch once; the latter retains its exact
capture and reports unknown input. 39 real postgres/process-crash cases preserve
conclusive staged receipts, uncertain reports and original wait evidence without
worker replay. creation-only route shape and jarvis recovery/validation audit
defects are corrected. exact installed paired artifacts and production persistence
qualification remain `NOT_RUN`, keeping this deployment dependency open.

baseline problem: the v2 requirement preserves the created terminal ref after initial
input failure/uncertainty. baseline start has no compound creation/input result;
its generic failure has no terminal ref. later creation inspection errors also
discard facts before a complete creation response is returned.

impact: jarvis may retain an unknown action without the target needed to inspect
its surviving terminal. retrying start risks duplicate work.

baseline evidence at skid `79679b4`, before the local changes:
[creation](../../internal/sessions/manager.go) lines 282–329 returns an
empty observation on errors after tmux accepts dispatch, including errors after
validating server/id evidence. [gateway completion](../../internal/gateway/gateway.go)
lines 538–563 reports dispatch unknown without a target.
[client failure](../../internal/fleetclient/client.go) lines 30–34 has only code
and dispatch. none submitted an initial prompt at that baseline.

counterexample: a complete create response reaches the client; readiness or input
then fails. returning only the last error would erase a known terminal ref.
separately, lose the create response entirely: the client cannot invent that ref.

selected scope: preserve each complete exact terminal ref once existing creation
validation captures its lifetime and pane, including across later host projection
failure. do not add earlier capture machinery, invent a ref from partial identity
or promise recovery of a lost create response.

resolved when: start preserves independent creation/input facts and every already
captured exact ref, including on nonzero exit; jarvis persists them before action
settlement. capture the pane and lifetime before claiming a usable terminal ref;
implement the stated lost-response limit. no durable skid receipt store, relaunch,
mutation replay or cleanup of an unproven target. qualify each accepted boundary.
