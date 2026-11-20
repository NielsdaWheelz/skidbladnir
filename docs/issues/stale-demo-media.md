# The demo media show the old desktop browser

Problem: `site/assets/continuity.mp4`, its poster and `desktop-browser.png` were
recorded on 2026-10-26, before the desktop browser's redesign shipped in v0.25.0
(2026-11-17). The site and the README show a browser that no longer exists.

Impact: a reader sees an out-of-date interface; the phone stills are still
current.

Evidence: the redesign's contract in [architecture §11](../architecture.md); the
media's date in [site/assets](../../site/assets/README.md). The owner deferred a
re-shoot on 2026-11-20 while the interface keeps changing.

Resolved when: the media are re-recorded with [scripts/media](../../scripts/media/README.md),
whose first live run is `NOT_RUN`, and the owner approves them. With the tmux
status line turned off for the demo terminal, `site/assets/README.md` drops its
note about painting out the host's name.
