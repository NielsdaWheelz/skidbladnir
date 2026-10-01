# terminal observation phone acceptance has remaining rows

problem: the physical-phone run of the [terminal observation](../terminal-observation.md)
cutover (2026-10-01, [record](../terminal-agent-control-qualification.md#2026-10-01-production-cutover-and-physical-phone))
did not cover four spec §8 rows: concurrent work with a request (`work continues`),
the generic `needs input` subtype, a delayed response that must not restore
consumed ready, and talkback's spoken reading of the chip and status bay.

impact: those android paths are proven only by robolectric probes. a device-only
defect there would ship unseen; the other phone rows passed on the device.

evidence: claude cannot show work with a request (an accepted cost), and the live
codex sessions produced no concurrent question or generic input request. the
accessibility labels were read from the view hierarchy, not spoken.

resolved when: each row passes on the phone through the production gateway,
recorded content-free in the qualification; then delete this record.
