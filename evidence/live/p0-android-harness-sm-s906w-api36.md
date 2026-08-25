# P0 Android terminal harness

Recorded: 2026-08-25T07:13:06Z

Result: **PASS (automated harness only)**

## Artifacts

- Target: `SM-S906W`, Android API `36`, build `BP2A.250605.031.A3`
- App: `dev.niels.skidbladnir` `0.1.0` (`versionCode=1`)
- Android System WebView: `151.0.7922.85`
- Selected Gboard: `17.6.5.924672101-release-arm64-v8a`
- xterm.js: `6.0.0`
- Terminal entry SHA-256: `aedc7fc0088fc4ea42414af70d260debf4c601eef028fd74ee292f1d9c6c3439`
- Terminal adapter SHA-256: `2f1e7624570338159ad9178b318db722fc3ec5dcb2bd61d2c344a835d2366ecf`
- xterm.js SHA-256: `14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972`
- Manifest SHA-256: `5d697bf7473a2c06f7c0267d03ea955817dc3a88cf35f78d0f5a3c327df91d19`

## Procedure

`./scripts/test platform`

The pinned Android SDK installed the debug app and instrumentation APK on the
single connected target. Seven instrumentation tests launched the real
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
- Programmatic composition, paste sanitization, and dictation fixtures remained
  editable and did not submit. The harness exposed that draft visibly, handled
  Unicode-safe backspace, and reported the live visual viewport. These are
  WebView/xterm harness assertions, not claims about physical Gboard
  interaction.
- The original run exposed three failures: two off-UI-thread WebView reads and
  one startup `null` race. After repair, the focused tests and the full
  seven-test platform gate passed on the target. This record reports automated
  harness coverage only; it does not claim manual rotation confirmation.

No device identifier, credential, prompt, clipboard value, terminal stream, or
user content is retained. Physical Gboard composition/dictation, Android
clipboard, IME resize, gesture and button navigation, 200% scale, TalkBack,
Switch Access, rotation, and process recreation remain explicitly `NOT_RUN`.
