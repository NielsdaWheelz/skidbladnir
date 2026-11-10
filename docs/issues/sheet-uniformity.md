# sheet uniformity

problem: the phone's bottom sheets share one body (`SheetForm` in `Theme.kt`)
but still disagree on chrome and busy dismissal.

- title: the group sheet hand-rolls `headlineSmall` Display without SemiBold;
  every other sheet uses `SheetTitle` (`SessionEdit.kt` `GroupSheet`).
- casing: "cancel" in the group sheet, "Cancel" in the rename sheet.
- pending: the group sheet shows Gold "changing group" text; forge and rename
  show a spinner in the commit button.
- busy dismissal: the group sheet disables gestures, Back and outside taps
  while sending. the rename sheet disables only gestures, and the forge nothing;
  their controllers refuse the dismissal instead. a sheet can still animate away
  while its controller keeps the state.
- partial expansion: rename and text size allow it; the other sheets skip it.
- drag handle: M3's default pill reads `shapes.extraLarge` and stays rounded,
  against "corners are cut". the theme leaves `extraLarge` unmapped for that reason.

impact: inconsistent feedback while a mutation is in flight. a sheet can vanish
while its mutation is still pending and its state is still held.

evidence: the emulator journey backWhileBusy (api 35, base and after the
2026-10-22 android ui structure pr alike): one Back hides a pending forge, which
stays set and hidden after its reply; a second Back hides a sending group sheet.
the robolectric busy journeys keep every sheet displayed, so they do not show it.

the [workspace review](workspace-coherence.md) recommends treating this as ordinary
interaction correctness before larger redesign work. use the existing sheet/action
owners for a single title, pending, back/outside/drag, focus-restoration and failure
contract. the proposed first choice is to keep a pending mutation visibly open and
non-dismissable; refusing dismissal in the controller alone is insufficient if the
material sheet has already animated away.

cost: a short operation temporarily retains its modal. the alternative, dismissing
while work continues, needs another discoverable operation/recovery surface and
clear retained draft ownership; that is disproportionate until demanded. a failed
submission must retain recoverable local input. this recommendation does not select
the remaining visual rulings or amend their existing owner decisions.

resolved when: the owner rules on one title weight, casing, pending indicator,
busy-dismissal rule, partial-expansion rule and drag-handle shape. the rulings
then live in `SheetForm` or the sheet call sites, and busy journeys show no
sheet hiding while its controller state remains.
