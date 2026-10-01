# fresh-install documentation

problem: there is no verified, ordered walkthrough from a fresh supported host
to a usable desktop client and paired phone. installation belongs to the external
dev-server repository; this repository mostly supplies contracts and historical
qualification records.

impact: a new reader can build the cli but cannot complete deployment from the
readme alone. extracting a release archive does not provision the gateway,
provider accounts, client configuration, or phone pairing.

evidence: [deployment ownership](../dev-server-handoff.md#owned-installation-and-runtime)
names the installer and fleet provisioning. the
[older distribution document](../public-fleet-distribution.md#42-host-apply)
contains a host-config example without the required `nativeControlPath` and
`zoxidePath`, and declares one profile where the current parser requires zero
or the complete ordered four. the current schema lives in the
[deployment contract](../dev-server-handoff.md#host-config-and-validator) and
[template](../../deployment/providers/host-config.json).

resolved when: one ordered guide identifies external prerequisites and links to
their authoritative setup instructions, covers the supported fleet's host install,
private client provisioning and phone pairing, and reaches an observed first
attachment. remove or redirect obsolete setup examples. qualification must use
an explicitly approved environment; this issue does not expand supported fleet
or account configurations.
