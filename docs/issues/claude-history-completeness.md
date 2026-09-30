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
read and no runtime reproduction was performed.

resolved when: qualify the provider history boundary with completeness/error
evidence that distinguishes legitimate compaction from missing or malformed
history; refuse unsupported enumeration rather than report an empty success.
the provider/sdk and helper own this repair. alternatively, explicitly revise
the product guarantee to the weaker sdk-visible-history contract and state its
losses. retries, file-size heuristics and a second transcript parser do not
establish the missing guarantee.

runtime qualification remains `NOT_RUN`. this record changes no capability or
acceptance requirement and authorizes no edits to the external helper.
