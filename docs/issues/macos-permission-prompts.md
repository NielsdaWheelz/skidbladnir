# repeated file-access prompts naming skid need attribution

problem: the owner reports repeated macos file/folder permission requests naming
skid, including categories such as documents or downloads. the operation behind
each prompt and whether an existing grant is being requested again are unknown.

impact: the owner cannot reliably tell whose access is being approved or why
consent is requested repeatedly. changing process attribution alone would not
eliminate legitimate permission requests.

evidence: the owner confirmed file/folder prompts during the 2026-10-29
investigation, rather than automation requests to control terminal or ghostty.
the [confirmed responsibility chain](macos-process-attribution.md) explains how
provider access can be attributed to skid, but no individual prompt was correlated
with its requesting process. skid also reads directories directly
(`internal/workdir/service.go:99,127`) and provider metadata during recovery
(`internal/sessions/recovery.go:478`).

the loaded v0.23.0 gateway's code-signing metadata reported an ad hoc signature,
identifier `a.out` and no team identifier. whether identity changes across releases
contribute to renewed prompts is untested; signing is a lead, not an established
cause. distinct protected locations or requesting identities may also explain
repeated prompts without any lost grant.

follow-up: correlate a reproduced prompt with its permission category, requesting
process, responsible identity and prior consent. distinguish new access from the
same request after a grant, and check restart/update behavior only if reproduction
implicates them. use disposable content and retain only content-free evidence.
do not reset the owner's permissions or grant broad access as a diagnostic shortcut.

resolved when: the repeated requests have a reproducible explanation; any defect
in attribution or identity continuity is repaired and the same approved access
does not unnecessarily prompt again. required consent for new access remains
expected. prompt reproduction and installed lifecycle checks require the owner's
current-turn live-state approval; none were performed in this investigation.
