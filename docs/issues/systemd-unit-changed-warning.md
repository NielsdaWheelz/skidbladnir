# Linux install warns that the unit changed on disk

Problem: updating arch to v0.23.0 printed systemd's warning that
`skidbladnir.service` changed on disk and `systemctl --user daemon-reload` should
run, before the installer's own reload, enable and restart
(`internal/install/services.go`). The cause is not established.

Impact: cosmetic; the service restarted on the new release.

Evidence: arch, 2026-10-26, `./workstation gateway apply skidbladnir`.

Resolved when: the cause is known and an update prints no systemd warning, or the
owner accepts it here.
