# claude request dialogs without a rule

problem: four human-request dialogs claude 2.1.286 can draw have no rule: the
auto-mode default offer, rate-limit options, the consumer-terms update and the
trial-expired notice. they read `unknown/unknown` instead of `needs setup` or
`needs input` ([claude grammar §2.6](../terminal-observation-claude.md#26-request-and-menu-families-no-composer-anchor),
§1 rows 52–53).

impact: a pane waiting on one of them shows `status unknown`, stays out of the
needs-input filter and refuses guarded send. no false idle or false request
results; the user just is not told that a decision is waiting.

evidence: the dialogs exist in the bundle; a local mock with an api key cannot
produce them (they need subscription, quota or account state). the enter-plan and
multi-server mcp dialogs, the other two of this class, were induced with the mock
and now have rules.

resolved when: each dialog is induced live (or its structure is captured
content-free from a real account) and gets a rule with a paired negative, or the
user explicitly waives live qualification for that named dialog, which then keeps
spec §9's unknown; then delete this record.
