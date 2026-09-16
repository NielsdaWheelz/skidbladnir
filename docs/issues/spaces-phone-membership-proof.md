# spaces phone membership proof

problem: the v0.5.0 phone suite never sends a membership edit to the real
isolated gateway. spaces acceptance remains incomplete even if the complete
release-bound platform gate passes.

evidence: `SpacesInstrumentedTest.selectorEditorAndHeadingCapsuleRetainExactNavigationIntent`
uses real compose/registry with synthetic inventory; its submit callback only
increments a counter. `ShellsInstrumentedTest` uses the real controller/gateway
to create terminals with initial/inherited membership, but never exercises
set/change/clear through the editor. the product journey tests fleet/update/
outage behavior, not membership mutation.

impact: actual phone membership dispatch, pinned target, ordered refresh/fence
completion, and resulting filtered collection are runtime-unproven. host tests
and component navigation tests establish their own boundaries only.

resolved when: one approved real phone → isolated gateway/tmux journey exercises
set/change/clear, cancellation/invalid drafts, intersecting machine/space filters,
and return/restoration through the existing controller; it verifies exact target
and source-session preservation and records actual results. retain the complete
exact-source platform result and exact public-apk/pairing restoration separately.

known constraint: the immutable release fixture has no interactive hold, and
its tests lack this journey. changing that fixture would require separately
attributed test evidence; editing the clean release checkout or weakening its
admission cannot make the missing v0.5.0 proof pass.
