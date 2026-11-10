# phone interaction redesign

problem: the owner wants a phone interaction redesign (2026-10-20). the
[workspace review](workspace-coherence.md) now supplies candidate journeys and
priorities; their product scope and acceptance remain unapproved.

impact: no new navigation, terminal controls, gestures, sizing or interaction model
exists. existing device issues keep their own evidence.

recommended sequence: [preserve terminal context](phone-terminal-continuity.md),
[remove notification entry's metadata dependency](phone-entry-metadata-latency.md),
[publish settled keyboard geometry](terminal-resize-churn.md) and
[make sheet behavior consistent](sheet-uniformity.md). retain the separate
[landscape](terminal-landscape-keyboard.md),
[selection](terminal-ime-reshow-selection.md) and
[stock xterm acceptance](stock-xterm-phone-acceptance.md) records.

then consider [launch presets](project-launch-presets.md),
[fleet search](fleet-search.md) and [profile visibility](profile-setup-usage.md).
existing fleet groups, stable dashboard anchors, direct card entry and shared
`terminalLifetimeActions` already provide the foundations. do not replace them
with parallel navigation or action owners.

interaction criteria: retain selection and scroll across terminal visits; keep
loading geometry stable; preserve local form drafts after failures; provide exact
recovery without focus theft. terminal composer input stays provider-owned and is
never reconstructed by replay. keyboard and touch expose the same operation where
applicable. a failed host preserves honestly stale context without disabling
healthy peers. scope process-death restoration separately from foreground return.

trade-offs are recorded with their owners: automatic return can resize the shared
desktop terminal; pending operations may remain non-dismissable; remembered launch
choices stay visible and explicitly overridable. the review did not run the phone,
so these are proposed behaviors and acceptance criteria, not observed improvements.

resolved when: the owner names the high-friction journeys, approves their target
behaviour and an owned implementation plan, and accepts the result on the real
phone; or retires the redesign. it uses the existing design language; visual
changes belong to the [visual redesign](visual-redesign.md). phone operations need
current-turn approval.
