# rename crashes on an error code outside its route

problem: `SessionRename.kt` `completeRenameHttp` answers every `ApiErrorCode`
outside the rename route (`ProfileUnknown`, `TerminalUnavailable`, …) with
`error("rename received an error outside its closed route")`. the per-route
error decoders that used to throw first are gone: `decodeGatewayHttpFailure` is
total and can deliver any known code to this completion.

impact: a same-release gateway answering rename with a code the phone did not
expect crashes the app instead of showing the outcome as unknown. docs/rules.md
says peer data never panics.

evidence: android-core cleanup (one total error decoder, design §11.1);
`completeRenameHttp`'s last `when` arm at the base commit 3ef8c2e.

resolved when: that arm becomes
`else -> renameNeedsInventory(state, gatewayFailureMessage(failure))` (owner:
android-ui-structure), and a JVM test feeds a foreign code without a throw.
