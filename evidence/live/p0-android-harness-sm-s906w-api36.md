# P0 Android terminal harness

Recorded: 2026-08-25T05:03:14Z

Result: **PASS (automated harness only)**

## Artifacts

- Target: `SM-S906W`, Android API `36`, build `BP2A.250605.031.A3`
- App: `dev.niels.skidbladnir` `0.1.0` (`versionCode=1`)
- Android System WebView: `151.0.7922.85`
- Selected Gboard: `17.6.5.924672101-release-arm64-v8a`
- xterm.js: `6.0.0`
- Terminal entry SHA-256: `965b6ed9941867e855830064aac9df6d27db3c8a5efb8624ab99f415cf7311f5`
- Terminal adapter SHA-256: `dc5cf13375787dd14344afde66e0498242edd42c94e63e2c21658169937fa574`
- xterm.js SHA-256: `14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972`
- Manifest SHA-256: `043202c48f584340c4bfb41d60ea0a2a065072a2fb08425b5a002e81679bb31b`

## Procedure

`./scripts/test platform`

The pinned Android SDK installed the debug app and instrumentation APK on the
single connected target. Five instrumentation tests launched the real
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
  editable and did not submit. This is a JavaScript harness assertion, not a
  claim about physical Gboard interaction.
- The original run exposed three failures: two off-UI-thread WebView reads and
  one startup `null` race. After repair, focused tests and the full five-test
  platform gate passed on the target.

No device identifier, credential, prompt, clipboard value, terminal stream, or
user content is retained. Physical Gboard composition/dictation, Android
clipboard, IME resize, gesture and button navigation, 200% scale, TalkBack,
Switch Access, rotation, and process recreation remain explicitly `NOT_RUN`.
