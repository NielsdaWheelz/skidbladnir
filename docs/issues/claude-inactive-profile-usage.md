# fresh claude usage without a running session

problem: the accepted [profile usage design](../profile-usage.md) obtains claude
quota fields from devbox's statusline input. it cannot fetch current account-wide quota
usage when no session is producing reports, and an idle callback can repeat
cached data.

impact: inactive claude profiles show last-reported, stale or unknown usage.
this is an explicit first-version limit, not a blocker for its implementation.
centralizing reporting on devbox does not change that acquisition limit; activity
on macbook or arch does not produce or forward a devbox report.

evidence: 2026-10-02 official statusline documentation supplies 5h/7d fields
after the first api response on supported subscriptions. the
[typescript sdk 0.3.287](https://unpkg.com/@anthropic-ai/claude-agent-sdk@0.3.287/sdk.d.ts)
also exposes `usage_EXPERIMENTAL_MAY_CHANGE_DO_NOT_RELY_ON_THIS_API_YET`, whose
`get_usage` control request can fetch account usage. `skipBehaviors: true`
avoids transcript behavior analysis. obtaining its query still starts and
initializes a claude process; startup settings, hooks, persistence and auth
effects need qualification. the current skid helper exposes no such operation.
live getter qualification is `NOT_RUN`.

resolved when: a separately accepted native account-read contract provides
fresh quota data without an inference prompt, terminal interaction, transcript
scan or unqualified startup/persistence effects, and its exact-source behavior
is proved with no active session. prefer a supported provider interface; adopting
an experimental one requires an explicit maintenance decision. this follow-up
does not authorize a getter, private http route, new helper process or daemon.
