# codebase map

tmux owns session state and process lifetimes. each gateway owns one host;
clients compose independent gateways. there is no application database. every
private state file (fleet, machine handle, notification observer, workspace
checkpoint, terminal contexts) is written through `internal/privatefile`, the
only package that syncs a directory.

the table describes current source. the hub's notifier persists attention
records and the owner's acknowledgements; clients reconcile native notices against
its state and keep no notification memory.

| slice | owner | boundary |
| --- | --- | --- |
| startup and host configuration | `cmd/skidbladnir`, `internal/hostconfig` | compose one local host from accepted yaml and the hub's fleet state |
| sessions and metadata | `internal/sessions`, `internal/tmux`, `internal/group`, `internal/catalog` | inventory, creation, exact lifetime mutations, direct attachment: one tmux read module (`tmux/inventory.go`: every session or one id per bracketed script), guarded single-session effects (`tmux/guard.go`), client-token admission (`sessions/identity.go`), creation (`sessions/create.go`); the terminal status domain type and focused observation (`Manager.ObservePane`: `tmux.Client.ObservePane`'s bounded screen regions, then the pane foreground re-observed against the session's own sample) |
| workspace recovery | `sessions/recovery.go`, `recovery_store.go`; `tmux/recovery.go`; gateway lifetime | exclusive private recipe, loss decision and checkpoints; native graph capture/reconstruction; required passive inventory fact. [checkpoint reliability](workspace-checkpoint-reliability.md) keeps retry/admission policy in sessions, transition diagnostics in gateway/logging and strict notice projection in existing clients; source implemented and qualified. dev-server owns retirement of old plugin entry points |
| agent observation and control | `internal/agentruntime`, `internal/process`, `internal/agentcontrol` | foreground provider/account identity and terminal controls; one observation sample per session for inventory, terminal inspect and guarded send; pure screen classifier (`detect.go`, `screen.go`, `codex.go`/`claude.go`) |
| account quota | `internal/quota` | read-only account quota: one observe-only app-server exchange per codex profile under one deadline, and the usage wire type |
| host resources | `internal/workdir`, `internal/pressure` | bounded directory browsing and native pressure observation; each owns its wire json |
| gateway transport and access | `internal/gateway`, `internal/auth`, `internal/fleet`, `internal/strictjson`, `internal/logging`, `internal/terminal` | authenticated http through one `ServeMux` route table (`gateway.go`), strict messages, owned websocket/pty lifetime, live connections and revocation (`terminal.go`); content-free event lines (`internal/logging`: one slog json handler per serve process; darwin `$XDG_STATE_HOME/skidbladnir/serve.log`, 8 MiB plus `serve.log.1`, timed lines; linux stdout to the journal) |
| desktop clients | `internal/fleetclient`, `internal/agentcli`, `internal/sessionui`, `internal/terminalclient` | shared peer routing and references; `fleetclient/status.go` owns typed status and queue category, alongside the strict request predicate for cli wait; `sessionui` owns one selected view, scoped machine inventory and browser presentation; local tty attachment stays in `terminalclient` |
| notifications | hub `internal/notifier` (observation, acknowledgement, projection, fcm receivers); contract `internal/attention`; clients `fleetclient/notifications.go`, `internal/macnotifications` (mac app), phone `NotificationOwner.kt`, `NotificationState.kt`, `NativeNotifications.kt`, push files | one durable owner on the hub; clients stateless against the os, one in-memory alert map per process |
| phone fleet and dashboard | `MachineStore.kt`, `FleetClient.kt`, `FleetInvite.kt`, `GatewayClient.kt`, `SkidbladnirController.kt`, `SessionCard.kt`, `ActionMenu.kt`, dashboard/forge/group/chooser files | encrypted device identity, hub routing, reconciliation, selection and mutations; `TerminalControl.kt` status types and validity; `ProductModel.kt` `sessionStatusContent` owns typed queue membership and status; `Groups.kt` `dashboardItems` owns grouped all, flat selected groups and the fleet-wide flat queue; `DashboardEntryState.kt` owns one selected view and schema-4 restoration; `DashboardMain` owns the scrolling view strip and one machines/details disclosure; `SessionCard.kt` the [session card](session-card.md); `ActionMenu.kt` shared action rows/overflow and the session lifetime-action list |
| phone polling and ordering | `Polling.kt` | coalesced reads, per-machine mutation fences and awaited inventory reads; the controller owns lane lifetimes |
| phone machine pressure | `Pressure.kt`, `PressurePresentation.kt`, `MachinePressureRail.kt` | strict pressure contract and state, dashboard visibility and content, rendered rail and details |
| phone terminal | `TerminalConnection.kt`, `LockedTerminalWebView.kt`, terminal composables, `assets/terminal` | transport, page protocol, input, selection and rendering |
| visual assets | `Theme.kt` (tokens, material theme, notice panel, modal back), seal/ornament files, `internal/catalog` (embedded; [curation](dvergatal.md)) | shared presentation and frozen artwork |
| build and operations | `scripts`, `.github/workflows`, android build files | engineering checks, release artifacts, installation and fleet operations; `internal/install` owns standalone installation; dev-server is an optional consumer |

phone source paths are relative to
`android/app/src/main/java/dev/niels/skidbladnir`; terminal assets are under
`android/app/src/main`.

cleanup proceeds one finding and one merged pr at a time: establish the current
contract and callers, demonstrate the finding, characterize important behavior
through its real boundary, simplify its owner, and independently review the
result. remove temporary tests before committing, as requested for this cleanup.
[testing policy](rules.md#testing) distinguishes that evidence from retained
engineering checks. unresolved findings belong in `docs/issues`, one per issue.
