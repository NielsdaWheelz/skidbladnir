# behavioral coverage after test retirement

problem: the 2026-09-17 user-directed reset removes the behavioral suites and
test harnesses. the subsequent cleanup deliberately removes its temporary
integration/live tests before each commit.

impact: engineering checks can catch invalid source, broken builds, asset drift,
and release-integrity failures. they cannot establish correct session targeting,
input delivery, pairing, terminal interaction, or client behavior. existing
acceptance gaps and historical failures remain.

evidence: go test files, android unit/instrumentation source sets, shell
self-tests, owned xterm assertions, and `scripts/test` are removed.
`scripts/check verify` runs only static checks and builds. xterm's upstream mock
signature adaptations remain solely because its retained build compiles upstream
test sources; no xterm test runner or repository-owned assertions remain.

resolved when: the owner chooses and retains a small executable suite for the
important behavioral guarantees, with measured runtime and demonstrated failure
sensitivity. no replacement suite is scheduled under the current temporary-test
workflow. external boundaries still require their current-turn approvals;
missing execution is never a pass.
