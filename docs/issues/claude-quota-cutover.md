# claude quota fleet cutover

the [quota contract](../architecture.md#account-quota-and-usage) changes both
usage routes and starts a new private `state/claude-quota/` tree. old skid-only
history is discarded at cutover. a reading held only there can stay absent
until claude supplies it again. provider files remain in place.

with owner approval, install the same release on all enrolled gateways and
desktop clients before checking fleet usage. the phone and jarvis use neither
quota route. the managed dev-server formatter still calls the same
`skid claude-usage` entry with the same output.

resolved when the installed desktop requests usage from every reachable
gateway, each serves the new array, and the footer reads from the new tree.
check that a stale eligible native cache advances and a second gateway's
reading survives source loss. builds without native qualification still
contribute their existing caches. rollback restores the previous release
lockstep. each release reads its own quota tree.
