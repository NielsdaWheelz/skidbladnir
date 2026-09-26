# delivery and remaining work

[architecture](architecture.md) owns the system and shared invariants;
accepted feature specifications own detailed product contracts.
[the codebase map](codebase-map.md) locates their implementation.
this index records present scope and open work, not a release diary.

## original-product restoration

2026-09-25: [the dev-server handoff](dev-server-handoff.md) specifies independent
coexistence with herdr-mobile. source now isolates provider homes and inherited
herdr context, adds a read-only host-config validator and shell templates, pins
the original native helper, and guards releases by numeric github repository
identity. the obsolete `v0.6.0` pin was removed; its artifacts belong to the other
repository. no new release, installation, or live fleet/phone acceptance is
claimed. github name transfer and immutable-release configuration are verified.
open boundaries: [release preparation](issues/restoration-release.md),
[runtime coexistence](issues/restoration-runtime.md), and
[native control](issues/restoration-native-control.md).

## implemented scope

| capability | contract owner |
| --- | --- |
| host inventory, exact session lifetimes, launch, rename, kill and direct attachment | [architecture](architecture.md), [client and attachment](agent-control-ux.md), [rename](session-renaming.md) |
| sampled agent status, bounded reads, terminal input, interrupt and stop | [agent control](agent-control.md) |
| session labels, client grouping/filtering and dashboard restoration | [spaces](spaces.md), [dashboard continuity](dashboard-return-continuity.md) |
| standalone terminal and new terminal here | [terminal creation](shells.md) |
| organized desktop browser and fullscreen attachment return | [desktop browser](desktop-browser.md) |
| phone fleet connect/reconnect, encrypted pairings and quarantine | [fleet distribution](public-fleet-distribution.md), [architecture §6](architecture.md#6-android-surface) |
| phone dashboard, directory chooser and machine pressure | [refresh](dashboard-pull-to-refresh.md), [chooser](working-directory-chooser.md), [pressure](machine-pressure-rail.md) |
| terminal sizing, keys, touch, selection and input composition | [sizing](terminal-readable-sizing.md), [key deck](terminal-key-deck.md), [touch](terminal-touch-scroll.md), [selection](terminal-selection-copy.md) |
| visual language and generated assets | [design language](design-language.md) |

source implementation does not establish every runtime or human acceptance
criterion. feature specs retain their detailed acceptance requirements.

## release and operations

after restoration publication, `release-pin.json` will again be the single
committed owner of the published version, source and artifact digests. it is
currently absent; no valid original-product release pin exists. that future
pin will not assert the installed version of any host or phone.

`dev-server` owns machine-local installation, services and configuration.
`scripts/fleet` owns `verify`, direct `invite`, and `provision-clients`.
`scripts/install-android` validates and installs an apk in place; installation
alone is not pairing or behavioral acceptance. [architecture §5](architecture.md#5-host-architecture)
and [§6](architecture.md#6-android-surface) own those boundaries.

new agent launches use deployment-owned permission bypass flags under
[architecture §2](architecture.md#2-fixed-contract). deployed configuration was
verified historically; restored private-home launches need qualification. existing
sessions retain their original launch policy.

## open work and acceptance

- [sequential cleanup](codebase-map.md): verified findings live in [issues](issues),
  one per issue. finish one reviewed pr before starting the next.
- [darwin desktop browser](issues/desktop-browser-runtime-acceptance.md): the
  real browser/pty/gateway/isolated-tmux journey remains skipped by user direction.
- [spaces and shells hands-on](issues/spaces-shells-hands-on.md): human workflow
  and usability acceptance remains unperformed; automated phone results do not
  supply it.
- [readable terminal sizing](terminal-readable-sizing.md#red--green--refactor-and-acceptance): the named
  fitting-grid/handback and human readability criteria remain unclaimed by the
  generic later release-suite result.
- [behavioral coverage](issues/test-system-reset.md): temporary change-specific
  tests are removed before commit. no retained suite protects the important
  behavior automatically. [testing policy](rules/testing.md) owns the workflow;
  `scripts/check verify` runs engineering checks and builds only.
- [terminal embedding](spaces-and-shells.md): separate feasibility work; there
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
