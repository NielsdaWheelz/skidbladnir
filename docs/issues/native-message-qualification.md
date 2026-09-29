# claude held messages can cross conversation boundaries

problem: claude 2.1.284 validates the recipient before holding a peer message,
but approval after `/clear` can deliver it into the successor conversation.
kernel process and socket checks cannot repair provider-owned admission.

impact: native peer send cannot satisfy the exact-conversation contract. skid
rejects all claude native input before provider discovery; the unsafe adapter is
removed. native status/history/results and matched background stop remain.
prompting requires deliberate terminal text/keys. this is an upstream capability
limit, not a blocker for the implemented unavailable-capability contract.

evidence, 2026-09-28: an isolated actual native receiver paused `/clear` with a
temporary supported `SessionEnd` command hook. send an old-session peer while
paused; observe its approval dialog; release the hook; confirm the same process
has a new session id and new sends with the old id fail before dispatch. the
original dialog remains. approve it, then approve a successor control. the sdk
reads both messages and the finalized control reply from successor history.
the original must be absent there; it is present. a prior UI-key fixture timed
out and supplies no pass; separate verified native approval and sdk reads prove
the recipient failure.

the former fork/helper checks passed codex guarded peer/user input, persistent
queue receipts, exact interruption, gateway composition and reciprocal routing.
that evidence does not qualify the stock cutover; its experimental queue is
unavailable under [the current contract](../native-agent-observation.md).
successful claude socket writes proved only `written`, never admission.

current contract: helper/gateway/cli rejects claude peer/user/queue sends with
`unavailable/not_sent` and preserves native result identities.
[current qualification](../native-agent-qualification.md) owns source and
installation evidence; the helper comes directly from merged source, without patches.

resolved when: native recipient revalidation spans holding and approval, the
same switch/admission reproduction passes, and an explicit capability scope
change qualifies the adapter. restoring peer sends requires that native guarantee.
no terminal fallback, forced admission or local callback can establish that fact.
