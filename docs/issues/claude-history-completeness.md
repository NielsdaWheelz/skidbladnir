# claude history completeness is not established by the sdk reader

problem: the native-results contract rejects incomplete history, but the pinned
claude sdk reader silently skips malformed transcript entries and ends a chain
at a missing parent. its returned message list carries no completeness evidence.
the helper accepts a nonempty list and can return successful empty result ids
when only a surviving user message remains visible.

impact: incomplete baseline or recovery can appear complete. this is an
independent provider-boundary issue, not an established cause of the reported
persistent desktop reply marker.

evidence: `deployment/native-control/pin.json` names llm-calling `992e791` and
sdk `0.2.130`. that helper's `src/provider_runtime/agent_runtime/claude_control.py`
accepts history in `_messages` and enumerates qualifying groups in `results`.
the sdk's `_internal/sessions.py` skips json decode failures in
`_parse_transcript_entries` and stops `_build_conversation_chain` at a missing
parent. source inspection establishes the counterexample; no user history was
read during initial inspection. later temporary SDK/file/helper tests reproduced
the boundary: six intact/control characterization checks passed; five required
distinctions failed against SDK 0.2.130 and pinned helper 992e791. no user history,
model generation or terminal operation was used.

missing/corrupt parents and truncated successor tails still produced successful
helper results. ordinary input and compact summaries can project identically;
SDK conversion removes ancestry/compaction metadata before the helper sees it.
tool-result content remains distinguishable despite the shared user role.
reproduction uses an isolated CLAUDE_CONFIG_DIR with an intact saved chain, then
removes/corrupts a referenced parent or truncates a successor. compare public
get_session_messages and helper results with intact controls. metadata lookup
continues to succeed. native admission/current-work qualification remains NOT_RUN.

the user explicitly replaced strict new-reply notification semantics with
[terminal attention](../reply-notifications.md). that removes this prerequisite
from notification delivery; it does NOT repair explicit native results.
notification implementation changes no provider/helper source or pin.

resolved when: qualify the provider history boundary with completeness/error
evidence that distinguishes legitimate compaction from missing or malformed
history; refuse unsupported enumeration rather than report an empty success.
the provider/sdk and helper own this repair. alternatively, explicitly revise
the product guarantee to the weaker sdk-visible-history contract and state its
losses. retries, file-size heuristics and a second transcript parser do not
establish the missing guarantee.

live-provider qualification remains `NOT_RUN`. this record changes no capability or
acceptance requirement and authorizes no edits to the external helper.
