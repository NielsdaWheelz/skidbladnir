# phone forge and copy qualification

problem: the final physical-phone journey covered terminal creation from `z`,
remote context, provider replacement, detach/reopen, and exact source close.
it did not record an initial-agent Forge launch or terminal selection and copy.

impact: the retained Forge controls and explicit Copy action are unqualified on
the installed signed `v0.10.3` apk. this is separate from the unexplained
session-lifetime loss in the
[phone preservation issue](terminal-continuity-phone-qualification.md).

evidence: [pr 11](https://github.com/NielsdaWheelz/skidbladnir/pull/11)
proved the remote header opens a source-scoped Forge on a signed candidate.
[pr 10](https://github.com/NielsdaWheelz/skidbladnir/pull/10) records the final
apk's named journey but no initial-agent Forge or Copy outcome. these parts of
the [phone row](../terminal-continuity.md#6-delivery-and-adversarial-acceptance)
are `NOT_RUN` for the final apk.

resolved when: with current-turn phone approval, use the exact installed signed
apk to launch an initial agent through the Forge, retain its shell after exit,
and exercise explicit terminal selection and Copy. record only content-free
pass/fail results, then remove this record.
