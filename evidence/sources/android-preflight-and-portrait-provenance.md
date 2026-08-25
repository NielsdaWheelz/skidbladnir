# Android preflight and portrait provenance design note

This note defines the host-readable evidence inputs for the P0/P5 gates. The
current automated target-harness result is recorded separately in
`evidence/live/p0-android-harness-sm-s906w-api36.md`; this note does not claim
that the interactive phone checks or portrait bundle exist.

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

P0 freezes the set-level style, source-safety, and rights method in
`dvergatal.md`; P5 creates the repository-owned `portrait-manifest.v1` beside
the bundled WebP assets. This note does not claim that the P5 manifest or
portrait assets exist. Each catalogue key has exactly one record with:

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

The production brief is one original single-figure, head-and-shoulders style
with a neutral background, common lighting/detail treatment, no text/logo/
watermark/border, and a minimum 256×256 square canvas. The cited work and
locus may justify a name, epithet, role, or restrained source-safe visual cue;
unsupported modern likenesses and borrowed actor, illustrator, film, game, or
stage designs are forbidden. Each portrait must remain distinguishable at
64×64 and 200% scale in grayscale as well as color; hue alone cannot carry the
distinction, and the card's accessible display name remains the label.

Static validation must prove that every catalogue key has one record and one
square WebP at the documented resource path, every recorded digest matches the
committed bytes, and no asset or manifest record is orphaned. A P5 review must
also identify each portrait's source-safe distinguishing cue and reject
near-duplicates at 64×64; this is a review criterion, not an additional
manifest field. Production and rights fields must be non-empty; source-derived
text may inform a prompt or brief but third-party artwork, scraped images,
book/film/stage depictions, and unassigned rights are not admissible. A failed
thumbnail, provenance, digest, or rights check is red, not a placeholder or
silent portrait reuse. The manifest is build evidence and is outside the API
protocol digest.
