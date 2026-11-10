# visual redesign

problem: the owner wants a visual redesign (2026-10-20). the
[workspace review](workspace-coherence.md) provides a proposed direction below;
surfaces, goals and acceptance remain owner decisions.

impact: readability, recognition, hierarchy and aesthetics have no acceptance
beyond the row, card and chrome checks in [unrun acceptance](unrun-acceptance.md).

## proposed direction and evidence

retain the existing dark stone, restrained gold, angular geometry and recognizable
session seals. the design thesis is coherent; the missing proof is that hierarchy
and interaction serve the work. `DashboardScreen.kt` already uses an adaptive lazy
grid, `ActionMenu.kt` already shares terminal actions, and `Theme.kt` / `Ornament.kt`
already own tokens and artwork. a new card system or component library is not the
insight.

let a compact work/session title, one literal attention sentence and quiet
host/profile/location metadata dominate. move exhaustive machine telemetry behind
its existing disclosure. decoration never substitutes for status or consumes more
attention than the task. maintain literal lifecycle/error/destructive language and
exact target context. no ambient dashboard of charts and agent animations.

the useful air-traffic-strip analogy is stable identity, location, intent and the
next intervention. the studio-workbench analogy is a small working set that stays
where it was left, with [search](fleet-search.md) for discovery and an explicit
[next-needs-me action](attention-dispositions.md) instead of automatic rearranging.

the review recommends reconsidering aesthetic absolutes when they force custom
interaction implementations. `ActionMenu.kt` custom rows partly avoid material's
round ripple; [sheet uniformity](sheet-uniformity.md) shows behavior deserves
priority over shape purity. accepting selected native behavior would be a deliberate
change to the design language, not permission to bypass it now. preserve useful
target sizes, labels, contrast and reduced-motion behavior. the current documented
screen-reader limitation remains explicit; do not claim accessibility acceptance
from visual tokens alone.

cost: less ambient spectacle and occasional native styling in exchange for legible,
predictable controls and fewer custom obligations. test narrow/large-text layouts,
keyboard/focus states, stale/offline data and pending failures before decorative
variants. no rendered comparison, screenshots or hands-on visual acceptance was
performed in the review; source inspection establishes structure only.

resolved when: the owner agrees surfaces and goals, a designer defines content,
hierarchy and narrow and large-text states, the owner approves a bounded change to
the [design language](../design-language.md) and a plan, and accepts the
implemented visuals; or retires the redesign. interaction changes belong to the
[phone redesign](phone-interaction-redesign.md). keep exact targets and
operational feedback throughout.
