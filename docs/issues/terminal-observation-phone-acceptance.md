# terminal observation lacks physical-phone acceptance

problem: the [terminal observation](../terminal-observation.md) cutover has no
gateway → physical-phone run. spec §8's full-product row requires one: work, each
request label, concurrent work, stale/unavailable, needs-input filter membership
and order, visits, dashboard return restoration and accessibility with large text.

impact: the android projection, needs-input chip, schema-3 task capsule and ready
attention are unproven on a device. a device-only defect (talkback reading of the
chip or status bay, large-text clipping, capsule restoration after a task kill,
pull-to-refresh with the filter on, the controller's ready gates) would ship
unseen. engineering checks cannot close it.

evidence: android probes ran under robolectric only (ingress, projection,
attention sequences, chip, capsule schema 3, narrow and 2× font layout); the
controller was constructed but never started. no adb, emulator or phone run is
recorded for this cutover. the root runs it under its own device approval and
appends the results to the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification).

resolved when: with current-turn approval, a physical phone paired to the real
gateway passes spec §8's full-product and attention rows (including a visit that
keeps an unanswered request, a delayed response that cannot restore consumed
ready, and restoration of the needs-input filter), with content-free results
recorded in the qualification; then delete this record.
