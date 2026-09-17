# unreachable dashboard quarantine presentation

problem: the dashboard carries and renders unreadable machine slots, but storage
admits either the whole fixed fleet or no credentials. unreadable storage sends
the controller to the reset screen before any dashboard is published.

impact: dormant state and rendering obscure the active flow; the required
distinction between an unreadable collection index and unreadable entries never
reaches that reset screen.

evidence: `MachineStore.readLocked`, the storage result branch in controller
`start`, and `FleetConnectScreen`'s generic reset content establish the path.

resolved when: show the distinction on the active reset surface and delete the
unreachable dashboard state, strip, empty-state branch, and storage notice.
characterize store-result admission through reset presentation; preserve
credential quarantine and rollback behavior.
