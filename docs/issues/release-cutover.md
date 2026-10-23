# release cutover

problem: the cleanup campaign changes wire and durable state without converters
or old-format readers. w2.1 (fleet-fixed-hub) replaces `identity.json`,
`host-state.json`, `resolved.json` and `authority.json` with one `fleet.json` per
host, drops signatures and the yaml `hub:` key, and gives the phone a new store.
w2.2-w2.8 add the installer receipt, the `machines:` cut, the mac serve log, the
compiled-in catalogue and a new terminal-context record format. v0.21.0
(`2c7cbe0`, pinned in `release-pin.json`) is therefore a fresh fleet, installed in
lockstep across dev-server, three hosts and the phone. the
[attention cutover](attention-cutover.md) holds the next release's obligations.

impact: a skipped step leaves a host exiting at every start (yaml rejected at
bootstrap), a computer outside the fleet, or the phone unpaired. hub notification
memory and the observer epoch reset; desktop machine order becomes hub first,
then join order. until a host joins, it serves only its own terminals.

evidence:
- skid: `internal/fleet` reads only `fleet.json` and `machine-handle`; without
  `fleet.json` it bootstraps a new fleet with itself as hub from `config.yaml`,
  checking `default_machine` against its own name only. `internal/hostconfig`
  rejects `hub` and `machines` as unknown keys (`config.go`, `KnownFields`). `MachineStore.kt` reads
  `fleet-v2.json`, so `fleet-v1.json` is inert. `internal/install/install.go`
  refuses a downgrade by reading `current/release.json`, ignores the old
  `install-owned.json` receipt (48,821 entries on the macbook) and, on its first
  good run, deletes it and every release except `current` (7 trees, 1.2 GB
  there). an older installed binary's `skid update` still keeps the receipt and
  prunes nothing; after the receipt is gone, a pre-cutover installer refuses the
  host ("existing path is not installer-owned") until `skid uninstall`. an
  observer file that names another fleet is replaced with a fresh epoch.
- dev-server `origin/main` 4630064 still pins v0.20.1, and its
  `assets/skidbladnir/config.yaml` still carries `hub: devbox` and
  `machines: {macbook: {terminal: ghostty}}`.
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
3. devbox: install the release with dev-server apply or the release's
   `bash install TAG`, not the old binary's `skid update`. the service
   bootstraps a fresh fleet with devbox as hub: machine handle kept, old files
   untouched, `notification-observer-v1.json` reset,
   `android-push-credentials.json` reused. `skid status` shows one member, a new
   `fleetId` and an application named `devbox` (the bootstrap name is the
   hostname minus `.local`; there is no rename).
4. macbook, then arch: install the release the same way; each first bootstraps a private
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
   configured, the phone's push status reaches `active`. on macbook,
   `${XDG_STATE_HOME:-~/.local/state}/skidbladnir/serve.log` exists with mode
   0600 and gains `Request.Completed` lines; it stays at or under 8 MiB beside
   at most one `serve.log.1`.
7. rollback, only before step 8. each host: `skid uninstall` (keeps settings and
   private state; removes `current`, every release, `bin/skid`, the services,
   the shell blocks and the tailnet route), then the
   previous release's `scripts/install vPREVIOUS`, then its `skid connect` once
   to restore the tailnet route. on devbox, before reinstalling, delete the new
   `notification-observer-v1.json` and revert the dev-server pin; the old
   observer starts a fresh epoch. the old `identity.json`, `host-state.json`,
   `resolved.json` and `authority.json` were never touched and resume. phone:
   android refuses an in-place downgrade, so uninstall the new apk (clearing app
   data), install the previous one, scan a qr from the rolled-back fleet and
   `skid remove` the phone's old device. hub notification memory is lost either
   way. terminal-context records under `~/.cache/skidbladnir/terminal-contexts`
   change format: the previous release cannot read new ones and drops them at its
   first registration, so reopen those terminals. it also refuses a workspace
   checkpoint above its old caps (256 sessions, 512 windows, 1024 panes) as
   `limit` until the next save.
8. residue, once satisfied: from each state directory
   (`${XDG_STATE_HOME:-~/.local/state}/skidbladnir`) delete `identity.json`,
   `host-state.json`, `resolved.json`, `authority.json`, `enrollment-*.json` and
   `notification-observer-v1.json.recovered-*`; on macbook and arch also
   `notification-observer-v1.json` (inert on a member); on macbook also
   `gateway.log` (249 MB, the dev-server-era event log; nothing writes it); on
   every host `~/.cache/skidbladnir/terminal-contexts/.lock`; from
   each config directory delete `config.yaml.inactive`. the phone's `fleet-v1.json` stays as
   inert app data. confirm each host has no `install-owned.json` and exactly one
   `releases/` entry, the one `current` names; then delete the two
   `install-owned.json` removals in `internal/install/install.go` (`Run` and
   `uninstall`).
9. carried obligations:
   - dev-server `docs/gateway-separation-validation.md` links skid
     `docs/native-agent-qualification.md` on main, deleted by #122; pin the
     link to `12b1516`.
   - jarvis adr 0044 (line 45) names skid `docs/agent-control.md` as the wire
     contract and adr 0045 (line 4) cites `docs/agent-control-ux.md`; both docs
     are deleted and their contract is architecture
     [§7](../architecture.md#7-observation-and-agent-control) and
     [cli](../architecture.md#cli). pin the names to `ba60841` in jarvis.
   - jarvis consumes the cli, not fleet files: no jarvis change for w2.1
     (jarvis ce6f488 reads none of the old files or removed status fields),
     and none for #133 (jarvis f397031 never reads `skid info --explain`).
10. macbook priority: this install applies `ProcessType` `Interactive`;
    [launchd priority](launchd-standard-priority.md) holds the check after the
    tmux restart.

resolved when: v0.21.0 runs on all three hosts and the phone, step 6 passes,
step 8's residue is deleted, and every carried obligation is closed or has its own
record.
