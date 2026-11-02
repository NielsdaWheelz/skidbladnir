# Joining does not make a computer reachable

Problem: `skid join` enrolls a computer but neither points Tailscale Serve's port
8443 at its gateway nor sets its origin; it reports an origin only if an earlier
`skid connect` set one. Only `skid connect` does both (`cmd/skidbladnir/fleet_cli.go`, `routeGateway` and `PUT /v1/local/network`),
so every joined computer must also run `skid connect` once and discard the
invitation it prints.

Impact: until then, phones and other computers cannot reach the new computer,
and the hub lists it without an origin. The extra step reads as a ritual, and
missing it looks like a broken fleet.

Evidence: 2026-10-26, after the v0.23.0 update, the hub listed the macbook
without an origin until `skid connect` ran there. The README, `docs/install.md`
and the architecture's fleet operations all document the extra step.

Resolved when: `skid join` routes port 8443 and reports the origin, with the
same Tailscale checks `skid connect` makes, or refuses with the fix it needs; the
docs drop the extra `skid connect`.
