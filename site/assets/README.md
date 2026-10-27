# Assets

Each asset is derived from a frozen source in the app. The artwork is the project's own, under the repository's Apache-2.0 license; the fonts are under the SIL Open Font License 1.1.

- `ship-mark.svg`: the ship under sail in Gold, Bronze and Garnet; the eight paths of `android/app/src/main/res/drawable/ic_launcher_foreground.xml`, cropped to the drawn content; Apache-2.0.
- `ship-mark-mono.svg`: the ship in `currentColor`; the five paths of `ic_launcher_monochrome.xml`, where the shield row is cut as notches in the sheer; Apache-2.0.
- `valknut.svg`: the Hlíðskjálf mark; the 12 `Valknut` segments of `Ornament.kt` in a unit box, stroke 0.055 of the size, as `HlidskjalfMark` draws them; Apache-2.0.
- `fret-band.svg`: one tile of the fret band, Gold at 40%, with its neighbours' strokes so it repeats horizontally without a seam; `FretCell` and `drawFretBand` in `Ornament.kt`; Apache-2.0.
- `seals/<key>.svg`: six dwarf seals for catalogue keys, from `sealSpec` and `DwarfPortrait` in `Ornament.kt`; the initial is a Big Shoulders glyph outline (SIL OFL 1.1); otherwise Apache-2.0.
- `social-card.png`: the 1280×640 link preview, with the ship mark, the wordmark in Big Shoulders and the canonical line's first sentence in JetBrains Mono (both SIL OFL 1.1); Apache-2.0.

The media are recordings of the product, staged with demo sessions, fake
repositories and stub agents that paint authored frames
(`internal/agentcontrol/testdata/frames`); no real terminal content. The
computer's hostname in the tmux status lines is painted over in the bar's own
color, in every frame of the video, its poster and `phone-terminal.png`.

- `continuity.mp4`: 31 seconds, side by side. The desktop session browser opens a
  terminal and runs `git log`, then detaches; the phone opens the same terminal,
  sees it, runs `ls` and detaches; the desktop returns, sees the `ls` and runs
  `git status`. Desktop recorded with VHS, phone with scrcpy.
- `continuity.png`: its poster frame.
- `desktop-browser.png`: the session browser on one group.
- `phone-dashboard.png`, `phone-terminal.png`, `phone-forge.png`: the phone's
  dashboard, a terminal, and the sheet that creates a session.
