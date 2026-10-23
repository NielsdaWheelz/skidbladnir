# phone tailscale dns outage

problem: on 2026-10-01 the phone's tailscale vpn stayed connected while name
resolution failed, so skid reached none of its three machines. restarting tailscale
restored resolution; all three machines loaded their sessions without re-pairing.

impact: a phone-local dns outage blocks every saved gateway. re-pairing does not
repair it; keep skid's origins and pairings.

evidence: tailscale android 1.102.3 on the api 36 phone. numeric peer routes on
8443 connected while saved hostnames and an `example.com:443` control failed; the
vpn advertised `100.100.100.100` as dns. upstream
[#21155](https://github.com/tailscale/tailscale/issues/21155) describes a matching
netstack failure after a network change; its repair,
[#21332](https://github.com/tailscale/tailscale/pull/21332), merged on 2026-09-21
(`3f3e56f`) and is contained in tailscale v1.104.2, which android stable
`1.104.2-tc3430f0d2-gb01fa89d5` (2026-10-21) pins (read with gh on 2026-10-22).
the incident's network change was not observed, so the match is a hypothesis.

recovery: force-stop `com.tailscale.ipn`, launch its app and reconnect the vpn
without clearing app data (vpn traffic drops briefly), then check saved gateway
hostnames and refresh skid's inventories.

resolved when: the phone runs tailscale android 1.104.2 or later, updated in place,
and the three-machine fleet still loads fresh inventories after a wifi/cellular
change. if the outage recurs, qualify its cause before crediting the repair.
