# P0 Android terminal harness

Recorded: 2026-08-25T18:30:46Z

Result: **PASS (exact P0 terminal claim)**

## Artifacts

- Target: `SM-S906W`, Android API `36`, build `BP2A.250605.031.A3`
- App: `dev.niels.skidbladnir` `0.1.0` (`versionCode=1`)
- Android System WebView: `151.0.7922.170`
- Selected Gboard: `17.6.5.924672101-release-arm64-v8a`
- xterm.js: `6.0.0`
- xterm FitAddon: `0.11.0`
- Terminal entry SHA-256: `084d3b3c7274e10242c02d4c54dac498e2d02b09f528ad88cd86d5c0e7036656`
- Terminal CSS SHA-256: `5afbdd6dcb277edf2bc3e19a702b6125cd1602dcac805774ca12afd080175d39`
- Terminal adapter SHA-256: `19ffee4f9d84b2170efcdaccfad81fec1e78a951026b9e54af33dec682db4120`
- xterm.js SHA-256: `14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972`
- xterm FitAddon SHA-256: `ba3ea256ce0620a0992a197d6c9baea64823fc93d8da07a9e366ca9943c18527`
- Manifest SHA-256: `bccaf739e0a9b608dfa758b7fce3ebef3e1c33d0f592919d27bf55f40cef68f8`
- Instrumentation SHA-256: `f75c2f0789bf9c9649dd2622e29d625de3b5c55a292c5b7f903dbb9ea5713aaa`
- Gradle properties SHA-256: `5964c5f1e3be0a7690eee5b8d2207b0dcc5d2f5eb3b04f7d032c1f813998226d`
- Debug APK SHA-256: `1244f6f18b647ad2ea6638f63a5761c6fb504ebbfc86999d15c1594612eb7b3f`
- Instrumentation APK SHA-256: `5d775d00231a3e88c834a80f94d7da9fb387c8db23638f7db75d1c591d688f69`

## Procedure

`./scripts/test platform`

The repository-bounded Gradle process packaged and installed the exact app and
instrumentation APKs on the single connected target. Nine tests launched the real
`MainActivity`, loaded the packaged xterm runtime in its locked WebView, and
collected the content-free `android-target-preflight.v1` report. The manifest
declared only the package-visibility query required to verify the installed
Tailscale client.

## Observed result

- Device model/API, WebView runtime, selected Gboard, and visible Tailscale
  package checks were `PASS`; the report's overall status remained `NOT_RUN`
  because interactive checks were not executed.
- The asset-only WebView loaded with network, file, and content access blocked,
  the exact CSP present, no JavaScript interface, and a WebMessagePort bridge.
- The real xterm helper textarea existed with screen-reader mode enabled. The
  harness rendered its ANSI/Unicode fixture, resized to 80×24, and emitted the
  pinned DA1, DA2, DSR, and CPR automatic-reply classes.
- The explicit typing control focused the real xterm helper textarea.
  Programmatic composition stayed visible before commit, committed exactly once
  across synchronous, delayed, and both overlapping callback orderings, and
  cleared on cancellation. The IME fallback excluded pre-existing hidden
  textarea content. Paste sanitization and dictation fixtures remained editable
  and did not submit.
- FitAddon refit the actual terminal content box. With the real Gboard window
  visible, a portrait/landscape round trip retained the exact visual-viewport
  scale, restored the original width and terminal cell geometry, avoided page
  overflow, retained the Unicode draft, and accepted another edit. These are
  device/WebView assertions, not claims about physical Gboard typing.
- A securely locked earlier attempt returned `NOT_RUN` before device execution.
  The exact current nine-test public platform gate passed on the unlocked target.
  Earlier 2 GiB Gradle attempts were killed under concurrent host pressure; the
  reviewed 768 MiB in-process compiler configuration completed static lint and
  platform gates. The automated harness covers the terminal mechanism,
  including editable multiline input; the physical observations below close
  the exact P0 Gboard-input claim. They do not replace later Core accessibility
  checks.

## Partial physical observation

At `2026-08-25T16:54:41Z`, the operator confirmed on the target phone that the
explicit typing control was discoverable, a physical Gboard composed character
appeared in the editable draft before rotation, a portrait/landscape
round trip preserved the draft at a stable scale, and editing could continue
after returning to portrait. No entered text or other user content was retained.

At `2026-08-25T16:59:48Z`, the operator confirmed that physical Gboard
dictation remained editable in the middle of the draft, Android clipboard text
could be pasted and edited, and neither path auto-submitted. No dictated or
pasted content was retained.

No device identifier, credential, prompt, clipboard value, terminal stream, or
user content is retained. Gesture and button navigation, 200% scale, TalkBack,
Switch Access, and process recreation remain explicitly `NOT_RUN` for later
Core acceptance; they are outside the exact `p0-android-terminal` claim.
