# Skíðblaðnir architecture

Skíðblaðnir (`skid`) lets one owner work in the tmux sessions of their computers
from any of those computers or from an Android phone. tmux owns sessions and pane
processes; providers (Codex, Claude) own execution, accounts and history. Each
computer's gateway controls only its own tmux, and clients call every gateway
directly. The hub, the computer that created the fleet or last ran `skid
recover`, owns membership, accepted configuration and attention; it never carries
terminal traffic.

Calibration: one owner, three computers and one phone; a single-user prototype
under live repair. The simplest complete design for that wins. Exact targets and
at-most-once effects are not negotiable; availability and polish are.

This document is the contract: concepts, invariants, interfaces, owners, error
semantics, topology and trust. Most mechanisms below follow from five constraints:

1. tmux is the store and the process supervisor. skid keeps no session database,
   its private files (fleet, handle, observer, workspace recipe, terminal
   contexts, the phone's text size and saved-state capsule) never stand in for
   inventory, and a restart re-lists tmux ([§6](#6-host-sessions-and-tmux)).
2. Every effect is guarded in tmux on an exact lifetime and attempted at most
   once. Lost completion is `unknown`, never replayed, rolled back or
   compensated, by the host or any client ([host](#host-invariants),
   [desktop](#desktop-invariants), [phone](#phone-invariants)).
3. Execution is host-local. No gateway acts for another, no computer relays
   terminal bytes, and one machine's failure fences only its own rows and
   actions; the hub is a dependency of edits, enrolment and attention only
   ([§3](#3-topology-and-trust)).
4. Status, attention, usage and remote context are sampled inference: evidence
   for a human, never authority for an effect, which re-reads what it needs
   ([§7](#7-observation-and-agent-control)).
5. One owner, one trust domain: the tailnet plus one bearer per device
   ([trust](#trust-invariants)).

Specs keep what the code cannot show: [terminal observation](terminal-observation.md)
with its [Codex](terminal-observation-codex.md) and
[Claude](terminal-observation-claude.md) grammars, [workspace recovery](session-recovery.md),
the [tmux exec budget](tmux-exec-budget.md),
[agent delegation](agent-delegation.md), and the [design language](design-language.md). [Rules](rules.md) bind implementation, the
[codebase map](codebase-map.md) locates code, [issues](issues) hold open problems
one per file, and the [roadmap](roadmap.md) holds what is next.

## 1. Scope

From a desktop browser, the CLI or the phone the owner can: see every tmux session
of every enrolled computer in one collection, with its machine, its foreground
provider and account when recognized, and its inferred status; create a terminal,
or a configured profile's agent, on an explicit machine and directory, or a
terminal beside an existing session; attach the same tmux client view the laptop
sees; type, paste and dictate through Gboard; select and copy terminal text on the
phone; detach without stopping anything; interrupt or close an exact, confirmed
session; read a session's screen in the browser and read, write and wait on terminals
from the CLI; exchange addressed peer messages; submit a profile-backed
assignment through the paired gateway and receive its native result in jarvis;
choose native model/effort options for a supported profile launch;
be notified when an agent needs input or is ready again; and rebuild a
lost tmux workspace with exact-ID native conversation resume requests from its
private checkpoint, using current configured commands.

skid remembers the most recent 5h, general 7d and Fable readings from the
enrolled machines for each Claude account. on desktop usage reads, a gateway
can request one read-only native `/usage` under an existing login on a qualified
build. the desktop shows remaining percentages and reset countdowns offline.
the managed claude footer shows used percentages from the same saved readings.
see [account quota](#account-quota-and-usage) and [desktop usage](#desktop-usage).

### Guarantees

- skid never closes a tmux session other than the exact confirmed target.
- Detach and close are visibly different actions; the browser and the phone
  confirm every close.
- Creation validates its directory and launches only the configured shell or a
  configured profile command, with explicitly requested native launch options.
- Provider credentials stay on their host. They never enter clients, push
  messages or skid state.
- A gateway or client restart re-lists surviving tmux; it never replays input.
- A peer reply names one live foreground lifetime. A delegated job reports
  one bound native turn's result, held on the worker host until jarvis acknowledges
  its durable copy. Both devices can operate the same worker in tmux.
- the native user service manager supervises tmux independently of the gateway.
  gateway restart, crash and gateway-only updates preserve terminal processes;
  clients can reattach after their gateway connection closes.
- an explicitly requested runtime restart validates the selected server's checkpoint
  before shutdown and uses ordinary recovery to reconstruct its workspace. this
  restarts processes; it does not preserve in-flight commands or unsaved memory.
- After proven tmux loss, recovery may use an inferred conversation ID. Ties stay
  unresolved; unusable targets become explained shells. Native resume success is
  never inferred from dispatch ([recovery](#8-workspace-recovery)).

### Non-goals

Provider conversation search, history, inboxes or generic hooks; chat UI;
copied provider history; transcript-derived semantic state; task-success
judgments; durable receipts or replay for arbitrary terminal input; a worker
queue, scheduling, a mailbox, autonomous orchestration; execution proxying; cross-machine move, broadcast, failover or
wake; automatic hub election; durable inventory; quota enforcement, manual reset
settings or general account queries; git or project
discovery; arbitrary host setup or tailnet policy automation; terminal embedding
in another app; containment between agents of one user; multi-user anything.

A new capability needs an explicit scope and acceptance decision by the owner.
Removing code authorizes nothing.

### Product language

Skíðblaðnir is the app. A visible session persona is a **dwarf** in product and
navigation copy (`Dwarves`, `Create dwarf`, `New dwarf`, `Back to Dwarves`);
**agent** means the foreground provider program. Lifecycle, recovery, error and
destructive copy is literal and names the tmux session and machine. Hlíðskjálf is
the grid's motif, the forge the creation sheet, and the Dvergatal the append-only
catalogue of named dwarves (`internal/catalog/characters.json`, at least 100
entries; [curation](design-language.md#18-dvergatal-curation)). Every session
carries one catalogue key; the key seeds its seal and never its tmux name.

## 2. Concepts

- **Machine**: one skid installation, named by its handle, `mh-` plus 32 lowercase
  hex, written once in `machine-handle`. Label, origin, bearer and platform are
  attributes, not identity.
- **Device**: one enrolment of a computer or a phone, `d-` plus 32 hex, with its
  own bearer. A computer is both a machine and a device.
- **Fleet** and **hub**: the enrolled devices and the one computer that holds
  their membership and accepted settings ([fleet](#4-fleet)).
- **Server lifetime**: `{epoch, pid, startTime}` of one tmux server. The epoch is
  `v1-` plus 32 lowercase hex in server option `@skid_server_epoch`, set only if
  absent; PID and start time are tmux builtins, so an old option restored into a
  new server never forges a lifetime.
- **Session lifetime**: a server lifetime plus a session ID `$N`, held by clients
  as the opaque `identityToken` `epoch.pid.start.N`. Names, panes and processes
  take no part.
- **Terminal target**: a session lifetime plus the active pane `%N` the caller
  saw; a different active pane is a different target. A **reference**
  `{machine, tmuxId, identityToken, paneId}` carries one across clients; a
  **handle** `t-…` names a session lifetime for humans
  ([CLI](#cli)).
- **Foreground**: the pane terminal's foreground process-group leader, one kernel
  sample. Exec keeps PID and start identity, so only an equal before/after
  bracket proves a foreground unchanged.
- **Agent**: a foreground recognized as Codex or Claude. **Profile**: a named shell
  command from the hub's YAML, optionally with a provider and account home. A
  session's **launch profile** (`@skid_profile`) records what created it; an
  agent's **observed profile** is the configured home its environment names.
  Neither stands in for the other.
- **Connection**: a pane whose foreground is `ssh` or `mosh-client`. The source
  session owns the card and every control; the far end is display context
  ([remote context](#remote-context)).
- **Inventory**: one sample of every session of one admitted server lifetime, with
  each agent's terminal status. A sample, not an atomic snapshot.
- **Terminal status**: independent activity, interaction and notice facts with a
  source and a closed reason, inferred from the current screen
  ([observation §2](terminal-observation.md#2-capability-and-wire-contract)).
- **Dwarf**: a session's catalogue key in `@skid_character`
  ([dwarves](#dwarves)). **Group**: an optional label in `@skid_space_b64`
  ([groups](#groups)). **View**: one of needs input, all, a named group or
  unassigned ([views](#views-and-the-needs-input-queue)).
- **Attention**: the hub's record per session lifetime of ready and action causes
  and of the owner's acknowledgement ([notifications](#12-notifications)).
- **Workspace recipe**: the private graph and selected conversation IDs from which
  recovery rebuilds a lost server and requests native resumes
  ([recovery](#8-workspace-recovery)); never live inventory or provider history.

## 3. Topology and trust

A fleet is one owner's computers and phones. Each computer runs one `skid serve`:
the gateway for its own default tmux server, its fleet member and, on the hub
only, the notification observer. The Mac also runs a separate notification app.
Desktop clients and the phone call every gateway directly.

```text
                   phone: compose + xterm.js
               /             |              \
      https/wss :8443        |        https/wss :8443
             /               |                \
   devbox serve        macbook serve         arch serve
   systemd, linux      launchd, darwin       systemd, linux
         |                   |                     |
     local tmux          local tmux            local tmux
```

| Owner | Owns | Never |
| --- | --- | --- |
| A computer's gateway | Its tmux server, processes, observation, terminal controls | Another host's tmux; execution on another's behalf |
| Desktop clients, phone | Composing enrolled gateways; their own presentation | Durable copies of inventory or attention |
| Hub | Fleet state, settings acceptance, invitations, attention and acknowledgement, Android push | Carrying terminal traffic; election; failover |
| Providers, the user | Provider installs, accounts, settings, history, daemons; Tailscale login and tailnet policy | |

### Trust invariants

1. Execution is host-local. A gateway observes and mutates only its own default
   tmux server. There is no proxy, coordinator or cross-host transaction, and one
   unavailable machine neither blocks nor authorizes action on another: one host
   out is the federation's normal state. skid manages tmux only where it is
   installed and enrolled; SSH or mosh from a skid terminal is ordinary use of the
   source terminal, never inventory of the destination.
2. Local use needs nothing remote. The gateway listens on `127.0.0.1:7341`; a
   computer's own CLI reaches it with no tailnet login, internet, provider, phone
   or Firebase. Clients synthesize the loopback route only for their own machine
   (`fleet.Local.Target`); loopback is never advertised.
3. Remote access is the tailnet. `skid connect` points tailscale serve `https:8443`
   at the loopback listener. A remote origin is exactly
   `https://<lowercase dns name>.ts.net:8443`, with no path, query, user or
   fragment. There is no public ingress, funnel, cookie, CORS, proxy or redirect.
   Other HTTPS endpoints are a non-goal: they would need certificate trust and
   renewal, ingress ownership, address changes and installer support.
4. Every device has its own random 256-bit bearer; hosts keep only its SHA256
   verifier and compare in constant time (`internal/auth`). Each computer also has
   an installed local bearer that authenticates its own CLI as that computer. A
   source address grants nothing: tailscale serve also arrives over loopback.
   Computers administer, phones are clients: the hub refuses a phone bearer on
   invitation, removal, apply and computer reports with 403 `Forbidden`. A
   phone's fleet routes are join, the public view and its revision stream.
5. `Skidbladnir-Machine` must equal the addressed gateway's machine handle. It
   binds a pairing to an installation and never authenticates. A mismatch is 409
   `MachineIdentityMismatch`, discloses no handle, and reaches no tmux, body or
   upgrade. Session rows carry no machine: a client labels rows with the gateway
   it addressed, so no gateway speaks for another.
6. One trusted user. Agents run as their host user; skid claims no containment
   against a same-user process. Client state is private to the login user:
   `fleet.json` and `machine-handle` are read only as `0600` regular files, so
   another Unix user, such as jarvis's system service, has no supported access
   ([jarvis's record](https://github.com/NielsdaWheelz/jarvis/blob/f397031fe089b90c601dcadbbc0d5ab0fc98d3a2/docs/issues/skid-installed-state-access.md)).
7. Logs are content-free ([operations](#logs)).

Accepted limits: tailnet TLS plus bearer is the whole defence. Nothing signs fleet
state, so a removed phone sees only auth failures, and same-fleet phone repair
sends its saved bearer to the scanned invitation's `*.ts.net:8443` origin.
Revocation is eventual ([fleet](#4-fleet), invariant 6).

## 4. Fleet

`fleet.json` is the one machine file: this installation's handle, name and local
bearer, its membership `{deviceId, bearer, state}`, its applied settings and, on
the hub only, `authority {invites, reports}`, whose presence is the hub role.
`state` is the one wire-and-cache type:
`{fleetId, hub, stateRevision, configRevision, machines, members, settings}`, with
machines hub first, then in join order. Phones get the public view: machines and
`androidPush`, no members or settings.

| Path | Owner | Note |
| --- | --- | --- |
| Config `machine-handle` | Fleet | Written once; the only identity root |
| Config `config.yaml` | The user | Read on the hub only |
| Config `android-push-credentials.json` | The user | Hub only, optional |
| State `fleet.json` | Fleet, serve only | The hub's is the fleet; a member's is a replaceable copy |
| State `authority.lock` | Fleet | flock for serve's lifetime |
| State `notification-observer-v2.json` | Hub notifier | Attention, acknowledgements, receivers |
| State `workspace.json`, `workspace.lock` | Sessions | [Workspace recovery](session-recovery.md) |
| State `serve.log`, `serve.log.1` | Logging | Mac only |
| State `install.lock` | Installer | |

Config is `${XDG_CONFIG_HOME:-~/.config}/skidbladnir`, state
`${XDG_STATE_HOME:-~/.local/state}/skidbladnir`. Private files are `0600` in `0700`
directories, written through `internal/privatefile`. Back up the handle and the
hub's `fleet.json` and observer file; a member's `fleet.json` is rebuilt by
joining again.

### Fleet invariants

1. One writer. `skid serve` holds `authority.lock` and alone writes `fleet.json`;
   a mutation edits a draft and becomes visible only after one durable atomic
   replace.
2. Crash-only identity. Machine identity and hub role are constants of one serve
   process. A committed join or recover, and any failed `fleet.json` write, end
   serve with a nonzero exit, because a failed replace may have landed and only a
   restart can tell. launchd `KeepAlive` and systemd `Restart=on-failure` restart
   it; the independent terminal service preserves tmux and its agents.
3. The handle is written once. A `fleet.json` naming another handle refuses to
   start. Deleting the handle re-keys the host and breaks workspace recovery.
4. The hub is fixed. Members fetch only from it, plain JSON over tailnet TLS with
   their own bearer: `GET /v1/fleet`, adopt, `PUT /v1/fleet/self`, then wait on
   `GET /v1/fleet/events`. Failures back off from 1 s to 30 s; a local request
   that reaches the hub resets the backoff. There are no signatures, relays,
   epochs, transfer or election.
5. Cached membership never expires. A member keeps admitting the fleet's devices
   and serving terminals while the hub is down; edits, enrolment, removal and
   notifications wait for the hub.
6. Revocation is 401: at once on the hub, and on every other host at its next
   fetch, which also closes the device's live terminal and notification
   connections. Accepted effects are not undone. The removed device learns
   nothing else: a phone keeps its store; a computer whose hub fetch gets 401
   drops its cached members, admits only its own local bearer, keeps serving its
   terminals, and `skid status` says it was removed.
7. Invitations. The hub mints a five-minute single-use token, keeps only its
   verifier, holds at most 32 live and refuses while it has no remote origin. An
   invitation is `skid://connect/` plus unpadded base64url of
   `{fleetId, hub:{machine,name,origin}, token, expiresAt}`, at most 4,096 decoded
   bytes. `skid join` and the phone admit it as human ingress; the hub alone
   decides use and expiry. It carries no permanent credential.
8. Join is a bearerless `POST /v1/fleet/join {token, deviceId, kind, name,
   machine?, origin?, verifier}` carrying the hub's machine header. Conflicts are
   checked before the token, which only a committed join consumes. A computer
   already a member is re-keyed in place, keeping its position; anything else is
   appended. Names are unique case-insensitively and origins unique; a duplicate
   needs an explicit `--name`, never a silent rename. The hub cannot join its own
   fleet or be removed. A lost join reply leaves the token spent: run `skid join`
   again with a fresh invitation.
9. Joining replaces a computer's fleet in one write and restarts it. Its handle
   and tmux sessions survive; its own `config.yaml` stays in place, unused while it
   is a member, and nothing merges; phones paired to its former fleet stay there.
10. The phone stages its device ID and bearer before joining, so it recovers a
    lost reply by fetching with them; repair first fetches with the saved identity
    and stages a new one only on 401. A phone paired to one fleet refuses another
    fleet's invitation until its app data is cleared.
11. Membership reaches running clients without a restart: the desktop re-reads
    `fleet.json` each inventory pass; member hosts and the phone follow the
    revision stream. `stateRevision` grows on membership, origin and settings
    changes, `configRevision` on accepted-settings changes; a report advances
    `stateRevision` only when it changes the reporter's origin.

### Fleet interfaces

Fleet routes authenticate with the caller's bearer unless noted, and bind the
machine header before authenticating:

| Route | Who | Contract |
| --- | --- | --- |
| `GET /v1/fleet` | Member | `state`; the public view for phones |
| `GET /v1/fleet/events` | Member | SSE `event: revision`, `data: {"stateRevision":N}` at connect and per revision; re-authenticates on each change, so a removed caller's stream ends |
| `POST /v1/fleet/join` | Hub, no bearer | `201` with `state` (phones: the public view) |
| `POST /v1/fleet/invites` | Hub, from computers | One invitation |
| `POST /v1/fleet/apply` | Hub, from computers | Accept the hub's `config.yaml`; status |
| `DELETE /v1/fleet/members/{deviceId}` | Hub, from computers | Removal; refuses the hub |
| `PUT /v1/fleet/self` | Hub, from computers | `{origin?, version, application}`, the caller's own report |
| `/v1/local/{status,network,join,apply,recover}` | Installed local bearer | The CLI's control surface; a member's apply and network report forward to the hub |

The fleet CLI (`cmd/skidbladnir/fleet_cli.go`) prints text on stdout; with `--json`,
every command but `connect` prints the resulting status as one JSON line instead;
errors stay text on stderr.
It exits 0; 1 on failure; 64 on usage, printing the reason and the command's help
on stderr. `-h` or `--help` prints that help on stdout and exits 0:

| Command | Effect |
| --- | --- |
| `skid connect` | Refuses when the running service's version differs from the CLI; then checks that Tailscale is running with MagicDNS and an HTTPS certificate domain for this host and, on Linux as a non-root user, this user as Tailscale's operator when `tailscale debug prefs` can be read, naming every missing fix at once and changing no tailnet or host policy; points `https:8443` at the gateway, replacing any 8443 handler, and shows Tailscale's own text when that fails; reports this host's origin; prints one invitation line, plus a QR on a TTY |
| `skid join [--name NAME] [--json]` | Reads one invitation line from stdin, enrols this computer, restarts its service; a failure changes nothing |
| `skid remove DEVICE_ID [--json]` | Removal at the hub |
| `skid recover [--json]` | A new fleet with this computer as hub, from its cached accepted settings with itself as default machine; restarts; startup then accepts its own `config.yaml` when valid |
| `skid apply [--json]` | The hub accepts its `config.yaml` |
| `skid status [--json]` | Members and each computer's last report, `applied`, `invalid_host` or `pending_offline`; no terminal or account content. A member shows the other computers `pending_offline`, since only the hub holds reports |

A command that needs the hub uses loopback and the local bearer on the hub, else
the hub's origin and this host's fleet bearer; with no hub origin it fails with
`hub has no remote address; connect tailscale on the hub and run skid connect there`.

Fleet errors are `{code, message}`: 400 `InvalidRequest` or `ConfigInvalid`; 401
`Unauthenticated` (including a removed device) or `PairingInviteRejected`
(unknown, expired or used token); 403 `Forbidden` (a phone on a route from
computers); 409 `FleetConflict`, including `this computer is not the hub` from a
member; 413 `RequestTooLarge`; 503 `HubUnavailable` from local routes while the
hub has no origin; 500 `InternalError`. Errors carry no routing; nothing
redirects a credential.

Accepted limits: join, recover and failed writes restart serve (3–10 s; WebSockets
drop and clients reattach). Moving or losing the hub is a new fleet
([operations](#fleet-operations)).

## 5. Configuration

One human file, `${XDG_CONFIG_HOME:-~/.config}/skidbladnir/config.yaml`, read on
the hub only (`internal/hostconfig`):

```yaml
version: 1
profiles:                    # ordered; [] is valid and the installed default
  - name: work               # profile key
    label: "codex · work"    # defaults to name
    command: codex-work      # opaque shell text, run once in the destination shell
    provider: codex          # optional observation metadata: codex | claude
    home: ~/.codex-work      # optional; requires provider
    resume_command: 'codex-work resume "$1"' # optional; recovery only
# shell: /bin/zsh            # absolute bash or zsh; default: each host's login shell
# terminal: apple-terminal   # mac notification clicks: apple-terminal | ghostty
# default_machine: laptop    # desktop creation default, by machine name
# android_push: ...          # optional; see android push
```

### Configuration invariants

1. Strict human ingress: at most 64 KiB, one document, unknown or duplicate keys
   and wrong types refused, then field rules with path-named errors. There is no
   `hub` or `machines` key, no per-machine override, include, interpolation,
   credential, endpoint override or provider preflight. `command` and
   `resume_command` are ordinary shell text. Every host reads the same `shell`,
   `terminal` and `default_machine`.
2. The installer writes the commented default once and never rewrites it; skid
   never edits human YAML.
3. The hub accepts its file at serve start, on `skid apply`, and in the background
   whenever a desktop browser opens on any computer. Acceptance is atomic: a valid
   file replaces the accepted settings, advancing revisions only on a change; an
   invalid one answers `ConfigInvalid` and the accepted settings stay. There is no
   file watcher; clients never upload settings; the hub stores accepted settings
   as JSON, not YAML bytes.
4. `default_machine` is checked against the fleet's machine names
   case-insensitively at acceptance. A removed machine still named there makes the
   next apply `ConfigInvalid`; until then a default naming no member resolves to
   self ([case defect](issues/default-machine-case.md)).
5. Each host resolves accepted settings itself (`hostconfig.Resolve`): the shell
   is the configured one or the account's login shell and must be an executable
   bash or zsh; terminal defaults to `apple-terminal`; provider homes default to
   `~/.codex` and `~/.claude` and expand against the destination home. A host
   whose new settings do not resolve keeps its previous applied settings and
   reports `invalid_host`, across restarts; membership and revocation still apply.
   A fresh host whose first settings do not resolve refuses to start.
6. Resolved settings swap into the session manager as one snapshot under its gate
   and affect later launches only; existing sessions keep their launch.
7. Joining adopts the destination's settings without merging; profiles of a former
   standalone hub are copied by hand.
8. A provisioner may own the hub's file through this same contract
   ([consumers](#consumers)); skid has no provisioner mode.

## 6. Host sessions and tmux

Each gateway serves one host: its local default tmux server, that server's
sessions and the processes in their panes. tmux is the store; the gateway keeps no
session database and replays nothing. §6 to §9 are the contract of
`internal/{sessions,tmux,terminalservice,group,catalog,terminalcontext,agentcontrol,agentruntime,process,quota,workdir,pressure}`.

### terminal service

one fixed native user service owns the default tmux server. the installer provisions
it; dev-server is optional. gateway tmux commands are connect-only (`-N`), including
creation, recovery and attachment. native clients use the same server. an externally
bootstrapped default server is a conflict; skid never adopts it or stamps its options.

`terminalservice.Inspect(ctx, observed, present)` returns managed, absent, unavailable
or unmanaged, or an observation error. it brackets the documented native service pid
with kernel start identities and the observed tmux process. mac uses `launchctl list`
and verifies the host is tmux's parent in the same process group; linux uses
`systemctl --user show --property=MainPID` and requires tmux itself be that process.
no job pid plus no reachable server is absent; a running job without a reachable
server is unavailable. errors never prove absence. recovery and drain separately
check any recorded source process. cache admission only for the exact server lifetime,
including an empty server, before any epoch/metadata write or terminal effect.

on mac the signed `skid terminals` host starts and waits for package-manager
`tmux -D -c 'exit 78'`; launchd supervises it in its ordinary process group. linux
systemd supervises that tmux command directly. unexpected completion restarts; a
collision exits without a restart loop. `-c` bypasses the default client command,
which could otherwise create unintended sessions during concurrent startup. a
collision can still run the existing default shell's initialization. skid preserves
normal user configuration; incompatible exit settings fail readiness with an action.

this is a process-lifecycle and consent-identity boundary, not containment. providers
remain shell children and retain their own accounts, execution and history. native
mac responsibility, grant persistence and crash cleanup remain
[unqualified](issues/terminal-service-qualification.md); private sockets and scratch
homes do not isolate privacy state. accounting separation alone saves no energy.

### tmux facts relied on

Observed on Linux tmux 3.4 and Darwin tmux 3.7b through 3.8; the behaviour binds,
the versions do not. Hosts run their package channel's latest stable tmux; a new
version counts as qualified only after observation's
[requalification](terminal-observation.md#7-requalification).

- Concurrent tmux clients of one session share its current window and active pane;
  navigation in either is visible to the other. skid targets a session and never
  moves another client's selection.
- Members of a tmux session group share windows and processes, so deleting one
  member can leave them alive through another. skid creates no session groups.
- `if-shell -F` inserts its chosen branch immediately after itself, and one
  client's queue drains before tmux returns to other event work
  ([`cmd-if-shell.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/cmd-if-shell.c),
  [`cmd-queue.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/cmd-queue.c)):
  a guard and its effect cannot be split by another client.
- `set-option -s` stores a user option among the server options, which format
  lookup checks before session, window and pane options, so a local option
  cannot shadow the epoch
  ([`options.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/options.c),
  [`format.c`](https://raw.githubusercontent.com/tmux/tmux/3.4/format.c)).
- Through a tmux client, raw ctrl-j (`0x0a`) is newline without submit and raw CR
  (`0x0d`) submits in the stock provider TUIs.
- A SIGKILLed TUI emits nothing, so only tmux and kernel facts prove liveness.
- Every tmux invocation drops inherited `TMUX` and `TMUX_PANE`, so a gateway
  started inside tmux still addresses the default server.

### Host invariants

1. Every effect on a session is one `if-shell -F -t $N` queue whose guard pins the
   server lifetime and the session ID (terminal effects also the active pane),
   with the effect and a trailing applied marker in the same queue. A refusal
   mutates nothing; a client failure after start means the effect may have run.
   No gateway lock, pre-read or retry substitutes for the guard.
2. One tmux client talks to one server for its life, so a script bracketed by
   equal server-identity records, printed first (`B`) and last (`E`), ran on that
   server; unequal records are `WorkspaceSourceChanged`.
3. Names, titles, launch profiles, dwarves and groups never select or authorize a
   live terminal effect. Recovery alone uses title inference as private restart
   intent after proven loss. Rename, cwd change and agent replacement keep every lifetime-addressed
   operation valid; a new server or a reused ID never matches an old token.
4. skid metadata are session-local tmux user options. A server, global or
   global-window `@skid_*` definition makes format values unattributable, so the
   reader falls back to `show-options -t` per session. Window- and pane-local
   definitions are unsupported and shadow undetected; skid writes none. Malformed
   optional metadata reads absent and is never repaired on read, except the dwarf
   key.
5. One capacity-one, context-aware manager gate serializes inventory
   normalization, creation, rename, group, kill and recovery; an expired waiter
   starts no tmux work. Checkpoint metadata discovery releases the gate, then
   validates its immutable recipe anchor, settings and final observations before
   committing. Terminal reads, input and attachment take no gate: they read
   immutable settings and are guarded in tmux.
6. A mutation is attempted once. Lost completion is `unknown` and never replayed,
   rolled back or compensated; clients reconcile from a later inventory.
7. Terminal status describes only the current screen of a freshly recognized local
   agent. A failed stage is `unavailable`, ambiguity is `unknown`, and neither is
   converted into the other. Shells, remote and unrecognized panes are never
   classified.
8. An observation describes its row's exact target: session, pane, pane PID,
   screen size, alternate screen and foreground. A reused detection is valid only
   while that key proves the screen unchanged.
9. One pane's failure or refusal never fails another pane or the inventory. Only
   required facts (the tmux snapshot, normalization, the lifetime proof, the
   budget) fail a host's inventory.
10. No provider history or conversation ID enters inventory or live control; no
    conversation route exists. Recovery alone stores selected IDs, profile/home
    scope and evidence source privately. It reads bounded native metadata and
    invokes the owner's exact-ID shell command; it stores no titles or history.
    the only account operations are the read-only codex daemon read and the
    qualified native claude usage read in [§9](#account-quota-and-usage).

### Inventory

`GET /v1/sessions` runs under 5 s, covering admission, normalization, enrichment
and projection; a late or failed required step is `500 InternalError`, never a
partial answer.

1. Read: one `-N source-file -` script prints `B`, an inherited-definition probe
   (`show-options -q` at `-s`, `-g` and `-g -w` for the five session keys), one
   `list-sessions -F` record per session (8 builtins, the five keys as
   `#{q:@skid_…}`, the active pane's width, height, alternate flag and window
   activity), the probe again, then `E`. Any probe line triggers one more exec
   reading local options per session. Fields are `#{q:}`-quoted with an unescaped
   `|` terminator and decoded before record boundaries; duplicate IDs and bad
   required fields are rejected; a transcript is bounded at 8 MiB, and overflow
   fails rather than truncates ([exec budget a1](tmux-exec-budget.md#a1-inventory-in-one-exec)).
2. admission: inspect and prove native management for each new server lifetime,
   including an empty server, before stamping its epoch and reading it again. the
   remembered lifetime is a hint; the brackets are the proof. a managed empty server
   succeeds with `sessions: []`; absent, unavailable or unmanaged service is
   `503 TerminalServiceUnavailable`. a server that vanishes or is replaced after
   admission retains its source/target error semantics.
3. Normalization, under the gate: dwarf repair, then automatic naming. A write
   projects only when tmux printed its applied marker.
4. Projection, outside the gate: per row one foreground sample (agent, observed
   profile, connection); then enrichment under one shared 2 s deadline: screenless
   statuses and reused detections, one capture batch for every remaining agent
   pane, and per-pane observation for each refused block
   ([exec budget a2, a3](tmux-exec-budget.md#a2-all-captures-in-one-exec)). A batch
   without a bracketed transcript falls back per pane and re-proves the lifetime
   once.
5. Envelope `{machine:{handle, platform}, observedAt, recovery, profiles,
   sessions}`; `observedAt` is taken after normalization; rows arrive in numeric
   tmux-ID order, and clients own display order.

A row is `tmuxId, activePaneId, tmuxName, nameMode, identityToken,
character{key,displayName}, attachedClients, terminalStatus, status` plus optional
`launchProfile, agent{paneId,pid,startIdentity,provider,profile?,label?},
connection{transport,id?}, objective, group, cwd, activeCommand, terminalUsage,
terminalFooter`. `status` is the host-rendered presentation
([observation §6](terminal-observation.md#6-content-attention-and-filtering)). A
row describes the current window's active pane, so one session is one card;
addressing other panes would need its own pane-level discovery. A connection row
has no agent and no cwd. `attachedClients` is tmux's `session_attached`. A profile
is `{key, label, provider?, accountScope?}`; inventories carry `profiles: []`,
never null.

### Creation

`POST /v1/sessions` takes `{kind:"agent", profile, cwd, optionalTmuxName?,
objective?, group?, model?, effort?}` or the same with `kind:"terminal"` and no
profile/model/effort;
`POST /v1/sessions/{tmuxId}/shell` takes `{identityToken}`. Both answer
`201 {observedAt, session}`. Terminal creation works with zero profiles.

1. Validation precedes any tmux effect: cwd (exact `~` or `~/…` under the service
   home, else absolute, cleaned, at most 4,096 bytes, no C0/C1, U+2028/2029 or
   bidi controls, an existing searchable directory), a profile of this host's
   current settings, a name matching `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$` and unused,
   an objective of 1–240 NFC scalars without C0/C1, U+2028/2029 or bidi controls
   ([NFC defect](issues/xtext-nfc-validation.md)), and a canonical group label. A present but empty name, objective or group is
   invalid; null is absent. Optional model (1–255 UTF-8 bytes) and effort (1–64)
   reject controls and edge whitespace. With either option, the session manager
   requires a codex/claude profile and one plain executable command that forwards
   native arguments; compound commands refuse before tmux with `InvalidRequest`
   and `not_sent`. Omitted options leave any configured command unchanged.
2. One `new-session` queue creates the session, stamps the epoch if absent, sets
   `@skid_profile` (agent only), `@skid_auto_name_b64` (omitted name only),
   `@skid_character`, the encoded objective and group when supplied, and prints the
   identity. An omitted name is the smallest free `skidbladnir-<profile>-<N>` or
   `skidbladnir-terminal-<N>`, in automatic mode.
3. The pane runs `skidbladnir terminal-exec <b64url cwd> <launch> <b64url shell>`
   as direct tmux argv, and the tmux client runs in the cwd so tmux stores it for
   later windows. terminal-exec drops inherited `SKIDBLADNIR_*` launch markers,
   sets `SHELL`, `SKIDBLADNIR_SHELL=1` and `SKIDBLADNIR_STARTUP_{PID,CWD,HELPER,AGENT}`,
   and execs the configured bash or zsh as a login shell. The reason: tmux expands
   cwd formats, falls back after a failed directory entry, substitutes an invalid
   default shell and parses trailing semicolons, and encoded argv meets none of
   those. A shell removed after settings were applied fails only that pane with
   `terminal startup failed`; the host reports `invalid_host` at its next settings
   commit.
4. `shell-init`, sourced at the end of bash and zsh login startup, runs once in the
   intended shell before its first prompt: it removes itself and the startup
   variables, enters the cwd (on failure it reports and stays home), and for an
   agent launch requires TTY stdin and stdout, decodes the profile's command text
   through `agent-exec` and evaluates it once. Re-sourcing and subshells cannot
   replay it. The agent is an ordinary foreground child: its exit, failure or
   `/exit` returns to the same shell; `exec` and `exit` keep their meaning.
5. Explicit launch options append separately shell-quoted native arguments to that
   forwarding command: codex `--model VALUE` and `-c model_reasoning_effort=STRING`
   (TOML string encoding first); claude `--model VALUE` and `--effort VALUE`.
   skid adds no account variables, daemons or hooks. Aliases, functions, wrappers
   and provider settings retain their authority; the profile command must forward
   arguments. Native support and entitlement are the provider's verdict after
   creation; skid validates no provider model catalog or effort enum. Observation
   metadata never changes execution; a tmux name never reaches a provider.
6. shell-here reads the source once (lifetime, active pane, its PID and cwd, local
   group), refuses an SSH or mosh foreground or an unavailable cwd with
   `WorkingDirectoryUnavailable`, and creates a terminal there in that group, its
   queue guarded by the source lifetime, so a vanished source or a new server
   creates nothing and starts no server. The two sessions are then independent.
7. Outcome ([dispatch](#gateway-wire-invariants), item 3): failures before tmux
   accepted the command are definite (`dispatch:not_sent`). After acceptance any failure is `500 dispatch:unknown`,
   with `target{tmuxId,identityToken,paneId}` once the complete target was read
   back; nothing is retried or cleaned up. `201` proves an observed session, not
   shell or provider readiness; its status usually precedes the provider's first
   draw.

### Attach and handoff

`GET /v1/sessions/{tmuxId}/terminal` upgrades to the terminal WebSocket
([wire](#gateway-wire-invariants)) for the session the `Skidbladnir-Session-Identity`
header names.

1. The gateway resolves the lifetime before upgrading. The client's first text frame
   must be a valid `Resize` within 5 s, before any PTY or tmux client exists; an
   invalid, oversized, late or closed start creates nothing, and an initial
   `Detach` closes cleanly. The PTY starts at that size.
2. One tmux client, `-u -T RGB` under `TERM=xterm-256color`, runs
   `attach-session -E -t $N` inside a guard pinning the session lifetime and
   requiring effective `window-size latest`, `destroy-unattached off` and
   `detach-on-destroy on`; otherwise the gateway sends
   `TerminalConfigurationUnsupported` before `Hello`. Attachment creates no
   session, group or option and changes none.
3. Startup and lifetime contexts are distinct: admission and readiness (25 ms
   polls, at most 5 s) use startup; cancelling a finished startup never kills the
   owned client.
4. `Hello {attachedClients, paneId, foreground?}` precedes terminal bytes: the owned
   client's pane is read twice with a foreground sample after each, and
   `foreground` appears only when pane and pane PID held and the samples are equal
   and recognized. Its uncertainty never refuses a valid terminal
   ([unused identity](issues/terminal-hello-identity.md)).
5. Presence reads the owned client every 2 s and sends `Presence` on change; a
   session switch closes the connection on detection, not atomically. Detach, loss
   or shutdown closes only the owned PTY and client; the session and its processes
   are never destroyed by detach. Laptop and phone share navigation and
   latest-client sizing, so a grid change on either resizes the shared window.

### Naming

1. `session_name` is the only name. An omitted creation name is automatic; a
   supplied name, a saved rename and any unmarked or externally renamed session
   are manual. `@skid_auto_name_b64` holds base64url of the last automatic name:
   automatic exactly while it decodes canonically to the current name. Anything
   else is manual, and a valid marker naming another name is unset by the next
   inventory.
2. Each inventory converts the active pane's complete `pane_title` of an automatic
   session: valid UTF-8 of at most 4,096 bytes without controls, bidi controls,
   U+2028 or U+2029; ASCII-lowercased; each run outside `[a-z0-9]` one hyphen;
   trimmed; cut to 64 and re-trimmed. An empty, invalid or non-ASCII-only title
   changes nothing. A current name already equal to the base or one of its valid
   suffixes stays; otherwise the base, then `<base>-s<N>`, then `<base>-s<N>-2`,
   `-3`, … from the inventory's names.
3. A naming write is one guarded queue: lifetime, current name and marker as format
   lookup sees them; automatic renames also the pane ID and full sampled title; any
   rename also that no other session holds the name. The rename precedes the
   marker change, so an interruption reads manual. tmux runs with `-u` because it
   sanitizes non-ASCII output under a non-UTF-8 locale; every literal crosses as
   `#{l:}`.
4. `PATCH /v1/sessions/{tmuxId}` `{identityToken, expectedNaming, naming}`, each
   `{mode:"automatic"}` or `{mode:"manual", name}`. Expected automatic accepts any
   current automatic name; expected manual needs that exact current name, which may
   lie outside skid's grammar. Saving the current name while automatic switches to
   manual; automatic resets ownership even with no usable title. `204`, then
   inventory. A changed mode or name is `SessionNameChanged`; a held name
   `SessionNameConflict`; a write failure on a still-valid lifetime is `500` and
   may have applied.

### Dwarves

Every returned session carries a valid catalogue key in `@skid_character`.
Inventory repairs an absent or invalid key: among this inventory's least-used
keys, the greatest `sha256(seed NUL key)`, with seed `epoch NUL $N` (at creation,
the minted epoch candidate), written only while the session's format value is the
sampled one. A lost race re-reads once: vanished is omitted, valid is accepted,
otherwise one more guarded attempt; non-convergence fails the inventory. Valid
keys never change; the key survives rename and process replacement and never
derives a name. Only the key persists: clients derive the seal and colours from
it ([design language §11](design-language.md#11-dwarf-seals-procedural-portraits)).
`internal/catalog` embeds the catalogue and panics at start on an invalid or
duplicate key.

### Groups

1. A session belongs to zero or one group: a canonical label in session-local
   `@skid_space_b64` as canonical unpadded base64url. There is no group registry,
   ID, lifecycle, launch context or tmux session group; equal labels group across
   hosts by exact equality.
2. Canonical label: 1–64 scalars, at most 256 UTF-8 bytes, NFC; no C0/C1, U+061C,
   U+200E–200F, U+2028–202E or U+2066–2069; no edge U+0020 and no other Unicode
   whitespace; interior spaces kept. `group.Parse` is the only grammar, and
   metadata and responses carry only canonical text. A write is a draft: the
   gateway canonicalizes it to NFC, then parses (`group.ParseDraft`), so clients
   send drafts as typed ([draft rule](rules.md#copy-and-drafts)).
   Order: ASCII-case-folded bytes, then exact bytes; unassigned last. Invisible
   code points are admitted ([issue](issues/group-label-invisible-characters.md)).
3. NFC is plain UAX #15: x/text's whole-string normalizer inserts CGJ after 30
   non-starters and composes across a blocking starter, so the label owner reuses
   only its per-scalar tables. Those tables follow the Go toolchain (Unicode 15.0
   before go1.27, 17.0 from it); `scripts/build-host-binary` holds the go.mod
   family. A toolchain move changes equality for newly assigned code points,
   and nothing checks it.
4. `PUT /v1/sessions/{tmuxId}/group` takes exactly `{identityToken, group}` (empty
   clears; absent or null is invalid): one guarded set or unset, `204`, last
   applied write wins, the same value and clearing an unassigned session succeed.
   Any tmux error is `500 dispatch:unknown`; a refusal is `SessionNotFound` or
   `SessionIdentityMismatch`; a bad label is `422 GroupInvalid`, never echoed.
5. Invalid or absent local metadata projects unassigned; globals are never
   inherited.

### Remote context

1. Inside skid shells and tracked inbound connections, `terminal-context-init`
   defines `ssh` and `mosh` functions, unless the user has one, that mint a
   128-bit connection ID and pass it as `SKIDBLADNIR_CONNECTION` with argv
   unchanged. An interactive remote shell with a TTY consumes the variable,
   registers its session leader and sets `SKIDBLADNIR_TERMINAL_CONTEXT=1` so nested
   shells keep the functions. Forwarding the variable (SSH `SendEnv`/`AcceptEnv`
   or Tailscale `acceptEnv`) is a deployment prerequisite outside skid.
2. A registration `{bootId, pid, startIdentity, tty}` lives at
   `~/.cache/skidbladnir/terminal-contexts/<id>.json` (`0700`/`0600`). Registering
   first prunes other-boot, unreadable and dead records; reads never prune. It
   holds only while the root is the same live session leader on the same nonzero
   TTY on this boot. It records association, never lifetime, status or history.
3. The source row carries `connection{transport, id?}` and hides cwd and agent.
   `GET /v1/terminal-contexts/{id}` answers `{observedAt, cwd?,
   agent?{provider,profile?,label?}, connection?}` (at most 64 KiB) or
   `404 TerminalContextUnavailable`. A further transport reports only its
   connection; tmux or screen in the foreground is unavailable; every read is
   bracketed by an equal process sample.
4. Clients resolve a marker across every paired peer in one 2 s pass, following
   onward connections up to 8 hops with a cycle stop; exactly one answering peer
   matches, none or several is `remoteUnknown`. Remote facts describe; the source
   keeps attachment, availability and every action, and no gateway calls another.

### Host interfaces

| tmux option | Scope | Value | Written by |
| --- | --- | --- | --- |
| `@skid_server_epoch` | Server | `v1-` + 32 hex | Creation, first admitting inventory, recovery |
| `@skid_character` | Session | Catalogue key | Creation, inventory repair, recovery |
| `@skid_auto_name_b64` | Session | base64url automatic name | Creation without a name, naming; unset by recovery |
| `@skid_space_b64` | Session | base64url label | Creation, group route, recovery |
| `@skid_profile` | Session | Launch profile key | Agent creation |
| `@skid_objective_b64` | Session | base64url objective | Creation |

| Entrypoint or variable | Contract |
| --- | --- |
| `skidbladnir terminal-exec`, `agent-exec`, `terminal-context new\|register` | Private; one producer each in this binary or its shell integration |
| `shell-init`, `terminal-context-init` | `deployment/providers`, sourced by installed shell blocks |
| `SKIDBLADNIR_SHELL=1`, `SKIDBLADNIR_STARTUP_*` | Set by terminal-exec, consumed once by shell-init |
| `SKIDBLADNIR_CONNECTION`, `SKIDBLADNIR_TERMINAL_CONTEXT=1` | Connection marker and nested-shell scope |

| Package | Owns |
| --- | --- |
| `tmux` | Every tmux invocation: guarded effects, inventory and capture batches, attachment, workspace capture and reconstruction, literal encoding |
| `sessions` | Lifetimes and token admission, inventory normalization and projection, creation, naming, dwarves, group writes, terminal targets and input, recovery policy |
| `agentcontrol` | Inventory enrichment and reuse, the pure classifier, inspect, read, guarded send and stop |
| `agentruntime` | Profile resolution, provider recognition, account attribution and scope |
| `process` | Kernel foreground, environment, cwd and boot identity: Linux `/proc`; Darwin `KERN_PROC`, `KERN_PROCARGS2`, `proc_pidinfo` and `proc_pidpath`, never parsed `ps` |
| `group`, `catalog` | Label grammar and order; the frozen dwarf catalogue |
| `terminalcontext` | Connection registrations |
| `quota` | Host-local Codex daemon reads, Claude cache observations and bounded native Claude usage requests |
| `workdir`, `pressure` | Directory admission, listing and search; pressure sampling, policy and history |

sessions imports neither agentcontrol nor quota; the gateway composes them. Log
events: `Session.Created`, `Session.Killed`, `Terminal.ObservationFailed`,
`Terminal.CleanupFailed`, `Workspace.RecoveryChanged`.

### Host accepted limits

- Inferred status can be wrong, five-second sampling misses brief transitions,
  and conservative unknown delays or loses idle; the grammars list each provider's
  costs and accepted false claims.
- Launchers hide providers ([issue](issues/provider-wrapper-recognition.md));
  deleted images are not recognized.
- Names follow titles only while some client inventories the host; titles lose
  punctuation and Unicode; a rename away and back between samples is undetectable.
- Window- or pane-local `@skid_*` definitions shadow undetected.
- Remote context is unknown without forwarding, integration or a unique answer,
  and unavailable under an inner tmux or screen.
- Reuse and recovery residuals live in their specs.

## 7. Observation and agent control

### Terminal observation

[Observation](terminal-observation.md) owns the status meaning, screen regions,
grammars, send admission and wait states. Around it:

1. Recognition: an executable whose base name is `codex` or `claude`, or a direct
   child of the canonical `~/.local/share/claude/versions` (resolved per call;
   Linux's ` (deleted)` suffix never matches), which outranks the name. Only the
   process-group leader is examined and argv is never read, so an npm or
   waiting-shell launcher hides its provider. Older images keep matching after a
   launcher relink: launch configuration is not recognition.
2. Account attribution: the one same-provider profile whose canonical home is
   `CODEX_HOME` or `CLAUDE_CONFIG_DIR`, else absolute `HOME/.codex` or
   `HOME/.claude`, from the foreground's environment; none or several is unknown,
   never the launch label. The read keeps only those keys and
   `SKIDBLADNIR_CONNECTION`, is bounded at 1 MiB, rejects incomplete or duplicate
   records, and closes with an equal foreground sample; a change discards the
   whole sample.
3. Focused observation: one guarded read of screen size and alternate state, one
   guarded queue capturing each selected row with its own `capture-pane -p -e`
   while that read holds, then a foreground re-sample against the session's own.
   Inspect, guarded send and the enrichment fallback share it. An expired or
   cancelled context is `observation_timeout` at any stage; a target change is an
   error for explicit operations and `capture_failed` in inventory.
4. The classifier is pure; a classifier defect panics, confined to its request,
   and is never reported as unavailable.
5. `api.PresentStatus` is the one presentation
   ([observation §6](terminal-observation.md#6-content-attention-and-filtering)):
   the gateway sends it as each row's `status` and Go clients call it.
6. a successful local sample can supply `terminalFooter` (1–1,024 bytes, no
   `Cc`, `Cf`, `Zl` or `Zp`, no edge whitespace). a codex sample can also
   supply `terminalUsage {fiveHour?, sevenDay?}`, with at least one window
   carrying its displayed integer `usedPercent` (0–100, including zero).
   each sample replaces these displayed facts. a failed or changed capture
   withdraws them. display extraction is independent of status, composer,
   rules and clipping. claude account usage travels on the profile-usage route.
   the grammars specify the admitted text
   ([Codex §8](terminal-observation-codex.md#8-displayed-statusline-items),
   [Claude §7](terminal-observation-claude.md#7-displayed-statusline-suffix)).
   `terminal/inspect` carries status only.
7. Every unavailable observation logs `Terminal.ObservationFailed` with its reason
   and duration.

### Terminal control

`POST /v1/sessions/{tmuxId}/terminal/{op}` takes `{identityToken, paneId}` and only
that op's members; the gateway checks shape and bounds once. Every op runs under
10 s, and each tmux command stops waiting 100 ms after its context ends. Every
terminal supports these ops whether or not an agent is present, and status
uncertainty never disables stop or close.

| Op | Members | Result | Contract |
| --- | --- | --- | --- |
| inspect | `explain?` | `{terminalStatus, diagnostics?:{rules}}` | Fresh resolve, one sample, 2 s; `rules` present iff explain, possibly empty; a failure after admission is `200` unavailable |
| read | `maxBytes?` 1–32,768, default 16,384 | `{text, source:"terminal", scope:"terminal_history"\|"visible", truncated}` | The visible screen under an alternate screen, else the retained tail; a foreground change across the read is `TerminalUnavailable`; the newest text fitting one response |
| address | `foreground?` | `{sender,digest}` | Resolve a named pane and its recognized foreground; a launch stamp must match the current process |
| send | `text`, `initialProfile?` | `{method:"terminal", outcome:"written"\|"unknown"}` | Paste and Enter only if the sample admits it ([observation §5](terminal-observation.md#5-controls-and-diagnostic-api)); `initialProfile` also needs that observed profile, idle and no notice, else `initial_not_ready` |
| text | `text` | Write receipt | Deliberate paste and Enter |
| keys | 1–16 of `enter escape ctrl-c up down left right tab backspace page-up page-down` | Write receipt | Deliberate keys |
| stop | None | Write receipt | One key, never repeated or escalated: escape to local Codex, ctrl-c otherwise |
| close | None | `{interrupt:written\|not_sent\|unknown, terminal:closed\|not_closed\|unknown}` | Validate the lifetime, one stop, end owned connections, delete the exact session |

Text is nonempty valid UTF-8 of at most 32 KiB, without NUL. `DELETE
/v1/sessions/{tmuxId}` with exactly `{identityToken}` is close without the
interrupt: one guarded `kill-session` queue after the gateway closes its owned
connections, answering `204`. Only the selected session is removed; shared windows
or processes of a tmux session group may survive. There is no working or idle gate:
the guarantee is the exactness of the target, not semantic safety.

1. Input is staged in a unique tmux buffer before the final foreground
   revalidation; paste (bracketed) and Enter run in one guarded branch that also
   requires a live pane; a refusal runs neither, and cleanup removes only that
   buffer within 1 s. Provider-specific input (send, stop to an agent) revalidates
   the agent and foreground; text and keys need terminal authority only.
2. Bytes written prove terminal delivery only, not a program's or provider's
   acceptance, submission or completion. Ctrl-C and closure prove no cancellation.
   No step joins job control, the screen read and consumption, and no supervisor
   pretends to.
3. close inside its 10 s: validation ends 2 s before the deadline; the interrupt
   gets at most 2 s, ending by deadline − 8 s; connection cleanup at most 6 s,
   keeping the last 2 s for deletion; each phase has its own context; cleanup
   failure is logged and never vetoes deletion. `not_closed` means refused before
   deletion; `unknown` means possibly deleted. Closure proves nothing about
   shared-window, background or remote work.
4. wait is a client loop over inspect: capture the target once, sample at once and
   every 5 s, 60 s by default, at most 1 h, matching the states
   [observation §5](terminal-observation.md#5-controls-and-diagnostic-api)
   defines. An unavailable sample fails the wait; `TerminalTargetChanged`,
   `SessionNotFound` or `SessionIdentityMismatch` end it `target_changed`. Provider
   exit or restart in the same pane is the same target.

Peer replies use `POST /v1/peer-replies/{digest}` with literal text. The host
resolves one recognized foreground lifetime and uses the ordinary guard.
Delegation uses `PUT/GET/DELETE /v1/jobs/{id}`. GET can hold for 25 s and accepts
one canonical `offset` for answer paging. The [delegation contract](agent-delegation.md)
defines the worker and result lifetime.

Managed workers classify their status and ordinary composer through their local
socket. Guarded sends join the same input path as the terminal and refuse a draft
or request. Managed codex consumes ctrl-c. Ordinary explicit read, text and keys
retain the terminal path. The managed process and its pending job are live only
for their exact lifetime; workspace restore never resubmits an assignment.

Accepted limits: terminal input is not atomic with consumption; remote panes get
generic controls only.

## 8. Workspace recovery

[The spec](session-recovery.md) owns the recipe, the restore algorithm and the
failure vocabulary. The contract:

1. The recipe is never inventory, status or authority; tmux stays the store. A
   gateway or client restart while the server survives only re-lists it.
2. Loss is proven only by a changed boot identity, or the saved server PID absent
   or reused (kernel start identity). A missing socket or a failed observation
   proves nothing.
3. restoration targets only a managed empty server, after validating destination
   readiness/settings and durably claiming the confirmed schema-2 recipe
   (`attempted=true`), and rebuilds final panes at their
   saved local cwds. Recognized local agents use native-ID then title discovery;
   unusable targets become explained shells. Each usable conversation launches
   once through the current profile's `resume_command`, in the lowest numeric
   eligible saved pane. Reconstruction is never merged, rolled back or replayed.
   an absent/unavailable destination delays until the existing tick without consuming
   the claim; unmanaged destinations refuse reconstruction. recovery never starts tmux.
4. One writer holds a flock on `workspace.lock`. Recovery starts after the listener
   and reconciles every 30 s and after each confirmed create, shell, rename, group
   and kill. Source admission precedes required epoch/character preparation;
   checkpoint captures then read without mutations and preserve actual tmux names.
   Automatic naming belongs to inventory. Provider reads run outside the manager
   gate within a 2 s sub-budget of the existing checkpoint deadline. Final source/graph/process/settings
   validation and durable publication run under it. A checkpoint failure never
   changes a confirmed mutation's outcome.
5. Every inventory carries `recovery {state:"tracking"|"recovered"|"delayed"|"broken",
   savedAt?, failure?{at,phase,cause}, resume?{requestedPanes,inferredPanes,
   skippedAgentPanes}, notice?{summary,action?}}`. Counts occur only in recovered
   and mean startup dispatch, never provider success. `savedAt` is the
   capture time of the last confirmed recipe. Ordinary observation/preparation/save
   failures delay until a fresh attempt on the tick; failure never proves loss.
   Broken latches startup ownership/admission failures, positive source conflicts,
   failed restoration preflight/claim and any post-claim failure for the process
   lifetime. The host renders `notice`, clients add the host and the local save
   time. Failed recovery leaves live inventory
   authoritative, and clients never use a recipe as inventory or revive old
   terminal authority from it.
6. The first restored checkpoint seeds only dispatched targets as recipe intent;
   the next ordinary capture replaces them. Schema 1 is rejected. Cutover retains
   the old recipe separately and arms a fresh schema-2 baseline while tmux lives;
   no migration or compatibility reader exists.

## 9. Quota, pressure and directories

### Account quota and usage

1. `GET /v1/profile-usage` reads the configured codex and claude profiles in
   configuration order with the ordinary 5 s read budget. codex reads run
   concurrently. each opens a websocket over
   `<home>/app-server-control/app-server-control.sock`, then `initialize` (client
   `skidbladnir`), `initialized` and `account/rateLimits/read`, with no close
   handshake. Server messages are skipped, never answered. It never starts,
   restarts, logs into or updates a daemon, runs no tmux and dispatches nothing to
   conversations. the gateway signals due local claude work and returns its
   snapshot independently. ordinary reads ingest the current native cache.
   the worker has no separate path into retained history.
2. the host supplies `machine`, `observedAt`, `profiles` and `claudeAccounts`.
   both arrays are present even when empty. a codex profile carries `readState`
   (`ok` or `unavailable`). its
   `report` carries `reportedAt` and optional `fiveHour` and `sevenDay`
   windows with `usedPercent` and an optional `resetsAt`. only the
   `rateLimits` default snapshot counts; windows are named by `windowDurationMins`
   300 and 10080, never position; unknown durations are skipped; a duplicate, a
   non-integer token or a reset outside years 1–9999 makes that profile
   unavailable; frames are at most 4 MiB. A report dated after `observedAt` is
   dropped; a settings change during the reads fails the route.
3. for claude, the gateway reads `~/.claude.json` for the default profile and
   `<profile home>/.claude.json` for a named one. claude's nonempty
   `<profile home>/.config.json` takes precedence. if redirecting claude to
   `~/.claude` would select a second, distinct global config, the gateway
   marks that default profile unavailable. the footer knows its actual
   `CLAUDE_CONFIG_DIR` and reads the file claude selected.
   `cachedUsageUtilization.accountUuid` must match `oauthAccount.accountUuid`
   in the same file. a matching cache with a positive, nonfuture `fetchedAtMs`
   independently dates every supplied amount and reset. unique unscoped
   `session` and `weekly_all` rows supply general fields, using `five_hour` or
   `seven_day` for fields those rows leave empty. one model-only
   `weekly_scoped` fable row supplies its own amount and reset. ambiguous rows
   supply no typed fields for that meter.
   [profile file location](https://code.claude.com/docs/en/claude-directory) ·
   [model-scoped usage rows](https://code.claude.com/docs/en/agent-sdk/typescript#sdkusagereport)
4. `accountScope` is lowercase hex `sha256(provider NUL canonical home)`: local
   configuration scope, not a verified account; omitted when the home cannot be
   resolved.
5. each running gateway reads the enrolled machines at startup and once a
   minute using authenticated `GET /v1/claude-usage`. this route reads the
   answering machine's configured claude caches and includes saved accounts
   without a current profile. the plain profile route includes its bound accounts.
   the passive route and `skid claude-usage` start no native process. the
   helper reads the current login and the local saved record for the footer.
6. a claude profile carries `readState=ok` and `claudeAccount` for a verified
   login, `no_login` for a readable absent/null login, or `unavailable` for a
   failed read. an account id is lowercase hex SHA-256 of `claude-account`,
   a NUL byte and the native id. both routes carry its readings once in
   `claudeAccounts` and keep each fact's original fetch time. a missing/null
   array imports neither claude facts nor bindings. each machine retains
   account records and machine bindings under private `state/claude-quota/`.
   sign-out clears that home's binding. unreadable data preserves a binding
   only for the same profile key and home. the desktop chooses a verified
   local login, then agreeing verified peers, then an unambiguous remembered
   login. distinct accounts never merge.
7. a matching native measurement under 60 s old, including an empty one, is
   sufficient. otherwise a full usage read may start one attempt per account
   and host in configured home order. starts are at least one minute apart.
   failure or an unchanged measurement holds that account for five minutes
   from completion. all retries require new desktop demand. one attempt has a
   separate 30 s process budget. its gateway owns cancellation, group cleanup
   and join. stdout is bounded at 1 mib and discarded after checking the
   result. a build without live qualification or a home without verifiable
   policy eligibility stays cache-only.
8. the gateway runs the qualified native target directly with safe mode,
   restricted tools, no session persistence and the matching native config
   directory. it excludes host and hybrid managed policy. `auth status`
   verifies the selected first-party pro, max or team login and its expiry.
   claude applies the team's own server-managed policy, which may run
   organization-configured hooks. personal logins with a remote policy
   artifact stay cache-only. the child inherits neither alternate auth nor
   routing overrides. the gateway accepts only
   structured builtin completion with zero model use and a valid first or
   strictly newer cache measurement for the same login and settings. unexpected
   dispatch blocks that build across restarts. failure to write the block
   stops further native work in the running gateway and requires operator repair.
   diagnostics carry no terminal or account content.

### Pressure

`GET /v1/pressure` returns `{unsupported, current, history}`, sampled in process
every five seconds. Both platforms report CPU, normalized load, swap and disk.
Linux also reports memory available and CPU, memory and I/O PSI; Darwin reports
native system memory pressure (`Normal | Warning | Critical`). Linux's
`unsupported` set is exactly `memoryPressure`; Darwin's is exactly
`memoryAvailablePercent`, `cpuPsiSomeAvg60Percent`, `memoryPsiFullAvg60Percent` and
`ioPsiFullAvg60Percent`. `unsupported` is sorted, unique and constant for a running
gateway; `missing` holds only supported signals that failed; absent metric keys
equal the union of the two.

Warm/hot thresholds: memory available 15%/8%, disk 15%/5%, normalized load
1.0/2.0, CPU PSI some avg60 20%/50%, memory and I/O PSI full avg60 1%/5% on Linux;
Darwin's warning is warm and critical hot. Required inputs are disk and load plus
Linux memory and PSI or Darwin memory pressure; a missing required supported
signal yields level `Unknown`, an unsupported one never does. CPU and swap are display
only. Levels escalate at once and step down one level per continuous 60 s below
the held level. Pressure never blocks creation. History holds at most 180
chronological samples from the last 15 minutes; its last item is `current`.

### Directories

Listing and search admit what creation admits and cache nothing; neither starts
tmux, a shell, an agent or another gateway.

- `POST /v1/directory-listings {directory}` takes a browse token: exactly `~` or
  `~/c1/…/cn` with no empty, `.` or `..` component, at most 4,096 bytes of valid
  text; anything else is `InvalidRequest`. It answers `{machine, directory,
  parentDirectory?, children:[{directory, kind}], omitted}`: the token's immediate
  child directories, read through an `os.Root` at home so nothing escapes it. A
  symlink is listed as `SymbolicLink` only when its relative target resolves
  lexically inside home to a searchable directory; files, links to files,
  absolute or escaping links are skipped, and unreadable or unsearchable
  directories set `omitted`. Children sort ASCII-folded, then exact. More than
  4,096 scanned entries, 256 children, 32 KiB of path text or a 64 KiB response is
  `DirectoryListingTooLarge`, with no partial result; an unreadable token is
  `DirectoryListingUnavailable`. A listing is advisory: creation validates again,
  and an entry may change kind under it ([FIFO hang](issues/directory-listing-fifo-hang.md)).
- `POST /v1/directory-searches {terms}` runs the `zoxide` found on serve's `PATH`
  (`query --list -- <terms>`): 1–8 terms without whitespace, at most 256 bytes,
  under 1.9 s, stdout at most 64 KiB. It answers at most 64 existing searchable
  absolute paths in zoxide order within 32 KiB, `omitted` when any were dropped; no
  zoxide is `DirectorySearchUnavailable`, an oversized answer
  `DirectorySearchTooLarge`. zoxide's own bookkeeping may update its database on
  query.

## 10. Gateway wire and errors

Admission, in order, for every request: an unescaped `/v1` path; bearer
authentication; machine binding; no query; a declared body of at most 64 KiB; a
canonical path matching one route (anything else, `HEAD` and trailing slashes
included, is 400). `/v1/fleet*` and `/v1/local/*` go to fleet, which admits its
own; `/v1/notifications/*` go to the hub's notifier.

| Route | Section |
| --- | --- |
| `GET /v1/sessions` | [Inventory](#inventory) |
| `POST /v1/sessions`, `POST /v1/sessions/{tmuxId}/shell` | [Creation](#creation) |
| `PATCH /v1/sessions/{tmuxId}` | [Naming](#naming) |
| `PUT /v1/sessions/{tmuxId}/group` | [Groups](#groups) |
| `GET /v1/sessions/{tmuxId}/terminal` | [Attach and handoff](#attach-and-handoff) |
| `POST /v1/sessions/{tmuxId}/terminal/{op}`, `DELETE /v1/sessions/{tmuxId}` | [Terminal control](#terminal-control) |
| `GET /v1/terminal-contexts/{connectionId}` | [Remote context](#remote-context) |
| `POST /v1/directory-listings`, `POST /v1/directory-searches` | [Directories](#directories) |
| `GET /v1/profile-usage`, `GET /v1/claude-usage` | [account quota](#account-quota-and-usage) |
| `GET /v1/pressure` | [Pressure](#pressure) |
| `/v1/fleet*`, `/v1/local/*` | [Fleet](#fleet-interfaces) |
| `/v1/notifications/*` | [Notifications](#notification-interfaces) |

### Gateway wire invariants

1. Request bodies are `application/json`, decoded strictly (`internal/strictjson`):
   duplicate or unknown members, invalid UTF-8 and trailing data are 400, oversize
   413; a `null` member reads as absent. Responses are same-release data decoded
   tolerantly ([rules](rules.md#trust-boundaries)): an undecodable response is a
   lost one, so an observation shows that machine unavailable and a possibly
   dispatched mutation is `unknown`.
2. Each JSON document is declared once in Go: `internal/api` holds the inventory,
   session, terminal-operation, profile-usage and error documents and the vocabulary
   they carry (terminal status, usage, footer, name mode, recovery, account quota)
   with the host's status and recovery copy; pressure,
   directory, notification (`internal/attention`) and WebSocket
   (`internal/terminal`) documents belong to their producers. Kotlin declares its
   own readers by hand; there is no codegen, digest or schema lock. Optional
   members are omitted, never `null`.
3. Errors are `{code, message, dispatch?, target?}`. `not_sent` means no effect
   happened; `unknown` means one may have, and no client replays it. A session
   mutation route's error defaults to `not_sent` unless its handler says
   `unknown`; rename has no default, because its internal error can follow a
   naming effect. `target` appears only on a creation's `InternalError` with
   `unknown`, once the new session's lifetime and pane were captured; a lost
   creation response yields no target, and nothing invents one. Producers own the
   copy (`internal/gateway/errors.go`, `internal/auth`, fleet, notifier); clients
   show it and branch on codes.
4. Status classes: 400 malformed; 401 authentication, with
   `WWW-Authenticate: Bearer`; 404 absent session or context; 409 changed identity,
   name conflict, machine mismatch or refused terminal input; 413 oversize; 422
   domain validation; 503 terminal service, hub or notification state unavailable; 500
   `InternalError`, content-free, for every unmodeled defect.
5. The terminal WebSocket takes the inventory `identityToken` in
   `Skidbladnir-Session-Identity`, never a query. Client text frames are
   `Resize | Detach`, gateway text frames `Hello | Presence | Error`; binary frames
   carry PTY bytes both ways (`internal/terminal`). `TerminalConfigurationUnsupported`
   is a frame, not an HTTP error. One socket owns one tmux client; a write stalled
   for 5 s ends it, and the gateway pings every 10 s, releasing a peer silent for 5 s
   more; there is no replay or gateway scrollback, and any loss means reconnect.
6. Bounds that cross components: request body and terminal frame 64 KiB; geometry
   20–1024 × 5–512 cells. Feature bounds live with their owners.
7. Each request logs one `Request.Completed` with method, route pattern, status,
   duration and error code; no diagnostics enter HTTP documents.

### Error codes

| Code | Status | Meaning |
| --- | ---: | --- |
| `InvalidRequest`, `RequestTooLarge`, `Unauthenticated` | 400, 413, 401 | Admission |
| `MachineIdentityMismatch` | 409 | The machine header names another installation |
| `WorkingDirectoryInvalid`, `WorkingDirectoryUnavailable` | 422 | Creation cwd, by grammar or by existence |
| `ProfileUnknown`, `SessionNameInvalid`, `ObjectiveInvalid`, `GroupInvalid` | 422 | Creation, rename or group input |
| `SessionNameConflict`, `SessionNameChanged` | 409 | Naming |
| `SessionIdentityMismatch` | 409 | The token names another lifetime |
| `SessionNotFound` | 404 | No such session |
| `TerminalServiceUnavailable` | 503 | inventory/create cannot reach an admitted managed terminal runtime; creation is `not_sent` |
| `TerminalTargetChanged`, `TerminalUnavailable`, `TerminalInputBlocked` | 409 | Terminal control; a blocked input's message names its reason (`dialog`, `draft`, `initial_not_ready` or `unknown`); there is no reason member |
| `TerminalContextUnavailable` | 404 | No live registration |
| `DirectoryListingUnavailable`, `DirectoryListingTooLarge`, `DirectorySearchUnavailable`, `DirectorySearchTooLarge` | 422 | Directories |
| `NotificationsUnavailable` | 503 | Hub notification state |
| `InternalError` | 500 | An unmodeled defect |

Token admission order: a malformed ID is not found; a malformed or foreign token
is a mismatch; no server or an absent ID is not found; another server is a
mismatch. Create, shell, group, delete and terminal routes answer errors as
`dispatch:not_sent` unless an effect may have run: possibly written input,
possible deletion, accepted creation and failed group writes are
`dispatch:unknown`. Fleet codes are listed under [fleet](#fleet-interfaces).
shell-here retains its exact source's not-found/mismatch precedence; direct terminal
creation and inventory report terminal-service unavailability as 503.

## 11. Desktop clients

the desktop uses the same gateways as the phone. there is no desktop daemon or
desktop state on disk. each run reads gateway inventory, the hub's notification
state and `fleet.json`. the browser also reads an open row's screen on request.

| Owner | Owns |
| --- | --- |
| `internal/fleetclient` | Peer routing from `fleet.json`, one HTTP/WebSocket transport, references and handles, tolerant decoding, handle resolution, remote-context resolution |
| `internal/fleetview` | The shared status projection (`ProjectStatus`) and the shared copy (`ErrorMessage`, `WriteText`, `RecoveryNotice`, `GroupHeading`) |
| `internal/agentcli` | `skid` argument parsing (the only request validator), help text, human and `--json` rendering, exit codes |
| `internal/sessionui` | The browser: one Bubble Tea model holding peers, one selected view, one cursor, one fold and at most one pending operation |
| `internal/terminalclient` | Lending the local TTY to one exact session over the terminal WebSocket, and acknowledging that visit |

### Desktop concepts

- **Peer**: one fleet machine as `fleet.json` lists it: label, handle, origin,
  bearer. The roster follows `fleet.json`, re-read every tick; clients never write
  host state.
- **Row**: one session with its peer's label, handle, inventory `observedAt` and
  availability (the peer's current inventory succeeded).
- **Execution context**: where a row's work runs: `local` (this host's pane cwd and
  agent), `remote` (the host that answers for an SSH/mosh connection chain) or
  `remoteUnknown`.
- **status view**: the host's sign and words with browser overlays. a failed
  host's rows read `—`. checking holds the sign and queue place while the host
  is re-read. the hub supplies ready over an idle sign.
- **guest**: the selected lifetime when fleet inventory still lists it but its
  group or queue view excludes it. it keeps live facts and action admission.
- **captured target**: a confirmation or editor retains its exact reference,
  session name and listing host. an open details fold follows the cursor.

### Desktop invariants

1. At most once. A write is sent at most once: no retry, no redirect, and requests
   carry no `GetBody`, so net/http cannot resend. Once a write may have left, any
   failure without a definite gateway rejection is `dispatch: unknown`. No CLI or
   browser path repeats an unknown write; a later deliberate command is a new
   write.
2. Exact targets. Every effect is addressed by a captured reference. Names, labels,
   row positions, rendered or shortened paths and provider text never become
   identities, mutation inputs or inferred facts. folds keep captured targets
   through refresh, rename, reorder and pane change. an unavailable or replaced
   lifetime disables its action. the gateway rechecks lifetime and pane.
3. One resolution. A handle resolves once per invocation over one complete
   inventory of its scope (`--machine` narrows it). An exact reference needs no
   inventory, except `info`, which reports its row.
4. Tolerant same-release reading. Responses decode with plain `encoding/json`:
   null is absent, unknown keys are ignored. The client checks only what it
   branches on: the answering machine handle, IDs that form a reference, and the
   closed enums (status, name mode, provider, read state, write and close
   outcomes). A failed check rejects the whole document as `protocol_error`, a lost
   response: the peer reads unavailable and a possibly sent write is unknown.
5. availability. attachment needs a listed lifetime and no failed read from
   its listing host. a checking row admits attach and local inspection. effects
   need a current successful host inventory and any hub read still owed for that
   row. a write fences its host's reads until the post-write inventory. a reply
   sampled before the write cannot restore eligibility. a typed access or machine
   identity rejection invalidates its host immediately.
6. one read per stream. each peer has at most one inventory and one remote-context
   pass in flight. the browser has a hub read, a usage read and one screen read
   on explicit request. busy reads can owe one successor. ticks skip occupied
   lanes. inventory and remote context replies must belong to the current read
   and the same continuous roster membership. entering a terminal fences
   browser reads and clears the open fold.
7. fleet-wide views. views and suggestions use retained fleet inventory. failed
   hosts keep their last rows and a notice. a healthy host is checking before
   its first read, after the browser writes to it and after a terminal visit.
   a checking row keeps its sign and queue place. the caption and facts show
   `checking` while it fits.
8. ready is the hub's verdict for an idle row with the same observed foreground
   (provider, pid and start identity). after a visit a previously ready row
   checks until its host and hub have answered. a failed hub read removes ready
   and a visit's acknowledgement error leaves the browser's hub evidence intact.
   the cli keeps no notification state.
9. creation. `n`, `N`, `T` and `c` reveal and select the returned lifetime before
   attaching. from needs input or a non-matching group, adoption selects the new
   session's group. failed or unknown creation preserves view and selection, and
   a later attach to a created row does not repeat creation. a closing browser
   does not enter a session it created.
10. return and selection. detach keeps the view and cursor, prints the session
    and listing host, then refreshes. every drawn row has a retained lifetime.
    a close receipt with `terminal: closed` removes that lifetime immediately,
    independent of the interrupt result. a successful inventory replaces its
    peer's rows. a roster removal drops the peer's rows. a rebuild follows a
    surviving selection, including its guest if the view now excludes it.
    after removal, pick the first surviving successor in the old view order,
    then the nearest surviving predecessor. if only new rows remain, pick the
    first. an empty view has no selection. movement, a view change or adoption
    ends a guest. a view change keeps a common lifetime or starts at the first
    row. an old enrollment's close receipt can still prove exact loss after
    re-add. fence the replacement's earlier reads when adopting that receipt.
    restart begins at the first row of `all`.
11. Display text. Every external string is sanitized with `attention.SingleLine`
    and measured in display cells before styling. Styled fragments never pass back
    through sanitization, which maps ESC to a space. Colour never carries meaning
    alone. `NO_COLOR` keeps the signs and words and strips every style, including
    the work line and the Fable underline.
12. The TTY is lent, not shared. While attached the browser draws nothing and reads
    no input. Before returning, terminalclient cancels and joins both readers and
    restores the local TTY (cooked mode, remote-set presentation modes reset), also
    after transport loss.
13. Quota is evidence only: usage never supplies readiness, attention, admission
    or control.

### Desktop interfaces

- **Files**: clients read `fleet.json` (written only by serve) from
  `${XDG_STATE_HOME:-~/.local/state}/skidbladnir`; `--config DIR` selects another
  state directory. The creation default is the hub configuration's
  `default_machine`, else this machine. Opening the browser asks the local service
  to apply the hub's configuration (`POST /v1/local/apply`, 10 s, failure ignored;
  `skid status` reports it).
- **Gateway calls**: every request carries `Authorization: Bearer` and
  `Skidbladnir-Machine`; the terminal WebSocket adds `Skidbladnir-Session-Identity`.
  Request bodies are at most 64 KiB; responses at most 64 KiB, inventories 1 MiB.
  An operation has 15 s, a directory search 2 s, a remote-context pass 2 s, a hub
  call 3 s. No proxy; redirects are never followed.
- **browser screen** uses the existing exact-pane terminal `read` with a
  4,096-byte budget. the gateway supplies visible alternate screen or retained
  history. the same fold renders either as screen evidence.
- **Remote context**: for an SSH/mosh row the client follows the connection chain
  through `GET /v1/terminal-contexts/{id}` on every unfailed peer
  ([remote context](#remote-context), item 4).
- **Hub**: the browser reads `GET /v1/notifications/state` each tick; a visit sends
  `POST /v1/notifications/handled {slot, episode, readyGeneration}` for the row
  read at entry, and the hub ignores a stale capture
  ([notifications](#12-notifications)).
- **Terminal WebSocket**: the client requires a 20–1024 × 5–512 TTY, sends `Resize`
  before anything else and waits at most 15 s for `Hello` or `Error`. Binary frames
  are PTY bytes both ways; `Presence` is ignored. `ctrl-] d` sends `Detach`;
  `ctrl-] ctrl-]` sends one `0x1d`; every other byte, `ctrl-c` included, reaches
  the remote program. SIGWINCH resends `Resize`; SIGINT, SIGTERM and SIGHUP end
  the visit. Each write has 5 s and is never repeated.

### CLI

`skid --help` (or `-h`, or `skid help`; `internal/agentcli/run.go`) is the command
reference: a section for people, then the automation guide, which ends with every
command's own help. `skid help COMMAND` and `skid COMMAND --help` print that one
entry on stdout and exit 0; `skid help install|update|uninstall` runs that
command's own `-h` (`internal/install`), which owns its text. `skid --version` is
`skid version`, byte for byte; the installer parses it. install/update/uninstall
exit 2 for an owner action-required refusal and 1 for an installation failure;
cleanup failures remain failures. the bootstrap preserves the installer's exit.
dev-server keeps only a
pointer to `skid --help`. Ordinary commands are `list`, `info`, `enter`, `read`,
`send`, `text`, `keys`, `wait`, `stop`, `close`, `start`, `shell` and `group`,
beside the fleet commands of [§4](#fleet-interfaces) and `update`/`uninstall`.

- **Targets**: a handle is `t-` plus the hex of the first 8 bytes of SHA256 over
  `["skid-terminal-handle", machine, tmuxId, identityToken]`. It names a lifetime,
  not a pane, so it resolves to the session's current pane; collisions are not
  promised away, and a selector over an incomplete inventory refuses. `--ref` takes
  a full reference as base64url JSON, at most 4,096 characters, admitted only if it
  re-encodes identically; it pins lifetime and pane, so automation captures it once
  and reuses it. Names are creation input only; any other target text is
  rejected before any effect.
- **Output**: exit 0 only for a confirmed outcome; 1 for any failure, unknown
  delivery, partial list or close, or unmatched wait; 2 for invalid input or a
  missing TTY. `--json` writes one envelope, `{ok:true, result}` or
  `{ok:false, error{code, dispatch, target?}}`, with full refs; control results
  are bounded at 64 KiB and list at 1 MiB, over which a result becomes
  `output_limit` ([list limit](issues/list-json-output-limit.md)). Ordinary output
  uses handles and machine labels; `read` prints text on stdout and its metadata on
  stderr.
- **Groups**: `list --group LABEL | --unassigned` filters locally after decoding and
  never narrows the lookup; `start --group LABEL` and `group TARGET --set LABEL |
  --clear` send the typed draft; `--set` acknowledges `{group}` after `204`.
- **info**: one row's facts; `--explain` inspects the same target and prints the
  status, reason and rule IDs from that sample, replacing the row's status and
  clearing its usage and footer, which inspect does not carry.
- **wait** follows [terminal control](#terminal-control), item 4.
- **stop** interrupts; **close** interrupts, then closes regardless, reporting both;
  `close --terminal-only` closes without interrupting.
- **start**: `start [NAME] --machine HOST (--profile PROFILE [--model MODEL]
  [--effort EFFORT] [--stdin] | --terminal)
  [--cwd DIR] [--group LABEL]` creates once, then, with `--stdin`, inspects only its captured terminal every 500 ms
  within the one 15 s start budget and sends the prompt once, only when the
  terminal reads idle with no interaction or notice; a request or notice ends it
  `TerminalInputBlocked` with `not_sent`, and the gateway's `initialProfile` guard
  re-checks. An initial prompt is bounded by the terminal text bound, checked before
  creation and again before input, and never truncated. The result keeps creation (`not_sent | created | unknown`) and
  prompt (`not_requested | not_sent | unknown | <send outcome>`) as separate facts
  and carries the target and handle once creation captured them; a partial attempt
  exits nonzero with its facts. `written` proves terminal delivery, not provider
  acceptance; a cold startup can return `created` with the prompt `not_sent`.
- **Consumers**: jarvis or the owner decides reuse, steering and waiting. skid
  enforces no workflow, reservation or reply protocol, keeps no durable receipts,
  and never certifies task completion; longer waits and persistence belong to the
  caller.

### Views and the needs-input queue

Shared with the phone. Views are exclusive, never intersecting filters, in this
order: `needs input`, `all`, the observed named groups in `group.Compare` order,
then unassigned. A selected group stays while empty; a label appearing or
vanishing moves tabs, never the view. Built-in views and same-named groups remain
distinct: views are typed values, never indices or label text.

Needs input holds rows signed request, error, ready ([desktop](#desktop-invariants),
[phone](#phone-invariants)) or interrupted, in that order. Ties keep each client's
stable session order. A request or notice outranks a stored ready. Membership
and order derive from the typed sign. A failed host's rows leave the list and a
healthy host's rows keep their places while it is re-read. The `needs input` tab
reserves one cell for the first row's sign and leaves it blank when the list is
empty.

A row leaves when its last reason ends: ready when a visit's output acknowledges it
or fresh non-idle replaces it; a request or notice when a fresh observation no
longer shows it. A replaced foreground never inherits ready. Selecting, opening
a row, changing views and failed entry acknowledge nothing; opening a terminal never
answers a request or clears a notice; there is no manual dismiss; tmux visits
outside skid acknowledge nothing. `all` and group views keep group, then session
order, and include terminals and stale rows; only `all` draws group headings.
a selected browser guest can remain in a view it has left. group choices come
from complete retained inventory. A hub read
failure hides ready, keeps requests and notices, and shows
`notifications unavailable` outside every view. The queue is a human view, never
control admission: `wait --state needs-input` stays the explicit-request
predicate.

### Browser

the table and any fold share the body. the view strip sits above it. notices,
the caption and quota rule, the usage band and key hints sit below it. the
table scrolls only as needed to keep the selected row visible when selection,
inventory, view, window size or footer height changes.

**size.** the browser draws at every window size. lines fit without wrapping,
and from four columns the last column stays blank. a one-line window holds the
cursor's row. the caption arrives at 2 lines, the strip at 3, hints at 8, the
blank line under the strip at 12 and the blank before each later heading at 24.
the strip needs 6 columns and headings need 14. below 6 columns the strip's
lines go to the body.

a key that opens an effect fold or sends its action checks the current
window. `x`, `s` and their send keys require the whole question and effect.
`r`, `g`, `c`, `N` and `n` and their sends require their fields and controls,
with eight cells for a value. attaching, `T` and creation submits require
20–1024 columns and 5–512 rows before fencing reads. a creation that returns
outside that range selects the session and reports the refused attach. a fold
that loses room keeps its text, and its send rearms when it fits. a refusal
names the action and reason. navigation, inspection, draft input, closing a
fold, refresh and quit keep their ordinary key rules at every size.

**strip.** other group labels share a cap, from 24 to 7 cells. when that no
longer fits, the farthest groups hide behind counted `←n` and `n→`. as the
window narrows the wordmark goes, then `needs input` shortens to its sign,
then only that sign and the current view remain, with at least three cells for
the current label. the sign alone is the smallest strip. left and right clamp.
a view change keeps a lifetime present in the new view, otherwise selects its
first row.

**table.** from two columns the cursor's lip occupies the first cell and
every status sign the second. at one column the sign fills the cell. names
begin in the fifth column and arrive at eight columns. two spaces separate
lanes. after name, the lanes are group in needs input (at most 16 cells),
profile (at least the width of `unknown`, at most 16), model/effort (20) and
place (at least 24, and the rest of the line). names and groups measure all
listed rows, including retained rows. profile measures configured profile
keys. `all` and group views share starts. status, footer and agent exit leave
those starts fixed. the name target is the longest listed name capped at 40.
the lanes are admitted in order while leaving 32 cells for names, or the
target if smaller. place requires the full target. with no later lane, names
take the whole line after the spine. automatic names keep the head when cut.
manual names keep both ends when there is room for at least three cells.

place gives the directory's last component and a slash, with `/` for root and
`directory unavailable` when missing. it prefixes the execution host and a
colon only when that host's handle differs from the browser's. the host label
is capped at 12 cells. a cut directory keeps its tail. an unresolved remote
reads `remote context unknown`. stale rows keep place and drop group and agent
facts. only `all` draws group headings. the table keeps a heading with its
first row when both fit. an inline fold can cover the heading while it
keeps the cursor in place.

the caption names the session and listing host, then the status label and
its stale, checking or guest markers as applicable, then the host's detail.
nonempty facts from lanes that did not fit follow in order: group, profile,
model and place. position comes last while rows are hidden.
as room falls the caption gives up position, the place's head then the place,
model, profile, group, listing host, the name's tail then the name. the status
is cut from the tail last. dashes frame the caption from 12 cells of room.
weekly quota takes remaining room while the band is hidden. a foot draft or
terminal chooser names its selected host.

notices put the outcome first, then hub and host failures and retained recovery
notices. a checking host shows `checking inventory` only when it has no listed
row. a host label is capped at 16 cells and its notice is cut at the tail.
notices take lines while the body keeps two. with no line to spare, an outcome
takes the caption until the next key press.

**folds.** one fold at a time. space opens any selected row, including a stale
row or guest. inline folds stay beside that row, moving later lines down or
earlier lines up without moving the selected row or table scroll. their cap is
`max(1, min(16, (body − 1) / 2))` in whole lines. foot folds cover at
most 11/20 of the body's last lines, including the title. they can cover the
cursor but cannot cover a notice. fold text fits the window. an inline fold
draws nothing in a one-line body, and a clipped fold keeps its focused line
visible.

open shows handle, pane, directory, command and any attached clients, then the
sampled screen. `i` switches to facts, with state and availability first, then
full name, execution host, directory and handle. the short view counts facts it
cannot show. screen reads come only from opening, carry with tab or shift-tab,
or explicit refresh. an unavailable row's request waits for its own next
inventory and can start once while still on that row. closing, moving, entering a
terminal, changing panes, roster removal or access rejection retires the sample.
the fold shows the client's receipt age; a failed inventory dates retained
screen evidence and names the unavailable host. screen and status are independent
samples. the fold sanitizes terminal lines, drops blank and drawing-only lines,
and shows the latest lines that fit.

`x` and `s` open an inline question naming the captured session and host above
the literal effect. enter or `y` sends once if the captured lifetime is
available and the whole question and effect fit. escape, `n` and `q` cancel.
sending or canceling returns to the table. `r` and `g` open inline editors
from an open row. a write's draft and uncertainty survive until a
fresh read settles them. pending reconciliation blocks another edit. matching
fresh metadata resolves an acknowledged write or reports observation after an
unknown outcome, then returns to the open row. conflicting metadata keeps the
draft for review. a definite rejection cannot become success because a separate
writer matched. name edits retain their expected naming state. saving an unchanged
automatic title claims manual naming, while `ctrl-a` restores automatic ownership.

removing the subject collapses open details. a confirmation or editor moves to
a disabled foot fold naming the captured subject and reason. an unanswered
question says `nothing sent`. an editor keeps its frozen draft. a sent edit's
reply reports against its captured subject and ends checking if the subject is
gone. the disabled fold takes precedence in a one-line body. with room,
the successor or empty state also draws. escape dismisses the fold. while
idle, `ctrl-c` quits and `ctrl-r` refreshes. every other key and paste belongs
to the disabled fold and has no effect. reappearance cannot rearm it. a seeded creation draft
becomes standalone and keeps its fields, focus and any valid search.

`n` opens the terminal chooser at the configured default. it cycles every
configured machine and sends terminal, home and the selected view's group only
to an available host. `T` creates beside an available local row, asking the host
to sample cwd and group. `N` opens a foot draft. `c` seeds an inline draft from
an available local row's listing machine, group and cwd. the recorded launch is
eligible while that machine declares it. the shell and a declared profile named
`terminal` are distinct choices. a shell with neither agent nor recorded
launch seeds terminal. a missing launch needs a choice before create. a seeded
draft with a known launch opens on create.

drafts keep machine, launch, name, directory, group and create in focus order.
blank name follows the terminal title. the directory chooser first offers the
draft value, home, and up to four directories from that machine's sessions by
frequency. typed absolute paths and home paths are literal. other text starts
a directory search after 150 ms. left and right preview values, tab or enter
accepts and advances, shift-tab returns without accepting and `ctrl-u` clears
to home or unassigned. edits, machine change, focus loss and cancellation retire
old search replies. group suggestions use observed labels. the group editor's
`ctrl-s` saves the typed draft. a focused text field shows the tail of what
was typed.

**keys.** the same entries drive dispatch, hints and the `?` map. the map shows
session, create and browser keys, attach chords, open-row keys and a sign
legend. it names the colour path when there is room. unavailable entries are
quiet. an admitted listed key closes the map and acts. the hint line takes
admitted entries by rank when they fit and ends with its kept close or quit
entries, which leave from the front when they alone do not fit. tab and
shift-tab wrap through rows that need input and carry an open fold.
movement keys close an open fold before navigating. arrow and letter keys inside
an editor or chooser belong to that field. escape closes a fold, returning an
editor to open. `q` quits from the table, closes open, confirm, terminal and
keys, and types in a text field. `u` shows and hides the usage band on the
table, in an open row and in a confirm. it waits for the terminal's colours
as attach does. showing it requires the whole grid and margin, 80 columns
with Claude or 71 without, and room for three band lines. hiding it needs
no room. the band starts hidden in each run and only `u` changes it.
`ctrl-r` refreshes inventory and usage in every fold and reads the open row's
screen. `ctrl-c` quits from any fold. a write in flight refuses every key until
its result lands. see `?` for the
remaining keys and the [design language](design-language.md#17-terminal-browser)
for the palette.

### Desktop usage

- Inventory supplies Codex terminal evidence every five seconds. The browser
  reads each host's profile usage once a minute while it owns the screen and on
  `ctrl-r`, whether the band is open or closed. `u` changes visibility.
- Values show remaining capacity: 100 minus the whole part of the used
  percentage, stopping at 0 when a provider reports more than 100% used. A `~` marks a value
  read from a terminal.
- Codex chooses one whole observation per profile: a native report with an
  unfaded window, then terminal evidence with an unfaded window, then the
  retained native report, then terminal evidence. Per host, the matching local
  session with the most known windows supplies terminal evidence; ties go to the
  smaller numeric tmux ID. Across hosts a native report with a window still
  ahead beats one whose windows have all passed; among those ahead a fresh one
  beats a stale one; then the latest client receipt wins; terminal evidence takes the latest
  receipt. Ties go to the smaller machine handle. A host whose access was
  rejected offers no native report; a host whose inventory failed offers no
  terminal evidence. Native age is `observedAt − reportedAt` plus client
  monotonic time since receipt, and resets count down from host timestamps plus
  client monotonic time. A profile with no observation reads `unavailable`
  after a failed read and `checking` before any read.
- Claude remembers six independent facts per account. The amount and absolute
  reset time for 5h, general 7d and Fable 7d each take their newest measured
  value across machines, ordered by Claude's fetch timestamp. If fetch times
  tie, the larger percentage or later deadline wins. A partial reading updates
  only the supplied facts. An old cache, disconnect or restart keeps the last
  reading. A newly measured zero is a value. Each reset counts down against the
  desktop's wall clock, and only another measurement can change an amount or
  supply a new reset. A Claude profile reads `unavailable` when its account is
  unknown or conflicting, has no remembered reading, or, with no remembered
  binding, its host's read failed, and `checking` before the first usage read lands.
- A window fades when its reset has passed. A Codex window also fades when its
  native report is stale (an hour old, or a failed host or profile read) or its
  terminal evidence is an hour old. Claude windows never fade by age. A faded
  window keeps its number, gauge cells and countdown, drawn quiet. A passed
  reset draws no stem, and its countdown shows its age. A profile's name turns
  quiet when its row has at least one window and all of them fade.
- The band is one grid at every terminal width, 78 cells with the Fable column
  and 69 without, one line per profile:

  | Column | x | Width | Align | Header |
  | --- | --- | --- | --- | --- |
  | Name | 0 | 11 | left, `…` | `usage` |
  | Weekly gauge | 13 | 24 | | |
  | 7d | 38 | 5 | right | `━ 7d` (`═ 7d` plain) |
  | Fable | 45 | 7 | right | `  fable`, its first cell a sample of the Fable underline |
  | Weekly countdown | 54 | 7 | left | `┴ reset` |
  | 5h | 64 | 5 | right | `5h` |
  | 5h countdown | 71 | 7 | left | `reset` |

  A right-aligned header ends at its column's last cell; a left-aligned one
  starts at its x. The Fable column exists when the band's profiles include a
  Claude profile; without it, later columns shift nine cells left. Rows are the
  declared profiles (inventory, then profile-usage reports, then remembered
  Claude bindings), so readings arriving or lapsing never move the grid. A
  missing window is blank; a window with an unknown amount prints a quiet `—`.
- The band fits in half the body left by notices and the fold. Profiles past
  its cap become one overflow line. When fewer than three lines fit or the
  grid and margin are too wide, the band keeps its state and stays hidden.
- The weekly gauge has 24 cells. General 7d left fills `round(left × 48 / 100)`
  half cells, at least 1 above 0 and at most 47 below 100: heavy `━`, `╾` where
  it ends mid-cell, light `─` for spent. The plain path fills
  `round(left × 24 / 100)` whole cells, at least 1 above 0 and at most 23 below
  100, heavy as `═`. The weekly reset is the general window's, else Fable's.
  While it is ahead, its stem sits in cell `clamp(⌈time left × 24⌉ − 1, 0, 23)`,
  time left being the share of a seven-day window still to run: `┷` on heavy,
  `┵` on the half cell, `┴` on light, `╧` on plain heavy. An unknown general
  amount leaves a blank track with its stem.
- The Fable underline runs beneath the weekly gauge on its scale.
  It spans `round(fable × 24 / 100)` cells from the left, at least 1 above 0
  and at most 23 below 100, none at 0 or without a Fable amount. On eligible
  plans, the included Fable allowance is half the weekly limit and spends from
  it ([Anthropic's plan terms](https://support.claude.com/en/articles/15424964-claude-fable-models-on-your-plan)).
  Each point of general left funds two of Fable. While Fable is at most twice
  the general amount, the underline is single throughout. Beyond that it is
  single for `round(2 × general × 24 / 100)` cells, bounded alike and capped
  one cell short of the underline's end. The remaining cells are dotted for
  the unfundable part. With the general amount unknown it is single throughout.
  It is gold, quiet when Fable's window fades. It runs under every glyph in its
  range, including blanks, stems and the reveal's marks. The 7d window, weekly
  reset and reveal determine the glyphs and their ink.
- Countdowns round down to whole minutes and show the two largest units:
  `2d 4h`, `3h 12m`, `41m`, `<1m`. A passed deadline shows its age: `<1m ago`,
  `12m ago`, `3h ago`, `2d ago`, `5w ago`, else the year and month. A countdown
  wider than seven cells is cut. The weekly column prints one countdown, the
  weekly reset's. Countdowns are quiet. One turns ink when its period's general
  window is at 0% and neither that window nor the countdown's source has faded.
- `checking` and `unavailable` print as italic muted words at column 13, the
  rest of the row blank. A profile with neither a state nor a window prints a
  quiet `—` there. The selected row's profile is bold. With no profile the band
  is `usage` and the declaration state at column 13.
- The collapsed rule carries `7d quota left` and every profile's general weekly
  percentage when it fits, else the selected profile as `name 7d 72% left`. A
  row without a 7d amount reads `name —`, or `name 7d —` alone, and a state
  keeps its word. Its roles are the band's: an unknown `—` is quiet, a state's
  word italic muted, and a faded week's item quiet.
- The band redraws when a sample changes a drawn value and when the clock
  changes a countdown, a stem's cell, an age or a fade. Nothing else moves in it
  but the reveal.
- Showing the band with `u` on the full path starts the reveal. A sample or a
  return from a terminal never starts it. Time since `u` counts in frames of
  1/15 s. A gold chomper crosses each gauge with a known general amount from its
  last cell at two frames per cell, `◕` then `●`. Wholly spent cells ahead of it
  are quiet `╍`, except a stem. Cells behind it are settled. It holds `◕` for
  both frames on the first wholly spent cell and then leaves. A gauge without a
  wholly spent cell stays settled throughout.
- The frame depends only on elapsed time and the current samples. A new profile
  joins at the current position, and a changed number can add or remove a
  chomper. An exhausted gauge settles at frame 48 (3.2 s); the reveal's own
  tick chain ends at frame 49. Toggling the band or entering a terminal
  advances its generation and retires any outstanding tick.

### Desktop errors

- `dispatch` is `not_sent` before transmission, for client refusals and for known
  gateway rejection codes; a write's other failures are `unknown`. A body's own
  dispatch wins. An unknown error code is still the gateway's answer and keeps its
  dispatch; an empty code or unknown dispatch fails the decode.
- Client codes: `machine_unknown`, `unavailable`, `protocol_error`, `input_limit`,
  `output_limit`, `handle_not_found`, `handle_ambiguous`, `inventory_incomplete`,
  `terminal_unavailable`, `readiness_timeout`, `cancelled`, `configuration_invalid`,
  `invalid_input`, `input_unavailable`; gateway codes pass through
  ([error codes](#error-codes)). `fleetview.ErrorMessage` owns the wording both
  clients print.
- A create whose follow-up failed after the host created it reports `created` with
  the new handle; uncertain creation reports `unknown`, to inspect, never relaunch.

### Desktop accepted limits

- While attached the browser's chrome and status are gone; navigation needs detach.
- `ctrl-c` is refused in flight for up to 15 s; a held `ctrl-c` then quits as soon
  as the outcome lands, possibly unread, and a restart keeps no record. After `n`,
  `N`, `T` or `c`, a creation attaches if the window fits. Detach before
  quitting. A reflexive `ctrl-c` after `ctrl-] d` quits skid.
- Duplicate suppression lasts only while an action is in flight; a later `n`
  creates another terminal. There is no exactly-once creation or remote
  cancellation.
- `n` restarts at the default and costs an Enter even with one machine;
  unavailable machines stay in its cycle; first creation waits for inventory, with
  no optimistic bypass.
- a longer listed name or group can move or drop lanes for every row, as can a
  longer configured profile when a host first reports it. admitting a lane
  can cut more names as the window widens. a name longer than 40 cells is cut
  wherever a later lane fits. below about 18 cells, cuts can make names look
  alike. directories repeat within a group. a home reads as its last path
  component. rows show execution place while the facts retain the full path
  and context.
- a returning browser holds signs while it awaits observations and admits
  attach and local inspection. an external close waits for a successful read.
  a guest keeps context after leaving a filter. reorder can move the cursor with the selected lifetime or its old successor.
- a foot fold can cover the cursor. short windows clip facts, field notes and
  key maps. unfocused draft values may look cut and are sent whole. a narrow
  window needs more height to show a whole question within its cap. for a
  26-cell name, closing takes 23 lines at 40 columns or 13 at 80. a refusal can
  scroll the table and its explanation can be cut, disappearing at one line.
  at one column under `NO_COLOR` the cursor has no mark. a window-size message
  can rearm a send before the next repaint.
- The usage band takes body lines while it shows. At 80×24 with four profiles
  the header and rows take five lines. Taller bands leave less room for an
  open fold. Showing the band can scroll the table and make a question too
  tall to send. With two notices and the band at 80×24, `c` and `N` need
  more room. Hiding the band does not scroll back. Profiles past its cap can
  include the selected row's own profile, which stays hidden with the others. Names sharing their first ten cells can read alike. `u` cannot
  hide the band from an editor, a draft or the terminal chooser. On the table,
  `q` quits with it shown. Opening it uses the last known readings; Claude's
  cache can predate the last gateway poll. The countdown displays whole
  minutes, so `40m` can mean 40 m 59 s. The all-profiles rule is longer than
  the selected form and gives way first. With the Fable column the grid fills
  all 78 cells, so a wider column needs a new layout. The selected profile's
  bold is its only link to the table, and it vanishes under `NO_COLOR` with
  every fade.
- Staleness is only a fade: under `NO_COLOR` a stale or lapsed reading looks
  current, though a lapsed one still shows its age. While a reading is kept, a
  failed usage read shows only as that fade for Codex; a kept Claude reading
  shows no sign of it.
- Fable's own reset prints only when general has none, so a diverging one is
  hidden.
- The stem sets quota left against time left, an even-spending norm for work
  that comes in bursts: a fresh window reads behind after one burst. It
  resolves about 4.2 points and states no rate or forecast, though the eye will
  read one. The 5h window has no stem.
- The Fable underline resolves whole cells, about 4.2 points. The full path's
  general gauge resolves half cells. One reserved dotted cell can stand for an
  excess of a single point. Fable at 0% and a row without Fable both draw no
  underline. The Fable column distinguishes `0%`, blank and `—`.
- The dotted boundary represents twice the general amount left, a point the
  gauge leaves unmarked. It assumes the included Fable allowance is half the
  weekly limit. A promotion or plan change can move the funded boundary.
- The full-path reveal repaints fifteen times a second for 3.27 s. The font
  supplies its discs. Another font may draw `◕` and `●` larger, smaller or not
  at all. In MesloLGS NF their sizes differ, so the chew can read as a pulse.
  The design language's §1.4, §1.6, §12, §14, §15 and gold's row stand as
  written against it.
- one screen read costs a direct peer call and a bounded tmux capture, on a key.
  a retained idle tail can consist mostly of the agent's composer. usage reads
  each minute while the browser has the screen.

- Group views hide other groups and do not sort by urgency; labels share one cap,
  can read like built-in views, and may differ only in invisible characters.
- Ready and the queue are sampled inference: unknown can hide a request; a notice
  persists after inspection until the screen changes; a deliberate interruption
  qualifies; an agent exit raises nothing ([issue](issues/agent-exit-attention.md));
  an empty queue is not health. Tie order differs between clients.
- claude's global config is a provider-owned file with a private cache schema.
  its contents can remain old until claude fetches usage again. the footer
  floors used percentages and the desktop shows their complements. ordering
  uses the fetching machines' clocks. the reset countdown uses the reported
  deadline and the desktop clock. codex
  whole replacement can discard an older complete window. countdown and age
  updates can lag by the browser's five-second tick.
- the first frame uses the plain palette, with a 100×30 layout until the
  window reports its size. a terminal that does not answer the device query
  can delay first attach by one second. a later reply can reach an attached
  session. the browser samples the theme once, so theme changes need restart. on the full path the pigments are fixed by the design language.
- The plain path keeps faint and bright yellow. Faint varies by terminal and
  vanishes under Mosh. Bright yellow can be weak on light themes. `▌`, `←`, `→`,
  `…`, `●`, `■`, `≡`, `·`, `—` and the band's `━`, `─`, `┷`, `┵`, `┴`, `═`, `╧`
  have ambiguous East Asian Width and can wrap a frame. Faint can dim the Fable
  underline in quiet cells of the gauge, including spent, blank and faded cells.
  A faded Fable underline takes the text colour.
- The work line is a double frost underline and the Fable underline is single
  or dotted gold. The terminal and intervening tmux determine whether each
  retains its style and ink, draws as a single line in the text colour, or
  disappears. A terminal that lacks SGR 58 support can read its colour
  parameters as other attributes, such as blink, faint or a stray colour,
  across the underlined run. `NO_COLOR` strips both marks and the header's
  underlined sample. The Fable column keeps its number. The font places the
  underline within the row and determines its gap below the general line.
- browser help points to the live key map. visits still join the bounded hub
  acknowledgement before returning the tty. the
  [restore resize race](issues/bubbletea-return-resize-race.md) remains open.

- A cancelled browser can race its input reader
  ([issue](issues/bubbletea-canceled-input-race.md)); SIGTERM can be ignored
  ([issue](issues/tui-sigterm-hang.md)).

### Desktop rationale

- One table with a strip of views: an agents sidebar would be a second rendering of the
  same rows with its own cursor; the strip gives left and right a visible referent
  and names views, never sessions. Digits or Tab cycling would name different
  views on different days, since tabs come from observed labels.
- folds preserve the table and put an answer or question beside its subject.
- one fixed cell for status leaves room for trailing columns. the glyph shapes
  survive `NO_COLOR`. a failed host's rows read `—` so a glance cannot mistake
  an old request for a current one. needs input puts requests and errors first
  because they can block work, then ready, then interruption, usually the
  owner's own stop. a notice tells how the turn ended and takes precedence over
  the owner's menu. the caption and facts keep the words near the cursor.
- a row prints the facts that distinguish it. the profile carries the provider
  association, and the host appears in place only off this machine. keeping
  headings as labels keeps changes to a row's facts local to that row.
- a small pane still shows sessions. effects need room for the controls or
  confirmation the user acts on.
- usage is a band, not a fold: quota is evidence one keeps in view while
  working, and a fold is the one fold open, takes the keys and locks the
  table. the band is the rule's quota opened out, so it hangs under the rule.
- The band is a fixed grid. A column is one quantity and a missing window is
  blank, so providers differ only in which cells are empty. A reset belongs to
  its period: Fable's weekly reset has matched the general one, so it prints
  once.
- Only the week has a gauge. A 5h percentage measures a different allowance
  from the week's, so a shared track would invite a comparison it cannot
  support, and a five-hour window is spent in bursts rather than rationed.
- A cell can display one glyph and an independently coloured underline. Giving
  Fable the underline keeps the general gauge visible over their shared range.
  Their common scale lets Fable's end read against the stem.
- Escape never quits because Codex and Claude train a reflexive escape, often
  right after `ctrl-] d`.
- Styles are plain `x/ansi` values: lipgloss 2 would pull an ultraviolet revision
  that Bubble Tea 2.0.9 was not released with.

## 12. Notifications

The hub's notifier is the one owner of attention. It reads every machine's
inventory, keeps one record per session lifetime with the owner's
acknowledgement, projects records into one verdict per session and hints clients
when that projection changes. Clients reconcile native notices against it and
keep only one in-memory `alerted: slot -> episode` map per process
(`internal/notifier`, `internal/macnotifications/reconcile.go`).

```text
gateways (5 s per machine) -> hub notifier: records -> projection
  -> GET /v1/notifications/state     (every client)
  -> sse "data: changed"             (mac app)
  -> fcm {kind:"hint"}               (phone, optional)
<- POST /v1/notifications/handled    (dismiss, click, a visit's first output; any device)
```

### Attention concepts

- **Slot**: `{machine, tmuxId, identityToken}`, one session lifetime and the
  native notice ID; the slot plus its pane is the [reference](#2-concepts) a click
  or entry targets.
- A record's foreground is its [agent](#2-concepts) as `{provider, pid,
  startIdentity}`: one process lifetime.
- **Record**: phase `quiet | armed | ready`, `readyGeneration`,
  `attentionEpisode`, `cause` (`ready`, or `action` with a request and/or a
  notice), `handledEpisode`, `ackedReadyGeneration`
  (`internal/notifier/transitions.go`). Episodes and generations are observer
  revisions: monotonic, never timestamps, never served.

### Attention invariants

1. Evidence: non-idle is a request interaction, a menu, or activity starting or
   working; idle is activity idle with interaction none; everything else is a
   gap, and gaps (unknown, unavailable, outage) retain state. Only fresh
   terminal-source evidence of a known local foreground moves a phase; a request
   or notice is an action cause even when another dimension is unknown
   (`transitions.go`).
2. Ready means the same foreground was seen non-idle and later idle: available
   again, never proof of completion, a new answer or an answered request.
   First-ever idle is quiet; startup, a closed menu or a declined request can
   lead to ready.
3. Identity: pane, process or agent replacement resets a record; SSH/mosh agents
   are never local targets; names, titles, groups and profiles never identify.
   A complete inventory retires absent lifetimes; failed machines, unproven
   foreground absence and removed machines keep their records, so a re-enrolled
   computer resumes its acknowledgements.
4. An episode is allocated when a cause appears, changes or belongs to a new
   foreground. Action outranks latent ready; clearing a notice to reveal pending
   ready alerts again, acknowledged ready does not; renames never allocate.
5. Durability: a commit is saved before it is published; unchanged inventory
   needs no write. Every value a client holds was saved under a revision at least
   as large and a restart allocates above it. Startup
   marks every machine a gap until its first fresh read. A missing or
   other-fleet file starts empty and a new fleet deletes it; any other read,
   decode or save failure pauses attention and answers 503
   `NotificationsUnavailable`, never an empty state. The observer retries
   admission on its existing five-second tick while the gateway and terminals
   keep running (`observer.go`, `cmd/skidbladnir/standalone.go`).
6. Projection, per session of a fresh machine (`transitions.go`): `clear`
   when there is no cause, the episode is handled, or the cause is ready and its
   generation acknowledged; otherwise title, body and episode with `post` while
   qualified terminal evidence still shows the cause (ready: only while pending)
   and `keep` otherwise, which never posts and never removes. Ready is pending
   when the foreground is known, the phase ready, the status idle and
   `readyGeneration > ackedReadyGeneration`; only then the row carries
   `readyGeneration` and `foreground`. A gap machine has no rows, and its
   notices are neither posted nor removed.
7. Acknowledgement is the owner's, held by the hub. `POST handled {slot,
   episode, readyGeneration}`: each nonzero field applies only while it equals
   the record's current value, so stale acks are no-ops, and every well-formed
   body answers 204. Dismiss and click send the episode; a visit (`skid enter`,
   browser entry, the phone's terminal page) reads its row while dialing and,
   after the first nonempty output, sends that row's episode and ready
   generation. The state read runs beside dialing and never delays output; a
   detach before any output posts nothing; a failed read or ack is secondary
   (`skid enter` prints `notifications unavailable`, its exit unchanged).
   Acknowledging on any device clears everywhere; a watched session alerts like
   any other.
8. Reconcile, per client (`reconcile.go`): read the OS's delivered notices,
   then the hub's state. `clear` cancels a copy or an alerted slot; `keep`
   silently retitles a same-episode copy; `post` silently updates a same-episode
   copy, does nothing when `alerted` holds the episode, posts audibly when
   allowed, else cancels the older copy. Delivered notices without a row are
   cancelled unless their machine is a gap. An audible post, dismiss or click
   records the slot in `alerted`, so a process never re-alerts what the owner
   has seen, even when the ack failed. A failed read applies nothing.
9. The hub samples each machine on its own coalesced five-second lane with a
   five-second deadline; a sleeping machine delays no other (`observer.go`).
   The hub is a readiness dependency without failover. Clients show a pending
   ready only beside a direct idle row with the same foreground: direct
   inventory qualifies ready and never creates it.

Recovery rereads the actual file, reconciles current membership and push settings,
and durably saves before reopening with every machine a gap. Only fresh inventory
qualifies readiness; late reads from cancelled lanes cannot restore freshness.
Failed requests are never replayed: a replacement may fail after rename, so a
saved acknowledgement can survive a 503. Corrupt or unreadable state stays
untouched until repaired; missing or other-fleet state retains the startup rule.
Current settings are retained during the outage, including receiver revocation
and push changes. File operations remain synchronous under the observer mutex;
five seconds bounds retry cadence, not filesystem latency. Cancellation admits
no new recovery attempt and shutdown joins the observer's workers.

Failure logs `Notifications.Unavailable` with classified phase (`read`, `decode`,
`admit`, `write`) and known operation/errno, once per changed classification;
recovery logs `Notifications.Recovered`. No raw errors or paths. Unavailable copy:
`notification state unavailable; retrying automatically. check hub storage and
service logs if this persists.`

Content: the title is the session name with control, format and separator
characters replaced (`attention.SingleLine`); the body is `ready`, the request
phrase, the notice phrase or `request · notice` (`api/status.go`). No
machine, directory, profile, objective, question or transcript. Alerts share one
prominence and the platform's default sound and respect do-not-disturb.
Excluded: provider hooks, agent-exit detection, foreground suppression,
per-device acknowledgement, priority tiers, custom sounds, snooze, mute, inline
replies, history or replay, Linux popups.

### Notification interfaces

| Route (hub) | Contract |
| --- | --- |
| `GET /v1/notifications/state` | The projection; phones also get `receiver.status` |
| `GET /v1/notifications/events` | `text/event-stream`: `data: changed` at connect, per projection change and before closing on observer failure; `: keepalive` every 60 s; no IDs or replay |
| `POST /v1/notifications/handled` | Strict JSON at most 4 KiB; 204 |
| `PUT /v1/notifications/receivers/me` | Phones only, else 403; [Android push](#android-push), item 3 |

Requests pass gateway admission, then the notifier authenticates again to learn
the device kind ([issue](issues/notification-member-auth.md)); responses are
`no-store`. A member answers 503 `NotificationsUnavailable` `notification hub
unavailable; check skid status and service logs`.

Mac app: the signed `Skid.app` in the Mac archive, one instance per state
directory, run by its own launch agent. One loop syncs on stream hints, wake,
permission, setup, dismiss and click; a hint during a fetch discards that
result; the stream reconnects with 1-30 s backoff. `skid notifications setup`
asks it over `notifications.sock` to show its setup window. A click runs
`skid --config STATE enter --ref REF` in a new Ghostty window or Apple Terminal
`do script`, per the accepted `terminal`.

`skid notifications setup` exists only on macOS; elsewhere it exits 2.

Phone: one `needs-input` channel; tag = slot; sync after FCM hints, resume, the
foreground's five-second cadence and fleet changes (`NotificationOwner.kt`,
`NativeNotifications.kt`).

### Android push

Optional. The hub sends content-free FCM messages straight to Google with the
user's own Firebase project and credential; skid runs no notification backend.
Omitting `android_push` is healthy: no Firebase initialisation or registration,
no setup error, and pairing, sessions and Mac notices all work.

```yaml
android_push:              # public client options; setup: docs/phone.md
  project_id: your-firebase-project
  application_id: '1:123456789:android:0123456789abcdef'
  sender_id: '123456789'
  api_key: your-public-android-client-key
```

1. Public options reach phones in the fleet view as `androidPush {id, projectId,
   applicationId, senderId, apiKey}`; `id` is lowercase hex SHA256 of
   `skidbladnir.android-push.v1\nPROJECT\nAPPLICATION\nSENDER\nAPIKEY` and names
   the configuration everywhere (`hostconfig/config.go`). The private
   service-account export (`0600`, at most 64 KiB, matching `project_id`, Google
   token URI) never leaves the hub (`internal/fcmpush/client.go`); serve
   reloads it when the accepted options or the file's bytes change, at start and
   on `skid apply`.
2. One APK serves every project: no compiled Firebase identity; the phone
   initialises Firebase at process start from its fleet's options. Options are
   immutable within an Android process, so a change fences push and reports
   restart required. After a project change the phone unregisters before it
   registers again: a cached registration can report success while delivery
   fails (`FirebasePush.kt`).
3. Each phone has one receiver `{deviceId, floor, fid?, failed}`.
   `PUT receivers/me {configuration, generation, fid}` for the current
   configuration activates when `generation > floor` and is 204 for the
   identical active tuple; anything else, or push off, is 409 `ReceiverConflict`
   (`internal/notifier/receivers.go`). A configuration change erases
   destinations and keeps floors; removed members' receivers are deleted and
   cannot be recreated by registrations admitted before removal.
4. A message is data-only `{kind:"hint"}`, restricted to `dev.niels.skidbladnir`,
   four-week TTL, collapse key and priority `attention`/high when an episode was
   allocated since the last send, else `sync`/normal (`receivers.go`). Hints
   go on projection change, hub start/recovery, push reconfiguration and registration,
   never periodically. A hint only wakes the phone: it checks the sender against
   its options and schedules one bounded authenticated state read (expedited
   when high, three retries); only that read presents. No text, terminal data or
   fleet identity passes through Google.
5. FCM `UNREGISTERED` or `SENDER_ID_MISMATCH` clears a destination; another
   failure marks it failed until the next change retries. Hub receiver status:
   `disabled` (push off), `unconfigured` (no usable credential), `unavailable`
   (no destination or failed), `active`. A state read never clears a push
   failure.

New sends pause while observer state is unavailable. Each target is admitted
against current membership, sender, configuration and registration; an admitted
send may finish, but its result applies only to that same current tuple. Recovery
forces a hint even when the projection is unchanged; a recovery-only hint uses
normal priority.

### Notification accepted limits

- Five-second sampling misses brief transitions; inference can be wrong and has
  no grace period.
- Attention pauses during storage outages and can miss a complete busy-to-idle
  turn. Restoring an older file or deleting it loses history absent from that
  file; automatic recovery cannot reconstruct it or repair corrupt data.
- A notice removed without a callback (Android reboot) or dismissed with a
  failed ack re-alerts once in the next process.
- Ready during an open visit stays unacknowledged until the next entry or until
  the session goes non-idle; a visit clears that session's action notices
  everywhere while the request stays queued.
- A gap machine's acknowledgement reaches other devices at the hub's next read
  of it; clears reach a dozing phone at normal priority; the foreground phone
  sees changes on its five-second cadence.
- A failed FCM send retries only at the next change; force-stop, revoked
  permission or stopped Tailscale suspend delivery, and nothing is delivered
  before first unlock. Linux has no OS popups.
- Each entry adds one state read beside dialing; exit may wait 3 s for its ack.
- Admitted hand edits can yield odd verdicts. Decode or closed-set failures stay
  unavailable until owner repair; deleting the file discards attention history.
- Fleet and notification hints stay two streams: one merged stream would make
  member gateways refetch the fleet on every attention change.

## 13. Phone app

The phone is one Compose activity over the fleet's gateways. Its dashboard
shows every enrolled machine's tmux sessions in one collection, the forge
creates sessions, and the phone terminal attaches one. It owns no inventory:
every fact it shows is the latest admitted gateway answer, held in process
memory and rebuilt on start. Three constraints explain most of what follows:
the host owns meaning, so the phone renders and never re-derives; knowledge
ages per machine, so one machine's failure fences only that machine; and
nothing is replayed, so an unknown outcome is shown, not retried. Visual values
belong to the [design language](design-language.md).

### Phone concepts

- **Machine state.** Per enrolled machine: access (`ready | auth required |
  identity changed`), inventory (`reading | fresh | superseded | stale |
  unreachable`) and pressure (`reading | fresh | stale | unavailable`).
  Superseded is a fresh snapshot older than a mutation the phone just sent. A
  machine *can act* only while access is ready and inventory fresh.
  *Availability* is the one classification derived from access and inventory
  (`ready`, `refreshing`, `reading`, `stale`, `unavailable`, `auth required`,
  `identity changed`); every notice, card marker, sign overlay and forge affordance
  reads it.
- **Dashboard entry.** One owner, above the dashboard/terminal switch, of the
  selected view, the grid's scroll state and a pending saved restoration. It
  outlives the dashboard leaving composition.
- **View.** Exactly one of `needs input | all | named group | unassigned`,
  identified by type and, for a group, a label fingerprint, never by tab index
  or displayed text (a group named `all` is not the built-in view). Strip
  order: needs input, all, observed groups in label order plus a selected label
  no longer observed, an unresolved selected group, then unassigned when
  selected or observed.
- **Card.** One tmux session lifetime on one machine: machine handle, tmux ID
  and identity token. Its actions capture that lifetime.
- **Forge.** The creation sheet, entered through the Forge seal at the
  dashboard's bottom-end corner.
- **Machines disclosure.** One transient modal: every machine's pressure rail
  and notice, and one machine's pressure details.

Copy follows the [product language](#product-language).

### Phone invariants

1. The host owns meaning. The phone renders the host's status sign and copy,
   host-evaluated pressure levels and signal states, and host recovery copy. It
   overlays only what it alone knows: staleness, the hub's pending ready and
   notification availability. It has no pressure thresholds, decays no status
   by time and re-derives nothing the host sent.
2. A machine that cannot act keeps its last cards inert under a marker naming
   the reason (`STALE · actions disabled`, `AUTH REQUIRED · actions disabled`,
   ...). While it refreshes after its own write, its cards keep their signs and
   needs-input places. After a failed read or trust event they read `unknown`
   and the session sheet appends `· stale` to the label. Trust events (auth
   required, identity changed) use failure ink. An unreachable or stale host
   is degraded, because one host out is the federation's normal state. A
   pressure failure never fences actions.
3. One projection, `dashboardItems`, owns membership, order, render keys and
   restoration anchors. Sessions sort by machine label (root-locale lowercase,
   then exact), machine handle, numeric tmux ID. `all` places them under one
   heading per nonempty group in label order (ASCII-folded UTF-8, then exact
   bytes), unassigned last. A selected group or unassigned view has no heading:
   the tab names the scope. Needs input is flat. The host promises no order.
4. Needs input holds a session on a ready or refreshing machine whose host
   sign is request, error or interrupted, or whose host sign is idle while
   the hub holds its pending ready and notifications are available. The latter
   reads ready. Rows follow the [shared order](#views-and-the-needs-input-queue).
5. Every card shows its machine. Every action, menu row and confirmation
   carries the card's captured lifetime. A machine that stops being able to *act*
   while a menu is open disables that menu's rows in place; nothing retargets from a displayed label,
   and the controller re-checks admission when a late tap lands. A menu
   dismisses before it acts. Destructive rows follow a rule and are
   failure-toned; both closures confirm, and only the confirmation's commit
   button carries the [Cleft](design-language.md#6-shape-grammar).
6. Pull over the collection is the only manual refresh. It asks every live
   poller, whatever the view, for one inventory read that starts after the
   pull; a read already in flight cannot end it. Pressure is not refreshed. A
   gesture that begins on a recovery notice scrolls but never refreshes. The
   view strip, notices and Forge seal lie outside the gesture.
7. Drill-in never resets the dashboard. Terminal return (top `Detach` or back)
   lands on the same entry with the same view and grid state, so the keyed grid
   keeps the first visible item's place and a vanished item clamps. Saved-task
   restoration holds the [capsule](#saved-state) until no ready machine is still on its first
   read, then scrolls once before any card is shown. A terminal is never
   restored: recreation or backgrounding from it lands on the dashboard with no
   attachment.
8. Creation names an explicit machine and submits only the visible draft. A
   create failure is definite only when the gateway answers `dispatch:
   not_sent`; any other failure is an unknown outcome: the forge closes, the
   draft becomes a recovery held on the dashboard, and nothing is replayed.
9. A directory or group draft is the one editable value. Choices, highlights
   and replies never rewrite it; acceptance writes an exact domain value into
   it; only the separate create or save action submits it: a directory after
   the domain re-validates it, a group as typed. A directory reply is admitted
   only for the draft revision and machine that issued it. Browsing never
   builds a path: it sends only tokens the host returned.
10. Decoration is silent and subordinate. Marks and seals carry no meaning and
    no action, every screen is complete with them deleted, and nothing
    ornaments an error, notice or destructive surface.

### Phone interfaces and owners

- Gateway routes, each to one machine with its own bearer and pinned machine
  header, responses decoded tolerantly: `GET /v1/sessions` (every five seconds
  and on demand) and `GET /v1/pressure` (every five seconds) per live poller;
  `GET /v1/terminal-contexts/{id}` for remote context; `POST /v1/sessions`,
  `POST /v1/sessions/{id}/shell`, `POST /v1/directory-searches`,
  `POST /v1/directory-listings`, `PATCH /v1/sessions/{id}`,
  `PUT /v1/sessions/{id}/group`, `POST /v1/sessions/{id}/terminal/stop`,
  `POST /v1/sessions/{id}/terminal/close`, `DELETE /v1/sessions/{id}` and the
  terminal WebSocket ([§14](#14-phone-terminal)).
  Mutations run in the machine's serial lane and are never retried.
- Android: the saved-state capsule (below) and the notification-settings
  intent behind the header menu.
- Owners: `Machines.kt` machine state, polling, mutation lanes and awaited
  reads; `SkidbladnirController.kt` the UI state machine (forge, recovery,
  confirmations, access loss); `ProductModel.kt` availability, notices, the
  status overlay and copy; `Groups.kt` views, group choices and
  `dashboardItems`; `DashboardEntryState.kt` the view, grid state and capsule;
  the screen files presentation; `Theme.kt` and `Ornament.kt` tokens and
  frozen art.

### Dashboard

Order: the header (`Dwarves` led by the Hlíðskjálf mark, and a
`dashboard options` overflow holding `machines` and `notification settings`);
the view strip; one notice line per machine whose availability or pressure is
exceptional; the notification-setup notice with `set up notifications`, absent
while notifications are ready or disabled; the controller's failure notice; a
forge-recovery notice; then the collection. The Forge seal overlays the
collection's bottom-end corner and is lit while any machine can act; the
grid's trailing padding lets the last row scroll clear of it while other rows
pass beneath. Opening the header menu
or the machines disclosure requests nothing, filters no card and prefills no
draft. The strip scrolls horizontally and reveals the selected tab only when
the selection, its resolved label or the strip's width changes; polls never
move it.

The collection is one lazy grid of adaptive columns in every state:
full-span workspace-recovery notices first (failure-toned when broken, the
save time in the phone's zone), then the items or one empty state (`no
sessions currently need input in this view`; `no matching sessions in
available inventory` with each waiting machine's copy, failure-toned if any
machine's is; or `no sessions in this view` with the Hlíðskjálf mark). A
pull draws only the [refresh line](design-language.md#13-component-register),
determinate while pulling and indeterminate while the pulled read runs, absent
at rest; it reserves no height.

**Card.** Work first: the exact tmux name (two lines), then the status sign
beside the dwarf's display name, with the host's label and detail in needs
input, or the command for a local plain terminal. The availability marker,
`notifications unavailable` and the objective (two lines) follow when present.
The wrapping footer has one profile/host cluster and the directory:

| Context | Cluster | Directory |
| --- | --- | --- |
| Local | `[profile · ]<machine>` | `…/<last two>` of the pane cwd |
| Resolved remote | `[profile · ]running on <execution machine> · terminal on <machine>` | The remote cwd, else `directory unavailable` |
| Unresolved remote | `remote context unknown · terminal on <machine>` | None |

The profile is the host's configured label, else `<provider> · profile
unknown`. A plain pane reads `$` and has no profile. The row gets its profile
from the observed agent. The directory moves whole to a second line and
start-ellipsizes only when wider than a line. In needs input a last line names
the group (`group: <label>` or `unassigned`) and the status words wrap in full.
The terminal's session sheet shows the sign, label, detail and evidence. Signs
stay still. Inventory freshness has a clock.

One body tap opens the terminal directly, without an inventory read; the
gateway checks the lifetime at attach. Every other verb sits in the card's overflow,
in this order: `change group`, `send interrupt`, `close terminal only`,
`interrupt and close terminal`. The last three are the terminal-lifetime list
the terminal's session sheet renders too. The overflow is enabled while any
row is; its description is `session actions for <tmux name> on <machine>`.

**Machines disclosure.** `machines` lists every machine's pressure rail and
notice, `reconnect fleet` (which closes the sheet first) and `dismiss`. A rail
opens that machine's details in the same modal; back returns to the list,
then closes. Each machine keeps its own five-second inventory and pressure
polling; the disclosure adds no request.

**Pressure rail.** One disclosure surface: a header `<MACHINE> <state>`, one
non-wrapping, horizontally scrolling metric row in the fixed order `CPU MEM
SWAP LOAD DISK`, then the history band; colours are in
[design language §13](design-language.md#13-component-register). `MEM` is
Linux RAM available, else Darwin memory pressure; PSI is detail-only. Each
metric reads `<label> <value> <mark>` with literal marks `i N W H ?`, so colour
is redundant; only host-evaluated warm and hot spend Gold or Ember. A supported
signal that is missing keeps its slot as `NO DATA ?`; an unsupported one has
none. `UNKNOWN` names the first missing metric other than CPU and swap.
Presentation is total: an inconsistent snapshot renders literally
(`RECOVERING FROM NORMAL`, bare `UNKNOWN`) instead of failing. Details list
every supported metric with its word: informational, normal, warm, hot or no
data.

### Creation sheet

The forge opens only while some machine can act and no group editor is open.
Its draft starts with no machine and no launch choice, empty name, objective
and directory, and the selected view's group (empty for the built-in views; a
restored, still unresolved named view needs an explicit group choice before
submission). Changing machine clears the directory and an agent profile,
keeps a terminal choice, the name, objective and group, closes browsing and
drops a pending search. Creation from a remote pane's terminal opens the forge
on that pane's own machine with a terminal launch and its group.

**Directory field.** Exact empty means home (`~`). A draft beginning `/` or
`~` is a literal: the phone checks its grammar, the host its existence at
creation, and it never searches. Leading whitespace before a path,
whitespace-only and display-unsafe text are invalid and stay editable.
Anything else is 1–8 search words within 256 bytes and must be resolved to a
literal before creation. Empty input offers home, then the distinct cwd values
of the selected machine's local sessions (remote cwd never suggests a local
launch); a query offers the host's zoxide results 150 ms after the last edit;
`browse home` is always offered. Accepting a choice touches no filesystem.

**Group field.** Choices are the observed labels of every retained snapshot,
ranked against the typed text: exact, ASCII-case-insensitive equal, prefix,
substring, then ordered subsequence, ties in label order; only ASCII folds, and
matching never rewrites the draft. A nonempty draft without an exact match also
offers `use label: <text>`. An explicit `unassigned` choice (first when empty,
last otherwise) means no membership and is distinct from a group named
`unassigned`. Observations may be stale: a choice is typing help, and the
target's admission still decides save. An empty draft is unassigned. A `422
GroupInvalid` puts the host's message under the field, failure-toned, until
the next edit; accepting the unchanged draft is not one. Every other failure
keeps its slot. The desktop ranks the same way
after normalizing its draft to NFC; its keys are in the desktop section.

**Both fields.** Focusing one opens its bounded choices in the same sheet and
closes the other's; an edit returns the list to its top; tapping a choice fills
the draft and ends text entry; keyboard next accepts the directory's valid
literal or default, or any chosen group draft, and never picks a result or
submits. Back first hides the keyboard, then closes the choices, then the
sheet. Autocorrect and capitalization are off; paths render left-to-right.

**Browse home.** A secondary page of the Forge sheet on the selected machine.
It opens `~` and navigates only host-returned children and parent. A local
basename filter (at most 256 scalars) ranks exact, prefix, substring, then
ordered subsequence under root-locale lowercase, ties by shorter basename,
then server order. Dot-folders are hidden by default, with a toggle only when
some exist. Back history keeps up to 32 views with their filter, hidden flag
and scroll position. Loading keeps the previous rows and disables parent,
folders and use; failure reactivates retained rows; transport and internal
failures offer retry, unavailable and too-large listings offer `enter path`.
`Use` writes only the directory draft, clears a directory rejection and returns
to the field; back from the root, cancel and `enter path` return to the
unchanged field. Backgrounding settles a loading browse to its retained view
(or the form) and drops a search, with no retry, focus or keyboard.

**Outcome.** Success opens the new session's terminal and, unless the current
view already admits it, selects its group (or unassigned) at the top. A
definite failure keeps the draft with the rejection. An unknown outcome closes
the forge, marks that machine's inventory stale and holds a recovery notice
naming the one available repair: `Pull down to check again`, reconnect the
fleet, or reset the fleet outside the app. Once that machine reads fresh it
offers `Resume draft` or `Discard`. A forge still pending when the app leaves
the foreground becomes the same recovery.

### Saved state

`MainActivity` installs the entry into its saved-state registry once, before it
constructs the controller. The capsule is a primitive bundle under
`dev.niels.skidbladnir.dashboard-entry`:

| Key | Value |
| --- | --- |
| `version` | `4` |
| `viewKind` | `needsInput`, `all`, `unassigned` or `named` |
| `viewLabelSha256` | Named only: the label fingerprint |
| `anchorKind` | `none`, `session`, `space` (a group heading) or `unassigned` |
| `anchorSha256` | Session or space only: the fingerprint |
| `fallbackIndex`, `offsetPx` | First visible rendered index and pixel offset; both 0 without an anchor |

A session fingerprint is SHA-256 over `skidbladnir.dashboard-card.v1`, then the
machine handle, tmux ID and identity token, each preceded by its 4-byte
big-endian UTF-8 length; a label fingerprint is SHA-256 over
`skidbladnir.space-label.v1`, the label's 4-byte length and the label. The
capsule holds no inventory, raw label, name, path, token, draft, attachment or
credential, and nothing else persists dashboard state: no preferences, file,
tmux option or gateway field.

An unsupported version starts `all` at the top; a malformed current-version
capsule is a trusted-state defect and crashes. A restored named view whose
label is unobserved stays selected as `previously selected group` until an
inventory shows it. While restoration is pending the collection shows only a
progress mark, and a save re-emits the pending capsule unchanged. Restoring
scrolls to the anchor's current index, else to the clamped former index.
Selecting another view cancels a pending restoration; selecting the active
view does nothing. A return from a terminal that never presented output
verifies inventory as a pull does. Terminal access loss returns to the
dashboard at the top of the same view with that machine's notice.

### Phone errors

- A failed inventory read downgrades only its machine: fresh or superseded
  becomes stale with its snapshot, reading becomes unreachable. A failed
  pressure read: fresh becomes stale, reading becomes unavailable.
- `401` becomes auth required. `409 MachineIdentityMismatch`, or a success
  naming another handle, becomes identity changed. Either stops that machine's
  polling, fails its open forge with the rejection, drops its confirmation and
  group editor, posts a dashboard notice and closes its open terminal.
- A refresh that reaches no live poller posts a notice naming each machine's
  access state.
- Copy names its machine and claims nothing tmux did not report. Recovery copy
  names only a path that exists for the target's modeled access: `Pull down to
  check again` when reachable, `Reconnect fleet` for auth, fleet reset outside
  the app for identity.

### Phone accepted limits

- No screen-reader support: no curated narration, roles, headings, live
  regions or custom actions. TalkBack reads Compose's merged visible text and
  the descriptions of the two text-less controls (the card overflow and the
  Forge seal). Labels carry no bidi isolates, so an RTL name may reorder
  neighbouring punctuation; paths stay left-to-right.
- Pull has no tap, menu or accessibility equivalent.
- One dashboard context: no per-view position history. A cold launch or an
  OS-discarded task starts at `all`, top. A deleted anchor clamps its index,
  since no neighbour list is saved and so no inventory persists; offsets land
  approximately after layout changes; the strip's scroll offset and input
  focus are not restored.
- Machines and notification settings sit behind the header overflow.
- Card seals are not individually distinguishable; recognition is seal plus
  names ([design language §11](design-language.md#11-dwarf-seals-procedural-portraits)).
- Directory search covers zoxide-visited paths only; browsing covers Home only;
  links that escape Home are reachable only by literal entry. Listings are
  advisory: creation re-validates, and under concurrent change a listing may
  show an entry's earlier kind ([FIFO hang](issues/directory-listing-fifo-hang.md)).
- Label fingerprints leak equality and are dictionary-guessable. They keep raw
  labels out of the OS-held bundle; they are not secrecy.
- An invalid group label costs one round trip. A lone surrogate in a group
  draft leaves `GatewayClient`'s UTF-8 encoding as `?`, and the host stores that.
- Open: [sheet uniformity](issues/sheet-uniformity.md),
  [picker nullables](issues/picker-model-nullables.md),
  [interaction redesign](issues/phone-interaction-redesign.md),
  [visual redesign](issues/visual-redesign.md),
  [invisible label characters](issues/group-label-invisible-characters.md) and
  the phone lines of [unrun acceptance](issues/unrun-acceptance.md).

## 14. Phone terminal

The phone renders one tmux client of one exact session. Kotlin owns the
connection, credentials and Android surfaces; a packaged xterm.js page owns
rendering, input encoding, modifiers, touch, selection and grid fitting. They
meet only at the page port. Four constraints explain the rest: the window is
shared with every other client under tmux `window-size latest`, so any grid
change resizes it for everyone; the bearer never enters the WebView; nothing is
replayed; terminal content stays transient.

### Phone terminal concepts

- **Attempt**: one entry into the terminal screen for one `SessionTarget`
  (machine handle, tmux ID, identity token). It is numbered and owns at most
  one page, one connection and one notification visit (`TerminalAttachment`).
  Every page and connection callback names its attempt; stale ones are ignored.
- **Page**: `assets/terminal` inside `LockedTerminalWebView`, one per attempt.
  A failed page never recovers.
- **Page port**: the one web-message channel between native and page,
  internal to one APK.
- **Connection status**: `Preparing` (no page) → `Connecting` (page ready) →
  `Connected(attachedClients)`; any failure → `ReconnectRequired(message)`.
- **Viewport**: `Pending | Fitted(columns, rows) | TooSmall`, published by the
  page and held natively apart from connection status.
- **Text size**: one phone-wide nominal size, 12..24 sp in steps of 1,
  default 16. The bounds are product choices, not optima, and the default
  awaits the owner's readability check ([unrun acceptance](issues/unrun-acceptance.md)).

### Phone terminal owners

| Owner | Owns |
| --- | --- |
| `SkidbladnirController` | Attempts, admission (connection × viewport), leave, text-size writes |
| `TerminalConnection` | The wss: opening resize, frame order and bounds, the input queue |
| `LockedTerminalWebView` | WebView lockdown, the port decoder, the output budget, IME requests, selection haptic and too-large toast |
| `TerminalSelectionToolbar` | The floating `Copy` action mode, its overlay Back callback, the one clipboard write |
| `terminal.js` | xterm, fitting, the input reducer and modifiers, deck encoding, paste sanitizing, IME containment, touch, wheel and tap routing, selection ranges and snapshot |
| `TerminalScreen` | Insets, rail, sheets, overlays; mirrors page snapshots |
| `TerminalTextSize` | The committed size file |

### Phone terminal invariants

1. At most one attempt is live; entering a terminal leaves the previous one.
   The phone names a session only by `SessionTarget`, never a raw tmux target;
   the gateway checks the lifetime at attach.
2. Leaving (detach, Back, backgrounding, page or connection failure, loss of
   the machine's authority, closure, another entry) closes only the owned
   client. Nothing is buffered or replayed; re-entry is a fresh attempt with a
   fresh page and empty scrollback. Activity or process recreation, including
   a font-scale change, lands on the dashboard and never reopens a terminal.
   Rotation and keyboard changes keep the attachment (the activity handles
   those configuration changes) and only refit.
3. The bearer, machine pin and identity header stay in Kotlin. The page has no
   network, file, storage, navigation or JavaScript-bridge capability.
4. One ordered ingress: typed, IME, dictation, paste, deck, wheel and tap
   reports and xterm's automatic replies all leave the page as `Input`
   messages, in order, and the connection sends them unchanged.
5. The page publishes only whole-cell grids fitted to its measured box in the
   loaded font, capped at 1024 × 512 and never raised to a floor. The wss opens
   only after page `Ready` and the first fitted grid, which is its first client
   frame. Below 20 × 5 the page keeps its last grid and publishes
   `ViewportTooSmall` once per episode; recovery republishes `Resize` even when
   unchanged.
6. User ingress (touch, focus, typing, deck) needs `Connected` and `Fitted`.
   Output and xterm's automatic replies never wait for either.
7. Chrome never changes the terminal's box. Only the keyboard, rotation,
   window size and font scale do; text size changes only the grid. The rail is
   one row whose height depends only on font scale.
8. The page alone owns Ctrl, Alt and Shift, each `Off` or `Armed`, and
   publishes all three in one `ModifierState`: all-off before `Ready`, all-off
   before the `Input` that consumes armed ones, never an unchanged snapshot.
   Every `Input` consumes all three. Reset emits no input and happens on page
   failure, disable, window-focus loss, rotation, opening a sheet or
   confirmation, sending an interrupt, and leave. Compose mirrors the current
   attempt's snapshot and never toggles optimistically.
9. `Detach` destroys nothing. Closures exist only in the session sheet and
   commit only through the confirmation.
10. The page owns, before xterm, every touch stream that starts on xterm's
    screen and every stream while a gesture or selection exists. A stream it
    cannot own whole (a second contact, a non-cancelable or edge start, a start
    during a claimed gesture) is swallowed until every finger lifts and emits
    nothing. Touch never fails the page.
11. Each wheel unit takes exactly one route; at most one remote unit leaves per
    animation frame, and nothing after release.
12. Selection emits no terminal input. Only an explicit `Copy` writes the
    clipboard, at most once per selection, with exactly the release snapshot.
    Selected text leaves the page only in `SelectionAvailable`. Production
    never reads the clipboard.
13. Terminal bytes, keys, touch coordinates and selected or clipboard text
    never enter logs, saved state, analytics or evidence.
14. The text size shown and applied is the committed one. Orientation,
    keyboard, machine and session never rewrite it. Android text scaling is
    applied once, natively (`applyDimension(SP) / density` CSS px, with
    `textZoom` 100 and zoom off).

### Terminal connection

`GET /v1/sessions/{tmuxId}/terminal` over wss, with the bearer, the machine pin
and `Skidbladnir-Session-Identity: <identityToken>`.
[Attach and handoff](#attach-and-handoff) owns the host side.

- Phone → gateway: text `{"kind":"Resize","columns":c,"rows":r}` within
  20..1024 × 5..512, first and within 5 s; binary input in frames of at most
  64 KiB; `{"kind":"Detach"}` then close 1000 on leave. More than 1 MiB of
  queued input fails the connection.
- Gateway → phone: `Hello{attachedClients,paneId,foreground?}` once and first,
  then `Presence{attachedClients}` and binary output of at most 64 KiB, and at
  most one final `Error{error{code,message}}`.
- The phone decodes each text frame once, tolerantly
  ([rules](rules.md#trust-boundaries)), and reads only `kind`, `attachedClients`
  and `error.code`: `Hello` and `Presence` both report the count, and an unknown
  code reads as `InternalError`. It re-checks no frame order, count or size.
- An undecodable text frame, an unknown kind, a missing read field, a close, or
  a transport failure is `ReconnectRequired`.

### Page port

Native posts the handshake as a window message carrying the one port:
`{"longPressMilliseconds":n,"fontSizeCssPx":x}`
(n from `ViewConfiguration`, a positive integer; x finite and positive).

| Native → page | Meaning |
| --- | --- |
| Binary `ArrayBuffer` | 1..65536 output bytes, posted as they arrive |
| `Accessory{key}` | One of `Escape Slash Shift Home Up End PageUp Tab Control Alt Left Down Right PageDown` |
| `ResetInputState` | Reset modifiers and gesture, dropping any selection |
| `FontSize{fontSizeCssPx}` | A committed text size |
| `ClearSelection` | The toolbar closed or could not open; drops only a released selection |

| Page → native | Meaning |
| --- | --- |
| `Ready` | Port bound; once |
| `Input{value}` | At most 1 MiB of UTF-8 |
| `Resize{columns,rows}` | A fitted grid |
| `ViewportTooSmall` | Below 20 × 5 |
| `ModifierState{control,alt,shift}` | Each `Off` or `Armed` |
| `ImeRequested` | A tap asks for the keyboard |
| `OutputApplied{bytes}` | xterm wrote one frame; one per frame, in order |
| `SelectionStarted` | A long press selected; native plays the long-press haptic |
| `SelectionAvailable{anchorX,anchorY,text}` | The release snapshot, at most 256 KiB; anchors in [0, 1] of the window |
| `SelectionCleared` | The page dropped a selection it held |
| `SelectionTooLarge` | The release text exceeded 256 KiB and was cleared |
| `PageFailure` | The page is dead |

Both ends ship in one APK: no version, negotiation, alias or fallback, and
neither end re-checks the bounds the other guarantees. Each end reads only the
fields it uses; an unparseable message, an unknown kind, accessory key or
modifier phase, or a missing or mistyped read field fails the receiving end for
good, and extra fields are ignored. Each direction is FIFO, so no correlation
token exists. Native admits at most 1 MiB of output posted and not yet
acknowledged; more fails the page. The first acknowledgement while the app is
foreground presents the attempt's notification visit.

### Terminal input

The key deck is one fixed 2 × 7 matrix, `Esc / Shift Home ↑ End PgUp` over
`Tab Ctrl Alt ← ↓ → PgDn`, of equal minimum-size cells; when the measured
labels exceed the width, both rows scroll together. Keys are raw terminal
input: no lifecycle, navigation, macro or local-history key, and PgUp/PgDn
never scroll locally. Each enabled tap plays the keyboard-tap haptic.

Encoding, from one modifier snapshot, with `m = 1 + shift + 2·alt + 4·ctrl`:

- Arrows, Home, End: xterm's normal or application-cursor sequence; modified,
  `CSI 1;m A/B/C/D/H/F`. PgUp, PgDn: `CSI 5~`, `CSI 6~`; modified `CSI 5;m~`,
  `CSI 6;m~`.
- Tab is `HT`, or backtab `CSI Z` under Shift; Ctrl leaves it, Alt prefixes
  `ESC`. Escape and slash ignore Shift.
- Ctrl, then Alt, apply to a proven key: a deck literal, or a typed unit that
  xterm's `onKey` proves is one printable ASCII keystroke. Ctrl uppercases
  ASCII letters, maps `@`..`_` to `& 0x1f` and `?` to `0x7f`; Alt prefixes
  `ESC`. IME commits, composition, dictation, paste, non-ASCII and
  multi-character text stay literal and still consume all three. Shift never
  changes text.

Shift exists for Codex's pending questions (Shift+←/→) and Claude's mode switch
(Shift+Tab): providers interpret keys, skid never detects a question.
Alt+Shift+Tab is `ESC CSI Z`, deliberately unlike xterm's physical-key
evaluator. Gboard supplies hyphen and capitals.

Gboard owns typing, dictation (editable, never auto-sent) and the paste UI;
Enter is CR. Paste drops C0 except tab and newline, DEL and C1, turns CR and
CRLF into LF, and wraps the result in bracketed paste when the application
asked. Only a tap and deck keys focus xterm's helper textarea, so page-sent
input never fakes a focus-in. IME composition is clipped to the terminal edge,
and page and WebView both pin scroll at the origin.

### Touch and selection

One page state machine per stream: `Composing | Pending | Scrolling |
Selecting | Selected | PendingSelected | BlockingSelected | Blocked`.

- A stream that starts during IME composition, with no released selection, is
  consumed whole without effect; the next fresh gesture routes normally.
- Slop is 8 CSS px. Before the long-press timeout, vertical-dominant travel
  claims scroll and horizontal-dominant travel blocks the stream. Over a
  released selection a drag is swallowed and keeps it; a tap drops it.
- Scroll accumulates signed travel and emits, per animation frame, one wheel
  unit of whole rows at the start cell, clamped to the screen with excess debt
  dropped. Release sends one bounded correction to the absolute travel.
- A wheel unit routes: mouse tracking `vt200`, `drag` or `any` → one SGR wheel
  report; else the alternate buffer → one cursor up or down (application mode
  respected); else local `scrollLines`, no input.
- A sub-slop release focuses the terminal, sends an SGR primary press and
  release when the application tracks the mouse (press only under X10), then
  `ImeRequested`. Native raises the IME only while live, enabled, shown and
  window-focused. The page never asks while it holds a selection.
- At the long-press timeout the page selects the word under the finger even
  under mouse reporting: the cell alone when blank, else the run of cells
  without `` ()[]{}',"` `` or space, continued across soft wraps. Hold-drag
  extends by cells and, beyond the top or bottom edge, scrolls 1..15 rows per
  50 ms tick. An xterm marker anchors the word, so it follows its text as
  output scrolls; the range is reapplied on every move, tick and release.
- Release snapshots `getSelection()`: empty clears; over 256 KiB clears with a
  toast; otherwise native shows the floating `Copy` toolbar at the release
  point if still enabled and window-focused, else answers `ClearSelection`.
- The page drops a selection on a new tap, a second contact, window blur, page
  hide, reset or failure. One still being made also drops on textarea blur or
  resize, and when its anchor row leaves the buffer or the buffer switches.
- `Copy` writes `ClipData.newPlainText("Terminal selection", snapshot)` with
  `EXTRA_IS_REMOTE_DEVICE`, closes the toolbar and sends `ClearSelection`. The
  system clipboard preview is the only success feedback, so
  `EXTRA_IS_SENSITIVE` stays unset.
- Back closes the toolbar first, through an overlay-priority callback, because
  the floating action mode does not consume Back; the next Back detaches.
- A window capture listener stops xterm's own copy handler, so the page never
  writes the clipboard.

### Terminal chrome and sizing

Top to bottom: the rail (under the status bar), the terminal, the key deck and
a band filling the navigation-bar or keyboard inset; their surfaces are in
[design language §13](design-language.md#13-component-register).
`TerminalScreen` is the sole inset owner; the app pins dark system bars on any
system theme.

- The rail is `Detach` (Back is equivalent), then one control showing the tmux
  name over `<place> · <presence>` that opens the session sheet. Place is where
  typed input runs, host first: `macbook`; `devbox via macbook` for a resolved
  SSH/mosh pane; `remote via macbook` when the destination is unknown. Presence
  is `preparing`, `connecting`, `input frozen` or `N client(s)`; only frozen
  input is coloured.
- The session sheet holds status and context and routes to existing owners:
  `rename`, `new terminal on <owner>` (create-here on a local pane, the
  source-scoped forge on an SSH/mosh pane), `text size · N`, then the card's
  lifetime rows `send interrupt` (unconfirmed, toast receipt),
  `close terminal only` and `interrupt and close terminal` (both confirmed).
  Rows disable while a rename or close is pending or their owner would refuse.
  Inferred status shows only here while attached.
- The too-small overlay covers terminal and deck above the inset band, so it
  never moves the WebView's bounds and stays reachable at zero height. It
  offers `Hide keyboard` while the IME is up and `Text size`; there is no
  automatic shrinking. `Detach` stays outside it.
- The text-size sheet steps `−`, `+` and `Reset to 16`, each admitted only
  while the page is live, no write is pending and the value changes.

Rejected, because each would resize the shared window or cost a scarce row:
hiding the status bar, an auto-hiding or bottom rail (it would put `Detach`
over `Esc`), a deck menu key (eight 48 dp keys overflow a 384 dp phone), pinch
zoom (column reflow, and it competes with selection).

### Page and files

- `filesDir/datastore/terminal-text-size`, replaced through `AtomicFile`:
  exactly two ASCII digits, 12..24. Absent means 16; anything else is a read
  failure. The size loads before the page exists.
- `assets/terminal/` (`index.html`, `terminal.css`, `terminal.js`, `vendor/`)
  is served only at `https://appassets.androidplatform.net/assets/terminal/`;
  every other request answers 403 and other main-frame navigation is refused.
  The CSP is `default-src 'none'` with bundle-only scripts, styles and fonts,
  plus the inline style elements and attributes xterm generates.
- The tmux client advertises `RGB` and runs with `TERM=xterm-256color`; the
  page owns one palette ([design language §10](design-language.md#10-terminal-theme)).
- Geometry is published only after both JetBrains Mono faces load (on failure,
  monospace); the page then re-measures xterm's cell through its private
  `_charSizeService`.
- `screenReaderMode` stays on: it selects xterm's keydown and `insertText`
  input path, and with it off stock xterm lost the first Gboard letter in 4 of
  72 emulator runs against 0 of 87. The phone has no screen-reader support
  ([design language §14](design-language.md#14-accessibility-floor)).
- `vendor/` holds verbatim upstream bytes, never edited, with no build step or
  check:

| File | Source | Identity |
| --- | --- | --- |
| `xterm-6.0.0.js` | `package/lib/xterm.js`, npm `@xterm/xterm@6.0.0` | Tarball `sha512-TQwDdQGtwwDt+2cgKDLn0IRaSxYu1tSUjgKarSDkUM0ZNiSRXFpjxEsvc/Zgc5kq5omJ+V0a8/kIM2WD3sMOYg==`; file SHA256 `14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972` |
| `xterm-6.0.0.css`, `xterm-6.0.0.LICENSE` | `package/css/xterm.css`, `package/LICENSE`, same tarball | Byte-identical |
| `xterm-addon-fit-0.11.0.{js,LICENSE}` | `package/lib/addon-fit.js`, `package/LICENSE`, npm `@xterm/addon-fit@0.11.0` | Tarball `sha512-jYcgT6xtVYhnhgxh3QgYDnnNMYTcf8ElbxxFzX0IZo+vabQqSPAjC3c1wJrKB5E19VwQei89QCiZZP86DCPF7g==` |
| `JetBrainsMono-{Regular,Bold}.woff2`, `JetBrainsMono-2.304.LICENSE` | `fonts/webfonts/` and `OFL.txt`, JetBrains Mono v2.304 release zip | Byte-identical |

Re-verify from the repository root:

```sh
curl -sL https://registry.npmjs.org/@xterm/xterm/-/xterm-6.0.0.tgz -o x.tgz
openssl dgst -sha512 -binary x.tgz | base64
tar -xzOf x.tgz package/lib/xterm.js | cmp - android/app/src/main/assets/terminal/vendor/xterm-6.0.0.js
rm x.tgz
```

An xterm upgrade is a file swap, a check that `_charSizeService.measure()`
still exists, and a rerun of the phone typing, touch and selection journeys.

### Phone terminal errors

- Page failure (a rejected message, over-bound input or output, an asset or
  HTTP error, a renderer crash, 10 s without `Ready`) and every connection
  failure end the attempt as `ReconnectRequired`. The panel says no input will
  be replayed and offers `Reattach` (a fresh attempt) and `Back to Dwarves`.
- A server `Error` shows its code's fixed message;
  `TerminalConfigurationUnsupported` names the three tmux options, which skid
  never changes.
- An upgrade answered `Unauthenticated` or `MachineIdentityMismatch` ends that
  machine's access, not only the terminal.
- A text-size read failure blocks the page with `Retry` (a reread); a write
  failure keeps the prior size and says so. Nothing retries automatically.
- A clipboard `SecurityException` keeps the toolbar for another explicit try;
  a toolbar that cannot start clears the selection.

### Phone terminal assumptions

- tmux enables SGR 1006 with every mouse mode, so the page encodes every
  report as SGR whatever encoding the application negotiated.
- A resizing client becomes the window's latest before tmux recalculates
  sizes, and `#{window-size}` resolves the session's current window option,
  then the global one (read from tmux source, not run).
- The API-36 WebView withholds PointerEvent move and up after a long press but
  keeps TouchEvents, so the page uses TouchEvents; xterm misreads constructed
  DOM wheel events, so the page never synthesizes them.
- Stock xterm on a physical phone (ANSI, Unicode, IME composition, editable
  dictation, clipboard, automatic DA/DSR/CPR replies, resize and rotation) is
  `NOT_RUN`, owner-accepted; only an emulator differential, with its own gaps,
  qualifies it ([phone acceptance](issues/stock-xterm-phone-acceptance.md)).

### Phone terminal accepted limits

- One swipe is local scrollback or shared application input, by mode; a wheel
  into tmux moves what the laptop sees. No fling; remote velocity is capped at
  one unit per frame. A physical mouse wheel bypasses the page router.
- Readable text costs columns; concurrent clients alternate the shared size;
  reflow can move content; conflicting tmux overrides need the operator.
- Opening a sheet takes window focus: Gboard hides, modifiers reset and the
  shared window resizes until the keyboard returns.
- One-shot modifiers must be re-armed per shortcut, and any intervening
  `Input`, including an automatic reply, consumes them. The five-letter
  `Shift` label can force shared deck overflow on a narrow phone.
- A gesture that starts during composition is lost.
- Selection: the highlight can vanish for one tick when tmux resends mouse
  modes, and later output can change highlighted cells, but `Copy` stays the
  release snapshot. Rows shifted in a region not starting at row 0 (a lower
  split pane) do not move the anchor, and a text-size change during a hold
  leaves it in the old grid. Word bounds differ from xterm's own at soft-wrap
  separators and exact cell boundaries. There are no post-release handles, and
  clearing after `Copy` prevents an immediate re-copy. A `ClearSelection`
  crossing a newer selection clears it; no wrong text is copied.
- The system preview shows copied text and Android may retain or sync the
  clip; the remote-device extra is a rendering hint, not a guarantee.
- Output frames queued on a stalled UI thread are uncounted until it runs.
- Open records: [stock xterm phone acceptance](issues/stock-xterm-phone-acceptance.md),
  [resize churn](issues/terminal-resize-churn.md),
  [landscape keyboard](issues/terminal-landscape-keyboard.md),
  [IME reshow](issues/terminal-ime-reshow-selection.md),
  [hello identity](issues/terminal-hello-identity.md),
  [error copy](issues/terminal-error-copy.md).

## 15. Operations

### Installation

A host archive holds the `skidbladnir` binary, `release.json` `{platform,
sourceSha, version}`, `providers/shell-init`, `providers/terminal-context-init`,
`third-party-notices.txt`
and, on mac, the signed `Skid.app` and `terminal-runtime/SkidTerminals.app`.
the gateway and terminal host have distinct stable signed identities. it ships no interpreter; runtime startup
downloads nothing. supported: `darwin-arm64` on macos 26+; `linux-amd64` on
ubuntu 24.04+ or arch-family (`ID=arch`, or `arch` in `ID_LIKE`) with systemd
user services (`internal/install/platform.go`); android 13+ (min sdk 33, target
36) with google play services, where the owner waives verification on versions
older than the test phone's.

`bash install [vX.Y.Z]` (the release's bootstrap; latest stable when omitted)
verifies and hands off to the archive's own `skid install`. Installed commands:
`skid install | update [--version vX.Y.Z] [--archive PATH --signature PATH]
[--restart-terminals]` and
`skid uninstall`, each printing its help to stdout on `-h` or `--help`; one
installation operation runs at a time (`install.lock`).

1. Trust starts at the bootstrap. `scripts/install` embeds the Ed25519 release
   public key; a key fetched beside an archive would authenticate nothing.
   Archives are signed over their SHA256 digest, and the installed binary
   verifies with its compiled-in key (`internal/install/release-public-key.pem`),
   so rotating the key needs a new bootstrap on every host. The bootstrap needs
   curl and OpenSSL 3+ (`pkeyutl -rawin`); on Mac it installs Homebrew
   `openssl@3` when the system one lacks it.
2. Nothing skid-owned changes until an archive is verified, safely extracted (no
   absolute or `..` paths, no duplicates, regular files and directories only),
   matched to this platform, its binary's `version` equals its manifest and, on
   mac, strict signature, pinned signer, identifier and designated-requirement checks
   accept the gateway and both apps before any signature normalization. a release older than
   `current/release.json` is refused.
3. Layout: `${XDG_DATA_HOME:-~/.local/share}/skidbladnir/releases/<version>-<sha>`,
   `current` a symlink to one, `~/.local/bin/skid` into `current`. Install
   keeps the terminal runtime at `terminal-runtime/`, beside `releases/`.
   gateway-only updates stop only the gateway/notification jobs, switch `current`,
   prove readiness, then prune old releases. an unchanged normalized runtime payload,
   signer/requirement and effective service definition leave its files/job/pid intact.
   after signature verification, comparison excludes code signatures and the Go
   build ID/Mach-O UUID metadata in scratch copies only. executable code, data and
   resources still match exactly; build identity alone does not force a restart.
   runtime artifacts contain no gateway version/source stamps. an actual runtime
   difference requires a drain or explicit checkpoint-preserving restart; no deferred update or second
   implementation. failures name the completed stage and failed component.
4. skid owns fixed paths by name and records nothing: `releases/`, `current`,
   `bin/skid`, `terminal-runtime/`, its service files and two marked shell blocks. install overwrites
   them; hand edits inside them are lost.
5. Shell blocks: a `managed shell` block goes last in `.bashrc`, the bash login
   file (first of `.bash_profile`, `.bash_login`, `.profile`),
   `$ZDOTDIR/.zlogin` and `$ZDOTDIR/.zshrc`, with `ZDOTDIR` as zsh itself
   resolves it. Its first line runs in every shell that reads it and prepends
   `~/.local/bin` to `PATH` only when absent, so new login and interactive
   shells find `skid`, while sourcing it again or an order the user chose
   changes nothing; install's last line names `export PATH` when the invoking
   shell's `PATH` lacks `~/.local/bin`. The rest sources
   `current/providers/shell-init` only when `SKIDBLADNIR_SHELL=1`.
   `.zshrc` also starts with a `managed zsh prompt` block
   that turns off powerlevel10k's instant prompt only in the exact shell skid
   started, because its cached preamble can redirect the first command's
   terminal. Symlinks are followed, modes kept, files replaced atomically; an
   incomplete or duplicated block stops installation. dev-server's dotfiles rely
   on this placement (`internal/install/shell.go`).
6. services: linux uses independent `skidbladnir.service` and
   `skidbladnir-terminals.service`; neither stops with the other. the terminal
   unit runs foreground tmux with `Restart=always`, `RestartPreventExitStatus=78`.
   startup before login still needs `sudo loginctl enable-linger UID`; without
   linger both services and their sessions stop at logout. install reports this
   host policy and an inactive tailscaled service without changing either. mac uses
   `dev.niels.skidbladnir`, `dev.niels.skidbladnir.notifications` and
   `dev.niels.skidbladnir.terminals`. the terminal host runs with
   `KeepAlive {SuccessfulExit:false}`, `ProcessType=Interactive` and ordinary process
   group cleanup; collision maps to success, unexpected tmux completion to failure.
   keep responsive scheduling. job environments contain explicit home/path/locale
   and xdg directory roots, no fleet/provider credentials. terminal readiness proves
   a reachable managed lifetime with `exit-empty off` and `exit-unattached off`.
   gateway readiness proves the new service lifetime and authenticated existing
   `/v1/local/status` for the installed version; its wait allows the existing
   30-second recovery budget plus five seconds of native startup. mere registration
   is not success.
7. Missing tmux or Tailscale install through Homebrew (`tmux`, cask
   `tailscale-app`; Homebrew is a prerequisite), apt plus Tailscale's official
   installer, or pacman; privilege prompts are the user's. providers, zoxide,
   ssh and mosh are never installed. pacman confirms on the controlling terminal;
   without `/dev/tty`, installation refuses before stopping services.
8. `skid update` runs the installed binary's installer, so a release's installer
   changes apply from the following update, or at once through its own
   `bash install TAG`. Running desktop browsers keep their loaded binary until
   reopened.
9. runtime replacement and uninstall require a drain: finish work, close tmux
   sessions, pause native/automated session creators and let the gateway save an
   empty checkpoint. preflight must prove an exact empty managed lifetime or process
   absence, including any recorded source; unavailable is never empty. stop the
   gateway, verify the checkpoint is still empty/readable, then recheck the exact
   server immediately before stopping it. a missing checkpoint is acceptable;
   a nonempty/unreadable one requires repair using the current installation.
   there is no transaction with native tmux clients: the accepted cost is a short
   maintenance window. pause manual gateway launches during installation too;
   an exclusive bind preflight refuses an existing unrelated listener after the old
   managed gateway exits. no force flag or checkpoint deletion. uninstall performs
   this before touching tailscale/services/files, then removes both services, owned
   runtime paths and shell blocks. settings, private state, provider data, the tmux package and
   shared packages remain; there is no purge.

`--restart-terminals` explicitly authorizes restarting the selected default server,
including an unmanaged server, after pausing new work and finishing important turns.
normal installs still refuse unmanaged servers. the installer must run outside the
selected tmux tree. it stops the gateway, takes the existing workspace writer lock,
validates the unattempted schema-2 checkpoint against the exact live source and full
current graph, and reconfirms usable distinct resume targets for every recognized
agent. unrecognized foreground programs or missing resume coverage refuse before
shutdown. no prompt, terminal text, account data or conversation id enters its report.

the exact source is stopped only after final validation. the installer establishes
a managed empty destination, releases the writer lock and starts the gateway;
ordinary recovery durably claims and reconstructs once. partial installation failure
preserves the recipe; a proven-lost source can retry setup without replacing it with
an empty checkpoint. `attempted=true` never resets. no checkpoint schema migration,
compatibility bootstrap or live process adoption. uninstall remains drain-required.
fresh installation remains `bash install`, then `skid`.

### Release

`scripts/release TAG` runs on the Darwin arm64 signing host.

1. Preconditions: a clean worktree at exact `origin/main` of the original public
   repository, immutable releases enabled, a successful hosted `verify` push run
   for that SHA, a tag that exists nowhere and whose Android version code
   (`major*10^6 + minor*10^3 + patch`) exceeds every existing tag's. Keys:
   `SKIDBLADNIR_MAC_SIGNING_IDENTITY` (SHA1 of the certificate pinned by
   `internal/install/mac-signing-cert.sha256`, in the user keychain), the ed25519 key at
   `SKIDBLADNIR_RELEASE_SIGNING_KEY` (default
   `~/.config/skidbladnir/release-signing.pem`) matching the pinned public key,
   and `${XDG_CONFIG_HOME:-~/.config}/skidbladnir/android-signing.properties`
   for the signer pinned by `android/app-signing-cert.sha256`.
2. Everything ships from a fresh clone of that SHA: host binaries by
   `scripts/build-host-binary` (go.mod's Go family at or above its patch,
   `GOTOOLCHAIN=local`, trimmed and stamped; Mac deployment target 26.0), the APK
   by Gradle `assembleRelease`, never the debug keystore.
3. Exactly nine assets: both host archives and their `.sig`,
   `skidbladnir-android.apk`, `android-signing-cert.sha256`, `install`, a
   five-entry `SHA256SUMS` naming every asset but itself and the signatures, and
   `SHA256SUMS.sig`, the release key's signature over its SHA256 digest, as for
   the archives. A user verifies it with OpenSSL 3+ against
   `internal/install/release-public-key.pem` at the tag; the digest must be a
   file, since Ed25519 verification cannot read it from a pipe.
   `scripts/check-release` verifies every artifact once, before the draft;
   `--reproduce` also rebuilds and compares bytes.
4. Third-party notices ship inside the artifacts. A host archive's
   `third-party-notices.txt` holds the license and notice files of the Go
   distribution (`golang.org/toolchain` at the binaries' Go version) and of each
   module `go version -m` names; a module without a license file stops the
   release, and check-release refuses notices that omit a linked module. The
   APK's `assets/licenses/third-party-notices.txt` comes from the app build's
   `<variant>ThirdPartyNotices` task: each runtime library with the licenses its
   pom or parent pom declares, then each distinct license or notice text the
   libraries carry, Google's `third_party_licenses` included; a library that
   declares no license fails the build. The font and xterm.js licenses are
   committed assets.
5. The notes give the source, verify run and signers, then the changes: the
   pull request titles GitHub's merge commits carry, on main's first-parent
   history since the previous release tag, then install and asset digests.
6. Publication is a separate reviewed action. Then `scripts/check
   published-release TAG SHA` re-verifies the public release, its tag and hosted
   run, and requires `release-pin.json` at `HEAD` to pin version, source SHA and
   five asset digests. The pin states what is published, never what a host runs.
7. Signing keys are operator backups outside git. A lost Android key means
   reinstalling the app, since Android refuses another signer. Checksums detect
   corruption; signatures prove possession of the key.
8. Hosted CI runs `scripts/check verify` on each pull request and main push,
   scoped by changed paths (`.github/workflows/verify.yml`); docs, `AGENTS.md`
   and `release-pin.json` build nothing. Release integrity and engineering
   checks are not behavioral acceptance ([testing](rules.md#testing)).

### Lockstep cutovers

Host, desktop and phone ship from one release and consumers decode tolerantly,
so an additive wire change needs no install order. Removing or retyping a field
a consumer reads, adding an enum value it branches on, or changing a durable
format is a lockstep cutover with no converter or old reader. Each such release
records its cross-repo operator steps as an issue, deleted once the fleet runs
it.

### Fleet operations

- First computer: `bash install` bootstraps a fleet with itself as hub, named by
  its hostname without `.local`. For remote access sign in to Tailscale, enable
  tailnet HTTPS and run `skid connect` on the hub first: invitations need its
  origin.
- Another computer: install, sign in to Tailscale, run `skid connect` on any
  enrolled computer, then `skid join --name NAME` on the new one and paste the
  line. Then run `skid connect` once on the new computer and discard its
  invitation: join neither routes port 8443 nor sets the computer's origin
  ([issue](issues/join-skips-serve-route.md)).
- Phone: `scripts/install-android APK` checks the pinned signer and the package
  `dev.niels.skidbladnir`, needs exactly one authorized adb device and SDK
  build-tools 36.1.0, and runs `adb install -r`, keeping app data; installation
  is not pairing. Sign the phone in to the same tailnet and scan a
  `skid connect` QR.
- Remove a device: `skid remove DEVICE_ID`, IDs from `skid status`.
- Move or lose the hub: with push configured, first place a matching credential
  on the new hub; run `skid recover` there; `skid join` every other computer
  with a fresh invitation; clear skid's app data on each phone and scan again.
  Attention, acknowledgements and receivers are not carried. A returned old hub
  cannot reclaim recovered devices, which fetch only from the new one; offline
  devices stay in the old fleet until re-enrolled.
- Roll back: `skid uninstall`, the previous release's `bash install vPREVIOUS`,
  then `skid connect` once for the route. Android refuses an in-place
  downgrade: uninstall the app, losing its data, and re-pair. The release's
  cutover issue names the state files to restore or delete.

### Logs

serve writes one content-free JSON event per line (`time`, `event.name`,
attributes, with codes, IDs and durations but never names, titles, cwd,
objectives, terminal bytes, prompts, homes, origins, bearers or account data;
the opaque machine handle may appear): to the journal on Linux, to `serve.log` on Mac, rotated at 8 MiB
into `serve.log.1` (two overlapping serve processes can approach twice that).
Mac serve stderr is discarded ([issue](issues/macos-serve-stderr-discarded.md)).

### Consumers

- dev-server, the owner's provisioner, fetches `scripts/install` at a reviewed
  source SHA, checks its SHA256, runs `bash install` for latest signed stable,
  preserves action-required exits, and on the hub
  alone writes `config.yaml` (`0600`) from its own source and runs `skid apply`.
  Its next run replaces hand edits, so lasting changes belong in dev-server;
  membership and generated state stay skid's.
- jarvis uses the public [CLI](#cli) and reads no fleet files. It pins the
  enrolled machine and its own action id for a delegated assignment, receives
  its saved outcome, and acknowledges that copy. Explicit terminal intervention
  captures a full `--ref` before acting. Its cross-user access to installed state
  is its own record ([topology](#trust-invariants), invariant 6); skid provisions
  no jarvis credentials. The [delegation contract](agent-delegation.md) owns job
  storage, delivery, and release qualification.

## 16. Cross-cutting invariants

Beside the five constraints at the top, these hold across every section; each
link names the rule that owns it.

1. Identities are distinct and never substitute for each other: machine handle,
   device ID, server lifetime, session lifetime, pane ID, foreground start
   identity. Names, labels, dwarves, groups and positions describe; they never
   select or authorize ([rules](rules.md#identities-and-exact-targets)).
2. Validate once, at ingress: human YAML and arguments, OS, tmux and provider
   output, gateway requests. Same-release peers decode tolerantly, and an
   undecodable answer is a lost answer ([rules](rules.md#trust-boundaries)).
3. Producers render copy. The host renders status, recovery notices and errors;
   clients add only what they alone know ([rules](rules.md#copy-and-drafts)).
4. Operational ages run on local monotonic time from receipt, and transmitted
   instants are UTC ([rules](rules.md#timing-and-polling)). Claude quota facts
   order by their original fetch instants. Clock skew can misorder close
   measurements ([quota](#account-quota-and-usage)).
