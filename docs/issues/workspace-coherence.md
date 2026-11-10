# a coherent workspace across machines, providers and devices

status: research and proposals recorded at the owner's request; not implementation
approval. the [architecture](../architecture.md) remains the current contract.

problem: skid's terminal identities and controls are coherent, but repeated work
still asks the owner to reconstruct intent, location and account choices. the
t3/bb reviews identify continuity across those boundaries as the valuable outcome.
feature-count parity would import competing products' assumptions and maintenance
costs without establishing that outcome.

impact: setup, finding work, phone interruptions and starting familiar work can
consume attention that should go to decisions. durable agent collaboration is a
separate missing capability, not another presentation of terminal delivery.

## evidence and research baseline

the review used three independent agent slices: t3 product/code, product needs and
bb, and skid's interfaces. two follow-up reviews challenged the proposed ownership
and reconnect semantics. this record preserves findings and decision rationale;
future work should inspect changed owners, not repeat the entire survey.

- initial skid source: `2a265e5bdec45e79dcef8428bdca7e180be087ff`; existing
  uncommitted roadmap, terminal-ownership and related issue documents were also
  read. the ownership plan was then an approved target awaiting implementation.
  before publication, integration was rechecked against
  `bdf0e12f2090c9b7294766efb1a5233523b102d8`: independent terminal supervision is
  implemented, its enduring contract is in the
  [architecture](../architecture.md#terminal-service), and remaining native/privacy
  proof is tracked by [terminal-service qualification](terminal-service-qualification.md).
  the earlier plan is not a dependency or a request to implement ownership again.
- t3 release examined: [v0.1.8](https://github.com/pingdotgg/t3code/releases/tag/v0.1.8),
  published 2026-11-09 at 22:46:42 utc, target
  `bf469bb19669671d0df2efda74ed73867a7137c0`.
- t3 main examined: `6c9adfb2739205244b84e607736f6fd8b4611e69`, committed
  2026-11-10 at 05:07 utc, still november 9 pacific. release and main copies of
  `remote-access.md`, `thread-sidebar.md` and `providers-claude.md` matched when
  fetched. other main-source findings are identified at their pinned links.
- bb evidence is its public guides, marketplace and changelog observed during the
  november 9 review, not a pinned source checkout or exercised application.
- the supplied user reviews are subjective, undated accounts. claims of smoothness,
  speed and mobile comfort were not reproduced. skid runtime, installed fleet,
  devices, provider accounts, t3 and bb execution were all `NOT_RUN`.

the review's largest factual correction: t3 already ships linked-environment
cross-host collaboration as beta in the inspected release; it is not merely an
open pr chain. details and limitations belong to
[managed collaboration](managed-collaboration.md). bb instead centralizes threads
and history; [client scope](client-extensibility.md) records the different bargain.

## needs expressed in the supplied reviews

| review observation | underlying need and qualification |
| --- | --- |
| remote/tailscale setup and mobile felt unusually smooth | several computers should form one intelligible workspace; subjective experience, not a measured comparison |
| providers retained familiar tools and could start/message other model threads | product-aware agents need discoverable controls without an unnecessarily reduced harness; adapters still mediate native behavior |
| threads could not communicate across hosts | separately working features must compose; this limitation was already repaired as released beta in the inspected t3 version |
| polished/snappy ui but a disliked sidebar and settlement model | interaction quality and fit to the owner's workflow are distinct; organization and execution need comprehensible semantics |
| a second subscription was easy to configure/use | account choice should not require repeatedly remembering shell/account setup; convenience is not live conversation portability |
| the reviewer began building a personal frontend over t3; bb was described as moddable | preserve useful customization seams, but assess the new backend/client/extension ownership cost before following the same route |

these are the reviewer's preferences, not automatic evidence of this owner's daily
workflow. the recommendations below select coherent journeys rather than importing
every taste or feature from that account.

## design position

minimize what the human must remember while preserving what the system must
distinguish. machine identity is not an address; project identity is not a checkout;
profile is not verified account identity; session is not conversation; terminal
delivery is not provider acceptance or completed work. one owner removes tenancy,
not concurrent agents, crashes or uncertain delivery.

keep execution ownership explicit while hiding routine address selection. preserve
native provider tools and ordinary shell commands. observation is evidence, never
authorization or proof of semantic completion. normal actions should be quiet,
geometry-stable and reversible where possible; failures preserve recoverable input
and name the next useful action. local form drafts and provider-owned terminal
composer contents are different data: restoring the latter by replaying bytes is
not draft recovery.

prefer a small set of deep existing owners over another control framework. shared
contracts matter more than forcing go and kotlin to share implementation. a
single-user prototype warrants strong identities and clear failure semantics with
little infrastructure. every selected trade-off belongs in its feature contract.

## what already exists

| existing capability | source owner | remaining direction |
| --- | --- | --- |
| direct host routing, exact refs and independent dashboard publication | `internal/fleetclient`, `internal/sessionui/inventory.go`, phone `Machines.kt` | setup and continuity, not a second federation |
| cross-host start/read/send/wait/stop | `internal/agentcli/run.go`, `internal/agentcontrol`, `internal/fleetclient` | discovery; explicit new semantics for durable collaboration |
| fleet-wide groups and needs-input | `internal/fleetview`, phone `Groups.kt`, `DashboardEntryState.kt` | search, stable working set and attention dispositions |
| shared phone terminal actions | `ActionMenu.kt`, `terminalLifetimeActions` | improve the existing action contract |
| native profile launches and desktop usage | `internal/sessions/create.go`, `internal/agentruntime`, `internal/sessionui/usage.go` | setup assistance and phone visibility |
| direct card entry and context-preserving dashboard return | `SkidbladnirController.kt`, `DashboardEntryState.kt` | background return and notification entry |

phone filenames here are under `android/app/src/main/java/dev/niels/skidbladnir`.
the desktop "browser" is bubble tea's terminal browser, not a graphical web app.

## proposed order and records

this is a recommendation, not a reordered approved roadmap:

1. finish the remaining [terminal-service qualification](terminal-service-qualification.md);
   retain [energy measurement](tmux-exec-overhead.md),
   [phone terminal qualification](stock-xterm-phone-acceptance.md) and existing
   release gaps. gateway-only maintenance and an explicit runtime restart have
   different guarantees: workspace reconstruction does not preserve in-flight
   commands or unsaved process memory. use the current architecture, not the
   superseded initial plan's drain-only restriction.
2. improve [phone continuity](phone-terminal-continuity.md),
   [notification entry](phone-entry-metadata-latency.md),
   [keyboard geometry](terminal-resize-churn.md) and
   [sheets](sheet-uniformity.md), within an approved
   [interaction scope](phone-interaction-redesign.md).
3. address [setup/recovery](fleet-setup-recovery.md),
   [launch presets](project-launch-presets.md), [retrieval](fleet-search.md),
   [account usability](profile-setup-usage.md) and
   [attention dispositions](attention-dispositions.md).
4. decide whether [managed collaboration](managed-collaboration.md) is a wanted
   capability; only then select its persistent owner and recipient protocol.

supporting directions: [interaction latency](interaction-latency.md),
[agent action discovery](agent-action-discovery.md),
[visual hierarchy](visual-redesign.md), [session handoff](session-handoff.md),
[human work notes](work-context-notes.md),
[client/extensibility scope](client-extensibility.md), and the proposed
[small regression suite](test-system-reset.md).

## candidate acceptance journeys

| journey | outcome to establish |
| --- | --- |
| add a second host | recognizable identity, explained setup stage, no unrelated session effects |
| find work created elsewhere | retrieval across hosts with stable selection and honest stale state |
| launch familiar work | few remembered facts; visible machine, profile and directory |
| intervene from the phone | direct target, predictable keyboard/geometry, preserved return context |
| lose the answer to a send | uncertain outcome remains explicit; no automatic repeat |
| use a second account | attributable selection without credential copying or implied portability |
| delegate across hosts, if approved | distinguish receipt, consumption, result and uncertainty after parent/recipient restart |
| return the following day | recover the working context without reconstructing the workspace |

these are proposed acceptance cases, not passes. measure latency and interaction
cost at the affected boundary. source inspection and builds prove neither device
comfort nor live provider behavior. follow the current testing and live-state
rules until an explicit policy change is made.

resolved when: the owner disposes the proposed directions, selected capabilities
receive explicit scope/owners/acceptance and are qualified, or the remaining
directions are deliberately retired. this index may then go; enduring contracts
belong in the architecture and surviving issues keep their own evidence.
