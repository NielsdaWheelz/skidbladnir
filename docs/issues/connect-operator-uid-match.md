# Connect's operator check can refuse a permitted user

Problem: on Linux, `skid connect` refuses a non-root user who is not Tailscale's
operator and names `sudo tailscale set --operator=USER`. A `tailscaled` running
as that same user, such as a userspace-networking daemon, already lets the user
change what it serves, so connect refuses a setup Tailscale would accept.

Impact: rare; the user runs one unnecessary command, or cannot connect without
it.

Evidence: the precheck in `cmd/skidbladnir/fleet_cli.go` compares the
preferences' operator with the current user and does not ask which user runs the
daemon.

Resolved when: connect admits a user whose own `tailscaled` serves the host, or
the owner accepts the limit here.
