# exact-session handoff between devices

status: exploratory product proposal from the
[workspace review](workspace-coherence.md), not an existing deep-link capability.

problem: work can be opened from enrolled devices, but there is no explicit
user-facing handoff that carries the exact selected session to another client.
the owner must find it again and verify that it is the same work.

impact: continuity across laptop and phone is interrupted by navigation and
identity reconstruction, even though execution need not move.

evidence: existing target/reference types in `internal/fleetclient/request.go`,
`terminalclient`, phone `SessionTarget` / `SkidbladnirController.kt`, and the
notification entry path. this issue proposes reusing identities, not copying
terminal content, provider transcripts or authorization.

## candidate contract

an explicit handoff opens the same captured machine and session lifetime on another
already-authorized device. the receiver uses its own membership and credentials;
a reference never grants access and must carry no bearer or provider secret.
machine labels and tmux names are display only. a missing/replaced session or
revoked device gets an explained refusal, never a name-based fallback.

choose the target granularity explicitly. attaching a session and controlling a
specific pane are different operations; a session link must not authorize a stale
pane action. the receiving client obtains current pane/provider facts before any
dependent controls. link encoding, dispatch to the app and desktop support need a
bounded design; do not assume the existing notification encoding is a public url.

handoff does not transfer execution, copy a conversation, synchronize a directory,
answer a question or move an account. opening on the phone can change the shared
tmux geometry; reuse the [continuity](phone-terminal-continuity.md) contract.
mere creation/receipt of a link must not attach or resize a terminal in the background.

## alternatives and cost

start with an exact-session link or equivalent local handoff affordance. a copied
instruction, a transcript summary and migrated execution are distinct products;
do not market them under the same guarantee. bb's Handoff plugin advertises session
adoption/import, which is not evidence of attaching the same live tmux lifetime
([listing](https://getbb.app/marketplace/handoff)).

cost: client link handling, stale-reference recovery and the shared-window sizing
trade-off. avoid a new delivery service or notification mechanism for this alone.
decide whether retaining local form drafts across devices is actually required;
it introduces a draft synchronization owner and is not implicit in session handoff.

resolved when: the owner selects a delivery affordance and target contract, and
desktop-to-phone/phone-to-desktop opening proves exact identity, authorization,
stale-target refusal, fresh controls and deliberate geometry changes. no provider
content or credential travels in the reference. otherwise retire the idea explicitly.
