# native terminal controls omit their conversation id

problem: mobile's terminal menu offers native stop for the recorded conversation
without displaying that conversation's id beside the action. terminal navigation
can display another conversation while native controls retain the original target.

impact: exact backend targeting does not alone establish informed user intent.
the card's tracking label supplies identity before attachment, but terminal
controls do not repeat it where the action occurs.

evidence: source review of `TerminalScreen.kt` and the controller's `stopAgent`.
the prior manual-association form, provider-prefill, eligibility and recovery
problems are resolved by removing manual linking entirely. codex association is
no longer written by creation or manual actions; existing recorded bindings
remain. this remaining disclosure issue is independent.

resolved when: terminal native actions identify their captured conversation and
remain visibly distinct from terminal effects, with narrow-layout and accessibility
verification. preserve existing bindings and exact native targets; no manual
linking, inference or new identity owner. source review is not phone acceptance.
live/provider/device verification remains `NOT_RUN`.
