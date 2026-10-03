# jarvis compatibility with the current skid cli

recorded 2026-10-02; high priority; repair belongs to jarvis. this local record
tracks the dependency of the [orchestration plan](../jarvis-orchestration.md),
with installed adapter cutover separate from current-source qualification.

source status: skid's compact cli, native-handle capture and compound start are
implemented. strict public cli/HTTPS and adapter checks pass. isolated darwin
stock-provider launch/input, lost acknowledgements and changed-pane control pass;
39 actual postgres/process-crash cases and eleven service/stock-cli/postgres observation checks
pass. the [audit defects](../jarvis-orchestration.md#adversarial-review) are corrected.
installed exact-artifact compatibility remains `NOT_RUN` and keeps this record
open. linux stock-provider controls, native capture and actual cognitive wait
integration pass; external delivery/production activation remain unqualified.

baseline problem: the source's obsolete read/stop flags were only part of the mismatch.
jarvis's closed session decoder lacked current skid fields and expected conversation
runtime where skid exposes identity metadata. send always supplies native-only
`--input peer`, which skid rejects on terminal refs. model tool inputs require
full opaque refs; info/start's default cli output also prints them. hiding refs
requires an actual host capture boundary, not shortening a string in the model.

impact: valid inventory/info/start responses fail validation; a successful start
can become an unknown jarvis write. ordinary new codex targets cannot use that
send command. full wire metadata burdens ordinary agent context. removing two
flags does not repair the boundary or provide the accepted short-target interface.

baseline evidence, not current source: jarvis at `cc4873f`, `src/jarvis/agent_tools.py` lines 72–73,
181–217: closed models omit `terminalStatus`, `nameMode`, `terminalHandle`,
process identity and the current conversation shape. compare
[skid session projection](../../internal/fleetclient/response.go) lines 27–66.
baseline jarvis `agent_control.py` lines 346–357 always supplies `--input peer`;
[skid validation](../../internal/fleetclient/request.go) lines 156–163 rejects
native input fields on terminal targets. jarvis lines 143–152 turns decoding
failure after a write into unknown. its normative spec also retains old shapes.

baseline jarvis `agent_tools.py` lines 76–83 requires a `ref` of up to 4096 characters;
`AgentTerminal` exposes that full ref while omitting existing short handles.
skid [handles](../../internal/fleetclient/handles.go) exclude pane id from the
terminal handle; its resolver captures the current pane per call. baseline
`inspect` accepted only `--ref`; native short-handle capture now uses the narrow
cli extension specified in [agent use and targeting](../jarvis-orchestration.md#agent-use-and-targeting).

owner scope 2026-10-02: optimize discovery and every control for ordinary agent
use in the same pr. model targets/results use existing short typed handles and
machine labels; the host retains full refs in existing action state. compact
default cli output omits opaque transport identities; full `--json` remains for
host consumers. preserve status, truncation, partial facts and uncertainty.

baseline reproduction: compare the closed models with projected session json and
terminal send argv with `Request.Valid`. current qualification uses real owned
provider terminals; no production worker or service is modified.

resolved when: jarvis audits the entire intended skid release boundary: schemas,
all command flags, requested target kinds, read/wait/close results and partial
receipts, short targeting, compact projections and target capture. qualify strict
wire decoding separately from concise model results and prove capture survives
approval delay/restart without following another pane/conversation or interrupting
a successor turn. preserve conversation-level wait. keep refs opaque and original;
revise owning contracts and qualify exact artifact compatibility.
never add a separate alias registry, consumer ref encoder, skid compatibility
aliases, silently substitute native/terminal mode or refresh mutation targets.
