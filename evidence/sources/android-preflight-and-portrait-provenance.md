# Android preflight and portrait provenance design note

This note defines the host-readable evidence inputs for the P0/P5 gates. It
does not claim that a target phone or portrait bundle exists.

## Target-device preflight

The result shape is `android-target-preflight.v1`, emitted by the Android
preflight collector as deterministic JSON. It contains only:

- target model/API (`SM-S906W`/`36`);
- observed build id, WebView package/version, selected IME package, and whether
  the Tailscale client is installed; and
- ordered checks with `PASS`, `FAIL`, or `NOT_RUN` plus a short reason.

It must not contain serials, account data, bearers, prompts, terminal bytes, or
user content. A missing device emits `overall=NOT_RUN`; it is never converted
to a pass or a synthetic device result.

Package presence is only a prerequisite check. The interactive checks remain
`NOT_RUN` until a human runs the real app on the target S22+: ANSI/UTF-8,
Gboard composition, editable dictation, clipboard paste, IME resize, gesture
and button navigation, 200% scale, TalkBack, Switch Access, rotation, and
process recreation. The result must be retained with the exact installed
build and target model, not a host emulator substitute.

## Portrait production manifest

P5 should add a repository-owned `portrait-manifest.v1` beside the bundled
WebP assets. Each catalogue key has exactly one record with:

```text
key
assetPath
sha256
width
height
format
production.method
production.artistOrTool
production.version
production.createdAt
production.prompt (only when applicable)
rights.basis
rights.assignment
```

Static validation must prove that every catalogue key has one record and one
square WebP at the documented resource path, every recorded digest matches the
committed bytes, and no asset or manifest record is orphaned. Production and
rights fields must be non-empty; source-derived text may inform a prompt or
brief but third-party artwork, scraped images, book/film/stage depictions, and
unassigned rights are not admissible. The manifest is build evidence and is
outside the API protocol digest.
