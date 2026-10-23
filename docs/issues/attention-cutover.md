# attention cutover

problem: the release after v0.21.0 carries hub-owned attention (#144),
host-rendered copy (#152) and the go quota client (#145) without converters. it
is another lockstep install of three hosts and the phone: the v0.21.0 apk cannot
read the new notification state and rejects the new `status`, `recovery.notice`
and `agent.label` members, and the new apk requires `status`. a new desktop reads
a v0.21.0 host but shows profile keys where labels were.

impact: until hosts and phone match, the phone shows no notifications. after
install, attention restarts on the hub: sessions ready before the install stay
quiet until they work and idle again, and current requests alert once more on
each device. a v0.21.0 binary's `skid update` refuses this release ("bundled quota
helper cannot start; installed release unchanged"): its validate runs
`providers/quota-read`, which the archive no longer carries.

evidence: the hub writes `notification-observer-v2.json`
(`cmd/skidbladnir/standalone.go:66`) and never reads `-v1`; computers read no
`notifications-v4.json`; the phone's push store is `push-v3.json`
(`PushStore.kt:20`), so it ignores `notifications-v3.json` and `push-v2.json`
and re-registers fcm on its first resume; older mac and phone notices are
reconciled away on the first fresh state. `skid notifications` accepts only
`setup` (`internal/agentcli/notifier.go:13`) and the mac setup window has no
reset. the installer extracts only directories and regular files
(`internal/install/archive.go:94-114`), so it cannot extract a pre-#145 archive.

steps, after [release cutover](release-cutover.md) (a host still on v0.20.x
follows that record first):

1. dev-server: bump `assets/skidbladnir/release-pin.json` to this release. delete
   `SPEC.md`'s `## native helper source` section (line 605; its first sentence
   becomes false) and check whether line 400's "native helper/provider launchers"
   still names anything. jarvis pins llm-calling itself and is unaffected;
   llm-calling's `provider_runtime.agent_runtime.quota_cli` loses its only
   consumer, and deleting it is llm-calling's follow-up.
2. in one window, install the release on devbox, macbook and arch (dev-server
   apply or `bash install TAG`, never the old binary's `skid update`) and the
   matching apk on the phone.
3. verify: a needs-input notice reaches the mac app and the phone; a notice
   dismissed on the phone leaves the mac within seconds; entering a ready session
   from a tui clears its ready notice on the mac and the phone; the desktop usage
   page shows `ok` for each codex profile whose daemon runs (the go client was
   proven only against a fake app-server).
4. rollback, only before step 5. each host: `skid uninstall` (the downgrade guard
   refuses an older release over `current`), then the previous release's own
   `bash install TAG`, then its `skid connect` once to restore the tailnet route.
   on devbox, before reinstalling, delete `notification-observer-v2.json`; the
   previous release resumes `notification-observer-v1.json` and every computer's
   `notifications-v4.json`, which this release never touches. phone: uninstall
   the new apk (clearing app data and its notification memory), install the
   previous one and re-pair.
5. residue, once satisfied: from each state directory delete
   `notification-observer-v1.json`; on every computer `notifications-v4.json` and
   `notifications-v4.json.lock` (keep `notifications.sock.lock`). the phone's
   `notifications-v3.json` and `push-v2.json` stay as inert app data.

from this release on, phone and desktop decode gateway documents tolerantly
(`Json.kt:6`), the phone's terminal frames included, so additive members stop
needing lockstep.

resolved when: the release runs on all three hosts and the phone, step 3 passes
and step 5's residue is deleted. the macbook measurement in
[exec overhead](tmux-exec-overhead.md) and the phone pass in
[stock xterm](stock-xterm-phone-acceptance.md) follow this install.
