# frozen terminal input is not announced

problem: when a terminal connection becomes `ReconnectRequired`, the rail's
presence text changes to `input frozen` in failure colour, but no live region
reports the change. talkback users learn of it only by exploring the rail or
the frozen overlay.

impact: a screen-reader user may not notice that the terminal stopped accepting
input. no input is replayed, so the failure is confusion, not corruption.

evidence: `terminalPresence` in `android/app/src/main/java/dev/niels/skidbladnir/TerminalScreen.kt`
maps `ReconnectRequired` to `input frozen`; the rail's `clearAndSetSemantics`
sets a content description without `liveRegion`, and the file uses none.
first noted on the api-36 emulator during the v0.13.0 chrome review; the
physical phone was not used.

resolved when: the transition to frozen is announced once by talkback on the
s22+, and returning to connected does not repeat stale announcements.
