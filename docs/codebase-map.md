# codebase map

tmux owns session state and process lifetimes. each gateway owns one host;
clients compose independent gateways. there is no application database.

the table describes current source. the
[attention owner](reply-notifications.md#owner-schemas-and-interfaces) persists
quiet/armed/ready and revision fences; the clients own freshness and local visits.

| slice | owner | boundary |
| --- | --- | --- |
| startup and host configuration | `cmd/skidbladnir`, `internal/hostconfig`, `internal/platform` | compose one host from deployment-owned configuration |
| sessions and metadata | `internal/sessions`, `internal/tmux`, `internal/group`, `internal/catalog` | inventory, creation, exact lifetime mutations, direct attachment; the terminal status domain type and focused observation (`Manager.ObservePane`: `tmux.Client.ObservePane`'s bounded screen regions, then the pane foreground re-observed against the session's own sample) |
| agent identity and control | `internal/agentruntime`, `internal/process`, `internal/agenthook`, `internal/agentcontrol` | foreground signatures and metadata; interrupt policy, separately from explicit native reads/controls; one observation sample per session for inventory, inspect and guarded send, and the pure screen classifier (`agentcontrol/detect.go` entry, `screen.go` shared row parsing, `codex.go`/`claude.go` grammars per [codex](terminal-observation-codex.md) and [claude](terminal-observation-claude.md)) |
| host resources | `internal/workdir`, `internal/pressure` | bounded directory browsing and native pressure observation |
| gateway transport and access | `internal/gateway`, `internal/auth`, `internal/pairing`, `internal/strictjson`, `internal/logging`, `internal/terminal` | authenticated http, strict messages, owned websocket/pty lifetime |
| desktop clients | `internal/fleetclient`, `internal/agentcli`, `internal/sessionui`, `internal/terminalclient` | shared peer routing and references; `fleetclient/status.go` owns typed status and queue category, alongside the strict request predicate for cli wait; `sessionui` owns one selected view, scoped machine inventory and browser presentation; local tty attachment stays in `terminalclient` |
| terminal attention | desktop `internal/fleetclient/notifications.go`, phone `NotificationStore.kt` | serialized v2 quiet/armed/ready persistence, scoped observation and visit operations; private transition reduction and committed-ready projection; clients keep freshness, read-sequence and visit gates; no native-history feed |
| phone fleet and dashboard | `MachineStore.kt`, `FleetPersistence.kt`, `FleetInvite.kt`, `GatewayClient.kt`, `SkidbladnirController.kt`, `SessionCard.kt`, `SessionActions.kt`, dashboard/forge/group/chooser files | encrypted pairings, reconciliation, selection and mutations; `TerminalControl.kt` status types and validity; `ProductModel.kt` `sessionStatusContent` owns typed queue membership and status; `Groups.kt` `dashboardItems` owns grouped all/group projections and the fleet-wide flat queue; `DashboardEntryState.kt` owns one selected view and schema-4 restoration; `DashboardMain` owns the scrolling view strip and one machines/details disclosure; `SessionCard.kt` the [session card](session-card.md); `SessionActions.kt` its ordered action list and overflow menu |
| phone polling and ordering | `Polling.kt` | coalesced reads, per-machine mutation fences and awaited inventory reads; the controller owns lane lifetimes |
| phone machine pressure | `Pressure.kt`, `PressurePresentation.kt`, `MachinePressureRail.kt` | strict pressure contract and state, dashboard visibility and content, rendered rail and details |
| phone terminal | `TerminalConnection.kt`, `LockedTerminalWebView.kt`, terminal composables, `assets/terminal` | transport, page protocol, input, selection and rendering |
| visual assets | theme/chrome/seal/ornament files, `catalog`, `scripts/gen-ornament` | shared presentation and generated artwork |
| build and operations | `scripts`, `.github/workflows`, android build files | engineering checks, release artifacts, installation and fleet operations; host installation belongs to `dev-server` |

phone source paths are relative to
`android/app/src/main/java/dev/niels/skidbladnir`; terminal assets are under
`android/app/src/main`.

cleanup proceeds one finding and one merged pr at a time: establish the current
contract and callers, demonstrate the finding, characterize important behavior
through its real boundary, simplify its owner, and independently review the
result. remove temporary tests before committing, as requested for this cleanup.
[testing policy](rules/testing.md) distinguishes that evidence from retained
engineering checks. unresolved findings belong in `docs/issues`, one per issue.
