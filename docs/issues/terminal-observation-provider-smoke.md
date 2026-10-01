# terminal observation has no live linux provider smoke

problem: spec §8 asks for a live ordinary-provider smoke separately from the
scripted runs. darwin ran it on 2026-10-01 against the real codex and claude
services ([record](../terminal-agent-control-qualification.md#2026-10-01-production-cutover-and-physical-phone));
devbox and arch have not.

impact: linux screens only a real service produces (pacing, rate limits,
account and plan notices, server-gated layouts) are unexercised through the
classifier on the installed linux provider versions.

evidence: every linux classifier run drove codex and claude against local scripted
endpoints with dummy credentials.

resolved when: on devbox and arch, each provider's managed launch runs against its
real service and idle, working and at least one request classify as expected,
recorded content-free in the qualification; then delete this record.
