# installed single-line row review pending

problem: v0.17.3 is deployed and verified; the owner has not yet accepted the
new rows in their terminal theme.

impact: automated geometry/interaction checks cannot establish human readability
or the terminal's treatment of faint text.

evidence: [source qualification](../desktop-single-line-rows.md#source-qualification)
records red/green rendered probes and isolated darwin/linux journeys.
[installed qualification](../desktop-single-line-rows.md#installed-qualification)
records v0.17.3/source `11f89da683502182832d80ad2aa4cf5265963409`, three-host
fleet verification, narrow/wide real-pty smokes, preserved sampled identities and
configuration, and the in-place phone update. temporary probes are retired.

resolved when the owner reviews a newly launched browser at 80×24 and 160×24:
one line per session, faint location, useful priority/omission, recognizable
names and full available info. record the outcome, then delete this issue.
installed owner readability review: `NOT_RUN`.

2026-10-19 source review identifies a naming-policy correction: the deployed
[row contract](../desktop-single-line-rows.md#deterministic-geometry) required
middle elision. the preceding [`tableLines`](../../internal/sessionui/layout.go)
renderer preserved only 13 opening cells at the 28-cell cap. for
descriptive session names, this interrupts the readable opening phrase.
android's card and terminal rail already use end ellipsis.

the [correction and qualification](../session-name-presentation.md#qualification)
record the implemented replacement, costs and passing disposable red/green
integration plus real browser/gateway/tmux journeys on darwin/linux. complete
captured-target confirmation is also qualified and its issue is removed.
installed owner review remains `NOT_RUN`; these source checks do not replace it.
