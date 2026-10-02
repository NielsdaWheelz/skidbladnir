# phone tailscale dns recurrence

problem: on 2026-10-01, the production phone's tailscale vpn remained connected
while name resolution failed. skid could not reach any of its three machines.
restarting tailscale restored hostname resolution and tcp access. the owner then
confirmed all three machines loaded their existing sessions without pairing again.
recurrence has not been excluded.

impact: a phone-local dns outage prevents access to every saved gateway. pairing
again does not repair it. keep skid's origins, trust and saved pairings intact.

evidence: tailscale `1.102.3-t9329c3677-gaea8f60c0` on the API 36 phone;
skid v0.12.2 was installed in place and its apk bytes matched the signed release.
all three numeric peer routes connected on port 8443, while their saved hostnames
and an `example.com:443` control failed. the vpn advertised `100.100.100.100`
as dns and covered skid's process uid. after a full tailscale process restart,
that resolver was advertised again and all four hostname connection probes passed.

[upstream #21155](https://github.com/tailscale/tailscale/issues/21155) describes
a matching android netstack failure after a network change;
[repair #21332](https://github.com/tailscale/tailscale/pull/21332) prevents its
sender from exiting on transient errors. this incident's initiating network
change and sender failure were not observed, so that cause remains a hypothesis.
as of 2026-10-01, the official android stable, `1.102.4-t3caf7d9e7-g8fbef364a`,
[pins backend v1.102.4](https://github.com/tailscale/tailscale-android/blob/1.102.4-t3caf7d9e7-g8fbef364a/go.mod),
whose [sender still exits on those errors](https://github.com/tailscale/tailscale/blob/v1.102.4/wgengine/netstack/netstack.go).
it does not contain the repair. no stable upgrade was presented as a fix.

recovery: force-stop `com.tailscale.ipn`, launch its normal app and reconnect the
vpn without clearing app data. this briefly interrupts vpn traffic. check saved
gateway hostname access and refresh skid's actual inventories afterward.

resolved when: an official stable android release demonstrably contains the
upstream repair, is updated in place, and the saved three-machine fleet still
loads fresh inventories after a wifi/cellular network change. if the outage
recurs, qualify its cause before claiming the upstream repair resolves it.
