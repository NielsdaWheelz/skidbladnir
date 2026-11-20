# The hub's own report goes stale after an update

Problem: after an update, `skid status` lists the hub at its previous version
until something re-reports it. Members re-report on every reconnection
(`internal/fleet/member.go`), but the hub records its own report only when its
origin is set (`setOrigin`, run by `skid connect`); `skid apply` does not.

Impact: `skid status` misstates the hub's version after every release; the
fleet works.

Evidence: 2026-10-26, after v0.23.0 reached devbox, its `skid status` showed the
hub at v0.22.0 with service v0.23.0, also after `skid apply`; `skid connect` on
devbox corrected it.

on 2026-11-20 utc, the devbox gateway reported v0.27.0 directly while the hub
roster still showed its own v0.23.0. macbook and arch roster entries reported
v0.27.0. all three quota and worker inventory reads passed.

Resolved when: the hub records its own report at startup, and `skid status`
shows the running version right after an update.
