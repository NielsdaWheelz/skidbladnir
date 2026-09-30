# current-work notification evidence needs qualification

problem: the existing result-id enumeration and independent status sample cannot
prove whether a saved reply belongs to the newest admitted work. the pinned
claude listing exposes activity without a correlated work id; its sdk loses
boundary/completeness metadata. codex has native turn ids, but newest admitted
turn visibility and finalized-item races still need qualification.

impact: clearing known ids can allow a delayed old reply to notify again. blindly
consuming the next history read can instead discard a genuinely new answer.
this blocks claiming the strict [notification contract](../reply-notifications.md),
not ordinary terminal use or orchestration reads.

evidence: read-only review of helper pin `992e791` and sdk `0.2.130`. both existing
results adapters enumerate historical finalized output independently of newer
work. claude grouping ignores intervening input; a sdk `user` row alone cannot
distinguish prompts, tool results and compact summaries. the spec defines the
replacement head contract and its temporary qualification counterexamples.

resolved when: both providers supply the specified native work order, current
head, finality and absence/error distinctions; unseen a / admitted b / delayed
history, completion races and rewind cases pass at the actual provider boundary.
repair missing evidence at its provider/sdk owner; no timestamp ordering, second
transcript parser, lifecycle ledger or silent weaker fallback.

qualification is `NOT_RUN`. this record authorizes no provider, tmux or device
operations and does not claim that the missing claude capability exists.
