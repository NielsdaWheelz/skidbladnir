# standalone generalization: implementation contract

2026-10-21 amendment: [terminal-only control](terminal-only-control.md) removes
conversation discovery/controls and hooks below, replaces `historyScope`
with `accountScope`, and narrows the packaged helper to quota; the go quota
client ([profile usage](profile-usage.md)) then removes the helper and its pins.
ordinary commands and existing terminal observation remain. the earlier native
clauses are historical.

status: optional user-owned direct-fcm direction, 2026-10-20. runtime feasibility
gate passed before production rewrite; source implemented and independently
reviewed. [qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/generalization-qualification.md) separates passing
isolated boundaries from deferred installed/background acceptance.
completed unrelated source/proof is retained.
source audited: `d34b6897d7938c301cdfed07d8c792f11beb06a1`.
[decisions](generalization.md) owns product choices; this document owns their
implementation boundaries and acceptance. [architecture](architecture.md) remains
the implemented contract until cutover. refresh the source map against main before
implementation; preserve intervening product changes.

## 1. outcome and invariants

the owner's skid must LOOK, FEEL, AND ACT THE SAME, except for the explicitly
[agreed changes](generalization.md#scope-preserve-existing-behavior).
preserve layouts/keys, creation/directory controls, terminal behavior, usage and
notification semantics. import machine handles/names/order, profile keys/labels/
order, client defaults/selection, effective commands/accounts, shell and ghostty
choice. truncate new long names through existing layout primitives; current
names render identically. no new mobile quota screen. creation model/effort
arguments are removed from cli/api; current desktop/phone forms have no such controls.

| owner | responsibility; forbidden dependency |
| --- | --- |
| each installed host | local tmux, processes, terminal/native apis, local observation; never manages another host's tmux |
| desktop/phone | compose enrolled host apis directly; hub is never an execution proxy |
| hub | accepted configuration, membership, observer and android publication; fixed, no elections or remote execution ownership |
| each notification device | acknowledgement, admission, exact focus/clicks, presentation; no fleet-wide acknowledgement |
| optional hub fcm adapter | user-owned credentials, per-phone destinations and content-free hints; no project backend |
| providers/user | provider installation, accounts, settings, history and daemon lifecycle |

local terminal access works without internet, tailscale login, a provider, native
daemon, phone or firebase setup. cached fleet access survives hub loss. manual ssh/mosh
retains ordinary source-terminal control, never destination inventory or inferred
provider/account/attention. existing exact targets and uncertain-input receipts
remain authoritative. [session latency](session-latency.md) still applies:
configuration reconciliation and optional observation must not delay attachment.

non-goals remain those in the decision record. no new product interaction,
conversation association, generic read facade, mixed-version support or retired
runtime/test machinery.

## 2. configuration and durable state

one human file: `${XDG_CONFIG_HOME:-~/.config}/skidbladnir/config.yaml` on the hub.
new installations write `version: 1` and `profiles: []`, with commented examples.
this populated example is explanatory, not the installed default:

```yaml
version: 1
profiles:
  - name: work
    label: "codex · work"
    command: codex-work
    provider: codex
    home: ~/.codex-work
  - name: tools
    command: 'cd ~/project && ./tools'
# shell: /bin/zsh
# terminal: apple-terminal
# default_machine: devbox
```

| field | exact meaning |
| --- | --- |
| `version` | required integer `1`; reject other versions, duplicate/unknown keys and multiple yaml documents |
| `profiles` | required ordered list; unique existing-valid profile keys; `label` defaults to `name`; nonempty opaque `command`; empty list is valid |
| `provider?`, `home?` | observation metadata; provider `codex` or `claude`; home requires provider and is absolute or `~/…`; omitted home uses that provider's ordinary default |
| `shell?` | absolute bash/zsh executable on the destination; omitted uses that host's login shell; unsupported shell is an actionable local error |
| `terminal?` | mac notification adapter: `apple-terminal` or `ghostty`; new mac default apple terminal; no installer dependency on ghostty |
| `default_machine?` | existing desktop default selection, by enrolled name; omitted uses self for a new installation; phone retains its local selection |
| `android_push?` | optional public firebase fields only, defined in §6; omission disables android background push |

machine names reuse existing case-insensitive uniqueness and display validation;
stable `mh-` identity is separate. preserve existing display spelling. every host
reads the same top-level `shell`, `terminal` and `default_machine`; there is no
`hub` or `machines` key. removed machines are not retained, so a removed machine
still named in `default_machine` makes the next apply `ConfigInvalid` until the
yaml drops it; meanwhile a desktop default naming it resolves to self. never
rewrite human yaml automatically.
membership commands own names/order;
append new machines. no yaml include, env interpolation, executable config,
credentials, endpoint overrides or provider command preflight. `command` alone
uses shell expansion. resolve `~` for metadata on the destination.
use one strict yaml decoder with the existing 64 kib input limit; no custom yaml
parser. the maintained [go yaml package](https://pkg.go.dev/go.yaml.in/yaml/v3)
decodes one document into the typed config with unknown fields refused, then the
semantic rules run. unknown and duplicate keys, extra documents and wrong value
types are errors, though a null key is skipped; aliases and merge keys expand; a
scalar in a string field keeps its literal text; a float `version` truncates;
null or empty leaves a field unset and a null list item is dropped. decode errors
carry the yaml package's `line N: …` text and go type names; semantic errors keep
field paths.

generated private state uses existing xdg paths, directories `0700`, files `0600`;
reuse machine handles, bearer generation, atomic replacement and notification stores.

```text
config/machine-handle           `mh-…`, written once at first start
state/fleet.json                machine, name, localBearer,
                                fleet {deviceId, bearer, state},
                                applied {settings, application},
                                authority (hub only)
state/notification-observer-v1.json   hub observer epoch, revision, records, receivers
state/authority.lock            flock held for the `skid serve` lifetime
existing device files           retained acknowledgement
```

```text
state:     {fleetId, hub, stateRevision, configRevision,
            machines:[{machine,name,origin?}], androidPush? (phone view only),
            members?:[{deviceId,kind,machine?,verifier}], settings?}
authority: {invites:[{verifier,expiresAt}], reports:{deviceId: report}}
```

`state` is the one wire and cache type: the hub persists it as authority, serves
it from `GET /v1/fleet` and the join reply (phones get it without `members` and
`settings`, plus `androidPush` derived from `settings.android_push`), and members
cache it. `authority` exists only on the hub; its
presence is the hub role, read once per process. `fleetId` is 32 random bytes as
64 lowercase hex. `stateRevision` increases on membership, origin and settings
changes; `configRevision` on accepted settings changes. `machines` lists the hub
first, then join order. the hub stores accepted settings, not yaml bytes; the human
file can hold pending edits. a member's cached `state` is replaceable; the handle, the
hub's `authority` and observer state are not.

one process writes these files. bootstrap, join, recover and every hub mutation
commit with one atomic replace of `fleet.json`; creating a fleet resets the
observer file before that replace, and the observer starts fresh from a file
that names an earlier fleet (the exiting process may write it once more). `applied` persists: a host whose newly
accepted settings do not resolve locally keeps its previous applied settings with
`invalid_host` across restarts. a deleted handle with a surviving `fleet.json`
refuses to start. there are no signatures, envelopes, authority epochs, relays or
staging files: members fetch plain json only from the hub, over tailnet tls, with
their own bearer.

## 3. application, enrollment and access

one host service composes gateway/fleet control and, when hub, the observer.
retain the separate mac app lifetime; no service/plugin framework.

opening desktop skid on any computer requests hub application of its own yaml;
hub service startup does likewise. coalesce concurrent requests under one hub
mutation lock. accept the complete structurally valid file atomically, or retain
the previous accepted file. clients never upload cached settings. no file watcher.
publish authenticated revision hints, one per state revision; connected clients
fetch on change/reconnect. a member backs off from 1 s to 30 s while the hub is
unreachable; a local apply or connect that reaches the hub resets it.
disconnected caches have no expiry. the hub reloads push transport only when the
accepted push options or its credential file change.

membership admission is independent of local launch resolution. adopt the hub's
membership first and close removed devices' connections even if the new
shell/path is invalid. resolve settings locally, swap one immutable runtime
snapshot, and report `{desiredConfigRevision, appliedConfigRevision, status,
reason?}`. status is `applied`, `pending_offline` or `invalid_host`. retain
previous launch settings on local failure. every create/native read captures one
revision; late results cannot restore invalidated account scopes after a config
change. a report advances `stateRevision` only when it changes the host's origin.

identity and role are constants of a process (crash-only). after a committed join
or recover, or any failed `fleet.json` write, the http reply is written and
`skid serve` exits non-zero; launchd `KeepAlive` and systemd `Restart=on-failure`
restart it, and `KillMode=process`/`AbandonProcessGroup` keep tmux. SIGTERM exits
zero. inside the process one change notification plus a snapshot pull delivers
each committed change to the gateway, session manager, desktop client and
observer.

retain `127.0.0.1:7341` and tailscale serve's https `8443` `/v1` route. installed
clients may synthesize the loopback endpoint ONLY for their own stored machine
identity, with authentication and exact machine header. never advertise loopback.
remote origins are optional until network setup, then strict tailscale https
origins on `8443`; retain disabled redirects/proxies and exact-host admission.
`connect` reports missing tailscale login/https/reachability before issuing an
unusable invite. network authentication remains external.

each enrolled device has its own random 256-bit bearer. hosts receive only its
sha256 verifier, mapped to device id; retain constant-time authentication.
computer members receive members and settings; phones receive public routing only.
ordinary members have full fleet access, including future machines. this is one
trusted user's fleet, not a role system. exact target-machine checks remain.

every cli command prints json on stdout and exits 0 on success, 1 on failure and
64 on usage errors. commands that need the hub call loopback with the local bearer
on the hub, else the hub's origin with this host's fleet bearer; without a hub
origin they fail with `hub has no remote address; connect tailscale on the hub
and run skid connect there`.

| cli | behavior |
| --- | --- |
| `skid connect` | refuse when the service version differs from the cli; configure the tailscale origin and serve route and report the origin; request a five-minute single-use invite from the hub; print one pasteable line, a qr only on a tty; never permanent credentials |
| `skid join [--name NAME]` | read one invite line from stdin/prompt; the service posts a fresh device id and verifier to the invite's hub with no staging, and on success replaces this host's fleet in one write and restarts; failure leaves state untouched; local `config.yaml` stays untouched, no inactive copy |
| `skid remove DEVICE_ID` | remove that device at the hub; refuse the hub's own device; no pending list |
| `skid recover` | new fleet with this host as hub from its cached accepted settings (default machine self); new fleet id, printed; the service restarts and its startup apply adopts this host's `config.yaml` when valid; re-join computers and re-pair phones |
| `skid apply` | request application of the hub's own yaml without opening the interactive browser |
| `skid status` | `{version, fleetId, hub, stateRevision, configRevision, members:[{deviceId,kind,machine?}], applications:[{deviceId,name,report:{origin?,version,application}}], message?}`; no terminal or account contents |

invite: `skid://connect/` plus base64url (no padding) of `{fleetId,
hub:{machine,name,origin}, token, expiresAt}`, at most 4096 decoded bytes, token
32 random bytes. `skid join` and the phone parse it as human ingress: prefix,
canonical base64url, strict json, valid handle/name/origin, unexpired. the hub
stays authoritative for one-use and expiry; it stores only the token's sha256
verifier and expiry, needs its own origin, prunes expired invites and admits at
most 32 live ones. one invitation can enroll either device kind. joining posts
`{token,deviceId,kind,name,machine?,origin?,verifier}` without a bearer. the hub
validates the body, matches the token in constant time, consumes it only on
success and commits membership in the same write. a computer whose machine is
already a member (not the hub) replaces that member's device id, verifier and
origin, keeping its position and, unless the request names a valid new one, its
name; any other device is appended. a retried committed join gets 401 (token
consumed): rerun `skid join` with a fresh invite. the phone persists a staged
identity before joining and recovers a lost reply by fetching with it; same-fleet
repair first authenticates with the saved identity, and only a 401 stages a new
device id and joins. machine identity/tmux survive joining; former standalone
phones do not join implicitly. duplicate names require an explicit new name,
never silent renaming.

revocation is 401. removing a device drops its member record (and a computer's
machine and report) at the hub, which rejects its bearer at once; every other
host rejects it after its next fetch from the hub and closes its connections.
the removed device learns nothing else: it sees auth failures until a fresh
invite re-enrolls it. a phone keeps its cache. a computer whose fetch gets 401
drops its cached members, since no later removal can reach it, and then admits
only its own local bearer; it keeps serving its own local terminals, and
`skid status` says it was removed. already accepted terminal effects are not
undone. never infer local authority from a loopback source address: tailscale
serve uses loopback too; the installed local bearer authenticates as the host's
own computer.

| new/changed api | authority and result |
| --- | --- |
| `GET /v1/fleet` | any member: plain json `state`; computers also get `members` and `settings`, phones public routing only |
| `GET /v1/fleet/events` | any member: sse `event: revision` / `data: {"stateRevision":N}` now and on each change; ends when the caller stops authenticating |
| `POST /v1/fleet/join` | hub, no bearer: single-use invite token; `201` with `state` (phones: public) |
| `POST /v1/fleet/apply` | hub: no settings body; accepts the hub's yaml, returns status and per-host application |
| `POST /v1/fleet/invites` | hub: five-minute single-use invite |
| `DELETE /v1/fleet/members/{deviceId}` | hub: durable removal; refuses the hub; status `device removed` |
| `PUT /v1/fleet/self` | hub: a computer reports only its own origin, version and config application status |
| `/v1/local/{status,apply,recover,join,network}` | installed local bearer only: cli control; a member's apply and network report forward to the hub |

reuse existing typed errors `{code,message}`. invalid input `400`, failed auth and
unknown/expired/used invites `401`, wrong device kind/target `403`, conflict `409`
(hub routes on a member: `FleetConflict` `this computer is not the hub`),
unavailable hub `503`. errors carry no routing; never redirect credentials.
request bodies decode strictly (unknown keys `400`, over 64 kib `413`); members,
the cli and the phone decode responses tolerantly. there is no `DeviceRemoved`,
removal proof, verifier tombstone, detached mode, host-state route or handoff
route.

preserve existing session/conversation/terminal/profile-usage endpoints and
`t-`/`c-` handles. profile descriptors become
`{key,label,provider?,historyScope?}` everywhere; absent metadata stays absent.
unannotated profiles remain selectable ordinary rows with unknown quota and no
profile-native read; live provider detection does not depend on this annotation.
only creation model/effort fields are removed, with strict unknown-field rejection.

## 4. fixed hub

the hub is the machine that created the fleet or last ran `skid recover`. there
is no transfer, placement, wrong-hub routing or relay. moving the hub is a new
fleet: `skid recover` on the new hub (with configured push, place a matching
private credential file there first), `skid join` with a fresh invite on each
other computer, and a fresh scan on each phone after clearing its app data (a
phone paired to the old fleet refuses another fleet's invite). observer epoch,
attention records and receiver state are not carried; the new hub starts a fresh
observer. a returned old hub cannot override new trust: recovered devices fetch
only from the new hub. no automatic failover.

## 5. launch, observation and quota

retain the existing generic startup and terminal-context machinery. replace
provider wrapper execution with one opaque command evaluated exactly once by
the initialized destination bash/zsh. pass command text as data, not generated
shell syntax. preserve aliases/functions, quoting, provider environment and shell
options; remove only skid's private inherited launch markers. honor requested cwd,
ordinary command return, and explicit `exec`/`exit`. keep source-terminal ssh/mosh
context tracking; do not override existing user ssh/mosh functions or aliases.

installer resolves actual startup destinations, including `ZDOTDIR` and bash's
selected login file, preserves symlinks/content, and adds one guarded source at
the final applicable startup boundary. use the existing one-shot prompt callback;
remove it before command evaluation. unsupported/missing shells fail locally,
never choose another shell silently. no provider preflight or alias interpretation.

remove `PrepareLaunch` daemon startup, forced codex remote/cwd/model/effort flags,
account env injection/deletion and replacement provider aliases. adapt the
launch-marker filter in `cmd/skidbladnir/terminal_exec.go`; deleting wrappers
alone is insufficient.
observation helpers may set their own account environment, never the new shell's.
discover claude's canonical native executable independently from command text
without running a provider; missing executable means unavailable claude native
capability. codex quota/native reads need only the existing daemon socket, never
a codex executable prerequisite. retain claude's versions-directory recognition.

process identity/account attribution remains separate from launch label:
recognize the live foreground provider and readable account-home environment,
resolve provider defaults only after a successful read proves the override unset,
then require one matching configured provider/home. unreadable, conflicting or
ambiguous evidence is unknown. do not infer from cwd, recent history, command
text or original launch profile. preserve existing exact native bindings and
content-free identity registration used by explicit configurations; remove
automatic hook injection and any artifact with no remaining supported caller.
`start --stdin` must still require observed profile and readiness before one send.

| capability | source/control contract |
| --- | --- |
| ordinary list/read/send/stop/close | existing tmux implementation, exact targets and receipts |
| model/effort/status/attention | existing supported terminal observation; no new native meaning |
| explicit native reads/mutations | existing exact conversation api or unavailable; no terminal substitution |
| quota | eligible fresh native account report first; otherwise attributable terminal report; this is the agreed exception to “no fallbacks” |

reuse existing five-second inventory and sixty-second native usage lanes; no
additional poller/store. one complete report per profile, never summed or spliced.
filter eligible FRESH native reports before receipt-order/machine tie breaking;
otherwise select fresh attributable terminal evidence using existing completeness/
session tie breaking; otherwise retain existing stale/absence diagnostics.
this supersedes the old newest-received-stale-native preference. retain the
one-hour staleness boundary. old-revision account candidates are
invalidated on metadata changes; unaffected terminal identities survive.
native windows retain `{usedPercent,resetsAt?}` and provider report time;
terminal windows retain optional integer `fiveHourUsedPercent`/
`sevenDayUsedPercent` in `0..100`, at least one, with no invented reset time.

retain claude grammar; add codex quota only from complete, anchored supported
statusline fields with proven window/used-versus-remaining semantics. no clipped
salvage or context-percentage guessing. qualify authored frames through the
existing [frame corpus](rules.md#testing), not a new parser harness.
document working provider statusline recipes and a quiet optional hint; never
overwrite provider settings or label a hidden footer “misconfigured”.

## 6. optional direct android push

retain existing observer inference, content, snapshot admission, mac native app,
foreground suppression/clicks, notification ids/channels, device schema4 memory
and current-only catchup. mac and authenticated foreground reads are independent
of firebase. phones and computers pair/manage sessions with push omitted.

### configuration, credentials and ownership

one standard apk; firebase messaging26.0.0, min-sdk33, existing package/signer.
no compiled project identity or google-services resource generation. public
configuration belongs to the optional hub yaml field:

```yaml
android_push:
  project_id: your-firebase-project
  application_id: '1:123456789:android:0123456789abcdef'
  sender_id: '123456789'
  api_key: your-public-android-client-key
```

all four fields are required, bounded nonempty firebase identifiers, and
`project_id` is a firebase project id (5–63 lowercase letters, digits or hyphens,
starting with a letter); the config decoder rejects unknown and duplicate fields.
no credential fields, backend selector or enable flag. omission or null is disabled.
`application_id` is firebase's android app id, not the apk package name. the
firebase android app must use `dev.niels.skidbladnir`; users install the ordinary
signed apk, never build their own. public client keys are not sender authority.

publish accepted public options as `androidPush` in the phones' fleet view:
`{id,projectId,applicationId,senderId,apiKey}`. `id` is lowercase sha256 of exact
UTF-8 `skidbladnir.android-push.v1\nPROJECT\nAPPLICATION\nSENDER\nAPIKEY`, with
no final newline; identifiers cannot contain newlines. use this identity in all
registration state and messages. the authenticated fleet fetch after join
transfers it automatically; no extra qr fields/manual phone setup.

private credential: fixed local
`${XDG_CONFIG_HOME:-~/.config}/skidbladnir/android-push-credentials.json`, regular
`0600` google service-account export inside the existing private config directory.
read at startup/ordinary apply, at most64kib; require `type:service_account`,
matching project_id, client_email, the google token uri and a nonempty private
key; oauth2 parses the key (pkcs8 or pkcs1 rsa) at each token fetch, so a bad key
reads `unavailable` after the first send. use explicit oauth2/google json
credentials with firebase.messaging scope; never ambient adc, shared yaml,
command-line key values, fleet cache or phone payloads. grant
only `cloudmessaging.messages.create`; document firebase API/project creation,
android app registration, key restrictions, file permissions, apply, pairing and
revocation/rotation. no new config framework, editor or credential broker.

users choosing push own firebase setup, key maintenance, google service
availability and phone google-play-services/notification permissions. skid has
NO project-operated notification backend. this tradeoff replaces the cancelled
global relay/domain/dns/caddy deployment; cloud cleanup is separate.

### runtime android lifecycle: prove before implementation

remove FirebaseInitProvider and all baked project values. leave messaging auto
init off. synchronously restore only paired-fleet public options in
Application.onCreate before messaging services/receivers; unpaired/disabled
startup initializes no FirebaseApp. initial pairing may initialize an absent
DEFAULT app. only one active fleet/configuration exists in this apk.

options remain immutable within a process. a changed options identity immediately
fences callbacks, enrollment, hints and jobs, persists desired options and reports
`restart required`. the owner accepts one app restart for project/options changes.
do not hot-delete/reinitialize firebase, kill the app automatically, add another
push process or change apks. same-options re-pairing preserves the SDK app but
requires a new fleet/device-scoped receiver tuple. cold startup restores options
from durable pairing state; force-stop needs reopening and pre-first-unlock
push remains unsupported. ordinary process death MUST support service-only wake.

registration uses SDK26 `register()`/`onRegistered(fid)`; no legacy token reader.
the actual gate found that returning to a project can yield a successful cached
callback while real delivery returns404. after cold initialization of changed
options, await SDK `unregister()` before `register()`. persist this reset intent
until unregister completes; do not reset an unchanged project on every startup.
callbacks must match captured runtime configuration/generation and current
installation identity before changing owned state. no callback can bind an old
fid to whichever fleet happens to be current. retain local receiver generation
floor across config changes, even when fid is unchanged; allocate fresh nonce.
the atomic private `push-v2.json` owns schema2, `generationFloor`, remembered
`configuration`, `resetRequired`, current optional `{configuration,fleetId,deviceId}`
target and optional registration/proof/hint. disabling clears target/registration,
not the floor or acknowledgement store. commit callback results only under the
pairing authority lock and the same target/floor; unsolicited SDK registration
callbacks schedule a fresh owned attempt, never adopt their supplied identity.
validate SDK-required options before starting its asynchronous registration so
malformed optional setup cannot crash session management.

feasibility gate, disposable actual-device proof BEFORE broad production rewrite:
no Firebase project resources in APK; same APK digest with projects A and B;
actual register callback and direct fcm challenge/data delivery; background
process death and cold service-only initialization; same-project re-pair; switch
and return projects after restart; old callbacks/proofs/hints rejected; disabled
startup quiet. failed/unavailable rows are NOT_RUN or failures, never assumed
supported. SDK source and a validate-only response are insufficient.

### hub, receiver and wire contract

the existing hub sends HTTPv1 directly to
`https://fcm.googleapis.com/v1/projects/{projectId}/messages:send` with its local
credential. reuse existing fcm request/error adapter and observer worker. remove
`cmd/skid-push-sender`, `deploy/push-sender`, relay signed requests, global fleet
registration, rate/replay/anti-abuse admission, relay tombstones/storage and
embedded sender origin. no new server/process/store/outbox or alternate backend.

derive phone identity from its fleet bearer. preserve existing routes:

| route | contract |
| --- | --- |
| `GET /v1/notifications/receivers/me` | `{configuration?,activeGeneration,desiredGeneration,status}`; configuration omitted only when disabled; status is `disabled`, `unconfigured`, `pending`, `active`, `unavailable` or `removed`; permission/restart are phone-local states |
| `PUT /v1/notifications/receivers/me` | `{configuration,generation,fid,clientNonce}`; reject non-current configuration/stale/conflicting tuple |
| `POST /v1/notifications/receivers/me/confirm` | `{configuration,generation,challenge}`; exact pending proof or idempotent exact committed proof |

observer privately owns full active and desired destinations, local generation
fences, pending random challenge/ten-minute expiry and last confirmed proof.
its existing private file is schema3: add optional `configuration`; each receiver
owns `{deviceId,configuration,floor,active?,desired?,confirmed?,removed,
registrationError,pushError}`. destinations contain
`{generation,fid,clientNonce,challenge,expires}`. epoch/revision/records retain
their existing meaning. no schema2 reader in final source.
persist pending proof BEFORE direct fcm send. keep an old active destination
while replacing it within the SAME configuration; only exact nonce/challenge
proof activates its replacement. confirmation-response loss is idempotent;
expired challenge renews on existing bounded reconciliation. membership removal
commits revocation and erases destinations locally before any further send; no
external delete/cleanup operation.

configuration change/disable retires all old active/pending destinations and
proofs while preserving observer epoch/revision/records and device acknowledgement.
keep per-device generation floors; new registration requires the current config
identity. transient fcm failure after activation affects push only. never echo
private credential errors or remote error text.

fcm remains data-only `message.fid`, restricted apk package, 120s ttl, separate
attention/sync/enrollment collapse keys; attention high priority, housekeeping
normal. no notification payload/text or terminal data. enrollment carries
`{kind:enrollment,fleetId,deviceId,configuration,generation,clientNonce,challenge}`;
hints `{kind:hint,schema:1,fleetId,deviceId,configuration,receiverGeneration,
epoch,revision,urgency}`. phone matches configuration, device and generation
before scheduling current-state fetch over tailscale. only authenticated snapshot
admission can present or acknowledge. preserve bounded WorkManager fetch retries;
no polling or permanent foreground service.

### disabled and failure behavior

- omitted android_push: healthy `disabled`; no SDK registration or enrollment
  retries and no setup/error banner. ordinary pairing/terminals/mac notices work.
- configured but absent/unreadable/wrong-project credential: `unconfigured` on
  the hub, with the one file/setup action. keep observer/mac/session APIs live;
  phone session management does not inherit a global error.
- configured but permission/google services unavailable, restart pending, or
  actual fcm failure: distinguish permission/restart/transport states and one
  recovery action. receiver-read success cannot erase transport failure.
- disabling push cancels its jobs/registration desire without resetting
  attention memory, cancelling/reposting existing notices or changing channels.
- invalid yaml retains accepted configuration under existing apply semantics;
  no raw credential/google error bodies, ids or notification contents in logs.

[setup guide](android-push-setup.md) and qualification own operational steps and
proof. independent phones, rotation/revocation, current-only admission and
output-only device acknowledgement remain required; two physical phones retain
the explicit owner deferral. no firebase setup becomes a core installation gate.

## 7. installer, cutover and removal

one signed host archive and bootstrap installer; internal components are not
separate user installations. `skid install` registers services/dependencies;
`skid update` stages a matching release; `skid uninstall` removes owned runtime.
keep the existing immutable release/signature/checksum workflow and catalogue.
include shell assets and the mac app in the archive. runtime startup never
downloads packages. clean installation must need no private repository checkout
or credentials.

install/reuse tmux and tailscale through native package/installation mechanisms.
linux uses apt/pacman and user systemd; mac uses the existing launch-agent model,
homebrew for tmux and official tailscale installer where absent. privilege,
login/extension permission and systemd linger needs must be reported honestly.
skid installs neither codex nor claude. zoxide remains optional; ssh/mosh remain
ordinary user tools.

initial host release matrix: macos arm64, ubuntu amd64 and current arch amd64
with systemd. the currently installed mac artifacts require macos 26; explicitly
pin and report that existing floor rather than promising older compatibility.
linux release qualification selects and records a supported ubuntu floor from
the actual dependency/build requirements before publication; no invented
“all ubuntu” claim. android 13+ retains the owner's dedicated older-version
verification waiver. ordinary android build/static and target-device notification
qualification are still required.
update `scripts/check-release` from min-sdk `36` to `33`; compile/target sdk need
not change. verify both mac binaries' encoded deployment targets against the
single declared release floor.

stage dependencies/assets and validate before atomic `current` switch; preserve
user yaml on normal upgrade. restart skid services only, never tmux or providers.
failed staging leaves the installed release untouched. coordinated releases
update host/phone together; no old-wire reader, ntfy branch or mixed-version mode.
uninstall stops skid services and deletes its fixed services, links and releases,
its marked shell blocks and its https:8443 serve route. retain settings/private
state, provider data, tmux and shared packages. no “purge everything” option in
this scope.

the next release's fresh-fleet cutover and every cross-repo obligation it carries
are recorded in [release cutover](issues/release-cutover.md). the completed
generalization cutover below is retained as its record.

cutover preparation belongs to the root integrator and personal dev-server
consumer, not a permanent legacy-config parser in skid:

1. capture content-free installed settings/identity inventories and current
   parity references; translate personal effective commands into desired yaml.
   dev-server continues owning the user's provider tools/daemons independently.
   account wrappers alone do not reproduce old permission, remote or identity-plugin
   arguments: encode intended effects explicitly and remove all dependencies on
   the deleted provider-command shim.
2. stage the new layout and enroll existing computers, preserving `mh-` handles.
   drain/stop old observer writers BEFORE exporting their final committed
   epoch/revision/records. never activate from an earlier live-state copy.
3. use temporary host conversion tooling and an android conversion build with
   the SAME app id/signing key. validate old stores; copy continuity fields and
   drop retired ntfy config/subscriptions. atomically stage the new schema while
   owners are stopped; retain originals until validated. preserve channel/app
   data, exact records, epoch and admitted/local revisions; conversion is
   idempotent and must not emit notifications. the owner approved this extra
   staged app update; delete converter sources from the final product.
4. hard-cut old service/auth/ntfy paths; activate the matching new release,
   preserving sessions, history and current interaction. run one qr scan per
   phone. a cutover pause is allowed; duplicate authorities are not.
5. update dev-server to invoke ordinary skid installation and write only the
   authoritative hub yaml. remove its copied skid implementation/services/assets.
   update jarvis create option/schema and configuration/auth adapter; preserve
   terminal/native target meanings and unrelated personal enums.
6. delete conversion-only material and dead callers/assets after qualification.
   keep no dual path. any rollback after new state commits requires a deliberate
   matching-state recovery, not switching a binary symlink under incompatible data.

conversion fields: android retains `observerEpoch`, `admittedRevision`,
`localRevision` and complete device records; drops `config`/`desiredRegistration`.
observer retains epoch/revision and complete attention records; drops singleton
subscription/receiver counter. go device schema4 memory is unchanged, including
`resetId`. retain store locations; final readers accept only the new schema,
rejecting unconverted state rather than silently creating empty memory.
conversion disables old owners/receivers/workers across restart; reruns cannot
overwrite newer converted state. clear retired pairing credentials into explicit
pairing-required state, preserving selections. retain channel ids, notification
tags, intent actions/identity extras; no cancellation/reposting during conversion.

## 8. exclusive work slices and content ownership

each slice gets an implementer, content designer and independent reviewer.
designer supplies actual copy/examples before implementation; reviewer writes no
production files/tests. exclusive ownership below; root assigns unlisted paths.
freeze shared contracts before dependent work.

| slice | exclusive paths / dependencies | designer's deliverable and standard |
| --- | --- | --- |
| a: config/fleet authority | new `internal/fleet/`; `internal/hostconfig/`, `internal/auth/`, `internal/pairing/`, `internal/machine/`; first | default/commented yaml; field/path-specific errors; distinct accepted/applied/pending/revoked states; no secret values |
| b: command/observation | `internal/agentruntime/`, `internal/agentcontrol/`, `internal/sessions/`, `internal/process/`, `deployment/providers/`, `deployment/native-control/`; consumes a | exact command examples, supported statusline recipes, unknown/source diagnostics; no implied daemon launch |
| c: host/desktop adapters | `internal/gateway/`, `internal/fleetclient/`, `internal/sessionui/`; consumes a/b | preserve current screens/keys/layout; add only required setup/error copy and bounded arbitrary-name rendering |
| d: observer/mac | `internal/notifier/`, `internal/attention/`, `internal/macnotifications/`, `cmd/skid-notifications/`, `macos/`; consumes a/c | retain closed reason/health strings; apple-terminal failure counterpart and exact focus behavior |
| e: android | `android/` including gradle/manifest/locks; consumes a/c/d/f | retain screens, channel ids and notification copy; pairing/permission/optional-push errors each identify one next action |
| f: optional push | `internal/fcmpush/`, `internal/notifier/{observer,receivers,http}.go`; delete `internal/pushsender/`, `cmd/skid-push-sender/`, `deploy/push-sender/`; consumes a public config; root composes in `cmd/skidbladnir/` | user setup guide, optional-state copy and content-free errors; no delivery claim from acceptance |
| g: installer/release | new `internal/install/`, `scripts/install`, `deployment/install/`, `deployment/services/`; existing `scripts/build-host-*`, `scripts/build-mac-app`, `scripts/release`, `scripts/check-release`; consumes a/b/d/f | install/update/uninstall steps; exact supported matrix, privilege boundaries and preserved-data statement |
| root: composition/cutover | `cmd/skidbladnir/`, `internal/agentcli/`, `internal/terminalclient/`, `internal/tmux/`, `catalog/`, `scripts/check*` except g's `check-release`, `.github/`, `go.mod`, `go.sum`, docs | final onboarding: install → optional profiles/network → join/connect; parity and truthful qualification report |
| external consumers | separately assigned dev-server `lib/skid*.sh`, owned assets/manifests; jarvis skid adapter/tool schema; coordinated after a/b/g | personal effective settings retained; no skid product assumptions about the owner's fleet |

do not split android store/controller changes among competing owners. d owns
shared attention schemas; root coordinates their freeze, does not duplicate them.
root owns extraction of `internal/attention/file.go`'s durable replace into
`internal/privatefile/` for a/d/f to call, preserving its fsync/error contract.
configuration definitions belong to a; b resolves them into runtime profiles.
remove `hostconfig`'s reverse dependency on `agentruntime` before parallel work.
g supplies package checks to root; only root changes check composition. external
repositories require their own scoped assignments before edits.

content: reuse existing copy. new copy states fact, impact, one recovery action;
no internal field names except config errors. examples: “config accepted; arch is
offline”, “device removed”. retain existing notification
health/reason strings. receipts/logs contain no terminal excerpts, accounts,
commands or credentials.

## 9. acceptance and execution order

per slice: refresh callers → independent design/content review → disposable
end-to-end RED → implement GREEN → review code/probe sensitivity → refactor →
rerun affected proof → delete temporary tests before commit. preserved behavior
uses passing baseline comparisons. retain engineering checks/provider corpus
under the [testing policy](rules.md#testing).

| journey | required evidence; rejects |
| --- | --- |
| a. fresh local install | clean supported host, zero providers, empty profiles, no tailnet login/internet after install; shell/create/attach and local mac notices work; upgrade/uninstall preserve user/session data |
| b. arbitrary fleet | three differently named hosts, new host auto-appears on running desktop/two phones; each gateway acts only locally; manual ssh remains source terminal; lost join response does not duplicate membership |
| c. config/access | apply from non-hub; invalid yaml retains settings; invalid local shell does NOT block simultaneous revocation; offline host catches up; removal closes connections, preserves tmux and the removed bearer gets 401 |
| d. restart/recovery | join, recover and a failed state write restart the service with tmux intact; restart preserves membership; lost join reply re-joins with a fresh invite; new-fleet recovery rejects old trust; the release follows [release cutover](issues/release-cutover.md) |
| e. command/capability | bash/zsh aliases/functions/env/quotes/composition/cwd/error options, return/failure/exec; no injected flags/daemon lifecycle; no helper still permits normal terminal api; jarvis exact-target/uncertain-write behavior retained |
| f. quota/observation | native present/absent/stale, newer stale plus older fresh native plus fresh terminal, attributable codex/claude frames, missing/clipped/ambiguous accounts, config revision race; one row/profile, whole-source selection, unchanged model/effort/status |
| g. push/admission | two independently paired phones, fid rotation, response loss/restart, wrong configuration/challenge/nonce/generation, removal during fcm outage; revoked receiver cannot resurrect; read success does not clear push failure |
| h. actual presentation | current reason composites, sound, Unicode names, exact click/focus/output ack across mac/two phones; locked/reclaimed/dozing phone, tailscale loss/recovery, reordered hints, session replacement; only still-current unseen episodes |
| i. parity/cutover | owner settings/rendering/keyboard and terminal flows before/after; both mac terminal adapters; no ntfy/dev-server runtime dependency; matching releases, one scan/phone, retained sessions/history/ack |

presentation assertions: acknowledging on phone a leaves b unchanged; clicking or
swiping alone never acknowledges; unproven focus never suppresses; departure
creates no reminder; replayed callbacks never sound twice. cutover proof includes
real old-store conversion, not empty fixtures.

run dependency order a → b/c; d/f can follow frozen a contracts; optional push production changes follow the
runtime feasibility gate; e after shared
contracts; g packages completed owners; root coordinates consumers and final
cutover. temporary cross-slice journeys belong to root, not duplicated harnesses.
tmux/live hosts/adb/phone use needs explicit current-turn authorization under
[agent guidance](../AGENTS.md); this plan grants none.

record source/version, boundary, expected/observed result and limits without user
content. missing/unrun boundaries are `NOT_RUN`. release requires passing proof
or explicit owner waivers; engineering checks are not behavioral acceptance.
android 13 compatibility qualification remains waived, not passed.
2026-10-20: the owner separately deferred
[two-physical-phone acceptance](issues/unrun-acceptance.md).
retain multi-device behavior and synthetic independent-principal proof; do not
substitute that proof for the deferred physical rows. single-phone real push
and installed cutover qualification remain required.

## 10. costs and remaining proof

retain [the agreed tradeoffs](generalization.md): delayed offline revocation,
hub-dependent edits/notifications, unknown unattributable usage, same-account
profiles and supported-platform limits. android push is optional: its users own
firebase project/credentials/maintenance and google services. skid operates no
notification backend. project changes cost one phone app restart; moving the hub
is a new fleet and needs a matching private credential on the new hub when push
is configured. join, recover and failed state writes restart the service. the
fleet has no signatures, relays or handoff; this design retains the approved
temporary conversion app update. no consensus,
permanent legacy reader or automatic downgrade. deleting tests retains
[the existing regression coverage gap](issues/test-system-reset.md).

no product preference remains open. implementation must qualify the ubuntu floor,
shell startup, apple terminal focus and physical background delivery. none is
proved by this design review. an unavoidable behavior change requires its
consequence/recommendation presented before changing this contract.

2026-10-20 original review, before the optional-push revision: independent authority/handoff, runtime/install/quota and
notification/push/content reviews found no remaining blocking design contradiction
after correction. this records design review only; implementation and live proof
remain `NOT_RUN`.

2026-10-20 implementation clarification: jarvis api/schema changes remain in
scope; the owner explicitly deferred its cross-user deployment integration.
[the deployment blocker](issues/jarvis-installed-state-access.md) must remain
visible and is not a standalone-skid release acceptance claim.
