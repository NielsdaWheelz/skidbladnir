# release rollback guidance

problem: `scripts/render-release-notes:94-98` always requires restoring a saved
fleet client and removing notification serve handlers. these are v0.14.0
notification-cutover steps, not universal rollback requirements.

impact: a later rollback could replace unchanged client configuration or disable
notification ingress unnecessarily. this does not block the current
v0.14.0 → v0.15.0 upgrade.

evidence: the [published v0.15.0 notes](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.15.0)
repeat that generic cleanup instruction. the renderer retains it at the lines
above; no rollback operation was exercised for this finding.

resolved when: future renders defer to the selected target release's actual
rollback procedure. require client-backup restoration and notification cleanup
only when that transition needs them; review rendered guidance against the real
rollback path. this follow-up changes no immutable artifacts or existing published notes.
