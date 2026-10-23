# terminal failures render the phone's copy table

problem: a terminal failure shows the phone's own `apiErrorMessage(code)` copy,
not the gateway's. rules.md says clients display host error copy and keep no
parallel copy tables.

impact: a gateway wording change shows stale phone copy, and a code this
release does not know shows the phone's `InternalError` copy instead of the
gateway's message. until terminal-trust, `TerminalConnection` failed a frame
whose message differed from the phone's table, which at least kept the two in
sync; that check is gone.

evidence: `TerminalConnection.kt` decodes only `error.code` from an `Error`
frame, and a refused upgrade keeps only the code of
`decodeGatewayHttpFailure`'s `GatewayFailure.Api`;
`TerminalConnectionObserver.onFailure(code: ApiErrorCode)` carries no message;
`SkidbladnirController.kt`'s terminal observer builds
`GatewayFailure.Api(code, apiErrorMessage(code))` and shows
`apiErrorMessage(code)`.

resolved when: `onFailure` carries the gateway's `GatewayFailure.Api` (the
frame's `error.message`, or the upgrade's envelope), and the controller renders
its message; phone-made copy remains only for the phone's own failures
(`ReconnectRequired` on a lost connection or page).
