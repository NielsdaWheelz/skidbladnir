# terminal-control phone update pending

problem: phone installation is explicitly pending by user direction; usb
preflight found no device. host deployment is authorized separately.

impact: this release hard-cuts terminal and inventory schemas. the previously
installed phone may reject the updated hosts until its signed app is updated.
no compatibility path or app-data reset is part of this rollout.

evidence: the user selected “deploy hosts; leave phone installation pending”
on 2026-09-30. installation and fresh device checks are `NOT_RUN`. earlier
[terminal evidence](../terminal-agent-control-qualification.md) and the
[merged-phone composition waiver](reply-notifications-phone-composition.md)
remain distinct from installation.

resolved when: install the signed apk from the exact published
[`release-pin.json`](../../release-pin.json) in place with
`scripts/install-android <apk>`, preserving app data; verify the app version,
existing three-host pairings and strict inventory admission. this does not
resolve the separate merged-phone composition qualification issue.
