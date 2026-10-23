# serve's launchd priority on the macbook

problem: a launchd job without `ProcessType` runs serve at priority 20, and the
tmux server and every pane shell and agent inherit it, while login shells run at
31. under cpu contention the terminal relay, tmux and every agent lose scheduling
to default-priority work. linux is unaffected.

status: since v0.20.1 the plist declares `ProcessType` `Interactive`
(`internal/install/services.go:53-54`), and the installer fails when serve runs
below 31 or no new serve appears within 5 s of bootstrap (`:155-174`). dev-server
pins v0.20.1 (dev-server `ad079ef`). only a release's own installer writes its
plist: an older binary's `skid install` or `skid update` writes the old plist and
skips the check. a running tmux server keeps its priority until it restarts.

maintenance window (dev-server macbook-capacity d6), once:

1. install a release of v0.20.1 or later on the macbook through dev-server's pinned
   path, `./workstation gateway apply skidbladnir`, which runs the reviewed
   bootstrap; launchd boots serve out and in again, and `AbandonProcessGroup`
   keeps tmux and the agents running.
2. verify that serve runs at priority 31.
3. stop agent work, then run `tmux kill-server`. serve's recovery rebuilds the
   workspace as fresh shells in a new server it starts itself
   ([workspace recovery](../session-recovery.md)); conversations resume by hand.
4. verify that the new tmux server and a pane shell run at priority 31.

resolved when: the macbook runs a release of v0.20.1 or later installed by its
own installer (dev-server apply or `bash install TAG`), the priority check passes,
and after the maintenance window's tmux restart, serve, the new tmux server and a
new pane shell all report priority 31.
