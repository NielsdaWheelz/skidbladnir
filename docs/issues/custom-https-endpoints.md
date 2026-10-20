# custom https endpoints

status: deferred by the owner during the standalone-installation design discussion.

decision: the first generalized release requires tailscale for remote access.
local-only use should remain independent of tailscale and internet access.
custom https endpoints are outside that release's scope.

problem: users cannot substitute their own https endpoints on another vpn or
lan for skid's tailscale-based remote setup.

impact: remote users must adopt tailscale even when they already have suitable
private connectivity and certificate management.

evidence: the [network contract](../architecture.md#2-fixed-contract) delegates
tls to tailscale serve. the [peer configuration reader](../../internal/fleetclient/config.go)
requires canonical https endpoints on port 8443. the owner explicitly chose to
retain the tailscale prerequisite and defer custom endpoints.

revisit when: a concrete non-tailscale deployment is needed. settle certificate
trust and renewal, ingress ownership, address changes, and installer support
before expanding the contract. this issue does not authorize that expansion.

resolved when: an approved later scope supports user-supplied trusted https
endpoints without tailscale, documents the external setup responsibilities, and
verifies enrollment, direct session access, and hub communication over that
path. preserve the standard tailscale setup and independent local-only use.
