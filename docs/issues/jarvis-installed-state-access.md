# jarvis access to installed skid state

2026-10-21: the owner authorized skid deployment with jarvis disabled and stopped.
this does not block skid's independent rollout. jarvis's
[authoritative issue](https://github.com/NielsdaWheelz/jarvis/blob/f397031fe089b90c601dcadbbc0d5ab0fc98d3a2/docs/issues/skid-installed-state-access.md)
records the access work and later activation criteria.

problem: jarvis's system service runs as `jarvis`, with `ProtectHome=true` and
`NoNewPrivileges=true`. standalone skid's live identity and signed host cache
belong to the enrolled login user, in private `0600` files. jarvis cannot read
that state through its current direct cli invocation.

impact: the generalized api/schema adapter can ship, but installed jarvis cutover
is BLOCKED. copying the files once would leave discovery and membership stale;
changing the service user or adding an authenticated bridge is outside this scope.

evidence: `jarvis/deploy/jarvis.service`, `deploy/install-agent-client`, and
`AgentController._call`; current provisioning copies a private static client json,
while the new cli reads the installed state directory. source inspection on
2026-10-20; no installed jarvis change.

resolution: a separate integration must give jarvis an explicit supported client
boundary, prove live membership/config updates and revocation, preserve exact
terminal targets and uncertain-write receipts, and qualify the deployed service.
the owner explicitly deferred this integration on 2026-10-20.
