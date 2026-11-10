# make fleet setup and recovery intelligible

status: product improvement proposal from the
[workspace review](workspace-coherence.md), not a diagnosed pairing failure.

problem: the reviews value setup that makes several computers feel like one place
to work. skid already has stable identity, invitations, enrollment and direct
access, but those mechanisms need a coherent user journey with stage-specific
recovery. a vague connection error cannot tell the owner what to repair.

impact: the owner may repeat pairing or change healthy configuration when the
actual failure is network reachability, authentication, version or hub availability.

evidence: `cmd/skidbladnir/fleet_cli.go`, `internal/fleet/{fleet,hub,member}.go`,
`internal/fleetclient/config.go`, phone `FleetClient.kt`, `FleetInvite.kt`,
`MachineStore.kt`, and [fleet/trust contracts](../architecture.md#3-topology-and-trust).
no current setup journey or competitor runtime was exercised.

## recommended direction

present one recognizable machine identity and the exact setup stage reached.
reuse existing outcomes to explain reachability, authentication/revocation,
configuration application, version mismatch and hub absence separately. do not
claim a machine is asleep from a connection timeout alone.

keep completed pairing when the current contract supports recovery. the phone
already stages credentials before join and can recover a lost response by fetching;
a computer's lost join response can consume its invitation and requires a fresh
one. preserve those differences instead of promising universal retry. reconnect
does not authorize resending mutations. repair, remove and recover must remain
distinct; `skid recover` creates a new fleet and is not a harmless reconnect button.

preserve local use without tailnet/provider login and cached direct terminal access
during hub loss. the hub remains the owner of membership, accepted settings and
attention; it does not become a terminal proxy. configuration and observation
freshness remain separate from whether a socket is open.

## what to learn from t3

t3 separates persistent environment identity from lan, tailscale, ssh and relay
routes. the inspected release requires an explicit add-route flow for a saved
machine's unknown address; ordinary pairing refuses it. pairing again over a known
address still works
([released route guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/remote-access.md#reach-one-machine-several-ways)).
this corrects the earlier review's broader pairing claim. its clients check an
advertised identity before sending credentials; settings
targets do not silently fall back to another host. its shared connection runtime
owns reconnection and distinguishes connection from synchronized data
([remote architecture](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/docs/internals/remote.md),
[connection runtime](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/docs/internals/connection-runtime.md)).

skid already separates its handle from its origin. retain the existing tailnet tls
and bearer boundary; identity is not authentication. multiple routes would require
an explicit expansion of the current single `*.ts.net:8443` origin contract,
certificate/trust handling, stale-route policy and credential routing. it is a
candidate only after a real need is established, not automatic feature parity.

## costs and acceptance

prefer better guidance and recovery through existing owners first. reject a hosted
relay, automatic hub election, arbitrary tailnet configuration, and another client
retry framework without a demonstrated requirement. these introduce independent
services or trust/availability obligations for little established benefit here.
the [phone dns incident](phone-tailscale-dns.md) is an existing bounded issue, not
proof that the whole transport should be replaced.

resolved when: selected setup improvements pass adding/rejoining a host and phone,
expired/consumed invitations, a lost join answer, invalid accepted settings,
unavailable hub and one offline peer. each outcome names the completed stage and
next valid action without disturbing healthy sessions or copying credentials into
logs. route expansion, if selected, has its own approved contract and evidence.
