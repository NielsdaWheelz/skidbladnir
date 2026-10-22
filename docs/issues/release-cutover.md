# release cutover

problem: the cleanup campaign changes wire and durable state without converters
or old-format readers. w2.1 (fleet-fixed-hub) replaces `identity.json`,
`host-state.json`, `resolved.json` and `authority.json` with one `fleet.json` per
host, drops signatures and the yaml `hub:` key, and gives the phone a new store.
the next release is therefore a fresh fleet, installed in lockstep across
dev-server, three hosts and the phone. this record holds every cross-repo cutover
obligation of the campaign.

impact: a skipped step leaves a host exiting at every start (yaml rejected at
bootstrap), a computer outside the fleet, or the phone unpaired. hub notification
memory and the observer epoch reset; desktop machine order becomes hub first,
then join order. until a host joins, it serves only its own terminals.

evidence:
- skid: `internal/fleet` reads only `fleet.json` and `machine-handle`; without
  `fleet.json` it bootstraps a new fleet with itself as hub from `config.yaml`,
  checking `machines:` and `default_machine` against its own name only.
  `internal/hostconfig` rejects `hub` as an unknown key. `MachineStore.kt` reads
  `fleet-v2.json`, so `fleet-v1.json` is inert. `internal/install/install.go`
  refuses a downgrade while its receipt exists. at 5e1d26d, `notifier.New`
  refuses an observer file that names another fleet.
- dev-server `origin/main` 5f4fef0: `assets/skidbladnir/config.yaml` carries
  `hub: devbox` and `machines: {macbook: {terminal: ghostty}}`.
  `lib/skidbladnir.sh` runs the release installer before it installs the desired
  yaml and runs `skid apply`, so devbox's first new start reads the `config.yaml`
  already there.

steps:

1. dev-server, merged before the release reaches devbox: in
   `assets/skidbladnir/config.yaml` delete `hub: devbox` and replace the
   `machines:` block with top-level `terminal: ghostty` (only the mac reads it, so
   macbook keeps ghostty). make the same edit on devbox in the private
   `~/.config/dev-server/skidbladnir.yaml`, which wins when present (the live
   settings carry `android_push`, which the template lacks, so it likely exists).
   drop a `default_machine` naming another host. bump
   `assets/skidbladnir/release-pin.json` (version, source sha, installer sha256).
   this also discharges w2.3's `machines:` obligation.
2. every host, before installing: `~/.config/skidbladnir/config.yaml` must parse
   under the new schema: no `hub:`, no `machines:` or `default_machine` naming
   another host. on devbox make step 1's edit in the live file by hand. otherwise
   serve exits at every start until dev-server's rewrite lands, and dev-server's
   `skid apply` can fail in that window. macbook and arch hold their own pre-join
   yaml (join never rewrote it); the installed default parses.
3. devbox: install the release (dev-server apply or `skid update`). the service
   bootstraps a fresh fleet with devbox as hub: machine handle kept, old files
   untouched, `notification-observer-v1.json` reset,
   `android-push-credentials.json` reused. `skid status` shows one member, a new
   `fleetId` and an application named `devbox` (the bootstrap name is the
   hostname minus `.local`; there is no rename).
4. macbook, then arch: install the release; each first bootstraps a private
   fleet. on devbox run `skid connect` (one invite per host, single-use, five
   minutes). on the host run `skid join --name macbook` (resp. `--name arch`) and
   paste the line; its service restarts. `skid status` on devbox then shows that
   computer `applied` with its origin. if the origin is missing, run
   `skid connect` on that host once and discard its invite.
5. phone: install the matching apk; it opens "Connect your fleet". scan a fresh
   `skid connect` qr from any computer.
6. verify: `skid status` on devbox lists four members (three computers, one
   phone) and three applications, all `applied`; `skid list` on each computer
   shows devbox, macbook, arch in that order; the phone shows the three machines;
   a needs-input notification reaches the mac app and the phone; with push
   configured, the phone's push status reaches `active`.
7. rollback, only before step 8. each host: `skid uninstall` (keeps settings and
   private state; removes the receipt, services and tailnet route), then the
   previous release's `scripts/install vPREVIOUS`, then its `skid connect` once
   to restore the tailnet route. on devbox, before reinstalling, delete the new
   `notification-observer-v1.json` and revert the dev-server pin; the old
   observer starts a fresh epoch. the old `identity.json`, `host-state.json`,
   `resolved.json` and `authority.json` were never touched and resume. phone:
   android refuses an in-place downgrade, so uninstall the new apk (clearing app
   data), install the previous one, scan a qr from the rolled-back fleet and
   `skid remove` the phone's old device. hub notification memory is lost either
   way.
8. residue, once satisfied: from each state directory
   (`${XDG_STATE_HOME:-~/.local/state}/skidbladnir`) delete `identity.json`,
   `host-state.json`, `resolved.json`, `authority.json`, `enrollment-*.json` and
   `notification-observer-v1.json.recovered-*`; on macbook and arch also
   `notification-observer-v1.json` (inert on a member); from each config
   directory delete `config.yaml.inactive`. the phone's `fleet-v1.json` stays as
   inert app data.
9. carried from [cleanup](../cleanup.md#cross-repo-cutover):
   - w3.1: hub-owned attention discards per-device notification memory.
   - hosts and apk ship in lockstep for w2.1 and w3.1; after w4.1 the phone
     decodes tolerantly and additive wire changes stop requiring lockstep.
   - w3.2: the go quota client removes the native helper and the llm-calling pin
     from skid and dev-server.
   - w3.4: check jarvis's use of `info --explain` output before it merges.
   - jarvis consumes the cli, not fleet files: no jarvis change for w2.1
     (jarvis ce6f488 reads none of the old files or removed status fields).
10. not part of this cutover: #131's launchd `ProcessType=Interactive` and its
    install-time priority check shipped in v0.20.1. its dev-server pin bump and
    macbook install are the owner's, before this release.

resolved when: the release runs on all three hosts and the phone, step 6
passes, step 8's residue is deleted, and every carried obligation is closed or
has its own record.
