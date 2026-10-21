# private-fleet cutover

status: installed activation is owner-authorized and underway. all three hosts
and the phone run immutable v0.19.1. conversion retained notification memory;
fresh phone pairing committed. v0.19.2 is building from `e13a8b9` to correct a
phone inventory-decoder crash and apple terminal's reordered-window receipt.
ordinary patch updates MUST preserve active fleet state: no restaging, converter
rerun, reset or fresh pairing. [qualification](generalization-qualification.md)
owns actual results and remaining live checks. standalone skid contains no old
configuration, credential or notification reader. new users use the
[ordinary installer](../README.md#install).

## inputs and stop conditions

- matching signed host archives and final apk from one clean source commit;
  release signatures, package identities and persistent signing certificates
  verified by `scripts/check-release`.
- temporary host stager/converter and same-id/same-signer android conversion apk
  `0.18.1/18001`; initial final apk was `0.19.0/19000`. subsequent patch
  updates use the matching published release without repeating conversion. conversion-only source and binaries
  are held privately outside the final repository/release.
- rebuild the temporary host converter against the final observer schema3 and
  `IsHub` contract. earlier schema2 converter binaries are superseded; verify
  converted output with the final decoder before admitting staged state.
- frozen dev-server checkout `aeadd57363558b245dbd7e95254819c2adc6a1e4`, its ownership
  receipts, and a private per-host quarantine inventory containing exact paths,
  hashes/symlink targets, modes and service identities. freeze dev-server applies.
- ordered public inventory: machine handle, name, tailnet origin, actual home
  and login shell. retain existing config/state paths and user selections.
- desired personal yaml from dev-server's consumer commit `a0fe999`; preserve
  profile keys/labels/order, explicit effective commands and ghostty override.

stop on unknown ownership, changed inputs, a live old observer writer, invalid
conversion, conflicting ingress, incomplete staging or source/artifact mismatch.
never reset tailscale serve, kill tmux, start a provider daemon or print private
state. a cutover pause is acceptable; two active authorities are not.

## freeze and hand back old resources

1. use the frozen consumer's `gateway_validate_namespace`,
   `gateway_validate_local_state`, `gateway_validate_installed_generations` and
   `gateway_validate_installed_unit_generations`. compare live unit/launcher
   identity with `gateway_unit_files_identity` and the saved active pair.
2. stop notification owners first: mac's exact admitted notification app/unit;
   devbox's `skid-notification-observer.service`, then `skid-ntfy.service`.
   stop each gateway through its validated service owner. verify termination;
   then export the FINAL committed observer file. earlier live copies are
   rehearsals and MUST NOT be activated.
3. privately retain config/credentials, `machine-handle`, `workspace.json`,
   `notifications-v4.json` and `notification-observer-v1.json`. preserve provider
   directories and live session identities. never call old `gateway_remove` or
   `skidbladnir_remove`: they delete required identity/runtime inputs.
4. use frozen `gateway_ingress_remove` for only the recorded handlers:
   `8443 /v1→7341`, `8443 /v1/notifications→7342`, `8444 /→2586`.
   refuse foreign/funnel handlers; preserve unrelated services. verify 8443 is
   clear before the new installer records its own route.
5. quarantine the exact old gateway/notification/ntfy service files, launcher,
   conflicting `~/.local/share/skidbladnir/current` and `~/.local/bin/skid` links,
   and proven retired helper/plugin links. retain receipts/old immutable runtime
   privately until acceptance; remove it afterward. no blind directory deletion.
6. use frozen `skidbladnir_shell_setup HOME restore-instant-prompt` to restore
   the ordinary p10k preamble, then remove only the exact old `dev-server skid
   shell init` blocks and historical zsh guard. preserve symlinks, modes and
   unrelated bytes. this old helper does not remove its source blocks itself.

mac: use bootout without persistent disable for the reused
`dev.niels.skidbladnir` label, or explicitly re-enable it before new activation.
old gateway and new host policies both preserve child processes; verify this
against the actual admitted unit before stopping it.

## construct the new fleet while offline

the temporary stager takes ordered inventory, desired yaml, the frozen observer
export and a NEW private output directory. it uses exported fleet operations:

```sh
skid-convert-observer stage /private/inventory.json /private/config.yaml \
  /private/frozen-observer.json /private/new-fleet
```

inventory is a private json array of `{machine,name,origin,home,shell}` objects.
output directories use canonical machine handles, each containing `config.yaml`,
`machine-handle` and `state/`. input files stay unchanged. a publication fsync
error leaves output explicitly uncertain: inspect it; never install a reported
failure or overwrite an existing output to retry.

1. bootstrap the first inventory machine with empty profiles. join the remaining
   machines in order, retaining handles/names and creating distinct credentials.
2. distribute the final signed membership to every staged computer; apply full
   desired yaml only now, when all referenced machines exist.
3. convert a copy of the frozen observer into the desired hub's staged state:
   the final observer schema, new fleet fingerprint, unchanged epoch/revision/complete records,
   empty receivers. retain the exact original. no observer runs during staging.
4. if necessary, use ordinary prepare/stage/fence/activate operations to move
   authority from the first machine to the desired hub. this initial offline
   construction has no running observer to transfer. empty-owned transfer is
   NEVER an alternative to draining an existing observer.
5. distribute final signed state; verify roster order, handles, hub and trust.
   recheck converted observer continuity. close every staging writer before
   publishing the private output. partial outputs cannot be installed.

local shell paths may not exist on the coordinator's os. preserve desired
settings and require successful resolution on each actual destination; offline
staging does not certify host application.

## activate and qualify

place each computer's staged identity/cache/config and preserved `machine-handle`
at its final paths BEFORE ordinary installation starts services. the hub alone
receives converted observer memory. preserve each mac device's schema4 memory.
install verified matching archives, then claim the freed tailnet routes with
`skid connect`. devbox currently has no nonroot tailscale operator; configure
its existing passwordless-sudo login user as operator before ordinary connect,
verify unrelated preferences/routes remain unchanged, and retain that setting
for normal connect/uninstall. do not disguise the dependency with a transient
privileged wrapper or run skid as root. apply personal hub yaml and verify each destination's status.
existing shells can retain already-loaded old launch functions referencing the
retired provider launcher. preserve those shells/processes; use a fresh login
shell before reusing an affected function. never inject refresh commands into
an existing session or retain the retired launcher as a compatibility path.

install/open the temporary android conversion apk in place. it disables old
owners and converts acknowledgement memory without cancelling/reposting notices.
verify completion, then install the final apk and perform one fresh connect scan
per phone. never clear app data, change package/signing identity or restore an
older notification file over newer acknowledgements.

qualify [installed parity](issues/generalization-installed-cutover.md), including exact session survival,
both mac terminal adapters and independent phone acknowledgement. keep jarvis
worker activation blocked by its [separate integration](issues/jarvis-installed-state-access.md).
update the dev-server consumer's release/bootstrap pin only after reviewed
publication. use ordinary patch updates after activation; never rerun staging
or conversion over live state.

firebase is not a core cutover prerequisite. optional android background push
uses the [user-owned setup](android-push-setup.md) and requires separate actual
delivery qualification; no public sender/domain/dns/caddy deployment occurs.
the `afa51ca` release candidate predates this direction and is ineligible for
cutover. preserve its artifact receipts as historical evidence only.

after acceptance, remove temporary conversion material and quarantined legacy
runtime. retain only required current backups privately. rollback after new
state commits requires deliberate matching-state recovery; changing a binary
symlink under incompatible data is not rollback.
