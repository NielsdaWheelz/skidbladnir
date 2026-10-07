# delivery and remaining work

[architecture](architecture.md) owns the system and shared invariants;
accepted feature specifications own detailed product contracts.
[the codebase map](codebase-map.md) locates their implementation.
this index records present scope and open work, not a release diary.

## desktop profile usage

2026-10-02: [the accepted spec and plan](profile-usage.md) adds a compact main-tui
quota summary and expandable per-profile disclosure. codex uses existing account
daemon reads; claude exports structured statusline quota reports. usage remains
independent of inventory, terminal status and account admission.
[source and installed qualification](profile-usage.md#acceptance) passed on
macbook, devbox and arch with the admitted v0.13.0 generation.
[fresh inactive-claude usage](issues/claude-inactive-profile-usage.md)
is a separate follow-up, not a first-version gate.

2026-10-07: the [source revision](profile-usage.md#acceptance) centralizes desktop
quota reads on devbox and puts remaining percentages in the footer. dev-server
enables claude publication only on devbox. isolated source/installer checks pass;
[installed cutover](issues/devbox-usage-cutover.md) remains pending.

## jarvis orchestration — implemented; cutover pending

2026-10-02: [the extracted spec, plan and adversarial review](jarvis-orchestration.md)
records jarvis v2's raw launch options, optional initial prompt and observation
requirements. both providers and reusable worker sessions are in scope. the owner
keeps reuse, steering and waiting as agent decisions; no host-enforced one-job
rule is accepted. ordinary conversation interpreted by jarvis is sufficient;
there is no reply protocol or exact-submission attribution guarantee. model/effort
selection uses explicit overrides then native account defaults. the same pr also
optimizes discovery, targeting and control for agent use: short model targets,
compact default cli/tool results and full refs retained by the host for deferred
effects/waits. no alias registry is added. the owner authorized implementation and
temporary integration/live qualification. the [audit findings](jarvis-orchestration.md#adversarial-review)
are corrected: optional uncertainty evidence, staged-receipt recovery, native
consent grounding, encoded prompt admission, strict route/reason/launch contracts
and positive submillisecond waits. current-source darwin stock-provider launches,
real native gate cognition and 39 postgres/process-crash cases pass. both providers
pass linux live launch/input/read/wait/control; eleven service/cli/postgres checks
and actual cognitive wait integration pass. installed paired fleet, external
Discord delivery and production activation remain separate.
open qualification:
[partial launch evidence](issues/jarvis-launch-receipts.md),
[readiness](issues/jarvis-launch-readiness.md),
[jarvis cli compatibility](issues/jarvis-cli-contract.md) and
[option ownership/provider behavior](issues/jarvis-launch-options.md).
jarvis owns durable asynchronous observations and decides when to notify the
owner. after the original owner turn closes, worker events confer read/integrate/
notify authority only; further writes require new current owner authority.
skid gains no watcher service.

## session views and needs input

2026-10-02: source implements [session views](session-views.md) in the isolated
branch: exclusive needs-input/all/group navigation, fleet-wide phone browsing,
horizontal tabs, schema-4 restoration and one machines/pressure disclosure.
ready sorts before requests and current notices; quiet idle stays out; all/group
ordering remains stable. inventory and attention admit together, including failure,
and rejected creation cannot revive unmatched ready.
[qualification](session-views.md#qualification) passes its darwin gateway/tmux/tty
and physical-phone controller, output, navigation and layout boundaries using
authored frames and controlled tls/wss peers. release and installed-fleet
deployment are separate. final engineering checks and test retirement are recorded
with that evidence.
[agent exit attention](issues/agent-exit-attention.md) is a separate scope gap.

## searchable group entry — source implemented

2026-10-02: [directory and group entry](directory-group-entry.md) supersedes the
separate input mechanics with shared native fields, inline phone directory
search after 150 ms and retained secondary Home browsing. exact drafts and
explicit acceptance remain separate from final creation/save.
[qualification](directory-group-entry.md#qualification) records terminal/https,
real darwin zoxide/gateway/tmux and physical-phone keyboard/touch/2× boundaries.
temporary tests and probe packages are removed; full `scripts/check verify`
passes. release and installed-fleet deployment remain separate.

2026-10-02: [the accepted spec and plan](group-entry.md) covers local search/select
in desktop and phone creation/editing, plus explicit desktop create/save actions
after the group field. [qualification](group-entry.md#qualification) records
desktop input/transport checks and production phone-sheet journeys on the
connected device, including native 2× text. production deployment is outside
this change.

## non-native status and needs input — source implemented

[terminal observation](terminal-observation.md) specifies the coordinated status
cutover: bounded screen regions, provider-specific activity/request recognition
by the [codex](terminal-observation-codex.md) and [claude](terminal-observation-claude.md)
grammars, interruption and error notices, guarded composer reuse, diagnostics and
desktop/phone needs-input filtering, with manual claude executable recognition.
screen ambiguity remains unknown; titles/progress are excluded. source is
released as `v0.11.0` and applied to devbox, arch, the macbook and the phone.
[qualification](terminal-agent-control-qualification.md#terminal-observation-qualification),
within its recorded limits, passes capture, recognition, classification and cost
on darwin and linux. composition, controls (guarded send and wait), the
gateway→cli/desktop product and attention pass on darwin only; on linux
composition covers the managed create path and inspect, and the rest is
`NOT_RUN` in that qualification. the 2026-10-02
[orchestration qualification](jarvis-orchestration.md#implementation-sequence-and-acceptance)
adds current-source arch gateway/public-cli launch/input/read/wait/refusal/stop/close
for stock codex 0.160.0 and claude 2.1.288. it does not close the desktop/attention,
all-family or installed-artifact gaps. the phone and a darwin live-provider smoke
passed at cutover. open:
[remaining phone rows](issues/terminal-observation-phone-acceptance.md),
[linux live provider smoke](issues/terminal-observation-provider-smoke.md),
required families `NOT_RUN` for [codex on darwin](issues/codex-observation-darwin-coverage.md),
[claude on darwin](issues/claude-observation-darwin-coverage.md) and
[linux](issues/terminal-observation-linux-coverage.md),
[claude request dialogs without a rule](issues/claude-unruled-request-dialogs.md),
[codex server-driven families](issues/codex-server-driven-families.md) and the desktop's
[live stale and checking cells](issues/terminal-observation-stale-live.md).
[status detection](issues/terminal-status-detection.md) closes with the coverage
and smoke records, and [needs input](issues/agent-needs-input.md) with the phone
run and an attention run through cancellation. [list latency](issues/session-list-latency.md) and
[hook tmux waits](issues/agenthook-tmux-wait-delay.md) are pre-existing costs found
during qualification.

## automatic session names — source cutover

[automatic names and public handles](automatic-session-names.md) specifies one
canonical tmux name, manual takeover/reset, terminal-title reconciliation,
typed live selectors and name-independent targeting/order. source is implemented
in the isolated branch; host/desktop qualification passes on darwin/linux.
provider names are entirely provider-owned. new codex sessions remain unassociated;
manual linking is removed below. the earlier naming qualification predates that
removal: actual codex/claude launch and title emission,
exact-id native controls after association, and both approved physical-phone
journeys pass. large text exposed a collapsed rename target; separating context
and action rows repaired it; the one-row [terminal chrome](terminal-chrome.md)
later moved rename into the session sheet. temporary behavioral tests are removed; final
`scripts/check verify` passes. this is coordinated source cutover, not
deployment. the spec records contracts, costs and bounded evidence.

## terminal control and attention — source cutover

2026-10-01: source implements the revised [attention contract](reply-notifications.md).
a v2 device-local owner remembers non-idle across unknown, outage and restart;
subsequent idle raises ready. output presentation consumes ready while preserving
armed activity; departure fences late samples. creation and explicit entry
preflights use the same scoped owner. predecessor maps and closing flags are removed.

[current qualification](reply-notifications.md#qualification) passes the temporary
transition, persistence, identity, concurrency and visit probes: actual darwin
gateway/tmux/browser/tty with both stock providers using local backends; linux
owner/stream/model in a local container; the isolated physical-phone app with
scripted TLS/WSS peers, actual WebView output, fresh process restoration and
storage failures. those boundaries do not establish installed-fleet deployment,
cloud-provider behavior or unrelated naming/control acceptance. tests are removed
under [testing policy](rules/testing.md).

[terminal control](terminal-agent-control.md) replaces ordinary native-first
observation/control with exact terminal operations. explicit native targets stay
separate; stock launch, provider-owned names and existing bindings are preserved.
[terminal attention](reply-notifications.md) owns exclusive inferred ready and
presentation-owned visits; its historical results remain source-attributed.
[merged phone naming composition](issues/reply-notifications-phone-composition.md)
retains its unperformed naming rows; the new attention/visit portions passed.
[claude history completeness](issues/claude-history-completeness.md) remains an
independent native-provider issue. [terminal qualification](terminal-agent-control-qualification.md)
records the remaining detector/input/deletion and fleet boundaries.

## skid-only cutover

2026-09-29: captured-target inspection is published in v0.10.6 and installed on
macbook, devbox, arch and android. herdr/mobile servers, hooks, gates, units,
owned ingress/state and phone package are removed. jarvis's worker adapter and
archive cut are delivered in [pr 43](https://github.com/NielsdaWheelz/jarvis/pull/43);
its actual service uid reads all three production gateways. jarvis stays disabled
and paused until its separate shared-cognition repair. installed native lifecycle
and owner phone attachment pass on all three hosts. [qualification](native-agent-qualification.md)
separates those checks from remaining phone-native interaction acceptance.

## native interaction source delivery

[native conversations](native-agent-observation.md): source implemented and
isolated contract qualified. native conversation and terminal targets are separate; codex uses
ordinary upstream npm plus its owning daemon, existing recorded bindings and native
operations. the selected-view fork/build and human unread/viewer paths are retired.
ordinary status/input is terminal-based; explicit native history/control remains separate.
experimental queueing is unavailable; Claude native input remains unavailable
following [failed recipient qualification](issues/native-message-qualification.md).

manual conversation association is removed from source: no mobile/browser form,
cli track/untrack command or host mutation route remains. normal remote-new
codex startup creates no card binding; all new codex terminals remain unassociated.
existing bindings and direct native conversation commands remain available.
existing valid metadata is retained because earlier manual and creation bindings
have the same representation. this source change does not assert deployment.

v0.10.4 is published historical output under the former selected-view contract.
the coordinated stock helper/gateway/client generation was released as v0.10.5;
v0.10.6 adds captured conversation inspection.
[qualification](native-agent-qualification.md) records stock evidence;
temporary behavioral tests are deleted after verification. human native-output
viewer qualification is retired with that feature; native-provider limitations
remain with their independent issue owners.

## original-product restoration

2026-09-25: [the dev-server handoff](dev-server-handoff.md) specifies independent
coexistence with herdr-mobile. source isolates original-skid launches and marked
terminals while reusing existing provider accounts, histories and memories.
skid's unused codex hook is retired; the explicitly loaded claude plugin is
launch-scoped. skid apply owns shell setup independently of shared provider
maintenance. deployed hook interaction is qualified at the integration boundary.
source adds a read-only host-config
validator and shell templates, pins the original native helper, and guards
releases by numeric github repository
identity. the obsolete `v0.6.0` pin was removed; its artifacts belong to the other
repository. immutable `v0.9.0` is published from `580e099` and pinned upstream
and in dev-server; [the handoff](dev-server-handoff.md#release-and-namespace)
records source, signer and publication verification. the root operator completed
three-host coexistence and the recorded fleet/phone cutover checks, including
arch prior-generation rollback/restore, opposite-product repeat-apply, both apk
reinstalls and exact probe cleanup. [live qualification](dev-server-handoff.md#qualification-and-remaining-work)
states the evidence and its limits; prior-conversation resumption is not claimed.
[installed-fleet native background-job stop](issues/restoration-native-control.md) remains
`NOT_RUN`, separate from namespace separation. broader ux waivers below remain.

## implemented scope

| capability | contract owner |
| --- | --- |
| host inventory, exact session lifetimes, launch, rename, kill and direct attachment | [architecture](architecture.md), [client and attachment](agent-control-ux.md), [rename](session-renaming.md) |
| inferred terminal status, exact terminal controls and device-local attention; separately explicit native history/control | [terminal observation](terminal-observation.md), [terminal control](terminal-agent-control.md), [terminal attention](reply-notifications.md), [native interaction](native-agent-observation.md) |
| session labels, client grouping/filtering and dashboard restoration | [groups](groups.md), [dashboard continuity](dashboard-return-continuity.md) |
| standalone terminal and new terminal here | [terminal creation](shells.md) |
| persistent terminal shell, current provider/home, direct remote context, zoxide directory search, desktop `n`/`N` and mobile forge | [terminal continuity](terminal-continuity.md), [directory/group entry](directory-group-entry.md) |
| desktop browser (needs-input, all and group views) and fullscreen attachment return | [desktop browser](desktop-browser.md) |
| phone fleet connect/reconnect, encrypted pairings and quarantine | [fleet distribution](public-fleet-distribution.md), [architecture §6](architecture.md#6-android-surface) |
| phone dashboard, session card, directory chooser and machine pressure | [refresh](dashboard-pull-to-refresh.md), [session card](session-card.md), [chooser](working-directory-chooser.md), [pressure](machine-pressure-rail.md) |
| terminal chrome, sizing, keys, touch, selection and input composition | [chrome](terminal-chrome.md), [sizing](terminal-readable-sizing.md), [key deck](terminal-key-deck.md), [touch](terminal-touch-scroll.md), [selection](terminal-selection-copy.md) |
| visual language and generated assets | [design language](design-language.md) |

source implementation does not establish every runtime or human acceptance
criterion. feature specs retain their detailed acceptance requirements.

## release and operations

`release-pin.json` is the single committed owner of the published version,
source and artifact digests. it pins immutable `v0.13.0` from
`b2ea62aea57a87668835eefbcaebcffcf5761559`; it does not assert the
installed version of any host or phone.

`dev-server` owns machine-local installation, services and configuration.
`scripts/fleet` owns `verify`, direct `invite`, and `provision-clients`.
`scripts/install-android` validates and installs an apk in place; installation
alone is not pairing or behavioral acceptance. [architecture §5](architecture.md#5-host-architecture)
and [§6](architecture.md#6-android-surface) own those boundaries.

new agent launches use deployment-owned permission bypass flags under
[architecture §2](architecture.md#2-fixed-contract). deployed configuration was
verified historically; restored shared-account launches need qualification. existing
sessions retain their original launch policy.

## open work and acceptance

- [sequential cleanup](codebase-map.md): verified findings live in [issues](issues),
  one per issue. finish one reviewed pr before starting the next.
- [desktop browser](issues/desktop-browser-runtime-acceptance.md): the full
  naming/control/creation journey remains unqualified; the new darwin attention
  attachment and return boundary passed.
- [groups and shells hands-on](issues/groups-shells-hands-on.md): human workflow
  and usability acceptance remains unperformed; automated phone results do not
  supply it.
- [groups cutover runtime acceptance](issues/groups-cutover-acceptance.md):
  android restore/interaction and darwin isolated-host journeys remain `NOT_RUN`.
- [readable terminal sizing](terminal-readable-sizing.md#red--green--refactor-and-acceptance): the named
  fitting-grid/handback and human readability criteria remain unclaimed by the
  generic later release-suite result.
- [behavioral coverage](issues/test-system-reset.md): temporary change-specific
  tests are removed before commit. no retained suite protects the important
  behavior automatically. [testing policy](rules/testing.md) owns the workflow;
  `scripts/check verify` runs engineering checks and builds only.
- [terminal continuity](terminal-continuity.md): `v0.10.3` is published and
  deployed on the three hosts. isolated mac zsh/bash, ssh/mosh, and the named
  physical-phone journey passed. open qualification: [shell](issues/terminal-continuity-shell-qualification.md),
  [observation](issues/terminal-continuity-observation-qualification.md),
  [directory](issues/terminal-continuity-directory-qualification.md),
  [desktop](issues/desktop-browser-runtime-acceptance.md),
  [remote](issues/terminal-continuity-remote-qualification.md), and
  [phone controls](issues/terminal-continuity-phone-controls-qualification.md).
  [phone preservation](issues/terminal-continuity-phone-qualification.md)
  retains one unexplained missing baseline session lifetime.
- [terminal embedding](groups-and-shells.md): separate feasibility work; there
  is no accepted production embedding contract.
- [terminal chrome](terminal-chrome.md): source implemented 2026-10-02; open:
  [physical-phone acceptance](issues/terminal-chrome-hands-on.md),
  [keyboard resize churn](issues/terminal-resize-churn.md) and
  [landscape with the keyboard](issues/terminal-landscape-keyboard.md).
- [terminal observation](terminal-observation.md): the blockers listed in its
  section above remain open; the physical-phone rows wait for an approved device
  run.

other unperformed visual/device checks and explicitly waived shipment checks
remain with their feature owners. a waiver is not a pass. unavailable or
unexecuted boundaries remain `NOT_RUN` and require their applicable approval.

## historical evidence

[source-attributed release and acceptance records through this cleanup](https://github.com/NielsdaWheelz/skidbladnir/blob/5986a650d02106a2c10415a81a6ad956fe198665/docs/roadmap.md)
remain in git. they include the v0.5.0 failures, corrected v0.6.0 phone results,
and shipment waivers. removing their duplicate active-document tables neither
erases failures nor proves current acceptance. retired commands are historical
recipes, not executable gates or instructions to rebuild a harness.
