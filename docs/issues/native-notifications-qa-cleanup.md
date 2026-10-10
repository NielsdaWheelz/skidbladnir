# ancillary notification qa cleanup

problem: runtime qa resources are retired, but two external cleanup boundaries
remain unverified.

impact: ntfy may retain its qa subscription/account after app uninstall; three
logged-out test nodes may remain in the tailnet's device list.

evidence: exact-owned services, isolated tmux servers, stage directories, mac
homes/jobs and phone qa packages are removed; protected production values match.
the phone disconnected before distributor ui inspection. browser automation has
no available browser. ntfy 1.25.2 requires explicit unregister or per-subscription
deletion; uninstall alone does not remove its subscription.

resolution: reconnect/unlock the admitted phone; in ntfy, verify the qa package
and full qa origin before deleting only that subscription and its qa account.
preserve other subscriptions, users and the restored default server. remove only
the logged-out tailnet devices named `skid-notify-qa-1df2958d6a72-{arch,devbox,macbook}`
through the tailnet admin's normal device interface. verify absence and delete
this record. private final-cleanup context remains until that phone step finishes.
