# desktop codex footer mixes model/effort with location

2026-10-15: installed v0.17.1 review requires rows to retain name, session status,
provider, profile, model, effort, machine and one compact current location.
cwd already comes independently from tmux or resolved remote context. its
[presentation refinement](desktop-cwd-truncation.md) needs no provider status-line
configuration change or second cwd source.

problem/impact: the desktop's apparent model/effort column uses `terminalFooter`.
claude supplies a qualified combined model/effort phrase; that remains acceptable
and needs no split into separate columns. codex supplies its entire configurable
status line, which may repeat the directory. live read-only
inventory found the cwd basename in 12 of 13 captured footers; values were not
logged. moving all model/effort into info would discard the owner's requirement.

evidence: `sessionui/details.go:rowFooter`, `sessionui/layout.go:rowMetadata`,
`agentcontrol/codex.go:codexComposer`, and `agentcontrol/footer.go`.
codex's [renderer](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/chatwidget/status_surfaces.rs#L202)
emits arbitrary ordered items without identifying their boundaries semantically.
positions, colors, separators and model-name catalogues do not identify a
model/effort item reliably.

investigated alternative: the codex native protocol exposes thread settings, but skid's
shared remote daemon has no proven active-tui/thread binding. the
[SessionStart payload](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/hooks/src/schema.rs#L499)
and remote initialization do not supply frontend pid/lifetime/pane identity.
loaded thread ids, launch defaults, account config and recent history cannot
substitute for the selected terminal's current facts. no new native integration
or registration scheme has been qualified; this is not a prescribed solution.

resolved when: one current machine/path appears per row, session status remains,
and model/effort display comes from an explicitly qualified source bound to the
observed session. preserve truthful absence, stale/remote rules and full info;
qualify any observation/schema change across all strict consumers. do not strip
path substrings from provider text, guess fields, or change provider status-line
configuration to obtain cwd. no production change is included in this finding.
