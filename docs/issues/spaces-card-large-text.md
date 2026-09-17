# narrow card actions with enlarged text

problem: v0.5.0 renders the space and kill actions in an ordinary row. at the
supported 170dp minimum card width and 2x text, space consumes 111.67dp and leaves
kill only 30.33dp. the kill label wraps vertically and its target violates the
48dp minimum. the dashboard uses `GridCells.Adaptive(170.dp)` without changing
that minimum for text scale, so this is a reachable product defect.

impact: narrow/enlarged-text card acceptance fails. other passed runtime
boundaries do not establish complete phone platform acceptance.

evidence: the complete s22+ platform sensitivity run using unchanged v0.5.0
runtime and corrected tests failed `minimumWidthLongContentAndLargeTypeKeepEveryStratumInsideTheCard`.
space bounds were `(10, 387.33)..(121.67, 435.33)` dp; kill bounds were
`(129.67, 387.33)..(160, 536.67)` dp. these are geometry only, with synthetic
fixture content. the original immutable-source run stopped at an earlier stale
footer assertion and did not reach this check.

resolved when: a future authorized public release includes the card repair and
records its required device acceptance: readable labels, both 48dp targets,
containment and separation at the minimum width and 2x text. ordinary wrapping at this container is the bounded repair; do not
weaken the assertion or hide the problem by increasing test width. keep release
and corrected-source evidence separate. the user currently forbids a new
version, release, or pin, so the published v0.5.0 remains affected.

source resolution: main now uses a standard wrapping action container. the
signed development proof at `9dc53f4451bb0617280c9bba4553d391af321cc8` passes all
11 card/dashboard tests, including full label bounds and 48dp targets at 170dp
and 2x text. exact public v0.5.0 was restored afterward. the user approved
fixing main without publishing; this issue remains open for deployment, not
for the already verified source repair. close it only after a future authorized
release ships the fix and its required release-bound acceptance is recorded.
