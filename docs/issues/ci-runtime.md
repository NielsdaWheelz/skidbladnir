# hosted ci runtime after input selection

problem: host-only and docs-only hosted durations have not yet been measured.
the previous workflow took 4m50s even for a desktop-only change.

impact: the estimates of 10–30s for docs and 20–60s for host remain estimates.

evidence: [the baseline run](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36748266365)
spent 3m08s on android compile/lint and 1m08s on a separate apk build. local
full verification passes with one 38s gradle invocation; host and cheap scopes
pass in 2.4s and 0.8s respectively. temporary git-diff checks pass for scopes,
renames, deletions, multi-commit pushes, absent bases and failed diff collection.
the first hosted combined build exhausted the former 768mb gradle heap during
dex merging. the 2gb heap fixes it: [cold hosted verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36753072510)
passes in 3m31s, including cache upload. [pr 35](https://github.com/NielsdaWheelz/skidbladnir/pull/35)
records subsequent cache reuse verification and timings.

resolved when: hosted runs record host-only and docs-only job durations. scoped
main runs require the workflow change to be merged first.
