# attention cutover

problem: #144 (w3.1, hub-owned attention) moves notification acknowledgement to
the hub and discards device memory without converters. the first release that
carries it, after v0.21.0, is another lockstep install of three hosts and the
phone: the old apk cannot read the new notification state.

impact: until hosts and phone match, the phone shows no notifications. after
install, attention restarts on the hub: sessions ready before the install stay
quiet until they work and idle again, and current requests alert once more on
each device.

evidence: the hub writes `notification-observer-v2.json` and never reads
`notification-observer-v1.json`; computers no longer read `notifications-v4.json`;
the phone ignores `notifications-v3.json` and `push-v2.json` and re-registers fcm
on its first resume; older mac and phone notices are reconciled away on the first
fresh state. `skid notifications reset` is a usage error and the mac setup window
has no reset button.

steps, after [release cutover](release-cutover.md) (a host still on v0.20.x
follows that record first, with this release, then these steps):

1. in one window, install the release on devbox, macbook and arch (dev-server
   apply after its pin bump, or `bash install TAG`) and the matching apk on the
   phone.
2. verify: a needs-input notice reaches the mac app and the phone; a notice
   dismissed on the phone leaves the mac within seconds; entering a ready session
   from a tui clears its ready notice on the mac and the phone.
3. rollback, only before step 4: on devbox delete `notification-observer-v2.json`
   before reinstalling the previous release, which resumes
   `notification-observer-v1.json` and every computer's `notifications-v4.json`
   (this release never touches them). phone: uninstall the new apk (clearing app
   data, so its notification memory is lost), install the previous one and
   re-pair.
4. residue, once satisfied: from each state directory delete
   `notification-observer-v1.json`; on every computer `notifications-v4.json` and
   `notifications-v4.json.lock` (keep `notifications.sock.lock`). the phone's
   `notifications-v3.json` and `push-v2.json` stay as inert app data.

resolved when: the release carrying #144 runs on all three hosts and the phone,
step 2 passes and step 4's residue is deleted.
