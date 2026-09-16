# spaces and shells platform assertions

problem: six existing platform methods still assume the pre-spaces/shells ui.
the exact v0.5.0 device run fails them. these failures do not by themselves
establish product regressions, but their unexecuted assertions prove nothing.

| failed method | evidence and required correction |
| --- | --- |
| `dwarfCollectionPullKeepsContentAndExposesOnlyActiveCheckingProgress` | expects the first card 12dp below grid top; actual 54dp includes the required full-span space heading. assert first rendered-item geometry while retaining the pull/indicator contract. |
| `machinePressureRailIsCompactAccessibleAndDisclosesOnlyItsMachine` | expects a 16dp rail-to-card gap; actual 58dp includes the heading. distinguish heading content from whitespace. |
| `dashboardReturnKeepsMachineAndSemanticViewportThroughDetachAndBack` | old first-card/index offset formula fails while setting up the non-top position, before `resetAll()`. use rendered-item anchors; final reset/return behavior remains unproved. |
| `commonCardLeadsWithWorkAndOwnsExactContextInBothScopes` | requires at most 200dp; actual 220dp includes the accepted additional action row. verify containment/readability/targets against the current contract. |
| `minimumWidthLongContentAndLargeTypeKeepEveryStratumInsideTheCard` | expects footer and kill on one horizontal row. the accepted footer now sits above separate space/kill actions; verify that geometry and target separation. |
| `terminalChromeRendersHeaderTextSizeRecoveryAndRenameOverTheProductionScreen` | queries visible text `Detach`; the accepted compact glyph exposes spoken name `Detach`. use the accessible action and retain recovering-state checks. |

impact: full release-bound platform acceptance is failed; restoration, card
geometry, and recovery checks after these assertions were not reached. keep
passed method evidence separate from these failures. accepted contracts are
spaces §§9–10 and shells §§1,7; do not shrink targets or undo those changes to
satisfy obsolete assertions.

reproduce: the documented complete v0.5.0 release-bound platform command on the
approved s22+, with the separate clean exact-source checkout and isolated shell
fixture. the roadmap records run counts and recovery.

resolved when: bounded assertions/setup reflect the accepted rendered-item,
card-action, and accessible-header contracts; independent review confirms their
sensitivity; the complete governed device run passes and restores the exact
public apk with pairing preserved. the immutable release test result remains
failed even after a later corrected-source run succeeds.
