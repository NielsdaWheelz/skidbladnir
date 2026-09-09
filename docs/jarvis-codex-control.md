# Jarvis: shared local Codex control

Approved target, amended 2026-09-09. Implementation is complete on the isolated
feature branches; live acceptance remains `NOT_RUN`. Repository names below
identify sibling checkouts, not new packages.

Host-deployment extension approved 2026-09-09: `dev-server/SPEC.md` now owns
shared App Server installation/start on MacBook and Arch as well as Devbox.
This supersedes the original Devbox-only installer/PATH/Forge restrictions
below, not Jarvis's devbox-local control scope. All hosts use the same exact
Codex pin and three local account services; no remote Jarvis control is added.

Human-CLI correction approved 2026-09-09: human commands are transparent account
selectors, not Jarvis launch requests. Native 0.153.4 automatically discovers
each supervised service through its default account socket path. Native
exceptions remain: incompatible startup overrides or an unavailable server can
use embedded execution; noninteractive/admin commands retain native behavior.
There is no always-shared human guarantee, argument allowlist, forced policy or
work-root restriction. Jarvis's explicit shared transport and closed terminal
launcher remain unchanged. `dev-server/SPEC.md` owns exact routing, discovery,
environment/config/resume limitations and installer proofs. Server handshakes
are not the live worker/tmux/phone acceptance defined here.

## Goal and scope

From ordinary Jarvis conversation, start and control Codex workers on the
devserver using Personal, Work, or Work2 subscriptions. Each worker started by
Jarvis opens automatically in tmux and is discoverable/attachable in unchanged
Skid.

- Exactly one supervised App Server per configured account home. Personal contains
  Jarvis cognition, other personal `llm-calling` sessions, and manual/worker
  threads that use native shared attachment. Work and Work2 contain their
  respective shared manual/worker threads. Human native exceptions above do
  not relax Jarvis's shared-only boundary.
- Jarvis's coordinator uses Personal; its host tools can target all three.
  Account selection is explicit and never falls back or changes another login.
- “Worker” means a top-level Jarvis launch action. Native Codex child threads
  and internal recaller/rememberer/gate calls do not each open a terminal.
- Codex owns native history and runtime state; tmux owns terminal existence.
  Jarvis keeps its existing messages/actions, not a worker table, copied
  transcript store, ownership map, or persisted inventory.
- No Skid source/API/Android changes. Existing Kill remains terminal-only;
  provider-aware Stop is a separate PR. Optional runtime identity may be absent;
  do not repair it with hooks or fabricated metadata.

Non-goals: other hosts, Claude integration, MCP, a new coordinator framework,
Android Jarvis UI, automatic worktrees, scheduled delegation, automatic worker
completion notifications, crash-safe relaunch, thread deletion, background-job
cleanup, guaranteed stopping of descendants, or permanent thread fencing.

## Composition and ownership

```text
Jarvis service (jarvis UID; existing kernel, policy and actions)
  ├─ llm-calling ── Unix sockets ── Personal / Work / Work2 App Servers
  └─ terminal launcher socket ── development UID ── ordinary default tmux
                                                       └─ stock remote Codex TUI
                                                                    ↑
                                                               unchanged Skid
```

1. **`dev-server` owns host deployment.** Three supervised App Servers run as
   the development user with separate existing account homes and sockets. One
   root-owned profile configuration supplies exact endpoint, account-home,
   binary/pin, empty non-secret cognition cwd and permitted work-root mappings.
   Jarvis consumes its non-secret
   client view; it does not maintain a second independently authored mapping.
   Bind locally; socket permissions grant only the intended local clients.
   Jarvis secrets/database credentials never enter server or worker environments.
2. **`llm-calling` owns the Codex protocol.** Replace private-server process
   ownership with one Unix-socket transport and native thread/turn routing.
   Reuse its JSON-RPC codec, correlation, typed sessions, policy mapping and
   error classification. Managed cognition and arbitrary-thread control share
   this transport, not duplicated clients or raw vendor calls inside Jarvis.
   Closing a handle/connection never kills the service or unrelated threads.
3. **Jarvis owns intent, authority and composition.** Add one typed Codex tool
   family through existing `llm-tools` bindings, grants, dispatch and actions.
   Its kernel remains serial; workers execute independently. Native worker
   events never enter the coordinator's structured-step decoder.
4. **One host-owned terminal launcher crosses the UID boundary.** A local
   socket-activated helper runs as the development user, authenticates kernel
   peer credentials, and accepts only the closed launch request below. No sudo
   grant, arbitrary command, prompt, environment, account home or socket supplied
   by a caller. Jarvis retains `NoNewPrivileges`, `ProtectHome` and its private
   application state. The helper uses the same default tmux server Skid observes;
   its exit/restart must not kill that server, including first launch. Use host
   `/tmp`, clear `TMUX`, `TMUX_PANE`, `TMUX_TMPDIR`, and give the per-request
   service `KillMode=process`, following the existing Skid service precedent.
5. **Human commands preserve native behavior on all three hosts.** The existing
   profile generator produces thin Bash `exec` wrappers for `codex`, `codex-work`
   and `codex-work2`. They select the declared account and forward all arguments;
   the native CLI owns transport selection. Devbox PATH and Forge select these
   commands. Jarvis never invokes them: its closed launcher still selects exact
   remote endpoint, safe policy, permitted cwd and clean environment itself.
   Claude launchers are unchanged.

Shared servers are a shared OS/environment/version/failure boundary, not
per-thread security isolation. Preserve Jarvis's per-thread read-only,
no-network, disabled-native-tool posture and strict inspected stream; replace
and requalify its old private-process containment claim. Jarvis-created workers use
explicit workspace-write policy and native human approval for escalation;
no unattended bypass. Jarvis never answers worker approvals. Its control
connection unsubscribes from each new worker before dispatch. In the intended
single-user flow the stock TUI is the only responding subscriber, but Codex
0.153.4 provides no exclusive approval-owner lease; other trusted clients must
not subscribe to a worker they do not intend to operate. Jarvis remains the
ordinary writer of its cognition threads.
Observed intervention invalidates that session before further host dispatch;
arbitrary-client fencing is not guaranteed under the shared-user trust model.

## Capability contract

Names below are the tool contract, not new HTTP routes. Define strict typed
models once in the owning layer; reject unknown fields and invalid input at
ingress. Use existing lower-layer session references internally. Native opaque
handles are full, profile-scoped values, not names, shortened aliases or authority.

```text
Profile = personal | work | work2
ThreadTarget = {profile, thread_handle}
TurnTarget = {thread: ThreadTarget, turn_handle}
Terminal = {tmux_session_id, tmux_name}  # observation, never stop authority

codex.list({profile, cursor?}) -> {threads, next_cursor?}
codex.read({thread: ThreadTarget}) -> observed status + current/latest turn
codex.start({profile, cwd, name, prompt}) -> launch outcome
codex.prompt({thread: ThreadTarget, input}) -> accepted TurnTarget
codex.interrupt({turn: TurnTarget}) -> observed turn outcome

input = Submit{text} | Steer{turn_handle, text}
ResolveCwd = {kind: ResolveCwd, profile, cwd}
LaunchTerminal = {kind: LaunchTerminal, profile, thread_handle, cwd, tmux_name}
TurnOutcome = Interrupted | Finished{native_status} | Stale | Unknown
ReadCoverage = Complete | Bounded{reason}
```

- `list/read` use supported native APIs; include interactive, App Server and
  relevant native-child sources explicitly. “All” composes three independent
  paginated calls; one failed profile never becomes an empty successful list.
  Report native `notLoaded`, `idle`, `active`/flags and `systemError` accurately.
  Idle means no current work, not “the requested task succeeded.”
- Read returns native status, optional current/latest TurnTarget/outcome, optional
  last answer and ReadCoverage; no second transcript/history subsystem.
- `start/prompt/interrupt` are Writes. Existing current-owner grounding and
  approval policy apply; worker output supplies no new authority. Reject generic
  worker controls targeting live Jarvis-internal cognition handles. The reusable
  library does not hardcode Jarvis's three-profile enumeration.
- Submit exposes Codex 0.153.4's atomic start-or-steer operation and returns the
  accepted TurnTarget; it does not claim idle-only admission. Steer retains the
  exact expected turn handle. No operation introduces an implicit host queue.
  Strict idle-only NewTurn is deferred until upstream exposes atomic admission.
- Interrupt uses the pinned native exact-turn precheck and observes the resulting
  turn outcome, not merely receipt of an RPC acknowledgement. Natural completion
  racing the interrupt is reported as Finished or Stale; the operation is never
  retried and never knowingly redirects to an observed successor.
  It does not close tmux, delete history, kill a server or promise rollback.
- Errors distinguish invalid/unauthorized input, missing target, busy/stale
  turn, profile unavailability/auth/quota, known partial launch, and unknown
  mutation outcome. A legitimate response that cannot fit the output bound is
  an explicit `output_limit`/`NotSent` error; list never drops rows behind a
  native cursor. Persistent profile unavailability is intentionally modeled;
  protocol/pin/configuration mismatch remains a defect, not a fallback result.
- Central bounds: 32 KiB prompt, 64 KiB control request/response, 50 threads per
  page, 4,096-byte cwd, existing 1–64-character tmux-name grammar. Reuse
  `llm-tools` tool deadlines/budgets; add no unbounded transcript or event queue.
  Large read output carries an explicit bounded-coverage result, never silent
  truncation. Profile sockets/paths/auth details stay out of model-facing results.

## Launch, failure and lifecycle rules

Ordered launch: validate bounded lexical profile/name/cwd → ask the host helper
to resolve existence and the permitted canonical root → create an idle native
thread at that exact path → unsubscribe the control connection from that thread → launch
its stock TUI in one new ordinary tmux session → observe that exact tmux
session once → submit the initial prompt through native start-or-steer →
return the exact thread, terminal and accepted turn.
The TUI connects to the same endpoint and thread; it never starts a second
conversation. No prompt is carried in argv, tmux options, environment or logs.
Use structured RPC input and argv-safe OS invocation, never `send-keys`.
TUI attachment is asynchronous and is not part of `Started`. Codex retains a
pending approval and replays it when the TUI later resumes the thread. This is
the accepted 80/20 boundary: launch acceptance is truthful, while exact remote
client readiness and exclusive approval ownership remain unavailable upstream.

Return a tagged launch outcome: `Started` with all three references;
`Rejected` when nothing was dispatched; `Partial` with the known created prefix
and failed stage; or `Unknown` with the uncertain stage and any known prefix.
The prefix is typed: no thread / thread only / thread+terminal / all three.
Never call a surviving idle thread or terminal a completed launch. Failure to
create and observe the exact tmux session sends no initial prompt. Do not
silently delete a created thread, close an uncertain terminal, or replay the
complete sequence. Process creation, PID, sleeps and terminal-text parsing do
not upgrade the result to TUI readiness. Do not add a proxy or custom TUI.

Reuse Jarvis's `action` record for authorization and command outcome, not worker
ownership. Codex Writes use existing `ReplayPolicy.BilledOnce`, one handler
entry with separately bounded stage I/O, and explicit outcome reconciliation;
adapt the current
`ReDispatchable`/two-attempt-only contract. A sent mutation without a conclusive
reply is never resent, including after cancellation/restart. Reconcile only
known exact native references; no title/cwd/latest-thread matching or inference
that “not found” proves an unacknowledged create did not happen. An unresolved
operation becomes the existing terminal `uncertain` action, not a retry signal.
The original owner input must not replay it under a fresh action identity.
Only `Started` settles launch success; `Rejected/Partial` settle failed with any
surviving prefix; `Unknown` or abandoned execution settles uncertain. Adapt the
recorder, uncertainty payload and restart settlement, not just the policy enum.
Codex never enters the old proved-absence/requeue or BudgetExceeded-as-failure
recovery after an ambiguous dispatch.

No automatic new worker or prompt after server loss. Reconnect re-reads native
facts; process-local client/turn state is disposable. Account failure does not
terminate other servers; Personal failure also prevents coordinator inference.
TUI loss leaves the native thread intact; an
operator can resume that exact thread through the configured profile launcher.
No permanent terminal/thread association is maintained after launch.

## Hard cut and reuse

Before production edits, root records this accepted delta in Jarvis's SPEC,
architecture/ADR and acceptance documents, the runtime contract, and host
deployment docs. Explicitly supersede Jarvis's no-delegation/private-Codex
assumptions; do not silently weaken them. Skid's opaque-terminal contract stays
binding. This document is the cross-repo plan, not an override of unrelated rules.

- Hard-cut `llm-calling` Codex to shared attachment; remove private App Server spawn,
  server-killing cleanup, client-side Codex-home enrollment and their unused
  configuration/tests. Retain primitives still used by Claude or API providers.
- Adapt existing `AgentRuntime`, session discovery, credential verification,
  JSON-RPC routing and policy translation. No duplicate worker SDK/client,
  protocol codegen, native-store parser or alternate inference path.
- Extend existing Jarvis tool composition, action settlement/recovery,
  authority descriptors and session compatibility fingerprint. No new tables,
  kernel fork, job framework or shadow command ledger.
- Reuse `dev-server`'s AI installer, profile deployment and shell machinery.
  Hard-cut that existing owner to the reviewed exact pin on all three hosts.
  Do not install latest and then overwrite it from a second installer. Generate
  each host's human wrappers from the authoritative account declaration; remove
  the restrictive `tui` path and duplicated wrapper asset. A second apply must
  be quiescent. Install the logical personal launcher before the raw binary in
  PATH. Change the Devbox Personal Forge command row; preserve both Claude rows.
- Pin one qualified server/TUI version across all three services and consumers.
  Qualification starts with locally observed CLI `0.153.4`; older `0.144.4`
  containment evidence does not qualify it. Record exact package/digests before
  implementation. No version range, compatibility reader, private fallback or
  automatic account substitution. Preserve native history and credentials;
  drain incompatible Jarvis actions and live private sessions explicitly before
  activation. Never kill existing sessions as installer cleanup.
  Defer a running backend's replacement until an explicitly authorized restart;
  ordinary apply must not interrupt every thread on that account.

## Non-overlapping work and proofs

Builders own their behavioral red and observe it before production edits;
compile/import errors are not red evidence. Green implements that behavior;
refactor consolidates the named existing owners and removes superseded paths.
At every transition, an independent reviewer tries to falsify the contract;
verifiers write neither tests nor production files.

| Owner | Exclusive implementation paths (relative to named repo) | Owning proof |
| --- | --- | --- |
| Runtime builder | `llm-calling/src/provider_runtime/agent_runtime/` excluding root-owned exports; corresponding existing `tests/test_agent_{codex_app_server,codex_sdk,runtime,sessions,auth}.py` and focused control tests | Public runtime over an external protocol-boundary fixture: routing, approval policy, conflicts, unknown outcomes, disconnect-only cleanup. Pure tables only for codecs/validation. |
| Jarvis builder | New `src/jarvis/codex_tools.py`, `codex_control.py`; existing settings/kernel/definitions, read/write composition, dispatch/policy/gate/actions; `deploy/`; corresponding tests | Real dispatch/action APIs and PostgreSQL: authority, BilledOnce settlement/restart and no duplicate effects. Launch-prefix cases use the real helper in the approved Linux tier, not a routine mock. |
| Host builder | New `dev-server/assets/codex/` except root-owned profile pin, and `ansible/roles/codex_shared/`; existing AI-tool/shell/workspace-asset roles, `lib/ai-tools.sh`, `lib/dotfiles.sh`, selected router/zshenv assets, Devbox host config; focused installer/launcher tests | Linux platform: peer UID, profiles, safe argv, isolated first tmux creation surviving helper exit, manual/Forge routing and second-apply quiescence. |
| Root integrator | Contract docs and exact reserved files below | One real-stack journey, dependency agreement and final hard-cut review. |

The helper owns two closed tagged operations: prompt-free `ResolveCwd` before
native creation and `LaunchTerminal` afterward. Launch revalidates the exact
resolved path. This preserves Jarvis `ProtectHome`; no source bind mount or
Jarvis-side traversal of development roots is introduced.

Root alone owns `dev-server/{ansible/playbooks/apply.yml,
ansible/group_vars/devbox.yml,lib/common.sh,test,assets/codex/profiles.json}`;
`llm-calling/{pyproject.toml,uv.lock,src/provider_runtime/__init__.py,
src/provider_runtime/agent_runtime/__init__.py}`;
`llm-agent-kernel/{pyproject.toml,uv.lock}` (its independent `0.144.4` SDK pin
must change; no provider fork); and
`jarvis/{pyproject.toml,uv.lock,src/jarvis/service.py,src/jarvis/cli.py,
src/jarvis/session-compatibility.json,scripts/verify,
scripts/qualify_codex_control.py}`. New profile pin and qualification script
are implementation deliverables. All documentation is root-owned.

Order: root freezes contracts and pinned-surface proof → runtime and host work
in parallel → Jarvis composition → integrated green → adversarial cleanup.
Pinned 0.153.4 source/schema inspection must prove control unsubscription,
thread-scoped approval routing, pending-request replay, native start-or-steer,
exact-steer checking and interrupt classification. The real-stack gate then
proves the accepted asynchronous stock-TUI journey. Builders must request root
ownership changes before touching another slice, including shared
exports/manifests.

## Acceptance: 80/20

Follow [Testing Standards](rules/testing.md), especially real-runtime tiers,
external-only mocking and outcome assertions, plus [architecture §9](architecture.md#9-verification).
One proof owner per boundary; do not duplicate each failure case at every tier.
An external-protocol fixture cannot qualify actual Codex/TUI concurrency.

One Linux real-stack journey, parameterized across the three profiles, must show:

Use test-owned deployment configuration: the real launcher and real gateway
target the same isolated `-L` socket through the existing real-tmux test runner,
not production default tmux. No simulated internal launcher/gateway. Phone
acceptance is separately arranged; it cannot silently retarget production
pairings or substitute for this isolated proof.

1. Jarvis resolves an existing permitted cwd before native creation, then starts
   one worker; exact native thread, initial turn and ordinary tmux
   terminal are present. Stock TUI joins that thread; unchanged Skid inventories
   it. A separately approved phone handoff proves actual visibility/attachment.
2. Manual input and Jarvis prompt/steer observe the same worker conversation.
   Jarvis does not answer worker approvals; after asynchronous attachment, a
   worker approval is answered exactly once through the TUI. A contained Jarvis turn
   coexists without accepting worker events or gaining native tools.
3. Native Submit, exact Steer, and a stale or naturally completed interrupt leave
   other workers, profiles and the shared service intact. Client/TUI/helper exit does not kill
   the worker backend. Jarvis-internal cognition remains protected from generic
   worker controls.
4. Inject failure after thread creation and loss after prompt submission:
   known partial/unknown results are honest, the first case sends no prompt,
   and restart/reconsideration of the original owner message dispatches no
   fresh replacement action. Native child/internal inference creates no
   tmux session. No inferred terminal/thread mapping appears.

Routine gates contain static checks and appropriate owner tests, not phone,
production host, tmux or subscription calls hidden in a composite command.
Real tmux requires explicit current-turn approval and isolated `-L` sockets;
only exact test-created lifetimes may be cleaned up. Provider, deployment and
device gates require their own approval/capability. Missing boundaries are
`NOT_RUN`, never green. Logs/evidence contain only bounded, content-free verdicts;
no prompts, transcripts, native account data, tokens or terminal captures.

Trade-offs accepted: shared account failure/trust/version boundaries; manual
reconciliation instead of a worker ledger/relaunch; no proactive completion
guarantee; terminal-only Skid Kill; optional Skid identity omission; and a pinned
experimental upstream dependency, not vendor-supported production infrastructure.
Human native CLI fidelity takes precedence over forcing every invocation onto
the shared server; Jarvis retains the stricter independent contract.
The real-stack journey cannot be replaced by more unit tests.

Protocol evidence: official [App Server](https://learn.chatgpt.com/docs/app-server)
documents Unix sockets, remote TUI attachment, native thread/turn operations and
experimental support status; [CLI reference](https://learn.chatgpt.com/docs/developer-commands)
documents remote resume. Pinned 0.153.4 source establishes that resume subscribes
the requesting client, approvals fan out to current thread subscribers, the
first response wins, and pending requests replay to later subscribers. It does
not expose cross-client TUI readiness or an approval-owner lease; the amended
contract does not claim either. The same source shows that `turn/start` is
start-or-steer without a reject-if-busy discriminator and that interrupt has an
exact-turn App Server precheck but no core-level successor token; the amended
contract exposes those limits rather than simulating stronger admission.
