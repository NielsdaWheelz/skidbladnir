# no retained behavioral suite

problem: behavioral suites and harnesses were retired on 2026-09-17 at the
owner's direction; a change proves itself with temporary tests deleted before
commit ([testing](../rules.md#testing)). the one retained test is the
classifier's frame replay, `internal/agentcontrol/frames_test.go`, run on demand.

impact: `scripts/check verify` runs whitespace, gofmt, shellcheck, go vet, module
verification, go build, `node --check` on the terminal page and android lint and
assemble (`scripts/check:48-83`). it catches invalid source, broken builds and
release-integrity failures, not session targeting, input delivery, pairing,
terminal interaction or client behaviour.

resolved when: the owner chooses and keeps a small executable suite for the
important behavioural guarantees, with measured runtime and demonstrated failure
sensitivity, or accepts the gap, which then lives in rules.md alone. missing
execution is never a pass.
