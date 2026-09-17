# terminal frame contracts outside their owner

problem: `ProductModel.kt` owns terminal events, wire payloads, decoding,
geometry bounds and outbound frame encoding, while `TerminalConnection.kt`
owns every codec call. the webview also consumes the shared geometry bounds.

resolved when: move the frame contract into the existing connection file,
preserving APIs and behavior. after the pressure extraction, move the remaining
shared JSON string/key helpers to `StrictJson.kt` and keep terminal-only numeric
admission with the codec. characterize actual compiled inbound/outbound frames,
strict fields, errors, duplicate keys and geometry bounds. this proves the
codec boundary, not websocket or device lifecycle behavior.
