# The demo media show old desktop and phone status

Problem: `site/assets/continuity.mp4`, its poster and `desktop-browser.png` were
recorded on 2026-10-26, before the desktop browser's redesign shipped in v0.25.0
(2026-11-17). The site and the README show a browser that no longer exists.
`phone-dashboard.png` shows a coloured shape and status words in place of the
app's sign.

Impact: a reader sees out-of-date interfaces. The other phone stills are
current.

Evidence: the redesign's contract in [architecture §11](../architecture.md); the
media's date in [site/assets](../../site/assets/README.md). The owner deferred a
re-shoot on 2026-11-20 while the interface keeps changing.

Resolved when: the media are re-recorded with [scripts/media](../../scripts/media/README.md),
whose first live run is `NOT_RUN`, and the owner approves them. With the tmux
status line turned off for the demo terminal, `site/assets/README.md` drops its
note about painting out the host's name. The owner uploads the new video as a
GitHub attachment, which has no API, so that the README plays it inline instead
of linking `site/assets/continuity.mp4`.
