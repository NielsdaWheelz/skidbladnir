# delivery and remaining work

[architecture](architecture.md) owns the system and shared invariants;
accepted feature specifications own detailed product contracts.
[the codebase map](codebase-map.md) locates their implementation.
this index records present scope and open work, not a release diary.

## non-native status and needs input — accepted plan

[terminal observation](terminal-observation.md) specifies the next coordinated
status cutover: bounded screen regions, provider-specific activity/request
recognition, guarded composer reuse, diagnostics, managed codex run-state chrome
and desktop/phone needs-input filtering. manual claude executable recognition is
a prerequisite. screen ambiguity remains unknown; titles/progress are excluded.
the plan assigns exclusive writers, content designers and adversarial review,
with temporary red/green/refactor integration/live checks deleted before commit.
no implementation, provider configuration, deployment or live acceptance is
claimed. [detection](issues/terminal-status-detection.md) and
[input](issues/agent-needs-input.md) remain open until qualified.

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
and action rows repaired it. temporary behavioral tests are removed; final
`scripts/check verify` passes. this is coordinated source cutover, not
deployment. the spec records contracts, costs and bounded evidence.

## terminal control and attention — source cutover

[terminal control](terminal-agent-control.md) replaces ordinary native-first
observation/control with exact terminal operations. explicit native targets stay
separate; stock launch, provider-owned names and existing bindings are preserved.
[terminal attention](reply-notifications.md) replaces human unread/viewer paths
with exclusive inferred `ready`, working/idle colors and presentation-owned visits.
unknown/outage breaks transition continuity; first qualified post-visit observation
is quiet. revision comparisons prevent stale polls from restoring consumed state.

desktop real gateway/tmux/tty and stock Codex/Claude terminal journeys pass;
Android isolated physical-phone controller/datastore/TLS/WSS/xterm, restart,
corruption, delayed-response, accessibility/bounds and pixel checks pass. explicit
native machine reads pass gateway/helper/decoder protocol fixtures. this qualifies
source, not deployment or cloud/native completion. the additional [merged phone
composition](issues/reply-notifications-phone-composition.md) is explicitly skipped/NOT_RUN.
tests are removed by policy;
engineering verification passes. accepted inference/sampling/closing-boundary/
concurrent-client costs are explicit in the spec. [Claude history completeness](issues/claude-history-completeness.md)
remains an independent native-provider issue, no longer a notification prerequisite.
[terminal qualification](terminal-agent-control-qualification.md) records the
remaining detector/input/deletion hardening and current-main composition checks.

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
| inferred terminal status, exact terminal controls and device-local attention; separately explicit native history/control | [terminal control](terminal-agent-control.md), [terminal attention](reply-notifications.md), [native interaction](native-agent-observation.md) |
| session labels, client grouping/filtering and dashboard restoration | [groups](groups.md), [dashboard continuity](dashboard-return-continuity.md) |
| standalone terminal and new terminal here | [terminal creation](shells.md) |
| persistent terminal shell, current provider/home, direct remote context, zoxide directory search, desktop `n`/`N` and mobile forge | [terminal continuity](terminal-continuity.md) |
| desktop browser (agents and group views) and fullscreen attachment return | [desktop browser](desktop-browser.md) |
| phone fleet connect/reconnect, encrypted pairings and quarantine | [fleet distribution](public-fleet-distribution.md), [architecture §6](architecture.md#6-android-surface) |
| phone dashboard, directory chooser and machine pressure | [refresh](dashboard-pull-to-refresh.md), [chooser](working-directory-chooser.md), [pressure](machine-pressure-rail.md) |
| terminal sizing, keys, touch, selection and input composition | [sizing](terminal-readable-sizing.md), [key deck](terminal-key-deck.md), [touch](terminal-touch-scroll.md), [selection](terminal-selection-copy.md) |
| visual language and generated assets | [design language](design-language.md) |

source implementation does not establish every runtime or human acceptance
criterion. feature specs retain their detailed acceptance requirements.

## release and operations

`release-pin.json` is the single committed owner of the published version,
source and artifact digests. it pins immutable `v0.10.7` from
`240141bde7aea3416d3be5b22065da2a3f9d8dc2`; it does not assert the
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
- [darwin desktop browser](issues/desktop-browser-runtime-acceptance.md): the
  real browser/pty/gateway/isolated-tmux journey remains skipped by user direction.
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

other unperformed visual/device checks and explicitly waived shipment checks
remain with their feature owners. a waiver is not a pass. unavailable or
unexecuted boundaries remain `NOT_RUN` and require their applicable approval.

## historical evidence

[source-attributed release and acceptance records through this cleanup](https://github.com/NielsdaWheelz/skidbladnir/blob/5986a650d02106a2c10415a81a6ad956fe198665/docs/roadmap.md)
remain in git. they include the v0.5.0 failures, corrected v0.6.0 phone results,
and shipment waivers. removing their duplicate active-document tables neither
erases failures nor proves current acceptance. retired commands are historical
recipes, not executable gates or instructions to rebuild a harness.
