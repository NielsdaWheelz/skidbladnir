# shells phone fixture rejects its fleet

problem: the complete v0.5.0 release-bound platform run fails
`ShellsInstrumentedTest.terminalCreationKeepsExactTargetsAndCurrentNavigation`
with `expected Installed but was InvalidFleet`, before the shell journey.

evidence: `tests/integration/android_platform_test.go` assigns the real gateway's
one bearer to all three fixture peers. `MachineStore.uniqueCredentials` excludes
every member of a duplicated-bearer group, so `isExactFleet` rejects the fixture
before encryption or persistence. the production validation is correct.

impact: phone forge/header creation, duplicate submission, actual attach/back,
late completion/recreation, inheritance, and narrow/enlarged-header checks in
that journey were not reached. a built and installed companion does not prove
these behaviors.

reproduce: run canonical `scripts/test platform --allow-device-mutation
--allow-isolated-tmux-mutation` with approved s22+, exact public v0.5.0 installed,
clean pin-control checkout, separate clean `df7ad49` source, and the documented
release-apk/source/isolated-tmux environment. retain exact public-apk restoration.

resolved when: the fixture retains the real gateway bearer for its reachable
peer and mints distinct canonical bearers for the two unavailable peers; the
real phone shell journey and complete governed gate then pass. keep the
production fleet-uniqueness rule. the immutable source was not patched, and
the failed run was not retried or admitted through a weaker prerequisite.
