# session card hands-on acceptance

problem: the compact [session card](../session-card.md), its overflow menu,
the one-column phone grid and the restored full-height grid under the
floating create seal have not been seen on the phone.

impact: the source was checked only by a temporary Robolectric render and
semantics harness on 2026-10-02. renders do not establish touch feel, the
menu's real popup placement near screen edges, TalkBack traversal and
custom actions on a device, or how the seal overlaps a card mid-scroll.

evidence: `SessionCard.kt`, `SessionActions.kt` and `DashboardDwarfGrid` in
`DashboardScreen.kt`. the old 84dp viewport exclusion below the grid (reported
by the user on 2026-10-02 as an unwanted bottom strip) is now trailing content
clearance, so cards use the space beside the seal.

reproduction: install a build carrying this change and open the dashboard
with several sessions across machines and groups.

resolved when, on the phone, with current-turn device approval:

- a common card is one compact row; `All` shows the machine and a machine
  filter hides it;
- the overflow opens beside its card, rows run change group, send interrupt,
  a rule, then the two Ember closures; both closures confirm; a fenced
  machine disables the overflow;
- TalkBack reads one card sentence, offers the four custom actions, then the
  overflow button;
- at large text and in landscape nothing clips, and the last row's overflow
  scrolls clear of the seal;
- a status change from `working` to `needs permission · work continues` does
  not move neighbouring cards (the detail ellipsizes instead).
