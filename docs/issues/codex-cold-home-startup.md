# codex cold-home concurrent startup

2026-10-15: stock codex 0.162.1 on linux can exit 1 when two embedded
`--no-daemon` clients initialize one fresh scratch home concurrently.
isolated native probes reproduced this before footer observation; dead-pane
error predicates identified database migration/initialization failure.

impact: concurrent cold-home embedded startup is unqualified. the footer probe
starts the first client to readiness, then the second; both remain concurrent
for session separation and interactive changes. production daemon startup is
unchanged and was not exercised. this is no evidence of a production outage.

resolution belongs to the native provider. close after an identified native fix
passes concurrent fresh-home startup on an owned isolated socket. no skid retry,
fallback or initialization workaround is introduced.
