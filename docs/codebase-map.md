# codebase map

tmux owns session state and process lifetimes. each gateway owns one host;
clients compose independent gateways. there is no application database.

| slice | owner | boundary |
| --- | --- | --- |
| startup and host configuration | `cmd/skidbladnir`, `internal/hostconfig`, `internal/platform` | compose one host from deployment-owned configuration |
| sessions and metadata | `internal/sessions`, `internal/tmux`, `internal/space`, `internal/catalog` | inventory, creation, exact lifetime mutations, direct attachment |
| agent identity and control | `internal/agentruntime`, `internal/process`, `internal/agenthook`, `internal/agentcontrol` | observe one foreground process; bind reads and controls to that lifetime |
| host resources | `internal/workdir`, `internal/pressure` | bounded directory browsing and native pressure observation |
| gateway transport and access | `internal/gateway`, `internal/auth`, `internal/pairing`, `internal/strictjson`, `internal/logging`, `internal/terminal` | authenticated http, strict messages, owned websocket/pty lifetime |
| desktop clients | `internal/fleetclient`, `internal/agentcli`, `internal/sessionui`, `internal/terminalclient` | shared peer routing and references; cli, browser, local tty |
| phone fleet and dashboard | `MachineStore.kt`, `FleetPersistence.kt`, `FleetInvite.kt`, `GatewayClient.kt`, `SkidbladnirController.kt`, dashboard/forge/space/chooser files | encrypted pairings, reconciliation, selection and mutations |
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
