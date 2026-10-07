# workspace recovery after actual host reboot

problem: linux user-service and mac login-agent recovery have not been qualified
after an actual host reboot. impact: source/server-loss evidence does not prove
os startup, persistent-file survival or changed-boot composition.

evidence, 2026-10-07: isolated native graph reconstruction and gateway policy pass
on both platforms. test units rendered from the existing service/plist templates
served current gateway composition before reboot; they used fixture config and
isolated sockets. the owner deferred both reboots to a maintenance window.
all test units, sockets, packages and private files were removed. production
deployment remains separate. [qualification](../session-recovery.md#qualification)
records the completed boundaries.

resolution: obtain current approval for actual reboots and isolated tmux use.
install current-source test owners from the existing linux service/mac plist
templates with private checkpoint paths and isolated sockets. save two sessions
and three panes with distinct cwd, names, dwarfs and group metadata. reboot the
linux kernel; reboot/login on mac. prove changed boot identity, automatic complete
reconstruction, fresh server/session authority and shells only without client
requests or manual startup. a gateway/container restart is insufficient.
remove all temporary owners/tests, record source and actual boundaries, then
delete this issue. stop before reboot if unrelated running work is not cleared.
