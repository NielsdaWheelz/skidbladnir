# terminal attention cannot describe pending human input adequately

problem: the ordinary status contract collapses every recognized dialog to
blocked/waiting and cannot distinguish permission, question, setup, or a menu
the user opened. pending questions may coexist with continuing work. neither
current execution nor a pending request establishes the last turn's outcome.

impact: the user cannot reliably find agents needing a decision; missed dialogs
can look idle or unknown; a visible menu can be mistaken for an agent request.
the existing ready indicator can suggest completion after a false idle sample.

evidence, 2026-09-30, source `564d32f`:

- `internal/sessions/types.go:99` and
  `android/app/src/main/java/dev/niels/skidbladnir/TerminalControl.kt:7` admit
  only working, blocked, idle and unknown. the source field distinguishes
  terminal observation from unavailable observation, not request kind or outcome.
- `internal/fleetclient/response.go:560` and android `ProductModel.kt:987`
  display blocked as waiting. the parser returns no input-request category.
- desktop `internal/fleetclient/notifications.go:222` and android
  `NotificationStore.kt:108` clear ready attention on blocked. existing attention
  belongs specifically to observed working-to-idle transitions; there is no
  separate pending-input notification state. the waiting status itself remains
  visible, so this is a missing attention contract, not total suppression.
- stock codex 0.159.2 has structured answer forms and a collapsed pending-question
  widget. those are evidence of an available human interaction; not every
  question means all work stopped. inspect the versioned
  [question widget](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/bottom_pane/questions.rs).
  a question in ordinary final prose supplies no equivalent structural signal.
- terminal observation intentionally has no done/failed/stopped inference under
  `docs/terminal-agent-control.md:90`. a stopped-looking screen, ordinary error
  output, or return to a shell cannot prove task success, process crash, or the
  absence of background work. an interruption receipt proves input delivery.

proposal for decision, not an accepted schema: separate observed activity,
visible human interaction, observation health, and a specifically evidenced
last outcome. use a concise primary label such as needs permission or question
waiting, with working as a secondary fact when independently visible. keep
unknown request type honest. label user-opened menus as interaction open, not
an urgent agent request. a current request remains visible after the user visits;
visiting may acknowledge a notification but does not answer the request.

the repair is a small extension of the shared provider screen classifier and
existing client attention owners. parsing can return activity and interaction
evidence independently of composer safety. input transport, native conversation
tracking, provider hooks and provider execution remain outside this proposal.
an unavailable observation must not clear a request as though it was answered;
any retained state must be visibly last observed, invalidated on target/process
replacement, and never used as fresh control authority.

comparative evidence:

- [herdr](https://herdr.dev/docs/agents/) rolls blocked state into its containing
  workspace and exposes detector explanations. its non-codex idle fallback is
  explicitly best effort. copy the attention intent, not that fallback.
- [warp](https://docs.warp.dev/agents/capabilities/agent-notifications/) separates
  request, completion and error notifications. its third-party integrations use
  plugins/configuration; their coverage is not proof screen scraping is complete.
- [cmux](https://cmux.com/docs/notifications) separates notification receipt,
  unread state and clearing. [issue 9523](https://github.com/manaflow-ai/cmux/issues/9523)
  reports false idle/needs-input and background-work mistakes; it is upstream
  user evidence, not a locally reproduced benchmark.
- [superset](https://docs.superset.sh/agent-status) documents lifecycle hooks and
  a manual clear-status action after force quitting. it offers useful attention
  presentation, not evidence that hooks eliminate stale state.
- [mixed-initiative interface research](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/11/chi99horvitz.pdf)
  motivates matching interruption and automation to evidence and the cost of
  being wrong. applied here: finding a decision deserves a stronger cue than
  ordinary activity; a status guess must not imply safe input or successful work.

resolved when: an agreed screen matrix distinguishes permissions, structured
questions, elicitation, setup and user-opened menus; concurrent activity remains
representable; stale or failed observations cannot fabricate resolution; and
phone/desktop attention remains consistent through visits, answers, cancellation,
provider replacement and reconnection. demonstrate false-idle cases cannot mint
ready attention. use approved isolated live tests plus synthetic negative cases
under the current testing policy; no existing engineering gate proves this.

blockers: the [implementation plan](../terminal-observation.md) now owns the
accepted interaction/attention scope. implementation and live qualification are
`NOT_RUN`. crash classification and
semantic detection of questions in prose have no sufficient terminal-only
evidence and are not promised. related parser and capture defects are owned by
[terminal status detection](terminal-status-detection.md); existing claude
[recognition](claude-launch-spelling.md) and
[identity](claude-resume-identity.md) issues retain their separate ownership.
