# codex residual ambiguity has no ruling

problem: the codex grammar's residual items r1–r7
([grammar §6](../terminal-observation-codex.md#6-accepted-costs-and-residual-ambiguity))
are neither accepted costs nor repaired. the implementation reads as each item
states.

impact: two can claim falsely: r6 in a pane at most header + 4 columns wide
during a millisecond window, and r5 after a provider kill under a silent shell.

- r6: during the millisecond stale-`Ready` window of `/compact`, review entry or
  the mcp pending turn, a pane at most header + 4 columns wide (≤ 22 for
  `• Compacting context`) reads idle instead of `evidence_conflict`. that is a
  false idle outside spec §9's accepted exceptions, and it can mint ready: the
  window can follow an armed working sample when `/compact` or a review starts
  within one poll of a turn whose working was sampled.
- r5: after the provider is killed, a shell that prints nothing, with the old
  cursor above an intact request overlay, leaves the old request readable until
  the relaunched provider draws, so the row can enter the needs-input filter and
  `skid wait --state needs-input` could return early. ready cannot fire across
  the foreground change.

r3 and r4 claim falsely under their own conditions: r3 reads a wrong picker
kind when a picker whose header has no bold row is drawn under a transcript row
that starts with a listed title; r4 reads a run-state cue, idle included,
from an unmanaged status line whose first item is a user value equal to a
run-state word, which the managed profile cannot produce. r1 is a true idle
narrower than a reader may assume: background terminals can survive it. r2 and
r7 read unknown or a request of another subtype.

evidence: grammar §6 and upstream source. r5's live samples read layout_unknown
for the 0.32–0.42 s before the first draw; the silent shell was not induced. r6's
header-cut status row was seen live; its window and width bound come from source.
r7's widths were measured live.

resolved when: each item is accepted into grammar §6's costs (and, where it
claims falsely, spec §9) or repaired with a paired negative; then delete this
record.
