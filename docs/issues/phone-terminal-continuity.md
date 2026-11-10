# phone backgrounding loses the selected terminal context

status: improvement proposal from the [workspace review](workspace-coherence.md),
not an approved change to phone lifecycle behavior.

problem: `SkidbladnirController.stopForBackground` cancels the client foreground
lifetime, closes its attachment and publishes the dashboard. returning therefore
requires finding and reopening the terminal. this does not stop tmux or provider
work. activity/process recreation also returns to the dashboard by contract.

impact: short interruptions and phone locking break continuity even though the
remote session still exists. recreating context becomes an ordinary repeated task.

evidence: `SkidbladnirController.kt:251-264`, `enterTerminal` at 783,
`TerminalAttachment.kt`, and [phone terminal invariants](../architecture.md#phone-terminal-invariants).
the source baseline and `NOT_RUN` limits are in the workspace review; no physical
phone reproduction was performed for this proposal.

## recommended direction

retain the selected machine and exact session lifetime when backgrounding. on a
direct foreground return to that terminal screen, create a fresh attachment to
that lifetime. detach/leave explicitly still returns to the dashboard. never reopen
a terminal because a notification arrived in the background or merely because the
dashboard became visible.

keep the transport attempt separate from optional display and pane/provider facts,
as proposed for [notification entry](phone-entry-metadata-latency.md). fresh facts
must precede controls that depend on them. do not fabricate a `TmuxSession`, reuse
an old foreground identity, or silently choose a replacement session by name.

restore no input queue or armed modifiers, replay no terminal bytes, and do not
open the keyboard or steal focus. the new xterm page is not a durable transcript.
preserve local form drafts through their existing owner; provider composer input
belongs to the provider, not an application replay buffer.

## material cost and decisions

reattachment is not passive observation. `internal/tmux/attachment.go:64-66`
requires `window-size latest`; a returning phone can resize and reflow the desktop's
shared terminal. the recommendation accepts this for direct return to the selected
terminal. a one-tap resume is the alternative if this consequence is unacceptable.
do not describe either choice as preserving independent client geometry, or add a
second observer transport just to conceal the cost.

decide ordinary background return separately from process death, app restart and
fleet replacement. extending saved-state restoration needs an explicit lifetime
and authority contract; retaining a selection is not retaining authorization.
an unavailable target should keep a useful destination/error view and a deliberate
retry action rather than silently move the user elsewhere.

implementation owner: the existing controller/attachment lifecycle and saved-state
owner if that wider scope is selected. keep the gateway's lifetime check and
[keyboard resize](terminal-resize-churn.md) publication owner. do not add another
terminal state machine or treat current behavior as an accidental regression.

resolved when: the owner selects the return and shared-geometry policy, updates the
phone contract, and verifies lock/unlock, background/foreground, network loss,
session replacement, revocation, keyboard focus and concurrent desktop attachment
at the actual phone boundary. old input must never be resent. process-death
restoration is either separately demonstrated or explicitly outside scope.
