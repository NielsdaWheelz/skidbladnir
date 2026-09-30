# reply notifications do not follow terminal entry or resumed work

problem: the desktop skid browser's reported claude and codex `new reply`
markers remain after terminal entry and followups typed directly in the terminal.
current source deliberately acknowledges only through native
`view replies`; terminal attachment and working status acknowledge nothing.
claude prompting is terminal-only, so the ordinary conversation workflow cannot
clear its own notification. this is a product-contract mismatch; the reported
installation and an actual failed acknowledgement have not been verified.

impact: a persistent marker stops conveying useful current attention information.
the existing unread-history policy conflicts with the user's terminal workflow.
terminal entry may address a different conversation from the card's tracked
conversation; clearing is a notification policy, not proof of viewing its text.

evidence: [the current contract](../native-agent-observation.md#5-unread-recovery-and-acknowledgement)
requires native-output acknowledgement. commit `7dc3fee` explicitly retired
terminal-opening acknowledgement. `internal/agentcli/run.go` separates `enter`
from `replies`; `internal/sessionui/unread.go` captures known reply ids for the
reply viewer. android `SkidbladnirController.kt` captures ids in `openReplies`
and acknowledges in `repliesPresented`; its `ConversationSheet.kt` presents the
native output. neither execution-state change nor ordinary terminal input is an
acknowledgement event. this is source review, not live reproduction.

settled direction, 2026-09-29: [terminal reply notifications](../reply-notifications.md)
owns the implementation plan. successful entry and fresh working clear; the whole
terminal visit consumes replies through the first qualified post-visit observation.
the user accepts that closing boundary's loss window. remove the human viewer;
retain machine `read`. working is blue, new reply green, idle grey. these decisions
replace the earlier explicit-dismissal recommendation; no open product question remains.

blocker: [native work-boundary qualification](reply-notification-native-boundary.md)
must prove that delayed old output cannot reappear as a new notice.

resolved when: implement the plan for both providers and clients, update the
owning contracts, and pass its temporary live acceptance, including failed entry,
whole visits, renewed work, delayed history, restart and local persistence errors.
record installed versions and actual desktop/phone boundary evidence.

live/provider/device checks remain `NOT_RUN`. this record changes no production
behavior and authorizes no tmux operation or device access.
